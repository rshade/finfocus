# Implementation Plan: TUI Immediate Launch with Phase Progress Feedback

**Branch**: `596-tui-phase-progress` | **Date**: 2026-02-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/596-tui-phase-progress/spec.md`

## Summary

Restructure the `executeOverview()` pipeline to launch the Bubble Tea TUI
immediately (before data loading) and push phase progress messages to it as
each loading stage completes. This eliminates the ~16-second blank-terminal
wait by adding a `ViewStateInitializing` state, three new Bubble Tea message
types, and moving the TUI launch before `resolveOverviewData()`.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Bubble Tea (charmbracelet/bubbletea), Lip Gloss
(charmbracelet/lipgloss), Cobra (spf13/cobra)
**Storage**: N/A (no new storage)
**Testing**: `go test` with testify (assert/require), table-driven tests
**Target Platform**: Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
**Project Type**: Single Go project (CLI application)
**Performance Goals**: TUI renders within 1 second of command invocation;
cancellation exits within 2 seconds
**Constraints**: No new dependencies; reuse existing LoadingState spinner;
Bubble Tea message queue must handle pre-Run sends
**Scale/Scope**: 4 files modified, ~150 lines added, ~50 lines restructured

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is orchestration/UI logic, not a
  provider integration. Core remains provider-agnostic. No violation.
- [x] **Test-Driven Development**: Unit tests planned for all new message
  types, state transitions, and view rendering. 80%+ coverage target.
- [x] **Cross-Platform Compatibility**: Uses only Bubble Tea and standard
  library. Terminal handling is cross-platform via `golang.org/x/term`.
- [x] **Documentation Integrity**: CLAUDE.md overview section will be updated
  to document the new `ViewStateInitializing` state.
- [x] **Protocol Stability**: No protocol buffer changes. No cross-repo impact.
- [x] **Implementation Completeness**: Full implementation — no stubs or TODOs.
  All state transitions, error handling, and cancellation are complete.
- [x] **Quality Gates**: `make lint` and `make test` will pass before PR.
- [x] **Multi-Repo Coordination**: Single-repo change (finfocus-core only).
  No spec or plugin changes needed.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/596-tui-phase-progress/
├── plan.md              # This file
├── research.md          # Phase 0: research findings
├── data-model.md        # Phase 1: state machine and message types
├── quickstart.md        # Phase 1: verification guide
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   └── overview.go              # Restructure executeOverview() and runInteractiveOverview()
└── tui/
    ├── cost_model.go            # Add ViewStateInitializing constant
    ├── overview_model.go        # Add message types, update NewOverviewModel(), Update()
    └── overview_view.go         # Add renderInitializingView(), update View() routing
```

**Structure Decision**: All changes are within the existing `internal/cli/` and
`internal/tui/` packages. No new packages or files are created. The feature
modifies 4 existing files.

### Detailed File Changes

#### 1. `internal/tui/cost_model.go` — Add ViewStateInitializing

Insert `ViewStateInitializing` as the first constant in the `ViewState` iota:

```go
const (
    ViewStateInitializing ViewState = iota  // Before data is available
    ViewStateLoading                         // During enrichment
    ViewStateList                            // Show resource table
    ViewStateDetail                          // Show single resource details
    ViewStateQuitting                        // Exiting application
    ViewStateError                           // Fatal error occurred
)
```

**Impact**: All existing `ViewState` values shift by +1. Since they are used
only via named constants (never as raw integers), this is safe.

#### 2. `internal/tui/overview_model.go` — Message Types and Model Changes

**New message types** (add alongside existing types):

```go
// OverviewPhaseMsg reports which phase of data loading is active.
type OverviewPhaseMsg struct {
    Phase string
}

// OverviewDataReadyMsg signals initial data loading is complete.
type OverviewDataReadyMsg struct {
    Rows       []engine.OverviewRow
    TotalCount int
}

// OverviewInitErrorMsg signals that initial data loading failed.
type OverviewInitErrorMsg struct {
    Err error
}
```

**Constructor change**: `NewOverviewModel` must accept `nil` skeleton rows
and start in `ViewStateInitializing` when rows are nil:

```go
func NewOverviewModel(
    ctx context.Context,
    skeletonRows []engine.OverviewRow,
    totalCount int,
) (OverviewModel, tea.Cmd) {
    initialState := ViewStateLoading
    if skeletonRows == nil {
        initialState = ViewStateInitializing
        skeletonRows = []engine.OverviewRow{} // avoid nil slice
    }
    m := OverviewModel{
        state:       initialState,
        allRows:     skeletonRows,
        // ... rest unchanged
    }
    // ...
}
```

**Update() additions** — handle three new message types:

```go
case OverviewPhaseMsg:
    m.progressMsg = msg.Phase
    return m, nil

case OverviewDataReadyMsg:
    m.allRows = msg.Rows
    m.rows = msg.Rows
    m.totalCount = msg.TotalCount
    m.state = ViewStateLoading
    m.table = m.buildOverviewTable()
    return m, nil

case OverviewInitErrorMsg:
    m.state = ViewStateError
    m.err = msg.Err
    return m, tea.Quit
```

**Key input handling**: During `ViewStateInitializing`, handle `q` and
`Ctrl+C` for cancellation (same pattern as `ViewStateLoading`).

#### 3. `internal/tui/overview_view.go` — Initializing View

**View() routing update**: Add `ViewStateInitializing` case:

```go
case ViewStateInitializing:
    return m.renderInitializingView()
```

**New renderInitializingView()**: Reuse the existing `LoadingState` spinner
with the dynamic `progressMsg`:

```go
func (m OverviewModel) renderInitializingView() string {
    spinnerView := ""
    if m.loadingState != nil {
        spinnerView = m.loadingState.spinner.View()
    }
    msg := m.progressMsg
    if msg == "" {
        msg = "Initializing..."
    }
    return fmt.Sprintf("\n %s %s\n\n", spinnerView, msg)
}
```

**Spinner tick forwarding**: During `ViewStateInitializing`, forward
`spinner.TickMsg` to `loadingState.Update()` to keep the spinner animated.

#### 4. `internal/cli/overview.go` — Pipeline Restructure

**Core change**: Split `executeOverview()` into two paths:

For the **interactive path** (`shouldUseInteractiveTUI() == true`):

1. Determine interactive mode early (after flag validation, before data load)
2. Create `OverviewModel` with `nil` rows (starts in `ViewStateInitializing`)
3. Create `tea.NewProgram(model, tea.WithAltScreen())`
4. Launch background goroutine that:
   a. Sends `OverviewPhaseMsg{Phase: "Loading stack state..."}`
   b. Calls `resolveOverviewData(ctx, params)`
   c. On error: sends `OverviewInitErrorMsg{Err: err}` and returns
   d. Sends `OverviewPhaseMsg{Phase: "Detecting changes..."}`
   e. Calls `DetectPendingChanges()` and `MergeResourcesForOverview()`
   f. Sends `OverviewPhaseMsg{Phase: "Starting cost plugins..."}`
   g. Calls `openPlugins()`
   h. Creates engine
   i. Sends `OverviewDataReadyMsg{Rows: rows, TotalCount: len(rows)}`
   j. Proceeds with enrichment (existing pattern)
5. Call `p.Run()` (blocks until user quits)
6. Cancel context, cleanup plugins

For the **plain path**: No changes — existing sequential flow continues.

**shouldUseInteractiveTUI() call timing**: Must be called before data loading
to determine the path. Currently called after data loading (step 10 of 12).
Move it to after flag validation. Note: its inputs (`cmd.OutOrStdout()`,
`params.output`, `params.plain`) are all available immediately.

**Plugin cleanup in background goroutine**: The cleanup function returned by
`openPlugins()` must be deferred within the background goroutine or passed
to the main goroutine for deferred cleanup after `p.Run()` returns. Use a
channel or shared variable protected by the enrichment context to coordinate
this.

## Complexity Tracking

No constitution violations. No complexity justification needed.
