---
title: AWS Cost Explorer Plugin
description: Report billed AWS actual cost from Cost Explorer by account, service, or EC2 instance.
parent: Plugins
nav_order: 15
---

## Overview

The `aws-ce` plugin reads billed AWS cost from
[AWS Cost Explorer](https://docs.aws.amazon.com/cost-management/latest/userguide/ce-what-is.html)
and serves it to FinFocus as actual cost. It reports what AWS charged, including
discounts, reservations, and Savings Plans, rather than a list-price estimate.

It reports actual cost only. Projected cost requests return gRPC `Unimplemented`;
use `aws-public` for projected cost.

## Features

- **Account and service totals**: Cost for the whole account or for one AWS
  service, for up to the last 14 months.
- **EC2 instance cost**: Cost for one EC2 instance id or instance ARN. This uses
  Cost Explorer resource-level data, which needs an account opt-in and covers
  only the last 14 days.
- **Commitment data**: Reservation and Savings Plans details in the FOCUS record
  when Cost Explorer returns them.
- **Per-request credentials**: The host can pass AWS credentials or a role to
  assume with each request. Without them the plugin uses the AWS default
  credential chain.
- **Caching and limits**: Results are cached, recent data for a shorter time
  than closed periods. An optional per-minute request budget limits Cost Explorer
  spend, because each paginated request costs $0.01.

Cost Explorer data can lag usage by 24 hours or more.

## Installation

```bash
finfocus plugin install aws-ce
```

`finfocus setup` does not install this plugin by default.

## Configuration

The plugin needs AWS credentials with Cost Explorer read access. Set a profile
or the standard AWS environment variables, and set `AWS_REGION`. Cost Explorer
uses a global endpoint, so the region does not choose a billing dataset.

Allow these IAM actions:

- `ce:GetCostAndUsage` for account and service totals
- `ce:GetCostAndUsageWithResources` for EC2 instance cost
- `ce:GetDimensionValues` for commitment discovery
- `sts:AssumeRole` when the host passes a `role_arn`

| Variable | Purpose | Default |
| --- | --- | --- |
| `FINFOCUS_AWS_CE_MAX_REQUESTS_PER_MINUTE` | Cost Explorer request budget per minute | Unlimited |
| `FINFOCUS_AWS_CE_MAX_BATCH_SIZE` | Largest batch request, 1 to 1000 | 100 |
| `FINFOCUS_AWS_CE_BATCH_WORKERS` | Concurrent batch workers, 1 to 50 | 10 |

See the [plugin README](https://github.com/rshade/finfocus-plugin-aws-ce) and its
credentials guide for per-request credential keys.

## Limits

- EC2 is the only service with resource-level cost today. Other resources return
  an explicit unsupported error. Use an `aws:service` total instead of a zero.
- Resource-level queries need the Cost Explorer resource opt-in and look back
  14 days.
- The caller supplies `billing_account_id` to get a FOCUS record.
- `finfocus plugin inspect aws-ce` reports that capability discovery is not
  implemented, because the plugin does not implement `DryRun`.
