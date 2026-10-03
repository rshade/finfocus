package cache

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

const defaultLRUMaxItems = 256

// TieredStore is an optional in-memory LRU in front of another Cache.
// Reads check memory first and promote a disk hit. Writes update both.
// The inner store remains the source of truth across processes.
type TieredStore struct {
	inner Cache
	mu    sync.Mutex
	items *lru.Cache[string, *CacheEntry]
}

// NewTieredStore wraps inner with an LRU of maxItems. A non-positive maxItems
// uses the default capacity. A nil inner store is rejected.
func NewTieredStore(inner Cache, maxItems int) (*TieredStore, error) {
	if inner == nil {
		return nil, errors.New("cache tier requires an inner store")
	}
	if maxItems <= 0 {
		maxItems = defaultLRUMaxItems
	}
	items, err := lru.New[string, *CacheEntry](maxItems)
	if err != nil {
		return nil, err
	}
	return &TieredStore{inner: inner, items: items}, nil
}

// Get returns a memory hit when it is still valid. Otherwise it reads the
// inner store and promotes that entry.
func (s *TieredStore) Get(key string) (*CacheEntry, error) {
	if entry, ok := s.memoryGet(key); ok {
		return cloneEntry(entry), nil
	}
	entry, err := s.inner.Get(key)
	if err != nil {
		return nil, err
	}
	s.memoryPut(entry)
	return cloneEntry(entry), nil
}

// Set writes through to the inner store, then refreshes the memory entry
// from that store so the TTL matches what was persisted.
func (s *TieredStore) Set(key string, data json.RawMessage) error {
	if err := s.inner.Set(key, data); err != nil {
		s.memoryDrop(key)
		return err
	}
	s.remember(key)
	return nil
}

// SetWithTTL writes through with the caller-specified TTL, then refreshes memory.
func (s *TieredStore) SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error {
	if err := s.inner.SetWithTTL(key, data, ttlSeconds); err != nil {
		s.memoryDrop(key)
		return err
	}
	s.remember(key)
	return nil
}

// IsEnabled reports the inner store's enabled state.
func (s *TieredStore) IsEnabled() bool {
	return s.inner.IsEnabled()
}

// Close closes the inner store and drops memory entries.
func (s *TieredStore) Close() error {
	err := s.inner.Close()
	s.mu.Lock()
	s.items.Purge()
	s.mu.Unlock()
	return err
}

// InvalidateByPrefix removes matching entries from memory and the inner store.
// An empty prefix clears both.
func (s *TieredStore) InvalidateByPrefix(prefix string) (int, error) {
	count, err := s.inner.InvalidateByPrefix(prefix)
	s.dropPrefix(prefix)
	return count, err
}

func (s *TieredStore) remember(key string) {
	entry, err := s.inner.Get(key)
	if err != nil {
		s.memoryDrop(key)
		return
	}
	s.memoryPut(entry)
}

func (s *TieredStore) memoryGet(key string) (*CacheEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.items.Get(key)
	if !ok {
		return nil, false
	}
	if entry == nil || entry.IsExpired() {
		s.items.Remove(key)
		return nil, false
	}
	return entry, true
}

func (s *TieredStore) memoryPut(entry *CacheEntry) {
	if entry == nil || entry.Key == "" {
		return
	}
	s.mu.Lock()
	s.items.Add(entry.Key, cloneEntry(entry))
	s.mu.Unlock()
}

func (s *TieredStore) memoryDrop(key string) {
	s.mu.Lock()
	s.items.Remove(key)
	s.mu.Unlock()
}

func (s *TieredStore) dropPrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prefix == "" {
		s.items.Purge()
		return
	}
	for _, key := range s.items.Keys() {
		if strings.HasPrefix(key, prefix) {
			s.items.Remove(key)
		}
	}
}

func cloneEntry(entry *CacheEntry) *CacheEntry {
	if entry == nil {
		return nil
	}
	cloned := *entry
	cloned.Data = append(json.RawMessage(nil), entry.Data...)
	return &cloned
}

// Compile-time check that TieredStore implements Cache.
var _ Cache = (*TieredStore)(nil)
