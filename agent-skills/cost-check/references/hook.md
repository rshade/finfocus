<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# cost-check hook

`hooks/cost-check-hook.sh` runs the cost check on its own when infrastructure
code changed, so a cost increase shows up before a handoff, a commit, or a
push. It needs `git`, `jq`, and `finfocus` on `PATH`, plus `pulumi` (logged in,
with the stack's passphrase and cloud credentials in the environment) for a
Pulumi project.

## Triggers

| Argument | Runs | Checks |
| --- | --- | --- |
| `claude-stop` | Claude Code `Stop` hook, once at the end of each agent turn | Uncommitted changes |
| `pre-commit` | git `pre-commit` | Staged files |
| `pre-push` | git `pre-push` | Files changed by the pushed commits; prices the working tree |
| `manual` | By hand | Uncommitted changes |

It exits at once, with no output, when no infrastructure file changed: Pulumi
project files and program sources, or `*.tf`, `*.tf.json`, and `*.tfvars`.
Pricing a change runs `pulumi preview`, which takes seconds to minutes, so use
the `Stop` hook and not a hook on every edit. `claude-stop` also skips a change
it already checked.

## What it prints

At most five lines: the cost change and the total, a status line, and the
three largest changes.

```text
FinFocus cost check: +120.00 USD/mo, plan total 220.00 USD/mo (2 create, 0 update, 0 delete)
Status: over threshold: increase +120.00 > 50; increase 120% > 20% (1 unpriced, so totals are a lower bound)
  +87.15 create db (aws:rds/instance:Instance)
  +32.85 create nat (aws:ec2/natGateway:NatGateway)
```

The status lists every `.cost-check.yml` threshold that is exceeded, and
`budget exceeded` when a FinFocus budget with `exit_on_threshold` is over.
The total is the stack total when the state was read, and the plan total
otherwise; the `total` threshold always uses the plan total.

It follows the skill's validation rules:

- Nothing in the change has a price (`emptyPlan`, or every resource
  unpriced): the status says `not priced` and nothing is flagged.
- The current cost of an updated resource failed to price: its change and
  both increase checks are unavailable, never counted as an increase.
- Costs in another currency than `.cost-check.yml`: thresholds are not
  applied.
- Terraform: a saved plan older than the changed `.tf` files is not priced.

| Result | `claude-stop` | `pre-commit`, `pre-push`, `manual` |
| --- | --- | --- |
| Within thresholds, or over with `hook.block: false` | `systemMessage` with the summary, shown to the user; exit `0` | Summary; exit `0` |
| Over threshold with `hook.block: true` | `decision: "block"` with the summary as the reason: the agent continues and must address the cost. Never on a turn the hook already continued (`stop_hook_active`) | Summary; exit `1` |
| `finfocus`, `jq`, `pulumi`, a plugin, a login, or the config is missing or fails | `systemMessage` with the reason; exit `0` | Reason; exit `0` |

A Claude Code `Stop` hook cannot add text to the agent's context without
continuing its turn, so the agent sees the summary only when it blocks.

## Setup

Claude Code, in `.claude/settings.json` (use the path where the skill is
installed):

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sh \"${CLAUDE_PROJECT_DIR}/.claude/skills/cost-check/hooks/cost-check-hook.sh\" claude-stop",
            "timeout": 300
          }
        ]
      }
    ]
  }
}
```

git, in `.git/hooks/pre-push` (and the same with `pre-commit`), made
executable:

```sh
#!/bin/sh
exec sh /path/to/cost-check/hooks/cost-check-hook.sh pre-push
```

## Environment

| Variable | Meaning |
| --- | --- |
| `FINFOCUS_COST_HOOK_STACK` | Stack to preview. Unset uses the selected stack. |
| `FINFOCUS_COST_HOOK_PROJECT_DIR` | Project directory when it is not the repository root. `.cost-check.yml` is read from it. |
