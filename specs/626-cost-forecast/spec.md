# Feature Specification: Cost Forecast

**Feature Branch**: `626-cost-forecast`
**Created**: 2026-10-05
**Status**: Implemented
**Input**: GitHub issue #364. Project future spend from the current plugin price
and GrowthType/GrowthRate. Render an ASCII line chart. The series model must
be something the future interactive history chart (#550) can plot without a
second projection engine.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Project a stack forward (Priority: P1)

A person runs `finfocus cost forecast` against the same plan or state they
already use for `cost projected`. FinFocus prices each resource, reads the
plugin growth model, and prints the monthly total from the current month
through N months ahead.

**Why this priority**: The projection is the feature #364 asks for. The chart
is useless without these numbers.

**Independent Test**: Feed priced resources with a known growth model and rate
and compare the series to the finfocus-spec formulas.

**Acceptance Scenarios**:

1. **Given** an EC2 priced at $100 with growth `none` and an S3 priced at $100
   with growth `linear` and a rate of 0.10, **When** forecasting 1 month ahead,
   **Then** month 0 is $200 and month 1 is $210.
2. **Given** a resource priced at $100 with growth `exponential` and a rate of
   0.10, **When** forecasting 2 months ahead, **Then** the points are $100,
   $110, and $121.
3. **Given** resources priced in USD and EUR, **When** forecasting, **Then**
   the command fails and names the mixed currencies.
4. **Given** a plugin error or a validation failure on one resource, **When**
   another resource has a price, **Then** the failed resource is omitted and
   named, and the priced resource is still projected.

### User Story 2 - See the forecast as a chart and as data (Priority: P2)

The same forecast renders as an ASCII line chart, as one JSON document, and
as NDJSON. A configured global budget draws as a horizontal line on the chart
and as a number in JSON.

**Why this priority**: #364 asks for the chart. Scripts and the future
interactive view need the same points in a stable document.

**Independent Test**: Render a two-point forecast and compare the full chart
text. Decode the JSON and NDJSON and compare the points.

**Acceptance Scenarios**:

1. **Given** a forecast of at least two months, **When** the output is plain,
   **Then** the chart names the currency and the month range and includes the
   forecast series.
2. **Given** `--split-providers`, **When** two providers have prices, **Then**
   the chart and the JSON each include one series per provider.
3. **Given** a global budget amount and no `--no-budget`, **When** the chart
   renders, **Then** a Budget series is drawn and JSON reports that amount.
4. **Given** `--output yaml`, **When** the command runs, **Then** it rejects
   the format before it prices anything.

### User Story 3 - Keep history beside the forecast (Priority: P3)

When the selected stack already has a cost-history database, the JSON series
list includes those snapshots as a `history` series. The command still
succeeds when the database is missing. A different history currency is left
out and reported.

**Why this priority**: Issue #550 will draw history and the forecast on one
time axis. The points have to exist before that chart does. This story does
not add pan, zoom, or a second TUI.

**Independent Test**: Pass stored snapshots in the forecast currency and in a
different currency and inspect the series list.

**Acceptance Scenarios**:

1. **Given** history snapshots in the forecast currency, **When** the output
   is JSON, **Then** a `history` series carries each snapshot time and total.
2. **Given** no history file, **When** forecasting, **Then** the command
   succeeds and omits the history series.
3. **Given** history in another currency, **When** forecasting, **Then** the
   history series is omitted and a warning names both currencies.

### Edge Cases

- Months ahead must be 1 through 36. Zero, negative, and 37 fail before pricing.
- A growth rate below -1 fails before pricing. A rate of 0 is a real rate.
- Linear growth that would go below zero is clamped to zero and warned once
  per resource. A non-finite result is clamped the same way.
- A rate above 100% per period is projected and keeps the SDK warning.
- `--growth-type linear` or `exponential` without `--growth-rate` fails.
  A plugin that reports linear or exponential without a rate keeps that
  resource flat and records a warning.
- `--growth-type none` ignores a rate.
- An empty plugin growth type is no growth. `cost projected` monthly totals
  do not change.
- A blank currency is USD, matching the engine default. Two explicit
  currencies still fail.
- `--plain` forces the chart. `--no-color` and a non-empty `NO_COLOR` drop
  chart colors.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `finfocus cost forecast` MUST price the same inputs as
  `cost projected` and project the after-change monthly cost.
- **FR-002**: Each resource MUST be projected with its own growth model, then
  summed. The formulas MUST be the finfocus-spec linear and exponential
  helpers, not a second implementation.
- **FR-003**: Growth type MUST come from the plugin projected-cost response.
  A CLI `--growth-type` overrides it. `--growth-rate` is the rate for every
  resource. The response proto has no rate field.
- **FR-004**: The forecast series MUST be monthly timestamps in UTC, starting
  on the first day of the current month, with one point per month from the
  current month through N months ahead.
- **FR-005**: JSON and NDJSON MUST expose those timestamped points, per-provider
  series, the budget amount when one is drawn, and warnings. The series MUST
  NOT use chart-library types.
- **FR-006**: Plain output MUST be an ASCII line chart of the forecast. Provider
  series and the budget line MUST be optional and MUST NOT change the points.
- **FR-007**: A readable cost-history database for `--stack` MUST add a
  `history` series when its currency matches. A missing database MUST NOT fail
  the command.
- **FR-008**: The command MUST NOT write a new database and MUST NOT require
  history, a TTY, or a previous forecast.
- **FR-009**: Plugin and validation failures MUST be excluded from the sum and
  listed. A forecast with no priced resource MUST fail.
- **FR-010**: An unknown `--output` MUST fail before plugins are opened.

### Key Entities

- **Forecast resource**: One priced resource, its currency, provider, growth
  model, and optional rate.
- **Point**: A UTC month start and a money amount.
- **Series**: A name (`forecast`, `history`, or a provider) and its points.
  Budget is a single amount, not a series, so a later chart can draw it as a
  horizontal line.
- **Projection**: Currency, months ahead, the forecast series, provider series,
  and warnings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The linear and exponential examples in the scenarios match to
  the cent.
- **SC-002**: A person can read the month range and the forecast total from
  the plain chart without a TTY.
- **SC-003**: A JSON consumer can plot `history` and `forecast` by `time`
  without reading the chart text.
- **SC-004**: Deleting the history database does not make `cost forecast` fail.
- **SC-005**: `cost projected` still reports the current monthly price. The
  forecast does not rewrite it.

## Assumptions

- finfocus-spec v0.7.5 already defines `GrowthType` and the pricing helpers.
  This feature does not change the protocol. The plugin response carries
  `growth_type` only. The rate is the CLI flag because the response has no
  rate field.
- "Current cost" is the after-change projected monthly cost. Deletes
  contribute zero and do not move the total.
- Issue #550 (interactive ntcharts history view) consumes this series later.
  It is not implemented here. Bubble Tea is already v2, so #550's ntcharts v1
  note is stale. The seam is the timestamped series, not a widget.
- Issue #539 (ARIMA on `cost estimate`) stays a separate command. It can emit
  the same point type later. This feature does not add a statistical model.
- The ASCII chart is index-based. #550 must use the timestamps, not the
  chart rows, when it overlays history and the forecast.
- 36 months is the maximum. That is the SDK's long-exponential warning
  boundary, so the command refuses a longer range instead of warning.

## Out of Scope

- Pan, zoom, deployment detail, and an interactive Bubble Tea model (#550).
- Adding ntcharts or any new module.
- ARIMA, CSV history import, and growth caps (#539).
- Applying growth inside `cost projected` or changing its monthly totals.
- A new BoltDB store.
