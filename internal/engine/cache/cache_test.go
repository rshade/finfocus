package cache

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("JSON", func(t *testing.T) {
		entry := NewCacheEntry(key, data, ttl)
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)

		var decoded CacheEntry
		err = json.Unmarshal(encoded, &decoded)
		require.NoError(t, err)
		assert.Equal(t, entry.Key, decoded.Key)
		assert.Equal(t, entry.TTLSeconds, decoded.TTLSeconds)
		// Compare times with RFC3339 precision
		assert.Equal(t, entry.CreatedAt.Format(time.RFC3339), decoded.CreatedAt.Format(time.RFC3339))
	})
}

func TestGenerateKey(t *testing.T) {
	params := KeyParams{
		Operation:     "test",
		Provider:      "aws",
		ResourceTypes: []string{"type2", "type1"},
		Filters:       map[string]string{"b": "2", "a": "1"},
		Pagination: &PaginationKeyParams{
			Limit:     10,
			SortOrder: "ASC",
		},
	}

	key1, err := GenerateKey(params)
	require.NoError(t, err)
	assert.NotEmpty(t, key1)

	// Test determinism (different order of resources/filters should produce same key)
	params2 := KeyParams{
		Operation:     "TEST ", // case and space
		Provider:      " AWS",
		ResourceTypes: []string{"type1", "type2"},
		Filters:       map[string]string{"a": "1", "b": "2"},
		Pagination: &PaginationKeyParams{
			Limit:     10,
			SortOrder: "asc",
		},
	}
	key2, err := GenerateKey(params2)
	require.NoError(t, err)
	assert.Equal(t, key1, key2)

	t.Run("SimpleKey", func(t *testing.T) {
		k := GenerateSimpleKey("op", "prov", "extra")
		assert.NotEmpty(t, k)
	})

	t.Run("QueryKey", func(t *testing.T) {
		k := GenerateKeyFromQuery("SELECT id, name FROM foo")
		assert.NotEmpty(t, k)
	})

	t.Run("Builder", func(t *testing.T) {
		builder := NewKeyParamsBuilder("op", "prov").
			WithResourceTypes("t1").
			WithFilter("f1", "v1").
			WithPagination(10, 0, "field", "asc")
		k, err := builder.Build()
		require.NoError(t, err)
		assert.NotEmpty(t, k)

		p := builder.BuildParams()
		assert.Equal(t, "op", p.Operation)
	})
}

func TestFileStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "finfocus-cache-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store, err := NewFileStore(tempDir, true, 60, 10)
	require.NoError(t, err)
	assert.True(t, store.IsEnabled())

	key := "test-key"
	data := json.RawMessage(`{"hello":"world"}`)

	t.Run("SetAndGet", func(t *testing.T) {
		err := store.Set(key, data)
		require.NoError(t, err)

		entry, err := store.Get(key)
		require.NoError(t, err)
		assert.JSONEq(t, string(data), string(entry.Data))

		count, _ := store.Count()
		assert.Equal(t, 1, count)

		size, _ := store.Size()
		assert.Greater(t, size, int64(0))
	})

	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(key)
		require.NoError(t, err)

		_, err = store.Get(key)
		assert.Equal(t, ErrCacheNotFound, err)
	})

	t.Run("Clear", func(t *testing.T) {
		_ = store.Set("k1", data)
		_ = store.Set("k2", data)
		err := store.Clear()
		require.NoError(t, err)
		count, _ := store.Count()
		assert.Equal(t, 0, count)
	})

	t.Run("Disabled", func(t *testing.T) {
		disabledStore, _ := NewFileStore("", false, 60, 10)
		assert.False(t, disabledStore.IsEnabled())
		assert.Equal(t, ErrCacheDisabled, disabledStore.Set("k", data))
		_, err := disabledStore.Get("k")
		assert.Equal(t, ErrCacheDisabled, err)
	})

	t.Run("ExpirationCleanup", func(t *testing.T) {
		shortStore, _ := NewFileStore(tempDir, true, -1, 10) // Expired immediately
		_ = shortStore.Set("expired", data)

		_, err := shortStore.Get("expired")
		assert.Equal(t, ErrCacheExpired, err)

		// Wait a bit for async delete
		time.Sleep(50 * time.Millisecond)

		err = shortStore.CleanupExpired()
		require.NoError(t, err)
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
		os.Setenv(EnvTTLSeconds, "500")
		defer os.Unsetenv(EnvTTLSeconds)
		assert.Equal(t, 500, GetTTLFromEnv())

		os.Setenv(EnvCacheEnabled, "false")
		defer os.Unsetenv(EnvCacheEnabled)
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
