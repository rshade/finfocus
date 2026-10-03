package cache

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTieredStore_PromotesAndServesMemory(t *testing.T) {
	t.Parallel()
	inner := newMemoryCache()
	require.NoError(t, inner.SetWithTTL("projected/a", json.RawMessage(`"disk"`), 60))
	tier, err := NewTieredStore(inner, 4)
	require.NoError(t, err)

	first, err := tier.Get("projected/a")
	require.NoError(t, err)
	assert.JSONEq(t, `"disk"`, string(first.Data))
	assert.Equal(t, 1, inner.gets)

	second, err := tier.Get("projected/a")
	require.NoError(t, err)
	assert.JSONEq(t, `"disk"`, string(second.Data))
	assert.Equal(t, 1, inner.gets)
}

func TestTieredStore_WriteThroughSkipsLaterDiskRead(t *testing.T) {
	t.Parallel()
	inner := newMemoryCache()
	tier, err := NewTieredStore(inner, 4)
	require.NoError(t, err)

	require.NoError(t, tier.SetWithTTL("projected/a", json.RawMessage(`"fresh"`), 60))
	assert.Equal(t, 1, inner.gets)
	got, err := tier.Get("projected/a")
	require.NoError(t, err)
	assert.JSONEq(t, `"fresh"`, string(got.Data))
	assert.Equal(t, 1, inner.gets)
	_, err = inner.Get("projected/a")
	require.NoError(t, err)
}

func TestTieredStore_EvictsOldestMemoryEntry(t *testing.T) {
	t.Parallel()
	inner := newMemoryCache()
	tier, err := NewTieredStore(inner, 2)
	require.NoError(t, err)
	require.NoError(t, tier.Set("projected/a", json.RawMessage(`"a"`)))
	require.NoError(t, tier.Set("projected/b", json.RawMessage(`"b"`)))
	require.NoError(t, tier.Set("projected/c", json.RawMessage(`"c"`)))
	before := inner.gets
	_, err = tier.Get("projected/a")
	require.NoError(t, err)
	assert.Equal(t, before+1, inner.gets)
	_, err = tier.Get("projected/c")
	require.NoError(t, err)
	assert.Equal(t, before+1, inner.gets)
}

func TestTieredStore_ExpiredMemoryFallsThrough(t *testing.T) {
	t.Parallel()
	inner := newMemoryCache()
	require.NoError(t, inner.SetWithTTL("projected/a", json.RawMessage(`"new"`), 60))
	tier, err := NewTieredStore(inner, 4)
	require.NoError(t, err)
	tier.memoryPut(&CacheEntry{
		Key:       "projected/a",
		Data:      json.RawMessage(`"stale"`),
		ExpiresAt: time.Now().Add(-time.Second),
	})
	got, err := tier.Get("projected/a")
	require.NoError(t, err)
	assert.JSONEq(t, `"new"`, string(got.Data))
	assert.Equal(t, 1, inner.gets)
}

func TestTieredStore_InvalidateDropsMemory(t *testing.T) {
	t.Parallel()
	inner := newMemoryCache()
	tier, err := NewTieredStore(inner, 4)
	require.NoError(t, err)
	require.NoError(t, tier.Set("projected/a", json.RawMessage(`"a"`)))
	require.NoError(t, tier.Set("actual/b", json.RawMessage(`"b"`)))
	removed, err := tier.InvalidateByPrefix("projected/")
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	_, err = tier.Get("projected/a")
	require.ErrorIs(t, err, ErrCacheNotFound)
	before := inner.gets
	got, err := tier.Get("actual/b")
	require.NoError(t, err)
	assert.JSONEq(t, `"b"`, string(got.Data))
	assert.Equal(t, before, inner.gets)
}

func TestTieredStore_SetFailureDropsMemory(t *testing.T) {
	t.Parallel()
	inner := newMemoryCache()
	tier, err := NewTieredStore(inner, 4)
	require.NoError(t, err)
	require.NoError(t, tier.Set("projected/a", json.RawMessage(`"old"`)))
	inner.failSet = errors.New("disk full")
	err = tier.Set("projected/a", json.RawMessage(`"new"`))
	require.ErrorContains(t, err, "disk full")
	got, err := tier.Get("projected/a")
	require.NoError(t, err)
	assert.JSONEq(t, `"old"`, string(got.Data))
}

func BenchmarkTieredStore_MemoryHit(b *testing.B) {
	inner := newMemoryCache()
	tier, err := NewTieredStore(inner, 16)
	require.NoError(b, err)
	require.NoError(b, tier.Set("projected/a", json.RawMessage(`"v"`)))
	b.ResetTimer()
	for b.Loop() {
		_, getErr := tier.Get("projected/a")
		if getErr != nil {
			b.Fatal(getErr)
		}
	}
}

func TestTieredStore_DefaultCapacityAndNilInner(t *testing.T) {
	t.Parallel()
	_, err := NewTieredStore(nil, 4)
	require.Error(t, err)
	inner := newMemoryCache()
	tier, err := NewTieredStore(inner, 0)
	require.NoError(t, err)
	require.NoError(t, tier.Set("projected/a", json.RawMessage(`"a"`)))
	_, err = tier.Get("projected/a")
	require.NoError(t, err)
	assert.Equal(t, 1, inner.gets)
	require.NoError(t, tier.Close())
	assert.True(t, inner.closed)
}

type memoryCache struct {
	mu      sync.Mutex
	items   map[string]*CacheEntry
	gets    int
	closed  bool
	failSet error
}

func newMemoryCache() *memoryCache {
	return &memoryCache{items: map[string]*CacheEntry{}}
}

func (m *memoryCache) Get(key string) (*CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gets++
	entry, ok := m.items[key]
	if !ok {
		return nil, ErrCacheNotFound
	}
	if entry.IsExpired() {
		delete(m.items, key)
		return nil, ErrCacheExpired
	}
	return cloneEntry(entry), nil
}

func (m *memoryCache) Set(key string, data json.RawMessage) error {
	return m.SetWithTTL(key, data, 60)
}

func (m *memoryCache) SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failSet != nil {
		return m.failSet
	}
	m.items[key] = NewCacheEntry(key, data, ttlSeconds)
	return nil
}

func (m *memoryCache) IsEnabled() bool { return true }

func (m *memoryCache) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}

func (m *memoryCache) InvalidateByPrefix(prefix string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for key := range m.items {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			delete(m.items, key)
			removed++
		}
	}
	return removed, nil
}
