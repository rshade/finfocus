# Tasks: Unified Engine Caching System

**Input**: Design documents from `/specs/592-engine-caching/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must maintain
minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all
tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments
are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity),
documentation (README, docs/) MUST be updated concurrently with implementation
and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational - Cache Interface + Shared Init (Issue #541)

**Purpose**: Extract Cache interface, deduplicate constants, create shared
`initCache()` helper, and refactor recommendations to use it. This phase MUST
complete before any user story caching can be added.

**Goal**: All cost commands share a single cache initialization mechanism via
`initCache()`, the engine accepts a `Cache` interface instead of a concrete type,
and duplicate TTL constants are eliminated.

**Independent Test**: Run `cost recommendations --cache-ttl 3600` twice; second
run returns cached results. Verify `--cache-ttl 0` disables caching.

### Interface and Constant Changes

- [x] T001 [P] [US1] Add `Cache` interface (Get, Set, IsEnabled) and compile-time check `var _ Cache = (*FileStore)(nil)` to `internal/engine/cache/store.go`
- [x] T002 [P] [US1] Rename `EnvTTLSeconds` constant value from `FINFOCUS_CACHE_TTL_SECONDS` to `FINFOCUS_CACHE_TTL` in `internal/engine/cache/ttl.go` (line 31)
- [x] T003 [P] [US1] Remove duplicate `defaultCacheTTLSeconds` and `defaultCacheMaxSizeMB` constants from `internal/config/config.go` (lines 57-59); use `cache.DefaultTTLSeconds` and `cache.DefaultCacheMaxSizeMB` instead; update env var name from `FINFOCUS_CACHE_TTL_SECONDS` to `FINFOCUS_CACHE_TTL` in `applyEnvOverrides()` (line ~730)

### Engine Type Update

- [x] T004 [US1] Change engine cache field from `*cache.FileStore` to `cache.Cache` and update `WithCache()` signature to accept `cache.Cache` in `internal/engine/engine.go` (lines 104, 120)

### Shared CLI Helper

- [x] T005 [US1] Write tests for `initCache()` in `internal/cli/init_cache_test.go`: table-driven tests covering TTL=0 returns nil, positive TTL returns non-nil Cache, `FINFOCUS_CACHE_TTL` env var override, CLI flag overrides env var, invalid env var logs warning and falls back, init failure returns nil gracefully
- [x] T006 [US1] Implement `initCache(cmd *cobra.Command, ctx context.Context) cache.Cache` in `internal/cli/common_execution.go` with precedence: CLI flag > env var > config > default (0=disabled)

### Recommendations Refactor

- [x] T007 [US1] Refactor `internal/cli/cost_recommendations.go`: remove `defaultCacheTTLSeconds` and `defaultCacheMaxSizeMB` constants (lines 40-41); replace 43-line cache init block (lines 203-246) with `initCache(cmd, ctx)` call and `eng.WithCache()` wiring
- [x] T008 [US1] Verify all existing cache and recommendation tests pass; add interface compliance test to `internal/engine/cache/cache_test.go`

**Checkpoint**: `cost recommendations --cache-ttl 3600` works identically to
before. `make test && make lint` pass. No duplicate TTL constants remain.

---

## Phase 2: User Story 2 - Projected Cost Caching (Priority: P2, Issue #600)

**Goal**: Repeated projected cost queries return cached results per-resource.
Changing one resource in a plan only invalidates that resource's cache entry.

**Independent Test**: Run `cost projected --pulumi-json plan.json --cache-ttl 3600`
twice. Second run shows `(cached)` in Adapter column and completes faster.

### Tests for User Story 2 (MANDATORY - TDD Required)

- [x] T009 [P] [US2] Write tests for `generateProjectedCostResourceKey()` in `internal/engine/engine_cache_test.go`: deterministic output, different properties produce different keys, empty Type returns error, property order independence
- [x] T010 [P] [US2] Write tests for projected cost cache integration in `internal/engine/engine_cache_test.go`: cache hit returns result with `(cached)` in Adapter, cache miss calls plugins and stores result, cache store failure logs WARN and returns live result, cache disabled skips all cache operations

### Implementation for User Story 2

- [x] T011 [US2] Add `generateProjectedCostResourceKey(resource ResourceDescriptor) (string, error)` to `internal/engine/engine.go`: convert `resource.Properties` to string map filters, use `cache.GenerateKey(KeyParams{Operation: "projected_cost", ...})`
- [x] T012 [US2] Add cache check before plugin loop and cache store after successful plugin result in `GetProjectedCost` worker in `internal/engine/engine.go` (~lines 300, 357): on hit append `" (cached)"` to Adapter field, on miss/error continue to plugins, on store failure log WARN
- [x] T013 [US2] Add cache check/store to `GetProjectedCostWithErrors` worker in `internal/engine/engine.go` (~lines 505, 552): same pattern as T012, on cache hit return CostResult with empty errors
- [x] T014 [US2] Wire `initCache()` and `eng.WithCache()` in `internal/cli/cost_projected.go` after plugin cleanup (~line 213), before engine creation

**Checkpoint**: `cost projected --cache-ttl 3600` caches per-resource. Second run
returns `(cached)` results. Partial plan changes only re-query changed resources.

---

## Phase 3: User Story 3 - Actual Cost Caching (Priority: P3, Issue #542)

**Goal**: Repeated actual cost queries with the same time range, tags, adapter,
and grouping return cached results. Different parameters produce different cache
entries.

**Independent Test**: Run `cost actual --from 2025-01-01 --to 2025-01-31
--cache-ttl 3600` twice. Second run returns cached results.

### Tests for User Story 3 (MANDATORY - TDD Required)

- [x] T015 [P] [US3] Write tests for `generateActualCostCacheKey()` in `internal/engine/engine_cache_test.go`: deterministic output, different time ranges produce different keys, different tags produce different keys, tag order independence, adapter included in key, groupBy included in key, EstimateConfidence excluded from key
- [x] T016 [P] [US3] Write tests for actual cost cache integration in `internal/engine/engine_cache_test.go`: cache hit returns results with `(cached)` markers without calling plugins, cache miss calls plugins and stores result, `WithErrors` variant returns empty errors on cache hit, cache store failure logs WARN

### Implementation for User Story 3

- [x] T017 [US3] Add `generateActualCostCacheKey(request ActualCostRequest) (string, error)` to `internal/engine/engine.go`: build KeyParams with Operation "actual_cost", Provider "multi", ResourceTypes from request, Filters from time range (RFC3339), adapter, groupBy, and tags (prefixed `tag:`)
- [x] T018 [US3] Add cache check before worker pool (~line 692) and cache store after results collected (~line 865) in `GetActualCostWithOptions` in `internal/engine/engine.go`: on hit unmarshal `[]CostResult`, append `" (cached)"` to each Adapter, return immediately
- [x] T019 [US3] Add cache check before worker pool (~line 925) and cache store after results collected (~line 977) in `GetActualCostWithOptionsAndErrors` in `internal/engine/engine.go`: on hit return `CostResultWithErrors{Results: cached, Errors: nil}`, cache only `result.Results` (not errors)
- [x] T020 [US3] Wire `initCache()` and `eng.WithCache()` in `internal/cli/cost_actual.go` after plugin cleanup (~line 204), before engine creation

**Checkpoint**: `cost actual --cache-ttl 3600` caches whole-query results.
Changing time range, tags, or adapter produces new cache entries. Output format
changes reuse cached results.

---

## Phase 4: Polish and Cross-Cutting Concerns

**Purpose**: Final validation, documentation, and quality gates.

- [x] T021 Run `make test` and verify all tests pass (existing + new)
- [x] T022 Run `make lint` and fix any linting issues
- [x] T023 Run `npx markdownlint-cli` on any modified markdown files
- [x] T024 Update caching documentation in `CLAUDE.md` to reflect new `initCache()` helper, `FINFOCUS_CACHE_TTL` env var, and per-command cache wiring

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Foundational/US1)**: No dependencies - start immediately. BLOCKS
  all subsequent phases.
- **Phase 2 (US2)**: Depends on Phase 1 completion (needs Cache interface +
  initCache())
- **Phase 3 (US3)**: Depends on Phase 1 completion. Independent of Phase 2 -
  can run in parallel with US2.
- **Phase 4 (Polish)**: Depends on all user stories being complete.

### Within Phase 1 (Foundational)

```text
T001 ─┐
T002 ─┼─→ T004 ─→ T005 ─→ T006 ─→ T007 ─→ T008
T003 ─┘
```

- T001, T002, T003: Parallel (different files)
- T004: Depends on T001 (needs Cache interface for type change)
- T005: Depends on T004 (needs interface for test signatures)
- T006: Depends on T005 (TDD: tests before implementation)
- T007: Depends on T006 (needs initCache() to refactor)
- T008: Depends on T007 (final verification)

### Within Phase 2 (US2)

```text
T009 ─┐
      ├─→ T011 ─→ T012 ─→ T013 ─→ T014
T010 ─┘
```

- T009, T010: Parallel (test files, TDD first)
- T011: Depends on T009 (implement after key gen tests)
- T012: Depends on T010, T011 (implement after integration tests + key gen)
- T013: Depends on T012 (same file, same pattern)
- T014: Depends on T013 (CLI wiring after engine changes)

### Within Phase 3 (US3)

```text
T015 ─┐
      ├─→ T017 ─→ T018 ─→ T019 ─→ T020
T016 ─┘
```

- T015, T016: Parallel (test files, TDD first)
- T017: Depends on T015 (implement after key gen tests)
- T018: Depends on T016, T017 (implement after integration tests + key gen)
- T019: Depends on T018 (same file, same pattern)
- T020: Depends on T019 (CLI wiring after engine changes)

### Parallel Opportunities

- T001 + T002 + T003: Three files in parallel
- T009 + T010: Test files in parallel
- T015 + T016: Test files in parallel
- Phase 2 + Phase 3: Can run in parallel after Phase 1 (if two developers)

---

## Implementation Strategy

### MVP First (Phase 1 + Phase 2)

1. Complete Phase 1: Cache interface + initCache() + refactor recommendations
2. Complete Phase 2: Projected cost caching (primary demo value)
3. **STOP and VALIDATE**: `make test && make lint`; demo 500+ resource stack
4. Ship as first PR if desired

### Full Delivery

1. Phase 1 → Phase 2 → Phase 3 → Phase 4
2. Each phase independently testable
3. Total: 24 tasks across 4 phases

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [US1/US2/US3] labels map to spec.md user stories
- All engine cache changes (T011-T013, T017-T019) touch `engine.go` - cannot
  be parallel within the same phase
- Tests MUST be written before implementation (TDD per Constitution Principle II)
- `make test && make lint` at every checkpoint
