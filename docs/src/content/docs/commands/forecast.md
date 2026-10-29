---
title: cost forecast
description: Project future monthly spend from the current price and each plugin's growth model
---

`finfocus cost forecast` prices a plan the same way `cost projected` does, then
projects that monthly cost forward. Each plugin reports a growth model. The
monthly price itself stays the current month.

The plain view is an ASCII chart. JSON and NDJSON emit one point per month,
with a timestamp, so a later interactive history chart can plot the same
series. Stored cost history for `--stack` is included when that database
exists and uses the same currency. A missing history file does not fail the
command.

## Usage

```bash
finfocus cost forecast --pulumi-json plan.json --growth-type linear --growth-rate 0.10
finfocus cost forecast --stack prod --months 6 --growth-rate 0.05 --split-providers
finfocus cost forecast --pulumi-json plan.json --growth-rate 0.05 --output json
```

`--months` is how many months ahead to project. The series starts at the
current month, so `--months 12` is 13 points. The allowed range is 1 to 36.

## Growth

| Model | Formula | When |
| --- | --- | --- |
| none | cost stays flat | Plugin omitted the model, or `--growth-type none` |
| linear | `base * (1 + rate * month)` | Plugin reports `linear`, or `--growth-type linear` |
| exponential | `base * (1 + rate) ^ month` | Plugin reports `exponential`, or `--growth-type exponential` |

`--growth-rate 0.10` is 10% per month. Linear and exponential need a rate.
A plugin that reports one of those models without a rate stays flat, and the
output says so. A rate below -1 is rejected. A result below zero is shown as
zero.

Resources are projected on their own, then added. A missing currency counts
as USD, so it does not join a total in another currency. Two currencies fail
the command. A plugin error is left out of the total and listed. A global
budget in a different currency is left off the chart, and the output says so.

## Statistical forecast

`--model arima` forecasts from past spend instead of from the plan's growth
models. `--spend-history` is a CSV of `date,value` rows, one per consecutive
day or one per consecutive month, at least ten rows. A first row that is not a
number is a header. Dates are `2006-01-02` or RFC3339. Values are in the
plan's currency, or USD when the plan has none.

```bash
finfocus cost forecast --pulumi-json plan.json --model arima \
  --spend-history spend.csv --months 12
```

finfocus picks a model automatically, seasonal or not, and reports its order,
for example `SARIMA(4,1,2)(0,1,1)[7]` for daily spend with a weekly pattern.
Daily spend is forecast per day and summed into calendar months. Days the CSV
already records count at their recorded value, so a partly recorded current
month is the recorded days plus the forecast for the rest. A monthly row counts
as that whole month, so leave out a current month that is not over yet. The
output warns when the CSV starts partway through the current month (the days
before it are in no total) or ends more than a month before it (the forecast
spans the gap).

Each month has a lower and upper bound at `--confidence` (default 0.95). For
daily spend the monthly bounds are the sum of the daily bounds, which can only
make them wider than the true interval. Bounds grow with the horizon.

The plan is still priced. Its growth projection is the `baseline` series, so
you can compare the trend in spend with what the plan alone predicts. A plan
with nothing to price leaves the baseline out with a warning.

`--split-providers` has no effect here: the CSV has no providers.

A constant spend history forecasts a flat line, with a warning. A CSV with a
gap, a duplicate date, two cadences, a negative value, or a bad column is
rejected with its line number. A CSV may hold at most 3,660 rows, ten years of
daily spend, and needs at least ten usable rows after `--exclude`. Every input
error, including a bad `--months` or growth setting, is reported before any
plugin starts and exits with code 2.

## Drivers

Drivers apply known changes on top of the forecast.

| Flag | Effect | Models |
| --- | --- | --- |
| `--exclude FROM:TO` | Replace spend in the range with a straight line between its neighbors before fitting. Points at either end of the CSV are dropped instead. Recorded days still count at their real value in the month they fall in. Repeatable | arima |
| `--step DATE=AMOUNT` | Add AMOUNT to every month from DATE's month on. AMOUNT may be negative. A step before the current month applies from the current month. Repeatable | both |
| `--growth-cap FACTOR` | Limit every month, and each bound, to FACTOR times the current month. `1.25` is +25%. Applied after steps; ignored, with a warning, when the current month is zero | both |

Values and bounds below zero are shown as zero. The output names the months a
cap limited. With the growth model, drivers change the `forecast` total only;
provider lines stay the undriven projection.

## Output

`--output plain` draws the chart. `--split-providers` adds one line per
provider. A global budget in the config draws a Budget line unless
`--no-budget` is set. `--plain` forces the chart. `--no-color` and a
non-empty `NO_COLOR` turn chart colors off.

`--output json` is one document. `series` contains `forecast`, one series per
provider, and `history` when snapshots were loaded. With `--model arima` it
contains `forecast`, `lower`, `upper`, `baseline`, and `spend` (the CSV as
given), and the document adds `model`, `modelOrder`, and `confidence`. `budget` is the line
amount, omitted when the line is hidden. `--output ndjson` is a `meta` line,
then one `point` line per month, then `warning` lines.

## Options

| Flag | Description | Default |
| --- | --- | --- |
| `--pulumi-json` | Pulumi preview JSON. Omitted, the command detects the project | auto |
| `--terraform-state` | Terraform state file. Mutually exclusive with other input flags | |
| `--terraform-plan` | Saved `tofu show -json` output; runs no binary | |
| `--pulumi-state` | Pulumi stack export JSON; mutually exclusive with other input flags | |
| `--include-workspace` | Scope Terraform history to its workspace for this run; ignored for Pulumi | false |
| `--stack` | Stack for auto-detect and for reading cost history | current |
| `--months` | Months ahead (1–36) | 12 |
| `--growth-type` | `none`, `linear`, or `exponential` for every resource | plugin |
| `--growth-rate` | Decimal growth per month | |
| `--model` | `growth` or `arima` | growth |
| `--spend-history` | Spend CSV for `--model arima` | |
| `--confidence` | Bound level for `--model arima`, between 0 and 1 | 0.95 |
| `--exclude` | Spend range to ignore, `FROM:TO`; repeatable; arima only | |
| `--step` | `DATE=AMOUNT` added monthly from DATE's month; repeatable | |
| `--growth-cap` | Factor of the current month that no month exceeds | |
| `--split-providers` | Provider lines on the chart | false |
| `--no-budget` | Hide the global budget line | false |
| `--no-history` | Do not read cost history | false |
| `--output` | `plain`, `json`, or `ndjson` | plain |
| `--plain` | Force the ASCII chart | false |
| `--no-color` | Disable chart colors | false |
| `--height` | Chart height in rows | 15 |
| `--width` | Chart width in columns. 0 uses the terminal | 0 |
| `--filter` | Resource filter, repeatable | |
| `--adapter` | One adapter plugin | all |
| `--spec-dir` | Pricing spec directory | config |
| `--jobs`, `-j` | Parallel workers. 0 is automatic | 0 |
| `--profile` | Workload hint passed to projected pricing: `dev`, `prod`, `burst`, or `default`. Inherited from `cost` | no preference |

With no input file, forecast uses the same project detection as `cost projected`.
A Terraform directory is planned with `terraform` or `tofu`. Use
`--terraform-plan` to read a saved plan without running a binary.
