# Research: Cost Forecast

## Decision: Project in core with the spec helpers

**Decision**: Call `pricing.ApplyGrowth`, `pricing.ValidateGrowthParams`, and
`pricing.CheckGrowthWarningsWithCost`. Do not reimplement the formulas.

**Rationale**: finfocus-spec already documents linear
`base * (1 + rate * n)` and exponential `base * (1 + rate)^n`. A second copy
would drift. Issue #364 says the engine takes current cost plus the plugin
growth model.

**Alternatives considered**: Applying growth inside each plugin. The response
field is documented as a hint so core can forecast without resource-type
knowledge. Putting the math in plugins would fork the formula per provider.

## Decision: The rate is a CLI flag

**Decision**: `--growth-rate` supplies the rate. The plugin response in
finfocus-spec v0.7.5 has `growth_type` and no `growth_rate`. The descriptor
and the request have the rate, and nothing in a Pulumi plan sets them today.

**Rationale**: Inventing a tag convention would be a new protocol. A missing
rate on a plugin-reported linear or exponential resource stays flat and is
warned. `--growth-type linear` without a rate is a user error and fails.

**Alternatives considered**: Requiring plugins to return a rate before this
ships. That is a finfocus-spec change and blocks #364 on a field the proto
does not have.

## Decision: Timestamped series, ASCII chart now

**Decision**: `forecast.Point` is a UTC month and an amount. `forecast.Series`
is a name plus points. asciigraph renders the forecast for humans. JSON keeps
the points.

**Rationale**: Issue #550 wants an ntcharts `TimeSeriesLineChart` over history
plus the future. asciigraph is index-based, so overlaying a shorter history
on the same axis would lie about months. #550 must read `time`. Bubble Tea is
already v2 (`charm.land/bubbletea/v2`), so #550's "stay on ntcharts v1" note
is stale. This change does not add ntcharts.

**Alternatives considered**: A Bubble Tea model in `internal/tui` now. That
would start #550 and give the next chart two models to delete.

## Decision: History is optional context

**Decision**: When `--stack` resolves a cost-history database in the same
currency, JSON includes a `history` series. The file is not required. A
currency mismatch drops the series and warns.

**Rationale**: The constitution forbids a command that fails because a store
is missing. #550 needs the points when they exist.

**Alternatives considered**: Drawing history on the ASCII chart by index.
Rejected because the x-axis would not be months.

## Decision: Do not absorb #539

**Decision**: No ARIMA, no CSV import, no growth cap. `cost estimate` is
unchanged.

**Rationale**: #539 is a different model on a different command. The point
type is enough for it to emit later. An unused forecaster interface would be
a stub.

## Decision: 36 month cap

**Decision**: `--months` is 1 through 36, default 12. The series includes the
current month plus N months ahead.

**Rationale**: The roadmap asks for 6–12 month charts. The SDK warns on
exponential projections past 36. Refusing 37 is clearer than plotting it.
