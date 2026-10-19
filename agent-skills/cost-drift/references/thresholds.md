<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Drift thresholds and CI exit codes

## `.cost-check.yml`

The `drift:` section lives in the same file as the `cost-check` thresholds.
Every key is optional. A missing key uses the default.

```yaml
currency: USD
drift:
  drift_pct: 10
  drift_absolute: 50
  total_drift_pct: 15
```

| Key | Default | Unit | Flags when |
| --- | --- | --- | --- |
| `drift.drift_pct` | 10 | percent | `abs(costDrift.percentDrift)` is above it |
| `drift.drift_absolute` | 50 | per month | `abs(costDrift.delta)` is above it |
| `drift.total_drift_pct` | 15 | percent | overall derived drift is above it |
| `currency` | USD | code | the currency the amounts are written in |

A value that is not a non-negative number is an error. If `currency` differs
from `summary.currency`, skip `drift_absolute` and say why.

## What the CLI limits

The CLI emits `costDrift` only for a drift above 10%. A `drift_pct` below 10
therefore cannot flag anything the CLI did not already report. Keep the key,
and add a report caveat when it is below 10. Rows without `costDrift` are
shown in the non-drifting table with actual and projected values and a drift
column that reads `within 10%`, `new or deleted`, or `insufficient data`.

## Overall drift (derived)

`summary.totalActualMTD` and `summary.projectedMonthly` are summed
independently. The CLI skips actuals for rows with `status` `creating`, but
their projection is in the projected total, so a pending $100 create next to a
$100 resource that spends as projected reads as about -50%. Missing actuals on
other rows do the same. Do not divide the two summary totals.

Use only comparable rows: rows that have both `actualCost` and
`projectedCost` and whose `status` is not `creating`. Label the figure
"derived" and give the row count. If any row with a projection lacks actuals,
or the rows are fewer than all projected rows, say which are left out.
Report the overall figure as unavailable when no comparable row exists or
`projectedMonthly` over those rows is `0`.

```text
elapsed_days = (metadata.timeWindow.end - metadata.timeWindow.start) in days
days_in_month = days in the month of metadata.timeWindow.end
actual_month = sum(mtdCost over comparable rows) * days_in_month / elapsed_days
projected_month = sum(monthlyCost over comparable rows) * days_in_month / 30.4167
overall_drift_pct = (actual_month - projected_month) / projected_month * 100
```

`30.4167` is 730 hours divided by 24, the month the projection is priced on.
A resource created inside the window makes `elapsed_days` too long for it, so
the figure is approximate.

## Baseline for the trend

The previous `COST_DRIFT.md` gives the earlier overall drift and actual
total. Without one, the trend section is omitted.

## CI exit codes

The skill runs inside an agent, so its own exit status belongs to the agent
runner. CI should read one of two things.

1. The result line at the end of the run:

   | Line | Meaning |
   | --- | --- |
   | `COST_DRIFT_RESULT: PASS` | Data present, nothing flagged. |
   | `COST_DRIFT_RESULT: DRIFTING` | Data present, at least one flag. |
   | `COST_DRIFT_RESULT: ERROR (<kind>)` | No trustworthy drift figure. Kinds are in [failure-modes.md](failure-modes.md). |

2. The CLI's own exit code, when the pipeline runs `finfocus` directly.
   Codes are in [cli-contract.md](cli-contract.md).
