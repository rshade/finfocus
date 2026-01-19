# Tasks: Cost Recommendations Command Enhancement

**Input**: Design documents from `/specs/109-cost-recommendations/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md
**GitHub Issue**: #216

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## User Story Summary

| Story | Priority | Description |
|-------|----------|-------------|
| US1 | P1 | Quick Overview of Recommendations (summary mode, top 5) |
| US2 | P2 | View All Recommendations in Detail (--verbose flag) |
| US3 | P2 | Filter Recommendations by Action Type (existing, verify) |
| US4 | P3 | Interactive Exploration (Bubble Tea TUI) |
| US5 | P2 | Machine-Readable Output for Automation (JSON enhancement) |
| US6 | P4 | View Loading Progress (spinner during queries) |

---

## Phase 1: Setup

**Purpose**: Verify existing infrastructure and prepare for modifications

- [x] T001 Verify existing `internal/cli/cost_recommendations.go` has required base functionality
- [x] T002 Verify existing `internal/tui/` package has required components (LoadingState, styles, table)
- [x] T003 [P] Create test fixtures for recommendations in `test/fixtures/recommendations/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Create shared TUI components that multiple user stories depend on

**⚠️ CRITICAL**: TUI model and view files must exist before US4 and US6 can be implemented

### Tests for Foundational Components (MANDATORY - TDD Required) ⚠️

- [x] T004 [P] Create unit tests for RecommendationsSummary in `internal/tui/recommendations_model_test.go`
- [x] T005 [P] Create unit tests for RecommendationRow in `internal/tui/recommendations_view_test.go`

### Implementation for Foundational Components

- [x] T006 [P] Create `RecommendationsSummary` struct in `internal/tui/recommendations_model.go`
- [x] T007 [P] Create `NewRecommendationsSummary()` function in `internal/tui/recommendations_model.go`
- [x] T008 [P] Create `RecommendationSortField` enum in `internal/tui/recommendations_model.go`
- [x] T009 [P] Create `RecommendationRow` struct in `internal/tui/recommendations_view.go`
- [x] T010 [P] Create `NewRecommendationRow()` function in `internal/tui/recommendations_view.go`

**Checkpoint**: Foundation ready - summary/view components exist for US1, US2, US5 to use

---

## Phase 3: User Story 1 - Quick Overview of Recommendations (Priority: P1) 🎯 MVP

**Goal**: Default command shows summary with top 5 recommendations sorted by savings

**Independent Test**: Run `finfocus cost recommendations --pulumi-json plan.json` and verify summary section + top 5 recommendations sorted by savings descending

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

- [x] T011 [P] [US1] Unit test for `renderRecommendationsSummary()` in `internal/cli/cost_recommendations_test.go`
- [x] T012 [P] [US1] Unit test for sorting recommendations by savings in `internal/cli/cost_recommendations_test.go`
- [x] T013 [P] [US1] Integration test for summary output in `test/integration/recommendations_test.go`

### Implementation for User Story 1

- [x] T014 [US1] Implement `sortRecommendationsBySavings()` helper in `internal/cli/cost_recommendations.go`
- [x] T015 [US1] Implement `renderRecommendationsSummary()` function in `internal/cli/cost_recommendations.go`
- [x] T016 [US1] Update `renderRecommendationsTable()` to show only top 5 by default in `internal/cli/cost_recommendations.go`
- [x] T017 [US1] Update `RenderRecommendationsOutput()` to call summary renderer in `internal/cli/cost_recommendations.go`
- [x] T018 [US1] Handle edge case: 0 recommendations in `internal/cli/cost_recommendations.go`
- [x] T019 [US1] Handle edge case: fewer than 5 recommendations in `internal/cli/cost_recommendations.go`

**Checkpoint**: Summary mode works - users see top 5 by savings by default

---

## Phase 4: User Story 2 - View All Recommendations in Detail (Priority: P2)

**Goal**: `--verbose` flag shows all recommendations with full details

**Independent Test**: Run `finfocus cost recommendations --pulumi-json plan.json --verbose` and verify all recommendations display with full descriptions

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [x] T020 [P] [US2] Unit test for `--verbose` flag parsing in `internal/cli/cost_recommendations_test.go`
- [x] T021 [P] [US2] Unit test for verbose table rendering in `internal/cli/cost_recommendations_test.go`

### Implementation for User Story 2

- [x] T022 [US2] Add `verbose` field to `costRecommendationsParams` struct in `internal/cli/cost_recommendations.go`
- [x] T023 [US2] Add `--verbose` flag to `NewCostRecommendationsCmd()` in `internal/cli/cost_recommendations.go`
- [x] T024 [US2] Update `renderRecommendationsTable()` to show all recommendations when verbose in `internal/cli/cost_recommendations.go`
- [x] T025 [US2] Ensure full description shown without truncation in verbose mode in `internal/cli/cost_recommendations.go`

**Checkpoint**: Verbose mode works - users can see all recommendations with `--verbose`

---

## Phase 5: User Story 3 - Filter Recommendations by Action Type (Priority: P2)

**Goal**: Verify existing `--filter "action=TYPE"` works correctly with new summary/verbose modes

**Independent Test**: Run `finfocus cost recommendations --pulumi-json plan.json --filter "action=RIGHTSIZE"` and verify only RIGHTSIZE recommendations appear

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [x] T026 [P] [US3] Unit test for filter + summary mode interaction in `internal/cli/cost_recommendations_test.go`
- [x] T027 [P] [US3] Unit test for filter + verbose mode interaction in `internal/cli/cost_recommendations_test.go`

### Implementation for User Story 3

- [x] T028 [US3] Verify `applyActionTypeFilter()` applies before summary calculation in `internal/cli/cost_recommendations.go`
- [x] T029 [US3] Update summary statistics to reflect filtered results in `internal/cli/cost_recommendations.go`
- [x] T030 [US3] Add invalid action type error message enhancement in `internal/cli/cost_recommendations.go`

**Checkpoint**: Filtering works with both summary and verbose modes

---

## Phase 6: User Story 5 - Machine-Readable Output for Automation (Priority: P2)

**Goal**: JSON output includes summary object with breakdowns

**Independent Test**: Run `finfocus cost recommendations --pulumi-json plan.json --output json` and validate JSON structure includes summary

### Tests for User Story 5 (MANDATORY - TDD Required) ⚠️

- [x] T031 [P] [US5] Unit test for JSON summary structure in `internal/cli/cost_recommendations_test.go`
- [x] T032 [P] [US5] Unit test for NDJSON with summary line in `internal/cli/cost_recommendations_test.go`

### Implementation for User Story 5

- [x] T033 [US5] Add `jsonSummary` struct to `internal/cli/cost_recommendations.go`
- [x] T034 [US5] Update `recommendationsJSONOutput` to include Summary field in `internal/cli/cost_recommendations.go`
- [x] T035 [US5] Update `renderRecommendationsJSON()` to populate summary in `internal/cli/cost_recommendations.go`
- [x] T036 [US5] Update `renderRecommendationsNDJSON()` to emit summary as first line in `internal/cli/cost_recommendations.go`
- [x] T037 [US5] Add `count_by_action_type` breakdown to JSON summary in `internal/cli/cost_recommendations.go`
- [x] T038 [US5] Add `savings_by_action_type` breakdown to JSON summary in `internal/cli/cost_recommendations.go`

**Checkpoint**: JSON/NDJSON output includes summary - ready for CI/CD integration

---

## Phase 7: User Story 4 - Interactive Exploration (Priority: P3)

**Goal**: Full Bubble Tea TUI with table navigation, detail view, and filtering

**Independent Test**: Run command in TTY terminal and verify keyboard navigation (up/down), detail view (Enter), filter (/), quit (q)

### Tests for User Story 4 (MANDATORY - TDD Required) ⚠️

- [x] T039 [P] [US4] Unit test for RecommendationsViewModel state transitions in `internal/tui/recommendations_model_test.go`
- [x] T040 [P] [US4] Unit test for keyboard handlers in `internal/tui/recommendations_model_test.go`
- [x] T041 [P] [US4] Unit test for filter logic in `internal/tui/recommendations_model_test.go`
- [x] T042 [P] [US4] Unit test for table rendering in `internal/tui/recommendations_view_test.go`
- [x] T043 [P] [US4] Unit test for detail view rendering in `internal/tui/recommendations_view_test.go`

### Implementation for User Story 4

- [x] T044 [US4] Create `RecommendationsViewModel` struct in `internal/tui/recommendations_model.go`
- [x] T045 [US4] Implement `NewRecommendationsViewModel()` constructor in `internal/tui/recommendations_model.go`
- [x] T046 [US4] Implement `Init()` method for RecommendationsViewModel in `internal/tui/recommendations_model.go`
- [x] T047 [US4] Implement `Update()` method with state machine in `internal/tui/recommendations_model.go`
- [x] T048 [US4] Implement keyboard handlers (Enter, Esc, /, s, q) in `internal/tui/recommendations_model.go`
- [x] T049 [US4] Implement `applyFilter()` method in `internal/tui/recommendations_model.go`
- [x] T050 [US4] Implement `cycleSort()` method in `internal/tui/recommendations_model.go`
- [x] T051 [US4] Implement `View()` method in `internal/tui/recommendations_model.go`
- [x] T052 [P] [US4] Create `NewRecommendationsTable()` in `internal/tui/recommendations_view.go`
- [x] T053 [P] [US4] Create `RenderRecommendationsSummaryStyled()` in `internal/tui/recommendations_view.go`
- [x] T054 [P] [US4] Create `RenderRecommendationDetail()` in `internal/tui/recommendations_view.go`
- [x] T055 [US4] Add TTY detection to `executeCostRecommendations()` in `internal/cli/cost_recommendations.go`
- [x] T056 [US4] Add `--plain` flag to bypass interactive mode in `internal/cli/cost_recommendations.go`
- [x] T057 [US4] Add `--no-color` flag to disable styling in `internal/cli/cost_recommendations.go`
- [x] T058 [US4] Implement `runInteractiveRecommendations()` in `internal/cli/cost_recommendations.go`
- [x] T059 [US4] Route to interactive mode based on DetectOutputMode in `internal/cli/cost_recommendations.go`

**Checkpoint**: Interactive TUI works - users can navigate, filter, and view details

---

## Phase 8: User Story 6 - View Loading Progress (Priority: P4)

**Goal**: Loading spinner displays during plugin queries

**Independent Test**: Run command with slow plugins and verify spinner displays with status text

### Tests for User Story 6 (MANDATORY - TDD Required) ⚠️

- [x] T060 [P] [US6] Unit test for loading state in RecommendationsViewModel in `internal/tui/recommendations_model_test.go`
- [x] T061 [P] [US6] Unit test for loading view rendering in `internal/tui/recommendations_view_test.go`

### Implementation for User Story 6

- [x] T062 [US6] Add `NewRecommendationsViewModelWithLoading()` constructor in `internal/tui/recommendations_model.go`
- [x] T063 [US6] Implement `loadingCompleteMsg` handling in Update() in `internal/tui/recommendations_model.go`
- [x] T064 [US6] Create `RenderRecommendationsLoading()` in `internal/tui/recommendations_view.go`
- [x] T065 [US6] Integrate loading state into `runInteractiveRecommendations()` in `internal/cli/cost_recommendations.go`

**Checkpoint**: Loading spinner provides feedback during plugin queries

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T066 [P] Update CLI help text and examples in `internal/cli/cost_recommendations.go`
- [x] T067 [P] Ensure 80% test coverage for `internal/cli/cost_recommendations.go`
- [x] T068 [P] Ensure 80% test coverage for `internal/tui/recommendations_model.go`
- [x] T069 [P] Ensure 80% test coverage for `internal/tui/recommendations_view.go`
- [x] T070 Run `make lint` and fix any issues
- [x] T071 Run `make test` and verify all tests pass
- [x] T072 Validate quickstart.md scenarios manually
- [x] T073 Test cross-platform behavior (Linux, macOS, Windows if available)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup - creates shared TUI components
- **User Stories (Phases 3-8)**: All depend on Foundational phase completion
  - US1 (P1): Can start immediately after Foundational
  - US2 (P2): Can run parallel to US1 (different functionality)
  - US3 (P2): Depends on US1 (needs summary calculation logic)
  - US5 (P2): Can run parallel to US1, US2 (different output path)
  - US4 (P3): Depends on Foundational TUI components
  - US6 (P4): Depends on US4 (needs TUI model)
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

```text
Foundational ──┬──> US1 (P1) ──> US3 (P2)
               │
               ├──> US2 (P2)
               │
               ├──> US5 (P2)
               │
               └──> US4 (P3) ──> US6 (P4)
```

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks in dependency order
- Story complete before moving to next priority

### Parallel Opportunities

**Phase 2 (Foundational)**: T004-T010 all [P] - can run in parallel

**User Stories**: After Foundational:

- US1, US2, US5 can run in parallel (different code paths)
- US4 can run parallel to US1, US2, US3, US5 (different files)

**Within US4**: T039-T043 and T052-T054 all [P] - can run in parallel

---

## Parallel Example: User Story 4

```bash
# Launch all tests for User Story 4 together:
Task: "Unit test for RecommendationsViewModel state transitions"
Task: "Unit test for keyboard handlers"
Task: "Unit test for filter logic"
Task: "Unit test for table rendering"
Task: "Unit test for detail view rendering"

# Launch parallelizable view implementations together:
Task: "Create NewRecommendationsTable()"
Task: "Create RenderRecommendationsSummaryStyled()"
Task: "Create RenderRecommendationDetail()"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Run `finfocus cost recommendations --pulumi-json plan.json` - should show summary + top 5
5. Deploy/demo if ready

### Incremental Delivery

1. **MVP**: Setup + Foundational + US1 → Summary mode works
2. **+US2**: Add verbose mode → Users can see all recommendations
3. **+US3**: Verify filtering → Users can filter by action type
4. **+US5**: Enhance JSON → CI/CD integration ready
5. **+US4**: Add TUI → Interactive exploration
6. **+US6**: Add spinner → Polish complete

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (P1) → US3 (P2)
   - Developer B: US2 (P2) → US5 (P2)
   - Developer C: US4 (P3) → US6 (P4)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Run `make lint` and `make test` after each phase
- Commit after each task or logical group
