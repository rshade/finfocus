---
title: AWS Cost Explorer Plugin
description: Report billed AWS cost, account and service forecasts, AWS Budgets, and AWS recommendations from Cost Explorer.
layout: docs
parent: Plugins
nav_order: 15
---

## Overview

The `aws-ce` plugin reads billed AWS cost from
[AWS Cost Explorer](https://docs.aws.amazon.com/cost-management/latest/userguide/ce-what-is.html)
and serves it to FinFocus. It reports what AWS charged, including discounts,
reservations, and Savings Plans, rather than a list-price estimate.

The plugin only reads data. It never creates budgets, buys commitments, or
changes resources.

## Features

- **Actual cost**: Totals for the account or one AWS service, for up to the last
  14 months, and cost for one resource by EC2 instance id or AWS ARN, for the
  last 14 days.
- **Forecasts**: Projected cost for `aws:account` and `aws:service` is the
  month-to-date amortized cost plus the Cost Explorer forecast. Cost Explorer
  cannot forecast one resource, so a projected request for any other type
  returns `InvalidArgument` and FinFocus falls back to the next plugin.
- **Budgets**: Reads AWS Budgets and reports spend and health.
- **Recommendations**: EC2 rightsizing, Savings Plans, reservation purchases,
  and cost anomalies that AWS already holds.
- **Spot comparison**: Off by default. With it on, a projected request for an
  EC2 instance returns the Spot price with the on-demand rate.
- **Per-request credentials**: The host can pass AWS credentials or a role to
  assume with each request. Without them the plugin uses the AWS default
  credential chain.

Cost Explorer data lags usage by 24 hours or more, and resource-level data by
up to 48 hours.

## Installation

```bash
finfocus plugin install aws-ce
```

`finfocus setup` does not install this plugin by default.

## Configuration

The plugin needs AWS credentials. Set a profile or the standard AWS environment
variables, and set `AWS_REGION`. Cost Explorer uses a global endpoint, so the
region does not choose a billing dataset.

Grant only the IAM actions for the features you use:

| Feature | IAM actions |
| --- | --- |
| Account and service cost | `ce:GetCostAndUsage`, `ce:GetDimensionValues` |
| Resource cost | `ce:GetCostAndUsageWithResources`, `ce:GetDimensionValues` |
| Forecasts | `ce:GetCostForecast`, `ce:GetCostAndUsage` |
| Budgets | `budgets:ViewBudget` |
| Anomalies | `ce:GetAnomalies` |
| Rightsizing | `ce:GetRightsizingRecommendation` |
| Savings Plans recommendations | `ce:GetSavingsPlansPurchaseRecommendation` |
| Reservation recommendations | `ce:GetReservationPurchaseRecommendation` |
| Spot comparison | `ec2:DescribeSpotPriceHistory`, `pricing:GetProducts` |
| Host-supplied role | `sts:AssumeRole` |

| Variable | Purpose | Default |
| --- | --- | --- |
| `FINFOCUS_AWS_CE_MAX_REQUESTS_PER_MINUTE` | Cost Explorer and Budgets request budget per minute | Unlimited |
| `FINFOCUS_AWS_CE_MAX_WAIT_SECONDS` | Longest a request waits for the budget or a throttle | 90 |
| `FINFOCUS_AWS_CE_MAX_BATCH_SIZE` | Largest batch request, 1 to 1000 | 100 |
| `FINFOCUS_AWS_CE_BATCH_WORKERS` | Concurrent batch workers, 1 to 50 | 50 |
| `FINFOCUS_AWS_CE_SPOT_COMPARISON` | `true` answers EC2 instance projections with a Spot quote | `false` |
| `FINFOCUS_AWS_ACCOUNT_ID` | Account for Budgets; empty uses STS `GetCallerIdentity` | Empty |

AWS charges $0.01 for each paginated Cost Explorer API request, and one
FinFocus command can make several: a query with no cache hit needs at least two
commitment-discovery requests and one cost request, and a long result adds a
request per page. Commands that show recommendations, including
`cost projected`, ask `aws-ce` for them, so they make billed requests too.
`FINFOCUS_AWS_CE_MAX_REQUESTS_PER_MINUTE` counts every page the plugin requests,
so it bounds spend. You can also leave `Recommendations` out of the `aws-ce`
routing features below.

See the [plugin README](https://github.com/rshade/finfocus-plugin-aws-ce) for the
full settings list and per-request credential keys.

## Routing with aws-public

Without routing, FinFocus asks plugins in name order, so `aws-ce` is asked
before `aws-public`. A projected request for a single resource then fails in
`aws-ce` and falls back to `aws-public`. With the Spot comparison on, a Spot
price can replace the on-demand list price in a stack total. Route projected
cost to `aws-public` to avoid both:

```yaml
routing:
  plugins:
    - name: aws-public
      features: [ProjectedCosts]
      priority: 20
    - name: aws-ce
      features: [ActualCosts, Recommendations, Budgets]
      priority: 10
```

See [Routing](../guides/routing.md) for the full configuration.

## Limits

- Resource-level cost needs the Cost Explorer resource opt-in, at daily
  granularity, and looks back 14 days. EC2 is the only resource type checked
  against a live account.
- Missing billing data returns an error, not a zero cost.
- Rightsizing must be enabled in Cost Management preferences, and Savings Plans
  recommendations must already exist in AWS.
- The caller supplies `billing_account_id` to get a FOCUS record.
- `finfocus plugin inspect aws-ce` reports that capability discovery is not
  implemented, because the plugin does not implement `DryRun`.
