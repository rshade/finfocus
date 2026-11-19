---
title: CloudZero Plugin
description: Read actual AWS, Azure, and GCP cost per resource from CloudZero.
parent: Plugins
nav_order: 16
---

## Overview

The `cloudzero` plugin reads actual cost per resource from the CloudZero
billing costs API (`GET /v2/billing/costs`) and serves it to FinFocus. It's an
actual-cost source only: projected cost, pricing specs, and estimates return
`Unimplemented`. See also the [Vantage plugin](vantage.md) and the
[Flexera plugin](flexera.md) for other billing-data sources.

It needs a CloudZero API key with the `billing:read_costs` and
`billing:read_dimensions` scopes, and the ID of a CloudZero dimension that has
one row per resource.

**About CloudZero:** [CloudZero](https://www.cloudzero.com/) and the
[CloudZero documentation](https://docs.cloudzero.com/docs/).

## Features

- **Actual Costs**: One result per day for each resource, read from a
  month-aligned daily query and sliced to the requested window.
- **Batch**: `BatchCost` answers many resources with one query per account and
  region, so a stack costs a few CloudZero requests rather than one per
  resource.
- **Resource Matching**: An AWS ARN is converted to a CloudZero resource name
  (CZRN). Resources with no ARN in Pulumi state, such as NAT gateways, are
  matched by their AWS id.
- **Request Budget**: The plugin stops before `CLOUDZERO_DAILY_BUDGET` requests
  (default 50) and caches each query.

## Installation

```bash
finfocus plugin install cloudzero

# Or pin the published release
finfocus plugin install cloudzero@v0.1.1
```

`finfocus setup` doesn't install this plugin by default.

## Configuration

```bash
export CLOUDZERO_API_KEY="api-key"
export CLOUDZERO_RESOURCE_DIMENSION="Resource"
```

| Variable                       | Purpose                                                         |
| ------------------------------ | --------------------------------------------------------------- |
| `CLOUDZERO_API_KEY`            | API key, sent without a `Bearer` prefix. Required for cost calls. |
| `CLOUDZERO_RESOURCE_DIMENSION` | Dimension ID with one row per resource. `Resource` works.       |
| `CLOUDZERO_TAG_DIMENSIONS`     | Optional `tagKey=dimensionID` pairs that narrow the query.      |
| `CLOUDZERO_COST_TYPE`          | Default `real_cost`, CloudZero's default.                       |
| `CLOUDZERO_CURRENCY`           | Label for results. The API returns no currency. Default `USD`.  |
| `CLOUDZERO_DAILY_BUDGET`       | Requests this process makes, retries included. Default `50`.    |

Without a key or a resource dimension, `Supports` declines every resource with
a reason, and cost calls return `FailedPrecondition`.

## Limitations

- **Rate Limit**: CloudZero allows 60 cost requests in a rolling 24 hours, and
  other callers on the same key count toward it. A 429 becomes
  `ResourceExhausted` that names the wait CloudZero gives. The plugin doesn't
  retry it: for 5 minutes after a 429, new queries fail at once without
  spending the daily budget.
- **Costs Outside the Stack**: CloudZero records root EBS volumes and the
  public IPv4 charge of an Elastic IP as their own resources (`volume` and
  `network-interface`). From v0.1.1, when an instance's attributes name its
  root volume (`rootBlockDevice.volumeId`, as Pulumi state does), the volume's
  cost is added to the instance as a separate root volume line. Data volumes
  aren't added, and the public IPv4 charge isn't part of the EIP result.
- **Verification**: v0.1.0 was checked against a live CloudZero account for
  AWS EC2 instances, NAT gateways, and Elastic IPs. Azure and GCP behaviour isn't
  verified.

See the
[plugin README](https://github.com/rshade/finfocus-plugin-cloudzero#readme)
for the call shape and the full list of settings.
