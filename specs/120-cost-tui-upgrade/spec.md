# Feature Specification: Cost Commands TUI Upgrade

**Feature Branch**: `106-cost-tui-upgrade`
**Created**: 2025-12-25
**Status**: Draft
**Input**: GitHub Issue #218 - Upgrade cost commands to Bubble Tea/Lip Gloss for enhanced TUI

## Clarifications

### Session 2025-12-25

- Q: Where does cost delta baseline data come from? → A: Deltas come from existing `CostResult.Delta` field provided by plugins or engine (no local caching or baseline files)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Projected Costs with Styled Summary (Priority: P1)

A developer runs `finfocus cost projected --pulumi-json plan.json` and sees a beautifully styled cost summary with provider breakdowns, total costs, and a resource table - all with consistent color coding that makes high-cost resources immediately visible.

**Why this priority**: This is the most common use case and provides immediate visual improvement over plain tabwriter output. It demonstrates the value of the TUI upgrade with minimal complexity.

**Independent Test**: Can be fully tested by running the projected cost command with a sample Pulumi plan and verifying styled output renders correctly in a TTY terminal.

**Acceptance Scenarios**:

1. **Given** a valid Pulumi plan JSON, **When** the user runs `cost projected` in a TTY terminal, **Then** they see a styled cost summary box with total monthly cost, resource count, and provider breakdown using the existing TUI color scheme (purple headers, white values, green/orange/red for cost indicators).

2. **Given** a valid Pulumi plan JSON, **When** the user runs `cost projected` with stdout redirected to a file, **Then** they receive plain text table output identical to current behavior.

3. **Given** a valid Pulumi plan JSON with resources from multiple providers, **When** the user views the summary, **Then** costs are grouped by provider with percentage breakdowns.

---

### User Story 2 - Navigate Interactive Resource Table (Priority: P2)

A platform engineer views cost results and navigates through a scrollable resource table using keyboard controls. They can scroll through resources sorted by cost and press Enter to see detailed breakdowns.

**Why this priority**: Interactive navigation adds significant value for users with many resources, enabling them to quickly identify costly items without scrolling through pages of output.

**Independent Test**: Can be tested by running the command with a plan containing 10+ resources and verifying arrow key navigation, selection highlighting, and quit functionality.

**Acceptance Scenarios**:

1. **Given** cost results with more than 5 resources, **When** the user is in interactive mode, **Then** they see a navigable table with up/down arrow key controls and visual selection indicator.

2. **Given** the user is viewing the resource table, **When** they press `q`, **Then** the application exits cleanly without errors.

3. **Given** the user is viewing the resource table, **When** they press Enter on a selected resource, **Then** they see a detailed view of that resource's cost breakdown.

4. **Given** the user is in resource detail view, **When** they press Escape, **Then** they return to the resource list.

---

### User Story 3 - View Actual Historical Costs with TUI (Priority: P2)

A FinOps analyst runs `finfocus cost actual --pulumi-json plan.json --from 2025-01-01` and sees styled historical cost data with the same visual consistency as projected costs, including time-based grouping tables for daily/monthly views.

**Why this priority**: Actual costs are critical for FinOps workflows and should have visual parity with projected costs. Time-based aggregations particularly benefit from styled presentation.

**Independent Test**: Can be tested by running the actual cost command with date range parameters and verifying styled output for both individual resources and cross-provider aggregations.

**Acceptance Scenarios**:

1. **Given** a valid Pulumi plan and date range, **When** the user runs `cost actual`, **Then** they see styled output with actual costs, period information, and currency symbols.

2. **Given** the user runs `cost actual --group-by daily`, **When** viewing cross-provider aggregation, **Then** they see a styled table with date column, provider columns, and total column.

3. **Given** the user runs `cost actual --group-by monthly`, **When** viewing monthly aggregation, **Then** they see a styled table with month labels instead of dates.

---

### User Story 4 - See Loading Progress During Plugin Queries (Priority: P3)

A user runs a cost command that queries multiple plugins. While waiting, they see an animated spinner with status indicators showing which plugins are responding.

**Why this priority**: Loading feedback improves perceived performance and provides confidence that the tool is working, especially when plugins are slow to respond.

**Independent Test**: Can be tested by running cost command with mock plugins that introduce artificial delays and verifying spinner animation and status updates.

**Acceptance Scenarios**:

1. **Given** the user runs a cost command that requires plugin queries, **When** plugins are being queried, **Then** a spinner animation appears with "Querying cost data from plugins..." message.

2. **Given** multiple plugins are configured, **When** a plugin completes, **Then** that plugin shows a checkmark with resource count while other plugins continue loading.

3. **Given** a plugin fails to respond, **When** the loading phase completes, **Then** failed plugins show a warning indicator with error summary.

---

### User Story 5 - View Cost Deltas in Projected Costs (Priority: P3)

A developer viewing projected costs can see delta indicators (arrows and color coding) showing whether costs are increasing, decreasing, or unchanged. Delta values are sourced from the `CostResult.Delta` field provided by plugins or the engine.

**Why this priority**: Cost deltas provide actionable insights when available from plugins. This builds on the core styled output and uses existing data without additional storage requirements.

**Independent Test**: Can be tested by providing cost results with delta values and verifying up/down arrows with appropriate colors (green for decrease, orange/red for increase).

**Acceptance Scenarios**:

1. **Given** cost results with positive delta values, **When** displayed in the resource table, **Then** the delta shows as orange "+$X.XX" with up arrow.

2. **Given** cost results with negative delta values, **When** displayed in the resource table, **Then** the delta shows as green "$X.XX" with down arrow (indicating savings).

3. **Given** cost results with zero delta, **When** displayed in the resource table, **Then** the delta shows as gray "$0.00" with right arrow.

---

### User Story 6 - Sort and Filter Resources Interactively (Priority: P4)

A user viewing the resource table can press keyboard shortcuts to sort by different columns (cost, name, type) or filter resources by typing a search query.

**Why this priority**: Sorting and filtering are power-user features that enhance productivity but require more complex interaction patterns.

**Independent Test**: Can be tested by loading a resource table and verifying sort key changes column order and filter input reduces visible rows.

**Acceptance Scenarios**:

1. **Given** the user is viewing the resource table, **When** they press `s`, **Then** they see a sort menu with options for cost, name, and type.

2. **Given** the user is viewing the resource table, **When** they press `/` and type a filter query, **Then** only matching resources are displayed.

3. **Given** a filter is active, **When** the user presses Escape in the filter input, **Then** the filter is cleared and all resources are shown.

---

### Edge Cases

- What happens when there are no resources to display? Show an empty state message: "No resources found in plan"
- What happens when all plugins fail? Display styled error summary and fall back to local specs if available
- How does the system handle very long resource names? Truncate to 40 characters with "..." as currently done
- What happens when the terminal is very narrow (< 60 chars)? Fall back to plain text output
- How does the system handle non-UTF-8 terminals? Use ASCII-only fallback characters for icons

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST detect terminal capabilities (TTY, color support, width) to select appropriate output mode
- **FR-002**: System MUST render styled cost summaries using Lip Gloss when in TTY mode with color support
- **FR-003**: System MUST provide interactive resource navigation using Bubble Tea when in full interactive mode
- **FR-004**: System MUST maintain identical output for `--output json` and `--output ndjson` flags (backward compatibility), regardless of TTY status
- **FR-005**: System MUST fall back to plain text table output when stdout is redirected or TTY is unavailable
- **FR-006**: System MUST display loading spinners during asynchronous plugin queries in interactive mode
- **FR-007**: System MUST use consistent color scheme matching existing TUI components (ColorOK=82, ColorWarning=208, ColorCritical=196, ColorHeader=99)
- **FR-008**: System MUST support keyboard navigation: arrow keys for navigation, Enter for detail view, Escape for back, q for quit
- **FR-009**: System MUST render cost deltas with directional indicators and color coding
- **FR-010**: System MUST apply TUI styling to both `cost projected` and `cost actual` commands
- **FR-011**: System MUST respect the NO_COLOR environment variable to disable all styling

### Key Entities

- **CostView Model**: Bubble Tea model representing the cost display state (list view vs detail view)
- **ResourceRow**: Individual resource in the interactive table with name, type, cost, and delta (delta sourced from `CostResult.Delta` field)
- **CostSummary**: Aggregated cost data for styled summary box (total, by provider, by service)
- **LoadingState**: State for spinner animation with per-plugin status tracking

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can identify the highest-cost resource within 3 seconds of viewing output (vs scanning full table)
- **SC-002**: Interactive mode allows navigation of 100+ resources without performance degradation (< 100ms response to keypress)
- **SC-003**: 100% of existing CLI tests pass after upgrade (no behavioral regression for non-interactive output)
- **SC-004**: Users in CI/CD pipelines receive consistent plain-text output with no ANSI escape codes when stdout is redirected
- **SC-005**: Loading feedback appears within 100ms of command execution when plugins are queried
- **SC-006**: Cost trend indicators (up/down arrows) are visible and color-coded for 90% of terminal configurations

## Assumptions

- Terminal width is at least 60 characters for styled output (narrower terminals use plain text)
- Users have terminals supporting basic ANSI colors (256-color mode preferred but not required)
- The existing `internal/tui` package components (Spinner, Table, styles, colors) are production-ready
- Bubble Tea v1.2.4 (already a transitive dependency) is sufficient for interactive features
- Plugin query operations are async-safe and can report completion status
- The `cost estimate` command mentioned in the issue does not exist and refers to `cost projected`

## Out of Scope

- Custom theme configuration (uses existing hardcoded color scheme)
- Mouse support for table navigation
- Persistent user preferences for output mode
- Export to PDF or HTML formats
- Real-time cost monitoring (live updates)
- Cost comparison between different plan versions
