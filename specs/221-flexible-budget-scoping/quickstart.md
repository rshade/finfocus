# Quickstart: Flexible Budget Scoping

**Feature Branch**: `221-flexible-budget-scoping`
**Time to Complete**: 5 minutes

## Prerequisites

- FinFocus CLI installed (`finfocus` binary in PATH)
- At least one cost source plugin configured
- A Pulumi project with resources

## Step 1: Configure Scoped Budgets

Edit your configuration file at `~/.finfocus/config.yaml`:

```yaml
cost:
  budgets:
    # Global budget applies to all resources
    global:
      amount: 5000.00
      currency: USD
      period: monthly
      alerts:
        - threshold: 80.0
          type: actual
        - threshold: 100.0
          type: forecasted

    # Optional: Per-provider budgets
    providers:
      aws:
        amount: 3000.00
      gcp:
        amount: 2000.00

    # Optional: Tag-based budgets with priority
    tags:
      - selector: "team:platform"
        priority: 100
        amount: 1500.00
      - selector: "env:prod"
        priority: 50
        amount: 3000.00

    # Optional: Resource type budgets
    types:
      "aws:ec2/instance":
        amount: 500.00

    # Exit with code 1 if any threshold is exceeded
    exit_on_threshold: true
    exit_code: 1
```

## Step 2: View Budget Status

Run the budget command to see all scoped budgets:

```bash
finfocus cost budget
```

**Example Output**:

```text
BUDGET STATUS
─────────────────────────────────────────────────────────────────

GLOBAL
  Budget: $5,000.00 | Spend: $3,250.00 (65%) | Status: OK
  ████████████████████░░░░░░░░░░

BY PROVIDER
  aws      Budget: $3,000.00 | Spend: $2,100.00 (70%)  | Status: OK
  gcp      Budget: $2,000.00 | Spend: $1,150.00 (58%)  | Status: OK

BY TAG
  team:platform  Budget: $1,500.00 | Spend: $1,200.00 (80%)  | Status: WARNING
  env:prod       Budget: $3,000.00 | Spend: $2,500.00 (83%)  | Status: WARNING

BY TYPE
  aws:ec2/instance  Budget: $500.00 | Spend: $450.00 (90%)  | Status: CRITICAL

─────────────────────────────────────────────────────────────────
```

## Step 3: Filter by Scope

Use the `--budget-scope` flag to filter output:

```bash
# View only provider budgets
finfocus cost budget --budget-scope=provider

# View a specific tag budget
finfocus cost budget --budget-scope="tag=team:platform"

# View resource type budgets
finfocus cost budget --budget-scope=type
```

## Step 4: Enable Debug Logging

See which scopes each resource matches:

```bash
finfocus cost budget --debug
```

**Debug Output Example**:

```text
DEBUG | component=budget | resource=i-1234567890abcdef0 |
        type=aws:ec2/instance | matched_scopes=["global","provider:aws",
        "tag:team:platform","type:aws:ec2/instance"]
```

## Common Use Cases

### Multi-Cloud Organization

```yaml
providers:
  aws:
    amount: 10000.00
  gcp:
    amount: 5000.00
  azure:
    amount: 3000.00
```

### Team-Based Accountability

```yaml
tags:
  - selector: "team:backend"
    priority: 100
    amount: 4000.00
  - selector: "team:frontend"
    priority: 100
    amount: 2000.00
  - selector: "team:data"
    priority: 100
    amount: 6000.00
```

### Environment Separation

```yaml
tags:
  - selector: "env:prod"
    priority: 50
    amount: 8000.00
  - selector: "env:staging"
    priority: 50
    amount: 2000.00
  - selector: "env:dev"
    priority: 50
    amount: 500.00
```

### High-Cost Service Control

```yaml
types:
  "aws:rds/instance":
    amount: 2000.00
    alerts:
      - threshold: 70.0
        type: actual
  "aws:eks/cluster":
    amount: 3000.00
```

## CI/CD Integration

Add budget checks to your pipeline:

```yaml
# .github/workflows/cost-check.yml
- name: Check Budget Status
  run: |
    finfocus cost budget --format=json > budget-status.json
    exit_code=$?
    if [ $exit_code -ne 0 ]; then
      echo "Budget threshold exceeded!"
      cat budget-status.json | jq '.critical_scopes'
    fi
```

## Troubleshooting

### "overlapping tag budgets without priority"

Add explicit `priority` values to tag budgets:

```yaml
tags:
  - selector: "team:platform"
    priority: 100  # Higher = higher priority
    amount: 1500.00
  - selector: "env:prod"
    priority: 50   # Lower priority
    amount: 3000.00
```

### "currency mismatch" Error

All scoped budgets must use the same currency as global:

```yaml
global:
  currency: USD
  amount: 5000.00

providers:
  aws:
    # currency: EUR  # ERROR: Must match global (USD)
    amount: 3000.00  # OK: Inherits USD from global
```

### Resources Not Matching Expected Scope

Enable debug logging to see scope matching:

```bash
finfocus cost budget --debug 2>&1 | grep "matched_scopes"
```

## Next Steps

- Read the full [Budget Scoping Guide](../../docs/guides/user/budget-scoping.md)
- Configure [Alert Notifications](../../docs/reference/configuration.md#alerts)
- Set up [CI/CD Exit Codes](../../docs/deployment/ci-cd.md#budget-checks)
