# Data Model: Cache Expires-At Hints

**Branch**: `606-cache-expires-at` | **Date**: 2026-03-12

## Entity Changes

### Proto Adapter Types (`internal/proto/adapter.go`)

#### CostResult (modified)

```go
type CostResult struct {
    Currency        string
    MonthlyCost     float64
    HourlyCost      float64
    Notes           string
    CostBreakdown   map[string]float64
    Sustainability  map[string]SustainabilityMetric
    StructuredError *StructuredError `json:"structuredError,omitempty"`
    ExpiresAt       *time.Time       // NEW: Plugin caching hint (nil = no hint)
}
```

**New field**: `ExpiresAt *time.Time`

- Populated from `pbc.GetProjectedCostResponse.GetExpiresAt()` via `timestamppb.AsTime()`
- Nil when the plugin does not set `expires_at` (default proto behavior)
- Carries through to `engine.CostResult` during type mapping

#### ActualCostResult (modified)

```go
type ActualCostResult struct {
    Currency       string
    TotalCost      float64
    CostBreakdown  map[string]float64
    Sustainability map[string]SustainabilityMetric
    ExpiresAt      *time.Time  // NEW: Plugin caching hint (nil = no hint)
}
```

**New field**: `ExpiresAt *time.Time`

- Populated from the earliest `pbc.ActualCostResult.GetExpiresAt()` across
  all results in a batch response (FR-007)
- Nil when no result in the batch has an `expires_at` set

### Engine Types (`internal/engine/types.go`)

#### Engine CostResult (modified)

```go
type CostResult struct {
    // ... existing fields unchanged ...
    ExpiresAt  *time.Time `json:"expiresAt,omitempty"`  // NEW: Plugin caching hint
}
```

**New field**: `ExpiresAt *time.Time`

- JSON tag `expiresAt,omitempty` — excluded from JSON output when nil
- Mapped from `proto.CostResult.ExpiresAt` in `getProjectedCostFromPlugin`
- Mapped from `proto.ActualCostResult.ExpiresAt` in `getActualCostFromPlugin`
- Consumed by `storeProjectedCostCache` and `storeActualCostCache` for TTL calculation

### Cache Types (`internal/engine/cache/`)

#### Cache Interface (modified)

```go
type Cache interface {
    Get(key string) (*CacheEntry, error)
    Set(key string, data json.RawMessage) error
    SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error  // NEW
    IsEnabled() bool
    Close() error
    InvalidateByPrefix(prefix string) (int, error)
}
```

**New method**: `SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error`

- Stores an entry with a caller-specified TTL instead of the store default
- Used by engine when a plugin provides an `expires_at` hint
- Implementation delegates to `NewCacheEntry(key, data, ttlSeconds)` — same
  as `Set()` but with the custom TTL parameter

#### CacheEntry (unchanged)

No changes. The existing struct already supports per-entry TTLs.

### New Function (`internal/engine/cache/ttl.go`)

```go
// CalculatePluginTTL determines the cache TTL from a plugin's expires_at hint.
//
// Returns:
//   - ttlSeconds: The TTL to use (0 if skip is true)
//   - skip: true if the result should not be cached (past expiration)
//
// Behavior:
//   - nil expiresAt → returns (defaultTTL, false)
//   - past/current expiresAt → returns (0, true)
//   - future expiresAt within MaxTTLSeconds → returns (remaining seconds, false)
//   - future expiresAt exceeding MaxTTLSeconds → returns (MaxTTLSeconds, false)
func CalculatePluginTTL(expiresAt *time.Time, defaultTTL int) (ttlSeconds int, skip bool)
```

## Relationships

```text
pbc.GetProjectedCostResponse.ExpiresAt
    → proto.CostResult.ExpiresAt
    → engine.CostResult.ExpiresAt
    → cache.CalculatePluginTTL()
    → cache.SetWithTTL()
    → cache.CacheEntry (ExpiresAt, TTLSeconds)

pbc.ActualCostResult.ExpiresAt (earliest across batch)
    → proto.ActualCostResult.ExpiresAt
    → engine.CostResult.ExpiresAt
    → cache.CalculatePluginTTL()
    → cache.SetWithTTL()
    → cache.CacheEntry (ExpiresAt, TTLSeconds)
```

## Validation Rules

| Field | Rule | Enforcement |
|-------|------|-------------|
| `ExpiresAt` (proto) | Nil means no hint | Proto default behavior |
| `ExpiresAt` (engine) | Nil means use default TTL | `CalculatePluginTTL` |
| TTL from plugin hint | Capped at MaxTTLSeconds (604800) | `CalculatePluginTTL` |
| TTL from plugin hint | No minimum floor (plugins bypass MinTTLSeconds) | `CalculatePluginTTL` |
| Past `ExpiresAt` | Do not cache | `CalculatePluginTTL` returns skip=true |

## State Transitions

No state transitions. `ExpiresAt` is a read-only hint that flows from plugin
response to cache storage. It does not change after being set.
