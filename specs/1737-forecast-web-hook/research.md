# Research: Cost Forecast Web Hook

## R1. Where the shared builder lives

- **Decision**: Pricing stays in `internal/cli` (`forecastCosts` for the CLI,
  `webData.priceForecast` for the web). Everything after pricing goes through
  one cmd-free `buildForecast(costs, settings, readHistory)` in
  `internal/cli/forecast_build.go`. Flag-independent pieces with no CLI
  dependency (`Document`, `NewDocument`, `SeriesList`, `ParseGrowth`,
  `InputError`) move to `internal/forecast`.
- **Rationale**: Pricing differs by design: the CLI opens plugins and loads
  the plan; the web reuses the session snapshot and engine (FR-006). The
  projection, history, budget, and document assembly are identical, so they
  share one function. `internal/webui` cannot import `cli`, so the document
  type must sit in a package both can import.
- **Alternatives considered**: A single `buildForecast(ctx, input)` that also
  prices (the issue's sketch) would need an "engine or plugin opener"
  parameter and duplicate the session's resource loading. Rejected.

## R2. Validation errors and HTTP 400

- **Decision**: `forecast.InputError` wraps each user-input error. Its
  `Error()` is the wrapped message, so CLI text is unchanged.
  `forecast.ParseGrowth(kind, rate)` and `forecast.ValidateMonths(months)`
  return it. The handler maps `errors.As(err, *forecast.InputError)` to 400
  `invalid_forecast` with `err.Error()`; any other error is 502
  `forecast_unavailable` with a fixed message.
- **Rationale**: The handler must tell input errors from pricing failures
  without string matching, and FR-005 requires the CLI's text verbatim.
  `errors.Is(err, forecast.ErrInvalidMonths)` still holds through `Unwrap`.
- **Alternatives considered**: Re-validating in the handler would duplicate
  the CLI's rules. Rejected.

## R3. Resource membership in the web session

- **Decision**: The web prices `snapshot.Descriptors()` with
  `planOriginResources`, then the launch filters and
  `resolveResourceTypes`, then `GetProjectedCostDiff(...).AfterCosts()`.
  A Terraform session uses its planned resources.
- **Rationale**: That is the CLI's path (`loadProjectedResources` →
  `planOriginResources` → `ApplyFilters` → resolve → diff). #1903 keeps
  plan membership until decided.
- **Alternatives considered**: `liveResources()` (the estimate and
  recommendations inventory) includes unchanged state and collapses
  replacements; using it would change the total. Rejected.

## R4. History stack

- **Decision**: The web reads history for the session stack:
  `snapshot.StackName`, falling back to the launch `--stack`. The read uses
  the same `historyPath` and `OpenCostDBRead` helper as the CLI
  (`readForecastHistory(stack)`).
- **Rationale**: The CLI reads history only for an explicit `--stack`; the
  web session always knows its stack after loading. For the same stack both
  read the same file, which is what parity requires.

## R5. Display strings

- **Decision**: The handler adds `display.budget` and, per series, a month
  label (`Jan 2026`, UTC) and `viewmodel.FormatMoney(value, currency)` per
  point.
- **Rationale**: Formatting is presentation, which `internal/webui` already
  does for recommendations and estimates through `viewmodel`; no value is
  computed. The browser renders strings only.

## R6. Warnings and redaction

- **Decision**: Warnings are returned as the CLI prints them, through the
  standard `WriteJSON` redaction. The document has no maps, so no data-keyed
  marking is needed. A failed pricing call returns a fixed message.
- **Rationale**: The dashboard may show what the CLI JSON shows; skip
  warnings are CLI JSON content. Whole-call failures may carry plugin or
  subprocess text and are replaced (FR-008).

## R7. Budget and no-history

- **Decision**: The web always evaluates the global budget
  (`forecastBudgetFor(cfg, currency, false)`) with the session config;
  `noHistory` maps to the request field.
- **Rationale**: `--no-budget` hides a chart line; the view can hide it
  itself (spec Assumptions).

## R8. Nothing to project, and session-scoped engine options

- **Decision**: `forecast.ErrNoCostData` and `forecast.ErrMixedCurrencies`
  are reported as 422 `no_forecast` with the CLI's message; they describe the
  stack, not an outage, and carry no plugin text. The web uses the session
  engine as built at launch, so the CLI-only `--spec-dir` and `--jobs`
  options do not apply.
- **Rationale**: Review found that a 502 would make an empty or
  mixed-currency plan look like a plugin failure. The web session has no
  `--spec-dir` or `--jobs` flags; every web view uses the session engine.
