# Budget health

## Status

`CalculateBudgetHealthFromPercentage` maps utilization to a status:

| Status | Utilization | Severity |
| --- | --- | --- |
| OK | Below 80% | 1 |
| WARNING | At least 80% and below 90% | 2 |
| CRITICAL | At least 90% and below 100% | 3 |
| EXCEEDED | At least 100% | 4 |
| UNSPECIFIED | No budget status to score | 0 |

Negative utilization is treated as OK. Constants: `HealthThresholdWarning`
80, `HealthThresholdCritical` 90, `HealthThresholdExceeded` 100.

Across scopes, the worst severity wins:
`EXCEEDED` over `CRITICAL` over `WARNING` over `OK` over `UNSPECIFIED`.

## Alerts

An empty `alerts` list becomes 50% actual, 80% actual, and 100% actual
(`DefaultThreshold50`, `DefaultThreshold80`, `DefaultThreshold100`).

| `type` | Compared with |
| --- | --- |
| `actual` | Current spend / amount |
| `forecasted` | Forecast spend / amount |

Threshold status (`evaluateThreshold` in `internal/engine/budget_cli.go`):

- `EXCEEDED` when utilization is at or above the threshold
- `APPROACHING` when it is within 5 points under the threshold
  (`ApproachingThresholdBuffer`)
- `OK` otherwise

## Forecast

`CalculateForecastedSpendAt` is the elapsed-duration helper. It extrapolates
linearly over the budget period:

```text
forecast = (currentSpend / elapsed) * periodLength
```

`elapsed` and `periodLength` are durations. Ten days into a 30-day period,
$100 spent forecasts $300. Before the period starts, or when no time has
elapsed, the forecast is the current spend. After the period ends, the
forecast is the current spend. Zero spend forecasts 0. A limit at or below 0
yields forecast utilization 0.

The global CLI evaluator and the scoped status enricher use a day-of-month
formula instead:

```text
forecast = (currentSpend / dayOfMonth) * daysInMonth
```

`dayOfMonth` is the current day, and a day of 0 is treated as 1.
`daysInMonth` is the length of that month. On day 15 of a 31-day month, $300 spent forecasts $620. Use this formula for a `forecasted` CLI alert. The
elapsed-duration helper answers a different question.

Overview cost drift uses a different pair of formulas (30-day delta versus
calendar-day drift). Do not reuse those for a budget forecast.

## What the table shows

`RenderBudgetStatus` and `RenderScopedBudgetStatus` print `BUDGET STATUS` for
table output: amount, current spend, utilization, health, and forecast.
Scoped output groups global, provider, tag, and type sections, then an
overall health line. `--budget-scope` hides sections. The banner is not
inserted into JSON or NDJSON cost output.
