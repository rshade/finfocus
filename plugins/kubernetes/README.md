# Kubernetes Cost Allocation Plugin

Splits Kubernetes cluster cost across namespaces, workloads, and nodes by
combining live cluster state with FinFocus's routed pricing plugins. It
reports both a run-rate estimate today (no historical billing data) and a
per-node accounting of unused (idle) capacity.

## What It Does

The plugin has two halves that together implement FinFocus's
`GetStats`/`Allocate` plugin contract (`finfocus-spec` v0.6.2 `pluginsdk`):

1. **Usage collection** (`usage.Collect`) lists nodes, pods, ReplicaSets, and
   Jobs from a live cluster and reports **run-rate** usage
   (`StatsMode_STATS_MODE_RUN_RATE`, a snapshot of the cluster as it is right
   now, not a historical time series): each node's allocatable CPU/memory,
   and each running pod's *effective requests* (the Kubernetes scheduling
   rule for containers, sidecars, and init containers, plus pod overhead).
   Nodes that can be priced (see [Node Pricing
   Requirements](#node-pricing-requirements)) and, on EKS, the cluster's
   control plane are reported as priceable resources for FinFocus's normal
   pricing plugins to price.
2. **Allocation** (`allocate.Allocate`) takes those priced resources plus the
   collected usage and splits each node's cost across the workloads
   scheduled on it, in proportion to each workload's share of the node's
   CPU/memory. Unused node capacity is reported as a separate idle row.

The repository contains the `policy`, `allocate`, and `usage` Go packages,
their tests, and the plugin's gRPC entry point itself: `cmd/main.go` serves
the plugin through `pluginsdk`, declaring explicit
`PLUGIN_CAPABILITY_USAGE_STATS` and `PLUGIN_CAPABILITY_ALLOCATION`
capabilities (see `Info()` in `plugin.go`) so hosts never route pricing
queries here. The `finfocus cost cluster` command drives the plugin end to
end — usage collection, node pricing, and allocation (see
[How It's Used](#how-its-used)). The sections below describe
the plugin's behavior and interfaces.

## Install

From the plugin registry:

```bash
finfocus plugin install kubernetes
```

Or build and install from the `finfocus` repository root:

```bash
make install-kubernetes
```

This builds the plugin and installs it to `~/.finfocus/plugins/kubernetes/<version>/`,
the same pattern as `make install-recorder`.

## Kubeconfig and Cluster Context

The plugin connects using standard `client-go` kubeconfig loading
(`KubeconfigClusters` in `kubeconfig.go`): the `KUBECONFIG` environment
variable if it's set, otherwise `~/.kube/config`, otherwise in-cluster
config when running as a pod. The plugin's own binary (`cmd/main.go`) takes
no `--context`/`--namespace` flags at all — every request-scoped choice
comes from the `GetStatsRequest` the host sends (`plugin.go`):

- **Kube context**: `GetStatsRequest.scope`. An empty scope uses the
  kubeconfig's current context; a non-empty scope selects that context by
  name. A scope naming a context that doesn't exist in the kubeconfig
  returns an `InvalidArgument` error naming the context and the kubeconfig
  paths that were searched. A cluster with no usable kubeconfig at all (not
  running in-cluster, and no config found via `KUBECONFIG` or
  `~/.kube/config`) similarly returns an `InvalidArgument` error that names
  `KUBECONFIG` and the searched paths.
- **Namespace**: the `"namespace"` key of `GetStatsRequest.selector`.
  Missing or empty collects cluster-wide.
- **Label filters**: every other key in `GetStatsRequest.selector` is
  treated as an exact-match pod label filter (`key=value`, ANDed together
  in sorted order) and passed straight through to the pods `List` call.
  Keys and values are validated client-side before the call; an invalid
  key or value (for example, one containing `,` or `=`) returns an
  `InvalidArgument` error naming the offending key instead of silently
  producing a corrupted selector.

The host is what fills in `scope` and `selector`, not the plugin itself.

When the plugin runs in-cluster (no kubeconfig, using the pod's service
account) with no explicit `scope`, `Context` resolves to `""`, so every usage
row and priceable resource carries an empty `cluster` subject value; and
because the in-cluster API server host is the internal service address, not
the public EKS endpoint, [EKS control-plane detection](#node-pricing-requirements)
never matches and no control-plane row is produced.

## How It's Used

Once installed, the plugin itself is ready to serve — it's a normal
`pluginsdk` gRPC plugin (`cmd/main.go`) declaring the `USAGE_STATS` and
`ALLOCATION` capabilities. `finfocus cost cluster` drives it automatically:
FinFocus calls the plugin's `GetStats` RPC to collect run-rate node and pod
usage, routes the priceable nodes and control plane through FinFocus's normal
cost-pricing plugins, then calls the plugin's `Allocate` RPC with the priced
resources and usage rows to get back per-workload allocation rows.

## Required RBAC

The plugin only **lists** four resource kinds — it never watches, gets,
patches, or otherwise mutates cluster state:

| Resource                | Why                                                        |
| ------------------------ | ----------------------------------------------------------- |
| `nodes`                  | Node capacity and pricing labels.                          |
| `pods`                   | Workload requests and node placement.                      |
| `apps/replicasets`       | Resolves a pod to its owning Deployment (or similar).      |
| `batch/jobs`             | Resolves a pod to its owning CronJob (or similar).          |

Apply the minimal ClusterRole in [`deploy/clusterrole.yaml`](deploy/clusterrole.yaml)
and bind it to the plugin's service account:

```bash
kubectl apply -f plugins/kubernetes/deploy/clusterrole.yaml
kubectl create clusterrolebinding finfocus-reader \
  --clusterrole=finfocus-reader \
  --serviceaccount=<namespace>:<service-account>
```

If the bound identity is missing a permission, collection fails with a
`PermissionDenied` error naming the exact resource and scope, for example:

```text
kubernetes RBAC: cannot list pods cluster-wide (grant list on pods; see docs for the minimal ClusterRole): ...
```

That message is exactly what this ClusterRole grants — add the `list`
permissions above for the named resource and the error goes away. A
`namespace`-scoped collection reports "in namespace `<ns>`" instead of
"cluster-wide" in the same message.

## Node Pricing Requirements

A node is only added to the priceable set — and therefore only gets a real
cost, instead of an idle-only allocation — if the plugin can determine its
provider, instance type (SKU), and region from labels:

- **Instance type**: `node.kubernetes.io/instance-type`, falling back to the
  deprecated `beta.kubernetes.io/instance-type`.
- **Region**: `topology.kubernetes.io/region`, falling back to the
  deprecated `failure-domain.beta.kubernetes.io/region`.
- **Provider**: the `finfocus.dev/provider` label, if set, always wins.
  Otherwise the provider is derived from the node's `spec.providerID`
  scheme: `aws://` → `aws`, `gce://` → `gcp`, `azure://` → `azure`.

If any of the three can't be determined, the node still contributes its
capacity rows (`cpu_allocatable`/`mem_allocatable`) and a warning is
emitted — `node <name>: cannot determine provider, instance type, or
region; not priced` — but the node itself is never added to the priceable
set. With no priced entry for it, the node gets **no** `__idle__` row and
**no** cost, and every pod scheduled on it is reported as a $0 `workload`
row with the note `node <name> not found in priced resources` — the same
orphan path used when a node has simply gone missing by the time pricing
runs (see [Allocation Rows](#allocation-rows)).

AWS nodes are priced under resource type `aws:ec2/instance:Instance`; every
other provider uses `<provider>:compute/instance`.

**Spot detection**: a node is tagged `spot` (driving the "spot node priced
on-demand" allocation note below) when either
`eks.amazonaws.com/capacityType` or `karpenter.sh/capacity-type` is set to
`spot`/`SPOT` (case-insensitive).

**Fargate detection**: a node with `eks.amazonaws.com/compute-type=fargate`
is an EKS Fargate virtual node and is skipped from collection entirely —
see [Limitations](#limitations).

**EKS control plane**: when `usage.Options.APIServerHost` is set and matches
the EKS API server hostname pattern (`*.<region>.eks.amazonaws.com`, or
`*.<region>.eks.amazonaws.com.cn` in the China partition), the cluster's
control plane is added as a priceable `aws:eks/cluster:Cluster` resource,
priced separately from the nodes.

## Allocation Policy

The allocator's behavior is controlled by a JSON (or Hujson) policy document
resolved the same way as other FinFocus configuration: a project-level
`.finfocus/allocation.hujson` overrides the global
`~/.finfocus/allocation.hujson`, and `finfocus cost cluster --policy <path>`
overrides both.

The document is decoded strictly onto the built-in defaults
(`policy.Defaults()`): unknown fields are rejected and reported with their
JSON path (for example `unknown field "node_split.cpu"`), and unsupported
values are rejected too. Nested objects merge field by field — overriding
`node_split.cpu_core_hour` alone leaves `node_split.method` and
`node_split.mem_gib_hour` at their defaults. **Lists replace, not append**:
none of the fields below are lists today, but any list-valued field added in
a future policy version follows this rule — supplying a list in an override
document replaces the default list wholesale, it never merges into it.

| Field                    | Type   | Default              | Supported values          |
| ------------------------- | ------ | --------------------- | --------------------------- |
| `version`                 | int    | `1`                    | `1` (the only schema version this plugin understands) |
| `idle`                    | string | `"separate"`           | `"separate"`, `"share"`     |
| `system_workloads`        | string | `"separate"`           | `"separate"`, `"share"`     |
| `node_split.method`       | string | `"unit-price-ratio"`   | `"unit-price-ratio"`        |
| `node_split.cpu_core_hour`| float  | `0.031611` ($/vCPU-hour) | any value `>= 0`          |
| `node_split.mem_gib_hour` | float  | `0.004237` ($/GiB-hour)  | any value `>= 0`          |
| `charge`                  | string | `"max-request-usage"`  | `"max-request-usage"`       |
| `control_plane`           | string | `"separate"`           | `"separate"`                |
| `spot_nodes`              | string | `"on-demand-with-note"`| `"on-demand-with-note"`     |

`idle` and `system_workloads` each accept `"separate"` (the default) or
`"share"`. Every other field except the two `node_split` unit prices
supports exactly one value; supplying anything else is rejected. The share
formula is locked in `specs/614-allocator-policy-v2/spec.md`.

`node_split` controls how much of a node's price is attributed to CPU versus
memory: each node's `cpu_core_hour`/`mem_gib_hour` weights are multiplied by
its allocatable CPU/memory to get a CPU weight and a memory weight, and the
node's cost is split between them in that ratio (if both weights are zero,
the full cost is treated as CPU cost). `charge` then splits each side of
that cost across the node's workloads in proportion to
`max(cpu_request, cpu_usage)` / `max(mem_request, mem_usage)` against the
node's allocatable capacity, scaled down if the node is oversubscribed so no
workload's share exceeds 100%. A non-finite (`NaN`/`±Inf`) or negative usage
amount from an upstream usage row is always treated as `0`, so a malformed
row can't corrupt shares or break conservation (the guarantee that every
allocation response's rows sum to the priced total).

`idle: "share"` then adds each dimension's leftover (the part no workload
claimed) onto that node's workload rows in proportion to the CPU cost or
memory cost those rows already hold. A dimension whose workload costs are
all zero stays on the idle row. The idle row is still emitted: the
allocation contract requires exactly one per priced node, so a fully shared
node reports an idle row of `$0`.

`system_workloads: "share"` treats a workload as system when its namespace
is `kube-system` or its controller kind is `DaemonSet`. That cost is moved
onto the other workloads on the same node, in proportion to their existing
CPU cost and memory cost, and the system rows are dropped. If the other
workloads have no cost in a dimension, that dimension is split evenly across
them. A node with no non-system workload keeps its system rows. When both
fields are `"share"`, idle is folded into every workload first (including
system workloads), and system rows are folded after that. Sharing never
crosses nodes. `finfocus cost cluster --show-policy` prints the effective
JSON, so `"idle": "share"` shows up there with a digest that changes with it.

## Allocation Rows

`Allocate` returns one `AllocationRow` per subject. The `kind` subject key
identifies the row type:

- **`workload`** — one row per pod, with `cpu_cost`/`mem_cost`/`total_cost`
  for its share of the node it runs on.
- **`__idle__`** — one row per priced node, holding the portion of that
  node's cost not claimed by any workload (`node`/`cluster` subject keys
  identify which node). With `idle: "share"` that portion is usually `$0`,
  and the row is still present. No idle row is emitted for a node that never made
  it into the priced set at all (see [Node Pricing
  Requirements](#node-pricing-requirements)), nor for the one edge case
  where a node does have a priced entry but it's marked unpriced with an
  empty resource id: an idle row requires a non-empty `node` subject key,
  and only an unpriced entry's id can ever be empty (a *priced* entry is
  always validated to have a non-empty id), so there is nothing meaningful
  to attach the row to. Either way the node's cost is $0, so nothing is
  lost.
- **`__cluster__`** — one row per priced resource that isn't a node (today,
  only the EKS control plane), holding that resource's own cost.

### Notes

Individual rows carry a `note` explaining anything non-standard about their
cost:

| Note                                             | When                                                                 |
| -------------------------------------------------- | ----------------------------------------------------------------------- |
| `node <name> has no price`                       | The pricing plugin returned an entry for the node but marked it unpriced. Applies to that node's workload rows and its idle row. |
| `spot node priced on-demand`                     | The node's `capacity_type` tag is `spot` (see spot detection above) and it does have a price. The plugin has no spot-price data source, so it prices spot nodes at the on-demand rate for the same instance type/region. |
| `Fargate pricing not supported yet`              | A pod's node name has the `fargate-` prefix (an EKS Fargate virtual node). The row is a $0 orphan — see [Limitations](#limitations). |
| `node <name> not found in priced resources`      | A pod's node has no matching entry among the priced resources at all — most commonly because the node was missing pricing labels (see [Node Pricing Requirements](#node-pricing-requirements)) and was never made priceable; less commonly because the node was deleted between collection and pricing. The row is a $0 orphan. |
| `unallocated priced resource kind "<kind>"`      | A priced resource is neither a node nor tagged `cluster` (a defensive fallback; the shipped collector never produces this today). |

## Limitations

- **No historical data**: `GetStats` reports `STATS_MODE_RUN_RATE`, a
  snapshot of current allocatable capacity and current pod requests. There
  is no time-series or billing-history integration.
- **Requests-based charging**: `charge: max-request-usage` charges each
  workload the larger of its request and its actual usage when both are
  available, but the usage collector only collects
  `cpu_request`/`mem_request` (no metrics-server integration yet), so
  allocation is effectively requests-based today.
- **Spot nodes priced on-demand**: spot/preemptible nodes are priced at the
  on-demand rate for the same instance type and region — every row on that
  node carries a `spot node priced on-demand` note so this is visible.
- **Fargate is $0**: EKS Fargate nodes are skipped from collection entirely,
  so every pod scheduled on one becomes a $0 orphan row with a `Fargate
  pricing not supported yet` note instead of a priced allocation.
- **Idle stays per node**: the default `idle: "separate"` reports each
  node's unused capacity as that node's own `__idle__` row. `idle: "share"`
  folds it into the workloads on that same node and leaves the idle row at
  `$0`. Idle is never netted against another node's idle.
- **EKS-only control plane pricing**: the control plane is detected purely
  from an EKS API server hostname pattern; GKE and AKS control planes are
  not detected or priced by this version.
- **Single policy schema version**: only `version: 1` is understood.
  `idle` and `system_workloads` accept `"separate"` or `"share"`. Every
  other field except the two `node_split` unit prices supports exactly one
  value.
