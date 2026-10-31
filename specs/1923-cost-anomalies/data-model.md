# Data Model: Cost Anomalies Command

## Anomaly (`engine.Anomaly`)

| Field | JSON | Source | Notes |
| --- | --- | --- | --- |
| Resource | `resource` | `OverviewRow.URN` | |
| Type | `type` | `OverviewRow.Type` | |
| Period | `period` | `ActualCost.Period` | `engine.DateRange` object (`start`, `end`); zero when absent |
| Baseline | `baseline` | `CostDrift.Projected` | Projected, calendar-month basis |
| Observed | `observed` | `CostDrift.ExtrapolatedMonthly` | Monthly, not MTD |
| Deviation | `deviation` | `CostDrift.Delta` | Observed minus baseline |
| PercentDrift | `percentDrift` | `CostDrift.PercentDrift` | |
| Severity | `severity` | constant `warning` | One band |
| Method | `method` | constant `drift` | #1863 adds another |
| Currency | `currency` | `ActualCost.Currency` | |

## AnomalyReport (`engine.AnomalyReport`)

| Field | JSON | Notes |
| --- | --- | --- |
| Considered | `considered` | Count of input rows |
| Anomalies | `anomalies` | Never null; `[]` when empty |

## AnomalyDisplay (`viewmodel.AnomalyDisplay`, web only)

Display text for one anomaly, built in Go by `viewmodel.BuildAnomalyDisplay`.
The CLI JSON does not carry it.

| Field | Example | Notes |
| --- | --- | --- |
| Baseline | `$720.00` | Currency-formatted with the existing helper |
| Observed | `$1,080.00` | |
| Deviation | `+$360.00` | Signed |
| Drift | `+50.0%` | Signed, one decimal |
| Period | `Oct 1 – Oct 11, 2026` | Empty when the period is zero |

## AnomalyPage (`webui.AnomalyPage`, web only)

`considered`, `summary` (`2 of 10 resources`), and `anomalies`, where each item
is the `Anomaly` fields inline plus a `display` object. Stripping `display`
and `summary` leaves exactly the CLI JSON.

## Selection rule

A row is an anomaly when `Error == nil`, `CostDrift != nil`, and its status is
not deleting. Order:
`abs(Deviation)` descending, then `Resource` ascending. Pure function; no I/O.
