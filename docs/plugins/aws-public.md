---
layout: default
title: AWS Public Plugin
parent: Plugins
nav_order: 10
---

The `aws-public` plugin is the default fallback provider for FinFocus. It provides estimated costs using public list
pricing (MSRP) without requiring AWS credentials.

## Features

- **Projected Costs**: Estimates monthly costs for Pulumi plans.
- **Offline Mode**: Uses an embedded database of pricing information.
- **No Credentials**: Safe to use in CI/CD without secrets.
- **Fallback**: Automatically used by Core if no other plugin claims the resource or if credentials are missing.

## Installation

```bash
finfocus plugin install github.com/rshade/finfocus-plugin-aws-public
```

## Usage

The plugin is automatically selected for `aws` resources when running `projected` cost analysis.

```bash
finfocus cost projected --pulumi-json plan.json
```

## Limitations

- **List Prices Only**: Does not account for Reserved Instances, Savings Plans, or EDP discounts.
- **Estimates**: Values are estimates and may differ from the final bill.
- **Data Freshness**: Pricing data is updated with plugin releases.

## Troubleshooting

If you see "Price not found" warnings:

1. Ensure you are using the latest version of the plugin.
2. The instance type or region might be new or not yet added to the embedded database.
3. Check if the resource type is supported (EC2, RDS, S3, etc.).
