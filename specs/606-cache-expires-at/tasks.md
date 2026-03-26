# Tasks: Cache Expires-At Hints

**Input**: Design documents from `/specs/606-cache-expires-at/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Go Test Path Conventions

Unit tests for Go projects MUST be colocated with source code, not placed in `test/unit/`.

- **Unit tests**: `internal/[package]/[name]_test.go` (colocated with source)
  - Black-box (public API): `package foo_test`
  - White-box (unexported access): `package foo`
  - Run with: `go test ./internal/...`
- **Integration tests**: `test/integration/` (cross-component, requires running plugins)
  - Run with: `go test ./test/integration/...`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extend the cache interface and TTL calculation to support plugin-provided TTLs.
These are foundational building blocks that all user stories depend on.

### Tests for Phase 1 (TDD Required)

- [X] T001 [P] Write table-driven tests for `CalculatePluginTTL` in `internal/engine/cache/ttl_test.go` covering: nil expiresAt returns default, future timestamp returns remaining seconds, past timestamp returns skip=true, current time returns skip=true, timestamp exceeding MaxTTLSeconds returns capped value, very short TTL (< 60s) is honored (no minimum for plugin hints)
- [X] T002 [P] Write tests for `SetWithTTL` method in `internal/engine/cache/cache_test.go` covering: stores entry with custom TTL, entry expires at correct time, validates key is non-empty, returns error when cache disabled

### Implementation for Phase 1

- [X] T003 [P] Add `CalculatePluginTTL(expiresAt *time.Time, defaultTTL int) (ttlSeconds int, skip bool)` function in `internal/engine/cache/ttl.go`. Handles: nil → default, past → skip, future → remaining seconds, exceeds max → cap at MaxTTLSeconds (604800). Plugin hints bypass MinTTLSeconds
- [X] T004 [P] Add `SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error` method to `Cache` interface and `BoltStore` implementation in `internal/engine/cache/store.go`. Implementation is identical to `Set()` but uses the caller-provided `ttlSeconds` instead of `s.ttlSeconds`
- [X] T005 Add `SetWithTTL` method to `mockCache` in `internal/engine/engine_cache_test.go` to satisfy the updated `Cache` interface. Store the TTL in a new `lastTTL int` field for test assertions

**Checkpoint**: Cache layer now supports custom TTLs. All Phase 1 tests pass.

---

## Phase 2: User Story 1 - Plugin-Directed Cache Lifetime (Priority: P1)

**Goal**: Plumb `expires_at` from plugin gRPC responses through adapter and engine types to the cache store, so cache entries use plugin-provided TTLs.

**Independent Test**: Run a projected cost query with a mock plugin that returns `expires_at` 24 hours in the future. Verify the cached entry has a ~24h TTL. Query again within 24h and confirm cache hit with default TTL behavior unchanged when `expires_at` is nil.

### Tests for User Story 1 (TDD Required)

- [X] T006 [P] [US1] Write tests for `ExpiresAt` extraction in `clientAdapter.GetProjectedCost` in `internal/proto/adapter_test.go`: verify `CostResult.ExpiresAt` is populated when proto response has `expires_at`, verify nil when unset
- [X] T007 [P] [US1] Write tests for `ExpiresAt` extraction in `clientAdapter.GetActualCost` in `internal/proto/adapter_test.go`: verify `ActualCostResult.ExpiresAt` is populated from earliest `expires_at` across batch results, verify nil when no results have `expires_at`
- [X] T008 [P] [US1] Write tests for `storeProjectedCostCache` TTL override in `internal/engine/engine_cache_test.go`: verify `SetWithTTL` is called with plugin TTL when `ExpiresAt` is set, verify `Set` is called with default TTL when `ExpiresAt` is nil
- [X] T009 [P] [US1] Write tests for `storeActualCostCache` TTL override in `internal/engine/engine_cache_test.go`: verify `SetWithTTL` is called with earliest TTL from batch results, verify `Set` is called with default when no `ExpiresAt`

### Implementation for User Story 1

- [X] T010 [P] [US1] Add `ExpiresAt *time.Time` field to `proto.CostResult` struct in `internal/proto/adapter.go` (line ~479)
- [X] T011 [US1] Add `ExpiresAt *time.Time` field to `proto.ActualCostResult` struct in `internal/proto/adapter.go` (line ~522). Execute after T010 (same file)
- [X] T012 [P] [US1] Add `ExpiresAt *time.Time` field with JSON tag `json:"expiresAt,omitempty"` to `engine.CostResult` struct in `internal/engine/types.go` (line ~244, after Confidence field)
- [X] T013 [US1] Extract `expires_at` from proto response in `clientAdapter.GetProjectedCost` in `internal/proto/adapter.go` (line ~1078): after building `CostResult`, check `resp.GetExpiresAt()` and convert via `timestamppb.AsTime()` to set `result.ExpiresAt`
- [X] T014 [US1] Extract earliest `expires_at` from batch results in `clientAdapter.GetActualCost` in `internal/proto/adapter.go` (line ~1168): iterate `resp.GetResults()`, track earliest non-nil `GetExpiresAt()`, set on `ActualCostResult.ExpiresAt`
- [X] T015 [US1] Map `proto.CostResult.ExpiresAt` to `engine.CostResult.ExpiresAt` in `Engine.getProjectedCostFromPlugin` in `internal/engine/engine.go` (line ~1458): add `engineResult.ExpiresAt = result.ExpiresAt` after existing field mapping
- [X] T016 [US1] Map `proto.ActualCostResult.ExpiresAt` to `engine.CostResult.ExpiresAt` in `Engine.getActualCostFromPlugin` in `internal/engine/engine.go` (line ~1634): add `ExpiresAt` to the returned `CostResult` literal
- [X] T017 [US1] Update `Engine.storeProjectedCostCache` in `internal/engine/engine.go` (line ~3620): extract `ExpiresAt` from first result, call `cache.CalculatePluginTTL`. If `skip=true`, return early without caching. Otherwise use `SetWithTTL` when plugin TTL is available or `Set` for default
- [X] T018 [US1] Update `Engine.storeActualCostCache` in `internal/engine/engine.go` (line ~3694): find earliest `ExpiresAt` across results, call `cache.CalculatePluginTTL`. If `skip=true`, return early without caching. Otherwise use `SetWithTTL` when plugin TTL is available or `Set` for default

**Checkpoint**: Plugin-provided `expires_at` now controls cache TTL for both projected and actual costs. Default behavior unchanged when `expires_at` is nil.

---

## Phase 3: User Story 2 - Stale Data Prevention (Priority: P2)

**Goal**: Validate that past/current `expires_at` timestamps skip caching entirely, preventing stale data from being served.

**Note**: The skip-cache logic is implemented in T017/T018 (US1) as part of the complete `CalculatePluginTTL` integration — Constitution Principle VI requires those tasks to fully handle all return values including `skip=true`. This phase adds dedicated test coverage to verify the skip behavior independently.

**Independent Test**: Simulate a plugin response with `expires_at` set to a past timestamp. Verify the result is NOT stored in cache. Query again and confirm a fresh plugin call is made (no cache hit).

### Tests for User Story 2 (TDD Required)

- [X] T019 [P] [US2] Write tests for skip-caching behavior in `internal/engine/engine_cache_test.go`: verify that when `CostResult.ExpiresAt` is in the past, neither `Set` nor `SetWithTTL` is called for projected costs
- [X] T020 [P] [US2] Write tests for skip-caching behavior in `internal/engine/engine_cache_test.go`: verify that when all results have past `ExpiresAt`, neither `Set` nor `SetWithTTL` is called for actual costs
- [X] T021 [P] [US2] Write test for cache-disabled edge case in `internal/engine/engine_cache_test.go`: verify that when cache is disabled, `expires_at` hints are ignored entirely and no caching occurs (EC-005 from spec edge cases)

**Checkpoint**: Skip-caching behavior is thoroughly validated for both projected and actual costs.

---

## Phase 4: User Story 3 - Transparent Behavior (Priority: P3)

**Goal**: Log TTL override decisions at debug level when plugin-provided TTL differs from default, and log warnings when TTL is capped.

**Independent Test**: Enable debug logging, run a cost query with plugin `expires_at` hint. Verify debug log message records the plugin TTL and how it differs from default.

### Tests for User Story 3 (TDD Required)

- [X] T023 [P] [US3] Write tests for debug logging in `internal/engine/engine_cache_test.go`: verify that when plugin TTL differs from default, a debug log message is emitted with plugin TTL, default TTL, resource type, and plugin name
- [X] T024 [P] [US3] Write tests for skip-cache logging in `internal/engine/engine_cache_test.go`: verify that when caching is skipped (past `expires_at`), a debug log explains the skip reason
- [X] T025 [P] [US3] Write tests for max-cap logging: verify that when `CalculatePluginTTL` caps TTL at MaxTTLSeconds, a warning log is emitted with the original and capped values

### Implementation for User Story 3

- [X] T026 [US3] Add debug logging to `Engine.storeProjectedCostCache` in `internal/engine/engine.go`: log at debug level when plugin TTL differs from default (include plugin name, resource type, plugin TTL, default TTL); log at debug level when caching is skipped due to past `expires_at`
- [X] T027 [US3] Add debug logging to `Engine.storeActualCostCache` in `internal/engine/engine.go`: same logging pattern as projected cost — debug for TTL override, debug for skip
- [X] T028 [US3] Add warning logging to `Engine.storeProjectedCostCache` and `Engine.storeActualCostCache` in `internal/engine/engine.go`: log at warn level when plugin TTL was capped. Detect by comparing `ttlSeconds == cache.MaxTTLSeconds` after `CalculatePluginTTL` returns (do NOT change `CalculatePluginTTL` signature — T003's `(int, bool)` return is final)

**Checkpoint**: All TTL decisions are observable via debug/warn logging.

---

## Phase 5: Polish and Cross-Cutting Concerns

**Purpose**: Ensure quality gates pass and documentation is updated.

- [X] T029 [P] Update CLAUDE.md engine cache section to document that plugin-provided `expires_at` hints control per-entry TTL, with skip-caching for past timestamps and MaxTTLSeconds cap
- [X] T030 [P] Add `BenchmarkSetWithTTL` in `internal/engine/cache/cache_test.go` to validate SC-005 (cache store operations remain under 10ms). Benchmark should store and retrieve entries with custom TTLs
- [X] T031 Run `make test` and verify all tests pass with 80%+ coverage on changed files
- [X] T032 Run `make lint` and fix any linting issues
- [X] T033 Run quickstart.md validation: verify debug logging examples in `specs/606-cache-expires-at/quickstart.md` match actual log output format

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — can start immediately
- **Phase 2 (US1)**: Depends on Phase 1 completion (needs `SetWithTTL` and `CalculatePluginTTL`)
- **Phase 3 (US2)**: Depends on Phase 2 completion (skip logic builds on TTL calculation path)
- **Phase 4 (US3)**: Depends on Phase 2 completion (logging wraps the TTL override path)
- **Phase 5 (Polish)**: Depends on Phases 2-4 completion

### User Story Dependencies

- **US1 (P1)**: Requires Phase 1 — no dependencies on other stories. Implements full `CalculatePluginTTL` integration including skip behavior (Principle VI)
- **US2 (P2)**: Requires US1 completion (tests validate skip logic implemented in T017/T018). Test-only phase — can run in parallel with US3
- **US3 (P3)**: Requires US1 (logging wraps the TTL override logic US1 introduces). Can run in parallel with US2

### Within Each Story

- Tests MUST be written first and FAIL before implementation
- Type changes (fields) before extraction logic
- Extraction logic before mapping logic
- Mapping logic before storage logic

### Parallel Opportunities

- T001, T002 can run in parallel (different test files)
- T003, T004 can run in parallel (different functions in different files)
- T006, T007, T008, T009 can all run in parallel (different test files)
- T010, T012 can run in parallel (different files); T011 follows T010 (same file)
- T019, T020 can run in parallel (different test scenarios)
- T023, T024, T025 can run in parallel (different test scenarios)
- US2 and US3 can run in parallel after US1 completes

---

## Parallel Example: Phase 1

```text
# Launch tests in parallel:
Task T001: "CalculatePluginTTL tests in internal/engine/cache/ttl_test.go"
Task T002: "SetWithTTL tests in internal/engine/cache/cache_test.go"

# Launch implementations in parallel:
Task T003: "CalculatePluginTTL in internal/engine/cache/ttl.go"
Task T004: "SetWithTTL in internal/engine/cache/store.go"
```

## Parallel Example: User Story 1 Type Changes

```text
# Launch type additions on different files in parallel:
Task T010: "ExpiresAt field on proto.CostResult in internal/proto/adapter.go"
Task T012: "ExpiresAt field on engine.CostResult in internal/engine/types.go"

# Then T011 sequentially (same file as T010):
Task T011: "ExpiresAt field on proto.ActualCostResult in internal/proto/adapter.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Cache infrastructure (SetWithTTL + CalculatePluginTTL)
2. Complete Phase 2: User Story 1 (plumb expires_at through all layers)
3. **STOP and VALIDATE**: Run `make test && make lint`
4. Verify projected and actual cost caching respects plugin hints

### Incremental Delivery

1. Phase 1 → Cache layer ready
2. US1 → Plugin TTL hints work end-to-end (MVP)
3. US2 → Past timestamps skip caching (safety)
4. US3 → Debug logging for observability
5. Polish → Quality gates pass

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently testable after Phase 1 foundation
- Verify tests fail before implementing
- The `mockCache` in `engine_cache_test.go` must be updated (T005) before US1 tests can compile
- No TUI changes — no golden file tests needed
- No new packages created — all changes in existing files
