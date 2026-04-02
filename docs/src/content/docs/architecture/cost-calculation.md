---
layout: default
title: Cost Calculation Algorithms
description: Detailed cost calculation algorithms, aggregation logic, and examples
---

This document explains how FinFocus calculates costs, including projected
cost estimation, actual cost retrieval, aggregation algorithms, and
cross-provider cost analysis.

## Overview

FinFocus supports two types of cost calculations:

1. **Projected Costs** - Estimate future infrastructure costs from Pulumi
   plans
2. **Actual Costs** - Retrieve historical costs from cloud providers via
   plugins

See [Cost Calculation Flow Diagram](diagrams/cost-calculation-flow.md) for
a visual representation of the calculation pipeline.

## Projected Cost Calculation

### Formula

```text
Monthly Cost = Unit Price × Hours Per Month

Where:
  Unit Price = From plugin or local spec ($/hour)
  Hours Per Month = 730 (constant)
```

### Calculation Steps

1. **Resource Ingestion** - Parse Pulumi plan JSON to extract resources
2. **Plugin Query** - Query plugin for unit price via `GetProjectedCost()`
3. **Monthly Calculation** - Multiply unit price by 730 hours
4. **Aggregation** - Sum costs across all resources
5. **Output** - Format as table, JSON, or NDJSON

### Example: AWS EC2 Instance

**Resource:**

```json
{
  "provider": "aws",
  "resource_type": "ec2:Instance",
  "sku": "t3.micro",
  "region": "us-east-1",
  "tags": {
    "env": "prod"
  }
}
```

**Plugin Response:**

```json
{
  "unit_price": 0.0104,
  "currency": "USD",
  "cost_per_month": 7.592,
  "billing_detail": "on-demand"
}
```

**Calculation:**

```text
Monthly Cost = 0.0104 $/hour × 730 hours/month = 7.592 $/month
```

### Hours Per Month Constant

The standard month is defined as 730 hours:

```text
730 hours = 365.25 days/year ÷ 12 months × 24 hours/day
          ≈ 30.4375 days/month × 24 hours/day
```

This constant is used consistently across all projected cost calculations
to ensure comparability.

## Actual Cost Calculation

### Retrieval Process

1. **Time Range** - User specifies start and end dates
2. **Filtering** - Optional tag-based filtering (`tag:key=value`)
3. **Plugin Query** - Query plugin for actual costs via `GetActualCost()`
4. **Aggregation** - Sum daily/monthly costs
5. **Grouping** - Optional grouping by resource, type, provider, or date

### Example: Actual Cost Query

**Request:**

```bash
finfocus cost actual \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --filter "tag:env=prod"
```

**Plugin API Call:**

```json
{
  "resource_id": "aws-prod-*",
  "start": "2024-01-01T00:00:00Z",
  "end": "2024-01-31T23:59:59Z",
  "tags": {
    "env": "prod"
  }
}
```

**Plugin Response:**

```json
{
  "results": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "cost": 73.42,
      "usage_amount": 730.0,
      "usage_unit": "hour",
      "source": "kubecost"
    },
    {
      "timestamp": "2024-01-02T00:00:00Z",
      "cost": 74.18,
      "usage_amount": 730.0,
      "usage_unit": "hour",
      "source": "kubecost"
    }
  ]
}
```

**Aggregation:**

```text
Total Cost = 73.42 + 74.18 + ... (31 days) = 2,287.64 USD
```

## Aggregation Algorithms

### By Resource

Sum costs for each individual resource:

```text
For each unique resource_id:
  Total[resource_id] = Σ cost where resource matches resource_id
```

**Example Output:**

```text
aws-ec2-i-123: $1,234.56
aws-ec2-i-456: $1,053.08
Total: $2,287.64
```

### By Type

Sum costs for each resource type:

```text
For each unique resource_type:
  Total[type] = Σ cost where resource matches type
```

**Example Output:**

```text
ec2: $1,834.22
s3: $312.48
rds: $140.94
Total: $2,287.64
```

### By Provider

Sum costs for each cloud provider:

```text
For each unique provider:
  Total[provider] = Σ cost where resource matches provider
```

**Example Output:**

```text
aws: $1,987.23
azure: $200.19
gcp: $100.22
Total: $2,287.64
```

### By Date (Daily)

Sum costs for each day:

```text
For each date in [start, end]:
  Total[date] = Σ cost where timestamp.Date == date
```

**Example Output:**

```text
2024-01-01: $73.42
2024-01-02: $74.18
2024-01-03: $73.89
...
Total: $2,287.64
```

### By Date (Monthly)

Sum costs for each month:

```text
For each month in [start, end]:
  Total[month] = Σ cost where timestamp.Month == month
```

**Example Output:**

```text
2024-01: $2,287.64
2024-02: $2,102.34
Total: $4,389.98
```

## Cross-Provider Aggregation

### Algorithm

The `CreateCrossProviderAggregation()` method aggregates costs across
multiple cloud providers with validation and normalization.

**Input:**

```go
type ActualCostResult struct {
    ResourceID   string
    Provider     string
    TotalCost    float64
    Currency     string
    StartDate    time.Time
    EndDate      time.Time
    GroupBy      GroupByType
}
```

**Validation Steps:**

1. **Empty Check** - Ensure results array is not empty
2. **Currency Validation** - Ensure all results use same currency
3. **Date Range Validation** - Ensure EndDate > StartDate
4. **GroupBy Validation** - Ensure time-based grouping (daily or monthly)

**Aggregation Steps:**

1. **Provider Extraction** - Extract provider from resource type
   (`aws:ec2:Instance` → `aws`)
2. **Cost Calculation** - Convert costs to daily/monthly based on time
   period
3. **Grouping** - Aggregate by date dimension
4. **Sorting** - Sort chronologically for trend analysis

### Currency Validation

```go
func validateCurrency(results []ActualCostResult) error {
    currency := ""
    for _, r := range results {
        if currency == "" {
            currency = r.Currency
        } else if r.Currency != currency {
            return ErrMixedCurrencies
        }
    }
    return nil
}
```

**Error Handling:**

- `ErrMixedCurrencies` - Different currencies detected (USD vs EUR)
- `ErrInvalidGroupBy` - Non-time-based grouping attempted
- `ErrInvalidDateRange` - EndDate before StartDate

### Cost Period Conversion

When actual costs span different time periods, costs are normalized:

**Daily Costs:**

```text
If period > 1 day:
  Daily Cost = Total Cost ÷ Days in Period
Else:
  Daily Cost = Total Cost
```

**Monthly Costs:**

```text
If period > 1 month:
  Monthly Cost = Total Cost ÷ Months in Period
Else if period < 1 month:
  Monthly Cost = Total Cost × (30.4375 ÷ Days in Period)
Else:
  Monthly Cost = Total Cost
```

### Example: Cross-Provider Aggregation

**Input (3 providers):**

```json
[
  {
    "resource_id": "aws-ec2-i-123",
    "provider": "aws",
    "total_cost": 1500.0,
    "currency": "USD",
    "start_date": "2024-01-01",
    "end_date": "2024-01-31"
  },
  {
    "resource_id": "azure-vm-456",
    "provider": "azure",
    "total_cost": 500.0,
    "currency": "USD",
    "start_date": "2024-01-01",
    "end_date": "2024-01-31"
  },
  {
    "resource_id": "gcp-instance-789",
    "provider": "gcp",
    "total_cost": 287.64,
    "currency": "USD",
    "start_date": "2024-01-01",
    "end_date": "2024-01-31"
  }
]
```

**Output (Daily Aggregation):**

```json
[
  {
    "date": "2024-01-01",
    "total_cost": 73.47,
    "currency": "USD",
    "breakdown": {
      "aws": 48.39,
      "azure": 16.13,
      "gcp": 9.28
    }
  },
  {
    "date": "2024-01-02",
    "total_cost": 73.47,
    "currency": "USD",
    "breakdown": {
      "aws": 48.39,
      "azure": 16.13,
      "gcp": 9.28
    }
  }
]
```

## Cost Flow Through System

### Projected Cost Flow

```text
1. Pulumi Plan JSON
   ↓
2. Resource Descriptors
   provider: aws, type: ec2, sku: t3.micro, region: us-east-1
   ↓
3. Plugin Query (GetProjectedCost)
   ↓
4. Plugin Response
   unit_price: 0.0104, currency: USD
   ↓
5. Monthly Calculation
   0.0104 × 730 = 7.592 USD/month
   ↓
6. Aggregation
   Sum all resources
   ↓
7. Output Formatting
   Table/JSON/NDJSON
```

### Actual Cost Flow

```text
1. User Request
   start: 2024-01-01, end: 2024-01-31, filter: tag:env=prod
   ↓
2. Plugin Query (GetActualCost)
   ↓
3. Plugin API Call
   Kubecost/Vantage/CloudProvider API
   ↓
4. Plugin Response
   31 daily cost entries
   ↓
5. Aggregation
   Sum daily costs, group by dimension
   ↓
6. Currency Validation
   Ensure consistent currency
   ↓
7. Output Formatting
   Table/JSON/NDJSON
```

## Fallback Cost Calculation

When plugins are unavailable, FinFocus uses local YAML specifications.

### Spec Format

```yaml
provider: aws
resource_type: ec2
sku: t3.micro
region: us-east-1
billing_mode: per_hour
rate_per_unit: 0.0104
currency: USD
description: AWS EC2 t3.micro instance pricing
```

### Calculation

```text
Monthly Cost = rate_per_unit × Hours Per Month
             = 0.0104 × 730
             = 7.592 USD/month
```

### Placeholder Costs

If no plugin or spec is available:

```json
{
  "cost_per_month": 0.0,
  "currency": "USD",
  "source": "unknown",
  "note": "No pricing data available"
}
```

## Real-World Examples

### Example 1: Simple EC2 Cost

**Input:**

```bash
finfocus cost projected --pulumi-json plan.json
```

**Plan (10 t3.micro instances):**

```json
{
  "resources": [
    {
      "type": "aws:ec2/instance:Instance",
      "inputs": {
        "instanceType": "t3.micro"
      }
    }
  ]
}
```

**Calculation:**

```text
Per Instance: 0.0104 × 730 = 7.592 USD/month
Total (10 instances): 10 × 7.592 = 75.92 USD/month
```

### Example 2: Mixed Resource Types

**Resources:**

- 5x EC2 t3.micro ($7.592/mo each)
- 3x RDS db.t3.micro ($24.82/mo each)
- 1x S3 bucket (100GB at $0.023/GB/mo)

**Calculation:**

```text
EC2: 5 × 7.592 = 37.96 USD/month
RDS: 3 × 24.82 = 74.46 USD/month
S3: 100 × 0.023 = 2.30 USD/month
Total: 37.96 + 74.46 + 2.30 = 114.72 USD/month
```

### Example 3: Actual Cost with Grouping

**Request:**

```bash
finfocus cost actual \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --group-by daily
```

**Output:**

```text
Date         | Cost (USD)
-------------|------------
2024-01-01   | 73.42
2024-01-02   | 74.18
2024-01-03   | 73.89
...
2024-01-31   | 72.95
-------------|------------
Total        | 2,287.64
```

## Performance Considerations

### Plugin Caching

Plugins are kept alive between requests to reduce startup overhead:

- First request: 100-500ms (plugin startup + API call)
- Subsequent requests: 10-50ms (API call only)

### Parallel Processing

Cost calculations for multiple resources are parallelized:

```text
Sequential: 100 resources × 50ms = 5000ms
Parallel (10 workers): 100 resources ÷ 10 × 50ms = 500ms
```

### API Rate Limiting

Plugins implement rate limiting to avoid API throttling:

- Exponential backoff on rate limit errors
- Request queuing and batching
- Configurable rate limits per plugin

---

**Related Documentation:**

- [System Overview](system-overview.md) - High-level architecture
- [Plugin Protocol](plugin-protocol.md) - gRPC protocol specification
- [Cost Calculation Flow](diagrams/cost-calculation-flow.md) - Flow
  diagram
- [Engine Implementation](../../internal/engine/CLAUDE.md) - Code details
