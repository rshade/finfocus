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

Resources are projected on their own, then added. Two currencies fail the
command. A plugin error is left out of the total and listed.

## Output

`--output plain` draws the chart. `--split-providers` adds one line per
provider. A global budget in the config draws a Budget line unless
`--no-budget` is set. `--plain` forces the chart. `--no-color` and a
non-empty `NO_COLOR` turn chart colors off.

`--output json` is one document. `series` contains `forecast`, one series per
provider, and `history` when snapshots were loaded. `budget` is the line
amount, omitted when the line is hidden. `--output ndjson` is a `meta` line,
then one `point` line per month, then `warning` lines.

## Options

| Flag | Description | Default |
| --- | --- | --- |
| `--pulumi-json` | Pulumi preview JSON. Omitted, the command detects the project | auto |
| `--terraform-state` | Terraform state file. Mutually exclusive with `--pulumi-json` | |
| `--stack` | Stack for auto-detect and for reading cost history | current |
| `--months` | Months ahead (1–36) | 12 |
| `--growth-type` | `none`, `linear`, or `exponential` for every resource | plugin |
| `--growth-rate` | Decimal growth per month | |
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
