<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# cost-check failure modes

Name the failure in the report, do what the table says, and end with
`COST_CHECK_RESULT: ERROR (<kind>)`. Never fall back to a guess.

| Kind | How it shows up | What the agent does |
| --- | --- | --- |
| `no-binary` | `command -v finfocus` prints nothing. | Stop. Point to the `finfocus-install` skill. |
| `old-version` | `finfocus --version` is below the required version. | Stop. Print both versions. Point to `finfocus-install`. |
| `no-plugin` | `finfocus plugin list --output json` prints `[]`. | Continue. Local YAML specs can still price resources. Say no plugin is installed and point to `finfocus-install`. If every resource comes back unpriced, treat it as `zero-cost`. |
| `empty-plan` | `finfocus.emptyPlan` is `true`, even if readable state supplies `finfocus.stack`. | Stop. Say the plan holds nothing to price. Mention a stack total separately with its completeness status; never call the plan's `$0` an estimate. Ask for `pulumi preview --json --show-sames`, or point to `finfocus overview`. Do not write a plan-cost report. |
| `zero-cost` | Every resource has an `error`, or the CLI exits `0` with a total of `0` and only `NO_COST_DATA` rows. | Stop. Point to the `finfocus-diagnose` skill ("Zero cost"). Do not write a `$0` report. |
| `partial` | Some resources have an `error`, or `finfocus.errors[]` is not empty. | Continue. List them as unpriced and mark every total a lower bound. A failed baseline (old side of an update or replacement) makes the affected deltas and the plan increase flags unavailable, not lower bounds. |
| `no-credentials` | stderr names a Pulumi backend login, a secrets passphrase, or a cloud credential, or a row note starts with `ERROR:` and mentions auth. | Stop. Say which credential is missing. Never ask for the secret, print it, or set it. Offer `--pulumi-json` with a plan the user exports themselves. Point to `finfocus-diagnose`. |
| `no-plan` | `pulumi` is not on `PATH`, `preview` fails, or no plan file exists. | Ask for a plan file produced with `pulumi preview --json`. Do not price the program source by hand. |
| `terraform-no-state` | `*.tf` files but no state file. | Say that Terraform plan input is not available as of v0.5.0 and ask for a state file path. |
| `terraform-state-only` | A state file is used. | Continue. Add the caveat that pending changes are not priced. |
| `bad-input` | Exit `2`. | Report `message`. Fix the flag or file path and retry once. |
| `bad-thresholds` | `.cost-check.yml` has a non-numeric or negative value. | Stop. Name the key. |
| `budget-exceeded` | Non-zero exit, `error_code` is `budget_exceeded`, and stdout holds JSON. | Parse stdout as usual. Add the budget breach to the report. |
| `other` | Any other non-zero exit. | Stop. Print `error_code` and `message` verbatim. |

## Notes

- A resource with `monthly` of `0` and no `error` is a priced resource that
  costs nothing, such as an IAM role. It is not a failure.
- Types that start with `pulumi:` are framework resources. The CLI skips
  them, so they do not appear in the output.
- A plan row without `newState` inputs can come back unpriced. The
  `finfocus-diagnose` skill covers that.
