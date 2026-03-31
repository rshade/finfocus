# Feature Specification: State-Only Flag for Overview Command

**Feature Branch**: `607-state-only-flag`
**Created**: 2026-03-30
**Status**: Draft
**Input**: GitHub Issue #690 — perf(cli): add --state-only flag to skip pulumi preview

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fast Cost-Only Overview (Priority: P1)

As a developer checking infrastructure costs during daily work, I want to get a
cost overview without waiting for `pulumi preview` so that I can see cost data in
~3 seconds instead of ~18 seconds.

**Why this priority**: This is the core value proposition. The 15-second
`pulumi preview` delay is the primary pain point — 83% of total `finfocus overview`
time on a typical 8-resource stack. Users who only need cost data, drift analysis,
and recommendations gain no value from the preview step.

**Independent Test**: Can be fully tested by running
`finfocus overview --state-only` in a Pulumi project directory and verifying that
cost data appears without running `pulumi preview`.

**Acceptance Scenarios**:

1. **Given** a Pulumi project directory with an active stack, **When** the user
   runs `finfocus overview --state-only`, **Then** the overview displays cost data
   for all state resources with "active" status and the `pulumi preview` step is
   completely skipped.
2. **Given** a Pulumi project directory with an active stack, **When** the user
   runs `finfocus overview --state-only --plain --yes`, **Then** the overview
   renders non-interactively with all resources showing "active" status and no
   pending change detection.
3. **Given** a Pulumi project directory, **When** the user runs
   `finfocus overview --state-only`, **Then** the total execution time is
   significantly reduced compared to the default overview (preview step eliminated).

---

### User Story 2 - Flag Conflict Validation (Priority: P2)

As a user who might accidentally combine incompatible flags, I want clear error
messages when using `--state-only` with `--pulumi-json` so that I understand
why they cannot be used together.

**Why this priority**: Prevents user confusion. `--state-only` means "skip
preview" while `--pulumi-json` means "use this preview file" — these are
contradictory intents.

**Independent Test**: Can be tested by running
`finfocus overview --state-only --pulumi-json plan.json` and verifying an error
is returned.

**Acceptance Scenarios**:

1. **Given** the user provides both `--state-only` and `--pulumi-json` flags,
   **When** the command is invoked, **Then** an error message is returned
   explaining the flags are mutually exclusive.
2. **Given** the user provides `--state-only` with `--pulumi-state`, **When**
   the command is invoked, **Then** the overview loads state from the explicit
   file and skips preview (no conflict — both intents align).

---

### User Story 3 - Interactive TUI With State-Only (Priority: P3)

As a user running the interactive TUI with `--state-only`, I want the TUI to
launch in state-only mode so that I can still trigger an on-demand preview later
with the `p` key if I decide I need change detection.

**Why this priority**: The TUI already supports state-only mode with on-demand
preview. The `--state-only` flag should integrate cleanly with this existing
capability, giving users the option to get fast initial results and then opt
into the slower preview only if needed.

**Independent Test**: Can be tested by running `finfocus overview --state-only`
in a TTY and verifying the TUI launches in state-only mode with the `p` key
available for on-demand preview.

**Acceptance Scenarios**:

1. **Given** the user runs `finfocus overview --state-only` in a terminal (TTY),
   **When** the TUI launches, **Then** it shows cost data immediately in
   state-only mode with an indication that preview was skipped.
2. **Given** the TUI is in state-only mode from `--state-only`, **When** the
   user presses `p`, **Then** the TUI runs `pulumi preview` in the background
   and updates the display with change detection results when complete.

---

### Edge Cases

- What happens when `--state-only` is used outside a Pulumi project directory
  without `--pulumi-state`? The same auto-detect failure error as the default
  overview occurs.
- What happens when `--state-only` is combined with `--yes`? The `--yes` flag
  is irrelevant for the preview decision (preview is unconditionally skipped),
  but the combination does not error. `--yes` still suppresses other prompts.
- What happens when `--state-only` is combined with `--stack`? Stack selection
  works normally for the state export step.
- What happens in non-TTY mode with `--state-only`? The overview renders in
  plain mode with state-only data, consistent with the existing non-TTY
  state-only fallback behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept a `--state-only` boolean flag on the `overview`
  command that defaults to `false`.
- **FR-002**: When `--state-only` is set, the system MUST skip the
  `pulumi preview --json` execution entirely (no subprocess spawned).
- **FR-003**: When `--state-only` is set, the system MUST skip the change
  detection phase (`DetectPendingChanges` not called).
- **FR-004**: When `--state-only` is set, all resources MUST display with
  "active" status (no "creating", "updating", "deleting", or "replacing"
  markers).
- **FR-005**: When `--state-only` is set, cost data (actual costs, projected
  costs, drift analysis, and recommendations) MUST work identically to the
  default overview.
- **FR-006**: The system MUST return an error when both `--state-only` and
  `--pulumi-json` are provided, with a message explaining they are mutually
  exclusive.
- **FR-007**: The `--state-only` flag MUST be compatible with `--pulumi-state`
  (explicit state file with no preview).
- **FR-008**: The `--state-only` flag MUST work in both interactive (TUI) and
  non-interactive (plain text) modes.
- **FR-009**: In interactive TUI mode with `--state-only`, the on-demand preview
  (`p` key) MUST remain available.
- **FR-010**: The `--state-only` flag MUST be compatible with all other existing
  flags (`--stack`, `--from`, `--to`, `--adapter`, `--output`, `--filter`,
  `--plain`, `--yes`, `--no-pagination`, `--exit-on-threshold`, `--exit-code`,
  `--budget-scope`).
- **FR-011**: The command help text and examples MUST document the `--state-only`
  flag and its purpose.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can obtain a cost overview in under 5 seconds on a typical
  8-resource stack when using `--state-only`, compared to ~18 seconds without it.
- **SC-002**: All cost data (actual, projected, drift, recommendations) is
  identical between `--state-only` and default overview for the same stack state.
- **SC-003**: The `--state-only` flag does not alter the behavior of any other
  existing flag or feature when not set (zero regression).
- **SC-004**: Users receive a clear, actionable error message within 1 second
  when combining `--state-only` with `--pulumi-json`.

## Assumptions

- The `pulumi preview --json` subprocess is the dominant contributor to overview
  latency (measured at ~15 seconds / 83% of total time on an 8-resource stack).
- The existing `isStateOnly` code path (state-only mode with on-demand preview)
  is stable and well-tested, meaning the `--state-only` flag can reuse this
  existing infrastructure without requiring new downstream logic.
- Users who use `--state-only` understand that pending infrastructure changes
  will not be visible until they either remove the flag or use the `p` key in
  the TUI.
- The `--state-only` flag complements (does not replace) the existing automatic
  state-only fallback behavior that occurs when change detection suggests no
  changes are pending.
