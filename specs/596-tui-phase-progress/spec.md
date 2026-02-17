# Feature Specification: TUI Immediate Launch with Phase Progress Feedback

**Feature Branch**: `596-tui-phase-progress`
**Created**: 2026-02-17
**Status**: Draft
**Input**: GitHub Issue #689 — perf(tui): launch TUI immediately with phase progress feedback

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Immediate Visual Feedback on Launch (Priority: P1)

As a developer running `finfocus cost overview` on a Pulumi stack, I want to see
the TUI appear immediately with a spinner and phase status messages so that I know
the tool is working and can estimate how long I need to wait.

**Why this priority**: This is the core user-facing problem. Today users stare at a
blank terminal for ~16 seconds with zero feedback, which feels broken. Showing the
TUI instantly with a spinner eliminates the perceived-hang experience.

**Independent Test**: Can be tested by launching the overview command on any Pulumi
stack and verifying the TUI renders within 1 second, showing a spinner and "Loading
stack state..." message before any data is available.

**Acceptance Scenarios**:

1. **Given** a user runs `finfocus cost overview` in interactive mode, **When** the
   command starts, **Then** the TUI appears within 1 second showing a spinner and
   initial phase message.
2. **Given** the TUI is displayed with a spinner, **When** the system transitions
   between loading phases (stack export, preview, plugin startup), **Then** the
   phase message updates to reflect the current activity (e.g., "Running pulumi
   preview...").
3. **Given** the TUI is in the initializing state, **When** all data loading
   completes, **Then** the view transitions seamlessly to the enrichment progress
   bar and then to the interactive resource list.

---

### User Story 2 - Graceful Error Display During Initialization (Priority: P2)

As a developer, if data loading fails during any phase (e.g., Pulumi preview
errors, missing stack state, plugin startup failure), I want to see a clear error
message inside the TUI rather than a raw terminal error dump.

**Why this priority**: Errors during data loading are common (wrong directory,
missing credentials, invalid stack). Without proper error handling in the TUI,
failures during background loading would leave the user with a frozen spinner or
a corrupted terminal state.

**Independent Test**: Can be tested by running the overview command in a directory
without a Pulumi stack and verifying the TUI shows an error message and exits
cleanly.

**Acceptance Scenarios**:

1. **Given** the TUI is showing "Loading stack state...", **When** the stack export
   fails (e.g., no Pulumi project found), **Then** the TUI displays the error
   message clearly and exits cleanly without corrupting the terminal.
2. **Given** the TUI is showing "Running pulumi preview...", **When** the preview
   command fails, **Then** the TUI transitions to an error state showing the
   failure reason.
3. **Given** the TUI is showing "Starting cost plugins...", **When** no plugins are
   available, **Then** the TUI displays an appropriate message and allows the user
   to quit gracefully.

---

### User Story 3 - User Cancellation During Loading Phases (Priority: P3)

As a developer, if I realize I launched the command in the wrong directory or
want to abort, I want to press `q` or `Ctrl+C` during the initialization phases
and have the TUI exit cleanly, cancelling any in-progress background work.

**Why this priority**: The long-running preview phase (~15s) makes cancellation
important. Without it, users would need to force-kill the process if they realize
they're in the wrong directory.

**Independent Test**: Can be tested by launching the overview command, pressing `q`
while the spinner is showing, and verifying the process exits within 2 seconds
with no zombie processes.

**Acceptance Scenarios**:

1. **Given** the TUI is in the initializing state showing any phase message,
   **When** the user presses `q` or `Ctrl+C`, **Then** the TUI exits within 2
   seconds and all background work (Pulumi preview, plugin processes) is cancelled.
2. **Given** background work is in progress (e.g., `pulumi preview` running),
   **When** the TUI exits due to user cancellation, **Then** no orphaned child
   processes remain.

---

### Edge Cases

- What happens when the user is in non-interactive mode (`--plain`,
  `--output json`)? The feature does not apply; plain/non-interactive output
  continues unchanged.
- What happens if the TUI terminal is too small to render the spinner? The loading
  view should degrade gracefully within minimum terminal dimensions.
- What happens if data loading completes almost instantly (cached/small stack)? The
  initializing phase should be so brief it's barely visible, transitioning
  immediately to enrichment. No artificial delays should be introduced.
- What happens if the user resizes the terminal during the initializing phase?
  Window resize events should be handled during initialization just as they are
  during the enrichment and list phases.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The TUI MUST render within 1 second of command invocation when
  interactive mode is selected, before any data loading begins.
- **FR-002**: The TUI MUST display a spinner animation during the initializing
  phase to indicate activity.
- **FR-003**: The TUI MUST display a human-readable phase message that updates
  as loading progresses through distinct phases (stack loading, preview, plugin
  startup, resource merging).
- **FR-004**: The TUI MUST transition from the initializing state to the
  enrichment/loading state once initial data is available, without user
  intervention.
- **FR-005**: The TUI MUST handle fatal errors during initialization by
  displaying the error and exiting cleanly (restoring terminal state).
- **FR-006**: The TUI MUST support user cancellation (`q`, `Ctrl+C`) during all
  initialization phases, propagating cancellation to background work.
- **FR-007**: Non-interactive output paths (`--plain`, `--output json/ndjson`)
  MUST remain completely unaffected by this change.
- **FR-008**: The TUI MUST distinguish between the pre-data initializing state
  and the existing enrichment loading state so that the correct visual treatment
  is applied to each.

### Key Entities

- **Phase**: A named stage of the data loading pipeline (e.g., "Loading stack
  state...", "Running pulumi preview..."), representing one discrete unit of
  background work before the TUI can show resource data.
- **View State**: The current mode of the TUI display, now extended to include
  an initializing state before data is available, in addition to the existing
  loading (enrichment), list, detail, error, and quitting states.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users see the TUI (spinner + phase message) within 1 second of
  running the command, eliminating the previous ~16 second blank-terminal wait.
- **SC-002**: Phase messages update at least once per distinct loading stage,
  giving users visibility into which operation is currently running.
- **SC-003**: The transition from initialization to enrichment to interactive list
  is seamless with no visual glitches or blank frames.
- **SC-004**: Cancellation during any initialization phase exits the program
  within 2 seconds with clean terminal restoration.
- **SC-005**: All existing interactive TUI functionality (sorting, filtering,
  pagination, detail view) continues to work identically after the initializing
  phase completes.
- **SC-006**: Non-interactive output modes produce identical results to the
  current behavior.

## Assumptions

- The existing spinner component in the TUI package is reusable for the
  initializing phase without modification.
- The TUI framework supports sending messages from background goroutines before
  the main event loop begins processing (messages are queued).
- The existing context cancellation pattern used for enrichment is sufficient
  for cancelling initialization work.
- The number and names of loading phases are fixed and do not need to be
  user-configurable.
- Terminal window resize handling during the new initializing state follows the
  same patterns already used for the loading/enrichment state.
