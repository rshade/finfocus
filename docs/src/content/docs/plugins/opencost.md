---
title: OpenCost Plugin
description: Report Kubernetes allocation cost from an OpenCost endpoint, with an experimental Kubecost profile.
parent: Plugins
nav_order: 12
---

## Overview

The `opencost` plugin reads Kubernetes allocation cost over HTTP from an
[OpenCost](https://www.opencost.io/) endpoint and serves it to FinFocus. It
reports what the cluster actually spent per namespace, controller, pod, or
node, so it is the source for Kubernetes actual cost.

It needs a reachable OpenCost allocation API. It does not read cloud provider
credentials. With the default `opencost` profile it sends no token.

## Features

- **Actual Costs**: `totalCost` from the allocation rows that match the
  resource and the request window. A matching row with zero cost returns zero.
  No matching row is `NotFound`.
- **Projected Costs**: A 30-day trailing average projected to a 730-hour
  month. The response says so in `billing_detail`.
- **Pricing Specs, Estimate, and Batch**: `GetPricingSpec` uses the same
  30-day data. `EstimateCost` and batch cost are also available. See the plugin
  README for how each profile answers them.
- **Budgets**: `GetBudgets` is available for the `kubecost` profile.
- **Caching and Limits**: A successful allocation query is reused for 30
  seconds by default, and outbound requests are rate limited.

## Installation

```bash
finfocus plugin install opencost

# Or from the repository path
finfocus plugin install github.com/rshade/finfocus-plugin-opencost
```

`finfocus setup` does not install this plugin by default.

## Configuration

Point the plugin at your allocation API and, optionally, a config file:

```bash
export KUBECOST_BASE_URL=http://localhost:9003
export OPENCOST_CONFIG=/path/to/config.yaml
```

| Setting             | Environment variable      | Default    | Effect                                              |
| ------------------- | ------------------------- | ---------- | --------------------------------------------------- |
| `baseUrl`           | `KUBECOST_BASE_URL`       | empty      | Origin of the allocation API. Queries need it.      |
| `profile`           | `OPENCOST_PROFILE`        | `opencost` | `opencost` or `kubecost`.                           |
| `currency`          | `OPENCOST_CURRENCY`       | empty      | ISO 4217 code when the data names no single one.    |
| `timeout`           | `KUBECOST_TIMEOUT`        | `15s`      | HTTP client timeout.                                |
| `tlsSkipVerify`     | `KUBECOST_TLS_SKIP_VERIFY`| `false`    | Skips certificate checks. Leave it off on shared clusters. |

The API token is read only from `KUBECOST_API_TOKEN` and is never written to
logs. See the
[plugin README](https://github.com/rshade/finfocus-plugin-opencost#configuration-reference)
for every key.

## Supported Providers

The registry lists provider `kubernetes`.

## Supported Resource Types

| Resource Type                              | Resource id                         |
| ------------------------------------------ | ----------------------------------- |
| `kubernetes:core/v1:Namespace`             | `namespace/<name>`                  |
| `kubernetes:apps/v1:Deployment`, `StatefulSet`, `DaemonSet`, `ReplicaSet`, `kubernetes:batch/v1:Job`, `CronJob` | `controller/<namespace>/<name>` |
| `kubernetes:core/v1:Pod`                   | `pod/<namespace>/<name>`            |
| `kubernetes:core/v1:Node`                  | `node/<name>`                       |

The short types `k8s-namespace`, `k8s-pod`, `k8s-node`, and `k8s-controller`
also work. `kubernetes:core/v1:Service` is not supported.

## Usage

```bash
finfocus cost actual --pulumi-state state.json
finfocus cost projected --pulumi-json plan.json
```

## Limitations

- **Needs a Backend**: Without a reachable OpenCost endpoint there are no
  results.
- **Trailing Average**: Projected cost extends the last 30 days of observed
  spend. It does not model a planned change.
- **Kubecost Profile Is Experimental**: Its responses are contract fixtures and
  are not verified against a live Kubecost.
- **No Allocation Service**: The plugin does not implement `AllocatorService`.
- **Idle and Shared Cost**: Requests exclude idle and shared cost.

See the
[plugin README](https://github.com/rshade/finfocus-plugin-opencost#readme) for
per-method details.
