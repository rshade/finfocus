# Feature Specification: Cost Forecast Web Hook

**Feature Branch**: `1737-forecast-web-hook`
**Created**: 2026-10-28
**Status**: Implemented
**Input**: Issue #1859: expose `finfocus cost forecast` (spec 626) through a
web API hook so the dashboard can show the month-by-month projection. The
view itself is a UX-only follow-up.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Query the forecast for the session stack (Priority: P1)

A team lead runs `finfocus --web` in a Pulumi project. The dashboard can ask
the session for the monthly cost projection of the loaded stack, with the
same months and growth settings the CLI accepts, and receives the same
projection the CLI would print as JSON.

**Why this priority**: Without it no forecast reaches the dashboard, and
Principle VII blocks every later forecasting feature (#539) until this hook
exists.

**Independent Test**: Start a web session against a fixture stack, request a
forecast, and compare the series values with `cost forecast --output json`
for the same stack, months, and growth settings.

**Acceptance Scenarios**:

1. **Given** a loaded session and no growth settings, **When** the dashboard
   requests a 12-month forecast, **Then** the response holds 13 forecast
   points per series with the same values, currency, budget, and warnings as
   the CLI JSON.
2. **Given** a growth type of linear and a rate of 0.10, **When** the
   dashboard requests a forecast, **Then** each value equals the CLI's value
   for `--growth-type linear --growth-rate 0.10`.
3. **Given** a session whose load has not finished, **When** a forecast is
   requested, **Then** the dashboard is told the data is still loading and
   no plugin is opened a second time.

---

### User Story 2 - Clear errors for invalid settings (Priority: P2)

A user who enters an out-of-range month count or a missing growth rate sees
the same explanation the CLI gives.

**Why this priority**: The view's toolbar depends on error text it can show
without rewording, but a valid query is useful on its own first.

**Independent Test**: Send each invalid combination and compare the returned
message with the CLI's error for the equivalent flags.

**Acceptance Scenarios**:

1. **Given** months of 0 or 37, **When** the forecast is requested, **Then**
   the request is rejected as invalid input with the CLI's months message.
2. **Given** growth type exponential and no rate, **When** the forecast is
   requested, **Then** it is rejected with the CLI's missing-rate message.
3. **Given** a rate below -1 or an unknown growth type, **When** the forecast
   is requested, **Then** it is rejected with the CLI's message for that
   case.

---

### User Story 3 - History beside the projection (Priority: P3)

When the stack has stored cost history, the response includes the history
series the CLI shows, so a later view can draw both on one time axis. A user
can ask for the projection without history.

**Why this priority**: History is an overlay; the projection is useful
without it.

**Independent Test**: With and without a cost-history database for the
session stack, request a forecast with and without the no-history option and
compare the series list with the CLI.

**Acceptance Scenarios**:

1. **Given** cost history in the forecast currency, **When** the forecast is
   requested, **Then** a `history` series precedes the forecast series.
2. **Given** no history database, **When** the forecast is requested,
   **Then** it succeeds without a `history` series.
3. **Given** the no-history option, **When** the forecast is requested,
   **Then** no history is read and no `history` series is returned.

### Edge Cases

- A stack where no plan resource was priced, or whose priced resources use
  two currencies, fails with the CLI's message as a distinct "nothing to
  project" error, not as unavailable data. Resources priced at zero still
  project to zero.
- A plugin failure while pricing is reported as unavailable data with a
  generic message; plugin error text does not reach the page.
- A history database in another currency is omitted with the CLI's warning.
- A budget in another currency is omitted with the CLI's warning.
- A state-only session forecasts only plan-listed resources, the same
  membership the CLI uses (#1903), so an empty plan yields an empty forecast.
- A plugin reload between queries uses the replacement engine, as the other
  query endpoints do.
- Two concurrent forecast requests are serialized through the session's
  query gate.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The session MUST answer a forecast query taking months, growth
  type, growth rate, and a no-history option, with the CLI defaults when a
  value is omitted (12 months, plugin growth, history included).
- **FR-002**: The CLI command and the web query MUST build the projection
  through one shared function. `cost forecast` output (plain, JSON, NDJSON)
  MUST NOT change.
- **FR-003**: The response MUST carry the same currency, months, budget,
  series (history, forecast, then per-provider), and warnings as the CLI JSON
  document for the same inputs.
- **FR-004**: The response MUST carry display strings for each point value,
  each point's month, and the budget, computed on the server. The browser
  MUST NOT compute forecast values or format money.
- **FR-005**: Invalid months, an unknown growth type, an unparseable or
  out-of-range rate, and a missing rate for linear or exponential growth MUST
  be rejected as invalid input with the CLI's message text.
- **FR-006**: The web query MUST price the session's already-loaded
  resources with the session's query engine, including its usage profile and
  launch filters. It MUST NOT open plugins or run a preview itself.
- **FR-007**: The history series MUST come from the same cost-history
  database the CLI reads for the session stack. A missing database MUST NOT
  fail the query.
- **FR-008**: Pricing failures MUST return a generic unavailable error, never
  plugin or subprocess text. A forecast with nothing to project (no priced
  resource, or mixed currencies) MUST be reported separately with the CLI's
  message.
- **FR-009**: Data-keyed maps in the response, if any, MUST keep their keys
  through the dashboard's payload redaction.
- **FR-010**: The new endpoint MUST follow the dashboard's existing security
  rules for state-reading POST requests (same origin, JSON body, session
  cookie).

### Key Entities

- **Forecast query**: months, growth type, growth rate (text, as the CLI
  flag), no-history option.
- **Forecast document**: currency, months, optional budget, ordered series,
  warnings; identical to the CLI JSON.
- **Display**: per-series, per-point month label and money string, plus the
  budget string.

## Web UI Fit *(mandatory)*

- **Disposition**: New view, "Forecast", reached from the header view links.
  It has its own controls (months, growth type, rate) and its own data, so it
  is a view built from the shared slots rather than an Overview extension.
  This issue ships only the hook; the view is the UX follow-up.
- **Web data**: the forecast series (and history and provider series when
  present), currency, budget, warnings, and the display strings for each
  value, exactly as the CLI JSON reports them.
- **Layout fit**: toolbar with months, growth type, rate, a history toggle,
  and a Run button; status line for loading and validation errors; totals
  bar with the current month, the final month, and the budget; captioned
  month table (month, forecast, then one column per provider) with numeric
  columns aligned like the Overview table; warnings panel below the table.
- **Additions**: an optional line chart of the series over time, drawn from
  the returned points without arithmetic.
- **Render evidence**: 2026-10-28, Overview view and budget panel rendered
  with Playwright against the `testdata/overview` mixed-changes plan and
  state; the endpoint matched the CLI JSON on the same files (details in
  `quickstart.md`).
- **Follow-up issue scope**: UX only (layout, rendering, interaction, copy,
  accessibility). No engine, API, redaction, or data-shape work.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For every months and growth combination in the parity test, the
  dashboard response and the CLI JSON report identical values, currency,
  budget, series names, and warnings.
- **SC-002**: 100% of the invalid-input cases return the CLI's message text.
- **SC-003**: Existing `cost forecast` tests and output fixtures pass
  unchanged.
- **SC-004**: A forecast query on a loaded session opens no new plugin
  process.
- **SC-005**: Removing the cost-history database does not make the query
  fail.

## Assumptions

- The session's launch flags (stack, filters, adapter, profile) define the
  forecast's inputs, as they do for every other view; the query cannot
  change them.
- Plain chart options (height, width, color, split providers) are CLI
  presentation only and have no web parameter. The response always carries
  per-provider series, as the CLI JSON does.
- The budget line is shown when a global budget exists; the web has no
  equivalent of `--no-budget` because the view can hide it.
- Terraform sessions forecast the planned Terraform resources the session
  already holds, matching the CLI's Terraform directory mode.

## Dependencies

- `specs/626-cost-forecast/` (the projection and the CLI JSON shape).
- #539 extends this hook's payload with a statistical model later.
- #1903 (plan versus whole-stack membership) is unresolved; this hook keeps
  the CLI's current plan membership.

## Out of Scope

- ARIMA or driver-based forecasting (#539).
- Rendering the forecast in the page (UX follow-up issue).
- Terraform state-file input (`--terraform-state`), which the web does not
  accept.
- Changing the forecast's resource membership (#1903).
