---
name: cost-check
description: >
  Estimate the monthly cost of a Pulumi or Terraform project with the FinFocus
  CLI, flag expensive resources, cost increases, and total cost against
  thresholds in .cost-check.yml, and write a COST_CHECK.md report. Reads
  machine-readable JSON, checks the CLI version, and names what to do when the
  CLI, a plugin, credentials, or a plan is missing. Use when checking
  infrastructure costs, reviewing the cost impact of a change, or gating CI on
  cost. Triggers on: "cost check", "estimate infrastructure cost", "how much
  will this cost", "flag expensive resources", "cost threshold", "COST_CHECK.md".
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Cost Check

Run `finfocus cost projected` on the current project, apply thresholds, and
write `COST_CHECK.md`.

Install: `npx skills add rshade/finfocus -s cost-check`.

**Required CLI: finfocus v0.5.0 or newer.** Update this line with the
release the skill was last run against.

**Core principle:** read the numbers the CLI returns. Never estimate a price
by hand, never parse the table output, and never report an unpriced resource
as `$0`.

## 1. Detect the input

| Found in the project | Input | Status |
| --- | --- | --- |
| `Pulumi.yaml` or `Pulumi.yml` (this directory or a parent) | `--pulumi-json plan.json`, or auto-detect | supported |
| `terraform.tfstate` | `--terraform-state terraform.tfstate` | supported, state only |
| `*.tf` with no state file | none | not supported: Terraform plan input is not available as of v0.5.0 |

Terraform: the CLI prices what is in the state file, so a state that does not
exist yet or is stale gives no estimate for the pending change. Say so in the
report. Do not run `terraform plan` and do not convert its output. When
Terraform plan input ships, edit this table and `references/cli-contract.md`.

If a Pulumi project and a Terraform state are both present, run each
separately. If nothing is found, stop: "No Pulumi project or Terraform state
found in this directory."

## 2. Preflight

```bash
command -v finfocus
finfocus --version
finfocus plugin list --output json
```

- No binary: stop, name the failure, point to the `finfocus-install` skill.
- Version older than the required one: stop and print both versions. A build
  such as `v0.5.0-6-g3cb7ded` counts as v0.5.0. An unparseable version, or
  `0.1.0` with no leading `v` (an unstamped source build), continues, and the
  report says "version unverified".
- `plugin list` returns `[]`: no plugin is installed. Continue. `cost
  projected` can still price resources from local YAML specs, so an empty
  list does not mean nothing is priced. Step 5 decides whether an estimate
  exists. Point to `finfocus-install` for the missing prices.

Failure handling for every step is in
[references/failure-modes.md](references/failure-modes.md).

## 3. Load thresholds

Read `.cost-check.yml` in the project root. Absent keys use the defaults.
Format, defaults, and CI exit codes:
[references/thresholds.md](references/thresholds.md).

## 4. Run the CLI

Prefer a plan file the user already has. Otherwise the CLI runs
`pulumi preview --json` itself, so tell the user before it starts.

```bash
finfocus cost projected --pulumi-json plan.json --output json
finfocus cost projected --output json --stack <stack>
finfocus cost projected --terraform-state terraform.tfstate --output json
```

Keep stdout and stderr apart and record the exit code. Stdout is one JSON
document. Stderr carries a JSON error with `error_code` and `message`. Field
names, exit codes, and the optional `cost forecast` call are in
[references/cli-contract.md](references/cli-contract.md).

## 5. Validate before trusting the numbers

1. Exit code `0`, or a budget exit with a JSON document on stdout. Any other
   exit: stop with the `error_code` and `message`.
2. `finfocus.resources` is non-empty.
3. A resource with an `error` object is **unpriced**. List it. Never add it
   as `0`. Also read `finfocus.errors[]`: when the old side of an update or
   replacement fails to price, the failure appears only there, and the diff
   treats that baseline as `0`. List each entry. Mark the delta of every
   affected resource, and any plan increase flag that depends on it,
   **unavailable**. Never report them as a lower bound.
4. Every resource unpriced: stop. This is the zero-cost case. Point to the
   `finfocus-diagnose` skill ("Zero cost").
5. A resource with no `error` and `monthly` of `0` is a real result. Report it.

## 6. Flag

1. Expensive resource: `monthly` above `thresholds.resource`.
2. Increase: `finfocus.diff.totalDelta` above `thresholds.increase`. A plan
   with `totalBefore` of `0`, such as one that only creates resources, is
   valid, so always make this check. Also flag the percentage above
   `thresholds.increase_pct` (`totalDelta / totalBefore * 100`), but only when
   `totalBefore` is above 0; otherwise skip the percentage check and say so.
   Skip both when step 5 marked the baseline unavailable.
3. Total: `finfocus.summary.totalMonthly` above `thresholds.total`.
4. Unpriced resources are not a threshold flag. They make every total a lower
   bound, and the report says that.

If `currency` is not the thresholds file's currency (default `USD`), skip the
dollar flags and say why.

## 7. Report

Print a terminal summary, then write `COST_CHECK.md` from
[references/report-template.md](references/report-template.md). Run
`markdownlint COST_CHECK.md` when the tool is installed.

End with one line CI can match:

```text
COST_CHECK_RESULT: PASS | FLAGGED | ERROR (<kind>)
```

Show the total, the flag counts, the top flagged items, the delta, and the
unpriced list. Then ask whether to adjust thresholds or look at an item.
