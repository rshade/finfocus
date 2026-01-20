# Tasks: Budget Status Display with Threshold Alerts

**Input**: Design documents from `/specs/001-cli-budget-alerts/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/engine.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Add `github.com/charmbracelet/lipgloss` and `golang.org/x/term` dependencies to `go.mod`
- [X] T002 [P] Create `internal/config/budget.go` defining `BudgetConfig` and `AlertConfig` structs per `data-model.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Integrate `BudgetConfig` into the main `Config` struct in `internal/config/config.go`
- [X] T004 Implement YAML parsing logic and Dot-notation accessors for budgets in `internal/config/budget.go`
- [X] T005 [P] Implement unit tests for budget configuration parsing in `internal/config/budget_test.go`
- [X] T006 Create `internal/engine/budget_cli.go` with `BudgetEngine` interface and `BudgetStatus` structs per `contracts/engine.md`
- [X] T007 Implement base budget evaluation logic (actual spend vs amount) in `internal/engine/budget_cli.go`
- [X] T008 [P] Implement unit tests for base budget evaluation in `internal/engine/budget_cli_test.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - View Budget Status (Priority: P1) 🎯 MVP

**Goal**: Users can see their monthly budget and a styled progress bar in the CLI.

**Independent Test**: Configure a budget and run `finfocus cost`. Verify stylized output with progress bar appears in TTY.

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

- [X] T009 [P] [US1] Create unit tests for styled budget rendering in `internal/cli/cost_budget_test.go`
- [X] T010 [P] [US1] Add integration test for budget status display in `test/integration/budget_test.go`

### Implementation for User Story 1

- [X] T011 [P] [US1] Implement `renderStyledBudget` in `internal/cli/cost_budget.go` using Lip Gloss
- [X] T012 [US1] Implement TTY detection and layout logic in `internal/cli/cost_budget.go`
- [X] T013 [US1] Integrate budget rendering into the cost command flow in `internal/cli/cost_actual.go`

**Checkpoint**: User Story 1 should be fully functional and testable independently in TTY mode.

---

## Phase 4: User Story 2 - Threshold Alerts (Priority: P1)

**Goal**: Visual warnings appear when spending exceeds configured thresholds (e.g., 80%).

**Independent Test**: Set threshold at 80% and spend at 85%. Verify "WARNING - Exceeds 80% threshold" appears in CLI.

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [X] T014 [P] [US2] Create unit tests for threshold evaluation logic in `internal/engine/budget_cli_test.go`
- [X] T015 [P] [US2] Update integration tests in `test/integration/budget_test.go` for threshold alerts

### Implementation for User Story 2

- [X] T016 [US2] Implement threshold status calculation (OK, EXCEEDED) in `internal/engine/budget_cli.go`
- [X] T017 [US2] Update `renderStyledBudget` in `internal/cli/cost_budget.go` to display threshold labels and color-coded warnings
- [X] T018 [US2] Ensure multiple thresholds can be evaluated and displayed simultaneously in `internal/engine/budget_cli.go`

**Checkpoint**: User Story 2 should correctly trigger and display alerts.

---

## Phase 5: User Story 4 - CI/CD Integration (Priority: P1)

**Goal**: Plain-text budget summary for non-TTY/pipeline environments.

**Independent Test**: Pipe output to `cat`. Verify budget status is rendered in clean ASCII without styling.

### Tests for User Story 4 (MANDATORY - TDD Required) ⚠️

- [X] T019 [P] [US4] Create unit tests for plain text budget rendering in `internal/cli/cost_budget_test.go`
- [X] T020 [P] [US4] Add integration test for non-TTY output in `test/integration/budget_test.go`

### Implementation for User Story 4

- [X] T021 [P] [US4] Implement `renderPlainBudget` in `internal/cli/cost_budget.go` for non-TTY environments
- [X] T022 [US4] Ensure `internal/cli/cost_budget.go` falls back to `renderPlainBudget` when `x/term.IsTerminal` is false

**Checkpoint**: CI/CD pipelines should now receive readable budget status in logs.

---

## Phase 6: User Story 3 - Forecasted Spend Alerts (Priority: P2)

**Goal**: Warnings trigger if forecasted monthly spend exceeds thresholds.

**Independent Test**: Set forecasted threshold at 100%. Spend 60% halfway through month. Verify "Forecast exceeds threshold" warning appears.

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [X] T023 [P] [US3] Create unit tests for linear forecasting logic in `internal/engine/budget_cli_test.go`
- [X] T024 [P] [US3] Update integration tests for forecasted alerts in `test/integration/budget_test.go`

### Implementation for User Story 3

- [X] T025 [US3] Implement linear extrapolation forecasting logic in `internal/engine/budget_cli.go`
- [X] T026 [US3] Update `Evaluate` to support `forecasted` alert types in `internal/engine/budget_cli.go`
- [X] T027 [US3] Update `renderStyledBudget` in `internal/cli/cost_budget.go` to display forecasted spend progress bars

**Checkpoint**: Forecasting alerts should now work for proactive budget management.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Edge case handling and final cleanup

- [X] T028 Implement edge case: Zero Budget (Disabled) validation in `internal/config/budget.go`
- [X] T029 Implement edge case: Negative Spend (Credit) handling in `internal/engine/budget_cli.go`
- [X] T030 Implement edge case: Currency Mismatch error in `internal/engine/budget_cli.go`
- [X] T031 Implement edge case: Over-budget capping at 100% in `internal/cli/cost_budget.go`
- [X] T032 [P] Update `docs/user-guide.md` with budgeting and alerts documentation
- [X] T033 [P] Update `README.md` with quickstart examples for budgeting
- [X] T034 Run final validation with `make lint` and `make test` across the whole project

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on T001, T002.
- **User Stories (Phase 3-6)**: All depend on Phase 2 completion.
  - US1, US2, US4 can proceed in parallel once Foundation is ready.
  - US3 depends on US1/US2 rendering structure but can start in parallel at engine level.

### Parallel Opportunities

- T001 and T002 can be done in parallel.
- T005, T008 (Tests) can be done in parallel with their implementations.
- Once Foundation (Phase 2) is done, US1, US2, and US4 phases can be worked on in parallel.
- Documentation updates (T032, T033) can be done in parallel with implementation.

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup and Foundational phases.
2. Implement User Story 1 (Basic display).
3. Verify with local cost analysis.

### Incremental Delivery

1. Add Threshold Alerts (US2) to provide immediate value for over-spending.
2. Add CI/CD support (US4) to enable pipeline integration.
3. Add Forecasting (US3) as an advanced feature for proactive monitoring.
