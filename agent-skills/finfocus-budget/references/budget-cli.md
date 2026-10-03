# Budget CLI

## Flags

`finfocus cost` persistent flags apply to `projected`, `actual`,
`recommendations`, and `estimate`:

| Flag | Default | Effect |
| --- | --- | --- |
| `--exit-on-threshold` | `false` | Overrides `cost.budgets.global.exit_on_threshold` for this command |
| `--exit-code` | `1` | Overrides `cost.budgets.global.exit_code` for this command, range 0-255 |
| `--budget-scope` | empty (all scopes) | Which budget sections to print |

`finfocus overview` has the same three flags. `--exit-on-threshold` on
overview is documented as applying to non-TTY output.

`--budget-scope` is a comma-separated list:

| Value | Shows |
| --- | --- |
| `global` | Global section |
| `provider` | Every provider |
| `provider=aws` | That provider only |
| `tag` | Every tag budget |
| `tag=team:platform` | That selector only |
| `type` | Every resource-type budget |
| `type=aws:ec2/instance:Instance` | That type only |

```bash
finfocus cost projected --pulumi-json plan.json --budget-scope=provider=aws,global
finfocus cost projected --pulumi-json plan.json --exit-on-threshold --exit-code 2
finfocus overview --exit-on-threshold --exit-code 2
```

An empty `--budget-scope` shows every scope.

## Exit path

`--exit-on-threshold` and `--exit-code` are overrides on the command's context:
they apply to a copy of the global scope for that one run and write nothing to the
config, and they win over the environment. `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD`
and `FINFOCUS_BUDGET_EXIT_CODE` set `cost.budgets.global` for the process.
Neither the flags nor the variables reach a provider, tag, or type scope.

`checkBudgetExit` runs when no provider, tag, or type budget is configured. It
reads the global scope, which is where those overrides and variables apply, then
the parent `cost.budgets` fields. Default is off, with exit code 1. It returns
a `BudgetExitError` when exit is enabled and a configured alert is exceeded,
including a `forecasted` alert. Exit code 0 prints `WARNING:` and does not
fail the process.

`checkScopedBudgetExit` runs when any provider, tag, or type budget is
configured, and only after overall health is CRITICAL or EXCEEDED. Each
breached scope uses its own `exit_on_threshold` when that pointer is set, and
otherwise `cost.budgets.exit_on_threshold`. A nil scope override reads `cost.budgets.exit_on_threshold`. A nil exit-code override reads `cost.budgets.exit_code`. The scoped gate uses actual-spend health: CRITICAL
from 90% up to 100%, EXCEEDED at 100% and above. A scoped gate can fail at 90% actual utilization. A forecasted alert does not open it.

`toAxExitError` wraps `BudgetExitError` so `ax.Execute` keeps the configured
code. Evaluation failure uses exit code 1
(`ExitCodeBudgetEvaluationError`), which is the same number as a generic
internal error. Codes 2, 3, and 4 are also ax-go's validation, network, and
auth codes, so pick a budget code with that overlap in mind.

## Config commands

```bash
finfocus config init            # project file, or global when no Pulumi.yaml
finfocus config init --global
finfocus config list
finfocus config get <key>
finfocus config set <key> <value>
```

`config set` stores one dotted key in `~/.finfocus/config.hujson`. For
budgets it accepts `cost.budgets.amount`, `cost.budgets.currency`, and
`cost.budgets.period`, and those write the global scope.
`cost.budgets.global.amount` and other nested budget keys return
`invalid cost.budgets key`. `cost.budgets.alerts` returns
`cost.budgets.alerts must be configured via YAML`. Edit `config.hujson` for
nested budgets.

## CI

```yaml
- name: Budget gate
  run: |
    finfocus cost projected --pulumi-json plan.json \
      --exit-on-threshold --exit-code 2
```

With only a global budget, the command exits 2 when a configured alert is
exceeded and the flag is set. It exits 0 when that alert is not exceeded, or
when `cost.budgets.global.exit_on_threshold` is false. A provider, tag, or
type gate still reads its own `exit_on_threshold` or
`cost.budgets.exit_on_threshold`.
