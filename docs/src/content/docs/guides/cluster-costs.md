---
title: Kubernetes Cluster Cost Allocation
description: Break down a Kubernetes cluster's monthly cost by namespace, controller, pod, node, or label with the kubernetes plugin.
---

## Overview

`finfocus cost cluster` allocates the monthly run-rate cost of a Kubernetes
cluster's nodes (and, on EKS, its control plane and each Fargate pod) across
the workloads running on them. A usage-source plugin reports live cluster state, FinFocus prices the
reported nodes through its normal pricing plugins, and an allocator plugin
splits each node's cost by workload resource requests. By default, unused
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

`__idle__` (unclaimed node capacity) and `__cluster__` (shared infrastructure
such as the control plane) always stay separate groups. Groups sort by total
cost descending.

```bash
finfocus cost cluster --group-by controller
finfocus cost cluster --group-by label:team
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
`warnings`. `--output ndjson` emits a `summary` line followed by one `group`
line per group. The command is also exposed as an MCP tool (`finfocus
mcp-server`), so agents can call it like any other read-only command.

## Limitations

- Run-rate only: historical allocation (`STATS_MODE_HISTORICAL`) is rejected
  with "historical usage is not supported yet" until the Prometheus usage
  source lands.
- Spot nodes are priced on-demand. Each EKS Fargate pod is priced on its own
  from its vCPU and memory request when the pricing plugin returns a positive
  monthly cost; otherwise the pod is a $0 row with a note. kind cannot
  simulate Fargate, so `make test-e2e-kind` does not cover that path. The
  rates live in
  [finfocus-plugin-aws-public#409](https://github.com/rshade/finfocus-plugin-aws-public/issues/409).
- A node whose price resolves to `$0` (for example an unknown instance type in
  aws-public) is treated as unpriced, never as free; if no node can be priced
  at all the command fails.
