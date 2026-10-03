# Budget CLI

## Flags

`finfocus cost` persistent flags apply to `projected`, `actual`,
`recommendations`, and `estimate`:

| Flag | Default | Effect |
| --- | --- | --- |
| `--exit-on-threshold` | `false` | Non-zero exit when a configured threshold is exceeded |
| `--exit-code` | `1` | Exit code used when that happens, range 0-255 |
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

1. Changed `--exit-on-threshold` / `--exit-code` write the global scope.
2. Otherwise `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` and
   `FINFOCUS_BUDGET_EXIT_CODE` do, when set and parseable.
3. Otherwise the scope's own fields, then `cost.budgets.exit_on_threshold`
   and `cost.budgets.exit_code`.
4. Default is off, with exit code 1.

`checkBudgetExit` returns a `BudgetExitError` only when exit is enabled and a
threshold is exceeded. `toAxExitError` wraps that error so `ax.Execute` keeps
the configured code. Exit code 0 prints `WARNING:` and does not fail the
process. Evaluation failure uses exit code 1
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

`config set` stores one dotted key in `~/.finfocus/config.hujson`. Nested
budget objects are clearer when edited in the file than when set one leaf at
a time.

## CI

```yaml
- name: Budget gate
  run: |
    finfocus cost projected --pulumi-json plan.json \
      --exit-on-threshold --exit-code 2
```

The command exits 2 when the configured threshold is exceeded and the flag is
set. It exits 0 when the threshold is not exceeded, or when
`exit_on_threshold` is false.
