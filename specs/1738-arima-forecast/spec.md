# Feature Specification: Statistical Cost Forecast (ARIMA and Drivers)

**Feature Branch**: `1738-arima-forecast`
**Created**: 2026-10-28
**Status**: Implemented
**Input**: Issue #539: time-series forecasting for cost projection. A
statistical trend (seasonal ARIMA) fitted to past spend, with deterministic
business drivers (growth cap, step changes, ignored ranges) on top.

The issue placed the flags on `cost estimate`. They go on `cost forecast`
instead, which already owns the timestamped monthly series, the chart, the
budget line, and the web hook (#1859); `cost estimate` stays a
point-in-time what-if tool (decision recorded with the maintainer,
2026-10-28).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Forecast from past spend (Priority: P1)

A FinOps lead exports six months of daily spend from their billing console
as a `date,value` CSV and runs `cost forecast --model arima --spend-history
spend.csv --months 12`. They get twelve future months of projected spend,
each with a lower and upper bound, alongside the plan-based growth
projection as a baseline.

**Why this priority**: The statistical trend is the feature; the drivers
modify it.

**Independent Test**: Run the command on a fixture CSV with a known trend and
weekly pattern and check that the monthly totals follow the trend, that each
total lies within its bounds, and that the baseline equals the
`--model growth` forecast for the same plan.

**Acceptance Scenarios**:

1. **Given** 180 consecutive days of spend, **When** forecasting 12 months,
   **Then** the output holds 13 monthly totals (the current month through
   month 12), a lower and upper bound per month, and the model's name.
2. **Given** daily spend with a weekday/weekend pattern, **When**
   forecasting, **Then** the chosen model is seasonal with a weekly period.
3. **Given** a monthly CSV of at least ten months, **When** forecasting,
   **Then** the model is fitted to monthly totals directly.
4. **Given** the current month is partly covered by the CSV, **When**
   forecasting, **Then** that month's total is the recorded days plus the
   forecast for the remaining days.

---

### User Story 2 - Apply business drivers (Priority: P2)

The lead knows things the history cannot show: a December spike that will
not repeat, a contract that adds $500 a month from June, and a budget
ceiling of 25% above today's spend. They pass `--exclude
2025-12-01:2025-12-31`, `--step 2027-06-01=500`, and `--growth-cap 1.25`.

**Why this priority**: Drivers make the trend usable for planning, but the
trend has value alone.

**Independent Test**: Apply each driver to a fixed model output and check
the exact resulting values.

**Acceptance Scenarios**:

1. **Given** an excluded range, **When** forecasting, **Then** spend inside
   it is replaced by a straight line between its neighbors before fitting,
   and the output lists the excluded range.
2. **Given** a step, **When** forecasting, **Then** every month from the
   step's month onward rises by the amount, bounds included.
3. **Given** a growth cap of 1.25, **When** forecasting, **Then** no month,
   and no bound, exceeds 1.25 times the current month's value, and a capped
   month is reported.
4. **Given** a model result below zero, **When** forecasting, **Then** the
   value and bounds are shown as zero.

---

### User Story 3 - The same forecast in the dashboard (Priority: P3)

A user in `finfocus --web` selects the statistical model, chooses a CSV
file, and sets the same drivers; the dashboard receives the same values as
the CLI.

**Why this priority**: Required by Principle VII; it reuses the hook from
issue #1859.

**Independent Test**: Send the CSV contents and drivers to the forecast
endpoint and compare with the CLI JSON for the same file and flags.

**Acceptance Scenarios**:

1. **Given** the CSV text and drivers, **When** the dashboard queries the
   forecast, **Then** the document matches the CLI JSON for the same inputs.
2. **Given** an invalid CSV or driver, **When** the dashboard queries,
   **Then** it gets the CLI's message as an input error.

### Edge Cases

- Fewer than ten usable points after exclusion: rejected with a message
  naming the count and the minimum.
- A constant spend history: no model fits; the forecast is flat at that
  value with a warning.
- Gaps, duplicate dates, mixed cadence, non-numeric or negative values, or a
  missing column: rejected with the line number.
- An excluded range that covers the first or last point: those points are
  dropped instead of interpolated.
- A step before the current month: applies from the current month.
- `--spend-history` without `--model arima`, or `--model arima` without it:
  rejected.
- Drivers with the growth model: steps and the cap apply to the growth
  forecast too; `--exclude` and `--confidence` need `--model arima`.
- The plan cannot be priced: the statistical forecast still runs; the
  baseline is omitted with a warning.
- A long horizon on daily data widens the bounds; bounds are clamped at
  zero and never become negative.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `cost forecast` MUST accept `--model growth|arima`, default
  `growth`. With `growth`, output MUST be unchanged from today.
- **FR-002**: `--model arima` MUST require `--spend-history <file>`, a CSV of
  `date,value` rows (optional header, dates as `2006-01-02` or RFC3339),
  one row per consecutive day or per consecutive calendar month.
- **FR-003**: The statistical model MUST be selected automatically
  (non-seasonal or seasonal, including weekly for daily data) and MUST
  report its order.
- **FR-004**: Daily spend MUST be modelled per day and summed into the
  command's calendar-month points; monthly spend MUST be modelled per month.
- **FR-005**: Each forecast month MUST have a lower and upper bound at
  `--confidence` (default 0.95, between 0 and 1 exclusive). Monthly bounds
  from daily data MUST be the sum of daily bounds (conservative).
- **FR-006**: `--exclude FROM:TO` (repeatable) MUST replace spend in the
  range by linear interpolation before fitting.
- **FR-007**: `--step DATE=AMOUNT` (repeatable, amount may be negative) MUST
  add the amount to every month from DATE's month onward.
- **FR-008**: `--growth-cap FACTOR` (> 0) MUST limit every month and bound to
  FACTOR times the current month's value, applied after steps.
- **FR-009**: Values and bounds MUST be floored at zero after all drivers.
- **FR-010**: With `--model arima`, the plan's growth projection MUST be
  emitted as a `baseline` series; the statistical result is the `forecast`
  series, with `lower`, `upper`, and the input as a `spend` series.
- **FR-011**: JSON MUST add `model`, `modelOrder`, and `confidence` only in
  arima mode; NDJSON and the chart MUST include the new series.
- **FR-012**: Every input error MUST be reported before any plugin is
  started.
- **FR-013**: The web forecast query MUST accept the same model, CSV text,
  confidence, cap, steps, and exclusions, and MUST produce the CLI's values
  through the same Go functions.
- **FR-015**: The dashboard MUST offer a Forecast view that sends every
  model and driver input to the forecast endpoint, reads a chosen CSV file in
  the browser, renders only the server's display strings, shows the server's
  error text, and never computes a cost value.
- **FR-014**: The statistical library is the maintained fork
  `github.com/rshade/goarima`, pinned to a commit (the fork is not tagged).

### Key Entities

- **Spend history**: dated spend amounts at one cadence (daily or monthly).
- **Drivers**: excluded ranges, steps (month, amount), growth cap, floor.
- **Statistical forecast**: model order, confidence, monthly totals with
  bounds.

## Web UI Fit *(mandatory)*

- **Disposition**: New view, "Forecast" (`#/forecast`), built in this
  feature for both the growth model (#1859) and the statistical model. No
  UX follow-up issue is filed (maintainer decision, 2026-10-29).
- **Web data**: the same document as the CLI JSON: forecast, bounds,
  baseline, spend, history, provider series, model order, confidence,
  budget, warnings, and the server's display strings.
- **Layout fit**: a toolbar form with months, model, growth type, growth
  rate, growth cap, steps, an include-history toggle, and (for the
  statistical model) a CSV file picker read in the browser and sent as text,
  confidence, and excluded ranges; a status line and error banner; a totals
  bar with the model, the current and final month, and the budget; a
  captioned month table with one numeric column per month-aligned series
  (forecast, lower, upper, baseline, providers), aligned like the Overview
  table; a summary of the history and spend inputs; a warnings list.
- **Additions**: none. An interactive chart stays with #550.
- **Render evidence**: the Forecast view in headless Chromium at 1280px and
  390px, light and dark, by `make test-e2e-web` (`TestWebUIForecast`), plus
  the endpoint checks in `quickstart.md`.
- **Follow-up issue scope**: none; the view ships here.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a fixture of 180 days with a linear trend and weekly
  pattern, every projected monthly total is within 5% of the true
  generating total for all twelve months ahead.
- **SC-002**: Every driver acceptance scenario produces the exact expected
  values in tests.
- **SC-003**: `--model growth` output is identical to the previous release
  for every existing forecast test.
- **SC-004**: The web document equals the CLI JSON for the same inputs in a
  parity test.
- **SC-005**: Invalid CSV rows are reported with their line number in 100%
  of the malformed-input cases tested.

## Assumptions

- `--spend-history` is used instead of the issue's `--history`, because
  `cost forecast` already has `--no-history` for the cost-history database
  overlay, which stays as it is.
- Spend values are in the plan's currency (USD when the plan has none).
- `--growth-cap` is relative to the current month's forecast value, which is
  "today's spend" for the user. The issue's example `1.25` means +25%.
- The issue's "TUI line chart" is the existing ASCII forecast chart, extended
  with the new series; an interactive chart remains #550.
- The issue's "weekdays" seasonality applies to daily input; monthly input
  needs about two years for a yearly pattern, which the model detects only
  when present.
- `--split-providers` has no meaning for spend history (the CSV has no
  provider) and is ignored with a warning in arima mode.

## Dependencies

- `specs/1737-forecast-web-hook/` (the web hook this extends).
- `github.com/rshade/goarima` at commit `6420b78`
  (`v0.4.1-0.20261029015414-6420b7848c38`): the fork created 2026-10-28,
  which returns errors instead of nil results and drops the constant after
  two or more orders of differencing.

## Out of Scope

- Reading spend from plugins (`GetActualCost`) or the cost-history database
  as model input.
- Changing `cost estimate`.
- An interactive chart (#550).
