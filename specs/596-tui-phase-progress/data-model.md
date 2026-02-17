# Data Model: TUI Immediate Launch with Phase Progress Feedback

**Branch**: `596-tui-phase-progress`
**Date**: 2026-02-17

## State Machine

### ViewState Transitions

```text
                    ┌──────────────────┐
                    │ ViewState        │
                    │ Initializing     │◄── TUI created with nil rows
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        OverviewPhaseMsg  DataReadyMsg  InitErrorMsg
        (updates phase    (transitions  (transitions
         message only)     to Loading)   to Error)
              │              │              │
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │Initiali- │  │ Loading  │  │  Error   │──► tea.Quit
        │zing      │  │(enrich)  │  └──────────┘
        │(updated  │  └────┬─────┘
        │ phase)   │       │
        └──────────┘       │ AllResourcesLoadedMsg
                           ▼
                    ┌──────────┐
                    │   List   │◄──► Detail (Enter/Esc)
                    └────┬─────┘
                         │ q / Ctrl+C
                         ▼
                    ┌──────────┐
                    │ Quitting │
                    └──────────┘
```

### ViewState Enum (Updated)

| Value | Constant | Description |
|-------|----------|-------------|
| 0 | `ViewStateInitializing` | **NEW** — TUI shown before data available |
| 1 | `ViewStateLoading` | Enrichment in progress (existing) |
| 2 | `ViewStateList` | Interactive table view (existing) |
| 3 | `ViewStateDetail` | Resource detail panel (existing) |
| 4 | `ViewStateQuitting` | Application exiting (existing) |
| 5 | `ViewStateError` | Fatal error display (existing) |

## Message Types

### New Messages

| Type | Fields | Sent By | Handled In | Effect |
|------|--------|---------|------------|--------|
| `OverviewPhaseMsg` | `Phase string` | Background goroutine | `Update()` | Updates `progressMsg` |
| `OverviewDataReadyMsg` | `Rows []engine.OverviewRow`, `TotalCount int` | Background goroutine | `Update()` | Populates model, transitions to `ViewStateLoading` |
| `OverviewInitErrorMsg` | `Err error` | Background goroutine | `Update()` | Sets error, transitions to `ViewStateError`, sends `tea.Quit` |

### Existing Messages (Unchanged)

| Type | Fields | Purpose |
|------|--------|---------|
| `OverviewResourceLoadedMsg` | `Index int`, `Row engine.OverviewRow` | Single resource enriched |
| `OverviewLoadingProgressMsg` | `Loaded int`, `Total int` | Batch progress update |
| `OverviewAllResourcesLoadedMsg` | (none) | Enrichment complete |

## Entities

### Phase (Logical)

Phases are string labels representing loading stages. They are not persisted
or stored — they exist only as message payloads displayed in the TUI.

| Phase Message | Triggered When |
|---------------|----------------|
| `"Loading stack state..."` | Before `resolveOverviewData()` |
| `"Running pulumi preview..."` | During `resolveOverviewData()` (auto-detect path) |
| `"Detecting changes..."` | Before `DetectPendingChanges()` |
| `"Merging resources..."` | Before `MergeResourcesForOverview()` |
| `"Starting cost plugins..."` | Before `openPlugins()` |
| `"Preparing cost engine..."` | Before engine creation |

### OverviewModel Fields (Changes)

| Field | Type | Change | Description |
|-------|------|--------|-------------|
| `state` | `ViewState` | Modified | Now starts at `ViewStateInitializing` when rows are nil |
| `progressMsg` | `string` | Reused | Displays phase messages during init, progress during enrichment |
| `allRows` | `[]engine.OverviewRow` | Modified | Initialized as empty slice (not nil) when in initializing state |
| `totalCount` | `int` | Modified | Set to 0 initially, updated by `OverviewDataReadyMsg` |

No new fields are added to `OverviewModel`. The existing `progressMsg`,
`loadingState`, `allRows`, and `totalCount` fields are reused.
