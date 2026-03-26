# Research Notes: Cache Expires-At Hints

**Branch**: `606-cache-expires-at` | **Date**: 2026-03-12

## R1: Proto Field Availability

**Decision**: finfocus-spec v0.5.7 provides both `expires_at` fields.

**Rationale**: Verified in the proto-generated Go code:

- `GetProjectedCostResponse.ExpiresAt` — field 13, `*timestamppb.Timestamp`
- `ActualCostResult.ExpiresAt` — field 8, `*timestamppb.Timestamp`

Both have `GetExpiresAt()` accessor methods that return nil when unset.

**Alternatives considered**: None — the proto fields are the prerequisite.

## R2: Cache Entry Structure Compatibility

**Decision**: No structural changes to `CacheEntry` are needed.

**Rationale**: The existing `CacheEntry` struct already has:

- `ExpiresAt time.Time` — stores the absolute expiration timestamp
- `TTLSeconds int` — stores the TTL for reference
- `NewCacheEntry(key, data, ttlSeconds)` — calculates `ExpiresAt = now + TTL`

The BoltDB wire format (`cacheEntryJSON`) stores times as Unix timestamps.
A custom TTL simply produces a different `ExpiresAt` value — no format change.

**Alternatives considered**: Adding a `Source` field to track whether TTL
came from plugin vs default. Rejected as unnecessary — debug logging at the
store call site provides the same observability.

## R3: Cache Interface Extension Pattern

**Decision**: Add `SetWithTTL` method to `Cache` interface.

**Rationale**: The current interface is:

```go
type Cache interface {
    Get(key string) (*CacheEntry, error)
    Set(key string, data json.RawMessage) error
    IsEnabled() bool
    Close() error
    InvalidateByPrefix(prefix string) (int, error)
}
```

Adding `SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error`
preserves backward compatibility. Existing `Set()` continues to use the
store-level default TTL.

**Alternatives considered**:

1. **Change `Set()` signature**: Would require updating all callers and all
   mock implementations (7+ test files). Higher blast radius for no benefit.
2. **Functional options (`Set(key, data, ...Option)`)**: Over-engineered for
   a single optional parameter. Go convention favors explicit methods.
3. **Context-based TTL**: Embedding TTL in context is an anti-pattern —
   makes the API implicit and harder to test.

## R4: TTL Calculation Edge Cases

**Decision**: Centralize TTL calculation in `CalculatePluginTTL()`.

**Rationale**: Multiple edge cases need consistent handling:

| Input | Behavior | Source |
|-------|----------|--------|
| `nil` ExpiresAt | Return default TTL, skip=false | FR-004 |
| Future timestamp | Return remaining seconds, skip=false | FR-003 |
| Past timestamp | Return 0, skip=true | FR-005 |
| Current time (now) | Return 0, skip=true | FR-005 (edge case) |
| Exceeds MaxTTLSeconds (7d) | Return MaxTTLSeconds, skip=false + log warning | FR-006 |
| Very short (< 60s) | Honor it (no minimum for plugin hints) | Spec edge case |

**Alternatives considered**: Inline TTL calculation at each store call site.
Rejected — would duplicate logic in `storeProjectedCostCache` and
`storeActualCostCache`.

## R5: Batch Actual Cost Aggregation

**Decision**: Use earliest `expires_at` across all `ActualCostResult` entries.

**Rationale**: The actual cost adapter (`clientAdapter.GetActualCost`) aggregates
multiple `pbc.ActualCostResult` entries per resource ID into a single
`proto.ActualCostResult`. The cache stores this aggregated response as one entry.

Using the shortest TTL ensures no individual result's data exceeds its intended
freshness. This is conservative but correct — if one data source says "refresh
in 30 minutes" and another says "good for 24 hours," the 30-minute hint
should dominate.

The aggregation happens in the adapter's `GetActualCost` method, where we
already iterate over `resp.GetResults()`. We extract the earliest `ExpiresAt`
during that iteration.

**Alternatives considered**:

1. **Use latest (longest) TTL**: Could serve stale data for the short-lived
   result. Rejected.
2. **Per-result caching**: Would require redesigning the actual cost cache
   from per-request to per-result granularity. Out of scope.

## R6: Timestamp Conversion Pattern

**Decision**: Use `timestamppb.Timestamp.AsTime()` for conversion.

**Rationale**: The proto-generated `GetExpiresAt()` returns `*timestamppb.Timestamp`.
The standard conversion is:

```go
if ts := resp.GetExpiresAt(); ts != nil {
    t := ts.AsTime()
    result.ExpiresAt = &t
}
```

This handles nil naturally (no conversion when unset) and produces a
`time.Time` in UTC. The `CalculatePluginTTL` function then uses
`time.Until()` to calculate remaining seconds.

**Alternatives considered**: Storing the raw `timestamppb.Timestamp` in
internal types. Rejected — internal types should not depend on protobuf types.

## R7: Debug Logging Strategy

**Decision**: Log at debug level when plugin TTL differs from default; log
at warn level when TTL is capped at maximum.

**Rationale**: Per FR-008, TTL override decisions should be observable. The
logging points are:

1. **`CalculatePluginTTL` returns skip=true**: Debug log "caching skipped:
   plugin expires_at is in the past"
2. **Plugin TTL differs from default**: Debug log "using plugin TTL hint"
   with both values
3. **TTL exceeds maximum**: Warn log "plugin TTL capped at maximum" with
   original and capped values

Logging happens in the engine's store functions (not in the cache package)
because the engine has access to resource context (type, ID, plugin name).

**Alternatives considered**: Logging inside `CalculatePluginTTL`. Rejected —
the function is a pure calculation; logging context (resource type, plugin
name) lives in the caller.
