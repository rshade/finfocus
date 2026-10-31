# Feature Specification: Daily Actual-Cost Series

**Feature Branch**: `issue-1862`
**Created**: 2026-10-31
**Status**: Implemented
**Input**: User description: "Store one daily actual-cost point per resource so a later anomaly method can compare a day with that resource's own history (GitHub issue #1862)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Record each resource's days (Priority: P1)

An operator who already runs the projected-cost collect step after deploys
runs a second command. It asks the same pricing plugins for actual cost and
stores one amount per resource, per UTC day, per currency.

**Why this priority**: Without this series, a later check cannot tell a
one-day spike from a change in the program. The projected timeline only
moves when someone deploys.

**Independent Test**: Run the command against a fixture whose plugins return
a daily series. The stored points match those days and amounts, and the
command prints how many resources were stored, how many were skipped, and
which resources failed.

**Acceptance Scenarios**:

1. **Given** a stack whose plugins return a daily amount for each day in the
   window, **When** the operator runs `finfocus cost history collect-daily`,
   **Then** each resource has one stored point per returned day and the
   summary counts those resources.
2. **Given** the flags are omitted, **When** the command runs, **Then** the
   window is the last 30 complete UTC days and does not include the current
   partial day.
3. **Given** `--from` and `--to` in `YYYY-MM-DD` or RFC3339, **When** the
   command runs, **Then** it prices that window the same way `finfocus cost
   actual` accepts dates.
4. **Given** `--output json`, **When** the command finishes, **Then** the
   summary has `storedResources`, `storedPoints`, `skippedResources`, and
   `errors`. `--output table` prints the same counts.

---

### User Story 2 - Correct a day without erasing the rest (Priority: P1)

Billing data arrives late. The operator runs the command again over a short
window. Days the plugin sends replace the stored amount. Every other stored
day stays.

**Why this priority**: A collect that deleted days outside the response would
make a short correction wipe the baseline the follow-up issue needs.

**Independent Test**: Store two days, collect again with only the second day
and a new amount, and read both days back.

**Acceptance Scenarios**:

1. **Given** a stored day outside the new response, **When** the operator
   collects again, **Then** that day and its amount are unchanged.
2. **Given** a stored day that the new response includes, **When** the
   operator collects again, **Then** the stored amount is the new amount.
3. **Given** one resource fails at the plugin, **When** other resources
   succeed, **Then** the failed resource gains no points and loses none, the
   successes are stored, and the command reports the failure.

---

### User Story 3 - Absence is not zero (Priority: P2)

A plugin that omits the daily series, or a day the plugin does not send, is
not recorded as zero. A zero the plugin does send is stored as zero. Two
currencies for one resource stay two series.

**Why this priority**: The follow-up issue treats a gap as "no observation".
A filled-in zero would look like a free day.

**Independent Test**: Price four fixtures: a dense series, a missing series,
a series that contains a zero, and two results that share a resource and a
day but use different currencies.

**Acceptance Scenarios**:

1. **Given** a result with no daily series, **When** the command stores
   points, **Then** that resource gains nothing and is counted as skipped.
   Its window total is not spread across the days.
2. **Given** a plugin total with no timestamps, and an even split of that
   total, **When** the command stores points, **Then** that resource gains
   nothing. The even split is not stored.
3. **Given** a daily amount of zero, **When** the command stores points,
   **Then** that day is stored as zero.
4. **Given** two currencies for one resource on one day, **When** the command
   stores points, **Then** both amounts remain and are never added together.

---

### User Story 4 - The projected timeline stays put (Priority: P2)

The operator's existing projected-cost database is unchanged in schema and
contents. `finfocus cost history view` and `finfocus cost history diff` keep
working. The new series lives in a separate file in the same history
directory and uses the same directory jail.

**Why this priority**: Mixing the series into the checkpoint database would
couple billing collection to a command that refuses to store an understated
checkpoint.

**Independent Test**: Seed a projected timeline, run the daily command, and
open the timeline again. Its snapshots match. Opening either file as the
other store fails.

**Acceptance Scenarios**:

1. **Given** an existing projected timeline, **When** daily points are
   stored, **Then** that file's snapshots and schema are unchanged.
2. **Given** a project-tier history directory that is absolute, contains
   `..`, or is a symlink escape, **When** the command resolves the
   directory, **Then** it ignores that directory with the same warning cost
   history already uses.
3. **Given** the daily file is a symlink, **When** the command opens it,
   **Then** the open fails and the link target is not written.
4. **Given** the command is registered, **When** an MCP client lists tools,
   **Then** `cost history collect-daily` is absent, and `cost history
   collect` stays absent too.

### Edge Cases

- A plugin sends a total and no timestamps: that resource is skipped and
  nothing is stored for it.
- A plugin sends a non-numeric amount: that resource is an error and nothing
  is stored for it.
- An empty stack stores nothing new, prints a zero summary, and exits 0.
- A blank currency is stored as USD, matching other cost commands. A second
  named currency is a second series.
- `--to` without `--from` is a validation error (exit 2). An unknown
  `--output` is a validation error before any stack is read.
- Only `--from` prices through the current time, so a plugin that includes
  the current partial day is stored.
- `cost.history.retention.auto_prune` with `max_age_days` greater than 0
  deletes daily points older than that many days after a collect.
  `max_snapshots` does not apply to daily points, because a snapshot count
  is the wrong unit. `cost.history.retention_days` stays the
  resource-observation cleanup and is not a second rule for this file.
- The command does not detect anomalies, print a baseline, or change
  `finfocus cost anomalies`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a flag-only command, `finfocus cost
  history collect-daily`, separate from `finfocus cost history collect`.
- **FR-002**: The command MUST accept `--stack`, `--from`, `--to`,
  `--pulumi-json`, `--pulumi-state`, and `--terraform-state`. Resource
  discovery MUST match `finfocus cost actual` for the current project and
  those input files. It MUST send no new plugin request shape.
- **FR-003**: With `--from` and `--to` omitted, the window MUST be the last
  30 complete UTC days, ending at the start of the current UTC day.
- **FR-004**: The system MUST store one point as one resource id (the same
  id `finfocus cost actual` uses), one UTC day, one currency, and one
  amount, plus the time it was written.
- **FR-005**: Stored days MUST come from the plugin's timestamped cost
  buckets. Buckets that share a UTC day and a currency MUST be summed. A
  bucket with no timestamp MUST NOT become a day. The even split of a window
  total that `finfocus cost actual` shows MUST NOT be stored. A non-finite
  amount MUST store nothing for that resource and MUST report an error.
- **FR-006**: A missing daily series MUST store nothing for that resource and
  MUST NOT split the window total across the days. A day that is not in the
  series MUST NOT be stored as zero. A zero that is in the series MUST be
  stored.
- **FR-007**: Re-collecting MUST replace only the days present in the new
  response. A failed resource MUST leave its stored points unchanged.
- **FR-008**: Two currencies for one resource MUST remain two series and MUST
  NOT be summed.
- **FR-009**: The projected timeline file and its schema version MUST stay
  unchanged. Opening a daily file as a projected timeline, or a projected
  timeline as a daily file, MUST fail.
- **FR-010**: The daily file MUST use the existing history-directory
  resolution and jail, including rejection of a symlinked daily file.
- **FR-011**: When `cost.history.retention.auto_prune` is on and
  `max_age_days` is greater than 0, a successful collect MUST delete daily
  points older than that age. `max_snapshots` MUST NOT delete daily points.
- **FR-012**: The command MUST be excluded from the MCP tool list, together
  with `cost history collect`, and MUST stay out of the pinned tool list.
- **FR-013**: The summary MUST report resources stored, resources skipped
  because the daily series was absent, and per-resource errors. Resource
  errors MUST exit non-zero after successes are stored. Validation errors
  MUST exit 2.
- **FR-014**: The command MUST NOT record resource-observation history, MUST
  NOT import Pulumi Cloud history, and MUST NOT start collection from the
  web dashboard.

### Key Entities

- **Daily point**: One resource, one UTC day, one currency, one amount, and
  the time the point was written.
- **Daily series file**: The per-stack file that holds daily points. It is
  not the projected checkpoint timeline.
- **Collect summary**: Counts of stored resources, stored points, skipped
  resources, pruned points, and the resource errors.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable. This command has no dashboard surface,
  matching `finfocus cost history collect`. No web handler starts a collect.
- **Web data**: None. The dashboard does not read or write this series in
  this feature.
- **Layout fit**: None. No existing view gains a control that starts
  collection.
- **Additions**: None.
- **Render evidence**: Not applicable. There is no page to render. The
  nearest related view is Cost History, which reads the projected timeline
  and is unchanged.
- **Follow-up issue scope**: No `web-ui` issue. A browser button that starts
  collection is out of scope, and there is no UX-only remainder.

## Assumptions

- `finfocus cost actual` fills `DailyCosts` by splitting the window total
  evenly. That series is not the plugin's days. This command reads the
  timestamp on each plugin bucket instead.
- A blank currency is USD. Named currencies are stored as the plugin sent
  them, after trimming space.
- The stored resource id is the id on the actual-cost result, which for
  Pulumi is the resource URN.
- The command prices the resources loaded from the project or the input
  file. It does not add destroyed resources from Pulumi Cloud history.
- Usage profile stays unspecified, as it does for `finfocus cost actual`.

## Out of Scope

- Detecting anomalies, printing baselines, or changing `finfocus cost
  anomalies` (#1590).
- The robust-median method (#1863), which reads this store later.
- A new plugin RPC, or any change that needs a finfocus-spec bump.
- Bumping the projected timeline schema or migrating its files.
- Scoring, forecasting, and a browser button that starts collection.
- Configurable anomaly thresholds.
- Applying `max_snapshots` to daily points.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For a fixture with a daily series, every returned day is stored
  and every omitted day is absent (100% of fixture days).
- **SC-002**: Re-running an overlapping window replaces only the days in the
  new response (0 days outside that response change).
- **SC-003**: A missing series produces 0 stored points for that resource and
  does not create a zero point for any day in the window.
- **SC-004**: A projected timeline used by view and diff is byte-for-byte
  unchanged in its snapshots after a daily collect.
- **SC-005**: The command is absent from the MCP tool list, and an invalid
  `--output` exits 2 before a stack read.
- **SC-006**: An operator can read the post-deploy step in the user guide and
  the CI guide and run it beside the existing projected collect.
