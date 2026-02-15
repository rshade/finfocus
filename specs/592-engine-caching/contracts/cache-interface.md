# Contract: Cache Interface

**Package**: `internal/engine/cache`

## Interface Definition

```go
// Cache defines the interface for cache operations used by the engine.
// This interface intentionally excludes maintenance operations (Delete, Clear,
// CleanupExpired) which remain on FileStore only.
type Cache interface {
    Get(key string) (*CacheEntry, error)
    Set(key string, data json.RawMessage) error
    IsEnabled() bool
}
```

## Compile-Time Check

```go
var _ Cache = (*FileStore)(nil)
```

## Error Contract

| Method | Error | Meaning |
| ------ | ----- | ------- |
| Get | `ErrCacheNotFound` | Key does not exist |
| Get | `ErrCacheExpired` | Entry exists but TTL exceeded |
| Get | `ErrCacheDisabled` | Cache is disabled |
| Get | `ErrInvalidCacheKey` | Empty key provided |
| Set | `ErrCacheDisabled` | Cache is disabled |
| Set | `ErrInvalidCacheKey` | Empty key provided |
| IsEnabled | (never errors) | Returns false when disabled |

## Engine Integration

```go
// Engine.WithCache accepts the interface, not the concrete type.
func (e *Engine) WithCache(cacheStore cache.Cache) *Engine
```

## Mock Contract (for tests)

Test implementations MUST:

- Return `ErrCacheNotFound` for unknown keys (not nil, nil)
- Return `ErrCacheDisabled` when disabled
- Accept any valid key/data pair in Set without error
- Be safe for concurrent use
