<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Thresholds and CI exit codes

## `.cost-check.yml`

Place the file in the project root. Every key is optional. A missing key uses
the default.

```yaml
currency: USD
thresholds:
  resource: 100
  increase: 50
  increase_pct: 20
  total: 1000
```

| Key | Default | Unit | Flags when |
| --- | --- | --- | --- |
| `thresholds.resource` | 100 | per month | one resource costs more than this |
| `thresholds.increase` | 50 | per month | the plan raises the total by more than this |
| `thresholds.increase_pct` | 20 | percent | the plan raises the total by more than this share |
| `thresholds.total` | 1000 | per month | the projected total is above this |
| `currency` | USD | code | the currency the amounts are written in |

A value that is not a non-negative number is an error. An unknown key is
ignored, and the report lists it. The `drift:` section of the same file
belongs to the `cost-drift` skill.

These thresholds are advisory flags for one run. They do not change the CLI
exit code. To make the CLI itself fail, configure a budget (see the
`finfocus-budget` skill).

## Baseline for the increase check

The preferred baseline is the plan itself: `finfocus.diff.totalBefore`. When
that is `0` and a previous `COST_CHECK.md` exists, its total is the baseline.
With neither, the increase flags are skipped and the report says so.

## CI exit codes

The skill runs inside an agent, so its own exit status belongs to the agent
runner. CI should read one of two things.

1. The result line at the end of the run:

   | Line | Meaning |
   | --- | --- |
   | `COST_CHECK_RESULT: PASS` | Priced, nothing flagged. |
   | `COST_CHECK_RESULT: FLAGGED` | Priced, at least one threshold flag. |
   | `COST_CHECK_RESULT: ERROR (<kind>)` | No trustworthy estimate. Kinds are in [failure-modes.md](failure-modes.md). |

2. The CLI's own exit code, when the pipeline runs `finfocus` directly.
   Codes are in [cli-contract.md](cli-contract.md). Add `--exit-on-threshold
   --exit-code <N>` to make a budget breach exit with `N`.
