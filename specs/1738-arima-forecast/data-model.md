# Data Model: Statistical Cost Forecast

## SpendHistory (`forecast.SpendHistory`)

| Field | Type | Notes |
| --- | --- | --- |
| Cadence | `Cadence` (`CadenceDaily`, `CadenceMonthly`) | Detected from dates |
| Points | `[]Point` | Sorted, UTC; monthly points at month start |

Validation (each a `forecast.InputError` with the CSV line number): two
columns, date `2006-01-02` or RFC3339, finite non-negative value, no
duplicate date, consecutive days or consecutive months, at least one row.
A first row whose value is not a number is a header.

## Drivers (`forecast.Drivers`)

| Field | Flag | Validation |
| --- | --- | --- |
| Excludes `[]DateRange` | `--exclude FROM:TO` | Both dates valid, FROM <= TO |
| Steps `[]Step{Month time.Time; Amount float64}` | `--step DATE=AMOUNT` | Date valid, amount finite |
| GrowthCap `float64` | `--growth-cap` | > 0 and finite; 0 means unset |

## Settings additions (`internal/cli.forecastSettings`)

`model` (`growth` or `arima`), `spend *forecast.SpendHistory`,
`confidence float64`, `drivers forecast.Drivers`.

## Projection additions (`forecast.Projection`)

| Field | Notes |
| --- | --- |
| Model, ModelOrder | Set in arima mode |
| Confidence | Set in arima mode |
| Lower, Upper | Bound series (`lower`, `upper`) |
| Baseline | Plan growth projection (`baseline`) |
| Spend | Input series (`spend`) |

## Document additions (`forecast.Document`)

`model`, `modelOrder`, `confidence` (all `omitempty`). Series order:
`history`, `forecast`, `lower`, `upper`, `baseline`, `spend`, then
providers (growth mode only).

## ForecastQuery additions (`webui.ForecastQuery`)

`model`, `spendHistory` (CSV text), `confidence *float64`,
`growthCap *float64`, `steps []string`, `exclude []string`; each string uses
the flag's syntax.
