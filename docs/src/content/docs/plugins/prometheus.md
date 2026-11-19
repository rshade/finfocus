---
title: Prometheus Plugin
description: Read historical Kubernetes CPU and memory usage from Prometheus for cluster cost over a window.
parent: Plugins
nav_order: 17
---

## Overview

The `prometheus` plugin is a usage source for `finfocus cost cluster`. It
reads the CPU and memory that workloads used over a past window from
[Prometheus](https://prometheus.io/docs/introduction/overview/), so FinFocus
can price that window. It doesn't price resources and it doesn't allocate
cost: the kubernetes plugin still allocates, and a pricing plugin such as
[aws-public](aws-public.md) prices the nodes. The
[cluster cost guide](../guides/cluster-costs.md) covers the whole setup.

The plugin serves a historical window only (`--from` and `--to`). It declines
a run with no window, which the kubernetes plugin serves, so both plugins can
stay installed. See
[Several usage sources](../guides/cluster-costs.md#several-usage-sources).

## Installation

```bash
finfocus plugin install kubernetes
finfocus plugin install prometheus

# Or pin a release
finfocus plugin install prometheus@v0.1.7
```

Releases are tagged `prometheus-vX.Y.Z` in the FinFocus repository.
`finfocus plugin update prometheus` moves to the newest of them.
`finfocus setup` doesn't install this plugin.

## Configuration

```bash
export FINFOCUS_PROMETHEUS_URL=http://127.0.0.1:9090
```

| Variable                           | Purpose                                                    |
| ---------------------------------- | ---------------------------------------------------------- |
| `FINFOCUS_PROMETHEUS_URL`          | Prometheus base URL.                                       |
| `FINFOCUS_PROMETHEUS_BEARER_TOKEN` | Optional bearer token sent with every query. It's secret. |

When `FINFOCUS_PROMETHEUS_URL` is unset and the plugin runs inside a cluster
(`KUBERNETES_SERVICE_HOST` is set), it uses the Prometheus Operator service,
`http://prometheus-operated.monitoring.svc:9090`. Outside a cluster, an unset
URL fails the run with `set FINFOCUS_PROMETHEUS_URL to the Prometheus base
URL`.

The plugin advertises both values as the settings `url` and `bearer_token`,
so they can also be set under `plugins.prometheus` in the configuration file
or on the dashboard's Plugins page. The token isn't written to logs or
errors, and it's sent only to the configured scheme and host.

## Required Metrics

Prometheus must scrape cAdvisor and kube-state-metrics over the whole window.

| Source             | Series                                                                                                   |
| ------------------ | -------------------------------------------------------------------------------------------------------- |
| cAdvisor           | `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes`                                |
| kube-state-metrics | `kube_node_status_allocatable`, `kube_node_labels`, `kube_node_info`                                     |
| kube-state-metrics | `kube_pod_info`, `kube_pod_labels`, `kube_pod_owner`, `kube_replicaset_owner`, `kube_job_owner`          |

The node series give each node's provider, instance type, and region, which
pricing needs. When they lack a value, the plugin reads the node's current state
through your kubeconfig and adds a warning that names the node and the fields
it took from the live node. A node that neither the recorded series nor the
live node identifies is omitted.

## Usage Data

Each workload row carries `cpu_usage` and `mem_usage` over the window, in
core-hours and GiB-hours. Each node row carries allocatable CPU and memory
over the same window. The plugin reports no requests, so the default
`max-request-usage` policy charges a workload for what it used. Capacity that
no workload used stays on the node's `__idle__` row.

```bash
finfocus cost cluster --from 2026-09-28 --to 2026-10-05
```

The report footer reads `historical (2026-09-28 to 2026-10-05, 7 days)`.

## Limitations

- **Retention**: When Prometheus holds no data for the start of the window,
  the report is marked incomplete with a warning naming the time stored data
  starts. The missing part isn't counted as zero usage.
- **Gaps**: A pod whose samples miss more than one 60-second step marks the
  report incomplete and adds one warning per pod and metric.
- **Pod labels**: kube-state-metrics exports only the labels named in its
  `--metric-labels-allowlist` flag, and replaces characters other than
  letters, digits, and underscores with `_`. Group by the recorded key, for
  example `label:app_kubernetes_io_name`.
- **Pod annotations**: The plugin doesn't read pod annotations, so
  `--group-by pulumi-stack` puts every workload under `<none>` over a window.
  Group by Pulumi stack in a run-rate report instead.
- **Query time**: The plugin sends its queries one after another, and
  Prometheus stops each one at its `--query.timeout` (2 minutes by default).
  FinFocus gives the whole call a deadline that grows with the window. See
  [Plugin call deadlines](../guides/cluster-costs.md#plugin-call-deadlines).

See the [cluster cost guide](../guides/cluster-costs.md#historical-window) for
the historical window in full, and the
[plugin README](https://github.com/rshade/finfocus/tree/main/plugins/prometheus)
for the node-label fallback and log output.
