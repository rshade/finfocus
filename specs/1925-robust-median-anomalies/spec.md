# Feature Specification: Robust-Median Method for Cost Anomalies

**Feature Branch**: `1925-robust-median-anomalies`
**Created**: 2026-11-02
**Status**: Draft
**Input**: User description: "When a resource has enough daily actuals in one
currency, compare the latest complete day with a rolling median of the days
before it (GitHub issue #1863). When it does not, keep the drift row #1590
already prints."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Judge a day against its own days (Priority: P1)

A person or an MCP client reading `cost anomalies` sees a one-day billing
spike reported against that resource's own recent days, so a deploy-sized
change in the projection and a billing spike are visibly different kinds of
row.

**Why this priority**: This is the whole feature. Without it `cost anomalies`
only compares an extrapolated month-to-date actual with the projected month
and cannot see a single unusual day.

**Independent Test**: A fixture daily store with fourteen or more earlier
points in one currency and a spiked latest day emits one row with
`method: "robust-median"`, the median as `baseline`, the latest complete day
as `observed`, and `severity: "warning"`.

**Acceptance Scenarios**:

1. **Given** a resource with 14 earlier daily points in one currency and a
   robust z at or beyond 3.5, **When** the command runs, **Then** the
   resource's row has `method: robust-median`, the median as `baseline`, the
   latest complete day as `observed`, `deviation` of `observed - baseline`,
   `percentDrift` of the percentage of the baseline, the observation day as
   `period`, the baseline window, `severity: warning`, and the series
   currency.
2. **Given** a resource with fewer than 14 earlier points, **When** the
   command runs, **Then** its drift row stands with `method: drift` when
   drift fired, and it gets no row when drift did not fire either.
3. **Given** a resource whose baseline MAD is 0 (a flat series), **When** the
   command runs, **Then** no statistical row replaces the drift row. The
   reader invents no zeros and no second threshold catches this hole; the
   command help and this spec document it.
4. **Given** a resource whose baseline median is 0 with a nonzero MAD,
   **When** the command runs, **Then** `percentDrift` is omitted from the
   JSON and `deviation` is still reported; there is no division by zero.
5. **Given** a resource stored in two currencies, **When** the command runs,
   **Then** the currencies are two series: one median each, one qualifying
   test each, one row with the larger absolute deviation. They are never
   combined into one median or one total.
6. **Given** a missing day between stored days, **When** the command runs,
   **Then** the gap is skipped: it is never stored, never treated as zero,
   never interpolated, and does not count toward the 14.
7. **Given** two rows for one resource, or a statistical row that replaces a
   drift row, **When** the command runs, **Then** the resource appears at
   most once.
8. **Given** an empty store, or a stack that has never been collected,
   **When** the command runs, **Then** the output is exactly the #1590 drift
   list.
9. **Given** a stack whose stored daily actuals reach back past the requested
   window, **When** the command runs without `--from` or `--to`, **Then** the
   whole stored history is scored, so the method does not depend on the day of
   the month. With an explicit window, the days outside it are not part of the
   baseline.

### Edge Cases

- The observation is the latest **complete** UTC day of the series; the
  current partial day is never an observation.
- The stored daily actuals are a rolling history of the stack. An explicit
  `--from` or `--to` bounds the series; without them the whole stored history
  is scored, so the method does not depend on the day of the month.
- A day at exactly the 3.5 cutoff is emitted; just under it is not. The
  cutoff and the 0.6745 scale factor live as constants next to the scoring
  function, the way `driftWarningThreshold` does. Neither is a `config.hujson`
  key.
- Severity stays `warning` on every emitted row.
- A resource a preview marks for deletion, or one whose cost lookup failed,
  is never an anomaly, statistical or drift.
- A series read failure is a command failure, not a silent fallback: the
  stored daily costs are half the evidence the command reports.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-1** `cost anomalies` (command, flags, MCP tool name
  `finfocus-cost-anomalies`, and JSON fields) stays the one #1590 shipped.
  The only output change is that some rows gain `method: robust-median` and a
  `baselineWindow` field, and some enter with the two-method selection below.
- **FR-2** After the drift list is built, a resource's drift row is replaced
  by a statistical row when the stored series qualifies. Selection rule per
  resource URN: statistical row qualifies -> emit that row and drop the drift
  row; statistical row does not qualify and a drift row exists -> emit the
  drift row; neither qualifies -> emit nothing.
- **FR-3** Observation: latest complete UTC day of the series; the current
  partial day is never an observation. Baseline: the median of the earlier
  points for one resource and one currency; the observation day is excluded
  from the median so it cannot pull the median toward itself.
- **FR-3a** The series follows the dates the caller asks for: an explicit
  `--from` or `--to` bounds the days the reader scores, and without them the
  whole stored history of the stack is used. The implicit default cost window
  (the current month, which the drift comparison needs) never truncates the
  stored history, or the method would be silent in the first half of every
  month. The daily store is already a rolling history of its stack.
- **FR-4** Minimum history: 14 qualifying earlier points in one currency.
  Gaps do not count and are not filled.
- **FR-5** Score: robust z = `0.6745 * (observed - median) / MAD`, MAD being
  the median of `|point - median|` over those same baseline points. Emit
  `method: robust-median` when the absolute score is at least 3.5. MAD of 0
  emits nothing.
- **FR-6** Currency is the series currency; currencies are never pooled.
- **FR-7** The shared selection function (#1590) takes the daily points as a
  new input; the CLI loads them from the store (`internal/cli`), and the web
  handler calls the same function with the session's stored points. Nothing
  recomputes in JavaScript and `internal/webui` still does not import
  `internal/cli`.
- **FR-8** The 3.5 cutoff, the 14-point minimum, and the 0.6745 factor are
  named constants in `internal/engine`, not config keys.

### Non-Functional Requirements

- **NFR-1** Documentation: the command page and the MCP guide document the
  two method values, the 14-day minimum, the 3.5 cutoff, and the MAD-of-zero
  hole.
- **NFR-2** No scorer plugin is called; `CalculateCostDrift` is unchanged;
  the drift threshold, the overview pipeline, and `cost history diff` are
  unchanged; this issue only reads the #1862 store and never collects.

## Web UI Fit *(mandatory)*

- **Disposition**: Extends the existing Anomalies hook
  (`GET /api/overview/anomalies`) with the statistical method; the page is
  left to the UX follow-up issue filed at the end of #1590. No new page.
- **Web data**: The same `AnomalyReport` the command renders, produced by
  `engine.SelectAnomalies` over the session's overview rows and the session
  stack's stored daily points, loaded by the same reader the CLI uses. The
  page builds no statistics itself.
- **Layout fit**: The existing Anomalies table keeps its shape; a
  robust-median row is one row more or one row different, not a new section.
- **Web hook**: `internal/webui/handlers_anomalies.go` already calls the
  shared function; it gains the daily series it was already structured for.

## Success Criteria

- Tests cover the sufficiency boundary at 14, gap handling, MAD-zero,
  median-zero, the 3.5 threshold edges, two currencies, drift-only fallback,
  statistical replacement, and the single-row rule.
- A CLI test with a fixture store shows `method` in the JSON output; a web
  handler test shows the same rows as the CLI for the same inputs.
- `go test ./...` green; `make validate`, `make lint`, `make test` pass; the
  MCP tool list unchanged.
