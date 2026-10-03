---
name: finfocus-budget
description: >
  Configure, check, and troubleshoot FinFocus budget thresholds. Use when setting
  cost.budgets in config.hujson, reading budget health, filtering scopes with
  --budget-scope, or making cost projected, cost actual, or overview exit non-zero
  in CI. Triggers on: "finfocus budget", "budget threshold", "exit-on-threshold",
  "budget health", "scoped budget", "cost.budgets", or a CI gate on FinFocus cost.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus budgets

This skill is FinFocus-specific. Generic multi-tool budget workflows live in
`rshade/agent-skills`, not here.

## 1. Put the limit in config

`finfocus config init` writes `$PROJECT/.finfocus/config.hujson` inside a Pulumi
project, or `~/.finfocus/config.hujson` otherwise. Project config overrides
global config at the top-level key. A legacy `config.yaml` is migrated on read.

Set `cost.budgets`. A global amount is required when any provider, tag, or type
budget is present. Amount `0` disables that scope. Only `monthly` is valid.

```hujson
{
  "cost": {
    "budgets": {
      "global": {
        "amount": 500,
        "currency": "USD",
        "exit_on_threshold": true,
        "exit_code": 2,
        "alerts": [{ "threshold": 100, "type": "forecasted" }]
      }
    }
  }
}
```

Full schema, scoped examples, and validation errors:
[references/budget-config.md](references/budget-config.md).

## 2. See whether spend is inside the limit

```bash
finfocus cost projected --pulumi-json plan.json
finfocus cost actual --pulumi-json plan.json --from 2026-10-01
finfocus overview
```

Table output prints a `BUDGET STATUS` section. JSON and NDJSON cost documents
do not include that banner. Health bands:

- OK below 80%
- WARNING from 80% up to 90%
- CRITICAL from 90% up to 100%
- EXCEEDED at 100% and above

Several scopes roll up to the worst status.

Details: [references/budget-health.md](references/budget-health.md).

## 3. Fail CI only when asked

Exit non-zero only when `exit_on_threshold` is true and a threshold is crossed.
Precedence is the CLI flag, then `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` and
`FINFOCUS_BUDGET_EXIT_CODE`, then the scope, then `cost.budgets`, then the
default (`false`, exit code `1`).

```bash
finfocus cost projected --pulumi-json plan.json --exit-on-threshold --exit-code 2
```

`overview` accepts the same flags. Its `--exit-on-threshold` help text says the
exit applies in non-TTY output. Exit code `0` logs a warning and still exits 0.
A budget evaluation failure uses exit code 1.

Flags and the exit path: [references/budget-cli.md](references/budget-cli.md).

## 4. Fix the usual misses

- No banner: `cost.budgets.global.amount` is missing or `0`. Read the
  effective `config.hujson` (`finfocus config list`).
- Exit stays 0: `exit_on_threshold` is still false. Pass `--exit-on-threshold`
  or set the config or `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD`.
- `global budget is required`: a provider, tag, or type budget exists without
  `global.amount > 0`.
- Currency error: a scoped currency differs from the global currency. Omit it
  to inherit.
- Tag rows stay at `$0`: tag allocation is validated but not applied to cost
  results yet. The config warning says so.

Do not invent `FINFOCUS_BUDGET_AMOUNT` or `FINFOCUS_BUDGET_CURRENCY`. The
process does not read those variables.
