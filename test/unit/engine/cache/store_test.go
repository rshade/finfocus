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

// TestNewFileStore verifies file store creation and directory setup.
func TestNewFileStore(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		directory  string
		enabled    bool
		ttlSeconds int
		maxSizeMB  int
		wantErr    bool
	}{
		{
			name:       "valid enabled store",
			directory:  filepath.Join(tempDir, "cache1"),
			enabled:    true,
			ttlSeconds: 3600,
			maxSizeMB:  100,
			wantErr:    false,
		},
		{
			name:       "disabled store",
			directory:  "",
			enabled:    false,
			ttlSeconds: 0,
			maxSizeMB:  0,
			wantErr:    false,
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
			store, err := cache.NewFileStore(tt.directory, tt.enabled, tt.ttlSeconds, tt.maxSizeMB)
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
			}
		})
	}
}

// TestFileStore_SetAndGet verifies basic cache set/get operations.
func TestFileStore_SetAndGet(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	testData := map[string]string{
		"user": "alice",
		"age":  "30",
	}
	data, err := json.Marshal(testData)
	require.NoError(t, err)

	// Set cache entry
	err = store.Set("test-key", json.RawMessage(data))
	require.NoError(t, err)

	// Get cache entry
	entry, err := store.Get("test-key")
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Verify data
	var retrieved map[string]string
	err = json.Unmarshal(entry.Data, &retrieved)
	require.NoError(t, err)
	assert.Equal(t, testData, retrieved)

	// Verify entry metadata
	assert.Equal(t, "test-key", entry.Key)
	assert.Equal(t, 3600, entry.TTLSeconds)
	assert.False(t, entry.IsExpired())
}

// TestFileStore_GetNonExistent verifies handling of missing cache entries.
func TestFileStore_GetNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	entry, err := store.Get("nonexistent-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheNotFound)
	assert.Nil(t, entry)
}

// TestFileStore_TTLExpiration verifies TTL expiration handling.
func TestFileStore_TTLExpiration(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	// Create store with 1-second TTL
	store, err := cache.NewFileStore(cacheDir, true, 1, 100)
	require.NoError(t, err)

	testData := []byte(`{"test": "data"}`)

	// Set cache entry with 1-second TTL
	err = store.Set("expiring-key", json.RawMessage(testData))
	require.NoError(t, err)

	// Immediately retrieve (should succeed)
	entry, err := store.Get("expiring-key")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.False(t, entry.IsExpired())

	// Wait for TTL to expire (1 second + buffer)
	time.Sleep(1200 * time.Millisecond)

	// Try to retrieve expired entry
	entry, err = store.Get("expiring-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheExpired)
	assert.Nil(t, entry)

	// Verify cache file was deleted (async cleanup)
	time.Sleep(100 * time.Millisecond) // Give async cleanup time to complete
	_, err = os.Stat(filepath.Join(cacheDir, "expiring-key.json"))
	assert.True(t, os.IsNotExist(err), "Expired cache file should be deleted")
}

// TestFileStore_Delete verifies cache entry deletion.
func TestFileStore_Delete(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	testData := []byte(`{"test": "data"}`)

	// Set cache entry
	err = store.Set("delete-key", json.RawMessage(testData))
	require.NoError(t, err)

	// Verify entry exists
	_, err = store.Get("delete-key")
	require.NoError(t, err)

	// Delete entry
	err = store.Delete("delete-key")
	require.NoError(t, err)

	// Verify entry no longer exists
	_, err = store.Get("delete-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheNotFound)

	// Delete again (should be idempotent)
	err = store.Delete("delete-key")
	require.NoError(t, err)
}

// TestFileStore_Clear verifies clearing all cache entries.
func TestFileStore_Clear(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	// Set multiple cache entries
	for i := range 5 {
		key := filepath.ToSlash(filepath.Join("key", string(rune('0'+i))))
		data := []byte(`{"index": ` + string(rune('0'+i)) + `}`)
		err = store.Set(key, json.RawMessage(data))
		require.NoError(t, err)
	}

	// Verify entries exist
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	// Clear all entries
	err = store.Clear()
	require.NoError(t, err)

	// Verify no entries remain
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestFileStore_CleanupExpired verifies cleanup of expired entries.
func TestFileStore_CleanupExpired(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 1, 100)
	require.NoError(t, err)

	// Set entries that will expire
	for i := range 3 {
		key := filepath.ToSlash(filepath.Join("expiring", string(rune('0'+i))))
		data := []byte(`{"index": ` + string(rune('0'+i)) + `}`)
		err = store.Set(key, json.RawMessage(data))
		require.NoError(t, err)
	}

	// Verify all entries exist
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Wait for TTL to expire
	time.Sleep(1200 * time.Millisecond)

	// Run cleanup
	err = store.CleanupExpired()
	require.NoError(t, err)

	// Verify all expired entries were removed
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestFileStore_Size verifies cache size calculation.
func TestFileStore_Size(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	// Initially empty
	size, err := store.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	// Set cache entry
	testData := []byte(`{"key": "value", "number": 42}`)
	err = store.Set("size-key", json.RawMessage(testData))
	require.NoError(t, err)

	// Verify size increased
	size, err = store.Size()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0))
}

// TestFileStore_Count verifies cache entry counting.
func TestFileStore_Count(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	// Initially empty
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add entries
	for i := range 10 {
		key := filepath.ToSlash(filepath.Join("count", string(rune('0'+i))))
		data := []byte(`{"index": ` + string(rune('0'+i)) + `}`)
		err = store.Set(key, json.RawMessage(data))
		require.NoError(t, err)
	}

	// Verify count
	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, 10, count)
}

// TestFileStore_DisabledOperations verifies disabled cache behavior.
func TestFileStore_DisabledOperations(t *testing.T) {
	store, err := cache.NewFileStore("", false, 0, 0)
	require.NoError(t, err)
	assert.False(t, store.IsEnabled())

	testData := []byte(`{"test": "data"}`)

	// All operations should return ErrCacheDisabled
	_, err = store.Get("key")
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	err = store.Set("key", json.RawMessage(testData))
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	err = store.Delete("key")
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	err = store.Clear()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	err = store.CleanupExpired()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	_, err = store.Size()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)

	_, err = store.Count()
	assert.ErrorIs(t, err, cache.ErrCacheDisabled)
}

// TestFileStore_EmptyKeyValidation verifies empty key handling.
func TestFileStore_EmptyKeyValidation(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	testData := []byte(`{"test": "data"}`)

	// Empty key should fail
	_, err = store.Get("")
	assert.ErrorIs(t, err, cache.ErrInvalidCacheKey)

	err = store.Set("", json.RawMessage(testData))
	assert.ErrorIs(t, err, cache.ErrInvalidCacheKey)

	err = store.Delete("")
	assert.ErrorIs(t, err, cache.ErrInvalidCacheKey)
}

// TestFileStore_AtomicWrite verifies atomic write operations.
func TestFileStore_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	// Set initial value
	data1 := []byte(`{"version": 1}`)
	err = store.Set("atomic-key", json.RawMessage(data1))
	require.NoError(t, err)

	// Overwrite with new value
	data2 := []byte(`{"version": 2}`)
	err = store.Set("atomic-key", json.RawMessage(data2))
	require.NoError(t, err)

	// Verify latest value
	entry, err := store.Get("atomic-key")
	require.NoError(t, err)
	require.NotNil(t, entry)

	var result map[string]int
	err = json.Unmarshal(entry.Data, &result)
	require.NoError(t, err)
	assert.Equal(t, 2, result["version"])

	// Verify no .tmp files left behind
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "No temporary files should remain")
	}
}

// TestFileStore_KeySanitization verifies special character handling in keys.
func TestFileStore_KeySanitization(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	testCases := []string{
		"key/with/slashes",
		"key\\with\\backslashes",
		"key:with:colons",
		"key/mixed\\chars:here",
	}

	testData := []byte(`{"test": "data"}`)

	for _, key := range testCases {
		t.Run(key, func(t *testing.T) {
			// Set with special characters
			err := store.Set(key, json.RawMessage(testData))
			require.NoError(t, err)

			// Get with same key
			entry, err := store.Get(key)
			require.NoError(t, err)
			require.NotNil(t, entry)
			assert.Equal(t, key, entry.Key)

			// Delete with same key
			err = store.Delete(key)
			require.NoError(t, err)
		})
	}
}

// TestFileStore_MultipleEntries verifies concurrent entry handling.
func TestFileStore_MultipleEntries(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(t, err)

	// Set multiple entries with different data
	entries := map[string][]byte{
		"user1": []byte(`{"name": "Alice", "age": 30}`),
		"user2": []byte(`{"name": "Bob", "age": 25}`),
		"user3": []byte(`{"name": "Charlie", "age": 35}`),
	}

	for key, data := range entries {
		err := store.Set(key, json.RawMessage(data))
		require.NoError(t, err)
	}

	// Verify all entries can be retrieved independently
	for key, expectedData := range entries {
		entry, err := store.Get(key)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.JSONEq(t, string(expectedData), string(entry.Data))
	}
}

// TestFileStore_MixedExpiredAndValid verifies handling of mixed cache states.
func TestFileStore_MixedExpiredAndValid(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 1, 100)
	require.NoError(t, err)

	// Set entry that will expire soon
	expiringData := []byte(`{"type": "expiring"}`)
	err = store.Set("expiring-key", json.RawMessage(expiringData))
	require.NoError(t, err)

	// Wait for first entry to expire
	time.Sleep(1200 * time.Millisecond)

	// Set entry with fresh TTL
	validData := []byte(`{"type": "valid"}`)
	err = store.Set("valid-key", json.RawMessage(validData))
	require.NoError(t, err)

	// Run cleanup
	err = store.CleanupExpired()
	require.NoError(t, err)

	// Verify expired entry is gone
	_, err = store.Get("expiring-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, cache.ErrCacheNotFound)

	// Verify valid entry remains
	entry, err := store.Get("valid-key")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.JSONEq(t, string(validData), string(entry.Data))
}

// BenchmarkFileStore_SetAndGet benchmarks cache set and get operations.
func BenchmarkFileStore_SetAndGet(b *testing.B) {
	tempDir := b.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 3600, 100)
	require.NoError(b, err)

	testData := []byte(`{"benchmark": "data", "value": 42}`)

	b.ResetTimer()
	for range b.N {
		_ = store.Set("bench-key", json.RawMessage(testData))
		_, _ = store.Get("bench-key")
	}
}

// BenchmarkFileStore_CleanupExpired benchmarks cleanup operations.
func BenchmarkFileStore_CleanupExpired(b *testing.B) {
	tempDir := b.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")

	store, err := cache.NewFileStore(cacheDir, true, 1, 100)
	require.NoError(b, err)

	// Create 100 entries
	for i := range 100 {
		key := filepath.ToSlash(filepath.Join("bench", string(rune('0'+i/10)), string(rune('0'+i%10))))
		data := []byte(`{"index": ` + string(rune('0'+i%10)) + `}`)
		_ = store.Set(key, json.RawMessage(data))
	}

	// Wait for entries to expire
	time.Sleep(1200 * time.Millisecond)

	b.ResetTimer()
	for range b.N {
		_ = store.CleanupExpired()
	}
}
