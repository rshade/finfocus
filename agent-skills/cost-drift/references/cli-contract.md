<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# cost-drift CLI contract

Read from the FinFocus CLI help and source, and checked against a binary
built from the `v0.5.0` line. A test in `internal/cli/cost_skills_test.go`
pins the flags and field names.

## Commands

| Command | Use |
| --- | --- |
| `finfocus --version` | Prints `finfocus version vX.Y.Z`, with `-N-gSHA` on a build past a tag. |
| `finfocus plugin list --output json` | JSON array of plugins with `name`, `version`, `capabilities`, `active`. `actual_costs` marks a plugin that can return actuals. |
| `finfocus overview` | Stack dashboard: state, plan, actual, projected, drift. |

## `overview` flags the skill uses

| Flag | Meaning |
| --- | --- |
| `--output json` | Machine output. Values: `table`, `json`, `ndjson`. |
| `--yes` | Skip confirmation prompts. |
| `--stack <name>` | Stack for auto-detection. Ignored with the file flags. |
| `--from <date>`, `--to <date>` | Window, `YYYY-MM-DD` or RFC3339. Default: the start of the month to now. |
| `--state-only` | Skip `pulumi preview`. Faster, and pending changes are not detected. |
| `--pulumi-state <file>`, `--pulumi-json <file>` | Use exported files instead of running Pulumi. |
| `--adapter <name>` | Use one plugin only. |

`overview` accepts `--terraform-state` for a saved state and `--terraform-plan` for saved plan JSON. Without `--output`,
`--format json` or `AGENT_MODE=1` also select JSON.
`-o` is not a flag.

## JSON document

| Path | Meaning |
| --- | --- |
| `metadata.stackName`, `metadata.timeWindow.start`, `.end` | Stack and the window that `mtdCost` covers. |
| `summary.totalActualMTD` | Actual spend over the window, summed. Not a monthly figure. |
| `summary.projectedMonthly` | Projected spend for a full month, summed. |
| `summary.currency` | Currency code. |
| `errors[]` | Fetch errors: `urn`, `errorType`, `message`, `retryable`. |
| `resources[]` | One row per resource: `urn`, `type`, `status`. |
| `resources[].actualCost.mtdCost` | Actual spend over the window. |
| `resources[].projectedCost.monthlyCost` | Projected monthly cost. |
| `resources[].costDrift` | `extrapolatedMonthly`, `projected`, `delta`, `percentDrift`, `isWarning`. Present only for a drift above 10%. |
| `resources[].warnings[]` | Values such as `drift`, `error`, `new`. |

`costDrift` is left out when the drift is at most 10%, when the resource has
actual spend but no projection or the reverse, when both are zero, and when
less than 2 elapsed days are available. `extrapolatedMonthly` scales
`mtdCost` to the calendar month. `projected` and `delta` are on the same
calendar-month basis, so do not recompute them from `monthlyCost`.

## Exit codes

| Code | Meaning | stderr `error_code` |
| --- | --- | --- |
| `0` | Success. Also `0` when no actual data came back. | none |
| `1` | Runtime failure, such as no Pulumi project or `pulumi` missing from `PATH`. | `internal_error` |
| `2` | Invalid flag, value, or file. | `validation_error` |
| `1`, or `--exit-code N` (0 to 255) | A configured budget threshold was exceeded. Stdout still holds the document. `--exit-on-threshold` on `overview` applies to non-TTY output. | `budget_exceeded` |

Budget gates are covered by the `finfocus-budget` skill.
