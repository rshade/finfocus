# Implementation Plan: Unified Engine Caching System

**Branch**: `592-engine-caching` | **Date**: 2026-02-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/592-engine-caching/spec.md`

## Summary

Add caching to projected and actual cost commands by extracting a `Cache` interface,
creating a shared `initCache()` CLI helper, implementing per-resource caching for
projected costs and whole-query caching for actual costs, deduplicating TTL constants,
and renaming the cache env var to `FINFOCUS_CACHE_TTL`.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: `internal/engine/cache` (FileStore, KeyParams, GenerateKey)
**Storage**: File-based JSON cache at `~/.finfocus/cache/`
**Testing**: `go test` with testify (`require`/`assert`), table-driven tests
**Target Platform**: Linux, macOS, Windows (amd64, arm64)
**Project Type**: Single Go module CLI application
**Performance Goals**: Second run of 500+ resource stack completes in under 1 second
**Constraints**: Cache errors must never block cost calculations; 1-hour default TTL
**Scale/Scope**: 8 files modified, ~200 lines new code, ~80 lines removed (dedup)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is orchestration logic (cache layer
  between engine and plugins). No plugin changes required.
- [x] **Test-Driven Development**: Tests planned for all new code. 80%+ coverage
  target for cache integration, `initCache()`, and key generation.
- [x] **Cross-Platform Compatibility**: File-based caching uses `os` and
  `filepath` packages. Atomic writes via temp+rename work on all platforms.
- [x] **Documentation Integrity**: CLAUDE.md cache section will be updated.
  No new public API surfaces requiring docs/ changes.
- [x] **Protocol Stability**: No protocol buffer changes. Cache is core-only.
- [x] **Implementation Completeness**: All cache paths fully implemented (no
  stubs). Cache hit, miss, error, and disabled paths all covered.
- [x] **Quality Gates**: `make test && make lint` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Cache is
  entirely within finfocus-core.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/592-engine-caching/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── cache-interface.md
├── checklists/
│   └── requirements.md
└── tasks.md                 # Created by /speckit.tasks
```

### Source Code (files to modify)

```text
internal/engine/cache/
├── store.go                 # ADD Cache interface + compile-time check
└── ttl.go                   # RENAME EnvTTLSeconds value to FINFOCUS_CACHE_TTL

internal/engine/
└── engine.go                # CHANGE cache field type to cache.Cache
                             # ADD generateProjectedCostResourceKey()
                             # ADD generateActualCostCacheKey()
                             # ADD cache check/store in GetProjectedCost worker
                             # ADD cache check/store in GetProjectedCostWithErrors worker
                             # ADD cache check/store in GetActualCostWithOptions
                             # ADD cache check/store in GetActualCostWithOptionsAndErrors

internal/cli/
├── common_execution.go      # ADD initCache() shared helper
├── cost_recommendations.go  # REPLACE 43-line cache block with initCache()
│                            # REMOVE defaultCacheTTLSeconds, defaultCacheMaxSizeMB
├── cost_projected.go        # ADD initCache() + eng.WithCache() wiring
└── cost_actual.go           # ADD initCache() + eng.WithCache() wiring

internal/config/
└── config.go                # UPDATE env var name in applyEnvOverrides()
                             # REPLACE local defaultCacheTTLSeconds with cache.DefaultTTLSeconds
```

**Structure Decision**: Existing Go module layout. All changes are modifications
to existing files in existing packages. No new packages or directories.

## Implementation Phases

### Phase 1: Cache Interface + Shared Init (Issue #541)

**Goal**: Extract `Cache` interface, create `initCache()`, refactor recommendations.

#### 1.1 Add `Cache` interface to `internal/engine/cache/store.go`

- Add 3-method interface: `Get`, `Set`, `IsEnabled`
- Add compile-time check: `var _ Cache = (*FileStore)(nil)`
- Insert before `FileStore` struct definition (~line 24)

#### 1.2 Rename env var in `internal/engine/cache/ttl.go`

- Change `EnvTTLSeconds` value from `"FINFOCUS_CACHE_TTL_SECONDS"` to
  `"FINFOCUS_CACHE_TTL"` (line 31)

#### 1.3 Update engine cache field in `internal/engine/engine.go`

- Change `cache *cache.FileStore` to `cache cache.Cache` (line 104)
- Change `WithCache(cacheStore *cache.FileStore)` to
  `WithCache(cacheStore cache.Cache)` (line 120)

#### 1.4 Add `initCache()` to `internal/cli/common_execution.go`

- Signature: `func initCache(cmd *cobra.Command, ctx context.Context) cache.Cache`
- Returns `nil` when disabled (TTL <= 0)
- Precedence: CLI flag (`--cache-ttl`) > env var (`FINFOCUS_CACHE_TTL`) >
  config file > default (0)
- Uses `cache.DefaultTTLSeconds` for default when config provides 0 but
  enablement is requested
- Uses `cache.DefaultCacheMaxSizeMB` for max size default
- Logs WARN on invalid env var values, DEBUG on cache enablement

#### 1.5 Refactor `internal/cli/cost_recommendations.go`

- Remove `defaultCacheTTLSeconds` and `defaultCacheMaxSizeMB` constants (lines 40-41)
- Replace 43-line cache init block (lines 203-246) with:

  ```go
  cacheStore := initCache(cmd, ctx)
  eng := engine.New(clients, nil)
  if cacheStore != nil {
      eng = eng.WithCache(cacheStore)
  }
  ```

#### 1.6 Update `internal/config/config.go`

- Remove local `defaultCacheTTLSeconds` and `defaultCacheMaxSizeMB` (lines 57-59)
- Import `cache` package and use `cache.DefaultTTLSeconds` and
  `cache.DefaultCacheMaxSizeMB`
- Update `applyEnvOverrides()`: change `"FINFOCUS_CACHE_TTL_SECONDS"` to
  `"FINFOCUS_CACHE_TTL"` (line ~730)

#### 1.7 Tests for Phase 1

- `cache/store_test.go`: Interface compliance test (compile-time)
- `cli/init_cache_test.go` (new): Table-driven tests for `initCache()`:
  - TTL=0 returns nil
  - Positive TTL returns non-nil Cache
  - Env var override works
  - CLI flag overrides env var
  - Invalid env var logs warning, falls back
  - Init failure returns nil gracefully
- Verify existing `cost recommendations` caching still works

---

### Phase 2: Projected Cost Caching (Issue #600)

**Goal**: Add per-resource caching inside projected cost worker goroutines.

#### 2.1 Add `generateProjectedCostResourceKey()` to `internal/engine/engine.go`

- Signature: `func (e *Engine) generateProjectedCostResourceKey(resource ResourceDescriptor) (string, error)`
- Converts `resource.Properties` to `map[string]string` filters
- Uses `cache.GenerateKey(KeyParams{Operation: "projected_cost", Provider: resource.Provider, ResourceTypes: []string{resource.Type}, Filters: filters})`
- Returns error if resource has empty Type

#### 2.2 Add cache check/store to `GetProjectedCost` worker

- **Before plugin loop** (~line 300): Check cache with resource key
  - On hit: create `CostResult` from cached data, append `" (cached)"` to
    Adapter field, skip plugin loop
  - On miss/error: continue to plugin loop
- **After successful plugin result** (~line 357): Store result in cache
  - Marshal `CostResult` to JSON, call `cache.Set(key, data)`
  - Log WARN on store failure, don't interrupt flow

#### 2.3 Add cache check/store to `GetProjectedCostWithErrors` worker

- Same pattern as 2.2, applied to the `WithErrors` variant
- On cache hit: return `CostResult` with empty errors
- **Before plugin loop** (~line 505): Check cache
- **After successful plugin result** (~line 552): Store result

#### 2.4 Wire cache in `internal/cli/cost_projected.go`

- Add after plugin cleanup, before engine creation (~line 213):

  ```go
  cacheStore := initCache(cmd, ctx)
  eng := engine.New(clients, spec.NewLoader(specDir))
  if cacheStore != nil {
      eng = eng.WithCache(cacheStore)
  }
  ```

#### 2.5 Tests for Phase 2

- `engine_cache_test.go` (new or extend existing):
  - `TestGenerateProjectedCostResourceKey_Deterministic`
  - `TestGenerateProjectedCostResourceKey_DifferentProperties`
  - `TestGenerateProjectedCostResourceKey_EmptyType`
  - Cache hit returns result with "(cached)" marker
  - Cache miss calls plugins and stores result
  - Cache error falls through to plugins (WARN logged)

---

### Phase 3: Actual Cost Caching (Issue #542)

**Goal**: Add whole-query caching before/after the actual cost worker pool.

#### 3.1 Add `generateActualCostCacheKey()` to `internal/engine/engine.go`

- Signature: `func (e *Engine) generateActualCostCacheKey(request ActualCostRequest) (string, error)`
- Builds `KeyParams` with:
  - `Operation: "actual_cost"`
  - `Provider: "multi"`
  - `ResourceTypes`: extracted from `request.Resources`
  - `Filters`: from/to (RFC3339), adapter, groupBy, tags (prefixed `tag:`)
- `EstimateConfidence` and `FallbackEstimate` excluded from key

#### 3.2 Add cache check/store to `GetActualCostWithOptions`

- **Before worker pool** (~line 692): Check cache
  - On hit: unmarshal `[]CostResult`, mark each with `" (cached)"` in Adapter,
    return immediately
  - On miss/error: proceed to worker pool
- **After results collected** (~line 865): Store results in cache
  - Marshal `[]CostResult` to JSON

#### 3.3 Add cache check/store to `GetActualCostWithOptionsAndErrors`

- **Before worker pool** (~line 925): Check cache
  - On hit: return `CostResultWithErrors{Results: cached, Errors: nil}`
  - On miss/error: proceed
- **After results collected** (~line 977): Store `result.Results` only
  - Errors are not cached (they contained `error` interfaces)

#### 3.4 Wire cache in `internal/cli/cost_actual.go`

- Add after plugin cleanup, before engine creation (~line 204):

  ```go
  cacheStore := initCache(cmd, ctx)
  eng := engine.New(clients, nil)
  if cacheStore != nil {
      eng = eng.WithCache(cacheStore)
  }
  ```

#### 3.5 Tests for Phase 3

- `engine_cache_test.go` (extend):
  - `TestGenerateActualCostCacheKey_Deterministic`
  - `TestGenerateActualCostCacheKey_DifferentTimeRanges`
  - `TestGenerateActualCostCacheKey_DifferentTags`
  - `TestGenerateActualCostCacheKey_TagOrderIndependent`
  - `TestGenerateActualCostCacheKey_WithAdapter`
  - `TestGenerateActualCostCacheKey_WithGroupBy`
  - Cache hit returns results without calling plugins
  - Cache miss calls plugins and stores result
  - `WithErrors` returns empty errors on cache hit

---

### Phase 4: Validation

- `make test` passes (all existing + new tests)
- `make lint` passes
- Manual verification with `--debug` flag shows cache hit/miss logs
- Second run of projected cost is significantly faster

## Key Files Reference

| File | Line(s) | What |
| ---- | ------- | ---- |
| `internal/engine/cache/store.go` | 24-42 | FileStore struct, insert interface before |
| `internal/engine/cache/ttl.go` | 13, 31 | DefaultTTLSeconds, EnvTTLSeconds |
| `internal/engine/engine.go` | 104, 120 | Cache field, WithCache method |
| `internal/engine/engine.go` | 237, 468 | GetProjectedCost, WithErrors |
| `internal/engine/engine.go` | 645, 905 | GetActualCostWithOptions, WithErrors |
| `internal/engine/engine.go` | 2804 | generateRecommendationsCacheKey (template) |
| `internal/cli/common_execution.go` | EOF | Insert initCache() |
| `internal/cli/cost_recommendations.go` | 40-41, 203-246 | Constants, cache init block |
| `internal/cli/cost_projected.go` | 213-215 | Engine creation (insertion point) |
| `internal/cli/cost_actual.go` | 204-214 | Engine creation (insertion point) |
| `internal/config/config.go` | 57-59, 730 | Duplicate constants, env override |
