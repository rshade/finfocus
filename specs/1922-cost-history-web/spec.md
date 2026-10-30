# Feature Specification: Cost History on the Web

**Feature Branch**: `1922-cost-history-web`
**Created**: 2026-10-30
**Status**: Implemented
**Input**: User description: "Expose the Cost Time Machine (cost history view, diff, and resource series) through read-only web API hooks (#1860), without drawing that history in the page yet."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Read the stack's cost history in the browser (Priority: P1)

An engineer investigating a cost change opens the dashboard for a stack whose
history was collected. The session returns the stored history for that stack:
each snapshot's monthly total, provider and type breakdown, and resources,
the deployment notes beside them, and the mixed-currency warning. These are
the same values `finfocus cost history view --output json` prints for the same
start date, end date, currency, and strict setting. The response also carries
the configured budget, which the command-line chart draws as its threshold
line.

**Why this priority**: Without this, the only way to see when spend moved is
to leave the browser and run the command.

**Independent Test**: For one stack and one set of bounds, the returned
snapshots, annotations, and warning equal the command-line JSON.

**Acceptance Scenarios**:

1. **Given** a stack with collected snapshots, **When** the browser asks for
   its history with no bounds, **Then** every snapshot and annotation the
   command line prints is returned, in the same order and with the same values.
2. **Given** a start and an end date, **When** the browser asks for the
   history, **Then** only the snapshots and annotations inside those dates are
   returned, as on the command line.
3. **Given** snapshots in two currencies, **When** the browser asks without a
   currency, **Then** the dominant currency's snapshots and the same warning
   text are returned; **When** it asks for one currency, **Then** only that
   currency is returned with no warning; **When** it asks for strict
   handling, **Then** the request is refused with the command line's message.
4. **Given** a configured global budget, **When** the history is returned,
   **Then** it carries that budget amount.

---

### User Story 2 - Compare two snapshots (Priority: P1)

The engineer picks two snapshots, by version (`v35`) or by date, and the
session returns what changed between them: added, removed, and changed
resources with their monthly cost and change, the unchanged count and total,
and the overall change. The values equal
`finfocus cost history diff --output json` for the same selectors and
threshold.

**Why this priority**: The diff explains why the total moved; the series alone
only shows that it did.

**Independent Test**: For one stack, one pair of selectors, and one threshold,
the returned diff equals the command-line JSON.

**Acceptance Scenarios**:

1. **Given** two versions, **When** the browser asks for their diff, **Then**
   the added, removed, changed, and unchanged figures equal the command line's.
2. **Given** no end selector, **When** the browser asks for a diff, **Then**
   the newest snapshot is the end, as on the command line.
3. **Given** a threshold, **When** the browser asks for a diff, **Then**
   the changes at or below it are hidden, as on the command line.
4. **Given** a missing start selector, an unknown version, a malformed
   selector, or a negative threshold, **When** the browser asks, **Then** the
   request is refused with the command line's message.

---

### User Story 3 - See one resource's cost series (Priority: P2)

From a resource's detail panel on the Overview or Cost view, the browser asks
for that resource's stored cost series. The session returns the same points,
in the same order, that the terminal detail view loads when the user presses
Enter on that row.

**Why this priority**: It ties a single resource's history to the row the
user is already looking at, but the stack-level history (Stories 1 and 2) is
useful without it.

**Independent Test**: For one resource id, the returned points equal the
terminal detail view's points.

**Acceptance Scenarios**:

1. **Given** a resource that appears in stored snapshots, **When** the
   browser asks for its series, **Then** one point per snapshot is returned
   with the same time, amount, and currency the terminal shows.
2. **Given** a resource that the history database does not mention, **When**
   the browser asks, **Then** an empty series is returned, not an error.
3. **Given** a request with no resource id, **When** the browser asks,
   **Then** the request is refused.

---

### User Story 4 - Tell "no history yet" apart from a failure (Priority: P2)

A stack that was never collected, or a session with no stack to look up, is
answered with a "no history" status that the page can turn into a hint. A
database that exists but cannot be read is a failure the page can show. The
failure message does not include a file path or the underlying error text.

**Why this priority**: Most stacks have no history until someone runs
`cost history collect`; that must not look like a broken dashboard.

**Independent Test**: With no database file, every history request answers
"no history". With a damaged database file, every history request answers
with a failure that has no path in it.

**Acceptance Scenarios**:

1. **Given** no history database for the session stack, **When** the browser
   asks for the history, a diff, or a resource series, **Then** each answers
   with a "no history" status.
2. **Given** a database with no snapshots, **When** the browser asks for a
   diff, **Then** the answer is "no history", not an input error.
3. **Given** a damaged database, **When** the browser asks, **Then** the
   answer is a failure whose message names no path.
4. **Given** the session has not finished loading, **When** the browser asks,
   **Then** it is told the data is still loading.

---

### Edge Cases

- A date range whose end is before its start is refused with the command
  line's message.
- A range that holds no snapshots returns an empty history with an ok status,
  as the command line does.
- A database recorded for a different stack name than the session's is a
  failure, as on the command line, and its message names no path.
- A resource type that contains a credential-looking word, such as
  `aws:secretsmanager/secret:Secret`, keeps its breakdown entry in the
  response.
- A Terraform session resolves its history identity the same way the
  command line does from that project directory.
- Cancelling the browser request stops the read.
- The dashboard never writes, collects, prunes, or exports history.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The browser session MUST return a stack's stored cost history
  with the same snapshots, annotations, and currency warning as
  `cost history view --output json` for the same start, end, currency, and
  strict values.
- **FR-002**: The history response MUST carry the configured global budget
  amount when one is set, the same amount the command-line chart draws.
- **FR-003**: The browser session MUST return the diff of two snapshots with
  the same values as `cost history diff --output json` for the same start
  selector, end selector, and threshold.
- **FR-004**: The browser session MUST return one resource's cost series with
  the same points the terminal detail view loads for that resource id.
- **FR-005**: The session MUST use the session's stack and the command line's
  history-directory rules, including the project-tier safety checks. It MUST
  NOT add a second way to locate the history store.
- **FR-006**: A missing database, a session with no stack, or a diff over a
  database with no snapshots MUST return a "no history" status, not a failure.
- **FR-007**: Invalid dates, a reversed range, a missing diff start, an
  unknown or malformed selector, a negative threshold, and a strict request
  over mixed currencies MUST be refused with the command line's message for
  the same input. A non-numeric threshold or a non-boolean strict value MUST
  be refused with a message that names the value.
- **FR-008**: A database that cannot be read, or that belongs to another stack,
  MUST be reported as a failure with a fixed message that names no path and
  contains no underlying error text.
- **FR-009**: The web layer and the page MUST NOT compute history values. Every
  value comes from the same functions the command line uses; the session adds
  only formatted display strings.
- **FR-010**: Provider, type, and deployment-change breakdown keys MUST appear
  in the response unchanged. Every response MUST still pass through the shared
  payload redactor; the history documents hold no property maps or free-form
  values, so the breakdown keys are the only part it could have removed.
- **FR-011**: The command-line output of `cost history view` and
  `cost history diff`, in every format, MUST stay the same.
- **FR-012**: The dashboard MUST NOT write to the history store.

### Key Entities

- **Snapshot**: One stored checkpoint of a stack: time, version, monthly
  total, currency, resource count, provider and type breakdowns, and priced
  resources.
- **Annotation**: The deployment note stored beside a snapshot: version,
  message, kind, and resource-change counts.
- **Diff**: Two snapshot points and the added, removed, changed, and unchanged
  resources between them.
- **Resource point**: One resource's monthly cost at one snapshot time, with
  its currency.
- **History status**: Whether the answer holds history (`ok`) or there is
  nothing to read yet (`no_history`).

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the Overview and Cost detail panels, and the Cost
  view. `cost history collect`, `prune`, and `export`: Not applicable - they
  write or delete the history store, or write a file, and the dashboard stays
  read-only toward the history store.
- **Web data**: The stack's snapshots (time, version, total, provider and type
  breakdowns), deployment annotations, currency warning, and the configured
  budget for the history chart; a diff between two chosen snapshots; and one
  resource's points for its detail panel. Display strings come with each
  value. The page computes nothing.
- **Layout fit**: A `Cost history` section in the existing Overview and Cost
  detail panels, under the current resource details: a captioned table of
  time and monthly cost, with the existing alert line for "no history" and
  failures. On the Cost view, a history panel below the table (slot 7): the
  series with the budget line and annotation markers, a from/to/currency
  controls row, and a two-selector diff table (added, removed, changed) with
  its totals bar.
- **Additions**: A chart for the series, budget line, and annotation markers.
  A "no history" hint that names `finfocus cost history collect`.
- **Render evidence**: 2026-10-30, `~/tailscale-phase-2/initial-30`, stack
  `dev`, with `FINFOCUS_HISTORY_DIR` holding history collected from that
  stack (v126, v127, v130, all destroy checkpoints at $0) and from
  `../finfocus-demo` (`dev`, five snapshots, two at $9.99). Overview showed
  the resource table, actual MTD $59.67, projected $141.03; the
  `windowsClientInstance` detail panel showed URN, type, status, costs,
  recommendations, and blast radius, and no history section. Cost showed
  total actual $135.77 over 16 resources and a detail panel with type,
  provider, total, period, breakdown, linked recommendations, and notes.
  From that session, `GET /api/history/view` returned 200 with three
  snapshots, three destroy annotations, and the $120.00 USD budget;
  `/api/history/diff?from=v126&to=v130` returned 200 with no changes;
  `/api/history/resource` for the demo's `demo-instance` URN returned two
  points at $8.39 USD; `from=v9` was 400 `snapshot version 9 not found`,
  `strict=maybe` was 400, and a missing id was 400. Both detail panels have
  room for a `Cost history` section after their last block, and the Cost
  view has the slot-7 area below its table for the chart.
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work. Filed as
  #1939.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For every tested combination of bounds, currency, and strict
  setting, the browser history equals the command-line history JSON value for
  value.
- **SC-002**: For every tested selector pair and threshold, the browser diff
  equals the command-line diff JSON value for value.
- **SC-003**: For a tested resource, the browser series equals the terminal
  detail view's points, including order.
- **SC-004**: With no history database, 3 of 3 history requests answer "no
  history"; with a damaged database, 3 of 3 answer with a failure whose message
  contains no path.
- **SC-005**: The command-line `cost history view` and `cost history diff`
  tests pass unchanged.

## Assumptions

- The session stack is the stack the dashboard loaded, the same one the
  forecast reads history for.
- The browser request uses the command line's selector syntax (`v35`,
  `YYYY-MM-DD`) and date syntax (`YYYY-MM-DD` or RFC 3339), so the command
  line's messages fit the browser's input.
- `--provider`, `--split-providers`, `--no-annotations`, `--height`, and
  `--width` shape only the terminal chart. The browser receives every provider
  and annotation and draws them itself.
- A resource id that names another stack in its URN reads that stack's
  history, as the terminal detail view does.
- Drawing the chart and the detail-panel section is a later UX change. This
  feature only makes the data available.
