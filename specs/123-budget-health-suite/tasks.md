# Tasks: Budget Health Suite

**Input**: Design documents from `/specs/123-budget-health-suite/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/budget-api.go, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV, documentation (README, docs/) MUST be updated concurrently with implementation to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Go package**: `internal/engine/` for all budget health functionality
- **Tests**: Co-located `*_test.go` files per Go convention
- **Integration tests**: `test/integration/` directory

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and shared type definitions

- [x] T001 Define BudgetFilterOptions type in `internal/engine/budget.go`
- [x] T002 Define BudgetResult type in `internal/engine/budget.go`
- [x] T003 Define ExtendedBudgetSummary type in `internal/engine/budget.go`
- [x] T004 Define BudgetHealthResult type in `internal/engine/budget.go`
- [x] T005 Define ThresholdEvaluationResult type in `internal/engine/budget.go`
- [x] T006 [P] Define health threshold constants in `internal/engine/budget_health.go`
- [x] T007 [P] Define currency validation regex constant in `internal/engine/budget.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] Write tests for ValidateCurrency in `internal/engine/budget_test.go`
- [x] T009 [P] Write tests for ValidateBudgetCurrency in `internal/engine/budget_test.go`

### Implementation for Foundational

- [x] T010 Implement ValidateCurrency function (FR-003) in `internal/engine/budget.go`
- [x] T011 Implement ValidateBudgetCurrency function in `internal/engine/budget.go`

**Checkpoint**: Currency validation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - View Budget Health Overview (Priority: P1) 🎯 MVP

**Goal**: Enable users to see health status (OK/WARNING/CRITICAL/EXCEEDED) for all budgets based on utilization percentage

**Independent Test**: Query budgets and verify correct health status based on utilization thresholds (0-79% OK, 80-89% WARNING, 90-99% CRITICAL, 100%+ EXCEEDED)

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T012 [P] [US1] Write table-driven tests for CalculateBudgetHealthFromPercentage in `internal/engine/budget_health_test.go`
- [x] T013 [P] [US1] Write tests for CalculateBudgetHealth with valid budget in `internal/engine/budget_health_test.go`
- [x] T014 [P] [US1] Write tests for CalculateBudgetHealth with nil/missing status in `internal/engine/budget_health_test.go`
- [x] T015 [P] [US1] Write tests for CalculateBudgetHealth with zero limit (edge case) in `internal/engine/budget_health_test.go`
- [x] T016 [P] [US1] Write tests for AggregateHealth with multiple budgets in `internal/engine/budget_health_test.go`
- [x] T017 [P] [US1] Write tests for AggregateHealth with empty slice in `internal/engine/budget_health_test.go`

### Implementation for User Story 1

- [x] T018 [US1] Implement CalculateBudgetHealthFromPercentage (FR-001) in `internal/engine/budget_health.go`
- [x] T019 [US1] Implement CalculateBudgetHealth using CalculateBudgetHealthFromPercentage in `internal/engine/budget_health.go`
- [x] T020 [US1] Implement AggregateHealth for worst-case aggregation (FR-008) in `internal/engine/budget_health.go`
- [x] T021 [US1] Add logging for health calculation in `internal/engine/budget_health.go`

**Checkpoint**: Health status calculation fully functional and independently testable

---

## Phase 4: User Story 2 - Filter Budgets by Provider (Priority: P2)

**Goal**: Enable users to filter budgets by cloud provider with case-insensitive matching

**Independent Test**: Apply provider filters (single and multiple) and verify only matching budgets are returned

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [x] T022 [P] [US2] Write tests for MatchesProvider with case variations in `internal/engine/budget_test.go`
- [x] T023 [P] [US2] Write tests for FilterBudgetsByProvider with single provider in `internal/engine/budget_test.go`
- [x] T024 [P] [US2] Write tests for FilterBudgetsByProvider with multiple providers (OR logic) in `internal/engine/budget_test.go`
- [x] T025 [P] [US2] Write tests for FilterBudgetsByProvider with empty filter (returns all) in `internal/engine/budget_test.go`
- [x] T026 [P] [US2] Write tests for FilterBudgetsByProvider with no matches in `internal/engine/budget_test.go`

### Implementation for User Story 2

- [x] T027 [US2] Implement MatchesProvider with case-insensitive comparison in `internal/engine/budget.go`
- [x] T028 [US2] Implement FilterBudgetsByProvider using MatchesProvider (FR-002, FR-009) in `internal/engine/budget.go`
- [x] T029 [US2] Add logging for provider filtering in `internal/engine/budget.go`

**Checkpoint**: Provider filtering fully functional and independently testable

---

## Phase 5: User Story 3 - Budget Summary Statistics (Priority: P2)

**Goal**: Aggregate budget counts by health status for executive reporting

**Independent Test**: Calculate summary for budgets with various health statuses and verify counts sum to total

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [x] T030 [P] [US3] Write tests for CalculateBudgetSummary with mixed health statuses in `internal/engine/budget_summary_test.go`
- [x] T031 [P] [US3] Write tests for CalculateBudgetSummary with empty input in `internal/engine/budget_summary_test.go`
- [x] T032 [P] [US3] Write tests for CalculateExtendedSummary by-provider breakdown in `internal/engine/budget_summary_test.go`
- [x] T033 [P] [US3] Write tests for CalculateExtendedSummary by-currency breakdown in `internal/engine/budget_summary_test.go`
- [x] T034 [P] [US3] Write tests for CalculateExtendedSummary critical budgets list in `internal/engine/budget_summary_test.go`

### Implementation for User Story 3

- [x] T035 [US3] Implement CalculateBudgetSummary (FR-004) in `internal/engine/budget_summary.go`
- [x] T036 [US3] Implement CalculateExtendedSummary with by-provider breakdown in `internal/engine/budget_summary.go`
- [x] T037 [US3] Add by-currency breakdown to CalculateExtendedSummary in `internal/engine/budget_summary.go`
- [x] T038 [US3] Add critical budgets identification to CalculateExtendedSummary in `internal/engine/budget_summary.go`
- [x] T039 [US3] Add logging for summary aggregation in `internal/engine/budget_summary.go`

**Checkpoint**: Summary statistics fully functional and independently testable

---

## Phase 6: User Story 4 - Threshold Alerting (Priority: P3)

**Goal**: Evaluate budget thresholds and identify which have been triggered

**Independent Test**: Evaluate thresholds against current/forecasted spend and verify correct triggered status

### Tests for User Story 4 (MANDATORY - TDD Required) ⚠️

- [x] T040 [P] [US4] Write tests for DefaultThresholds returning 50/80/100 in `internal/engine/budget_threshold_test.go`
- [x] T041 [P] [US4] Write tests for ApplyDefaultThresholds when none exist in `internal/engine/budget_threshold_test.go`
- [x] T042 [P] [US4] Write tests for ApplyDefaultThresholds when thresholds exist (no-op) in `internal/engine/budget_threshold_test.go`
- [x] T043 [P] [US4] Write tests for EvaluateThresholds with ACTUAL type in `internal/engine/budget_threshold_test.go`
- [x] T044 [P] [US4] Write tests for EvaluateThresholds with FORECASTED type in `internal/engine/budget_threshold_test.go`
- [x] T045 [P] [US4] Write tests for EvaluateThresholds timestamp tracking (FR-010) in `internal/engine/budget_threshold_test.go`

### Implementation for User Story 4

- [x] T046 [US4] Implement DefaultThresholds (FR-007) in `internal/engine/budget_threshold.go`
- [x] T047 [US4] Implement ApplyDefaultThresholds in `internal/engine/budget_threshold.go`
- [x] T048 [US4] Implement EvaluateThresholds for ACTUAL type (FR-005) in `internal/engine/budget_threshold.go`
- [x] T049 [US4] Add FORECASTED type support to EvaluateThresholds in `internal/engine/budget_threshold.go`
- [x] T050 [US4] Add triggered timestamp tracking (FR-010) in `internal/engine/budget_threshold.go`
- [x] T051 [US4] Add logging for threshold evaluation in `internal/engine/budget_threshold.go`

**Checkpoint**: Threshold alerting fully functional and independently testable

---

## Phase 7: User Story 5 - Forecasted Spending (Priority: P3)

**Goal**: Calculate forecasted end-of-period spending using linear extrapolation

**Independent Test**: Calculate forecast from current spend and period dates, verify linear extrapolation formula

### Tests for User Story 5 (MANDATORY - TDD Required) ⚠️

- [x] T052 [P] [US5] Write tests for CalculateForecastedSpend with mid-period spend in `internal/engine/budget_forecast_test.go`
- [x] T053 [P] [US5] Write tests for CalculateForecastedSpend with period not started in `internal/engine/budget_forecast_test.go`
- [x] T054 [P] [US5] Write tests for CalculateForecastedSpend with zero current spend in `internal/engine/budget_forecast_test.go`
- [x] T055 [P] [US5] Write tests for CalculateForecastedPercentage in `internal/engine/budget_forecast_test.go`
- [x] T056 [P] [US5] Write tests for CalculateForecastedPercentage with zero limit in `internal/engine/budget_forecast_test.go`
- [x] T057 [P] [US5] Write tests for UpdateBudgetForecast in `internal/engine/budget_forecast_test.go`

### Implementation for User Story 5

- [x] T058 [US5] Implement CalculateForecastedSpend (FR-006) in `internal/engine/budget_forecast.go`
- [x] T059 [US5] Implement CalculateForecastedPercentage in `internal/engine/budget_forecast.go`
- [x] T060 [US5] Implement UpdateBudgetForecast in `internal/engine/budget_forecast.go`
- [x] T061 [US5] Add logging for forecast calculation in `internal/engine/budget_forecast.go`

**Checkpoint**: Forecasting fully functional and independently testable

---

## Phase 8: Engine Integration

**Purpose**: Integrate all user story components into the main Engine.GetBudgets method

### Tests for Engine Integration

- [x] T062 [P] Write tests for Engine.GetBudgets with no filter in `internal/engine/budget_test.go`
- [x] T063 [P] Write tests for Engine.GetBudgets with provider filter in `internal/engine/budget_test.go`
- [x] T064 [P] Write tests for Engine.GetBudgets error handling in `internal/engine/budget_test.go`
- [x] T065 Write integration test for full budget health flow in `test/integration/budget_health_test.go`

### Implementation for Engine Integration

- [x] T066 Implement Engine.GetBudgets orchestration in `internal/engine/budget.go`
- [x] T067 Add plugin query integration to GetBudgets in `internal/engine/budget.go`
- [x] T068 Integrate provider filtering into GetBudgets in `internal/engine/budget.go`
- [x] T069 Integrate health calculation into GetBudgets in `internal/engine/budget.go`
- [x] T070 Integrate threshold evaluation into GetBudgets in `internal/engine/budget.go`
- [x] T071 Integrate forecast calculation into GetBudgets in `internal/engine/budget.go`
- [x] T072 Integrate summary calculation into GetBudgets in `internal/engine/budget.go`
- [x] T073 Add error aggregation to GetBudgets in `internal/engine/budget.go`

**Checkpoint**: Full engine integration complete - all user stories work together

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and final validation

- [x] T074 [P] Update `internal/engine/CLAUDE.md` with budget health patterns
- [x] T075 [P] Create `docs/guides/budget-health.md` user guide per Constitution Principle IV
- [x] T076 [P] Run `make lint` and fix any issues
- [x] T077 [P] Run `make test` and verify 80% coverage minimum
- [x] T078 Validate quickstart.md examples compile correctly
- [x] T079 Run performance benchmarks (1000 budgets < 100ms health, < 500ms filtering)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phases 3-7)**: All depend on Foundational phase completion
  - US1 (P1): Can start immediately after Foundational
  - US2 (P2): Can start immediately after Foundational (parallel with US1)
  - US3 (P2): Can start immediately after Foundational (parallel with US1/US2)
  - US4 (P3): Can start after Foundational (parallel with US1/US2/US3)
  - US5 (P3): Can start after Foundational (parallel with all others)
- **Engine Integration (Phase 8)**: Depends on ALL user stories being complete
- **Polish (Phase 9)**: Depends on Engine Integration being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent - no dependencies on other stories
- **User Story 2 (P2)**: Independent - no dependencies on other stories
- **User Story 3 (P2)**: Uses health calculation from US1 but can be tested independently
- **User Story 4 (P3)**: Independent - no dependencies on other stories
- **User Story 5 (P3)**: Independent - no dependencies on other stories

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation follows test completion
- Story complete when all tests pass

### Parallel Opportunities

- Setup tasks T006-T007 marked [P] can run in parallel (type definitions in different files)
- Foundational test tasks T008-T009 marked [P] can run in parallel
- All tests within a user story marked [P] can run in parallel
- Different user stories (Phases 3-7) can be worked on in parallel by different developers
- All Phase 9 tasks marked [P] can run in parallel (T074-T077)

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "T012 [P] [US1] Write table-driven tests for CalculateBudgetHealthFromPercentage"
Task: "T013 [P] [US1] Write tests for CalculateBudgetHealth with valid budget"
Task: "T014 [P] [US1] Write tests for CalculateBudgetHealth with nil/missing status"
Task: "T015 [P] [US1] Write tests for CalculateBudgetHealth with zero limit (edge case)"
Task: "T016 [P] [US1] Write tests for AggregateHealth with multiple budgets"
Task: "T017 [P] [US1] Write tests for AggregateHealth with empty slice"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (currency validation)
3. Complete Phase 3: User Story 1 (health calculation)
4. **STOP and VALIDATE**: Test health calculation independently
5. Proceed to integration if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently (MVP!)
3. Add User Story 2 → Test independently (filtering)
4. Add User Story 3 → Test independently (summary)
5. Add User Story 4 → Test independently (thresholds)
6. Add User Story 5 → Test independently (forecasting)
7. Complete Engine Integration → Full feature
8. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (health)
   - Developer B: User Story 2 (filtering)
   - Developer C: User Story 3 (summary)
3. After P1/P2 stories:
   - Developer A: User Story 4 (thresholds)
   - Developer B: User Story 5 (forecasting)
   - Developer C: Engine Integration prep
4. Final: Engine Integration and Polish

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All functions must follow contracts defined in `specs/123-budget-health-suite/contracts/budget-api.go`
