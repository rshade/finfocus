---
layout: default
title: AWS Public Plugin
description: Default fallback plugin for estimating AWS costs using public list pricing.
parent: Plugins
nav_order: 10
---

## Overview

The `aws-public` plugin is the default fallback provider for FinFocus. It uses
public list pricing (MSRP) to estimate costs and requires no AWS credentials,
making it safe for CI/CD pipelines.

## Features

- **Projected Costs**: Estimates monthly costs from Pulumi plans using public list pricing.
- **Actual Costs**: Derives historical costs from Pulumi state with SKU/region-aware pricing.
- **Offline Mode**: Uses an embedded database of pricing information.
- **No Credentials**: Safe to use in CI/CD without secrets.
- **Regional Binaries**: Install region-specific binaries for faster lookups via `--metadata`.
- **Fallback**: Automatically used by Core if no other plugin claims the resource or if credentials are missing.

## Installation

Both full repository path and short name forms are supported. Use the full path
to install from a specific repository; use the short name when the plugin is
published in the default registry.

```bash
finfocus plugin install github.com/rshade/finfocus-plugin-aws-public

# Install with region metadata (selects region-specific binary)
finfocus plugin install aws-public --metadata="region=us-west-2"
```

## Usage

The plugin is automatically selected for `aws` resources when running cost analysis.
When `--pulumi-json` and `--pulumi-state` are omitted, the CLI auto-detects the
Pulumi project in the current directory. Date ranges are inferred from state timestamps.

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

## Limitations

- **List Prices Only**: Does not account for Reserved Instances, Savings Plans, or EDP discounts.
- **Estimates**: Values are estimates and may differ from the final bill.
- **Data Freshness**: Pricing data is updated with plugin releases.

## Troubleshooting

If you see "Price not found" warnings:

1. Ensure you are using the latest version of the plugin.
2. The instance type or region might be new or not yet added to the embedded database.
3. Check if the resource type is supported (EC2, RDS, S3, etc.).
