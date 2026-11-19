---
title: Automatic Cost Checks
description: Run the cost check when an agent finishes editing infrastructure code, or before a commit or push.
---

The `cost-check` agent skill ships a hook script,
`hooks/cost-check-hook.sh`, that prices a change on its own. It runs at the
end of a Claude Code turn, before a commit, or before a push, and reports the
cost change in at most five lines. When nothing in the infrastructure code
changed, it exits at once and runs nothing.

The Pulumi Analyzer policy pack already reports cost during `pulumi preview`.
The hook covers the moments before that: an agent handing its work back, and
a developer committing or pushing.

## Requirements

- `git`, `jq`, and `finfocus` on `PATH`, with a pricing plugin installed.
- For a Pulumi project: `pulumi`, logged in to the stack's backend, with the
  stack's passphrase and cloud credentials in the environment. The hook runs
  `pulumi preview`, never `pulumi up`, and never prompts.
- For a Terraform or OpenTofu project: a saved `tofu show -json` plan named by
  `hook.terraform_plan` in `.cost-check.yml`. The hook does not run
  `terraform plan`.

Pricing a change takes as long as `pulumi preview`: seconds to minutes. That is
why the Claude Code trigger is the `Stop` event, once per turn, and not a hook
on every edit.

## Install the skill

```bash
npx -y skills@1.7.0 add https://github.com/rshade/finfocus/tree/main/agent-skills \
  --skill cost-check --agent claude-code --copy -y
```

The script is then at `.claude/skills/cost-check/hooks/cost-check-hook.sh` in
the project.

## Claude Code

Add a `Stop` hook to `.claude/settings.json`:

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

`pulumi preview` executes the Pulumi program, and at the end of an agent turn
that program is code the agent just edited. A `Stop` hook runs outside the
agent's approval prompts, so by default the hook does not run the preview. It
tells you, once per change, that infrastructure code changed, and you run the
check yourself with `manual`. To let the hook price the change on its own,
opt in from your user settings, `~/.claude/settings.json`, which is outside
the repository the agent edits:

```json
{
  "env": {
    "FINFOCUS_COST_HOOK_RUN_PREVIEW": "1"
  }
}
```

With the opt-in, when the agent finishes a turn that changed infrastructure
code, you see the summary. A change the hook already checked is not checked
again. Terraform needs no opt-in: its saved plan is read, not executed.

A Claude Code `Stop` hook cannot add text to the agent's context without
continuing its turn. So by default only you see the summary. Set
`hook.block: true` to hand an exceeded threshold back to the agent: it then
continues and has to reduce the cost or explain why it is needed. The hook
never blocks a turn that it already continued, so it cannot loop.

## git

In `.git/hooks/pre-push` (or `pre-commit`), made executable:

```sh
#!/bin/sh
exec sh .claude/skills/cost-check/hooks/cost-check-hook.sh pre-push
```

`pre-commit` checks staged files. `pre-push` checks the files the pushed
commits change, and prices the working tree. With `hook.block: true`, an
exceeded threshold stops the commit or push with exit code `1`.

## Configuration

The hook reads the same `.cost-check.yml` as the skill:

```yaml
currency: USD
thresholds:
  resource: 100
  increase: 50
  increase_pct: 20
  total: 1000
hook:
  block: false
```

| Variable | Meaning |
| --- | --- |
| `FINFOCUS_COST_HOOK_STACK` | Stack to preview. Unset uses the selected stack. |
| `FINFOCUS_COST_HOOK_PROJECT_DIR` | Project directory when it is not the repository root. |
| `FINFOCUS_COST_HOOK_RUN_PREVIEW` | `1` lets the Claude Code hook run `pulumi preview` on code the agent edited. Set it in your user settings. |

## Output

```text
FinFocus cost check: +120.00 USD/mo, plan total 220.00 USD/mo (2 create, 0 update, 0 delete)
Status: over threshold: increase +120.00 > 50; increase 120% > 20% (1 unpriced, so totals are a lower bound)
  +87.15 create db (aws:rds/instance:Instance)
  +32.85 create nat (aws:ec2/natGateway:NatGateway)
```

The status names every exceeded threshold, plus `budget exceeded` when a
FinFocus budget with `exit_on_threshold` is over. The first line shows the
stack total when the stack's state was read, and the plan total otherwise.

The hook never reports a number it cannot trust:

- When nothing in the change has a price, the status says `not priced` and
  points to `finfocus plugin list`.
- When the current cost of an updated resource fails to price, the change
  reads `change unavailable` and the increase checks are skipped, so a pricing
  failure never blocks.
- For Terraform, a saved plan older than the changed `.tf` files is not
  priced; the hook asks for a new one.

The hook never blocks on a missing tool, plugin, or login, or on a failed
preview. It prints the reason instead.
