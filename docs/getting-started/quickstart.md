---
layout: default
title: 5-Minute Quickstart
description: Get started with FinFocus in 5 minutes
---

Get FinFocus running and see your first cost dashboard in just 5 minutes.

## Prerequisites

- A Pulumi project (local or existing)
- Terminal/command line access
- ~5 minutes of time

## Step 1: Install (1 minute)

### Option A: Install script (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

### Option B: From source

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
make build
export PATH="$PWD/bin:$PATH"
```

**Verify installation:**

```bash
finfocus --version
```

## Step 2: Launch the Overview (1 minute)

Navigate into any Pulumi project directory and run finfocus with no arguments:

```bash
cd your-pulumi-project
finfocus
```

FinFocus auto-detects your project and stack, exports the current state, and
runs `pulumi preview --json` — then opens an interactive cost dashboard:

```text
Resource                          Type                    Status  Actual(MTD)  Projected   Recs
my-instance                       aws:ec2/instance:I...   ✓       $12.40       $15.00      2
my-bucket                         aws:s3/bucket:Bucket    ✓       $0.83        $1.00       0
my-db                             aws:rds/instance:I...   ✓       $48.20       $50.00      1

Total Actual (MTD): $61.43    Projected Monthly: $66.00    Potential Savings: $45.00
```

Press `Enter` on any resource to drill into its cost breakdown and recommendations.
Press `q` to quit.

> **Tip:** `finfocus overview` and `finfocus ov` are aliases for the same command.

## Step 3: Non-interactive Output (1 minute)

For scripts or CI/CD, use `--plain --yes` to skip the TUI and prompts:

```bash
# Plain text table
finfocus overview --plain --yes

# JSON for scripting
finfocus overview --output json --yes | jq .summary

# Projected costs only (no state required)
finfocus cost projected
```

## Step 4: Filter by Provider or Type (1 minute)

```bash
# Only AWS resources
finfocus overview --filter provider=aws --plain --yes

# Only EC2 instances
finfocus overview --filter type=aws:ec2/instance:Instance --plain --yes
```

## Step 5: Set Up Budgets (Optional)

Create `~/.finfocus/config.yaml` with hierarchical budgets:

```yaml
cost:
  scoped_budgets:
    global:
      amount: 5000.00
      currency: USD
      period: monthly
    providers:
      aws:
        amount: 3000.00
    tags:
      - selector: 'team:platform'
        priority: 100
        amount: 2000.00
```

Budget status appears automatically in the overview. For projected costs:

```bash
finfocus cost projected --budget-scope=provider
finfocus cost projected --budget-scope=provider=aws
```

---

## What's Next?

- **Full overview docs:** [Overview Command](../commands/overview.md)
- **Analyzer setup:** [Pulumi Analyzer Setup](analyzer-setup.md) — see costs
  inline during `pulumi preview`
- **User Guide:** [User Guide](../guides/user-guide.md)
- **CLI reference:** [CLI Commands](../reference/cli-commands.md)
- **Installation details:** [Installation Guide](installation.md)
