# Feature Specification: Cost Anomalies Command

**Feature Branch**: `1923-cost-anomalies`
**Created**: 2026-10-30
**Status**: Draft
**Input**: User description: "Add a flag-only `finfocus cost anomalies`
command (GitHub issue #1590) that lists overview drift warnings as
machine-readable rows in table, JSON, and NDJSON, exposed as an MCP tool, with
one shared selection function that the web dashboard also calls, and an
Anomalies page in the dashboard."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - List what looks off right now (Priority: P1)

A person runs one command and sees only the resources whose month-to-date
run-rate has diverged from the projected month, instead of reading every row
of the overview.

**Why this priority**: This is the whole feature. Without it, "what looks off
right now?" needs the full overview payload.

**Independent Test**: Run the command against a fixture stack where some
resources drift past the warning threshold and others do not. The output lists
exactly the drifting resources, each with baseline, observed, deviation, and
percent drift.

**Acceptance Scenarios**:

1. **Given** a stack where two of ten resources drifted past the overview
   warning threshold, **When** the person runs `finfocus cost anomalies`,
   **Then** the output has two rows and reports that ten rows were considered.
2. **Given** a stack where no resource drifted, **When** the command runs,
   **Then** it exits 0 with an empty list and a `considered` count above zero,
   so an empty result differs from a failed load.
3. **Given** a drifting resource, **When** the command prints JSON, **Then**
   the row carries `resource`, `type`, `period`, `baseline`, `observed`,
   `deviation`, `percentDrift`, `severity` (`warning`), `method` (`drift`),
   and `currency`.
4. **Given** the same stack, **When** the person asks for table, JSON, or
   NDJSON, **Then** all three show the same anomalies.

---

### User Story 2 - An agent asks over MCP (Priority: P1)

An MCP client calls a single tool and receives the short anomaly list.

**Why this priority**: The issue exists so agents can ask this question
cheaply. The MCP guide's `detect_anomalies` row is only partial today.

**Independent Test**: The tool list pinned for the MCP server contains
`finfocus-cost-anomalies`, and a call returns the same JSON as the command.

**Acceptance Scenarios**:

1. **Given** the MCP server is running, **When** a client lists tools,
   **Then** `finfocus-cost-anomalies` appears.
2. **Given** the tool is called with filters, **When** it returns, **Then**
   the rows match the command run with the same flags.

---

### User Story 3 - Narrow the window and the resource set (Priority: P2)

A person limits the run by stack, saved plan or state file, adapter, date
range, or resource filter, using the same inputs the overview takes.

**Why this priority**: Large stacks need narrowing. The inputs already exist
on the overview, so reuse keeps the command predictable.

**Independent Test**: Run with `--filter type=...` and a date range, and see
only matching resources.

**Acceptance Scenarios**:

1. **Given** `--filter type=aws:ec2/instance:Instance`, **When** the command
   runs, **Then** only resources of that type are considered and listed. The
   filter keys are the overview's: `type`, `status`, and `provider`.
2. **Given** an unknown `--output` value, **When** the command runs, **Then**
   it fails with a validation error (exit 2) before any state is loaded.
3. **Given** a malformed date, **When** the command runs, **Then** it fails
   with a validation error (exit 2).

---

### User Story 4 - The dashboard serves the same list (Priority: P2)

The web dashboard can request the anomaly list for the stack it already
loaded, with no second pricing run.

**Why this priority**: Constitution Principle VII requires a web hook for a
user-facing CLI feature.

**Independent Test**: A handler test feeds the session rows and the CLI
fixture through both paths and gets the same anomalies.

**Acceptance Scenarios**:

1. **Given** a loaded dashboard session, **When** the anomalies endpoint is
   requested, **Then** it returns the list computed from the session's current
   overview rows without starting another overview run.
2. **Given** the same rows, **When** the endpoint and the CLI JSON are
   compared, **Then** every anomaly field matches. The endpoint adds display
   text for the page and nothing else.

---

### User Story 5 - See the anomalies in the dashboard (Priority: P2)

A person opens the Anomalies page in `finfocus --web` and sees the same list
the command prints, laid out like the Overview, without leaving the browser.

**Why this priority**: The web dashboard is a first-class surface. The page
ships with the command rather than as a later ticket, so the feature is
complete when it merges.

**Independent Test**: Load the page against a session with drifting and steady
rows. A renderer test and a Chromium test show the table, the count line, and
the empty state.

**Acceptance Scenarios**:

1. **Given** a session with two anomalies among ten rows, **When** the person
   opens the Anomalies page, **Then** a table lists the two anomalies with
   resource, type, period, baseline, observed, deviation, drift, and currency,
   and a status line reads `2 of 10 resources`.
2. **Given** a session where nothing drifted, **When** the page loads, **Then**
   it says no anomalies were found and shows how many resources were
   considered.
3. **Given** the overview is still loading or its rows change (a preview is
   run), **When** the session reports ready or a new snapshot, **Then** the
   page refreshes its list.
4. **Given** the request fails, **When** the page handles the error, **Then**
   an alert shows a short message and the previous table is not shown as
   current.
5. **Given** a keyboard or screen reader user, **When** they use the page,
   **Then** the table has a caption and column headers, the status line is
   announced, and the scrollable table region is reachable by keyboard.

---

### Edge Cases

- Drift at or under 10%, elapsed time under 2 days, a new resource (no actual
  spend yet), and a deleted resource (no projection) yield no row, because the
  existing drift calculation returns no drift for them. A resource a preview
  marks for deletion also yields no row, but enrichment still prices it, so
  the selector checks its status. The command has no other rule of its own.
- A row with a plugin error is not an anomaly, even when it has drift data.
- Resources in different currencies are listed separately. No total ever sums
  across currencies; any table total is per currency.
- An empty stack exits 0 with an empty list and `considered` of 0.
- A row whose actual-cost period is missing still lists, with an empty period.
  The page shows a dash for it.
- Before the session has any rows, the page shows its loading state, not the
  empty-result message.
- NDJSON emits one anomaly per line; an empty list emits no lines.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide `finfocus cost anomalies` that takes no
  positional arguments and lists overview rows that carry drift data.
- **FR-002**: The command MUST accept `--stack`, `--pulumi-json`,
  `--pulumi-state`, `--from`, `--to`, `--adapter`, `--filter`, and `--output`
  with the same meaning as the overview, including the overview's filter keys
  (`type`, `status`, `provider`). Dates accept `2006-01-02` and RFC3339.
- **FR-003**: The command MUST support table, JSON, and NDJSON output and MUST
  NOT open an interactive screen.
- **FR-004**: Each anomaly MUST report `resource` (the resource URN), `type`,
  `period`, `baseline` (projected monthly cost normalized to the calendar
  month), `observed` (extrapolated monthly cost), `deviation`, `percentDrift`,
  `severity` (always `warning`), `method` (always `drift`), and `currency`.
- **FR-005**: `observed` MUST NOT be labelled or computed as raw month-to-date
  spend.
- **FR-006**: The command MUST NOT reimplement the 10% threshold, calendar
  month normalization, or the early-month, new-resource, and deleted-resource
  skips. It selects rows whose existing drift data is present.
- **FR-007**: A row with an error, with no drift data, or with a status of
  deleting (a preview marks it for deletion) MUST NOT be listed.
- **FR-008**: JSON output MUST include `considered` (rows the overview
  produced) alongside the anomaly array. An empty list MUST exit 0.
- **FR-009**: The command MUST NOT sum amounts across currencies.
- **FR-010**: An unknown `--output` MUST be rejected with exit 2 before state
  is loaded.
- **FR-011**: The selection MUST live in one shared Go function that both the
  command and the web handler call. The command MUST price through the
  overview pipeline in-process and MUST NOT shell out to the overview command.
- **FR-012**: The command MUST be exposed as the MCP tool
  `finfocus-cost-anomalies`, listed in the pinned tool list. The MCP guide's
  `detect_anomalies` entry MUST name it.
- **FR-013**: The web handler MUST call the shared function on the session's
  current rows, MUST NOT start a second overview pipeline, and MUST NOT import
  the CLI package. The browser MUST do no arithmetic and no number, currency,
  date, or percentage formatting.
- **FR-016**: The dashboard MUST have an Anomalies page, linked from the
  navigation, built from the same layout slots as the other views.
- **FR-017**: Every amount, percentage, and period the page shows MUST come
  from display text computed in Go by a shared function. The anomaly fields
  themselves stay identical to the CLI's.
- **FR-018**: The page MUST show a count line (`N of M resources`), an empty
  state that includes the considered count, a loading state, and an alert for
  a failed request.
- **FR-019**: The page MUST refresh when the overview session reports ready or
  publishes a new snapshot, and MUST ignore a response that arrives after the
  page was left or a newer request started.
- **FR-020**: The page MUST be operable by keyboard and screen reader: a table
  caption, column header scopes, a live status line, an alert role for errors,
  and a focusable scroll region.
- **FR-014**: Plugin credentials and Pulumi secrets MUST NOT appear in the
  output; the rows carry only the fields in FR-004.
- **FR-015**: Documentation MUST include a command page, a user-guide section,
  and the updated MCP guide entry.

### Key Entities

- **Anomaly**: One resource whose run-rate drift is over the warning
  threshold. Attributes are the FR-004 fields.
- **Anomaly report**: The count of rows considered plus the ordered list of
  anomalies, with the output format applied.

## Web UI Fit *(mandatory)*

- **Disposition**: New view (an Anomalies page in the existing dashboard),
  shipped with this feature together with its data hook.
- **Web data**: The same anomaly rows and `considered` count as the command:
  resource, type, period, baseline, observed, deviation, percent drift,
  severity, method, currency, plus display text for amounts, percentage, and
  period built in Go. The page computes and formats nothing.
- **Layout fit**: The Overview slots in order: header with a new `Anomalies`
  link, heading, a status line (`N of M resources`, loading, errors), and a
  captioned table in the Overview table style with right-aligned numeric
  columns and a scrollable wrapper. No toolbar: the list has no filter, sort,
  or run control, and rows keep the order the shared function returns. No
  totals bar: amounts can be in different currencies.
- **Additions**: None. No detail panel, no link to the Overview row, and no
  pager.
- **Render evidence**: 2026-10-30, real Chromium through `make test-e2e-web`
  against the deterministic fixture (a deployed `aws:ec2/instance:Instance`,
  state-only, window 2026-10-01 to 2026-10-07; no cloud stack or Pulumi CLI
  was available). The Anomalies page was compared with the Overview in light
  and dark at 1280 and 390 pixels. It uses the same slots: header with an
  `Anomalies` link, heading, status line, and a captioned table. The status
  line read `1 of 1 resources`, and the row matched
  `finfocus cost anomalies --output json` for the same state and window:
  baseline $30.58, observed $77.50, deviation +$46.92, drift +153.5%, USD,
  period Oct 1 – Oct 7, 2026. Numeric columns and their headers are
  right-aligned, a long URN wraps, and the table scrolls inside a
  keyboard-focusable wrapper at 390 pixels. The contrast and overflow checks
  passed, and the empty state read `No anomalies found (0 of 1 resources).`
  Earlier, a fixture state priced by the recorder plugin showed the Overview
  with 49 rows, and the endpoint answered `considered: 49` like the CLI.
- **Follow-up issue scope**: None. The page ships in this feature, so no
  `web-ui` follow-up issue is filed. Constitution Principle VII asks for a
  UX-only follow-up when a feature ships without its page; delivering the page
  here meets the principle's purpose, and the pull request says so.

## Assumptions

- Issue #1590 mentions `tag:` filters. The overview pipeline rejects them
  today, and this command reuses that pipeline, so it accepts the overview's
  keys only. Widening the overview filter is a separate change.
- Without `--pulumi-json`, the command reads deployed state only and does not
  run `pulumi preview`: drift compares actual spend with deployed resources.

- Ordering is by absolute deviation, largest first, then resource URN, so
  output is stable.
- Severity has one band; thresholds in configuration are out of scope.
- The command is exactly as slow as `finfocus overview --output json`.

## Out of Scope

- A stored daily actual-cost series (#1862) and robust median or z-score
  detection (#1863).
- Scorer-based triage of drift.
- `cost history diff`, forecasting, and changes to the drift calculation or
  its threshold.
- Severity thresholds in `config.hujson`.
- Anomaly drill-down: a detail panel, a link from a row to the Overview
  detail, and sort or filter controls on the page.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A person gets the list of drifting resources from one command,
  with at most one row per drifting resource and none for resources inside the
  threshold.
- **SC-002**: For every fixture, the command, the MCP tool, and the web
  endpoint return identical anomaly sets (100% match).
- **SC-003**: An empty result is distinguishable from a failed load in 100% of
  runs: empty lists exit 0 with a `considered` count, failures exit nonzero.
- **SC-004**: No output ever contains a total that mixes currencies.
- **SC-005**: An invalid `--output` fails before any stack state is read.
- **SC-006**: The Anomalies page lists exactly the anomalies the command
  prints for the same rows, and a person can reach it from the dashboard
  header in one click.
- **SC-007**: The page can be used with the keyboard alone, and its table is
  announced with a caption and column headers.
