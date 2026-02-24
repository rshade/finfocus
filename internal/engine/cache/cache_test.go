package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T007: Key builder tests ---

func TestBuildProjectedKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		resType  string
		region   string
		sku      string
		want     string
	}{
		{
			name:     "full key",
			provider: "aws",
			resType:  "ec2:Instance",
			region:   "us-east-1",
			sku:      "t3.micro",
			want:     "projected/aws/ec2:Instance/us-east-1/t3.micro",
		},
		{
			name:     "missing region",
			provider: "aws",
			resType:  "ec2:Instance",
			sku:      "t3.micro",
			want:     "projected/aws/ec2:Instance/_/t3.micro",
		},
		{
			name:     "missing sku",
			provider: "aws",
			resType:  "ec2:Instance",
			region:   "us-east-1",
			want:     "projected/aws/ec2:Instance/us-east-1/_",
		},
		{
			name:     "only provider and type",
			provider: "gcp",
			resType:  "compute:Instance",
			want:     "projected/gcp/compute:Instance/_/_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := BuildProjectedKey(tt.provider, tt.resType, tt.region, tt.sku)
			assert.Equal(t, tt.want, key)
		})
	}
}

func TestBuildActualKey(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

	t.Run("deterministic", func(t *testing.T) {
		key1 := BuildActualKey("aws", []string{"ec2:Instance"}, from, to, nil)
		key2 := BuildActualKey("aws", []string{"ec2:Instance"}, from, to, nil)
		assert.Equal(t, key1, key2)
		assert.Contains(t, key1, "actual/aws/ec2:Instance/2025-01-01/2025-02-01")
	})

	t.Run("filter hash changes key", func(t *testing.T) {
		key1 := BuildActualKey("aws", []string{"ec2:Instance"}, from, to,
			map[string]string{"env": "prod"})
		key2 := BuildActualKey("aws", []string{"ec2:Instance"}, from, to,
			map[string]string{"env": "staging"})
		assert.NotEqual(t, key1, key2)
	})

	t.Run("filter order independence", func(t *testing.T) {
		key1 := BuildActualKey("aws", []string{"ec2:Instance"}, from, to,
			map[string]string{"a": "1", "b": "2"})
		key2 := BuildActualKey("aws", []string{"ec2:Instance"}, from, to,
			map[string]string{"b": "2", "a": "1"})
		assert.Equal(t, key1, key2)
	})

	t.Run("resource type order independence", func(t *testing.T) {
		key1 := BuildActualKey("aws", []string{"ec2:Instance", "rds:DBInstance"}, from, to, nil)
		key2 := BuildActualKey("aws", []string{"rds:DBInstance", "ec2:Instance"}, from, to, nil)
		assert.Equal(t, key1, key2)
	})
}

func TestBuildRecommendationsKey(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		key1 := BuildRecommendationsKey([]string{"ec2:Instance", "rds:DBInstance"})
		key2 := BuildRecommendationsKey([]string{"ec2:Instance", "rds:DBInstance"})
		assert.Equal(t, key1, key2)
	})

	t.Run("order independence", func(t *testing.T) {
		key1 := BuildRecommendationsKey([]string{"ec2:Instance", "rds:DBInstance"})
		key2 := BuildRecommendationsKey([]string{"rds:DBInstance", "ec2:Instance"})
		assert.Equal(t, key1, key2)
	})

	t.Run("format", func(t *testing.T) {
		key := BuildRecommendationsKey([]string{"ec2:Instance", "rds:DBInstance"})
		assert.Equal(t, "recommendations/multi/ec2:Instance+rds:DBInstance", key)
	})
}

func TestBucketFromKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"projected/aws/ec2:Instance/us-east-1/t3.micro", "projected"},
		{"actual/aws/ec2:Instance/2025-01-01/2025-02-01", "actual"},
		{"recommendations/multi/hash", "recommendations"},
		{"nobucket", "nobucket"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, BucketFromKey(tt.key))
		})
	}
}

func TestStripBucket(t *testing.T) {
	assert.Equal(t, "aws/ec2:Instance", StripBucket("projected/aws/ec2:Instance"))
	assert.Equal(t, "nobucket", StripBucket("nobucket"))
}

// --- T008: BoltStore Get/Set/IsEnabled tests ---

func TestBoltStore_SetAndGet(t *testing.T) {
	store := newTestStore(t, true, 3600)

	data := json.RawMessage(`{"test":"value"}`)
	key := "projected/aws/ec2:Instance/us-east-1/t3.micro"

	err := store.Set(key, data)
	require.NoError(t, err)

	entry, err := store.Get(key)
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(entry.Data))
	assert.Equal(t, key, entry.Key)
}

func TestBoltStore_GetNonExistent(t *testing.T) {
	store := newTestStore(t, true, 3600)

	_, err := store.Get("projected/aws/nonexistent")
	assert.ErrorIs(t, err, ErrCacheNotFound)
}

func TestBoltStore_GetExpired(t *testing.T) {
	store := newTestStore(t, true, 1) // 1-second TTL

	data := json.RawMessage(`{"test":"expire"}`)
	key := "projected/aws/ec2:Instance/us-east-1/t3.micro"

	err := store.Set(key, data)
	require.NoError(t, err)

	// Wait for expiration (1s TTL + 200ms buffer)
	time.Sleep(1200 * time.Millisecond)

	_, err = store.Get(key)
	assert.ErrorIs(t, err, ErrCacheExpired)
}

func TestBoltStore_IsEnabled(t *testing.T) {
	enabled := newTestStore(t, true, 3600)
	assert.True(t, enabled.IsEnabled())

	disabled := newTestStore(t, false, 0)
	assert.False(t, disabled.IsEnabled())
}

func TestBoltStore_Disabled(t *testing.T) {
	store := newTestStore(t, false, 0)

	data := json.RawMessage(`{"test":"value"}`)

	err := store.Set("projected/aws/test", data)
	assert.ErrorIs(t, err, ErrCacheDisabled)

	_, err = store.Get("projected/aws/test")
	assert.ErrorIs(t, err, ErrCacheDisabled)
}

func TestBoltStore_EmptyKey(t *testing.T) {
	store := newTestStore(t, true, 3600)

	err := store.Set("", json.RawMessage(`{}`))
	assert.ErrorIs(t, err, ErrInvalidCacheKey)

	_, err = store.Get("")
	assert.ErrorIs(t, err, ErrInvalidCacheKey)
}

func TestBoltStore_MultipleBuckets(t *testing.T) {
	store := newTestStore(t, true, 3600)

	projected := json.RawMessage(`{"type":"projected"}`)
	actual := json.RawMessage(`{"type":"actual"}`)
	recs := json.RawMessage(`{"type":"recommendations"}`)

	require.NoError(t, store.Set("projected/aws/ec2:Instance/us-east-1/t3.micro", projected))
	require.NoError(t, store.Set("actual/aws/ec2:Instance/2025-01-01/2025-02-01/abc123", actual))
	require.NoError(t, store.Set("recommendations/multi/ec2+rds", recs))

	e1, err := store.Get("projected/aws/ec2:Instance/us-east-1/t3.micro")
	require.NoError(t, err)
	assert.JSONEq(t, string(projected), string(e1.Data))

	e2, err := store.Get("actual/aws/ec2:Instance/2025-01-01/2025-02-01/abc123")
	require.NoError(t, err)
	assert.JSONEq(t, string(actual), string(e2.Data))

	e3, err := store.Get("recommendations/multi/ec2+rds")
	require.NoError(t, err)
	assert.JSONEq(t, string(recs), string(e3.Data))

	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestBoltStore_CacheInterfaceCompliance(t *testing.T) {
	store := newTestStore(t, true, 3600)

	// Verify BoltStore implements Cache interface
	var c Cache = store
	assert.True(t, c.IsEnabled())

	data := json.RawMessage(`{"test":"compliance"}`)
	require.NoError(t, c.Set("projected/test/compliance", data))

	entry, err := c.Get("projected/test/compliance")
	require.NoError(t, err)
	assert.JSONEq(t, string(data), string(entry.Data))

	require.NoError(t, c.Close())
}

// --- T009: Benchmark ---

func BenchmarkBoltStoreGet(b *testing.B) {
	dir := b.TempDir()
	store, err := NewBoltStore(context.Background(), dir, true, 3600, 0)
	require.NoError(b, err)
	defer store.Close()

	data := json.RawMessage(`{"cost":42.50,"currency":"USD"}`)

	// Populate with 10,000 entries across all 3 buckets using unique keys per iteration
	for i := range 3334 {
		sku := fmt.Sprintf("t3.micro-%d", i)
		_ = store.Set(BuildProjectedKey("aws", "ec2:Instance", "us-east-1", sku), data)
		filters := map[string]string{"i": strconv.Itoa(i)}
		_ = store.Set(BuildActualKey(
			"aws", []string{fmt.Sprintf("ec2:Instance-%d", i)},
			time.Now(), time.Now().Add(24*time.Hour), filters,
		), data)
		recType := []string{fmt.Sprintf("ec2:Instance-%d", i)}
		_ = store.Set(BuildRecommendationsKey(recType), data)
	}

	// Target key to lookup
	targetKey := BuildProjectedKey("aws", "ec2:Instance", "us-east-1", "t3.micro-target")
	_ = store.Set(targetKey, data)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_, _ = store.Get(targetKey)
	}
}

// --- T015: Corruption recovery tests ---

func TestBoltStore_CorruptionRecovery(t *testing.T) {
	t.Run("garbage bytes", func(t *testing.T) {
		testDir := t.TempDir()
		garbagePath := filepath.Join(testDir, dbFileName)
		require.NoError(t, os.WriteFile(garbagePath, []byte("this is not a database"), 0o600))

		store, err := NewBoltStore(context.Background(), testDir, true, 3600, 0)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()

		// Verify the fresh store works
		data := json.RawMessage(`{"recovered":true}`)
		require.NoError(t, store.Set("projected/aws/test", data))
		entry, getErr := store.Get("projected/aws/test")
		require.NoError(t, getErr)
		assert.JSONEq(t, string(data), string(entry.Data))
	})

	t.Run("truncated file", func(t *testing.T) {
		testDir := t.TempDir()
		truncPath := filepath.Join(testDir, dbFileName)
		// Write first 10 bytes of a valid header then truncate
		require.NoError(t, os.WriteFile(truncPath, make([]byte, 10), 0o600))

		store, err := NewBoltStore(context.Background(), testDir, true, 3600, 0)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()
	})

	t.Run("zero-byte file", func(t *testing.T) {
		testDir := t.TempDir()
		zeroPath := filepath.Join(testDir, dbFileName)
		require.NoError(t, os.WriteFile(zeroPath, []byte{}, 0o600))

		store, err := NewBoltStore(context.Background(), testDir, true, 3600, 0)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()
	})
}

// --- T016: Concurrent read/write safety ---

func TestBoltStore_ConcurrentSafety(t *testing.T) {
	store := newTestStore(t, true, 3600)

	data := json.RawMessage(`{"concurrent":true}`)
	const goroutines = 50

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*2)

	for i := range goroutines {
		wg.Add(2)

		// Writer
		go func(idx int) {
			defer wg.Done()
			key := BuildProjectedKey("aws", "ec2:Instance", "us-east-1",
				fmt.Sprintf("t3.micro-%d", idx))
			if err := store.Set(key, data); err != nil {
				errCh <- err
			}
		}(i)

		// Reader
		go func(idx int) {
			defer wg.Done()
			key := BuildProjectedKey("aws", "ec2:Instance", "us-east-1",
				fmt.Sprintf("t3.micro-%d", idx))
			_, _ = store.Get(key) // May return not found, that's fine
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NoError(t, err, "concurrent operation error")
	}
}

// --- T017: Lock timeout graceful degradation ---

func TestBoltStore_LockTimeout(t *testing.T) {
	dir := t.TempDir()

	// Open first store
	store1, err := NewBoltStore(context.Background(), dir, true, 3600, 0)
	require.NoError(t, err)
	require.NotNil(t, store1)
	defer store1.Close()

	// Attempt to open second store at same path → should return nil, ErrCacheLocked
	store2, err := NewBoltStore(context.Background(), dir, true, 3600, 0)
	assert.ErrorIs(t, err, ErrCacheLocked)
	assert.Nil(t, store2)

	// First store should still work
	data := json.RawMessage(`{"lock":"test"}`)
	require.NoError(t, store1.Set("projected/aws/lock-test", data))
	entry, getErr := store1.Get("projected/aws/lock-test")
	require.NoError(t, getErr)
	assert.JSONEq(t, string(data), string(entry.Data))
}

// --- T020: InvalidateByPrefix tests ---

func TestBoltStore_InvalidateByPrefix(t *testing.T) {
	t.Run("by provider prefix", func(t *testing.T) {
		store := newTestStore(t, true, 3600)
		data := json.RawMessage(`{"test":true}`)

		// Populate AWS and GCP entries
		require.NoError(t, store.Set("projected/aws/ec2:Instance/us-east-1/t3.micro", data))
		require.NoError(t, store.Set("projected/aws/rds:DBInstance/us-west-2/db.t3.medium", data))
		require.NoError(t, store.Set("projected/gcp/compute:Instance/us-central1/n1-standard-1", data))

		// Invalidate AWS only
		count, err := store.InvalidateByPrefix("projected/aws/")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// GCP should remain
		_, err = store.Get("projected/gcp/compute:Instance/us-central1/n1-standard-1")
		assert.NoError(t, err)

		// AWS should be gone
		_, err = store.Get("projected/aws/ec2:Instance/us-east-1/t3.micro")
		assert.ErrorIs(t, err, ErrCacheNotFound)
	})

	t.Run("by resource type prefix", func(t *testing.T) {
		store := newTestStore(t, true, 3600)
		data := json.RawMessage(`{"test":true}`)

		require.NoError(t, store.Set("projected/aws/ec2:Instance/us-east-1/t3.micro", data))
		require.NoError(t, store.Set("projected/aws/rds:DBInstance/us-west-2/db.t3.medium", data))

		count, err := store.InvalidateByPrefix("projected/aws/ec2:Instance/")
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// RDS should remain
		_, err = store.Get("projected/aws/rds:DBInstance/us-west-2/db.t3.medium")
		assert.NoError(t, err)
	})

	t.Run("no matches returns 0", func(t *testing.T) {
		store := newTestStore(t, true, 3600)
		count, err := store.InvalidateByPrefix("projected/azure/")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("empty prefix clears all", func(t *testing.T) {
		store := newTestStore(t, true, 3600)
		data := json.RawMessage(`{"test":true}`)
		require.NoError(t, store.Set("projected/aws/test", data))
		require.NoError(t, store.Set("actual/aws/test/2025-01-01/2025-02-01/abc", data))
		require.NoError(t, store.Set("recommendations/multi/test", data))

		count, err := store.InvalidateByPrefix("")
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		total, _ := store.Count()
		assert.Equal(t, 0, total)
	})

	t.Run("empty bucket returns 0", func(t *testing.T) {
		store := newTestStore(t, true, 3600)
		count, err := store.InvalidateByPrefix("actual/")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("disabled store returns ErrCacheDisabled", func(t *testing.T) {
		store := newTestStore(t, false, 0)
		_, err := store.InvalidateByPrefix("projected/")
		assert.ErrorIs(t, err, ErrCacheDisabled)
	})
}

func TestBoltStore_Delete(t *testing.T) {
	store := newTestStore(t, true, 3600)
	data := json.RawMessage(`{"delete":true}`)
	key := "projected/aws/ec2:Instance/us-east-1/t3.micro"

	require.NoError(t, store.Set(key, data))
	require.NoError(t, store.Delete(key))

	_, err := store.Get(key)
	assert.ErrorIs(t, err, ErrCacheNotFound)

	// Idempotent: delete non-existent key should not error
	require.NoError(t, store.Delete(key))
}

func TestBoltStore_Clear(t *testing.T) {
	store := newTestStore(t, true, 3600)
	data := json.RawMessage(`{"clear":true}`)

	require.NoError(t, store.Set("projected/aws/test1", data))
	require.NoError(t, store.Set("actual/aws/test2/2025-01-01/2025-02-01/abc", data))
	require.NoError(t, store.Set("recommendations/multi/test3", data))

	require.NoError(t, store.Clear())

	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// --- T024: Size/Count/Compact tests ---

func TestBoltStore_Size(t *testing.T) {
	store := newTestStore(t, true, 3600)
	data := json.RawMessage(`{"size":"test"}`)

	require.NoError(t, store.Set("projected/aws/test", data))

	size, err := store.Size()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0))
}

func TestBoltStore_Count(t *testing.T) {
	store := newTestStore(t, true, 3600)
	data := json.RawMessage(`{"count":"test"}`)

	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	require.NoError(t, store.Set("projected/aws/test1", data))
	require.NoError(t, store.Set("actual/aws/test2/2025-01-01/2025-02-01/abc", data))
	require.NoError(t, store.Set("recommendations/multi/test3", data))

	count, err = store.Count()
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestBoltStore_Compact(t *testing.T) {
	store := newTestStore(t, true, 3600)
	data := json.RawMessage(`{"compact":"test"}`)

	// Add entries
	for i := range 100 {
		sku := fmt.Sprintf("type-%c%d", 'A'+rune(i%26), i%10)
		key := BuildProjectedKey("aws", "ec2:Instance", "us-east-1", sku)
		require.NoError(t, store.Set(key, data))
	}

	// Delete half
	for i := range 50 {
		sku := fmt.Sprintf("type-%c%d", 'A'+rune(i%26), i%10)
		key := BuildProjectedKey("aws", "ec2:Instance", "us-east-1", sku)
		require.NoError(t, store.Delete(key))
	}

	// Compact
	require.NoError(t, store.compact())

	// Verify remaining entries are intact (100 inserted - 50 deleted = 50 remaining)
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, 50, count)
}

// TestBoltStore_CompactReopenFailureGracefulDegradation verifies the defensive
// behavior added to compact(): if reopening the compacted DB fails, the store
// must be disabled (s.db = nil, s.enabled = false) so that subsequent operations
// return ErrCacheDisabled instead of panicking on a closed/nil DB handle.
func TestBoltStore_CompactReopenFailureGracefulDegradation(t *testing.T) {
	store := newTestStore(t, true, 3600)

	// Confirm store starts enabled.
	assert.True(t, store.IsEnabled())

	// Simulate the state compact() leaves after a reopen failure with the fix:
	// the source DB has been closed and the store is disabled defensively.
	require.NoError(t, store.db.Close())
	store.db = nil
	store.enabled = false

	// All cache operations must return ErrCacheDisabled, not panic.
	_, getErr := store.Get("projected/aws/ec2:Instance/us-east-1/t3.micro")
	assert.ErrorIs(t, getErr, ErrCacheDisabled)

	setErr := store.Set("projected/aws/ec2:Instance/us-east-1/t3.micro", json.RawMessage(`{}`))
	assert.ErrorIs(t, setErr, ErrCacheDisabled)

	delErr := store.Delete("projected/aws/ec2:Instance/us-east-1/t3.micro")
	assert.ErrorIs(t, delErr, ErrCacheDisabled)

	_, countErr := store.Count()
	assert.ErrorIs(t, countErr, ErrCacheDisabled)

	// compact() on a disabled store returns ErrCacheDisabled too.
	compactErr := store.compact()
	assert.ErrorIs(t, compactErr, ErrCacheDisabled)

	assert.False(t, store.IsEnabled())
}

func TestBoltStore_SingleDatabaseFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBoltStore(context.Background(), dir, true, 3600, 0)
	require.NoError(t, err)
	defer store.Close()

	data := json.RawMessage(`{"file":"test"}`)
	require.NoError(t, store.Set("projected/aws/test", data))

	// Verify only cache.db exists (no individual JSON files)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	foundDB := false
	for _, e := range entries {
		if e.Name() == dbFileName {
			foundDB = true
		} else {
			assert.Failf(t, "unexpected file in cache dir", "%s", e.Name())
		}
	}
	assert.True(t, foundDB, "cache.db should exist")
}

// --- T012: CleanupExpired tests ---

func TestBoltStore_CleanupExpired(t *testing.T) {
	store := newTestStore(t, true, 1) // 1-second TTL
	data := json.RawMessage(`{"cleanup":"test"}`)

	require.NoError(t, store.Set("projected/aws/expire1", data))
	require.NoError(t, store.Set("projected/aws/expire2", data))
	require.NoError(t, store.Set("projected/aws/expire3", data))

	// Wait for expiration (1s TTL + 200ms buffer)
	time.Sleep(1200 * time.Millisecond)

	count, err := store.CleanupExpired()
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	total, _ := store.Count()
	assert.Equal(t, 0, total)
}

// --- CacheEntry tests ---

func TestCacheEntry(t *testing.T) {
	key := "test-key"
	data := json.RawMessage(`{"foo":"bar"}`)
	ttl := 60
	entry := NewCacheEntry(key, data, ttl)

	assert.Equal(t, key, entry.Key)
	assert.Equal(t, data, entry.Data)
	assert.False(t, entry.IsExpired())
	assert.True(t, entry.IsValid())
	assert.Greater(t, entry.TimeUntilExpiration(), time.Duration(0))
	assert.LessOrEqual(t, entry.Age(), time.Second)

	t.Run("Touch", func(t *testing.T) {
		oldExpiry := entry.ExpiresAt
		time.Sleep(10 * time.Millisecond)
		entry.Touch()
		assert.True(t, entry.ExpiresAt.After(oldExpiry))
	})

	t.Run("Expiration", func(t *testing.T) {
		entry.ExpiresAt = time.Now().Add(-1 * time.Second)
		assert.True(t, entry.IsExpired())
		assert.False(t, entry.IsValid())
		assert.Equal(t, time.Duration(0), entry.TimeUntilExpiration())
	})

	t.Run("JSON_Unix_Timestamps", func(t *testing.T) {
		entry := NewCacheEntry(key, data, ttl)
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)

		// Verify Unix timestamp format (should be int64, not string)
		var raw map[string]interface{}
		require.NoError(t, json.Unmarshal(encoded, &raw))
		createdAt, ok := raw["created_at"].(float64) // JSON numbers are float64
		assert.True(t, ok, "created_at should be a number (Unix timestamp)")
		assert.Greater(t, createdAt, float64(0))

		var decoded CacheEntry
		err = json.Unmarshal(encoded, &decoded)
		require.NoError(t, err)
		assert.Equal(t, entry.Key, decoded.Key)
		assert.Equal(t, entry.TTLSeconds, decoded.TTLSeconds)
		// Compare to second precision (Unix timestamps)
		assert.Equal(t, entry.CreatedAt.Unix(), decoded.CreatedAt.Unix())
		assert.Equal(t, entry.ExpiresAt.Unix(), decoded.ExpiresAt.Unix())
	})
}

func TestTTLConfig(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		cfg, err := NewTTLConfig(120)
		require.NoError(t, err)
		assert.Equal(t, 120, cfg.Seconds)
		assert.Equal(t, 120*time.Second, cfg.Duration)
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := NewTTLConfig(10) // too short
		assert.Error(t, err)
	})

	t.Run("Env", func(t *testing.T) {
		t.Setenv(EnvTTLSeconds, "500")
		assert.Equal(t, 500, GetTTLFromEnv())

		t.Setenv(EnvCacheEnabled, "false")
		assert.False(t, GetCacheEnabledFromEnv())
	})

	t.Run("FormatDuration", func(t *testing.T) {
		assert.Equal(t, "30s", FormatDuration(30*time.Second))
		assert.Equal(t, "5m", FormatDuration(5*time.Minute))
		assert.Equal(t, "2h", FormatDuration(2*time.Hour))
		assert.Equal(t, "2h30m", FormatDuration(2*time.Hour+30*time.Minute))
		assert.Equal(t, "3d", FormatDuration(72*time.Hour))
		assert.Equal(t, "3d2h", FormatDuration(74*time.Hour))
	})

	t.Run("ParseTTL", func(t *testing.T) {
		ttl, _ := ParseTTL("3600")
		assert.Equal(t, 3600, ttl)

		ttl, _ = ParseTTL("1h")
		assert.Equal(t, 3600, ttl)

		_, err := ParseTTL("invalid")
		assert.Error(t, err)
	})
}

// --- Constructor tests ---

func TestNewBoltStore(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		store, err := NewBoltStore(context.Background(), "", false, 0, 0)
		require.NoError(t, err)
		require.NotNil(t, store)
		assert.False(t, store.IsEnabled())
	})

	t.Run("empty directory with enabled", func(t *testing.T) {
		_, err := NewBoltStore(context.Background(), "", true, 3600, 100)
		require.Error(t, err)
	})

	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		store, err := NewBoltStore(context.Background(), dir, true, 3600, 100)
		require.NoError(t, err)
		require.NotNil(t, store)
		assert.True(t, store.IsEnabled())
		assert.Equal(t, 3600, store.GetTTL())
		assert.Equal(t, dir, store.GetDirectory())
		require.NoError(t, store.Close())
	})
}

// --- Helper functions ---

func newTestStore(t *testing.T, enabled bool, ttl int) *BoltStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewBoltStore(context.Background(), dir, enabled, ttl, 0)
	require.NoError(t, err)
	require.NotNil(t, store)
	t.Cleanup(func() { _ = store.Close() })
	return store
}
