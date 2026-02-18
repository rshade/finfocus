# Research: TUI Immediate Launch with Phase Progress Feedback

**Branch**: `596-tui-phase-progress`
**Date**: 2026-02-17

## R1: Bubble Tea Message Queueing Before p.Run()

**Decision**: Messages sent via `p.Send()` before `p.Run()` are queued and
delivered when the event loop starts.

**Rationale**: Bubble Tea's `Send()` method writes to a buffered channel
(`msgs chan Msg`). The `Run()` method reads from this channel in a select
loop. Messages sent before `Run()` accumulate in the channel buffer and are
processed in order once `Run()` begins. This is the standard pattern used in
Bubble Tea examples for initial data loading.

**Alternatives considered**:

- Using `tea.Cmd` returning functions from `Init()`: Would not work because
  the data loading goroutine needs to be launched from the CLI layer, not
  from inside the model.
- Passing a channel to the model: Adds unnecessary complexity; `p.Send()`
  is the idiomatic approach.

## R2: ViewState Iota Reordering Safety

**Decision**: Adding `ViewStateInitializing` as the first iota constant is
safe because all code references `ViewState` values by name, never by
integer value.

**Rationale**: Grep of the codebase confirms no raw integer comparisons
against `ViewState`. All usages are `m.state == ViewStateLoading`,
`case ViewStateList:`, etc. The iota shift from 0→1 for `ViewStateLoading`
has no runtime impact.

**Alternatives considered**:

- Adding `ViewStateInitializing` at the end (value 5): Would work but breaks
  the logical ordering where states progress from left to right
  (initializing → loading → list → detail → quitting/error).
- Using a separate boolean field `isInitializing`: Adds state complexity and
  makes the View() routing less clean.

## R3: Spinner Tick Forwarding During Initializing State

**Decision**: Forward `spinner.TickMsg` to `loadingState.Update()` during
`ViewStateInitializing` to keep the spinner animated.

**Rationale**: The existing `ViewStateLoading` case in `Update()` returns
`(m, nil)` — it relies on the spinner ticks being handled elsewhere. Looking
at the code, the `Init()` method returns `loadingState.Init()` which starts
the tick. But subsequent ticks need to be processed. The current code handles
this in the `default` case or through the `OverviewResourceLoadedMsg` flow.
For `ViewStateInitializing`, we need explicit tick forwarding since no
resource messages arrive yet.

**Alternatives considered**:

- Using a separate spinner instance for the initializing phase: Unnecessary
  duplication; the existing `LoadingState` spinner works.
- Not animating the spinner during init: Defeats the purpose of showing
  immediate feedback.

## R4: Plugin Cleanup Coordination

**Decision**: Use a cleanup channel to pass the plugin cleanup function from
the background goroutine to the main goroutine, which defers it after
`p.Run()` returns.

**Rationale**: `openPlugins()` returns a cleanup function that must be called
to prevent resource leaks. Since `openPlugins()` now runs in the background
goroutine but `p.Run()` blocks the main goroutine, the cleanup must be
coordinated. A channel carrying the cleanup function allows the main goroutine
to defer cleanup after the TUI exits.

**Alternatives considered**:

- Defer cleanup inside the background goroutine: Risk of premature cleanup if
  the goroutine finishes before the TUI exits (the TUI still needs the engine
  and plugin clients for the enrichment phase).
- Open plugins before launching the TUI: Would add ~1s delay before TUI
  appears, partially defeating the purpose.

## R5: shouldUseInteractiveTUI() Early Evaluation

**Decision**: Call `shouldUseInteractiveTUI()` immediately after flag parsing,
before data loading, to determine the execution path.

**Rationale**: The function's inputs are `cmd.OutOrStdout()` (always
available), `params.output` (from flags), and `params.plain` (from flags).
None depend on data loading results. Moving the call earlier is safe and
enables the path split before expensive operations.

**Alternatives considered**:

- Always launching the TUI and falling back to plain output: Would complicate
  the plain output path unnecessarily.
- Lazy evaluation with a sync.Once: Over-engineering for a simple boolean
  check.

## R6: Error Auto-Quit vs Pause Behavior

**Decision**: Errors during initialization trigger `tea.Quit` immediately
(auto-quit), consistent with the existing `ViewStateError` behavior.

**Rationale**: The existing error handling pattern in the overview TUI sends
`tea.Quit` when entering `ViewStateError`. Since the TUI uses alt-screen
mode, the error message is printed to stderr by the CLI layer after `p.Run()`
returns. Pausing for acknowledgment would require a new interaction pattern
and is out of scope.

**Alternatives considered**:

- Pausing with "Press any key to exit": Would improve error visibility but
  changes the existing UX pattern and adds scope.
