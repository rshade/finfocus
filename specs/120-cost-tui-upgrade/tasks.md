# Tasks: Cost Commands TUI Upgrade

**Feature**: Cost Commands TUI Upgrade (Branch: `106-cost-tui-upgrade`)
**Status**: Generated via `.specify`
**Priority**: P1

## Phase 1: Setup & Data Model
*Goal: Initialize project structure and update core data models.*

- [x] T001 Create `internal/cli/cost_tui.go` with initial package declaration
- [x] T002 Create `internal/tui/cost_model.go` with initial package declaration
- [x] T003 Create `internal/tui/cost_view.go` with initial package declaration
- [x] T004 Create `internal/tui/cost_loading.go` with initial package declaration
- [x] T005 Update `internal/engine/types.go` to add `Delta` field to `CostResult` struct

## Phase 2: Foundational Structures
*Goal: Implement the core data structures and types required for the TUI.*

- [x] T006 Implement `ViewState` and `SortField` enums in `internal/tui/cost_model.go`
- [x] T007 Implement `CostViewModel` struct definition with required fields in `internal/tui/cost_model.go`
- [x] T008 Implement `ResourceRow` struct and `NewResourceRow` transformation function in `internal/tui/cost_view.go`
- [x] T009 Implement `LoadingState` and `PluginStatus` structs in `internal/tui/cost_loading.go`

## Phase 3: User Story 1 - View Projected Costs with Styled Summary
*Goal: Display a beautiful cost summary in TTY mode, with fallback to plain text.*

- [x] T010 [US1] Implement `DetectOutputMode` usage and router logic in `internal/cli/cost_tui.go`
- [x] T011 [US1] Implement `RenderCostSummary` function using Lip Gloss in `internal/tui/cost_view.go`
- [x] T012 [US1] Implement plain text fallback logic for non-TTY/piped output in `internal/cli/cost_tui.go`
- [x] T013 [US1] Refactor `internal/cli/cost_projected.go` to use the `cost_tui.go` router
- [x] T014 [US1] Create `internal/tui/cost_view_test.go` and add tests for `RenderCostSummary`

## Phase 4: User Story 2 - Navigate Interactive Resource Table
*Goal: Enable interactive navigation of the resource list using Bubble Tea.*

- [x] T015 [US2] Implement `Init` and `Update` methods for `CostViewModel` in `internal/tui/cost_model.go` to handle basic key commands
- [x] T016 [US2] Implement table rendering using `bubbles/table` in `internal/tui/cost_view.go`
- [x] T017 [US2] Implement `RenderDetailView` for selected resource in `internal/tui/cost_view.go`
- [x] T018 [US2] Add view switching logic (List <-> Detail) in `internal/tui/cost_model.go` Update loop
- [x] T019 [US2] Create `internal/tui/cost_model_test.go` and add tests for table navigation and state transitions

## Phase 5: User Story 3 - View Actual Costs with TUI
*Goal: Bring the TUI experience to the actual cost command.*

- [x] T020 [US3] Refactor `internal/cli/cost_actual.go` to use the `cost_tui.go` router
- [x] T021 [US3] Implement adaptation logic for Actual Cost results (time periods) in `internal/tui/cost_model.go`
- [x] T022 [US3] Add test cases for actual cost rendering in `internal/tui/cost_view_test.go`

## Phase 6: User Story 4 - See Loading Progress
*Goal: Provide visual feedback while plugins are querying data.*

- [x] T023 [US4] Implement `InitLoading` and tick handling in `internal/tui/cost_loading.go`
- [x] T024 [US4] Add plugin status update handling (messages) in `internal/tui/cost_model.go`
- [x] T025 [US4] Implement spinner rendering overlay in `internal/tui/cost_view.go`
- [x] T026 [US4] Create `internal/tui/cost_loading_test.go` and add tests for spinner state

## Phase 7: User Story 5 - View Cost Deltas
*Goal: Visualize cost changes with directional indicators.*

- [x] T027 [US5] Implement `RenderDelta` helper with color/arrow logic in `internal/tui/cost_view.go` (include ASCII fallback icons for non-UTF-8)
- [x] T028 [US5] Update `ResourceRow` transformation to populate `Delta` field in `internal/tui/cost_view.go`
- [x] T029 [US5] Add unit tests for `RenderDelta` scenarios in `internal/tui/cost_view_test.go`

## Phase 8: User Story 6 - Sort and Filter Resources
*Goal: Add power-user features for managing large resource lists.*

- [x] T030 [US6] Implement sorting logic (by Cost, Name, Type) in `internal/tui/cost_model.go`
- [x] T031 [US6] Add text input model for filtering in `internal/tui/cost_model.go`
- [x] T032 [US6] Implement filter logic to restrict table rows in `internal/tui/cost_model.go`
- [x] T033 [US6] Add tests for sorting and filtering behavior in `internal/tui/cost_model_test.go`

## Phase 9: Polish & Verification
*Goal: Ensure robustness and code quality.*

- [x] T034 Verify `NO_COLOR` and terminal width compliance in `internal/cli/cost_tui.go` logic
- [x] T035 Create `test/integration/cli_tui_test.go` for end-to-end verification of output modes
- [x] T036 Run `make lint` and `make test` to ensure full compliance

## Implementation Strategy
- **MVP (P1)**: Deliver Phase 1-3. This provides styled output for the most common use case (`cost projected`) and replaces the basic tabwriter.
- **Interactive (P2)**: Deliver Phase 4-5. Adds the interactive table and actual cost support.
- **Enhanced (P3/P4)**: Deliver Phase 6-8. Adds loading states, deltas, and advanced list controls.

## Dependencies & Parallelization
- Phase 1 & 2 are blocking for all other phases.
- Phase 3 (US1) and Phase 4 (US2) can be implemented in parallel after Phase 2, but US1 is prioritized.
- Phase 5 (US3) depends on the Router from Phase 3 and Model from Phase 4.
- Phase 6, 7, 8 are largely independent of each other but depend on the core Model (Phase 4).
