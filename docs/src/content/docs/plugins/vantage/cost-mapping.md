---
layout: default
title: Vantage Plugin Cost Mapping
description: How Vantage costs map to FinFocus resources, tag transformation, and data processing details
---

This document explains how cost data from Vantage is transformed and mapped to
FinFocus's internal schema, including tag normalization, provider mapping,
and aggregation behavior.

## Table of Contents

1. [Overview](#overview)
2. [Cost Record Mapping](#cost-record-mapping)
3. [Provider Mapping](#provider-mapping)
4. [Tag Mapping and Transformation](#tag-mapping-and-transformation)
5. [Cost Aggregation Behavior](#cost-aggregation-behavior)
6. [Data Freshness and Update Frequency](#data-freshness-and-update-frequency)
7. [FOCUS 1.2 Compliance](#focus-12-compliance)
8. [Examples](#examples)

---

## Overview

The Vantage plugin acts as an adapter between Vantage's REST API and
FinFocus's internal data model. It performs several transformation steps:

```text
Vantage API Response
        ↓
   Parse JSON
        ↓
   Extract Dimensions (provider, service, account, region, tags)
        ↓
   Normalize Tags (kebab-case, prefix filtering)
        ↓
   Map to FOCUS 1.2 Schema
        ↓
   Convert to FinFocus Resource Descriptor
        ↓
FinFocus Cost Record
```

### Key Transformations

- **Tag Normalization**: Convert tag keys to lowercase kebab-case
- **Provider Extraction**: Infer provider from resource type or service name
- **Cost Type Mapping**: Map Vantage metrics to FinFocus cost types
- **Date Handling**: UTC timezone normalization
- **Currency Handling**: Default to USD (configurable in future)

---

## Cost Record Mapping

### Vantage API Response Format

Example Vantage API response:

```json
{
  "date": "2024-01-15",
  "provider": "aws",
  "service": "ec2",
  "account": "123456789012",
  "region": "us-east-1",
  "resource_id": "i-0abc123def456",
  "tags": {
    "Environment": "production",
    "CostCenter": "engineering",
    "user:team": "platform"
  },
  "cost": 45.67,
  "usage": 730.0,
  "unit": "hrs",
  "amortized_cost": 42.3,
  "taxes": 0.5,
  "credits": 0.0
}
```

### FinFocus Internal Format

Mapped to FinFocus schema:

```json
{
  "resource": {
    "type": "aws:ec2:Instance",
    "name": "i-0abc123def456",
    "provider": "aws",
    "properties": {
      "region": "us-east-1",
      "account": "123456789012",
      "service": "ec2"
    },
    "tags": {
      "environment": "production",
      "cost-center": "engineering",
      "user:team": "platform"
    }
  },
  "cost": {
    "projected": 0.0,
    "actual": 45.67,
    "currency": "USD",
    "period": "2024-01-15T00:00:00Z"
  },
  "metrics": {
    "usage": 730.0,
    "unit": "hrs",
    "amortized_cost": 42.3,
    "taxes": 0.5,
    "credits": 0.0
  }
}
```

### Field Mapping

| Vantage Field    | FinFocus Field                | Transformation              |
| ---------------- | ----------------------------- | --------------------------- |
| `date`           | `cost.period`                 | Parse to RFC3339 UTC        |
| `provider`       | `resource.provider`           | Lowercase                   |
| `service`        | `resource.properties.service` | Lowercase                   |
| `account`        | `resource.properties.account` | Pass-through                |
| `region`         | `resource.properties.region`  | Pass-through                |
| `resource_id`    | `resource.name`               | Pass-through                |
| `tags`           | `resource.tags`               | Normalize keys (kebab-case) |
| `cost`           | `cost.actual`                 | Parse as float              |
| `usage`          | `metrics.usage`               | Parse as float              |
| `amortized_cost` | `metrics.amortized_cost`      | Parse as float              |
| `taxes`          | `metrics.taxes`               | Parse as float              |
| `credits`        | `metrics.credits`             | Parse as float              |

---

## Provider Mapping

### Automatic Provider Detection

The plugin extracts provider information from multiple sources:

#### 1. Explicit Provider Field

If Vantage response includes `provider`:

```json
{
  "provider": "aws",
  "service": "ec2",
  "resource_id": "i-0abc123"
}
```

Maps to:

```text
Provider: aws
Type: aws:ec2:Instance
```

#### 2. Resource Type Inference

If no explicit provider, infer from resource type:

```text
aws:ec2:Instance       → aws
google:compute:Disk    → google (normalized to "gcp")
azurerm:compute:VM     → azurerm (normalized to "azure")
kubernetes:apps:Pod    → kubernetes
```

#### 3. Service Name Mapping

Fallback to service-based mapping:

| Service                              | Provider   |
| ------------------------------------ | ---------- |
| `ec2`, `s3`, `rds`, `lambda`         | AWS        |
| `compute`, `storage`, `bigquery`     | GCP        |
| `virtualmachines`, `storageaccounts` | Azure      |
| `pods`, `deployments`, `services`    | Kubernetes |

### Provider Normalization

Vantage may return different provider identifiers:

| Vantage Value                          | Normalized Value |
| -------------------------------------- | ---------------- |
| `aws`, `AWS`, `amazon`                 | `aws`            |
| `gcp`, `GCP`, `google`, `google-cloud` | `gcp`            |
| `azure`, `Azure`, `azurerm`            | `azure`          |
| `kubernetes`, `k8s`                    | `kubernetes`     |

---

## Tag Mapping and Transformation

### Tag Normalization Process

Tags undergo several transformations:

#### Step 1: Key Normalization

Convert tag keys to lowercase kebab-case:

```text
CostCenter        → cost-center
Environment       → environment
user:team         → user:team (prefixes preserved)
kubernetes.io/app → kubernetes.io/app (special chars preserved)
```

#### Step 2: Prefix Filtering

Apply `tag_prefix_filters` if configured:

```yaml
params:
  tag_prefix_filters:
    - 'user:'
    - 'kubernetes.io/'
    - 'cost-center'
```

**Input Tags:**

```json
{
  "user:team": "platform",
  "kubernetes.io/app": "api",
  "cost-center": "engineering",
  "internal-id": "abc123",
  "pod-uid": "xyz789"
}
```

**Output Tags (after filtering):**

```json
{
  "user:team": "platform",
  "kubernetes.io/app": "api",
  "cost-center": "engineering"
}
```

Tags `internal-id` and `pod-uid` filtered out (no matching prefix).

#### Step 3: Raw Tag Preservation

Original tags preserved in `labels_raw`:

```json
{
  "tags": {
    "user:team": "platform"
  },
  "labels_raw": {
    "user:team": "platform",
    "User:Team": "Platform"
  }
}
```

### Tag Transformation Examples

| Original Vantage Tag    | Normalized Key          | Filtered? | Notes            |
| ----------------------- | ----------------------- | --------- | ---------------- |
| `CostCenter=eng`        | `cost-center=eng`       | No        | No prefix filter |
| `user:Team=platform`    | `user:team=platform`    | No        | Prefix preserved |
| `kubernetes.io/app=api` | `kubernetes.io/app=api` | No        | Special chars    |
| `pod-uid=abc123`        | `pod-uid=abc123`        | Yes       | High cardinality |

---

## Cost Aggregation Behavior

### Aggregation Dimensions

Costs are aggregated based on `group_bys` configuration:

#### Example 1: Group by Provider and Service

```yaml
params:
  group_bys:
    - provider
    - service
```

**Result:** One cost record per (provider, service) pair per day

```json
[
  { "provider": "aws", "service": "ec2", "cost": 100.0, "date": "2024-01-15" },
  { "provider": "aws", "service": "s3", "cost": 50.0, "date": "2024-01-15" },
  { "provider": "gcp", "service": "compute", "cost": 75.0, "date": "2024-01-15" }
]
```

#### Example 2: Group by Resource ID

```yaml
params:
  group_bys:
    - provider
    - service
    - resource_id
```

**Result:** One cost record per resource per day

```json
[
  { "provider": "aws", "service": "ec2", "resource_id": "i-abc123", "cost": 45.0 },
  { "provider": "aws", "service": "ec2", "resource_id": "i-def456", "cost": 55.0 }
]
```

### Granularity Impact

#### Daily Granularity

One cost record per day per dimension group:

```json
{"date": "2024-01-01", "provider": "aws", "cost": 100.00}
{"date": "2024-01-02", "provider": "aws", "cost": 105.00}
{"date": "2024-01-03", "provider": "aws", "cost": 110.00}
```

#### Monthly Granularity

One cost record per month per dimension group:

```json
{"date": "2024-01-01", "provider": "aws", "cost": 3100.00}
{"date": "2024-02-01", "provider": "aws", "cost": 2900.00}
```

### Cost Metric Aggregation

Multiple metrics aggregated per record:

```json
{
  "date": "2024-01-15",
  "provider": "aws",
  "service": "ec2",
  "cost": 100.0,
  "amortized_cost": 95.0,
  "usage": 2400.0,
  "unit": "hrs",
  "taxes": 1.5,
  "credits": 0.0
}
```

---

## Data Freshness and Update Frequency

### Cost Data Latency

Vantage cost data has inherent latency:

| Cloud Provider | Typical Lag    | Notes                                       |
| -------------- | -------------- | ------------------------------------------- |
| **AWS**        | 12-24 hours    | Can take up to 48h for final reconciliation |
| **GCP**        | 24-48 hours    | BigQuery billing export lag                 |
| **Azure**      | 24-48 hours    | Usage API lag                               |
| **Kubernetes** | Near real-time | Via Kubecost integration                    |

### Late Posting Window

Cost data can be updated after initial posting:

```text
Day 0: Usage occurs
Day 1: Initial cost data appears (incomplete)
Day 2: Cost data updated (adjustments, taxes)
Day 3: Final cost data (reconciliation complete)
```

**Recommendation:** Use 3-day lag window for incremental syncs:

```yaml
# Captures D-3 to D-1 data
params:
  start_date: null # Automatic: today - 3 days
  end_date: null # Automatic: today - 1 day
```

### Update Frequency

Recommended sync schedule:

| Sync Type       | Frequency        | Purpose                                 |
| --------------- | ---------------- | --------------------------------------- |
| **Incremental** | Daily (2 AM UTC) | Capture previous day costs (D-3 to D-1) |
| **Backfill**    | Weekly           | Re-sync previous week for corrections   |
| **Full Sync**   | Monthly          | Complete historical data refresh        |

### Idempotency

Cost records have idempotency keys:

```text
Key: {date}:{provider}:{service}:{account}:{region}:{resource_id}
```

Duplicate syncs with same key will overwrite existing records (upsert
behavior).

---

## FOCUS 1.2 Compliance

### FinOps FOCUS Spec

The plugin generates FOCUS 1.2 compliant cost records:

#### Required Fields

- `BillingPeriodStart`: Start of billing period (mapped from `date`)
- `BillingPeriodEnd`: End of billing period (mapped from `date`)
- `ChargeCategory`: Type of charge (`Usage`, `Tax`, `Credit`)
- `ChargeDescription`: Description of charge (from `service`)
- `ChargeFrequency`: Frequency (`Daily`, `Monthly`)
- `ChargePeriodStart`: Start of charge period
- `ChargePeriodEnd`: End of charge period
- `Provider`: Cloud provider name
- `Publisher`: Same as provider
- `InvoiceIssuer`: Cloud provider
- `BilledCost`: Net cost after discounts
- `EffectiveCost`: Amortized cost including RI/SP
- `ListCost`: List price (if available)

#### Optional Fields

- `ResourceId`: Cloud resource identifier
- `ResourceName`: Human-readable resource name
- `ServiceName`: Cloud service name
- `ServiceCategory`: Service category
- `Region`: Geographic region
- `AvailabilityZone`: AZ (if available)
- `Tags`: Custom tags/labels

### FOCUS Field Mapping

| FOCUS Field          | Vantage Source   | Transformation             |
| -------------------- | ---------------- | -------------------------- |
| `BillingPeriodStart` | `date`           | Parse to UTC date          |
| `BillingPeriodEnd`   | `date`           | Parse to UTC date          |
| `ChargeCategory`     | `metric type`    | Map: cost→Usage, taxes→Tax |
| `Provider`           | `provider`       | Normalize                  |
| `BilledCost`         | `cost`           | Parse float                |
| `EffectiveCost`      | `amortized_cost` | Parse float                |
| `ResourceId`         | `resource_id`    | Pass-through               |
| `ServiceName`        | `service`        | Pass-through               |
| `Region`             | `region`         | Pass-through               |
| `Tags`               | `tags`           | Normalize and filter       |

---

## Examples

### Example 1: AWS EC2 Instance Cost Mapping

**Vantage Response:**

```json
{
  "date": "2024-01-15",
  "provider": "aws",
  "service": "ec2",
  "account": "123456789012",
  "region": "us-east-1",
  "resource_id": "i-0abc123def456",
  "tags": {
    "Name": "web-server-prod",
    "Environment": "production"
  },
  "cost": 45.67,
  "usage": 730.0,
  "unit": "hrs"
}
```

**FinFocus Mapping:**

```json
{
  "resource": {
    "type": "aws:ec2:Instance",
    "name": "i-0abc123def456",
    "provider": "aws",
    "properties": {
      "region": "us-east-1",
      "account": "123456789012",
      "service": "ec2"
    },
    "tags": {
      "name": "web-server-prod",
      "environment": "production"
    }
  },
  "cost": {
    "projected": 0.0,
    "actual": 45.67,
    "currency": "USD",
    "period": "2024-01-15T00:00:00Z"
  },
  "metrics": {
    "usage": 730.0,
    "unit": "hrs"
  }
}
```

### Example 2: Kubernetes Pod Cost with Tag Filtering

**Vantage Response:**

```json
{
  "date": "2024-01-15",
  "provider": "kubernetes",
  "service": "pods",
  "namespace": "production",
  "resource_id": "api-deployment-abc123",
  "tags": {
    "kubernetes.io/app": "api",
    "user:team": "platform",
    "pod-uid": "xyz789-abc123",
    "cost-center": "engineering"
  },
  "cost": 12.34,
  "usage": 24.0,
  "unit": "hrs"
}
```

**Configuration:**

```yaml
params:
  tag_prefix_filters:
    - 'kubernetes.io/'
    - 'user:'
    - 'cost-center'
```

**FinFocus Mapping:**

```json
{
  "resource": {
    "type": "kubernetes:apps:Pod",
    "name": "api-deployment-abc123",
    "provider": "kubernetes",
    "properties": {
      "namespace": "production",
      "service": "pods"
    },
    "tags": {
      "kubernetes.io/app": "api",
      "user:team": "platform",
      "cost-center": "engineering"
    }
  },
  "cost": {
    "projected": 0.0,
    "actual": 12.34,
    "currency": "USD",
    "period": "2024-01-15T00:00:00Z"
  },
  "metrics": {
    "usage": 24.0,
    "unit": "hrs"
  }
}
```

**Note:** Tag `pod-uid` filtered out (no matching prefix).

### Example 3: Multi-Cloud Cost Aggregation

**Vantage Response (multiple records):**

```json
[
  { "provider": "aws", "service": "ec2", "cost": 100.0, "date": "2024-01-15" },
  { "provider": "aws", "service": "s3", "cost": 50.0, "date": "2024-01-15" },
  { "provider": "gcp", "service": "compute", "cost": 75.0, "date": "2024-01-15" },
  { "provider": "azure", "service": "vm", "cost": 80.0, "date": "2024-01-15" }
]
```

**FinFocus Aggregated Cost:**

```json
{
  "total_cost": 305.0,
  "currency": "USD",
  "period": "2024-01-15T00:00:00Z",
  "breakdown": [
    { "provider": "aws", "cost": 150.0 },
    { "provider": "gcp", "cost": 75.0 },
    { "provider": "azure", "cost": 80.0 }
  ]
}
```

---

## Additional Resources

- [FinOps FOCUS Specification v1.2](https://focus.finops.org/)
- [Vantage API Documentation](https://docs.vantage.sh/api)
- [Vantage Data Model](https://docs.vantage.sh/data-model)
- [Setup Guide](setup.md) - Installation and configuration
- [Authentication Guide](authentication.md) - API key management
- [Features Guide](features.md) - Supported capabilities
- [Troubleshooting Guide](troubleshooting.md) - Common issues
