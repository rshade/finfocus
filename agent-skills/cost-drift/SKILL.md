---
name: cost-drift
description: >
  Compare actual and projected spend for a Pulumi stack with the FinFocus CLI,
  flag drifting resources against thresholds in .cost-check.yml, explain likely
  causes, track the trend, and write a COST_DRIFT.md report. Reads
  machine-readable JSON, checks the CLI version, and names what to do when the
  CLI, a billing plugin, credentials, or a Pulumi preview is missing. Use when
  investigating a cost overrun, validating projections against reality, or
  watching spend drift in CI. Triggers on: "cost drift", "actual vs projected",
  "why is spend higher than expected", "drift report", "COST_DRIFT.md".
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Cost Drift

Run `finfocus overview` on a Pulumi stack, flag resources whose actual spend
has moved away from the projection, and write `COST_DRIFT.md`.

Install: `npx skills add rshade/finfocus -s cost-drift`.

**Required CLI: finfocus v0.5.0 or newer.** Update this line with the
release the skill was last run against.

**Core principle:** use the drift the CLI computes. Do not query billing APIs,
and do not turn a missing `costDrift` into "no drift".

Pulumi only. `overview` has no Terraform input as of v0.5.0. For a Terraform
project, run the `cost-check` skill and say that drift is not available. Edit
this paragraph when that changes.

## 1. Detect the project

Look for `Pulumi.yaml` or `Pulumi.yml` in this directory or a parent. If there
is none, stop: "No Pulumi project detected."

## 2. Preflight

```bash
command -v finfocus
finfocus --version
finfocus plugin list --output json
```

- No binary: stop and point to the `finfocus-install` skill.
- Version below the required one: stop and print both versions. A build such
  as `v0.5.0-6-g3cb7ded` counts as v0.5.0. An unparseable version, or `0.1.0`
  with no leading `v` (an unstamped source build), continues, and the report
  says "version unverified".
- No plugin whose `capabilities` include `actual_costs`: stop. Drift needs a
  plugin that returns billing data. Point to `finfocus-install`.

Every failure and its handling:
[references/failure-modes.md](references/failure-modes.md).

## 3. Load thresholds and baseline

Read the `drift:` section of `.cost-check.yml`. Absent keys use the defaults:
[references/thresholds.md](references/thresholds.md). If a previous
`COST_DRIFT.md` exists, keep its drifting-resource count and actual total for
the trend.

## 4. Run the CLI

```bash
finfocus overview --output json --yes
finfocus overview --output json --yes --stack <stack> --from 2026-10-01
finfocus overview --output json --yes --state-only
```

By default `overview` runs `pulumi stack export` and `pulumi preview --json`.
`--state-only` skips the preview. `--pulumi-state` and `--pulumi-json` use
files the user already has. Keep stdout and stderr apart and record the exit
code. Fields and exit codes: [references/cli-contract.md](references/cli-contract.md).

## 5. Validate before trusting the numbers

1. Exit `0`, or a budget exit (`error_code` `budget_exceeded`) with a JSON
   document on stdout. Record the budget breach in the report and keep
   going: an overrun is when drift matters most. Any other exit: stop with
   `error_code` and `message`.
2. `errors` is empty, or each entry is listed in the report.
3. At least one row has `actualCost`. If none does, stop with `no-actuals`
   and point to `finfocus-diagnose`. A present `actualCost` does not prove
   billing data: when a plugin returns `0` with no breakdown, the CLI
   substitutes the projected hourly rate times uptime, and the overview JSON
   drops the adapter, confidence and notes that would show it. Never call the
   figures billing-backed. Print "Billing verification: unavailable" in the
   summary and the report, and read `PASS` as "no drift flagged against the
   CLI's figures".
4. A row has drift only when it carries `costDrift`. The CLI leaves
   `costDrift` out when the drift is within 10%, when the resource is new or
   deleted, and when less than 2 days of the window have elapsed. Report those
   rows as "within 10%", "new or deleted", or "insufficient data". Never as
   zero drift.

## 6. Flag

Read each row's `costDrift`:

1. `abs(percentDrift)` above `drift_pct`.
2. `abs(delta)` above `drift_absolute`.

There is no overall drift figure. The CLI measures each row from its real
start, and the summary totals cannot be compared fairly (see
[references/thresholds.md](references/thresholds.md)).

Overspend and underspend both count. `drift_pct` below 10 cannot flag more
than the CLI reports, and the report says so when the user set one.

## 7. Explain

For each flagged row, match `type` to
[references/drift-explanations.md](references/drift-explanations.md). Pick
overspend when `delta` is above 0, underspend when below. Use the fallback for
types that are not listed. Each entry gives a likely cause and an
investigation step.

## 8. Report

Print the drifting-resource count, the trend against the previous check, and
each flagged resource with actual, projected, delta, drift, cause, and next
step.
Write `COST_DRIFT.md` from
[references/report-template.md](references/report-template.md). Run
`markdownlint COST_DRIFT.md` when the tool is installed.

End with one line CI can match:

```text
COST_DRIFT_RESULT: PASS | DRIFTING | ERROR (<kind>)
```

Then ask whether to investigate a resource or adjust thresholds.
