---
name: finfocus-cost
description: >
  Guide for using FinFocus CLI to analyze cloud infrastructure costs. Use when running
  projected or actual cost commands, interpreting cost output, configuring Pulumi
  integration, working with budgets and recommendations, filtering resources, or
  understanding cost data flows. Triggers on: "run cost analysis", "projected costs",
  "actual costs", "cost recommendations", "budget health", "Pulumi preview costs",
  "cross-provider aggregation", "cost filtering", or any task involving finfocus
  cost estimation and analysis workflows.
---

# FinFocus Cost Analysis

## Projected Costs

Estimate future costs from a Pulumi preview:

```bash
# From Pulumi JSON
finfocus cost projected --pulumi-json plan.json

# With output format
finfocus cost projected --pulumi-json plan.json --output json

# With specific adapter
finfocus cost projected --pulumi-json plan.json --adapter aws-public

# Auto-detect Pulumi project (searches for Pulumi.yaml)
finfocus cost projected
```

Generate Pulumi JSON: `pulumi preview --json > plan.json`

## Actual Costs

Fetch historical costs from cloud providers:

```bash
# Basic usage
finfocus cost actual --pulumi-json plan.json --from 2024-01-01

# Date range
finfocus cost actual --pulumi-json plan.json --from 2024-01-01 --to 2024-01-31

# With grouping
finfocus cost actual --pulumi-json plan.json --from 2024-01-01 --group-by daily
finfocus cost actual --pulumi-json plan.json --from 2024-01-01 --group-by monthly

# Cross-provider aggregation
finfocus cost actual --group-by daily --from 2024-01-01 --to 2024-01-31
```

Date formats: `2006-01-02` or RFC3339. `--to` defaults to now.

## Recommendations

Get cost optimization suggestions:

```bash
# All recommendations
finfocus cost recommendations --pulumi-json plan.json

# Filter by action type
finfocus cost recommendations --pulumi-json plan.json --action-type RIGHTSIZE,TERMINATE

# Dismiss/undismiss
finfocus cost recommendations dismiss <id>
finfocus cost recommendations undismiss <id>
finfocus cost recommendations history
```

Valid action types: RIGHTSIZE, TERMINATE, PURCHASE_COMMITMENT, ADJUST_REQUESTS,
MODIFY, DELETE_UNUSED, MIGRATE, CONSOLIDATE, SCHEDULE, REFACTOR, INVESTIGATE, OTHER.

## Budgets

Monitor cloud spending against budget limits:

```bash
# View budgets
finfocus cost budget

# Filter by provider
finfocus cost budget --filter "provider=kubecost"

# Filter by tags
finfocus cost budget --filter "tag:namespace=production"
finfocus cost budget --filter "tag:namespace=prod-*"  # Glob patterns

# Combined filters
finfocus cost budget --filter "provider=aws" --filter "tag:env=staging"
```

Health statuses: OK (<80%), WARNING (80-89%), CRITICAL (90-100%), EXCEEDED (>100%).

## Output Formats

Three formats via `--output`:

| Format | Flag | Use Case |
|--------|------|----------|
| Table | `--output table` | Human-readable (default) |
| JSON | `--output json` | Programmatic consumption |
| NDJSON | `--output ndjson` | Streaming large datasets |

## Pulumi Integration

See [references/cli-commands.md](references/cli-commands.md) for complete flag
reference and auto-detection behavior.

## Engine Internals

See [references/engine-api.md](references/engine-api.md) for the cost calculation
pipeline, aggregation types, and error handling.

## Resource Filtering

Filter patterns for `--filter`:

- `provider=aws` - By cloud provider
- `type=ec2` - By service type
- `service=rds` - By extracted service
- `instanceType=t3.micro` - By resource property
- `id=i-123` - By resource ID

## Zero-Click Cost Estimation

Configure in `Pulumi.yaml` for automatic cost estimation during `pulumi preview`:

```yaml
plugins:
  analyzers:
    - path: ./bin/finfocus
      args: ["analyzer", "serve"]
```

The analyzer runs as a gRPC server, receives resources from Pulumi, and returns
ADVISORY diagnostics (never blocks deployments).

## Debug Mode

```bash
# Enable verbose logging
finfocus cost projected --debug --pulumi-json plan.json

# Or via environment
export FINFOCUS_LOG_LEVEL=debug
export FINFOCUS_LOG_FORMAT=json
```
