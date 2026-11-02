package engine

import (
	"sort"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// CapacityUnlabeled is the bucket for rows whose node carries no capacity_type
// tag, and for rows that name no node (the control plane).
const CapacityUnlabeled = "unlabeled"

const capacityTypeTag = "capacity_type"

// CapacityGroup is the allocated cost of the rows whose node carries one
// capacity_type tag value. Every row lands in exactly one group, so the group
// totals add up to ClusterResult.Total and idle cost is counted once, under
// the node it belongs to.
type CapacityGroup struct {
	CapacityType string   `json:"capacity_type"`
	CPUCost      float64  `json:"cpu_cost"`
	MemCost      float64  `json:"mem_cost"`
	TotalCost    float64  `json:"total_cost"`
	IdleCost     float64  `json:"idle_cost"`
	Nodes        int      `json:"nodes"`
	Rows         int      `json:"rows"`
	Notes        []string `json:"notes,omitempty"`
}

// capacityByNode maps each priceable resource id to its capacity_type tag.
// Resources without the tag, or with an empty value, are absent.
func capacityByNode(descs []*pbc.ResourceDescriptor) map[string]string {
	out := make(map[string]string, len(descs))
	for _, d := range descs {
		if v := d.GetTags()[capacityTypeTag]; v != "" {
			out[d.GetId()] = v
		}
	}
	return out
}

// GroupClusterByCapacity aggregates rows by the capacity_type of their node.
// The tag value is opaque: core keeps no table of capacity types and reports
// the plugin's row notes as they come.
func GroupClusterByCapacity(rows []ClusterRow, capacity map[string]string) []CapacityGroup {
	groups := map[string]*CapacityGroup{}
	nodes := map[string]map[string]bool{}
	notes := map[string]map[string]bool{}
	for _, r := range rows {
		node := r.Subject[subjectKeyNode]
		key := CapacityUnlabeled
		if v := capacity[node]; v != "" && node != "" {
			key = v
		}
		g := groups[key]
		if g == nil {
			g = &CapacityGroup{CapacityType: key}
			groups[key] = g
			nodes[key] = map[string]bool{}
			notes[key] = map[string]bool{}
		}
		g.CPUCost += r.CPUCost
		g.MemCost += r.MemCost
		g.TotalCost += r.TotalCost
		g.Rows++
		if r.Subject[subjectKeyKind] == rowKindIdle {
			g.IdleCost += r.TotalCost
		}
		if node != "" {
			nodes[key][node] = true
		}
		if r.Note != "" {
			notes[key][r.Note] = true
		}
	}
	out := make([]CapacityGroup, 0, len(groups))
	for key, g := range groups {
		g.Nodes = len(nodes[key])
		for n := range notes[key] {
			g.Notes = append(g.Notes, n)
		}
		sort.Strings(g.Notes)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalCost != out[j].TotalCost {
			return out[i].TotalCost > out[j].TotalCost
		}
		return out[i].CapacityType < out[j].CapacityType
	})
	return out
}
