---
title: AWS Public Plugin
description: Estimate AWS costs from public list prices, with no AWS credentials.
parent: Plugins
nav_order: 10
---

## Overview

The `aws-public` plugin estimates the monthly cost of the AWS resources in a
Pulumi or Terraform plan from AWS public list prices. The prices are built into
the plugin, so it needs no AWS credentials, which makes it safe for CI/CD
pipelines. `finfocus setup` installs it by default.

**About AWS pricing:** [Amazon EC2 pricing](https://aws.amazon.com/ec2/pricing/),
the [AWS Price List bulk API](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/using-the-aws-price-list-bulk-api.html),
and [AWS Cost Explorer](https://docs.aws.amazon.com/cost-management/latest/userguide/ce-what-is.html)
for billed spend.

## What it prices

- **Compute**: EC2 instances (on-demand; Linux, Windows, RHEL or SUSE; shared,
  dedicated or host tenancy), Auto Scaling groups, Lambda, and ECS services on
  Fargate. It also prices EKS pods on Fargate that the `kubernetes` plugin
  reports.
- **Storage and data**: EBS, S3, EFS, RDS, DynamoDB and ElastiCache.
- **Networking and the rest**: NAT Gateways, Application and Network Load
  Balancers, EKS clusters, and CloudWatch log groups and alarms.

Free resources such as VPCs, subnets, security groups and IAM are reported at
$0. Gateway Load Balancers are reported as unsupported.

## Features

- **Projected costs**: Estimates monthly cost from
  [Pulumi](https://www.pulumi.com/docs/iac/cli/commands/pulumi_preview/) plans
  and Terraform plans and state. Terraform resource types and attribute names
  are mapped to their Pulumi equivalents.
- **Actual costs**: Estimates actual cost as list price times how long the
  resource has run, from its creation time in
  [Pulumi state](https://www.pulumi.com/docs/iac/concepts/state-and-backends/)
  or from `--from` and `--to`. This is not billed spend.
- **Carbon**: Estimates gCO2e for compute, storage and database resources: EC2,
  Auto Scaling groups, EBS, S3, EFS, Lambda, RDS, DynamoDB and ElastiCache.
- **Recommendations**: Newer instance generations and Graviton for EC2 (not for
  Windows), RDS and ElastiCache; Redis to Valkey for ElastiCache; gp3 instead of
  gp2 for EBS; arm64 for Lambda; and S3 and DynamoDB gateway endpoints for NAT
  Gateways when a data-volume tag is set.
- **No credentials**: Safe to use in CI/CD without secrets.

## Installation

```bash
finfocus plugin install aws-public
```

This installs the multi-region build, `finfocus-plugin-aws-public`. The first
time it prices a resource in a region, it downloads that region's build from the
plugin's GitHub release, so it needs network access to GitHub. Builds exist for
10 regions: us-east-1, us-west-1, us-west-2, eu-west-1, ap-southeast-1,
ap-southeast-2, ap-northeast-1, ap-south-1, ca-central-1 and sa-east-1.

To install one region's build instead, pass its region:

```bash
finfocus plugin install aws-public --metadata="region=us-west-2"
```

A region build has its prices built in and works offline, but it prices only
resources in that region. Resources in other regions are reported as
unsupported.

## Usage

The plugin is selected for `aws` resources when running cost analysis. When
`--pulumi-json` and `--pulumi-state` are omitted, the CLI auto-detects the
Pulumi project in the current directory. Date ranges are inferred from state
timestamps.

```bash
# Auto-detect Pulumi project (no flags needed)
finfocus cost projected
finfocus cost actual

# Auto-detect with specific stack
finfocus cost actual --stack production

# Projected costs from a Pulumi plan
finfocus cost projected --pulumi-json plan.json

# Actual costs from Pulumi state (dates auto-detected)
finfocus cost actual --pulumi-state state.json

# Actual costs with explicit date range
finfocus cost actual --pulumi-state state.json --from 2025-01-01 --to 2025-01-31
```

### With other AWS plugins

Without routing, FinFocus asks every plugin that supports `aws` in name order,
and falls back to the next one when a plugin returns an error. If you also
install [`aws-ce`](aws-ce.md), it is asked before `aws-public`. To send
projected cost to `aws-public`, configure
[routing](../guides/routing.md).

## Limitations

- **List prices only**: Does not account for Reserved Instances, Savings Plans,
  or EDP discounts.
- **Spot**: EC2 Spot instances and Fargate Spot are priced at on-demand rates.
- **Estimates**: Values are estimates and may differ from the final bill.
- **Data freshness**: Pricing data is updated with plugin releases.

For billed spend, add [`aws-ce`](aws-ce.md), which reads AWS Cost Explorer and
needs AWS credentials, or another billing-data plugin such as
[Vantage](vantage.md). See [multi-plugin routing](../guides/multi-plugin.md) and
the [cost calculation guide](../guides/cost-calculation.md).

## Troubleshooting

If a resource shows a message such as `EC2 instance type "t3.micro" not found
in pricing data`:

1. Ensure you are using the latest version of the plugin.
2. The instance type might be new and not yet in the plugin's prices.
3. Check that the resource type is in [What it prices](#what-it-prices).

If a resource is reported as unsupported:

1. Check that it is in one of the 10 built regions.
2. Give the resource a region (its `region`, availability zone or ARN), or set
   `AWS_REGION`. A resource with no region is not priced.
3. With a region build, check that the resource is in that build's region.
4. With the multi-region build, check that the machine can reach GitHub to
   download the region build.
