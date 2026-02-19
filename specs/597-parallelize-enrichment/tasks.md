# Tasks: Parallelize Per-Row Enrichment Sub-Calls

**Input**: Design documents from `/specs/597-parallelize-enrichment/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Signature Refactoring)

**Purpose**: Refactor enrichment function signatures to return errors instead of writing to shared `row.Error`. This is a prerequisite for all user stories because the parallelization depends on error values being returned, not written directly.

- [x] T001 Refactor `enrichActualCost` in `internal/engine/overview_enrich.go` to return `*OverviewRowError` instead of writing to `row.Error` directly. Change line 76 from `row.Error = classifyError(row.URN, err)` to `return classifyError(row.URN, err)`. Change line 97 (implicit return) to `return nil`. Remove the `row.Error` write. Keep `row.ActualCost` writes as-is.
- [x] T002 Refactor `enrichProjectedCost` in `internal/engine/overview_enrich.go` to return `*OverviewRowError` instead of writing to `row.Error` directly. Remove the `if row.Error == nil` guard on lines 110-112. Change line 111 to `return classifyError(row.URN, err)`. Change line 131 (implicit return) to `return nil`. Keep `row.ProjectedCost` writes as-is.
- [x] T003 Update `EnrichOverviewRow` in `internal/engine/overview_enrich.go` to capture return values from `enrichActualCost` and `enrichProjectedCost` into local variables and assign `row.Error` after all calls complete using the error merge contract: actual cost error takes precedence over projected cost error. This preserves sequential behavior while enabling the parallel refactor.
- [x] T004 Run `go test -race ./internal/engine/...` to verify all existing tests pass with the signature refactoring and no behavioral changes.

**Checkpoint**: Function signatures refactored. All existing tests pass. Behavior is identical to before. Ready for parallelization.

---

## Phase 2: User Story 1 - Faster Overview Enrichment (Priority: P1)

**Goal**: Execute actual cost, projected cost, and recommendation enrichment calls concurrently within `EnrichOverviewRow` using goroutines and `sync.WaitGroup` to reduce per-row wall-clock time.

**Independent Test**: Run `EnrichOverviewRow` with a resource that has all three data sources available and verify all fields are populated identically to the sequential implementation.

### Tests for User Story 1 (TDD)

- [x] T005 [US1] Write test `TestEnrichOverviewRow_ParallelPopulatesAllFields` in `internal/engine/overview_enrich_test.go` that calls `EnrichOverviewRow` with an active resource (no plugins, spec fallback) and asserts that `ActualCost`, `ProjectedCost`, and `Recommendations` fields are populated with the same values as the sequential implementation.
- [x] T006 [US1] Write test `TestEnrichOverviewRow_CreatingStatus_SkipsActualCostGoroutine` in `internal/engine/overview_enrich_test.go` that calls `EnrichOverviewRow` with `StatusCreating` and asserts `ActualCost` is nil while `ProjectedCost` is populated (verifies actual cost goroutine is not launched).

### Implementation for User Story 1

- [x] T007 [US1] Parallelize `EnrichOverviewRow` in `internal/engine/overview_enrich.go`: add `var wg sync.WaitGroup` and local error variables `var actualErr, projectedErr *OverviewRowError`. Launch each enrichment call as a goroutine with `wg.Add(1)` and `defer wg.Done()`. Conditionally skip actual cost goroutine for `StatusCreating`. Call `wg.Wait()` before error merge and cost drift calculation. Ensure `"sync"` import is present (already imported).
- [x] T008 [US1] Run `go test -race ./internal/engine/...` to verify parallel execution produces identical results to sequential and the race detector reports zero warnings.

**Checkpoint**: `EnrichOverviewRow` runs all three sub-calls concurrently. All existing tests pass with `-race`. FR-001, FR-002, FR-005, FR-006 satisfied.

---

## Phase 3: User Story 2 - Thread-Safe Error Handling (Priority: P1)

**Goal**: Ensure the parallel enrichment correctly captures errors from concurrent calls without data races, with deterministic error precedence (actual cost error preferred over projected cost error).

**Independent Test**: Trigger error conditions in actual and projected cost fetches and verify `row.Error` is deterministic and race-free across multiple runs.

### Tests for User Story 2 (TDD)

- [x] T009 [US2] Write test `TestEnrichOverviewRow_ErrorMerge_ActualErrorOnly` in `internal/engine/overview_enrich_test.go` using a mock engine or error-inducing setup where actual cost fails (returns a classified error) but projected cost succeeds. Assert `row.Error` contains the actual cost error and `row.ProjectedCost` is still populated.
- [x] T010 [US2] Write test `TestEnrichOverviewRow_ErrorMerge_ProjectedErrorOnly` in `internal/engine/overview_enrich_test.go` where projected cost fails but actual cost succeeds. Assert `row.Error` contains the projected cost error and `row.ActualCost` is still populated.
- [x] T011 [US2] Write test `TestEnrichOverviewRow_ErrorMerge_BothErrors_ActualWins` in `internal/engine/overview_enrich_test.go` where both actual and projected cost fail. Assert `row.Error` always contains the actual cost error (not projected), verifying FR-004 deterministic precedence. Run this test 100 times in a loop to confirm no scheduling-dependent variation.
- [x] T012 [US2] Write test `TestEnrichOverviewRow_RaceDetector` in `internal/engine/overview_enrich_test.go` that runs `EnrichOverviewRow` concurrently for multiple rows (using `EnrichOverviewRows` with the worker pool) and verifies zero race detector warnings via `go test -race -count=5`.

### Implementation for User Story 2

- [x] T013 [US2] Verify the error merge logic in `EnrichOverviewRow` in `internal/engine/overview_enrich.go` implements the contract: after `wg.Wait()`, set `row.Error = actualErr` if non-nil, else `row.Error = projectedErr` if non-nil, else leave nil. This was partially implemented in T003/T007 but must be validated with the parallel goroutine paths now exercised by tests.
- [x] T014 [US2] Run `go test -race -count=10 ./internal/engine/... -run TestEnrichOverviewRow` to stress-test the race detector across all enrichment test variants.

**Checkpoint**: Error handling is race-free and deterministic. FR-003, FR-004, FR-007 satisfied. SC-003, SC-005 verified.

---

## Phase 4: User Story 3 - Cost Drift After Parallel Completion (Priority: P2)

**Goal**: Verify cost drift is correctly calculated after both actual and projected costs complete in parallel, producing identical results to the sequential implementation.

**Independent Test**: Provide a resource with both cost types available and verify drift calculation uses correct values post-parallel-completion.

### Tests for User Story 3 (TDD)

- [x] T015 [US3] Write test `TestEnrichOverviewRow_CostDrift_AfterParallelCompletion` in `internal/engine/overview_enrich_test.go` that provides a resource where both actual and projected costs are available (via engine with no plugins returning spec fallback). Assert `row.CostDrift` is populated and matches the expected drift calculation from `CalculateCostDrift`.
- [x] T016 [US3] Write test `TestEnrichOverviewRow_CostDrift_SkippedWhenMissingCost` in `internal/engine/overview_enrich_test.go` that verifies `row.CostDrift` is nil when either actual or projected cost is nil (e.g., `StatusCreating` resource where actual cost is skipped).

### Implementation for User Story 3

- [x] T017 [US3] Verify that the `enrichCostDrift` call in `EnrichOverviewRow` in `internal/engine/overview_enrich.go` remains after `wg.Wait()` and only executes when both `row.ActualCost != nil && row.ProjectedCost != nil`. This was preserved in T007 but must be confirmed with the parallel tests from T015/T016.

**Checkpoint**: Cost drift calculation is correct after parallel completion. FR-006, FR-008 satisfied.

---

## Phase 5: Validation and Quality Gates

**Purpose**: Final validation across all user stories, existing test suite, linting, and race detection.

- [x] T018 Run `go test -race ./internal/engine/...` to verify all new and existing tests pass with the race detector enabled.
- [x] T019 Run `make test` to verify the full project test suite passes.
- [x] T020 Run `make lint` to verify no linting regressions (golangci-lint + markdownlint).
- [x] T021 Verify test coverage for `internal/engine/overview_enrich.go` meets 80% minimum by running `go test -coverprofile=coverage.out ./internal/engine/... && go tool cover -func=coverage.out | grep overview_enrich`.

**Checkpoint**: All quality gates pass. SC-001 through SC-005 verified. Feature complete.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Foundational)**: No dependencies - start immediately
- **Phase 2 (US1 - Parallelization)**: Depends on Phase 1 completion
- **Phase 3 (US2 - Error Handling)**: Depends on Phase 2 completion (needs parallel code to test race conditions)
- **Phase 4 (US3 - Cost Drift)**: Depends on Phase 2 completion (needs parallel code to test drift timing)
- **Phase 5 (Validation)**: Depends on all previous phases

### User Story Dependencies

- **US1 (Parallelization)**: Foundational signature refactor must complete first (Phase 1)
- **US2 (Error Handling)**: Depends on US1 (needs parallel goroutines to be in place to test race conditions)
- **US3 (Cost Drift)**: Depends on US1 (needs parallel completion to verify drift timing). Can run in parallel with US2.

### Parallel Opportunities

Within Phase 1:

- T001 and T002 can run in parallel (different functions in same file, but non-overlapping lines)

Within Phase 3 (US2 tests):

- T009, T010, T011, T012 can all be written in parallel (different test functions)

Within Phase 4 (US3):

- T015 and T016 can be written in parallel (different test functions)

Cross-phase:

- Phase 3 (US2) and Phase 4 (US3) can run in parallel after Phase 2 (US1) completes

---

## Implementation Strategy

### MVP First (Phase 1 + Phase 2 = US1)

1. Complete Phase 1: Refactor function signatures (T001-T004)
2. Complete Phase 2: Parallelize and verify correctness (T005-T008)
3. **STOP and VALIDATE**: Run `go test -race` - parallelization works, all existing tests pass
4. This alone delivers the core performance improvement

### Incremental Delivery

1. Phase 1 (Foundational) → Signatures refactored, tests pass
2. Phase 2 (US1) → Parallelized, race-free → **MVP complete**
3. Phase 3 (US2) → Error handling hardened with dedicated tests
4. Phase 4 (US3) → Cost drift verified post-parallel
5. Phase 5 (Validation) → All quality gates pass → **Feature complete**

---

## Notes

- All tasks target `internal/engine/overview_enrich.go` and `internal/engine/overview_enrich_test.go`
- No new files are created in the source tree
- Public API signatures (`EnrichOverviewRow`, `EnrichOverviewRows`) are unchanged
- The `"sync"` import already exists in `overview_enrich.go` (used by `EnrichOverviewRows`)
- Existing test functions in `overview_enrich_test.go` must not be modified (FR-008, SC-002)
