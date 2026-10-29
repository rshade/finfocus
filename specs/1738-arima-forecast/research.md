# Research: Statistical Cost Forecast

## R1. Command placement

- **Decision**: `cost forecast --model arima`, not `cost estimate`.
- **Rationale**: `cost forecast` already has the monthly timestamped series,
  chart, budget, history overlay, and web hook (#1859). Spec 626 lists ARIMA,
  CSV import, and growth caps as left for #539. Confirmed with the
  maintainer on 2026-10-28.

## R2. Library and fork

- **Decision**: `github.com/rshade/goarima`, pinned to commit `6420b78`.
  The fork is not tagged further (maintainer decision, 2026-10-28); fixes land
  on its `main` and finfocus pins the commit's pseudo-version.
- **Rationale**: Probing `sartorproj/goarima` showed `AutoARIMA` returns
  `(nil, nil)` for fewer than 10 points or when no candidate fits, and
  `PredictWithInterval` returns all-nil for an empty result. Upstream has
  merged nothing since 2026-04-14 and its fixes after v0.3.0 are untagged.
  The fork renames the module, returns `ErrSeriesTooShort`, `ErrNoModel`, and
  `ErrNilResult` (tagged v0.4.0 before the no-tag decision). It has no dependencies and passes
  `go vet` under Go 1.27.3.
- **Alternatives considered**: Pinning an upstream pseudo-version (keeps the
  nil contract); a hand-written Holt-Winters model (drops the issue's SARIMA
  requirement).

## R2a. Constant after differencing (fork fix)

- **Decision**: The fork fixes the intercept at zero when `d + D >= 2`.
- **Rationale**: goarima used the mean of the differenced series as a
  constant for every order. After two orders of differencing it integrates
  into a quadratic drift, so on the 180-day fixture the 12-month forecast was
  63% low. With the fix every month of the year is within 3.2%. R's `Arima`
  makes the same choice.

## R3. Model input

- **Decision**: A spend CSV (`--spend-history`), daily or monthly.
- **Rationale**: The cost-history database stores the projected run-rate at
  each deployment, a step function. ARIMA reverts a step toward the mean
  (probe: 100→150 step forecast decayed 147→130) and finds no model on flat
  runs. The weekday seasonality in the acceptance criteria exists only in
  billed daily spend. The database stays the `history` overlay.

## R4. Daily to monthly

- **Decision**: Fit daily, forecast through the last day of the horizon's
  final month, sum per calendar month; recorded days contribute their actual
  value. Monthly bounds are the sum of daily bounds.
- **Rationale**: The command's grid is calendar months. Summing daily bounds
  assumes perfectly correlated errors, so it can only overstate the width;
  it is documented as conservative.

## R5. Seasonal periods

- **Decision**: Daily candidates `autoarima.DailyPeriods` (7, 14, 30, 365);
  monthly `[12]`. Other settings stay at `DefaultConfig`.
- **Rationale**: Limits the search to periods that make sense for each
  cadence. The probe found `SARIMA(4,1,2)(0,1,1)[7]` on a weekday fixture.

## R6. Short or constant input

- **Decision**: Fewer than `autoarima.MinObservations` usable points is an
  input error. `ErrNoModel` on a zero-variance series is a flat forecast
  with a warning; otherwise an input error.
- **Rationale**: A constant spend has an obvious forecast, and refusing it
  would surprise users; a non-constant series no model fits is the user's
  data to fix.

## R7. Driver order

- **Decision**: Model → steps → cap → floor at zero, applied to the forecast
  and both bounds. The cap base is month 0 after steps. With the growth
  model, steps and the cap adjust the total only; provider lines remain the
  undriven projection.
- **Rationale**: A contract step is known spend; a budget ceiling limits the
  total including it. Zero last so no driver produces a negative cost.

## R8. Excluded ranges

- **Decision**: Points in a range are replaced by linear interpolation
  between the nearest kept neighbors; points at either end with no
  neighbor are dropped.
- **Rationale**: Keeps the cadence contiguous, which the model needs.

## R8a. Review corrections (2026-10-28)

- **Decision**: An exclusion only keeps values out of the fit; recorded days
  count at their real value, and excluded trailing days are not forecast a
  second time. The output warns when history starts inside month 0 or ends
  more than a month before it. A cap on a month 0 of zero or less is ignored
  with a warning. CSV read errors report the parser's line; a first row is a
  header only when neither column parses; a UTF-8 BOM is skipped.
- **Rationale**: Independent review found a `FieldPos` panic on a malformed
  quote, an understated month 0, and a cap that zeroed every month.

## R9. Web transport

- **Decision**: The browser reads the CSV and sends its text in
  `spendHistory`; the server never takes a file path.
- **Rationale**: A path from the page would let any page in the session read
  local files. The 1 MiB request limit holds decades of daily rows.
