# Quickstart: Terraform Plan Ingestion

Validation guide for [spec.md](spec.md). Implementation steps live in
`tasks.md`, which `/speckit-tasks` writes. Contracts:

- [cli.md](contracts/cli.md)
- [plan-json.md](contracts/plan-json.md)
- [history-identity.md](contracts/history-identity.md)
- [web-ui.md](contracts/web-ui.md)

## Preconditions

- `make build` has produced `bin/finfocus`.
- A Pulumi project you already price, left unchanged.
- A Terraform directory that has been initialized and whose credentials the
  `terraform` or `tofu` on your PATH can already use. Do not point FinFocus
  at a backend it must log into.
- One real `tofu show -json` file kept outside the git project you are
  editing. Tests use a committed copy of that kind of file, not a hand-written
  document.

`finfocus --web` is included: it must detect this Terraform-compatible directory,
run OpenTofu, and show a populated overview with the same prices.

## 1. Pulumi still projects

From the Pulumi project, with no Terraform files in that directory:

```bash
bin/finfocus cost projected
```

Expected: the same preview path as before this feature. No Terraform binary
is started.

From a directory that contains both `Pulumi.yaml` and a `.tf` file:

```bash
bin/finfocus cost projected
```

Expected: Pulumi is selected.

## 2. Terraform projects with no file

From the initialized Terraform directory, whose parent is not a nearer
Pulumi project:

```bash
bin/finfocus cost projected
```

Expected: a projected total, exit 0. The project directory gains no plan
file. A second run with `tofu` only, after `terraform` is removed from
PATH, still projects.

Repeat with `bin/finfocus`, `bin/finfocus overview`,
`bin/finfocus cost forecast`, `bin/finfocus cost estimate`, and
`bin/finfocus cost recommendations`. Each plans. Overview does not ask
whether to skip the plan.

A plan that proposes no changes prints `No changes. Projected monthly cost
is zero.` and exits 0.

## 3. A saved plan file

```bash
bin/finfocus cost projected --terraform-plan /path/to/plan.json
```

Expected: a projection, and no `terraform` or `tofu` process. Combining
that flag with `--pulumi-json`, `--pulumi-state`, or `--terraform-state`
exits 2 and does not run a binary.

## 4. Hints

Each case exits non-zero, prints one hint, and prints no cost total.

| Setup | Hint contains |
| --- | --- |
| OpenTofu missing from PATH | `on PATH` |
| `.tf` files and no init | `does not run init` |
| Credentials or the backend rejected by the tool | `does not log in` |
| Any other plan error, or a binary older than OpenTofu 1.6.0 | `OpenTofu plan failed.` |

Interrupt a running plan. The command returns, and the temp plan file is
gone. Provider processes started for that plan are gone on Unix.

## 5. Actual cost and state

```bash
bin/finfocus cost actual --terraform-state /path/to/terraform.tfstate --from 2026-01-01
```

Expected: the state is priced as it is today. No plan runs.

```bash
bin/finfocus cost actual
```

From the Terraform directory. Expected: exit 2, and the text tells you to
pass `--terraform-state`.

## 6. History identity

From two clones with the same `origin` and the same path relative to the
git root:

```bash
bin/finfocus cost projected
bin/finfocus cost history list
```

Expected: both clones list the same `<hash>@unscoped` database. The stored
label is the relative path.

```bash
bin/finfocus cost history list --include-workspace
```

Expected: a different database whose stack part is the workspace name.
`--include-workspace` inside a Pulumi project does not change that
project's history file.

```bash
bin/finfocus cost history collect
```

From the Terraform directory. Expected: exit 2, and the text says collect
reads Pulumi checkpoints.

## 7. Automated checks

```bash
make test
make lint
make validate
```

Parser tests build a `ParseRequest` from a committed `tofu show -json`
fixture, call `Parse`, and require `pluginsdk.ValidateParseResponse` to
accept the response. They assert the operation map, skipped data sources,
and unknown paths. Runner tests use a fake binary on PATH. They do not
call a cloud API. Detection tests cover the nearer-marker rules. History
tests use a temporary git repository.

## Validation record

See [validation.md](validation.md) for the executed checks, coverage,
credential-free smoke results, and environment-dependent checks not run.
