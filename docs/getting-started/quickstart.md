---
layout: default
title: 5-Minute Quickstart
description: Get started with FinFocus in 5 minutes
---

Get FinFocus running and see your first cost estimate in just 5 minutes.

## Prerequisites

- A Pulumi project (local or existing)
- Terminal/command line access
- ~5 minutes of time

## Step 1: Install (1 minute)

### Option A: From source (recommended)

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
make build
export PATH="$PWD/bin:$PATH"
```

### Option B: Download binary (coming soon)

```bash
# Download latest release
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus
```

**Verify installation:**

```bash
finfocus --version
```

## Step 2: Run FinFocus (1 minute)

The simplest way: just run FinFocus inside your Pulumi project directory.
It auto-detects the project and runs `pulumi preview --json` for you:

```bash
cd your-pulumi-project
finfocus cost projected
```

Alternatively, you can provide the plan file explicitly:

```bash
pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json
```

**Output:**

```text
RESOURCE                          TYPE              MONTHLY   CURRENCY
aws:ec2/instance:Instance         aws:ec2:Instance  $7.50     USD
aws:s3/bucket:Bucket              aws:s3:Bucket     $0.00     USD
aws:rds/instance:Instance         aws:rds:Instance  $0.00     USD

Total: $7.50 USD
```

## Step 4: Try JSON Output (1 minute)

```bash
finfocus cost projected --pulumi-json plan.json --output json | jq .
```

**Output:**

```json
{
  "summary": {
    "totalMonthly": 7.5,
    "currency": "USD"
  },
  "resources": [
    {
      "type": "aws:ec2:Instance",
      "estimatedCost": 7.5,
      "currency": "USD"
    }
  ]
}
```

## Step 5: Try Filtering (1 minute)

```bash
# Show only EC2 resources
finfocus cost projected --pulumi-json plan.json --filter "type=aws:ec2*"

# Show only database resources
finfocus cost projected --pulumi-json plan.json --filter "type=aws:rds*"
```

## Step 6: Configure Scoped Budgets (Optional)

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
      gcp:
        amount: 2000.00
    tags:
      - selector: 'team:platform'
        priority: 100
        amount: 2000.00
    types:
      'aws:ec2/instance':
        amount: 1000.00
```

Then run with budget display:

```bash
finfocus cost projected --pulumi-json plan.json

# Filter by scope
finfocus cost projected --pulumi-json plan.json --budget-scope=provider
finfocus cost projected --pulumi-json plan.json --budget-scope=tag
finfocus cost projected --pulumi-json plan.json --budget-scope=type
```

---

## What's Next?

- **Learn more:** [User Guide](../guides/user-guide.md)
- **Installation details:** [Installation Guide](installation.md)
- **Setup with Vantage:** [Vantage Plugin Setup](../plugins/vantage/setup.md)
- **CLI reference:** [CLI Commands](../reference/cli-commands.md)
- **Examples:** [More Examples](examples/)

---

**Congratulations!** You've just run FinFocus! 🎉
