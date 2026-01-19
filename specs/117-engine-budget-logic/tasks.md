# Implementation Tasks - Engine Budget Logic

**Feature**: Engine Budget Logic
**Branch**: `117-engine-budget-logic`
**Status**: Ready for Implementation

## Implementation Strategy
- **Approach**: Test-Driven Development (TDD).
- **Phasing**: Sequential implementation of Budget Filtering followed by Summary Calculation.
- **Completeness**: All tasks aim for full implementation with no stubs.

## Phase 1: Setup
- [x] T001 Verify `finfocus-spec` dependency in `go.mod` matches requirement (v0.5.2+)

## Phase 2: Foundational
- [x] T002 Create `internal/engine/budget.go` and `internal/engine/budget_test.go` with initial package declaration

## Phase 3: User Story 1 - Advanced Budget Filtering (P1)
**Goal**: Implement robust budget filtering by Provider, Region, ResourceType, and Tags.
**Independent Test**: Verify `FilterBudgets` correctly reduces a list of mixed budgets based on `BudgetFilter` criteria.

### Tests
- [x] T003 [US1] Create table-driven tests for `FilterBudgets` covering Provider case-insensitivity, Region, ResourceType, and Tag matching in `internal/engine/budget_test.go`
- [x] T004 [US1] Create test cases for empty filter returning all budgets and non-matching filters returning empty list in `internal/engine/budget_test.go`

### Implementation
- [x] T005 [US1] Implement `FilterBudgets` function signature and basic Provider filtering logic in `internal/engine/budget.go`
- [x] T006 [US1] Implement Region and ResourceType filtering logic (checking `Metadata`) in `internal/engine/budget.go`
- [x] T007 [US1] Implement Tag filtering logic (subset match against `Metadata`) in `internal/engine/budget.go`
- [x] T008 [P] [US1] Implement helper `matchStringSlice` for OR logic matching in `internal/engine/budget.go`

## Phase 4: User Story 2 - Budget Health Summary (P1)
**Goal**: Aggregate budget health statuses into a summary object.
**Independent Test**: Verify `CalculateBudgetSummary` returns correct counts for a known set of budget statuses.

### Tests
- [x] T009 [US2] Create table-driven tests for `CalculateBudgetSummary` covering all health status buckets in `internal/engine/budget_test.go`
- [x] T010 [US2] Create test case for missing/nil health status (should warn and exclude from buckets) in `internal/engine/budget_test.go`

### Implementation
- [x] T011 [US2] Implement `CalculateBudgetSummary` function signature and iteration logic in `internal/engine/budget.go`
- [x] T012 [US2] Implement health status counting logic and `TotalBudgets` calculation in `internal/engine/budget.go`
- [x] T013 [US2] Implement Zerolog warning for budgets with missing health status in `internal/engine/budget.go`

## Phase 5: User Story 3 - Multi-Currency Budget Display (P2)
**Goal**: Validate and preserve currency codes.
**Independent Test**: Verify invalid currencies are rejected and valid ones preserved.

### Tests
- [x] T014 [US3] Create test cases for currency validation (valid ISO 4217 vs invalid) in `internal/engine/budget_test.go`

### Implementation
- [x] T015 [US3] Implement `validateCurrency` helper using regex `^[A-Z]{3}$` in `internal/engine/budget.go`
- [x] T016 [US3] Integrate currency validation into `FilterBudgets` or input processing (if applicable) in `internal/engine/budget.go`

## Phase 6: Polish & Cross-Cutting
- [x] T017 Run full engine test suite `go test ./internal/engine/...` to ensure no regressions
- [x] T018 Update `README.md` to mention new Budget Engine capabilities

## Dependencies

- **Phase 3** (Filtering) is independent.
- **Phase 4** (Summary) depends on `Budget` struct but can be developed in parallel with filtering logic if using mocked inputs.
- **Phase 5** (Currency) is a validation layer on top of `Budget` entities.

## Parallel Execution Examples

- **T008** (Helper) can be implemented while **T003** (Tests) is being written.
- **Phase 4** tests (T009) can be written while **Phase 3** implementation (T005-T007) is in progress.
