<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# cost-check CLI contract

Everything here was read from the FinFocus CLI help and source, and checked
against a binary built from the `v0.5.0` line. A test in
`internal/cli/cost_skills_test.go` pins the flags and field names.

## Commands

| Command | Use |
| --- | --- |
| `finfocus --version` | Prints `finfocus version vX.Y.Z`, with `-N-gSHA` on a build past a tag. |
| `finfocus plugin list --output json` | JSON array of installed plugins: `name`, `version`, `specVersion`, `capabilities`, `active`. `[]` means none. |
| `finfocus cost projected` | Projected monthly cost of a plan or a Terraform state. |
| `finfocus cost forecast` | Optional. Projects the same cost forward by month. |

## `cost projected` flags the skill uses

| Flag | Meaning |
| --- | --- |
| `--pulumi-json <file>` | Pulumi `preview --json` output. Omit it to auto-detect the project and run `pulumi preview --json`. |
| `--terraform-state <file>` | Terraform state file. Mutually exclusive with `--pulumi-json`. |
| `--stack <name>` | Stack for auto-detection. Ignored with `--pulumi-json`. |
| `--output json` | Machine output. Values: `table`, `json`, `ndjson`. |
| `--adapter <name>` | Use one plugin only. |
| `--filter <expr>` | Resource filter, for example `type=aws:ec2/instance`. |

`--output json` is explicit and wins over everything else. Without
`--output`, `--format json` or `AGENT_MODE=1` also select JSON.
`-o` is not a flag: it fails with exit `2`.

## JSON document

`cost projected --output json` prints one object with a single key,
`finfocus`:

| Path | Meaning |
| --- | --- |
| `finfocus.summary.totalMonthly` | Total projected monthly cost. |
| `finfocus.summary.currency` | Currency code. |
| `finfocus.summary.byProvider`, `byService`, `byAdapter` | Totals keyed by provider, service, and plugin. |
| `finfocus.resources[]` | One row per resource. Use this array, not `summary.resources`. |
| `resources[].resourceType`, `resourceId` | Pulumi type token and URN, or the Terraform address. |
| `resources[].monthly`, `hourly` | Projected cost of the resource after the change. |
| `resources[].error` | Absent when priced. Otherwise `{code, message, resourceType}`. |
| `resources[].operation`, `beforeMonthly`, `deltaMonthly` | Plan operation and its cost change. |
| `finfocus.errors[]` | Present when plugin calls failed: `resourceType`, `resourceId`, `pluginName`, `message`. |
| `finfocus.diff.totalBefore`, `totalAfter`, `totalDelta` | Whole-plan change. `creates`, `updates`, `deletes`, `unchanged` count operations. |

`error.code` is one of `NO_COST_DATA`, `PLUGIN_ERROR`, `VALIDATION_ERROR`,
`TIMEOUT_ERROR`. The `finfocus-diagnose` skill explains each one. The
document has no stack name. Group by `resourceType` in the skill when a type
breakdown is needed.

A Terraform state has no pending operations, so `totalDelta` is `0` and every
resource is `unchanged`.

## Exit codes

| Code | Meaning | stderr `error_code` |
| --- | --- | --- |
| `0` | Success. Also `0` when every resource is unpriced. | none |
| `1` | Runtime failure, such as no Pulumi project, `pulumi` missing from `PATH`, or a failed preview. | `internal_error` |
| `2` | Invalid flag, value, or input file. | `validation_error` |
| `1`, or `--exit-code N` (0 to 255) | A configured budget threshold was exceeded. Stdout still holds the full JSON document. | `budget_exceeded` |

The budget exit only happens when `--exit-on-threshold` is passed or the
budget config sets `exit_on_threshold`. See the `finfocus-budget` skill. The
exit code a user picks can collide with `1` or `2`, so read `error_code` as
well as the number.

## Optional forecast

```bash
finfocus cost forecast --pulumi-json plan.json --output json --months 12
```

Prints `currency`, `months`, a `series` array (`name`, `points[]` with `time`
and `value`), and a `warnings` array that lists each resource it could not
price. Use it only when the user asks for a horizon beyond the current month.
