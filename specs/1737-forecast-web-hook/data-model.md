# Data Model: Cost Forecast Web Hook

## ForecastQuery (`webui.ForecastQuery`, request body)

| Field | JSON | Type | Default | Validation |
| --- | --- | --- | --- | --- |
| Months | `months` | `*int` | 12 (`forecast.DefaultMonths`) | 1 to 36, else `ErrInvalidMonths` |
| GrowthType | `growthType` | string | empty (plugin growth) | empty, `none`, `linear`, `exponential` (case-insensitive) |
| GrowthRate | `growthRate` | string | empty | decimal, >= -1; required with linear or exponential |
| NoHistory | `noHistory` | bool | false | none |

The rate is text, like the CLI flag, so the parse errors match.

## forecast.Document (shared CLI JSON and web payload)

Moved unchanged from `cli.forecastJSON`.

| Field | JSON | Notes |
| --- | --- | --- |
| Currency | `currency` | Forecast currency |
| Months | `months` | Horizon |
| Budget | `budget,omitempty` | Global budget when set and same currency |
| Series | `series` | `history` (if any), `forecast`, then providers |
| Warnings | `warnings,omitempty` | Sorted |

## ForecastResponse (`webui`, response body)

`forecast.Document` fields at the top level, plus:

| Field | JSON | Notes |
| --- | --- | --- |
| Display.Budget | `display.budget` | Empty when no budget |
| Display.Series[i].Name | `display.series[i].name` | Same order as `series` |
| Display.Series[i].Points[j].Month | `.month` | `Jan 2006`, UTC |
| Display.Series[i].Points[j].Value | `.value` | `viewmodel.FormatMoney` |

## forecast.InputError

Wraps a user-input error. `Error()` returns the wrapped text; `Unwrap()`
returns it, so `errors.Is(err, forecast.ErrInvalidMonths)` still works.

## forecastSettings (`internal/cli`)

`months`, `growthType`, `rate *float64`, `noHistory`, `noBudget`, `stack`,
`now`, `cfg`. Built from flags by the CLI and from `ForecastQuery` plus the
session by the web.
