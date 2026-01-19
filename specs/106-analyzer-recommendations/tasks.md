# Tasks: Analyzer Recommendations Display

**Input**: Design documents from `/specs/106-analyzer-recommendations/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness),
all tasks MUST be fully implemented. Stub functions, placeholders, and TODO
comments are strictly forbidden.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This is a Go CLI project with paths at repository root:

- Source: `internal/engine/`, `internal/analyzer/`
- Tests: `internal/analyzer/`, `test/e2e/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Define the core data types that all user stories depend on

- [x] T001 [P] Add `Recommendation` struct to `internal/engine/types.go`
- [x] T002 [P] Add `Recommendations` field to `CostResult` in
  `internal/engine/types.go`
- [x] T003 Verify engine populates `Recommendations` from plugin response in
  `internal/engine/engine.go` (FR-003 coverage)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core formatting functions that MUST be complete before ANY user
story can be tested

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Add unit tests for `formatRecommendation` helper in
  `internal/analyzer/diagnostics_test.go`
- [x] T005 Implement `formatRecommendation` helper in
  `internal/analyzer/diagnostics.go`
- [x] T006 Add unit tests for `formatRecommendations` (multiple) in
  `internal/analyzer/diagnostics_test.go`
- [x] T007 Implement `formatRecommendations` function with 3-item limit in
  `internal/analyzer/diagnostics.go`

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - View Recommendations During Preview (Priority: P1) 🎯

**Goal**: Display cost optimization recommendations alongside cost estimates in
`pulumi preview` diagnostic output

**Independent Test**: Run `pulumi preview` with a plugin that returns
recommendations and verify diagnostic output includes both cost estimates and
actionable recommendations

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write tests FIRST, ensure they FAIL before impl**

- [x] T008 [P] [US1] Unit test for `CostToDiagnostic` with single recommendation
  in `internal/analyzer/diagnostics_test.go`
- [x] T009 [P] [US1] Unit test for `CostToDiagnostic` with multiple
  recommendations in `internal/analyzer/diagnostics_test.go`
- [x] T010 [P] [US1] Unit test for `CostToDiagnostic` with no recommendations
  in `internal/analyzer/diagnostics_test.go`
- [x] T011 [P] [US1] Unit test for recommendations combined with sustainability
  metrics in `internal/analyzer/diagnostics_test.go`
- [x] T012 [P] [US1] Unit test verifying ADVISORY enforcement level in all
  recommendation diagnostics (FR-008) in `internal/analyzer/diagnostics_test.go`

### Implementation for User Story 1

- [x] T013 [US1] Update `formatCostMessage` to append recommendations in
  `internal/analyzer/diagnostics.go`
- [x] T014 [US1] Ensure recommendations display follows sustainability pattern
  (pipe-separated) in `internal/analyzer/diagnostics.go`
- [x] T015 [US1] Run `make lint` and fix any issues
- [x] T016 [US1] Run `make test` and verify 80%+ coverage for new code

**Checkpoint**: User Story 1 complete - recommendations visible in diagnostics

---

## Phase 4: User Story 2 - Aggregated Recommendations in Stack Summary (P2)

**Goal**: Display aggregate recommendation count and total potential savings in
stack summary diagnostic

**Independent Test**: Run `pulumi preview` with multiple resources that have
recommendations and verify stack summary shows aggregate savings

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [x] T017 [P] [US2] Unit test for `StackSummaryDiagnostic` with recommendations
  in `internal/analyzer/diagnostics_test.go`
- [x] T018 [P] [US2] Unit test for aggregate savings calculation (same currency)
  in `internal/analyzer/diagnostics_test.go`
- [x] T019 [P] [US2] Unit test for mixed currency handling in stack summary
  in `internal/analyzer/diagnostics_test.go`

### Implementation for User Story 2

- [x] T020 [US2] Add `AggregateRecommendations` helper to calculate totals in
  `internal/analyzer/diagnostics.go`
- [x] T021 [US2] Update `StackSummaryDiagnostic` to include recommendation
  summary in `internal/analyzer/diagnostics.go`
- [x] T022 [US2] Handle mixed currencies with "mixed currencies" indicator in
  `internal/analyzer/diagnostics.go`
- [x] T023 [US2] Run `make lint` and `make test` to verify changes

**Checkpoint**: User Story 2 complete - stack summary shows aggregate savings

---

## Phase 5: User Story 3 - Graceful Handling of Missing Recommendations (P3)

**Goal**: Ensure preview completes successfully when recommendation data is
unavailable or malformed

**Independent Test**: Mock a plugin that returns costs but no recommendations,
verify cost diagnostics display without errors

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [x] T024 [P] [US3] Unit test for nil `Recommendations` slice in
  `internal/analyzer/diagnostics_test.go`
- [x] T025 [P] [US3] Unit test for empty `Recommendations` slice in
  `internal/analyzer/diagnostics_test.go`
- [x] T026 [P] [US3] Unit test for recommendation with zero savings in
  `internal/analyzer/diagnostics_test.go`
- [x] T027 [P] [US3] Unit test for recommendation with empty description in
  `internal/analyzer/diagnostics_test.go`

### Implementation for User Story 3

- [x] T028 [US3] Add nil/empty check in `formatRecommendations` in
  `internal/analyzer/diagnostics.go`
- [x] T029 [US3] Skip recommendations with empty Type or Description in
  `internal/analyzer/diagnostics.go`
- [x] T030 [US3] Log warning for skipped malformed recommendations in
  `internal/analyzer/diagnostics.go`
- [x] T031 [US3] Run `make lint` and `make test` to verify graceful handling

**Checkpoint**: User Story 3 complete - graceful degradation verified

---

## Phase 6: E2E Testing (Required per FR-010)

**Purpose**: Verify recommendations appear correctly in actual `pulumi preview`
output via E2E tests

**⚠️ NOTE**: T033-T034 are placeholder tests that validate graceful handling.
Full recommendation verification requires Phase 8-11 (GetRecommendations RPC).

- [x] T032 [P] Add E2E test fixture for recommendations in
  `test/e2e/fixtures/analyzer/` or extend existing fixture
- [x] T033 Add `TestAnalyzer_RecommendationDisplay` E2E test in
  `test/e2e/analyzer_e2e_test.go` (placeholder - awaits RPC integration)
- [x] T034 Add `TestAnalyzer_StackSummaryWithRecommendations` E2E test in
  `test/e2e/analyzer_e2e_test.go` (placeholder - awaits RPC integration)
- [x] T035 Run E2E tests with `make test-e2e` and verify pass

**Checkpoint**: E2E placeholder tests exist - full verification in Phase 11

---

## Phase 7: Intermediate Validation

**Purpose**: Validate diagnostics formatting works (pending RPC integration)

- [x] T036 [P] Run full test suite with coverage: `make test`
- [x] T037 [P] Run linting: `make lint`
- [x] T038 Verify 80%+ coverage for `internal/analyzer/diagnostics.go`

**Checkpoint**: Diagnostics formatting complete - RPC integration required next

---

## Phase 8: Proto Adapter - GetRecommendations RPC Integration ✅ COMPLETE

**Purpose**: Add `GetRecommendations` RPC support to proto adapter layer

**⚠️ BLOCKING**: Without this phase, recommendations will never appear in
diagnostics because `GetProjectedCostResponse` does NOT include recommendations.

### Tests for Phase 8 (MANDATORY - TDD Required) ⚠️

- [x] T041 [P] Add unit test for `GetRecommendationsRequest` type in
  `internal/proto/adapter_test.go`
- [x] T042 [P] Add unit test for `GetRecommendationsResponse` type in
  `internal/proto/adapter_test.go`
- [x] T043 [P] Add unit test for `clientAdapter.GetRecommendations` method in
  `internal/proto/adapter_test.go`

### Implementation for Phase 8

- [x] T044 Add `GetRecommendationsRequest` struct to `internal/proto/adapter.go`
- [x] T045 Add `GetRecommendationsResponse` struct to `internal/proto/adapter.go`
- [x] T046 Add `ProtoRecommendation` struct to `internal/proto/adapter.go`
- [x] T047 Add `GetRecommendations` to `CostSourceClient` interface in
  `internal/proto/adapter.go`
- [x] T048 Implement `GetRecommendations` in `clientAdapter` in
  `internal/proto/adapter.go`
- [x] T049 Run `make lint` and `make test` to verify proto adapter changes

**Checkpoint**: Proto adapter supports GetRecommendations RPC

---

## Phase 9: Engine - GetRecommendationsForResources ✅ COMPLETE

**Purpose**: Add engine method to fetch recommendations from plugins

### Tests for Phase 9 (MANDATORY - TDD Required) ⚠️

- [x] T050 [P] Add unit test for `GetRecommendationsForResources` with single
  plugin in `internal/engine/engine_test.go`
- [x] T051 [P] Add unit test for `GetRecommendationsForResources` with multiple
  plugins in `internal/engine/engine_test.go`
- [x] T052 [P] Add unit test for `GetRecommendationsForResources` with plugin
  failure (graceful degradation) in `internal/engine/engine_test.go`
- [x] T053 [P] Add unit test for `MergeRecommendations` helper in
  `internal/engine/engine_test.go`

### Implementation for Phase 9

- [x] T054 Add `GetRecommendationsForResources` method to `Engine` in
  `internal/engine/engine.go`
- [x] T055 Implement plugin iteration and recommendation aggregation in
  `internal/engine/engine.go`
- [x] T056 Add `MergeRecommendations` function to merge recs with CostResults in
  `internal/engine/engine.go`
- [x] T057 Run `make lint` and `make test` to verify engine changes

**Checkpoint**: Engine can fetch and merge recommendations from plugins

---

## Phase 10: Analyzer Server - Call GetRecommendations in AnalyzeStack ✅ COMPLETE

**Purpose**: Integrate recommendation fetching into analyzer workflow

### Tests for Phase 10 (MANDATORY - TDD Required) ⚠️

- [x] T058 [P] Add unit test for `AnalyzeStack` with recommendations in
  `internal/analyzer/server_test.go`
- [x] T059 [P] Add unit test for `AnalyzeStack` with recommendation fetch
  failure (graceful degradation) in `internal/analyzer/server_test.go`
- [x] T060 [P] Add unit test for `AnalyzeStack` matching recommendations to
  resources in `internal/analyzer/server_test.go`

### Implementation for Phase 10

- [x] T061 Update `AnalyzeStack` to call `GetRecommendationsForResources` in
  `internal/analyzer/server.go`
- [x] T062 Update `AnalyzeStack` to call `MergeRecommendations` before
  diagnostics in `internal/analyzer/server.go`
- [x] T063 Add graceful handling when recommendation fetch fails in
  `internal/analyzer/server.go`
- [x] T064 Run `make lint` and `make test` to verify analyzer changes

**Checkpoint**: Analyzer fetches and merges recommendations with costs

---

## Phase 11: E2E Testing with Recommendations ✅ COMPLETE

**Purpose**: Verify complete recommendation flow end-to-end

- [x] T065 Update E2E test fixture to use plugin that returns recommendations
  in `test/e2e/fixtures/analyzer/`
- [x] T066 Update `TestAnalyzer_RecommendationDisplay` to verify recommendation
  patterns in `test/e2e/analyzer_e2e_test.go`
- [x] T067 Update `TestAnalyzer_StackSummaryWithRecommendations` to verify
  aggregate savings in `test/e2e/analyzer_e2e_test.go`
- [x] T068 Remove TODO comments about GetRecommendations RPC in
  `test/e2e/analyzer_e2e_test.go`
- [x] T069 Run E2E tests with `make test-e2e` and verify recommendations appear

**Checkpoint**: E2E tests verify complete recommendation flow

---

## Phase 12: Final Polish ✅ COMPLETE

**Purpose**: Final validation and documentation

- [x] T070 [P] Run full test suite with coverage: `make test`
- [x] T071 [P] Run linting: `make lint`
- [x] T072 Verify 80%+ coverage for all modified files
- [x] T073 Update CLAUDE.md with GetRecommendations integration patterns
- [x] T074 Run `quickstart.md` validation scenarios with actual recommendations

---

## Dependencies & Execution Order

### Phase Dependencies

**Part 1: Diagnostics Formatting (Phases 1-7)** - MOSTLY COMPLETE ⚠️

- **Setup (Phase 1)**: ✅ Complete
- **Foundational (Phase 2)**: ✅ Complete
- **User Stories (Phases 3-5)**: ✅ Complete (formatting logic)
- **E2E Testing (Phase 6)**: ⚠️ Placeholder tests only - awaits Phase 8-11
- **Intermediate Validation (Phase 7)**: ✅ Validates formatting works

**Part 2: GetRecommendations RPC Integration (Phases 8-12)** - REQUIRED ⚠️

- **Proto Adapter (Phase 8)**: No dependencies - can start now
- **Engine (Phase 9)**: Depends on Phase 8 (proto adapter)
- **Analyzer Server (Phase 10)**: Depends on Phase 9 (engine)
- **E2E with Recommendations (Phase 11)**: Depends on Phase 10 (full flow)
- **Final Polish (Phase 12)**: Depends on all phases being complete

### Critical Path

```text
Phase 8 (Proto) → Phase 9 (Engine) → Phase 10 (Analyzer) → Phase 11 (E2E)
```

### Within Each Phase

- Tests MUST be written and FAIL before implementation
- Implementation follows test verification
- Lint and test commands run after each phase completion

### Parallel Opportunities

- T041-T043 can run in parallel (different test cases)
- T050-T053 can run in parallel (different test cases)
- T058-T060 can run in parallel (different test cases)

---

## Parallel Example: User Story 1 Tests

```bash
# Launch all US1 tests together:
Task: T008 - Unit test for CostToDiagnostic with single recommendation
Task: T009 - Unit test for CostToDiagnostic with multiple recommendations
Task: T010 - Unit test for CostToDiagnostic with no recommendations
Task: T011 - Unit test for recommendations combined with sustainability
Task: T012 - Unit test verifying ADVISORY enforcement level (FR-008)
```

---

## Implementation Strategy

### Part 1: Diagnostics Formatting (MOSTLY COMPLETE ⚠️)

1. Phases 1-5, 7: Setup, formatting, validation - DONE ✅
2. Phase 6: E2E tests are placeholders - NOT COMPLETE (awaits RPC integration)
3. Diagnostics can format recommendations when they exist
4. But recommendations are NOT being fetched yet!

### Part 2: GetRecommendations RPC Integration (REQUIRED ⚠️)

1. Complete Phase 8: Proto Adapter (T041-T049)
2. Complete Phase 9: Engine (T050-T057)
3. Complete Phase 10: Analyzer Server (T061-T064)
4. Complete Phase 11: E2E with actual recommendations (T065-T069)
5. Complete Phase 12: Final Polish (T070-T074)

### Why This Is Needed

**Critical Architecture Issue**: The `GetProjectedCostResponse` proto does NOT
include recommendations. They must be fetched via a separate `GetRecommendations`
RPC call.

Without Phases 8-12, recommendations will NEVER appear in diagnostics, even
though the formatting code is complete and tested.

---

## Notes

- [P] tasks = different files or independent test cases, no dependencies
- [Story] label maps task to specific user story for traceability
- Verify tests fail before implementing
- Commit after each task or logical group
- Follow existing sustainability metrics pattern in diagnostics.go:122-148
- **CRITICAL**: GetRecommendations is a SEPARATE RPC from GetProjectedCost
