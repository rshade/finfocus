# Contract: CLI

Commands and flags for Terraform plan mode. Pulumi flags keep their current
meaning. Exit 2 is `ax.ExitValidation`. Exit 1 is the operational failure
used today when `pulumi preview` fails.

## Flags

| Flag | Commands | Meaning |
| --- | --- | --- |
| `--terraform-plan` | `cost projected`, `cost forecast`, `cost estimate`, `cost recommendations`, `overview` | Read this `tofu show -json` file. Do not run a binary. Wins over detection |
| `--include-workspace` | The commands above, plus `cost history` and its subcommands | For this run, put the Terraform workspace in the history identity. Does not write config. Ignored when the detected project is Pulumi |
| `--terraform-state` | Unchanged | Price a state file. Do not plan |

`--terraform-plan`, `--terraform-state`, `--pulumi-json`, and
`--pulumi-state` are mutually exclusive on every command that has more than
one of them. A command that does not grow `--terraform-plan` (`cost actual`)
rejects that flag as unknown.

`cost.terraform.include_workspace` is a boolean, default false, in the
merged config. The flag wins over it for one run.

## Resolution

One function decides the projected input, in this order:

1. `--terraform-state`: existing state loader.
2. `--pulumi-json`: existing Pulumi plan loader.
3. `--pulumi-state`: existing state export loader, on commands that have it.
4. `--terraform-plan`: parse that file.
5. Otherwise detect the project. Pulumi runs the preview or export that
   command runs today. Terraform runs a plan, except `cost actual`.

`cost forecast` already calls `loadProjectedResources`. It keeps doing that
after the new arm exists.

## Command behavior

| Command | Terraform directory, no input file |
| --- | --- |
| `finfocus` | Delegate to overview, as it does for a Pulumi project |
| `finfocus --web` | Run OpenTofu and serve the existing dashboard and query endpoints |
| `overview` | Always run a plan. Do not prompt and do not compare file times |
| `cost projected`, `cost forecast`, `cost estimate`, `cost recommendations` | Run a plan and price it |
| `cost recommendations dismiss`, `cost recommendations snooze` | Unchanged. No plan: `--pulumi-json` stays optional and only decides whether plugins open |
| `cost actual` | Do not plan. Tell the person to pass `--terraform-state` |
| `cost history view`, `list`, `diff`, `export`, `prune` | Open the Terraform identity. Do not plan |
| `cost history collect` | Do not plan. Say that checkpoint collection is for a Pulumi project |

Estimate `--modify` keeps the current selector rules (ID, logical name,
type). A Terraform address is the resource ID. No new selector syntax.

A plan with no creates, updates, deletes, or replaces prints an
informational line and a zero projected total, and exits 0. The line is
`No changes. Projected monthly cost is zero.`

## Hints

A resource from a skipped provider (see [plan-json.md](plan-json.md)) prints one
stderr note per provider, for example `Skipped 3 helm resources: no cost.`, and never changes stdout or
the exit code.

A `--terraform-plan` file over 32 MiB is a user-input error (exit 2) that
names the limit.

Operational failures print exactly one of these and no cost total. The last
20 stderr lines are appended only to the plan-failed hint. Stdout of
`show -json` is never printed and never logged.

| Kind | Text |
| --- | --- |
| Binary missing | `OpenTofu plan needs the tofu command on PATH. Install OpenTofu, then run this command again.` |
| Not initialized | `This directory is not initialized. Run tofu init here, then run this command again. FinFocus does not run init.` |
| Credentials or backend | `OpenTofu could not read credentials or the backend. Fix the access OpenTofu uses for this directory and run this command again. FinFocus does not log in or store credentials.` |
| Plan failed | `OpenTofu plan failed.` plus the last 20 stderr lines. A binary older than OpenTofu 1.6.0 names that minimum instead of a stack trace |

`cost actual` in a Terraform directory with no input file prints:
`This directory is Terraform. Pass --terraform-state with a state file. FinFocus does not run a plan for actual cost.`
That is a user-input error, exit 2.

`cost history collect` in a Terraform directory prints:
`cost history collect reads Pulumi checkpoints. This directory is Terraform, and a plan has no checkpoint list.`
Exit 2.

Neither marker, on a command that detects a project, prints an error that
contains both `Pulumi` and `Terraform`. Exit 2.

## Pulumi non-change

For a directory whose winning marker is Pulumi:

- The preview, export, required flags, and defaults are unchanged.
- `--include-workspace` does not change the Pulumi history key.
- `--terraform-state` still prices state and does not preview.
- ESC loading and `analyzer` install still search only for a Pulumi project.
