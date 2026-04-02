---
layout: default
title: Core Concepts
description: Key concepts and terminology used in FinFocus
---

This page explains the fundamental concepts and terminology used throughout FinFocus.

## Table of Contents

- [Resource Descriptors](#resource-descriptors)
- [Cost Types](#cost-types)
  - [Projected Costs](#projected-costs)
  - [Actual Costs](#actual-costs)
- [Plugin Architecture](#plugin-architecture)
  - [How Plugins Work](#how-plugins-work)
  - [Plugin Types](#plugin-types)
  - [Plugin Selection](#plugin-selection)
- [Cost Calculation Flow](#cost-calculation-flow)
- [Aggregation and Grouping](#aggregation-and-grouping)
- [Budgets and Thresholds](#budgets-and-thresholds)
- [Output Formats](#output-formats)
- [Related Documentation](#related-documentation)

## Resource Descriptors

A **Resource Descriptor** is the internal representation of a cloud resource that FinFocus uses
for cost calculations. It contains:

- **Provider**: The cloud provider (e.g., `aws`, `azure`, `gcp`)
- **Type**: The resource type in Pulumi format (e.g., `aws:ec2/instance:Instance`)
- **SKU**: The specific SKU or instance type (e.g., `t3.xlarge`)
- **Region**: The deployment region (e.g., `us-east-1`)
- **Inputs**: Key-value pairs of resource properties from Pulumi state

### Example Resource Descriptor

```json
{
  "provider": "aws",
  "type": "aws:ec2/instance:Instance",
  "sku": "t3.xlarge",
  "region": "us-east-1",
  "inputs": {
    "instanceType": "t3.xlarge",
    "availabilityZone": "us-east-1a"
  }
}
```

## Cost Types

FinFocus supports two types of cost calculations:

### Projected Costs

**Projected costs** are estimates based on infrastructure definitions before deployment.
They answer the question: "How much will this infrastructure cost?"

- Calculated from `pulumi preview --json` output
- Uses public pricing APIs or local pricing specifications
- Ideal for budget planning and cost estimation

### Actual Costs

**Actual costs** are real historical spending data from cloud providers.
They answer the question: "How much did this infrastructure actually cost?"

- Retrieved from cost management APIs (AWS Cost Explorer, Azure Cost Management)
- Supports time ranges and grouping options
- Ideal for cost tracking and analysis

## Plugin Architecture

FinFocus uses a plugin-based architecture to support multiple cost data sources.

### How Plugins Work

1. **Discovery**: FinFocus scans `~/.finfocus/plugins/` for installed plugins
2. **Launch**: When a resource matches a plugin's provider, the plugin is started
3. **Communication**: FinFocus communicates with plugins via gRPC
4. **Fallback**: If no plugin handles a resource, local YAML specs are used

### Plugin Types

| Type         | Description                            | Example                       |
| ------------ | -------------------------------------- | ----------------------------- |
| Cost Plugins | Query cloud provider pricing APIs      | AWS Public, AWS Cost Explorer |
| Spec Files   | Local YAML/JSON pricing specifications | Custom pricing data           |

### Plugin Selection

For each resource, FinFocus:

1. Identifies the provider from the resource type
2. Checks if a plugin supports that provider
3. Sends the resource descriptor to the plugin
4. Receives cost data or falls back to local specs

## Cost Calculation Flow

```text
┌─────────────────┐
│ Pulumi JSON     │
│ (preview/state) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Ingestion       │
│ (Parse JSON)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Resource        │
│ Descriptors     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Engine          │◄───── Plugins (gRPC)
│ (Orchestrate)   │◄───── Local Specs (YAML)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Cost Results    │
│ (Aggregate)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Output          │
│ (Table/JSON)    │
└─────────────────┘
```

## Aggregation and Grouping

FinFocus supports various ways to aggregate and group cost data:

### Group By Options

| Option     | Description                      |
| ---------- | -------------------------------- |
| `resource` | Group by individual resource     |
| `type`     | Group by resource type           |
| `provider` | Group by cloud provider          |
| `date`     | Group by date (for actual costs) |

### Example: Grouping by Type

```bash
finfocus cost projected --pulumi-json plan.json --group-by type
```

Output:

```text
TYPE                          MONTHLY   CURRENCY
aws:ec2/instance:Instance     $750.00   USD
aws:s3/bucket:Bucket          $5.00     USD
aws:rds/instance:Instance     $200.00   USD
```

## Budgets and Thresholds

FinFocus supports budget enforcement with configurable thresholds.

### Budget Configuration

Budgets are configured in `~/.finfocus/config.yaml`:

```yaml
cost:
  budgets:
    amount: 500.00
    currency: USD
    alerts:
      - threshold: 80
        type: actual
      - threshold: 100
        type: forecasted
```

### Budget Scopes

Budgets can be scoped to specific resources:

| Scope                  | Description       | Example                 |
| ---------------------- | ----------------- | ----------------------- |
| `global`               | All resources     | Default                 |
| `provider:<name>`      | By cloud provider | `provider:aws`          |
| `tag:<key>=<value>`    | By resource tag   | `tag:env=prod`          |
| `type:<resource-type>` | By resource type  | `type:aws:ec2/instance` |

## Output Formats

FinFocus supports multiple output formats:

| Format   | Use Case                                 |
| -------- | ---------------------------------------- |
| `table`  | Human-readable terminal output (default) |
| `json`   | API integration and programmatic access  |
| `ndjson` | Streaming and pipeline processing        |

## Related Documentation

- [System Overview](system-overview.md) - High-level architecture
- [Plugin Protocol](plugin-protocol.md) - gRPC plugin specification
- [Cost Calculation](cost-calculation.md) - Detailed calculation logic
- [CLI Commands](../reference/cli-commands.md) - Command reference
