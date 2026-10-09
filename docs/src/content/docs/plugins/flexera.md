---
title: Flexera Plugin
description: Read AWS, Azure, and GCP cost from Flexera One Cloud Cost Optimization.
parent: Plugins
nav_order: 13
---

## Overview

The `flexera` plugin reads cloud cost from the Flexera One Cloud Cost
Optimization Bill Analysis API and serves it to FinFocus. It reports what
Flexera has billed for a resource, so it is a source for actual cost across AWS,
Azure, and GCP.

It needs a Flexera One organization, at least one billing center, and either a
refresh token or a service account.

## Features

- **Actual Costs**: Cost rows from `costs/select`, filtered by resource id, in
  windows of at most 31 days.
- **Projected Costs**: The current month from `forecasts/report`. When that
  report has no amounts, the plugin extrapolates the recent actuals linearly.
  `billing_detail` on the result says which method ran.
- **Pricing Specs**: `GetPricingSpec` is available for the supported resource
  types.
- **Regions**: Flexera zones NAM, EU, and APAC.
- **Billing Center Allocation**: Optional tag-to-billing-center rules label the
  result lineage.

## Installation

```bash
finfocus plugin install flexera

# Or pin the published release
finfocus plugin install flexera@v0.1.1
```

`finfocus setup` does not install this plugin by default.

## Configuration

Set the organization and billing centers, plus one credential:

```bash
export FLEXERA_ORG_ID="12345"
export FLEXERA_REGION="nam"
export FLEXERA_BILLING_CENTER_IDS="billing-center-id"
export FLEXERA_REFRESH_TOKEN="refresh-token"
```

| Variable                     | Purpose                                                    |
| ---------------------------- | ---------------------------------------------------------- |
| `FLEXERA_ORG_ID`             | Numeric organization id. Required.                         |
| `FLEXERA_BILLING_CENTER_IDS` | Comma-separated ids. Required for cost calls.              |
| `FLEXERA_REFRESH_TOKEN`      | Refresh token (preferred credential).                      |
| `FLEXERA_CLIENT_ID`          | Service account id, with `FLEXERA_CLIENT_SECRET`.          |
| `FLEXERA_REGION`             | `nam` (default), `eu`, or `apac`.                          |
| `FLEXERA_COST_METRIC`        | Default `cost_amortized_unblended_adj`.                    |

The plugin exits at start when the org id or a credential is missing, so
`finfocus plugin list` shows it as failed to start until both are set.

The org id is the number after `/orgs/` in the Flexera One URL. A billing
center id is in the billing center's page URL under Cloud Cost Optimization.
The plugin README lists the remaining settings and the billing center mapping
file.

## Resource Types

`Supports` accepts these resource type ids, case-insensitively:

| Resource Type   | Provider |
| --------------- | -------- |
| `aws-ec2`       | AWS      |
| `aws-s3`        | AWS      |
| `aws-rds`       | AWS      |
| `azure-vm`      | Azure    |
| `azure-storage` | Azure    |
| `gcp-compute`   | GCP      |
| `gcp-storage`   | GCP      |

Any other type is declined with `unsupported resource type`.

## Limitations

- **Pulumi Type Tokens**: As of v0.1.1 the plugin matches the ids above and does
  not map Pulumi type tokens such as `aws:ec2/instance:Instance`. FinFocus sends
  Pulumi tokens for resources read from a plan or state, so those resources are
  declined until the plugin maps them. Check the plugin releases before relying
  on it in a Pulumi workflow.
- **Billing Centers Required**: Every call sends the configured billing centers.
  The plugin does not take a billing center as a per-request filter.
- **Forecast Source**: A projected cost can come from a linear extrapolation
  instead of the Flexera forecast report; check `billing_detail`.

See the
[plugin README](https://github.com/rshade/finfocus-plugin-flexera#readme)
for setup details and troubleshooting.
