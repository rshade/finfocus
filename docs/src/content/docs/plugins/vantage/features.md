---
layout: default
title: Vantage Plugin Features
description: Supported cloud providers, cost types, limitations, and roadmap for the FinFocus Vantage plugin
---

This document describes the features, capabilities, and limitations of the
Vantage plugin for FinFocus.

## Table of Contents

1. [Overview](#overview)
2. [Supported Cloud Providers](#supported-cloud-providers)
3. [Supported Cost Types](#supported-cost-types)
4. [Supported Operations](#supported-operations)
5. [Limitations](#limitations)
6. [Feature Roadmap](#feature-roadmap)
7. [API Version Compatibility](#api-version-compatibility)

---

## Overview

The Vantage plugin is a read-only adapter that fetches normalized cost and
usage data from Vantage's REST API and maps it to FinFocus's internal
schema with FinOps FOCUS 1.2 compatibility.

### Key Capabilities

- **Multi-Cloud Cost Aggregation**: Unified cost reporting across AWS, Azure,
  GCP, and more
- **Historical Cost Analysis**: Retrieve actual costs with daily granularity
- **Flexible Filtering**: Filter by provider, service, account, region, tags
- **Cost Forecasting**: Snapshot-based cost projections
- **FOCUS 1.2 Compliance**: Industry-standard cost record format
- **Incremental Sync**: Bookmark-based synchronization with rate limit backoff
- **Tag Normalization**: Automatic tag key formatting and filtering

---

## Supported Cloud Providers

The Vantage plugin supports cost data from the following cloud providers
through Vantage's aggregation API:

### Fully Supported

| Provider               | Support Level | Notes                                 |
| ---------------------- | ------------- | ------------------------------------- |
| **AWS**                | Full          | All services, RI/SP, data transfer    |
| **Google Cloud (GCP)** | Full          | All services, committed use discounts |
| **Microsoft Azure**    | Full          | All services, reservations            |
| **Kubernetes**         | Full          | Via Vantage's Kubernetes integration  |
| **Snowflake**          | Full          | Compute and storage costs             |
| **Databricks**         | Full          | Workspace and cluster costs           |
| **MongoDB Atlas**      | Full          | Cluster and data transfer costs       |
| **Datadog**            | Full          | Monitoring and APM costs              |
| **Confluent Cloud**    | Full          | Kafka cluster costs                   |
| **Fastly**             | Full          | CDN and compute costs                 |

### Partial Support

| Provider          | Support Level | Limitations              |
| ----------------- | ------------- | ------------------------ |
| **Oracle Cloud**  | Partial       | Limited service coverage |
| **Alibaba Cloud** | Partial       | Core services only       |

### Provider Detection

The plugin automatically extracts provider information from resource types:

```text
aws:ec2:Instance       → Provider: AWS
google:compute:Disk    → Provider: GCP
azurerm:compute:VM     → Provider: Azure
kubernetes:apps:Pod    → Provider: Kubernetes
```

---

## Supported Cost Types

### Cost Metrics

The plugin supports multiple cost metric types:

| Metric                   | Description                            | Availability    |
| ------------------------ | -------------------------------------- | --------------- |
| **cost**                 | Net cost after discounts, before taxes | All providers   |
| **amortized_cost**       | Amortized including RI/SP allocation   | AWS, GCP, Azure |
| **usage**                | Usage quantity in native units         | All providers   |
| **effective_unit_price** | Computed per-unit cost                 | All providers   |
| **taxes**                | Tax amounts                            | AWS, Azure      |
| **credits**              | Free tier, promotional credits         | AWS, GCP, Azure |
| **refunds**              | Refund amounts                         | AWS, Azure      |

### Cost Dimensions

Group and filter costs by:

- **provider**: Cloud provider (AWS, GCP, Azure, etc.)
- **service**: Cloud service (EC2, S3, Compute Engine, etc.)
- **account**: Billing account or AWS account ID
- **project**: GCP project or organizational unit
- **region**: Geographic region (us-east-1, europe-west1, etc.)
- **resource_id**: Specific cloud resource identifier
- **tags**: Custom tags/labels applied to resources

### Granularity Options

- **Daily**: Day-by-day cost breakdown (recommended)
- **Monthly**: Aggregated monthly costs

---

## Supported Operations

### Historical Cost Retrieval

Fetch actual historical costs from Vantage:

```bash
# Daily costs for date range
finfocus cost actual \
  --plugin vantage \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --granularity day

# Group by service
finfocus cost actual \
  --plugin vantage \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --group-by service

# Filter by tags
finfocus cost actual \
  --plugin vantage \
  --filter "tag:environment=production" \
  --start-date 2024-01-01 \
  --end-date 2024-01-31
```

### Cost Forecasting

Retrieve forecast snapshots:

```bash
# Get forecast data
finfocus-vantage forecast \
  --config config.yaml \
  --out forecast.json
```

### Incremental Synchronization

Daily sync with bookmark tracking:

```bash
# Incremental sync (captures D-3 to D-1 for late postings)
finfocus-vantage pull --config config.yaml

# Backfill historical data
finfocus-vantage backfill --config config.yaml --months 12
```

### Tag Filtering

Filter by specific tag prefixes:

```yaml
params:
  tag_prefix_filters:
    - 'user:'
    - 'kubernetes.io/'
    - 'cost-center:'
```

### Output Formats

- **Table**: Human-readable formatted table
- **JSON**: Single JSON object
- **NDJSON**: Newline-delimited JSON stream

---

## Limitations

### Known Limitations

#### 1. Read-Only Access

- **Description**: Plugin cannot create or modify Vantage resources
- **Impact**: Cost optimization requires manual actions in cloud provider
- **Workaround**: Use Vantage console or cloud provider tools for changes

#### 2. Data Latency

- **Description**: Cost data lags 12-48 hours from actual usage
- **Impact**: Real-time cost tracking not possible
- **Workaround**: Account for lag in reporting; use 3-day window for
  incremental syncs

#### 3. Rate Limiting

- **Description**: Vantage API enforces rate limits (varies by subscription)
- **Impact**: Large data syncs may require retry with backoff
- **Workaround**: Plugin automatically retries with exponential backoff
  (configurable via `max_retries`)

#### 4. Tag Cardinality

- **Description**: High tag cardinality increases record count dramatically
- **Impact**: Performance degradation, increased API page count
- **Workaround**: Use `tag_prefix_filters` to limit included tags

#### 5. Forecast Limitations

- **Description**: Forecasts require Cost Report tokens (not Workspace tokens)
- **Impact**: Forecast functionality unavailable with Workspace tokens
- **Workaround**: Use Cost Report tokens for production deployments

#### 6. No Direct Cost Optimization

- **Description**: Plugin retrieves cost data but doesn't provide
  recommendations
- **Impact**: Optimization analysis must be done separately
- **Workaround**: Use FinFocus analyzers or Vantage console for
  recommendations

#### 7. Provider Metric Availability

- **Description**: Not all metrics available for all providers
- **Impact**: Some cost types may return `null` values
- **Workaround**: Verify metric availability in Vantage console for target
  providers

---

## Feature Roadmap

### Current Version (v0.1.0)

- ✅ Multi-cloud cost retrieval
- ✅ Daily and monthly granularity
- ✅ Tag-based filtering
- ✅ Incremental sync with bookmarks
- ✅ Forecast snapshots
- ✅ FOCUS 1.2 compatibility
- ✅ Multiple output formats (table, JSON, NDJSON)

### Planned Features

#### Q1 2025

- 🔄 **Cost Anomaly Detection**: Automatic identification of cost spikes
- 🔄 **Budget Alerts Integration**: Sync Vantage budgets with FinFocus
- 🔄 **Enhanced Tag Management**: Advanced tag normalization rules

#### Q2 2025

- 📅 **Cost Allocation Rules**: Custom cost allocation logic
- 📅 **Multi-Workspace Support**: Query multiple Vantage workspaces
- 📅 **Commit Discount Tracking**: Detailed RI/SP utilization metrics

#### Q3 2025

- 📅 **Real-Time Streaming**: WebSocket-based cost updates (if Vantage
  supports)
- 📅 **Cost Optimization API**: Programmatic recommendation retrieval
- 📅 **Custom Metric Definitions**: User-defined calculated metrics

### Future Considerations

- Enhanced Kubernetes cost allocation
- Multi-region cost aggregation with currency conversion
- Integration with FinFocus policy engine
- Advanced filtering with VQL (Vantage Query Language) support

---

## API Version Compatibility

### Current Protocol Version

**Plugin Version:** v0.1.0
**Vantage API Version:** v1
**FinFocus Protocol:** v0.1.0
**FOCUS Spec:** 1.2

### Compatibility Matrix

| Plugin Version   | Vantage API | FinFocus Core | FOCUS Spec |
| ---------------- | ----------- | ------------- | ---------- |
| v0.1.0           | v1          | v0.1.0+       | 1.2        |
| v0.2.0 (planned) | v1          | v0.2.0+       | 1.2        |

### Breaking Changes

None in current version (v0.1.0)

Future breaking changes will be announced with:

- Major version bump (e.g., v1.0.0 → v2.0.0)
- Migration guide in release notes
- Deprecation warnings for 90 days minimum

### Deprecation Policy

- **Minor deprecations**: 90-day notice
- **Major deprecations**: 180-day notice
- **Critical security fixes**: Immediate (with migration path if possible)

---

## Performance Characteristics

### Throughput

- **Page Size**: Configurable 1,000-10,000 records (default: 5,000)
- **API Requests**: ~1-5 requests/second (rate limit dependent)
- **Typical Sync Duration**: 1-5 minutes for 30 days of data

### Resource Usage

- **Memory**: ~50-200 MB during sync (depends on page size)
- **CPU**: Low (mostly I/O bound)
- **Network**: ~1-10 MB per sync (depends on data volume)

### Optimization Tips

1. **Use Cost Report Tokens**: Better performance than Workspace tokens
2. **Reduce Dimensions**: Fewer `group_bys` = faster queries
3. **Increase Page Size**: For large syncs (but watch memory)
4. **Filter Tags**: Use `tag_prefix_filters` to reduce cardinality
5. **Monthly Granularity**: Use for long historical ranges

---

## Supported Use Cases

### Cost Reporting

- ✅ Generate monthly/quarterly cost reports
- ✅ Track cost trends over time
- ✅ Compare actual vs projected costs

### Cost Allocation

- ✅ Allocate costs by team, project, or environment
- ✅ Tag-based cost distribution
- ✅ Multi-cloud cost rollups

### Cost Optimization

- ⚠️ Identify high-cost resources (data only, no recommendations)
- ⚠️ Detect cost anomalies (manual analysis required)
- ❌ Automated optimization actions (use Vantage console)

### Compliance & Governance

- ✅ FOCUS 1.2 compliant cost records
- ✅ Audit trail with bookmark tracking
- ✅ Read-only access (no modification risk)

---

## Feature Comparison

### Vantage Plugin vs. Other Plugins

| Feature              | Vantage       | Kubecost          | AWS Cost Explorer |
| :------------------- | :------------ | :---------------- | :---------------- |
| **Multi-Cloud**      | ✅ Full       | ❌ K8s only       | ❌ AWS only       |
| **FOCUS Compliance** | ✅ Yes        | ⚠️ Partial        | ❌ No             |
| **Tag Filtering**    | ✅ Advanced   | ✅ Basic          | ✅ Basic          |
| **Forecasting**      | ✅ Snapshot   | ❌ No             | ✅ ML-based       |
| **Real-Time**        | ❌ 12-48h lag | ✅ Near real-time | ❌ 24h lag        |
| **Optimization**     | ❌ No         | ✅ Yes            | ✅ Yes            |

---

## Additional Resources

- [Vantage API Documentation](https://docs.vantage.sh/api)
- [Vantage Feature Matrix](https://docs.vantage.sh/features)
- [FinOps FOCUS Specification](https://focus.finops.org/)
- [Setup Guide](setup.md) - Installation and configuration
- [Authentication Guide](authentication.md) - API key management
- [Cost Mapping Guide](cost-mapping.md) - Data transformation details
- [Troubleshooting Guide](troubleshooting.md) - Common issues
