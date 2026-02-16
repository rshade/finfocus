package cache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/cache"
)

// TestNewBoltStore verifies BoltDB store creation and directory setup.
func TestNewBoltStore(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		directory  string
		enabled    bool
		ttlSeconds int
		maxSizeMB  int
		wantErr    bool
		wantNil    bool
	}{
		{
			name:       "valid enabled store",
			directory:  filepath.Join(tempDir, "cache1"),
			enabled:    true,
			ttlSeconds: 3600,
			maxSizeMB:  100,
		},
		{
			name:       "disabled store",
			directory:  "",
			enabled:    false,
			ttlSeconds: 0,
			maxSizeMB:  0,
		},
		{
			name:       "empty directory with enabled",
			directory:  "",
			enabled:    true,
			ttlSeconds: 3600,
			maxSizeMB:  100,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := cache.NewBoltStore(tt.directory, tt.enabled, tt.ttlSeconds, tt.maxSizeMB)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, store)
			} else {
				require.NoError(t, err)
				require.NotNil(t, store)

				if tt.enabled {
					assert.Equal(t, tt.ttlSeconds, store.GetTTL())
					assert.Equal(t, tt.directory, store.GetDirectory())
					assert.True(t, store.IsEnabled())

					// Verify directory was created
					_, err := os.Stat(tt.directory)
					require.NoError(t, err)
				} else {
					assert.False(t, store.IsEnabled())
				}
				require.NoError(t, store.Close())
			}
		})
	}
}

// TestBoltStore_SetAndGet verifies basic cache set/get operations.
func TestBoltStore_SetAndGet(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	testData := map[string]string{
		"user": "alice",
		"age":  "30",
	}
	data, err := json.Marshal(testData)
	require.NoError(t, err)

	key := "projected/aws/ec2:Instance/us-east-1/t3.micro"

	// Set cache entry
	err = store.Set(key, json.RawMessage(data))
	require.NoError(t, err)

	// Get cache entry
	entry, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Verify data
	var retrieved map[string]string
	err = json.Unmarshal(entry.Data, &retrieved)
	require.NoError(t, err)
	assert.Equal(t, testData, retrieved)

	// Verify entry metadata
	assert.Equal(t, key, entry.Key)
	assert.Equal(t, 3600, entry.TTLSeconds)
	assert.False(t, entry.IsExpired())
}

// TestBoltStore_GetNonExistent verifies handling of missing cache entries.
func TestBoltStore_GetNonExistent(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	entry, err := store.Get("projected/nonexistent-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheNotFound)
	assert.Nil(t, entry)
}

// TestBoltStore_TTLExpiration verifies TTL expiration handling.
func TestBoltStore_TTLExpiration(t *testing.T) {
	tempDir := t.TempDir()

	// Create store with 1-second TTL
	store, err := cache.NewBoltStore(tempDir, true, 1, 100)
	require.NoError(t, err)
	defer store.Close()

	testData := []byte(`{"test": "data"}`)
	key := "projected/aws/expiring"

	// Set cache entry with 1-second TTL
	err = store.Set(key, json.RawMessage(testData))
	require.NoError(t, err)

	// Immediately retrieve (should succeed)
	entry, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.False(t, entry.IsExpired())

	// Wait for TTL to expire (1 second + buffer)
	time.Sleep(1500 * time.Millisecond)

	// Try to retrieve expired entry
	entry, err = store.Get(key)
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheExpired)
	assert.Nil(t, entry)
}

// TestBoltStore_Delete verifies cache entry deletion.
func TestBoltStore_Delete(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	testData := []byte(`{"test": "data"}`)
	key := "projected/aws/delete-test"

	// Set cache entry
	err = store.Set(key, json.RawMessage(testData))
	require.NoError(t, err)

	// Verify entry exists
	_, err = store.Get(key)
	require.NoError(t, err)

	// Delete entry
	err = store.Delete(key)
	require.NoError(t, err)

	// Verify entry no longer exists
	_, err = store.Get(key)
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheNotFound)

	// Delete again (should be idempotent)
	err = store.Delete(key)
	require.NoError(t, err)
}

// TestBoltStore_Clear verifies clearing all cache entries.
func TestBoltStore_Clear(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	data := json.RawMessage(`{"test":true}`)

	// Set entries across buckets
	require.NoError(t, store.Set("projected/aws/test1", data))
	require.NoError(t, store.Set("actual/aws/test2/2025-01-01/2025-02-01/abc", data))
	require.NoError(t, store.Set("recommendations/multi/test3", data))

	// Verify entries exist
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Clear all entries
	err = store.Clear()
	require.NoError(t, err)

	// Verify no entries remain
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestBoltStore_Size verifies cache size calculation.
func TestBoltStore_Size(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	// BoltDB file exists even when empty
	size, err := store.Size()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0))

	// Add entry
	testData := []byte(`{"key": "value", "number": 42}`)
	err = store.Set("projected/aws/size-test", json.RawMessage(testData))
	require.NoError(t, err)

	// Verify size
	size, err = store.Size()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0))
}

// TestBoltStore_Count verifies cache entry counting.
func TestBoltStore_Count(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	// Initially empty
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add entries
	data := json.RawMessage(`{"test":true}`)
	for i := range 10 {
		key := cache.BuildProjectedKey("aws", "ec2:Instance", "us-east-1",
			"type-"+string(rune('0'+i)))
		err = store.Set(key, data)
		require.NoError(t, err)
	}

	// Verify count
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, 10, count)
}

// TestBoltStore_DisabledOperations verifies disabled cache behavior.
func TestBoltStore_DisabledOperations(t *testing.T) {
	store, err := cache.NewBoltStore("", false, 0, 0)
	require.NoError(t, err)
	assert.False(t, store.IsEnabled())

	testData := []byte(`{"test": "data"}`)

	// Get returns ErrCacheDisabled
	_, err = store.Get("projected/key")
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	// Set is no-op when disabled (returns nil, not error)
	err = store.Set("projected/key", json.RawMessage(testData))
	assert.NoError(t, err)

	// Delete returns ErrCacheDisabled
	err = store.Delete("projected/key")
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	// Clear returns ErrCacheDisabled
	err = store.Clear()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	// InvalidateByPrefix returns ErrCacheDisabled
	_, err = store.InvalidateByPrefix("projected/")
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	// Size returns ErrCacheDisabled
	_, err = store.Size()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	// Count returns ErrCacheDisabled
	_, err = store.Count()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)
}

// TestBoltStore_EmptyKeyValidation verifies empty key handling.
func TestBoltStore_EmptyKeyValidation(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	testData := []byte(`{"test": "data"}`)

	// Empty key should fail
	_, err = store.Get("")
	assert.ErrorIs(t, err, cache.ErrInvalidCacheKey)

	err = store.Set("", json.RawMessage(testData))
	assert.ErrorIs(t, err, cache.ErrInvalidCacheKey)

	err = store.Delete("")
	assert.ErrorIs(t, err, cache.ErrInvalidCacheKey)
}

// TestBoltStore_AtomicOverwrite verifies overwrite behavior.
func TestBoltStore_AtomicOverwrite(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	key := "projected/aws/atomic-test"

	// Set initial value
	data1 := []byte(`{"version": 1}`)
	err = store.Set(key, json.RawMessage(data1))
	require.NoError(t, err)

	// Overwrite with new value
	data2 := []byte(`{"version": 2}`)
	err = store.Set(key, json.RawMessage(data2))
	require.NoError(t, err)

	// Verify latest value
	entry, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, entry)

	var result map[string]int
	err = json.Unmarshal(entry.Data, &result)
	require.NoError(t, err)
	assert.Equal(t, 2, result["version"])
}

// TestBoltStore_MultipleEntries verifies handling of multiple entries.
func TestBoltStore_MultipleEntries(t *testing.T) {
	tempDir := t.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(t, err)
	defer store.Close()

	entries := map[string][]byte{
		"projected/aws/user1":                        []byte(`{"name": "Alice", "age": 30}`),
		"projected/aws/user2":                        []byte(`{"name": "Bob", "age": 25}`),
		"actual/aws/user3/2025-01-01/2025-02-01/abc": []byte(`{"name": "Charlie", "age": 35}`),
	}

	for key, data := range entries {
		err := store.Set(key, json.RawMessage(data))
		require.NoError(t, err)
	}

	for key, expectedData := range entries {
		entry, err := store.Get(key)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.JSONEq(t, string(expectedData), string(entry.Data))
	}
}

// BenchmarkBoltStore_SetAndGet benchmarks cache set and get operations.
func BenchmarkBoltStore_SetAndGet(b *testing.B) {
	tempDir := b.TempDir()

	store, err := cache.NewBoltStore(tempDir, true, 3600, 100)
	require.NoError(b, err)
	defer store.Close()

	testData := []byte(`{"benchmark": "data", "value": 42}`)
	key := "projected/aws/bench-test"

	b.ResetTimer()
	for range b.N {
		_ = store.Set(key, json.RawMessage(testData))
		_, _ = store.Get(key)
	}
}
