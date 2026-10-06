---
title: Kubernetes Cluster Cost Allocation
description: Break down a Kubernetes cluster's cost by namespace, controller, pod, node, or label.
---

## Overview

`finfocus cost cluster` allocates the cost of a Kubernetes
cluster's nodes (and, on EKS, its control plane and each Fargate pod) across
the workloads running on them. With no window, that cost is the monthly run
rate. With `--from` and `--to`, it is the actual spend over that window.
A usage-source plugin reports cluster state, FinFocus prices the
reported nodes through its normal pricing plugins, and an allocator plugin
splits each node's cost across the workloads. By default, unused
capacity is its own idle row. An allocation policy can set `idle` or
`system_workloads` to `share`, which folds that cost into the workloads on
the same node. Every run enforces a conservation invariant: allocated rows
must sum to the priced total.

## Prerequisites

- A reachable cluster via kubeconfig (`KUBECONFIG`, `~/.kube/config`, or
  in-cluster config). Read access requires list on nodes, pods, ReplicaSets,
  and Jobs — see the minimal ClusterRole in
  `plugins/kubernetes/deploy/clusterrole.yaml`.
- The kubernetes plugin (usage source and allocator):

  ```bash
  finfocus plugin install kubernetes
  ```

- A pricing plugin for the nodes' provider and region, for example
  aws-public pinned to the nodes' region:

  ```bash
  finfocus plugin install aws-public --metadata region=us-east-1
  ```

## First run

```bash
finfocus cost cluster
```

```text
GROUP          CPU      MEMORY   TOTAL    NOTES
payments       40.00    20.00    60.00    spot node priced on-demand
search         2.50     2.50     5.00
__idle__       0.00     0.00     7.00
__cluster__    73.00    0.00     73.00

Mode:    run-rate (monthly, 730 h)
Total:   $145.00 USD
Idle:    $7.00 (4.8%)
Policy:  built-in defaults · 3f2a9c1b4d5e
```

The footer reports the mode and period, the priced total, idle cost and its
share, the policy source with a short digest, and any resources that could not
be priced.

## Grouping

`--group-by` selects the aggregation dimension (default `namespace`):

- `namespace` — one group per namespace
- `controller` — `namespace/controller_kind/controller` (for example
  `payments/Deployment/api`)
- `pod` — `namespace/pod`
- `node` — node name; idle rows group under their node
- `label:<key>` — the value of pod label `<key>`; pods without the label group
  under `<none>` and are never dropped
- `pulumi-stack` — `<stack>/<project>` parsed from the pod annotation
  `finfocus.dev/pulumi-urn`. A missing or malformed URN groups under `<none>`
  and the pod is still counted. JSON and NDJSON groups list those URNs in
  `pulumi_urns`

`__idle__` (unclaimed node capacity) and `__cluster__` (shared infrastructure
such as the control plane) always stay separate groups. Groups sort by total
cost descending.

```bash
finfocus cost cluster --group-by controller
finfocus cost cluster --group-by label:team
finfocus cost cluster --group-by pulumi-stack
```

Use `--context` to pick a kubeconfig context, and `--selector key=value`
(repeatable) to restrict pods by label.

## Namespace scope

`--namespace payments` restricts allocation to one namespace. Workload rows
are exact for that namespace; the `__idle__` and `__cluster__` rows are
omitted and the footer says so (`Idle: omitted (--namespace scoped)`).

## Allocation policy

The allocator's policy is resolved, first found wins (files never merge):

1. `--policy <file>`
2. `$PROJECT/.finfocus/allocation.hujson` (project config dir)
3. `~/.finfocus/allocation.hujson` (global config dir)
4. none — the plugin's built-in defaults

Files are HuJSON (comments and trailing commas allowed), standardized before
being passed to the allocator. A discovered file that fails to parse is a
fatal error — FinFocus never silently falls back to defaults. See
`plugins/kubernetes/README.md` for the policy fields.

`idle` and `system_workloads` default to `separate`. Set either to `share`
to fold unused node capacity, or kube-system and DaemonSet cost, into the
other workloads on the same node. The formula is locked in
`specs/614-allocator-policy-v2/spec.md`. `--show-policy` prints the
effective policy JSON, including whichever of those values is in effect,
and a digest that changes when they change.

Print the effective policy (defaults plus overrides) and its digest without
contacting a cluster:

```bash
finfocus cost cluster --show-policy
```

Every report footer shows the policy source and digest, so any number can be
traced back to the exact policy that produced it.

## Automation: JSON, NDJSON, and MCP

`--output json` emits a single document with `mode`, `period`, `currency`,
`group_by`, `total`, `idle` (omitted when namespace-scoped),
`namespace_scoped`, `incomplete`, `groups`, `priced`, `policy`, and
`warnings`. In historical mode, `priced[].monthly` is the window total
(`TotalCost`), not a 730-hour projection. Each group may include `pulumi_urns` when a workload in it
carries annotation `finfocus.dev/pulumi-urn`. `--output ndjson` emits a `summary` line followed by one `group`
line per group. The command is also exposed as an MCP tool (`finfocus
mcp-server`), so agents can call it like any other read-only command.

## Price workloads declared in a Pulumi plan

`cost cluster` prices a running cluster. Before a workload is deployed, the
same plugin can estimate it from the plan: `cost projected` prices
Deployments, StatefulSets, DaemonSets, Jobs, and CronJobs from their declared
resource requests and rates you set. No cluster connection is needed.

```bash
export FINFOCUS_KUBERNETES_CPU_HOURLY_RATE=0.04         # USD per vCPU-hour
export FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE=0.005 # USD per GiB-hour
export FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT=4       # optional, DaemonSets
export FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH=10       # optional, Jobs and CronJobs

pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json
```

A Deployment with `replicas: 3` and one container requesting `cpu: 500m` and
`memory: 1Gi` costs `3 × (0.5 × 0.04 + 1 × 0.005) × 730 = 54.75` USD a month.
The note on each result names the method and the pod count source.

Without the rates, each workload shows `NO_COST_DATA` with a note naming the
missing variable, never a `$0` price. A DaemonSet without a node count, or a
Job or CronJob without hours, is reported the same way. Other `kubernetes:*`
types such as `ConfigMap` stay declined.

These are estimates from configured rates, useful for comparing changes in a
pull request. `cost cluster` remains the authoritative number for a running
cluster. The variables, the decline reasons, and the request rules are in the
[plugin README](https://github.com/rshade/finfocus/blob/main/plugins/kubernetes/README.md#projected-cost-from-a-pulumi-plan).

## Historical window

`--from` and `--to` price actual spend over that window. Both accept
`2006-01-02` and RFC3339. `--to` defaults to now when only `--from` is set.
Select Prometheus when more than one usage source is installed:

```bash
export FINFOCUS_PROMETHEUS_URL=http://127.0.0.1:9090
finfocus cost cluster \
  --usage-source prometheus \
  --from 2026-09-28 \
  --to 2026-10-05
```

The footer mode is `historical` and the period is the window and its
length, for example `historical (2026-09-28 to 2026-10-05, 7 days)`. A
window that is not whole UTC days prints RFC3339 bounds and hours. A run with no window stays `run-rate (monthly, 730 h)`.
When Prometheus holds no data for the start of the window, for example
because its retention is shorter than the window, the report is marked
incomplete with a warning naming the time stored data starts. The missing
part is not counted as zero usage.

Prometheus must be scraping cAdvisor and kube-state-metrics, including the
node-label metrics `kube_node_labels` and `kube_node_info`. Outside a
cluster, set `FINFOCUS_PROMETHEUS_URL`. Inside a cluster, an unset URL uses
the Prometheus Operator service. The
[plugin README](https://github.com/rshade/finfocus/blob/main/plugins/prometheus/README.md)
lists the address, the bearer token, and the install gate.

Pod labels come from kube-state-metrics, which exports only the labels named
in its `--metric-labels-allowlist` flag and replaces every character that is
not a letter, digit, or underscore with `_`. A historical `--group-by` uses
that recorded key: `app.kubernetes.io/name` is
`label:app_kubernetes_io_name`, and `--group-by label:app.kubernetes.io/name`
puts every pod under `<none>`. The `--selector` flag accepts either form.

## Limitations

- The kubernetes usage source still rejects a window with its run-rate-only
  error. A window needs a source that reports historical mode.
- On the no-window path, spot nodes are priced on-demand. Each EKS Fargate pod
  is priced on its own from its vCPU and memory request when the pricing
  plugin returns a positive monthly cost; otherwise the pod is a $0 row with
  a note. kind cannot simulate Fargate, so `make test-e2e-kind` does not
  cover that path. The rates live in
  [finfocus-plugin-aws-public#409](https://github.com/rshade/finfocus-plugin-aws-public/issues/409).
- A node whose price resolves to `$0` (for example an unknown instance type in
  aws-public) is treated as unpriced, never as free; if no node can be priced
  at all the command fails.
