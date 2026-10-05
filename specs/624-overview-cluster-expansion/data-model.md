# Data Model: Overview Cluster Expansion

## OverviewRow additions (`internal/engine/overview_types.go`)

| Field | Type | JSON | Meaning |
|-------|------|------|---------|
| `ParentURN` | `string` | `parentUrn,omitempty` | URN of the parent cluster row; empty for top-level rows |
| `ExpansionSource` | `string` | `expansionSource,omitempty` | `"live"` or `"projected"`; empty for non-expansion rows |
| `ChildURNs` | `[]string` | `childUrns,omitempty` | Ordered child URNs on a cluster row with children |

Validation: all three are optional; `ExpansionSource`, when present, must be
one of the two constants. A row with `ParentURN` set must not appear in
`ChildURNs` of itself.

## ExpansionSource constants (`internal/engine/overview_cluster.go`)

- `ExpansionSourceLive = "live"` — synthetic allocation child from
  `RunClusterAllocation`; excluded from summary totals.
- `ExpansionSourceProjected = "projected"` — a real declared-workload row
  re-parented under a cluster; keeps its cost and totals contribution.

## Cluster detection

`IsClusterResource(typeToken)` — true for:

- `aws:eks/cluster:Cluster`, `eks:index:Cluster`
- `gcp:container/cluster:Cluster`, `google-native:container/v1:Cluster`
- `azure:containerservice/kubernetesCluster:KubernetesCluster`,
  `azure-native:containerservice:ManagedCluster`

`IsWorkloadResource(typeToken)` — true for:

- `kubernetes:apps/v1:Deployment`, `kubernetes:apps/v1:StatefulSet`,
  `kubernetes:apps/v1:DaemonSet`
- `kubernetes:batch/v1:Job`, `kubernetes:batch/v1:CronJob`

## Live allocation child row

Synthesized from `ClusterResult` grouped by namespace
(`GroupClusterRows(rows, "namespace")`):

| Field | Value |
|-------|-------|
| `URN` | `<clusterURN>#ns/<namespace>` (`#ns/__idle__` for idle) |
| `Type` | `finfocus:k8s/namespace:Allocation` |
| `Status` | `StatusActive` |
| `ParentURN` | cluster row URN |
| `ExpansionSource` | `"live"` |
| `ProjectedCost` | `{MonthlyCost: group total, Currency, Breakdown: {cpu, memory}}` |

## StackContext addition

`ExpansionNotes []string json:"expansionNotes,omitempty"` — footnotes such as
"live cluster data preferred; 2 projected workload row(s) hidden" or "cluster
context assumed from current kubeconfig context".

## Configuration addition (`internal/config`)

```jsonc
"overview": {
  "cluster_contexts": { // cluster name or full URN → kubeconfig context
    "prod-cluster": "prod"
  }
}
```

`OverviewConfig{ClusterContexts map[string]string yaml:"cluster_contexts,omitempty"}`
on `Config` as `Overview *OverviewConfig yaml:"overview,omitempty"`.

## Relationships

```text
StackContext 1───* OverviewRow
OverviewRow (cluster) 1───* OverviewRow (child)   via ChildURNs / ParentURN
child.ExpansionSource ∈ {live, projected}
```
