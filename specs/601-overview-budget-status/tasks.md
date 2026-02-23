# Tasks: Overview Budget Status and Health

**Input**: Design documents from `/specs/601-overview-budget-status/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

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
- **E2E tests**: `test/e2e/` (requires built binary and external credentials)
  - Run with: `go test -tags e2e ./test/e2e/...`

> **RETIRED**: `test/unit/` is retired as of issue #732. Do NOT place new Go unit tests
> there -- they will not be discovered by `make test` or CI.

---

## Phase 1: Foundational (Shared Types and Message Infrastructure)

**Purpose**: Add shared types, message definitions, and model fields that ALL user stories depend on. These are the building blocks for budget integration across TUI, plain text, and JSON output.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T001 [P] Add `BudgetDataReadyMsg` type (with `Result *engine.BudgetResult` and `Error error` fields) to `internal/tui/overview_messages.go`
- [X] T002 [P] Add budget state fields (`budgetResult *engine.BudgetResult`, `budgetErr error`, `budgetLoaded bool`) to `OverviewModel` struct in `internal/tui/overview_model.go`
- [X] T003 [P] Add `BudgetHealthSummary` type (`OverallHealth string`, `TotalBudgets int`, `CriticalCount int` with JSON tags and `omitempty`) to `internal/engine/overview_types.go` and add `BudgetHealth *BudgetHealthSummary` field (with `json:"budgetHealth,omitempty"` tag) to `StackContext` struct

**Checkpoint**: All shared types and model fields exist. User story implementation can begin.

---

## Phase 2: User Story 1 -- Interactive Budget Health at a Glance (Priority: P1) MVP

**Goal**: Display a color-coded budget health footer in the TUI list view that loads asynchronously without delaying the resource table. Shows overall spend vs limit and utilization percentage with green/yellow/red health badges.

**Independent Test**: Launch TUI with budgets configured, verify footer renders with correct health badge, spend/limit, and percentage. Delivers value as standalone visual indicator.

### Tests for User Story 1 (MANDATORY -- TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T004 [P] [US1] Test `BudgetDataReadyMsg` handling in `OverviewModel.Update()` -- verify `budgetResult`, `budgetErr`, and `budgetLoaded` fields are set correctly for success, error, and nil-result cases in `internal/tui/overview_model_test.go`
- [X] T005 [P] [US1] Test budget footer rendering for all health states (OK at 45%, WARNING at 85%, CRITICAL at 95%, EXCEEDED at 105%), no-budget case (empty footer), mixed-currency case (health badge only, no dollar amounts), and single-budget case (direct spend/limit) in `internal/tui/overview_budget_test.go`

### Implementation for User Story 1

- [X] T006 [US1] Handle `BudgetDataReadyMsg` in `OverviewModel.Update()` switch statement -- store result in `budgetResult`, error in `budgetErr`, set `budgetLoaded = true`, and trigger re-render in `internal/tui/overview_model.go`
- [X] T007 [US1] Create `internal/tui/overview_budget.go` with `renderBudgetFooter(m OverviewModel) string` -- implement color-coded health badge (green OK 0-79%, yellow WARNING 80-89%, red CRITICAL 90-99%, red bold EXCEEDED 100%+), aggregated spend/limit/percentage for same-currency budgets, health-badge-only for mixed currencies, exclude budgets with `Limit <= 0` from aggregation (treated as disabled per data model), and return empty string when no budgets or not loaded
- [X] T008 [US1] Insert `renderBudgetFooter()` call in `renderListView()` between the table/pagination section and the status bar in `internal/tui/overview_view.go`
- [X] T009 [US1] Launch concurrent budget fetch goroutine in `overviewInitAndEnrich()` after engine creation (phase 5) -- call `engine.GetBudgets(ctx, nil)`, send `tui.BudgetDataReadyMsg{Result, Error}` via `p.Send()`, log warning on error (non-fatal) in `internal/cli/overview.go`

**Checkpoint**: TUI displays budget health footer with async loading. Resource table renders immediately; footer appears when budget data arrives.

---

## Phase 3: User Story 2 -- Budget Detail View in TUI (Priority: P2)

**Goal**: When viewing resource detail (pressing Enter), show a "BUDGET STATUS" section with per-budget breakdown including name, limit, current spend, forecasted spend, utilization percentage, and triggered alerts.

**Independent Test**: Navigate to detail view when budgets are loaded, verify BUDGET STATUS section displays correct per-budget information.

**Depends on**: US1 (budget data must be fetched and stored in model)

### Tests for User Story 2 (MANDATORY -- TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T010 [P] [US2] Test detail budget section rendering -- verify per-budget breakdown with name/limit/spend/forecasted/utilization/health for multiple budgets (one OK, one WARNING), triggered alert display (80% actual, 100% forecasted thresholds), and hidden section when no budgets loaded in `internal/tui/overview_budget_test.go`

### Implementation for User Story 2

- [X] T011 [US2] Implement `renderDetailBudgetStatus(m OverviewModel) string` in `internal/tui/overview_budget.go` -- render "BUDGET STATUS" header, per-budget lines with name, limit, current spend, forecasted spend, utilization percentage, health status badge, and triggered threshold alerts; return empty string when `budgetLoaded == false` or `budgetResult == nil` or no budgets
- [X] T012 [US2] Insert `renderDetailBudgetStatus()` call in `renderDetailView()` after the recommendations section in `internal/tui/overview_view.go`

**Checkpoint**: TUI detail view shows per-budget breakdown. Both footer (US1) and detail (US2) work independently.

---

## Phase 4: User Story 3 -- Plain Text Budget Status (Priority: P2)

**Goal**: Render budget status box after the resource table in non-TTY output (piped, CI/CD, `--plain`), reusing existing `RenderBudgetStatus()` format. Support `--exit-on-threshold` for CI/CD budget gate enforcement.

**Independent Test**: Pipe `finfocus overview` output and verify budget status box appears after table. Test exit code behavior with `--exit-on-threshold`.

### Tests for User Story 3 (MANDATORY -- TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T013 [P] [US3] Test budget flag registration (`--exit-on-threshold`, `--exit-code`, `--budget-scope`) on overview command, budget evaluation wiring in non-TTY path, exit code behavior when budget exceeded with and without `--exit-on-threshold`, and `--exit-on-threshold` ignored in TTY mode in `internal/cli/overview_test.go`

### Implementation for User Story 3

- [X] T014 [US3] Add `--exit-on-threshold` (bool, default false), `--exit-code` (int, default 1), and `--budget-scope` (string, default empty) flags to `NewOverviewCmd()` in `internal/cli/overview.go` -- match flag names and semantics from `cost projected` command
- [X] T015 [US3] Wire budget evaluation into `executeOverview()` non-TTY path in `internal/cli/overview.go` -- after `renderOverviewOutput()`, aggregate `totalCost` by summing `ProjectedCost.MonthlyCost` from enriched `OverviewRow` objects, extract currency from the first row with cost data, call `evaluateBudgetStatus(cmd, costResults, totalCost)` (note: `evaluateBudgetStatus` uses `engine.GetBudgets()` internally for budget data -- the costResults/totalCost are used for scope rendering and currency extraction), and return `BudgetExitError` if thresholds exceeded

**Checkpoint**: Non-TTY output includes budget status. CI/CD pipelines can gate on budget health via exit codes.

---

## Phase 5: User Story 4 -- Budget Data in JSON/NDJSON Output (Priority: P3)

**Goal**: Include budget health data in JSON output as a top-level `budgets` array for programmatic consumption. NDJSON output remains unchanged (budgets are stack-scoped, not resource-scoped).

**Independent Test**: Run `finfocus overview --output json` with budgets configured, validate JSON contains `budgets` array with expected fields.

### Tests for User Story 4 (MANDATORY -- TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T016 [P] [US4] Test JSON output includes `budgets` array with correct fields (budgetID, budgetName, provider, health, utilization, forecasted, currency, limit, currentSpend), omits `budgets` when empty, and NDJSON output excludes budget data in `internal/engine/overview_render_test.go`

### Implementation for User Story 4

- [X] T017 [US4] Add `Budgets []BudgetHealthResult` field (with `json:"budgets,omitempty"` tag) to `OverviewJSONOutput` struct in `internal/engine/overview_render.go`
- [X] T018 [US4] Extend `RenderOverviewAsJSON()` signature to accept an optional `*BudgetResult` parameter, populate `Budgets` field by converting `BudgetResult.Budgets` to `[]BudgetHealthResult` using existing `prepareBudget()` helpers, omit field when budget result is nil or empty, and update all callers to pass the budget result (or nil) in `internal/engine/overview_render.go`

**Checkpoint**: JSON output includes machine-readable budget health data. NDJSON remains per-resource only.

---

## Phase 6: Polish and Cross-Cutting Concerns

**Purpose**: Verify quality gates, update documentation, and validate against quickstart scenarios.

- [X] T019 [P] Update CLAUDE.md with overview budget integration patterns (TUI budget message flow, footer/detail rendering helpers, non-TTY budget evaluation, JSON budget output)
- [X] T020 Run `make lint` and `make test` to verify all quality gates pass -- zero lint errors, all tests green, coverage meets 80% minimum for new code
- [X] T021 Validate implementation against `specs/601-overview-budget-status/quickstart.md` test scenarios -- verify each step produces expected results

---

## Phase 7: Config-Based Budget Fallback for TUI

**Purpose**: When plugins return no budget data, fall back to config-based budgets using the existing `DefaultBudgetEngine.Evaluate()` and enriched cost data so the TUI budget footer still appears.

- [X] T022 Add `ConfigBudgetToProto` and `BuildConfigBudgetResult` bridge functions to `internal/engine/budget.go`; modify budget goroutine in `overviewInitAndEnrich` (`internal/cli/overview.go`) to use channel-based budget fetch with config fallback after enrichment; add `sumOverviewProjectedCost` helper; add unit tests in `internal/engine/budget_test.go`

---

## Dependencies and Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies -- can start immediately
- **US1 (Phase 2)**: Depends on Phase 1 completion -- BLOCKS US2
- **US2 (Phase 3)**: Depends on US1 (budget data in model)
- **US3 (Phase 4)**: Depends on Phase 1 only -- can run PARALLEL with US1
- **US4 (Phase 5)**: Depends on Phase 1 only -- can run PARALLEL with US1
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

```text
Phase 1 (Foundational)
    |
    +-----+-------+-------+
    |     |       |       |
    v     v       v       v
   US1   US3    US4    (parallel)
    |
    v
   US2
    |
    v
  Polish
```

- **US1 (P1)**: After Foundational. Blocked by nothing else.
- **US2 (P2)**: After US1. Needs budget data in model from US1 goroutine.
- **US3 (P2)**: After Foundational. Independent of US1 (different code path: `evaluateBudgetStatus` vs `engine.GetBudgets` goroutine).
- **US4 (P3)**: After Foundational. Independent of US1 (modifies JSON output, not TUI).

### Within Each User Story

1. Tests written FIRST, verified to FAIL
2. Implementation tasks in dependency order
3. Story checkpoint verified before next story

### Parallel Opportunities

- **Phase 1**: T001, T002, T003 all parallel (different files)
- **Phase 2**: T004, T005 parallel (different test files)
- **Cross-story**: US1, US3, US4 can run in parallel after Phase 1
- **Phase 6**: T019 parallel with T020

---

## Parallel Example: Phase 1 (Foundational)

```text
# All three tasks touch different files -- launch in parallel:
Task T001: Add BudgetDataReadyMsg to internal/tui/overview_messages.go
Task T002: Add budget fields to OverviewModel in internal/tui/overview_model.go
Task T003: Add BudgetHealthSummary type to internal/engine/overview_types.go
```

## Parallel Example: User Story Tests

```text
# US1 tests touch different files -- launch in parallel:
Task T004: Test message handling in internal/tui/overview_model_test.go
Task T005: Test footer rendering in internal/tui/overview_budget_test.go

# Cross-story tests can also run in parallel:
Task T004 + T005 (US1) | Task T013 (US3) | Task T016 (US4)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational (3 tasks)
2. Complete Phase 2: User Story 1 (6 tasks)
3. **STOP and VALIDATE**: Budget footer appears in TUI, async loading works
4. Run `make lint` and `make test`

### Incremental Delivery

1. Phase 1 (Foundational) -- shared types ready
2. US1 (TUI Footer) -- interactive dashboard shows budget health (MVP)
3. US2 (TUI Detail) -- detail view shows per-budget breakdown
4. US3 (Plain Text) -- CI/CD pipelines get budget awareness
5. US4 (JSON Output) -- machine-readable budget data
6. Polish -- documentation, quality gates

### Parallel Execution Strategy

With one developer executing sequentially:

1. Phase 1 (3 tasks, parallel)
2. US1 (6 tasks, sequential within)
3. US2 (3 tasks, sequential within)
4. US3 (3 tasks, sequential within)
5. US4 (3 tasks, sequential within)
6. Polish (3 tasks)

Total: 21 tasks across 6 phases.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently testable at its checkpoint
- Budget data flow: TUI path uses `engine.GetBudgets()` goroutine; non-TTY path uses `evaluateBudgetStatus()` (per research Decision 1)
- Budget flags added independently to overview command (per research Decision 2 -- overview is top-level, not child of cost)
- Footer placement: between table/pagination and status bar (per research Decision 3)
- Mixed currencies: health badge only, no dollar aggregation (per research Decision 4)
- JSON budgets: top-level array in OverviewJSONOutput (per research Decision 6)
- NDJSON: no budget data (per research Decision 7)
