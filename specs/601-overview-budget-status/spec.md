# Feature Specification: Overview Budget Status and Health

**Feature Branch**: `601-overview-budget-status`
**Created**: 2026-02-22
**Status**: Draft
**Input**: GitHub Issue #744 — feat(overview): display budget status and health in overview command

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Interactive Budget Health at a Glance (Priority: P1)

A platform engineer runs `finfocus overview` in their terminal as a daily cost dashboard.
After the resource table loads, a color-coded budget health footer appears at the bottom
of the list view showing overall spend versus limit and utilization percentage. The footer
loads asynchronously and does not delay the initial resource table display.

**Why this priority**: The TUI is the primary interactive interface for day-to-day use.
Showing budget health alongside resource costs eliminates the need to run a separate
`cost projected` command and provides immediate awareness of budget status.

**Independent Test**: Can be fully tested by launching the TUI with budgets configured
and verifying the footer renders with correct health badge, spend/limit, and percentage.
Delivers value as a standalone visual indicator even without detail view or non-TTY support.

**Acceptance Scenarios**:

1. **Given** a stack with budgets configured and spend at 45% utilization,
   **When** the user runs `finfocus overview` in an interactive terminal,
   **Then** a green budget health footer appears showing spend/limit and "OK" status
   after budget data loads, without delaying the resource table.

2. **Given** a stack with budgets configured and spend at 85% utilization,
   **When** the user runs `finfocus overview` in an interactive terminal,
   **Then** a yellow budget health footer appears showing spend/limit, percentage, and
   "WARNING" status.

3. **Given** a stack with budgets configured and spend at 105% utilization,
   **When** the user runs `finfocus overview` in an interactive terminal,
   **Then** a red budget health footer appears showing spend/limit, percentage, and
   "EXCEEDED" status.

4. **Given** a stack with no budgets configured,
   **When** the user runs `finfocus overview`,
   **Then** no budget footer appears and the resource table renders unchanged.

5. **Given** the budget data fetch fails (plugin error, timeout),
   **When** the user runs `finfocus overview`,
   **Then** no budget footer appears, the resource table renders normally, and a
   warning is logged (not shown to the user in the TUI).

---

### User Story 2 - Budget Detail View in TUI (Priority: P2)

When viewing a resource detail (pressing Enter on a row), the detail view includes a
"BUDGET STATUS" section showing per-budget breakdown: budget name, limit, current spend,
forecasted spend, and any triggered alerts. This gives the user context on how the
overall stack budget health breaks down.

**Why this priority**: Enhances the detail view with actionable budget information.
Depends on P1 (budget data must already be fetched and available in the model).

**Independent Test**: Can be tested by navigating to the detail view of any resource
when budgets are loaded and verifying the BUDGET STATUS section displays correct
per-budget information.

**Acceptance Scenarios**:

1. **Given** the TUI has budget data loaded with two budgets (one OK, one WARNING),
   **When** the user presses Enter to view any resource detail,
   **Then** a "BUDGET STATUS" section appears showing each budget with name, limit,
   current spend, utilization percentage, forecasted spend, and health status.

2. **Given** a budget has triggered threshold alerts (e.g., 80% actual, 100% forecasted),
   **When** the user views the resource detail,
   **Then** the triggered alerts appear in the BUDGET STATUS section with alert type and
   threshold value.

3. **Given** no budget data is available (not configured or fetch failed),
   **When** the user views the resource detail,
   **Then** no BUDGET STATUS section appears in the detail view.

---

### User Story 3 - Plain Text Budget Status (Priority: P2)

When running `finfocus overview` in a non-interactive context (piped output, CI/CD,
`--plain` flag, or non-TTY terminal), the budget status box appears after the resource
table, matching the existing format used by `cost projected`. The `--exit-on-threshold`
flag controls whether budget exceedance causes a non-zero exit code.

**Why this priority**: Enables budget awareness in CI/CD pipelines and scripted
workflows. Shares priority with P2 because it serves a different audience (automation)
than P1 (interactive users).

**Independent Test**: Can be tested by piping `finfocus overview` output to a file and
verifying the budget status box appears after the resource table. Exit code behavior
can be tested independently with `--exit-on-threshold`.

**Acceptance Scenarios**:

1. **Given** a stack with budgets configured,
   **When** the user runs `finfocus overview --plain` or pipes to a file,
   **Then** the budget status box is printed after the resource table, showing budget
   name, limit, current spend, percentage, and status.

2. **Given** budget spend exceeds the limit and `--exit-on-threshold` is set,
   **When** the user runs `finfocus overview --plain --exit-on-threshold`,
   **Then** the command exits with the configured non-zero exit code (semantic exit code
   for budget exceedance).

3. **Given** budget spend exceeds the limit but `--exit-on-threshold` is NOT set,
   **When** the user runs `finfocus overview --plain`,
   **Then** the budget status is rendered but the command exits with code 0.

4. **Given** the user runs `finfocus overview` in TUI mode with `--exit-on-threshold`,
   **When** budgets are exceeded,
   **Then** the flag has no effect — the TUI displays the budget status visually but
   never auto-terminates.

---

### User Story 4 - Budget Data in JSON/NDJSON Output (Priority: P3)

When running `finfocus overview --output json` or `--output ndjson`, the output includes
budget health data in the stack context / metadata object, enabling programmatic
consumption of budget status by downstream tools and AI agents.

**Why this priority**: Supports machine-readable budget data for automation and
integration. Lower priority because it extends an already-functional JSON output path.

**Independent Test**: Can be tested by running `finfocus overview --output json` with
budgets configured and validating the JSON structure contains budget data in the
expected location.

**Acceptance Scenarios**:

1. **Given** a stack with budgets configured,
   **When** the user runs `finfocus overview --output json`,
   **Then** the JSON output includes a `budgets` array in the metadata/stack context
   containing budget health data (name, limit, spend, health status, forecasted spend).

2. **Given** a stack with no budgets configured,
   **When** the user runs `finfocus overview --output json`,
   **Then** the `budgets` field is absent or an empty array (no noise).

3. **Given** a stack with budgets configured,
   **When** the user runs `finfocus overview --output ndjson`,
   **Then** budget data is not included in per-resource lines (budgets are stack-scoped,
   not resource-scoped).

---

### Edge Cases

- What happens when budget data loads before the resource table is ready?
  Budget data is held in the model and rendered only after the list view is active.
- What happens when the budget fetch takes much longer than enrichment?
  The resource table is fully interactive; the budget footer appears whenever data arrives.
- What happens when mixed currencies exist across budgets?
  Each budget is rendered independently with its own currency. The overall health
  badge uses the worst-case health status regardless of currency.
- What happens when a budget has no limit set (limit = 0)?
  The budget is treated as disabled and excluded from the footer and detail view.
- What happens when multiple budget scopes exist (global, provider, tag)?
  The footer shows the overall worst-case health. The detail view shows all scopes
  if scoped budgets are configured.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST fetch budget data asynchronously in TUI mode without delaying
  the initial resource table render.
- **FR-002**: System MUST display a budget health footer in the TUI list view when
  budgets are available, showing: health badge (color-coded), spend vs limit, and
  utilization percentage.
- **FR-003**: System MUST use color coding for budget health: green for OK (0-79%),
  yellow for WARNING (80-89%), red for CRITICAL (90-99%), red bold for EXCEEDED (100%+).
- **FR-004**: System MUST display a "BUDGET STATUS" section in the TUI detail view
  showing per-budget breakdown with name, limit, current spend, forecasted spend,
  and triggered alerts.
- **FR-005**: System MUST hide the budget footer and detail section when no budgets
  are configured or when the budget fetch returns no results.
- **FR-006**: System MUST render budget status in plain text mode after the resource
  table, reusing the existing budget rendering format from `cost projected`.
- **FR-007**: System MUST support the `--exit-on-threshold` flag in non-TTY mode,
  causing a non-zero exit code when budgets are EXCEEDED or CRITICAL.
- **FR-008**: System MUST ignore `--exit-on-threshold` in TUI mode — interactive
  sessions must never auto-terminate due to budget status.
- **FR-009**: System MUST include budget health data in JSON output: a top-level
  `budgets` array with per-budget health details in `OverviewJSONOutput`, and a
  lightweight `budgetHealth` summary (overall health, total count, critical count)
  in the `StackContext` metadata object.
- **FR-010**: System MUST treat budget fetch failures as non-fatal — the overview
  continues without budget data and logs a warning.
- **FR-011**: System MUST NOT include per-resource budget columns in the resource
  table (budgets are stack-scoped, not resource-scoped).
- **FR-012**: System MUST NOT add new budget configuration syntax — all budget
  configuration reuses existing config patterns.

### Key Entities

- **Budget Health Footer**: A visual element in the TUI list view showing aggregate
  budget health. Contains a color-coded badge, total spend, total limit, utilization
  percentage, and overall health status label.
- **Budget Detail Section**: A section within the TUI resource detail view showing
  per-budget breakdown. Contains budget name, limit, current spend, forecasted spend,
  utilization percentage, health status, and triggered threshold alerts.
- **Budget Data Message**: An asynchronous message delivered to the TUI model when
  budget data finishes loading. Carries the complete budget result for rendering.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users running `finfocus overview` in a terminal see budget health status
  without running any additional commands, reducing the number of commands needed for
  daily cost monitoring from two to one.
- **SC-002**: Budget health footer appears in the TUI within the same loading cycle as
  resource enrichment — users do not perceive a separate loading step for budgets.
- **SC-003**: Plain text and JSON output modes include budget data, enabling CI/CD
  pipelines to detect budget exceedance from a single `finfocus overview` invocation.
- **SC-004**: Budget fetch failures do not degrade the existing overview experience —
  the resource table and all existing features continue to function normally.
- **SC-005**: All existing overview tests continue to pass with no regressions.
- **SC-006**: New budget integration achieves test coverage consistent with the
  project's coverage standards.

## Assumptions

- Budget configuration and plugin support already exist — this feature only wires
  existing budget capabilities into the overview command.
- The existing `engine.GetBudgets()` returns data compatible with the overview
  rendering requirements without modification.
- The TUI footer area has sufficient vertical space to display a single-line budget
  health summary without displacing the status bar or pagination controls.
- Budget data fetch latency is comparable to cost enrichment latency, so both can
  run concurrently without one significantly blocking the other.
- The existing `RenderBudgetStatus()` function in the CLI layer is suitable for
  plain text overview rendering without modification.
- The `--exit-on-threshold` flag semantics match exactly what `cost projected` uses
  today (same exit codes, same threshold logic).
