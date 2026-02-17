# Tasks: TUI Immediate Launch with Phase Progress Feedback

**Input**: Design documents from `/specs/596-tui-phase-progress/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Blocking Prerequisites)

**Purpose**: Add the new ViewState constant and message types that all user stories depend on

- [X] T001 Add `ViewStateInitializing` as first iota constant in `internal/tui/cost_model.go` — insert before `ViewStateLoading` so the enum becomes: Initializing(0), Loading(1), List(2), Detail(3), Quitting(4), Error(5). Add godoc comment: "ViewStateInitializing indicates the TUI is shown before data is available."
- [X] T002 Add three new Bubble Tea message types in `internal/tui/overview_model.go` — `OverviewPhaseMsg{Phase string}`, `OverviewDataReadyMsg{Rows []engine.OverviewRow, TotalCount int}`, `OverviewInitErrorMsg{Err error}`. Add godoc comments per the data-model.md definitions. Place alongside existing message types (after line 33).
- [X] T003 Update `NewOverviewModel()` in `internal/tui/overview_model.go` — when `skeletonRows` is nil, set initial state to `ViewStateInitializing` instead of `ViewStateLoading`, initialize `allRows` as empty slice `[]engine.OverviewRow{}`, and set `totalCount` to 0. When non-nil, preserve existing behavior (state = `ViewStateLoading`).

**Checkpoint**: Foundation ready — new constant and types compile, existing tests still pass (`go test ./internal/tui/...`)

---

## Phase 2: User Story 1 - Immediate Visual Feedback on Launch (Priority: P1)

**Goal**: TUI appears within 1 second with spinner and phase messages, transitions seamlessly to enrichment

**Independent Test**: Launch `finfocus cost overview` on any stack; TUI renders immediately with spinner before data loads

### Tests for User Story 1 (MANDATORY - TDD Required)

- [X] T004 [P] [US1] Write test `TestOverviewModel_PhaseMsg` in `internal/tui/overview_model_test.go` — create model with `NewOverviewModel(ctx, nil, 0)`, assert initial state is `ViewStateInitializing`, send `OverviewPhaseMsg{Phase: "Loading stack state..."}`, assert `progressMsg` equals "Loading stack state..."
- [X] T005 [P] [US1] Write test `TestOverviewModel_DataReadyMsg` in `internal/tui/overview_model_test.go` — create model with nil rows (state = `ViewStateInitializing`), send `OverviewDataReadyMsg{Rows: testRows, TotalCount: 3}`, assert state transitions to `ViewStateLoading`, assert `allRows` has 3 elements, assert `totalCount` is 3, assert table is rebuilt
- [X] T006 [P] [US1] Write test `TestOverviewModel_NilRowsInit` in `internal/tui/overview_model_test.go` — create model with `NewOverviewModel(ctx, nil, 0)`, assert state is `ViewStateInitializing`; create model with `NewOverviewModel(ctx, rows, 5)`, assert state is `ViewStateLoading` (backward compatibility)
- [X] T007 [P] [US1] Write test `TestOverviewView_InitializingRender` in `internal/tui/overview_view_test.go` — create model in `ViewStateInitializing` with `progressMsg` set, call `View()`, assert output contains the phase message text and does not contain table elements

### Implementation for User Story 1

- [X] T008 [P] [US1] Add `renderInitializingView()` method in `internal/tui/overview_view.go` — render spinner frame from `m.loadingState.spinner.View()` combined with `m.progressMsg` (default to "Initializing..." if empty). Use `fmt.Sprintf("\n %s %s\n\n", spinnerView, msg)` format matching `RenderLoading()` pattern in `internal/tui/cost_view.go`
- [X] T009 [US1] Update `View()` method routing in `internal/tui/overview_view.go` — add `case ViewStateInitializing: return m.renderInitializingView()` in the switch statement, before `ViewStateLoading`
- [X] T010 [US1] Handle `OverviewPhaseMsg` in `Update()` in `internal/tui/overview_model.go` — add type assertion check for `OverviewPhaseMsg`, set `m.progressMsg = msg.Phase`, return `(m, nil)`. Place alongside existing message handlers (after `OverviewLoadingProgressMsg` handler)
- [X] T011 [US1] Handle `OverviewDataReadyMsg` in `Update()` in `internal/tui/overview_model.go` — set `m.allRows = msg.Rows`, `m.rows = msg.Rows`, `m.totalCount = msg.TotalCount`, transition `m.state = ViewStateLoading`, rebuild table with `m.table = m.buildOverviewTable()`, return `(m, nil)`
- [X] T012 [US1] Forward spinner ticks during `ViewStateInitializing` in `Update()` in `internal/tui/overview_model.go` — in the `case ViewStateInitializing:` block of the state-specific switch, check for `spinner.TickMsg` and delegate to `m.loadingState.Update(msg)`, returning its command to keep the spinner animating
- [X] T013 [US1] Restructure `executeOverview()` in `internal/cli/overview.go` — move `shouldUseInteractiveTUI(cmd.OutOrStdout(), params.output, params.plain)` call to immediately after `resolveOverviewDateRange()` (its inputs are flag-derived, available early). If interactive, call the new `runInteractiveOverviewWithInit()` instead of the existing sequential pipeline. If not interactive, preserve the existing sequential flow unchanged.
- [X] T014 [US1] Implement `runInteractiveOverviewWithInit()` in `internal/cli/overview.go` — create new function with signature `func runInteractiveOverviewWithInit(ctx context.Context, cmd *cobra.Command, params overviewParams, dateRange engine.DateRange) error`. This function: (1) creates model via `tui.NewOverviewModel(ctx, nil, 0)`, (2) creates `tea.NewProgram(model, tea.WithAltScreen())`, (3) creates `enrichCtx, enrichCancel` from context, (4) launches background goroutine that sends phase messages, calls `resolveOverviewData`, `DetectPendingChanges`, `MergeResourcesForOverview`, `openPlugins`, creates engine, sends `OverviewDataReadyMsg`, then proceeds with enrichment (bridge `progressChan` to Bubble Tea messages same as existing `runInteractiveOverview`), (5) calls `p.Run()`, (6) calls `enrichCancel()` and plugin cleanup

**Checkpoint**: TUI launches immediately with spinner. Phase messages update. Transition to enrichment works. All US1 tests pass.

---

## Phase 3: User Story 2 - Graceful Error Display During Initialization (Priority: P2)

**Goal**: Errors during any initialization phase display in TUI and exit cleanly

**Independent Test**: Run `finfocus cost overview` in directory without Pulumi project; TUI shows error and exits without corrupting terminal

### Tests for User Story 2 (MANDATORY - TDD Required)

- [X] T015 [P] [US2] Write test `TestOverviewModel_InitErrorMsg` in `internal/tui/overview_model_test.go` — create model with nil rows (state = `ViewStateInitializing`), send `OverviewInitErrorMsg{Err: fmt.Errorf("no Pulumi project found")}`, assert state transitions to `ViewStateError`, assert `m.err` contains "no Pulumi project found", assert returned command is `tea.Quit`
- [X] T016 [P] [US2] Write test `TestOverviewView_ErrorStateRender` in `internal/tui/overview_view_test.go` — create model in `ViewStateError` with `err` set to a test error, call `View()`, assert output contains the error message text

### Implementation for User Story 2

- [X] T017 [US2] Handle `OverviewInitErrorMsg` in `Update()` in `internal/tui/overview_model.go` — add type assertion check, set `m.state = ViewStateError`, set `m.err = msg.Err`, return `(m, tea.Quit)`. Place alongside other new message handlers.
- [X] T018 [US2] Add error sends in background goroutine in `internal/cli/overview.go` — in `runInteractiveOverviewWithInit()`, after each phase call that can fail (`resolveOverviewData`, `openPlugins`, `validateAndApplyOverviewFilters`), check error and send `tui.OverviewInitErrorMsg{Err: err}` then return from the goroutine. This ensures any init failure is displayed in the TUI rather than silently dropped.

**Checkpoint**: Errors during initialization display in TUI. All US2 tests pass.

---

## Phase 4: User Story 3 - User Cancellation During Loading Phases (Priority: P3)

**Goal**: User can press `q` or `Ctrl+C` during initialization to exit cleanly with no orphaned processes

**Independent Test**: Launch overview, press `q` during spinner; process exits within 2 seconds

### Tests for User Story 3 (MANDATORY - TDD Required)

- [X] T019 [P] [US3] Write test `TestOverviewModel_QuitDuringInitializing` in `internal/tui/overview_model_test.go` — create model in `ViewStateInitializing`, send `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}`, assert state transitions to `ViewStateQuitting` and returned command is `tea.Quit`. Repeat for `Ctrl+C` key.
- [X] T020 [P] [US3] Write test `TestOverviewModel_WindowResizeDuringInitializing` in `internal/tui/overview_model_test.go` — create model in `ViewStateInitializing`, send `tea.WindowSizeMsg{Width: 120, Height: 40}`, assert `m.width` and `m.height` are updated (existing window resize handler already covers this, but verify the initializing state path)

### Implementation for User Story 3

- [X] T021 [US3] Add key handling for `ViewStateInitializing` in `Update()` in `internal/tui/overview_model.go` — in the `case ViewStateInitializing:` block, before the spinner tick handling from T012, check for `tea.KeyMsg` with `q` or `ctrl+c` keys. On match, set `m.state = ViewStateQuitting` and return `(m, tea.Quit)`. This ensures the TUI exits cleanly during initialization.
- [X] T022 [US3] Implement plugin cleanup coordination in `internal/cli/overview.go` — in `runInteractiveOverviewWithInit()`, create a `cleanupChan chan func()` (buffered, size 1). Background goroutine sends the cleanup function from `openPlugins()` on this channel. Main goroutine reads from `cleanupChan` after `p.Run()` returns (non-blocking select with default) and calls cleanup if available. Also defer `enrichCancel()` to propagate context cancellation to all background work including `pulumi preview` child processes.

**Checkpoint**: User can quit during any init phase. No orphaned processes. All US3 tests pass.

---

## Phase 5: Polish and Cross-Cutting Concerns

**Purpose**: Quality gates, documentation, final validation

- [X] T023 Run `go test ./internal/tui/... -v -cover` and verify 80%+ coverage on modified files in `internal/tui/`
- [X] T024 Run `go test ./internal/cli/... -v` and verify all existing overview tests still pass (backward compatibility for non-interactive path)
- [X] T025 Run `make lint` and fix any linting issues in modified files
- [X] T026 Run `make test` for full test suite validation
- [X] T027 [P] Run `npx markdownlint-cli` on any modified markdown files
- [X] T028 Verify godoc coverage completeness on all new exported symbols (`OverviewPhaseMsg`, `OverviewDataReadyMsg`, `OverviewInitErrorMsg`, `ViewStateInitializing`) — confirm godoc comments added in T001/T002 are present and accurate
- [X] T029 [P] Update `CLAUDE.md` internal/tui section to document the new `ViewStateInitializing` state in the ViewState enum and the three new message types per Constitution Principle IV (Documentation Integrity)

---

## Dependencies and Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies — can start immediately
- **User Story 1 (Phase 2)**: Depends on Phase 1 completion
- **User Story 2 (Phase 3)**: Depends on T014 (background goroutine exists to add error sends)
- **User Story 3 (Phase 4)**: Depends on T012 and T014 (initializing state handler and background goroutine exist)
- **Polish (Phase 5)**: Depends on all user stories complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 1 — core feature
- **User Story 2 (P2)**: Depends on US1 (T014 provides the goroutine structure for error sends)
- **User Story 3 (P3)**: Depends on US1 (T012 provides the ViewStateInitializing handler for key input)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- View changes (T008, T009) can parallel model changes (T010, T011, T012)
- CLI restructure (T013, T014) depends on TUI model/view being ready

### Parallel Opportunities

Within Phase 1:

- T001 (cost_model.go) and T002 (overview_model.go) can run in parallel — different files

Within Phase 2 (US1):

- T004, T005, T006, T007 (all test files) can run in parallel
- T008 (view) and T010, T011, T012 (model) can run in parallel — different files
- T013, T014 (CLI) depend on T008-T012 (TUI changes) being complete

Within Phase 3 (US2):

- T015, T016 (tests) can run in parallel

Within Phase 4 (US3):

- T019, T020 (tests) can run in parallel

---

## Parallel Example: User Story 1

```text
# Wave 1 — Tests (all parallel, different test functions):
T004: TestOverviewModel_PhaseMsg
T005: TestOverviewModel_DataReadyMsg
T006: TestOverviewModel_NilRowsInit
T007: TestOverviewView_InitializingRender

# Wave 2 — TUI implementation (parallel by file):
T008 + T009: overview_view.go changes
T010 + T011 + T012: overview_model.go changes

# Wave 3 — CLI restructure (sequential):
T013: Move shouldUseInteractiveTUI() early
T014: Implement runInteractiveOverviewWithInit()
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational (T001-T003)
2. Complete Phase 2: User Story 1 (T004-T014)
3. **STOP and VALIDATE**: TUI launches immediately with spinner and phase messages
4. Run `make test && make lint` to verify no regressions

### Incremental Delivery

1. Phase 1 (Foundational) — New types compile, existing tests pass
2. Phase 2 (US1) — TUI launches immediately with phase feedback (MVP)
3. Phase 3 (US2) — Errors display gracefully in TUI
4. Phase 4 (US3) — Cancellation works during initialization
5. Phase 5 (Polish) — Quality gates, docs, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US2 and US3 are additive on top of US1; US1 is the MVP
- Existing `renderLoadingView()` and `renderProgressBanner()` remain unchanged — they continue to serve the `ViewStateLoading` (enrichment) phase
- The old `runInteractiveOverview()` function was removed (unused); `runInteractiveOverviewWithInit()` replaces it for the interactive path
- No new files are created; all changes are edits to existing files
