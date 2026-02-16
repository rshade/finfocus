# Contract: Cache Interface

**Package**: `internal/engine/cache`
**Feature**: 595-boltdb-cache

## Interface Definition

```go
// Cache defines the contract for cache storage backends.
// All implementations must be safe for concurrent use by multiple goroutines.
type Cache interface {
    // Get retrieves a cache entry by key. Returns ErrCacheNotFound if the key
    // does not exist. Returns ErrCacheExpired if the entry exists but has
    // expired (the expired entry is lazily deleted). The key format is
    // "{bucket}/{provider}/{resourceType}/{...additional}".
    Get(key string) (*CacheEntry, error)

    // Set stores a cache entry. The key format determines which bucket the
    // entry is stored in. Concurrent calls are batched for efficiency.
    // Returns ErrCacheDisabled if caching is disabled, ErrInvalidCacheKey
    // if key is empty.
    Set(key string, data json.RawMessage) error

    // IsEnabled returns whether caching is active. When false, Get always
    // returns ErrCacheDisabled and Set is a no-op.
    IsEnabled() bool

    // Close releases the database file handle and flushes pending writes.
    // Must be called on shutdown. Safe to call multiple times.
    Close() error

    // InvalidateByPrefix removes all cache entries whose keys start with
    // the given prefix. Returns the count of entries removed. The prefix
    // may target a bucket (e.g., "projected/aws/") or span buckets.
    // An empty prefix clears the entire cache.
    InvalidateByPrefix(prefix string) (int, error)
}
```

## Error Sentinel Values

```go
var (
    // ErrCacheNotFound indicates the requested key does not exist.
    ErrCacheNotFound = errors.New("cache entry not found")

    // ErrCacheExpired indicates the entry existed but has expired.
    // The expired entry is deleted as a side effect.
    ErrCacheExpired = errors.New("cache entry expired")

    // ErrInvalidCacheKey indicates an empty or malformed cache key.
    ErrInvalidCacheKey = errors.New("invalid cache key")

    // ErrCacheDisabled indicates caching is not enabled.
    ErrCacheDisabled = errors.New("cache is disabled")
)
```

## Constructor

```go
// NewBoltStore creates a new BoltDB-backed cache store.
// The database file is stored at {directory}/cache.db where directory
// is typically the project's .finfocus/ directory (resolved via
// ResolveProjectDir) or ~/.finfocus/ as a global fallback.
// If enabled is false, returns a disabled store where Get returns
// ErrCacheDisabled and Set is a no-op.
// If the database file is locked by another process (timeout 500ms),
// returns nil, nil to signal graceful degradation.
// If the database file is corrupt, it is deleted and recreated.
func NewBoltStore(directory string, enabled bool, ttlSeconds, maxSizeMB int) (*BoltStore, error)
```

## Key Generation Functions

```go
// BuildProjectedKey constructs a human-readable key for per-resource
// projected cost caching. Format: projected/{provider}/{type}/{region}/{sku}
func BuildProjectedKey(provider, resourceType, region, sku string) string

// BuildActualKey constructs a key for whole-query actual cost caching.
// Format: actual/{provider}/{type}/{from}/{to}/{filter-hash}
func BuildActualKey(provider string, resourceTypes []string,
    from, to time.Time, filters map[string]string) string

// BuildRecommendationsKey constructs a key for recommendation result caching.
// Format: recommendations/multi/{sorted-types-hash}
func BuildRecommendationsKey(resourceTypes []string) string

// BucketFromKey extracts the bucket name from a structured cache key.
// Returns the first path segment before the first "/".
func BucketFromKey(key string) string
```

## Maintenance Operations

```go
// BoltStore-only methods (not on Cache interface):

// Delete removes a single cache entry by exact key.
func (s *BoltStore) Delete(key string) error

// Clear removes all entries from all buckets.
func (s *BoltStore) Clear() error

// CleanupExpired removes all expired entries across all buckets.
// Returns the number of entries removed.
func (s *BoltStore) CleanupExpired() (int, error)

// Size returns the current database file size in bytes.
func (s *BoltStore) Size() (int64, error)

// Count returns the total number of entries across all buckets.
func (s *BoltStore) Count() (int, error)

// Compact rewrites the database to reclaim free pages.
// Should be called when file size exceeds expected data size.
func (s *BoltStore) Compact() error
```
