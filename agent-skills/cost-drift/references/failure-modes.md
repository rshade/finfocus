<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# cost-drift failure modes

Name the failure in the report, do what the table says, and end with
`COST_DRIFT_RESULT: ERROR (<kind>)`.

| Kind | How it shows up | What the agent does |
| --- | --- | --- |
| `no-binary` | `command -v finfocus` prints nothing. | Stop. Point to the `finfocus-install` skill. |
| `old-version` | `finfocus --version` is below the required version. | Stop. Print both versions. |
| `no-plugin` | `plugin list --output json` prints `[]`, or no active plugin has `actual_costs`. | Stop. Point to `finfocus-install`. |
| `no-actuals` | Exit `0`, but no row has `actualCost`. | Stop. Point to `finfocus-diagnose`. A plugin that estimates from public prices does not supply billing actuals. |
| `no-credentials` | stderr or an `errors[]` message names a billing, backend, or secrets credential. | Stop. Say which credential is missing. Never ask for it, print it, or set it. Point to `finfocus-diagnose`. |
| `no-preview` | `pulumi` is missing from `PATH` or `preview` fails. | Retry once with `--state-only`. If it still fails, ask for `--pulumi-state` and `--pulumi-json` files. |
| `no-project` | No `Pulumi.yaml` or `Pulumi.yml`. | Stop. |
| `terraform` | The project is Terraform. | Stop. Say drift is Pulumi only as of v0.5.0. Suggest `cost-check`. |
| `insufficient-data` | Fewer than 2 days into the window, so no row has `costDrift`. | Report "insufficient data". Suggest `--from` an earlier date or rerun later. |
| `partial` | `errors[]` is not empty but some rows have data. | Continue. List the errors and the rows they affect. |
| `budget-exceeded` | Non-zero exit, `error_code` is `budget_exceeded`, and stdout holds JSON. | Parse stdout as usual. Add the budget breach to the report. |
| `bad-input` | Exit `2`. | Report `message`. Fix the flag or path and retry once. |
| `bad-thresholds` | `drift:` has a non-numeric or negative value. | Stop. Name the key. |
| `other` | Any other non-zero exit. | Stop. Print `error_code` and `message` verbatim. |

A report built on a partial result says which resources are missing.

An `actualCost` of `0` is a valid result: a plugin can return zero charges.
Zero alone shows neither missing data nor billing provenance, so it is not a
`no-actuals` case. The CLI omits `costDrift` when both sides are zero.
