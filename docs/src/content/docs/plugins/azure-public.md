---
title: Azure Public Plugin
description: Estimate Azure costs from the public Azure Retail Prices API without credentials.
parent: Plugins
nav_order: 11
---

## Overview

The `azure-public` plugin estimates Azure costs from the public
[Azure Retail Prices API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices).
It needs no Azure credentials, so it is safe to run in CI/CD pipelines.

## Features

- **Projected Costs**: Monthly retail quotes for resources in a Pulumi plan.
- **Actual Costs**: The projected monthly quote scaled by `hours / 730`. This
  is a list-price projection, not billed spend.
- **Pricing Specs**: Returns the unit rate, billing mode, and assumptions for a
  resource, used by `cost estimate` and `--pricing-spec-fallback`.
- **Estimate Cost and Dry Run**: Quick what-if quotes and field-mapping
  inspection through `finfocus plugin inspect`.
- **No Credentials**: Reads only public pricing data.
- **Caching**: Price pages are cached in memory (24 hours by default).

## Installation

```bash
finfocus plugin install azure-public

# Or from the repository path
finfocus plugin install github.com/rshade/finfocus-plugin-azure-public
```

`finfocus setup` does not install this plugin by default.

## Supported Providers

The registry lists provider `azure`. The plugin prices resource types from both
Pulumi packages: classic `azure:` (for example
`azure:compute/linuxVirtualMachine:LinuxVirtualMachine`) and `azure-native:`
(for example `azure-native:compute:VirtualMachine`).

## Supported Resource Types

| Resource Type                        | Azure Service Name       | Example SKU    |
| ------------------------------------ | ------------------------ | -------------- |
| `compute/VirtualMachine`             | Virtual Machines         | `Standard_B1s` |
| `storage/ManagedDisk`                | Managed Disks            | `Premium_LRS`  |
| `storage/BlobStorage`                | Storage                  | `Hot LRS`      |
| `storage/StorageAccount`             | Storage                  | `Hot LRS`      |
| `web/AppServicePlan`                 | Azure App Service        | `P1v3`         |
| `web/FunctionApp`                    | Functions                | `Y1`           |
| `containerservice/KubernetesCluster` | Azure Kubernetes Service | `Standard`     |
| `sql/Database`                       | SQL Database             | `GP_Gen5_2`    |
| `cosmosdb/Account`                   | Azure Cosmos DB          | `400 RU`       |
| `network/LoadBalancer`               | Load Balancer            | `Standard`     |

Virtual machine scale sets are priced as the VM meter times the instance count.
Resource type matching is case-insensitive.

## Inputs

The plugin reads the region and SKU from the resource properties Pulumi sets
for each type (for example `location` and `vmSize`, `storageAccountType`, or
native `sku.name`). Usage inputs that change the monthly total are read from
tags, for example `size_gb`, `ru_per_second`, `rule_count`, and
`node_pool_1_sku` with `node_pool_1_count`. To see what a type needs, run:

```bash
finfocus plugin inspect azure-public azure-native:compute:VirtualMachine
```

## Usage

```bash
finfocus cost projected --pulumi-json plan.json
finfocus cost actual --pulumi-state state.json
```

## Limitations

- **List Prices Only**: Reservations, savings plans, and negotiated discounts
  are not applied to the monthly cost. VM quotes list them as advisory price
  options.
- **Actual Cost Is an Estimate**: `cost actual` returns list price times
  runtime, not your Azure bill.
- **Partial Coverage**: NAT Gateway, Cache for Redis, and PostgreSQL and MySQL
  servers are not priced. SQL Database supports General Purpose Gen5
  provisioned compute only.
- **Network Access**: The plugin calls `prices.azure.com`, so it needs outbound
  HTTPS.

See the
[plugin README](https://github.com/rshade/finfocus-plugin-azure-public#readme)
for per-type details.
