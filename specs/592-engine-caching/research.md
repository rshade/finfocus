# Research: Unified Engine Caching System

**Branch**: `592-engine-caching` | **Date**: 2026-02-14

## R1: Cache Interface Design

**Decision**: Minimal 3-method `Cache` interface in `internal/engine/cache/store.go`.

**Rationale**: The engine only calls `Get`, `Set`, and `IsEnabled` on the cache.
Maintenance operations (`Delete`, `Clear`, `CleanupExpired`, `Size`, `Count`) stay
on `FileStore` only. A minimal interface enables mock injection for testing without
exposing unnecessary surface area.

**Alternatives considered**:

- Full interface (all FileStore methods): Rejected, violates interface segregation.
- Interface in a separate file: Rejected, Go convention keeps interfaces near
  their primary consumer; since FileStore implements it, co-locate.

## R2: Projected Cost Caching Strategy

**Decision**: Per-resource caching inside the worker goroutine, keyed by
provider + resource type + all properties.

**Rationale**: Per-resource caching allows partial cache hits when a Pulumi plan
changes (only modified resources re-query plugins). Using all properties in the
key via `cache.KeyParams.Filters` captures pricing-affecting values (instanceType,
region, storageSize) without importing the adapter's SKU/region extraction logic.

**Alternatives considered**:

- Whole-result SHA-based caching (issue #543): Rejected, any single resource
  change invalidates the entire cache. Closed #543 in favor of #600.
- SKU+region only in key: Rejected, requires adapter imports from engine and
  misses other pricing-affecting properties.

## R3: Actual Cost Caching Strategy

**Decision**: Whole-query caching before/after the worker pool, keyed by
resource types + time range + tags + adapter + groupBy.

**Rationale**: Actual cost queries are defined by the full request parameters.
Unlike projected costs (where individual resource properties vary), actual cost
queries for the same time range and filters should return identical results. The
whole-query approach is simpler and matches the existing recommendations pattern.

**Alternatives considered**:

- Per-resource caching: Possible but unnecessary complexity; actual cost queries
  typically use the same time range for all resources.

## R4: Shared `initCache()` Helper

**Decision**: Add `initCache(cmd, ctx) cache.Cache` to `common_execution.go`
returning the `Cache` interface (nil when disabled).

**Rationale**: The 43-line cache initialization block in `cost_recommendations.go`
would be duplicated verbatim in `cost_projected.go` and `cost_actual.go`. A shared
helper eliminates 86 lines of duplication. Returning `cache.Cache` (interface)
instead of `*cache.FileStore` aligns with FR-001.

**Precedence logic**: CLI flag > env var (`FINFOCUS_CACHE_TTL`) > config file >
default (0 = disabled).

## R5: TTL Constant Deduplication

**Decision**: Remove `defaultCacheTTLSeconds` from `cost_recommendations.go` (line
40) and from `config.go` (line 58). Use `cache.DefaultTTLSeconds` (3600) from the
cache package everywhere.

**Rationale**: Three duplicate definitions of the same constant (cache/ttl.go,
cli/cost_recommendations.go, config/config.go) violates DRY and risks
inconsistency if one is updated without the others.

## R6: Environment Variable Naming

**Decision**: Rename `FINFOCUS_CACHE_TTL_SECONDS` to `FINFOCUS_CACHE_TTL`.

**Rationale**: Matches the `--cache-ttl` CLI flag naming. Simpler. The unit
(seconds) is documented, not encoded in the name. Update `EnvTTLSeconds` constant
in `cache/ttl.go` and `applyEnvOverrides()` in `config/config.go`.

## R7: "(cached)" Adapter Field Marker

**Decision**: Append `(cached)` suffix to the `Adapter` field on cache hits.

**Rationale**: Users need visual feedback that results came from cache vs live
plugin calls. The Adapter field is already displayed in table output. This is a
lightweight, non-breaking change. Example: `"aws-public"` becomes
`"aws-public (cached)"`.

## R8: Worker-Level Cache Integration (Projected)

**Decision**: Cache check/store happens inside the worker goroutine, wrapping the
plugin iteration loop. On cache hit, the worker returns immediately without trying
any plugins.

**Key code locations** (from exploration):

| Function | Cache check before | Cache store after |
| -------- | ------------------ | ----------------- |
| `GetProjectedCost` | Line 313 | Line 357 |
| `GetProjectedCostWithErrors` | Line 515 | Line 552 |

**Cache key generation**: Uses existing `cache.GenerateKey(KeyParams{...})` with
resource properties converted to string filters via `fmt.Sprintf("%v", v)`.

## R9: Query-Level Cache Integration (Actual)

**Decision**: Cache check before the worker pool starts; cache store after all
results are collected.

**Key code locations**:

| Function | Cache check before | Cache store after |
| -------- | ------------------ | ----------------- |
| `GetActualCostWithOptions` | Line 692 (before jobs channel) | Line 865 (after results collected) |
| `GetActualCostWithOptionsAndErrors` | Line 925 (before jobs channel) | Line 977 (after results collected) |

**Cache key generation**: Uses `cache.GenerateKey(KeyParams{...})` with time range,
tags, adapter, and groupBy as filters. `EstimateConfidence` excluded from key.
