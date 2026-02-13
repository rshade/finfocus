# Tasks: Unified Cost Overview Dashboard

**Input**: Design documents from `/specs/509-overview-command/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: Go CLI tool, monorepo structure
- Source code: `internal/` (cli, engine, tui packages)
- Test data: `testdata/overview/`
- Documentation: `docs/commands/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create directory structure for test fixtures and golden files

- [x] T001 Create testdata/overview/ and testdata/overview/golden/ directory structure

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core data types, merge logic, and cost drift calculation that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T002 Define ResourceStatus enum (Active/Creating/Updating/Deleting/Replacing) with String() method, OverviewRow, ActualCostData, ProjectedCostData, CostDriftData, OverviewRowError, ErrorType enum, DateRange, and StackContext types with Validate() methods in internal/engine/overview_types.go
- [x] T003 [P] Write table-driven validation tests for all core types (OverviewRow, ActualCostData, ProjectedCostData, CostDriftData, DateRange, StackContext, OverviewRowError) in internal/engine/overview_types_test.go
- [x] T004 [P] Implement MergeResourcesForOverview (state + plan merge preserving state file order per FR-011), MapOperationToStatus (create/update/delete/replace/same mapping), and DetectPendingChanges (scan plan for change operations per FR-008) in internal/engine/overview_merge.go
- [x] T005 Write table-driven tests for MergeResourcesForOverview (empty state/plan, no changes, all operation types, new resources in plan not in state, mixed scenarios) and DetectPendingChanges in internal/engine/overview_merge_test.go
- [x] T006 [P] Implement CalculateCostDrift (extrapolate MTD to monthly, 10% threshold per FR-004, day 1-2 insufficient data, deleted/new resource edge cases) and CalculateProjectedDelta (net monthly cost change for all rows) in internal/engine/overview_drift.go
- [x] T007 Write table-driven tests for CalculateCostDrift (threshold boundary at 10%, day 1-2 error, zero values, deleted resources, new resources, extrapolation accuracy) and CalculateProjectedDelta (additions, deletions, mixed changes) in internal/engine/overview_drift_test.go

**Checkpoint**: Foundation ready - all core types compiled, merge and drift logic tested at 95%+ coverage

---

## Phase 3: User Story 1 - Unified Stack Dashboard (Priority: P1) MVP

**Goal**: Users can run `finfocus overview` and see a complete cost profile (actual + projected + recommendations) for all resources in a single view

**Independent Test**: Run `finfocus overview --pulumi-state state.json --pulumi-json plan.json --plain --yes` and verify unified table output with actual costs, projected costs, deltas, and recommendation counts

### Tests for User Story 1 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] [US1] Write enrichment unit tests with mock plugins (single resource enrichment, partial failure populates row.Error, concurrent enrichment of 100 resources, progress channel updates) in internal/engine/overview_enrich_test.go
- [x] T009 [P] [US1] Write table rendering tests with golden file comparison (column alignment, currency formatting, status icons, summary footer, empty rows, error indicators) in internal/engine/overview_render_test.go
- [x] T010 [P] [US1] Write CLI unit tests for flag parsing (--pulumi-json, --pulumi-state, --from, --to, --adapter, --output, --filter, --plain, --yes, --no-pagination), input validation (invalid dates, missing files), and error message formatting in internal/cli/overview_test.go

### Implementation for User Story 1

- [x] T011 [US1] Implement EnrichOverviewRow (call plugin GetActualCost if resource exists in state, GetProjectedCost if pending changes, GetRecommendations, calculate CostDrift when both actual and projected exist, populate row.Error on failure) in internal/engine/overview_enrich.go
- [x] T012 [US1] Implement EnrichOverviewRows with concurrency (semaphore channel limited to 10 goroutines, sync.WaitGroup, progress channel sending OverviewRowUpdate for progressive loading) in internal/engine/overview_enrich.go
- [x] T013 [US1] Implement RenderOverviewAsTable with fixed-width columns (Resource 30 chars, Type 20 chars, Status 10 chars with icons, Actual/Projected/Delta 12 chars right-aligned, Drift% 8 chars, Recs 4 chars), currency formatting ($X,XXX.XX), +/- delta signs, drift warning indicator at >10%, and summary footer (Total Actual MTD, Projected Monthly, Projected Delta, Potential Savings) in internal/engine/overview_render.go
- [x] T014 [US1] Implement NewOverviewCmd with all flags per cli-interface.md contract (--pulumi-json string, --pulumi-state string, --from/--to string with YYYY-MM-DD and RFC3339 parsing, --adapter string, --output string default table, --filter []string, --plain bool, --yes/-y bool, --no-pagination bool) in internal/cli/overview.go
- [x] T015 [US1] Implement executeOverview pipeline: load Pulumi state and preview, detect pending changes (skip projected pipeline if none per FR-008), display pre-flight confirmation prompt (resource count, plugin count, estimated API calls) unless --yes, merge resources, open plugins, enrich rows concurrently, render output based on --output flag in internal/cli/overview.go
- [x] T016 [US1] Register NewOverviewCmd() as top-level subcommand in internal/cli/root.go

**Checkpoint**: `finfocus overview --plain --yes` produces a unified ASCII table with actual costs, projected costs, deltas, drift%, and recommendation counts for all resources

---

## Phase 4: User Story 2 - Progressive Data Loading (Priority: P2)

**Goal**: Dashboard opens immediately and shows data as it's fetched with a persistent progress banner, rather than waiting for all API calls to finish

**Independent Test**: Run `finfocus overview` on a stack with many resources and verify the TUI appears instantly with a progress banner ("Loading: X/Y resources") that updates as data arrives and disappears when complete

### Tests for User Story 2 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T017 [P] [US2] Write TUI model tests for state transitions (ViewStateLoading to ViewStateList on allResourcesLoadedMsg), resourceLoadedMsg row updates, loadingProgressMsg banner updates, keyboard navigation (up/down/j/k), sort cycling (s key through cost/name/type/delta), filter mode (/ to enter, Esc to exit, text matching), pagination (PgUp/PgDn at boundaries), and quit (q/Ctrl+C) in internal/tui/overview_model_test.go

### Implementation for User Story 2

- [x] T018 [US2] Implement OverviewModel struct (ViewState enum with Loading/List/Detail/Error/Quitting, SortField enum with Cost/Name/Type/Delta), message types (resourceLoadedMsg, loadingProgressMsg, allResourcesLoadedMsg), and NewOverviewModel (initialize table columns, textinput, set ViewStateLoading, return Cmd to start loading) in internal/tui/overview_model.go
- [x] T019 [US2] Implement Update method handling resourceLoadedMsg (update row in allRows, increment loadedCount), loadingProgressMsg (update progress banner text), allResourcesLoadedMsg (transition to ViewStateList, hide banner), tea.WindowSizeMsg (update width/height, resize table), and tea.KeyMsg dispatching in internal/tui/overview_model.go
- [x] T020 [US2] Implement sorting (cycleSortField cycling through Cost/Name/Type/Delta, refreshTable re-sorting rows with sort.Slice, getCost/getDelta helpers) and filtering (applyFilter with case-insensitive URN and Type substring matching, showFilter toggle, textinput Focus/Blur) in internal/tui/overview_model.go
- [x] T021 [US2] Implement pagination with maxResourcesPerPage=250, enablePaginationIfNeeded (auto-enable when >250 rows per FR-009), getVisibleRows (page slice), PgUp/PgDn handlers, and renderPaginationFooter ("Page X/Y | Use PgUp/PgDn to navigate") in internal/tui/overview_model.go
- [x] T022 [US2] Implement View method rendering progress banner (lipgloss styled "Loading: X/Y resources (Z%)"), table with visible rows, filter input bar (when showFilter=true), pagination footer (when enabled), and status bar with sort/filter indicators in internal/tui/overview_view.go
- [x] T023 [US2] Integrate TUI launch with CLI command: add TTY detection via term.IsTerminal(os.Stdout.Fd()), launch tea.NewProgram(NewOverviewModel(...)) when interactive, bridge enrichment progress channel to Bubble Tea Cmd for progressive updates in internal/cli/overview.go

**Checkpoint**: `finfocus overview` launches interactive TUI with progress banner that updates per-resource, table supports sorting (s), filtering (/), and pagination for >250 resources

---

## Phase 5: User Story 3 - Resource Cost Drill-down (Priority: P2)

**Goal**: Users can select a resource from the overview table and see a detailed breakdown of costs and specific optimization recommendations

**Independent Test**: Navigate to a resource in the TUI table, press Enter, verify detail view shows actual cost breakdown (compute/storage/network), projected cost breakdown, and list of recommendations with savings amounts. Press Escape to return to main table with position preserved.

### Tests for User Story 3 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T024 [P] [US3] Write detail view rendering tests (metadata display, actual cost breakdown formatting, projected cost breakdown formatting, recommendations list with savings, empty states for nil fields, Escape footer text) and state transition tests (Enter opens detail, Escape returns to list with cursor position preserved) in internal/tui/overview_model_test.go

### Implementation for User Story 3

- [x] T025 [US3] Implement DetailViewModel struct (row OverviewRow, width/height int) and renderDetailView function (lipgloss-styled header "Resource Detail", metadata section with URN/Type/Status, actual cost section with MTD total and breakdown map iteration, projected cost section with monthly total and breakdown, recommendations numbered list with description and savings, "Press ESC to return" footer) in internal/tui/overview_detail.go
- [x] T026 [US3] Add Enter key handler in Update to create DetailViewModel from selected table row and transition to ViewStateDetail, add Escape key handler to transition back to ViewStateList with nil detailView and preserved cursor position, update View to delegate to renderDetailView when in ViewStateDetail in internal/tui/overview_model.go

**Checkpoint**: TUI detail view shows full cost breakdown and recommendations for selected resource, Escape returns to main table

---

## Phase 6: User Story 4 - Non-Interactive Overview (Priority: P3)

**Goal**: Users can get a unified cost overview in non-interactive environments (CI/CD pipelines, piped output) with JSON/NDJSON/table formats

**Independent Test**: Run `finfocus overview --output json --yes` and verify valid JSON output matching schema in output-format.md. Run with `--plain` flag and verify ASCII table output.

### Tests for User Story 4 (MANDATORY - TDD Required)

> **CONSTITUTION REQUIREMENT: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T027 [P] [US4] Write JSON output tests (metadata fields, resources array, summary totals, currency consistency), NDJSON tests (one valid JSON per line, all rows present, no metadata wrapper), and non-interactive mode tests (--plain forces table, non-TTY auto-detects plain, exit codes 0/1/2/130) in internal/engine/overview_render_test.go

### Implementation for User Story 4

- [x] T028 [US4] Implement RenderOverviewAsJSON (JSON object with metadata StackContext, resources []OverviewRow with camelCase field names, summary with totalActualMTD/projectedMonthly/projectedDelta/potentialSavings/currency, errors array) using encoding/json with proper struct tags in internal/engine/overview_render.go
- [x] T029 [US4] Implement RenderOverviewAsNDJSON (iterate rows, json.Marshal each OverviewRow, write one per line with newline delimiter, no metadata wrapper) in internal/engine/overview_render.go
- [x] T030 [US4] Add --output format routing in executeOverview (table/json/ndjson switch), --plain flag forces non-interactive even with TTY, non-TTY auto-defaults to plain mode, exit code 2 on user cancellation of pre-flight prompt in internal/cli/overview.go

**Checkpoint**: `finfocus overview --output json --yes` produces valid JSON. Piping output auto-detects non-interactive mode.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Integration testing, test fixtures, documentation, and final validation

- [x] T031 [P] Create test fixture state-no-changes.json (10 resources with URNs, types, resource IDs, no pending updates) in testdata/overview/
- [x] T032 [P] Create test fixture state-mixed-changes.json (50 resources, 10 with pending updates across all operation types) in testdata/overview/
- [x] T033 [P] Create test fixture plan-no-changes.json (valid Pulumi preview with empty steps array) in testdata/overview/
- [x] T034 [P] Create test fixture plan-mixed-changes.json (Pulumi preview with create, update, delete, replace operations matching URNs in state-mixed-changes.json) in testdata/overview/
- [x] T035 Create golden files table-no-changes.txt and table-with-changes.txt (expected ASCII table output for fixture data) in testdata/overview/golden/
- [x] T036 Write end-to-end integration tests (full command execution with fixture files, verify exit codes, JSON/table output matches golden files, error scenarios for missing files and invalid dates, optimization path with no changes detected) in internal/cli/overview_integration_test.go
- [x] T037 [P] Create command documentation with full reference (usage, all flags with defaults, examples for interactive/plain/JSON/NDJSON/filter/date-range modes, troubleshooting tips) in docs/commands/overview.md
- [x] T038 Update README.md with "Unified Cost Overview" section including usage example and brief description
- [x] T039 Run `make lint` and `make test` to verify all quality gates pass (80% minimum coverage, 95% critical paths, zero lint warnings)
- [x] T040 Run benchmarks via `go test -bench=. -benchmem ./internal/engine/` and verify performance targets (MergeResourcesForOverview <5ms/100 resources, CalculateCostDrift <10ms/1000 calculations, RenderOverviewAsTable <50ms/250 resources)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup (T001) - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational (Phase 2) completion
- **User Story 2 (Phase 4)**: Depends on US1 (Phase 3) for CLI command and enrichment pipeline
- **User Story 3 (Phase 5)**: Depends on US2 (Phase 4) for TUI model infrastructure
- **User Story 4 (Phase 6)**: Depends on US1 (Phase 3) for render infrastructure; independent of US2/US3
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Depends on US1 for CLI command (overview.go) and enrichment pipeline (overview_enrich.go)
- **User Story 3 (P2)**: Depends on US2 for TUI model (overview_model.go) - cannot add detail view without base TUI
- **User Story 4 (P3)**: Depends on US1 for render infrastructure (overview_render.go) - can be parallel with US2/US3

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD)
- Types/models before services/logic
- Core implementation before integration points
- Story complete before moving to next priority

### Parallel Opportunities

**Phase 2 (after T002 completes)**:

- T003 (type tests), T004 (merge impl), T006 (drift impl) can all run in parallel

**Phase 3 (US1 tests)**:

- T008, T009, T010 can all run in parallel (different test files)

**Phase 4-6 (between stories)**:

- US4 (Phase 6) can run in parallel with US2 (Phase 4) and US3 (Phase 5) since it modifies overview_render.go and cli output routing independently of TUI work

**Phase 7 (fixtures)**:

- T031, T032, T033, T034 can all run in parallel (different fixture files)
- T037 (docs) can run in parallel with T031-T036 (fixtures/tests)

---

## Parallel Example: User Story 1

```text
# Launch all US1 tests in parallel (TDD - write first, expect failures):
Task: T008 "Write enrichment tests in internal/engine/overview_enrich_test.go"
Task: T009 "Write render tests in internal/engine/overview_render_test.go"
Task: T010 "Write CLI tests in internal/cli/overview_test.go"

# Then implement sequentially (tests should pass after each):
Task: T011 "Implement EnrichOverviewRow in internal/engine/overview_enrich.go"
Task: T012 "Implement EnrichOverviewRows in internal/engine/overview_enrich.go"
Task: T013 "Implement RenderOverviewAsTable in internal/engine/overview_render.go"
Task: T014 "Implement NewOverviewCmd in internal/cli/overview.go"
Task: T015 "Implement executeOverview in internal/cli/overview.go"
Task: T016 "Register command in internal/cli/root.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002-T007) - CRITICAL blocking phase
3. Complete Phase 3: User Story 1 (T008-T016)
4. **STOP and VALIDATE**: Run `finfocus overview --plain --yes --pulumi-state state.json --pulumi-json plan.json` and verify unified table output
5. Deploy/demo if ready - MVP delivers unified view in plain table format

### Incremental Delivery

1. Setup + Foundational -> Foundation ready (types, merge, drift)
2. Add User Story 1 -> Test independently -> Deploy/Demo (**MVP!** - plain table output)
3. Add User Story 2 -> Test independently -> Deploy/Demo (interactive TUI with progressive loading)
4. Add User Story 3 -> Test independently -> Deploy/Demo (resource drill-down)
5. Add User Story 4 -> Test independently -> Deploy/Demo (JSON/NDJSON for CI/CD)
6. Polish phase -> Final integration tests, docs, benchmarks

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (CLI + enrichment + table render)
   - Developer B: Can start US4 after US1 render infrastructure exists
3. After US1 completes:
   - Developer A: User Story 2 (TUI) -> User Story 3 (detail view)
   - Developer B: User Story 4 (JSON/NDJSON, non-interactive polish)
4. Both contribute to Polish phase

---

## Notes

- [P] tasks = different files, no dependencies on other [P] tasks in same phase
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (TDD)
- Run `make lint` and `make test` after each phase completion
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence
- Performance targets from research.md: merge <5ms, drift <10ms, render <50ms, TUI update <16ms
