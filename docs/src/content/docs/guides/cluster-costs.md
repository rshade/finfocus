---
title: Kubernetes Cluster Cost Allocation
description: Break down a Kubernetes cluster's cost by namespace, controller, pod, node, or label.
---

## Overview

`finfocus cost cluster` allocates the cost of a [Kubernetes](https://kubernetes.io/docs/concepts/overview/)
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

## Several usage sources

The kubernetes plugin serves only the run rate and the prometheus plugin only
a historical window, so both can stay installed. When more than one usage
source is installed and `--usage-source` is not set, FinFocus asks each one,
in name order, and uses the one that accepts the request. A source that
rejects the request as invalid (`InvalidArgument`, which is how a source
declines a mode it cannot serve) is skipped. With both plugins installed, a
run with no window uses the kubernetes plugin and a run with `--from` uses
Prometheus.

When two sources accept the same request, FinFocus cannot choose and exits 2
with both names:

```text
several usage sources can serve run-rate usage (kubernetes, other); choose one with --usage-source
```

When every source rejects the request, the run exits 2 and the error lists
each rejection, for example a kubeconfig context that does not exist next to
Prometheus declining a run with no window. `--usage-source <name>` always
uses that plugin alone, with no other source asked, and its rejection exits 2
too:

```text
usage source kubernetes rejected the request: kubernetes usage source is run-rate only; use a metrics-backed usage source for history
```

## Spend by capacity type

When the usage source tags its nodes with `capacity_type` (the kubernetes and
prometheus plugins do, including Karpenter's `karpenter.sh/capacity-type`), the
table adds a section after the groups. This example is illustrative:

```text
CAPACITY TYPE  NODES  TOTAL   SHARE   IDLE   NOTES
on-demand      2      55.00   55.0%   3.00
spot           1      35.00   35.0%   3.00   spot node priced on-demand
unlabeled      1      10.00   10.0%   1.00
```

Each allocated row, idle rows included, counts once, under the capacity type
of its node (matched by cluster and node name; a usage source that sends no
cluster tag still matches, as in the allocator), so the rows sum to the cluster
total and idle cost is never counted twice. Rows that match no node, such as
the control plane, Fargate pods (which name a pod host that is not a priced
node), and orphan rows (which name a node that no longer exists), go under
`unlabeled`, as does
any node whose usage source sends no `capacity_type`; they are not folded into
on-demand. The kubernetes and prometheus plugins default a node with no spot
label to `on-demand`, so with them `unlabeled` holds only rows that match no node. Tag
values are shown as the plugin reports them. The section is omitted when no
row is labeled.

In run-rate mode a pricing plugin has no spot rate, so the allocator prices a
spot node at its on-demand rate and says so in the NOTES column. A historical
window carries actual spend and adds no such note.

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
  `pulumi_urns`. Run-rate mode only: over a `--from`/`--to` window every
  workload groups under `<none>` (see [Historical window](#historical-window))

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

## Namespace and selector scope

`--namespace payments` restricts allocation to one namespace, and
`--selector team=storefront` to pods with that label. Either one, or both,
scopes the report. Workload rows are exact for the selected pods. The
`__idle__` and `__cluster__` rows are omitted, because node capacity that
unselected pods use would otherwise be reported as idle. The footer names the
flags that scoped the report: `Idle: omitted (--namespace scoped)`,
`Idle: omitted (--selector scoped)`, or
`Idle: omitted (--namespace and --selector scoped)`.

## Allocation policy

The allocator's policy is resolved, first found wins (files never merge):

1. `--policy <file>`
2. `$PROJECT/.finfocus/allocation.hujson` (project config dir)
3. `~/.finfocus/allocation.hujson` (global config dir)
4. none — the plugin's built-in defaults

Files are HuJSON (comments and trailing commas allowed), standardized before
being passed to the allocator. A discovered file that fails to parse is a
fatal error — FinFocus never silently falls back to defaults. A policy file
that cannot be read or is not valid HuJSON exits 2 before any plugin starts,
and the message names the file and, for a syntax error, the line and column
(error code `config_invalid`), for example:

```text
allocation policy $PROJECT/.finfocus/allocation.hujson is not valid HuJSON: line 1, column 11: parsing value: unexpected EOF
```

`--show-policy` reports the same error. The allocator
checks a policy file before any cluster data is collected or priced, so an
unknown field or value exits 2 at once and the message names the allocator
and the file. See `plugins/kubernetes/README.md` for the policy fields.

`idle` and `system_workloads` default to `separate`. Set either to `share`
to fold unused node capacity, or kube-system and DaemonSet cost, into the
other workloads on the same node. The formula is locked in
`specs/614-allocator-policy-v2/spec.md`. `--show-policy` prints the
effective policy JSON, including whichever of those values is in effect,
and a digest that changes when they change.

`charge` is always `max-request-usage`. A workload's share of a node's CPU
cost is `max(cpu_request, cpu_usage)` over the node's allocatable CPU, and
memory works the same way. Which of the two inputs exists depends on the
usage source. The kubernetes source reports requests and no usage, so a
run-rate report charges requests. With the default `system_workloads: "separate"`,
a pod with no effective CPU or memory request (Kubernetes copies a limit into
a missing request) is charged `$0`, and unclaimed node cost stays in `__idle__`.
`idle: "share"` alone does not change that, because idle is shared in proportion
to existing workload cost. With `system_workloads: "share"`, zero-request
application pods can still receive an equal share of system cost when all
non-system recipients have zero weight.
The Prometheus source reports usage and no requests; see
[Historical window](#historical-window).

Print the effective policy (defaults plus overrides) and its digest without
contacting a cluster:

```bash
finfocus cost cluster --show-policy
```

Every report footer shows the policy source and digest, so any number can be
traced back to the exact policy that produced it.

## Automation: JSON, NDJSON, and MCP

`--output json` emits a single document with `mode`, `period`, `currency`,
`group_by`, `total`, `idle` (omitted when scoped), `namespace_scoped`
(true with `--namespace`), `selector_scoped` (true with `--selector`),
`incomplete`, `groups`, `capacity`, `priced`, `policy`, and `warnings`. The
report is scoped when either flag is true. `capacity` is an array (empty when
there are no rows) of `capacity_type`, `cpu_cost`, `mem_cost`, `total_cost`,
`idle_cost` (omitted when scoped, like the top-level `idle`), `nodes`, `rows`,
and `notes` (omitted when the group has none); the `total_cost` values sum to
`total`. With `--namespace` or `--selector`, the table shows `-` in the IDLE
column. In historical mode, `priced[].monthly` is the window total
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

### When another plugin prices the same workload

The [OpenCost plugin](../plugins/opencost.md) prices these workload types
too, from what the cluster has really cost: a 30-day trailing average of the
workload's OpenCost allocation, projected to 730 hours. With both plugins
installed, FinFocus offers the workload to one plugin at a time and uses the
first price it gets. A plugin that cannot price the workload declines and the
next one is asked.

Without a `routing` section in `config.hujson`, the order is the plugin
name, so `kubernetes` is asked before `opencost`:

| Kubernetes plugin | Workload priced by |
| --- | --- |
| Prices it: rates set, requests declared, and any DaemonSet or Job hint set | `kubernetes`, the configured-rate estimate |
| Declines it, for example a rate unset or no requests declared | `opencost`, the observed cost |

Setting the rates therefore replaces the observed cost of a workload OpenCost
already knows with the configured-rate estimate. The `adapter` field of the
JSON output names the plugin that priced each workload, and the note on the
result names the method.

To prefer the observed cost, give `opencost` a priority in `config.hujson`
(`~/.finfocus/config.hujson`, or `.finfocus/config.hujson` in the project):

```json
{
  "routing": {
    "plugins": [
      { "name": "opencost", "priority": 10 }
    ]
  }
}
```

A higher priority is asked first, and a plugin that is not listed keeps
priority 0. Fallback is on by default: when OpenCost has no data for the
workload, for example because it is not deployed yet, or cannot be reached,
the `kubernetes` estimate is used if that plugin can price it.
`finfocus config routes list` shows the rules in effect, and the
[routing guide](./routing.md) covers the rest of the configuration.

## Historical window

`--from` and `--to` price actual spend over that window. Both accept
`2006-01-02` and RFC3339. `--to` defaults to now when only `--from` is set.
An invalid window, `--group-by`, `--selector`, or `--output`, and a policy
file that cannot be read or parsed, exit 2 before any plugin starts. Input a
plugin rejects also exits 2: a `--usage-source` or `--allocator` that names
no installed plugin or is missing where several could serve, a request the usage source rejects as invalid (a
`--context` it cannot find, or a window it cannot serve), and a policy file
the allocator rejects. Other plugin, allocation, and conservation failures
exit 1.
With the kubernetes and prometheus plugins both installed, a window goes to
Prometheus without `--usage-source` (see
[Several usage sources](#several-usage-sources)):

```bash
export FINFOCUS_PROMETHEUS_URL=http://127.0.0.1:9090
finfocus cost cluster \
  --from 2026-09-28 \
  --to 2026-10-05
```

The footer mode is `historical` and the period is the window and its
length, for example `historical (2026-09-28 to 2026-10-05, 7 days)`. A
window whose bounds are not both UTC midnight prints RFC3339 bounds. A length
that is not whole days prints its largest unit and the next one, for example
`historical (2026-11-11T00:00:00Z to 2026-11-11T20:24:27Z, 20 hours 24 minutes)`;
smaller remainders are dropped, and a window under a minute is
`under 1 minute`. A run with no window stays `run-rate (monthly, 730 h)`.
When Prometheus holds no data for the start of the window, for example
because its retention is shorter than the window, the report is marked
incomplete with a warning naming the time stored data starts. The missing
part is not counted as zero usage. Pods with gaps in their samples are
reported the same way, one warning per pod and metric.

The footer says why a report is incomplete: what the usage source reported
missing, and, only when there are any, the resources that could not be
priced:

```text
Incomplete: the usage source reported partial data
  - stored data starts at 2026-10-27T06:00:20Z, after the window start 2026-10-01T00:00:00Z; Prometheus retention may be shorter than the window
  - gap in cpu_usage for 20 resources
  - gap in mem_usage for 20 resources
```

On stderr, warnings that repeat one problem for many resources are folded
into one line with a count and a few examples, after the store-wide
warnings. The `warnings` field of `--output json` and `--output ndjson`
lists every warning as the plugin wrote it.

A historical window charges what workloads consumed. The Prometheus source
reports `cpu_usage` and `mem_usage` in core-hours and GiB-hours over the
window, and no requests, so `max-request-usage` reduces to usage: a
workload's share is its consumed core-hours over the node's allocatable
core-hours for the window. A pod that requested a full core and used almost
none costs almost nothing, and the rest of the node's cost goes to
`__idle__`. The policy and its digest are the same in both modes, but the
inputs differ: a namespace's share of the node cost can be much smaller over
a window than in a run-rate report, and `__idle__` is larger when workloads
use less than they request.

Prometheus must be scraping cAdvisor and kube-state-metrics, including the
node-label metrics `kube_node_labels` and `kube_node_info`. Outside a
cluster, set `FINFOCUS_PROMETHEUS_URL`. Inside a cluster, an unset URL uses
the Prometheus Operator service. The
[plugin README](https://github.com/rshade/finfocus/blob/main/plugins/prometheus/README.md)
lists the address, the bearer token, and the install gate.

A node's provider, instance type, and region come from those recorded
series. When they lack a value the node needs, the source reads the node as
it is now through your kubeconfig and prices it with the current labels. The
report then carries a warning that names the node and the fields read from
the live node, such as
`node ip-10-0-1-5: instance type, region taken from the live node, ...`.
A node that was replaced or relabeled after the window can be priced with
its new instance type. The warning does not mark the report incomplete.

Pod labels come from kube-state-metrics, which exports only the labels named
in its `--metric-labels-allowlist` flag and replaces every character that is
not a letter, digit, or underscore with `_`. A historical `--group-by` uses
that recorded key: `app.kubernetes.io/name` is
`label:app_kubernetes_io_name`, and `--group-by label:app.kubernetes.io/name`
puts every pod under `<none>`. The `--selector` flag accepts either form.

The Prometheus source does not read pod annotations, so
`--group-by pulumi-stack` has no `finfocus.dev/pulumi-urn` to parse over a
window: every workload groups under `<none>` and JSON and NDJSON groups have
no `pulumi_urns`. Adding the annotation to the kube-state-metrics
`--metric-annotations-allowlist` does not change this, because the source
does not query `kube_pod_annotations`. To group by Pulumi stack, run without
`--from` and `--to` (with `--usage-source kubernetes` when several sources are
installed), so the kubernetes source reads the annotation from the live pods.

## Plugin call deadlines

Each usage-source and allocator call has its own deadline, so a plugin that
stops answering cannot hang `cost cluster` or overview cluster expansion:

| Call | Deadline |
| --- | --- |
| `GetStats` with no window | 2 minutes |
| `GetStats` over a window | 2 minutes plus 30 seconds per started day, at most 30 minutes |
| `Allocate` policy check and `--show-policy` | 10 seconds |
| `Allocate` with usage | 2 minutes |

A 7-day window gets 5 minutes 30 seconds, a 41-day window 22 minutes 30
seconds, and a window of 56 days or more 30 minutes. The window deadline
grows because a metrics source evaluates each query over the whole window.
The prometheus plugin sends 15 queries in sequence, and Prometheus stops
each one at its default `--query.timeout` of 2 minutes. The policy check
only reads the policy.

A call that runs out its deadline ends the run with exit 1 and names the
plugin and the call, for example
`get usage stats: usage source prometheus did not answer GetStats within 22m30s`.
FinFocus then stops the plugin processes. With several usage sources and no
`--usage-source`, a source that runs out its deadline stops the selection, and
later sources are not asked. Ctrl-C still stops the run at once. In the
overview, a deadline skips live expansion for that cluster, keeps its
projected rows, and adds a `live cluster expansion skipped` note.

Pricing calls keep the engine's own per-resource limits.

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
