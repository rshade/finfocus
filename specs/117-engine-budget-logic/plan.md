# Implementation Plan - Engine Budget Logic

**Feature**: Engine Budget Logic
**Branch**: `117-engine-budget-logic`
**Spec**: [specs/117-engine-budget-logic/spec.md](spec.md)
**Status**: Phase 2 (Implementation)

## Technical Context

**Language/Framework**: Go 1.25.7
**Existing Components**:
- `internal/engine/`: Core engine logic.
- `github.com/rshade/finfocus-spec`: Proto definitions (v0.5.2).

**Data Interactions**:
- **Input**: `[]*pbc.Budget`, `*pbc.BudgetFilter`.
- **Output**: `[]*pbc.Budget`, `*pbc.BudgetSummary`.

## Constitution Check

| Principle | Compliance Check |
|-----------|------------------|
| **Plugin-First** | N/A - Logic layer. |
| **TDD** | **Compliant**: Tasks include test creation first. |
| **Cross-Platform** | **Compliant**: Pure Go. |
| **Docs Sync** | **Compliant**: Docs update task included. |
| **Completeness** | **Compliant**: No TODOs allowed. |

## Gates

- [x] **Spec Quality**: Passed.
- [x] **Constitution Alignment**: Confirmed.
- [x] **Clarifications**: Resolved (Research Complete).

---

## Phase 0: Research & Decisions

- **Proto**: Using `finfocus-spec` v0.5.2.
- **Currency**: Regex validator `^[A-Z]{3}$`.
- **Tags**: "Match All" logic against Metadata/Tags.
- **Missing Health**: Log warning, exclude from buckets.

## Phase 1: Design & Contracts

- **Data Model**: [data-model.md](data-model.md)
- **Quickstart**: [quickstart.md](quickstart.md)

## Phase 2: Implementation Tasks

### T001: Create Budget Logic & Tests
**Goal**: Implement `FilterBudgets` and `CalculateBudgetSummary` with TDD.
**Files**: `internal/engine/budget.go`, `internal/engine/budget_test.go`
**Steps**:
1. Create `budget_test.go` with table-driven tests for Filtering (Provider, empty filter, case-insensitivity).
2. Create `budget_test.go` with table-driven tests for Summary (counts, missing health).
3. Implement `FilterBudgets` in `budget.go`.
4. Implement `CalculateBudgetSummary` in `budget.go`.
5. Implement `validateCurrency` helper.
6. Verify 80%+ coverage.

### T002: Advanced Filtering
**Goal**: Add Region and Tag filtering support.
**Files**: `internal/engine/budget.go`, `internal/engine/budget_test.go`
**Steps**:
1. Add test cases for Region and Tag filtering.
2. Update `FilterBudgets` to check `Regions` and `Tags` (likely mapping to `Metadata` if direct fields don't exist).

### T003: Integration Verification
**Goal**: Verify with `go test ./internal/engine/...`.

### T004: Documentation
**Goal**: Update `README.md` or `docs/` to mention new engine capabilities.
