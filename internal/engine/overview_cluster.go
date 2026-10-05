package engine

import (
	"fmt"
	"strings"
)

// Expansion sources for OverviewRow.ExpansionSource.
const (
	// ExpansionSourceLive marks a synthetic allocation child row produced by
	// RunClusterAllocation. Live rows re-allocate node cost already
	// represented by other rows and are excluded from summary totals.
	ExpansionSourceLive = "live"
	// ExpansionSourceProjected marks a declared workload row re-parented
	// under a cluster row. It keeps its cost and totals contribution.
	ExpansionSourceProjected = "projected"
)

// allocationChildType is the synthetic type token for live allocation children.
const allocationChildType = "finfocus:k8s/namespace:Allocation"

// Breakdown keys for live allocation children.
const (
	breakdownKeyCPU    = "cpu"
	breakdownKeyMemory = "memory"
)

// IsValidExpansionSource reports whether s is a known expansion source.
func IsValidExpansionSource(s string) bool {
	return s == ExpansionSourceLive || s == ExpansionSourceProjected
}

// IsClusterResource reports whether a Pulumi type token is a Kubernetes
// cluster resource (EKS, GKE, or AKS) eligible for overview expansion.
func IsClusterResource(typeToken string) bool {
	switch typeToken {
	case "aws:eks/cluster:Cluster",
		"eks:index:Cluster",
		"gcp:container/cluster:Cluster",
		"google-native:container/v1:Cluster",
		"azure:containerservice/kubernetesCluster:KubernetesCluster",
		"azure-native:containerservice:ManagedCluster":
		return true
	default:
		return false
	}
}

// IsWorkloadResource reports whether a Pulumi type token is a declared
// Kubernetes workload kind that can nest under a cluster row.
func IsWorkloadResource(typeToken string) bool {
	switch typeToken {
	case "kubernetes:apps/v1:Deployment",
		"kubernetes:apps/v1:StatefulSet",
		"kubernetes:apps/v1:DaemonSet",
		"kubernetes:batch/v1:Job",
		"kubernetes:batch/v1:CronJob":
		return true
	default:
		return false
	}
}

// ExpandClustersProjected re-parents declared workload rows under the
// stack's cluster row. Grouping happens only when the row list contains
// exactly one cluster: multi-cluster attribution requires provider-chain
// data that is not propagated into StateResource today, so multi-cluster
// stacks are left flat (documented limitation). The returned slice is
// ordered parent-then-children. Rows already carrying a ParentURN (e.g.
// from a previous expansion pass) are left as-is, making the function
// idempotent.
func ExpandClustersProjected(rows []OverviewRow) []OverviewRow {
	var cluster *OverviewRow
	for i := range rows {
		if rows[i].ParentURN == "" && IsClusterResource(rows[i].Type) {
			if cluster != nil {
				return rows // more than one cluster: leave flat
			}
			cluster = &rows[i]
		}
	}
	if cluster == nil {
		return rows
	}

	var workloadIdx []int
	for i := range rows {
		if rows[i].ParentURN == "" && IsWorkloadResource(rows[i].Type) {
			workloadIdx = append(workloadIdx, i)
		}
	}
	if len(workloadIdx) == 0 {
		return rows
	}

	clusterURN := cluster.URN
	childURNs := make([]string, 0, len(workloadIdx))
	for _, i := range workloadIdx {
		rows[i].ParentURN = clusterURN
		rows[i].ExpansionSource = ExpansionSourceProjected
		childURNs = append(childURNs, rows[i].URN)
	}
	cluster.ChildURNs = childURNs
	return orderRowsWithChildren(rows)
}

// LiveChildrenFromResult synthesizes namespace-granularity allocation child
// rows for a cluster from a RunClusterAllocation result. The cluster-scope
// group (__cluster__) is skipped because the parent row already shows
// control-plane cost; the idle group is kept so unused capacity is visible.
func LiveChildrenFromResult(clusterURN string, res *ClusterResult) ([]OverviewRow, error) {
	groups, err := GroupClusterRows(res.Rows, "namespace")
	if err != nil {
		return nil, fmt.Errorf("grouping allocation rows: %w", err)
	}
	children := make([]OverviewRow, 0, len(groups))
	for _, g := range groups {
		if g.Key == rowKindCluster {
			continue
		}
		children = append(children, OverviewRow{
			URN:    clusterURN + "#ns/" + g.Key,
			Type:   allocationChildType,
			Status: StatusActive,
			ProjectedCost: &ProjectedCostData{
				MonthlyCost: g.TotalCost,
				Currency:    res.Currency,
				Breakdown: map[string]float64{
					breakdownKeyCPU:    g.CPUCost,
					breakdownKeyMemory: g.MemCost,
				},
			},
			ParentURN:       clusterURN,
			ExpansionSource: ExpansionSourceLive,
		})
	}
	return children, nil
}

// ApplyLiveExpansion replaces a cluster's expanded children with live
// allocation rows. Existing children of the cluster (projected rows from
// ExpandClustersProjected or rows from an earlier live pass) are removed
// from the list; the number of suppressed projected rows is returned so the
// caller can footnote the suppression. The cluster row's ChildURNs is set
// to the live children and the slice is ordered parent-then-children.
func ApplyLiveExpansion(
	rows []OverviewRow, clusterURN string, children []OverviewRow,
) ([]OverviewRow, int) {
	if len(children) == 0 {
		return rows, 0
	}
	clusterIdx := -1
	suppressed := 0
	kept := make([]OverviewRow, 0, len(rows)+len(children))
	for i := range rows {
		switch {
		case rows[i].URN == clusterURN && rows[i].ParentURN == "":
			clusterIdx = len(kept)
			rows[i].ChildURNs = nil
			kept = append(kept, rows[i])
		case rows[i].ParentURN == clusterURN:
			if rows[i].ExpansionSource == ExpansionSourceProjected {
				suppressed++
			}
			// Drop all prior children of this cluster.
		default:
			kept = append(kept, rows[i])
		}
	}
	if clusterIdx < 0 {
		return rows, 0
	}
	childURNs := make([]string, len(children))
	for i := range children {
		childURNs[i] = children[i].URN
	}
	kept[clusterIdx].ChildURNs = childURNs
	return orderRowsWithChildren(append(kept, children...)), suppressed
}

// orderRowsWithChildren returns rows ordered so that each child immediately
// follows its parent row. Children whose parent is absent keep their
// relative order at their original position.
func orderRowsWithChildren(rows []OverviewRow) []OverviewRow {
	childrenByParent := map[string][]OverviewRow{}
	parentSeen := map[string]bool{}
	for _, r := range rows {
		if r.ParentURN != "" {
			childrenByParent[r.ParentURN] = append(childrenByParent[r.ParentURN], r)
		}
	}
	out := make([]OverviewRow, 0, len(rows))
	for _, r := range rows {
		if r.ParentURN != "" {
			continue
		}
		out = append(out, r)
		parentSeen[r.URN] = true
		out = append(out, childrenByParent[r.URN]...)
	}
	// Orphaned children (parent filtered out or missing): append at the end
	// in their original relative order, preserving their parent reference.
	for _, r := range rows {
		if r.ParentURN != "" && !parentSeen[r.ParentURN] {
			out = append(out, r)
		}
	}
	return out
}

// ClusterRowName extracts a display name for a cluster row: the `name`
// property when present, else the final segment of the `arn` property,
// else the rightmost URN segment.
func ClusterRowName(row OverviewRow) string {
	if name, ok := row.Properties["name"].(string); ok && name != "" {
		return name
	}
	if arn, ok := row.Properties["arn"].(string); ok && arn != "" {
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			return arn[idx+1:]
		}
		if idx := strings.LastIndex(arn, ":"); idx >= 0 && idx < len(arn)-1 {
			return arn[idx+1:]
		}
	}
	parts := strings.Split(row.URN, "::")
	return parts[len(parts)-1]
}
