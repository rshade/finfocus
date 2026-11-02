package engine

import (
	"sort"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
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

// nodeKey identifies a node by cluster and name, because node names repeat
// across clusters. A row or descriptor without a cluster uses the empty one.
func nodeKey(cluster, node string) string {
	return cluster + "\x00" + node
}

// capacityEntry is one priced node: its capacity_type tag (possibly empty) and
// its canonical cluster-and-id key, unique across the request.
type capacityEntry struct {
	capacity string
	id       string
}

// capacityIndex finds a node's capacity entry from an allocation row.
type capacityIndex map[string]capacityEntry

// capacityByNode indexes the node descriptors twice: under cluster and
// Kubernetes name, which workload rows carry, and under cluster and priced id,
// which idle rows carry. Descriptors of other kinds (control plane, Fargate
// pods) are left out.
func capacityByNode(descs []*pbc.ResourceDescriptor) capacityIndex {
	idx := make(capacityIndex, len(descs))
	for _, d := range descs {
		tags := d.GetTags()
		if tags[subjectKeyKind] != pluginsdk.KindNode {
			continue
		}
		cluster := tags[pluginsdk.SubjectCluster]
		name := tags[subjectKeyNode]
		if name == "" {
			name = d.GetId()
		}
		e := capacityEntry{capacity: tags[capacityTypeTag], id: nodeKey(cluster, d.GetId())}
		idx[nodeKey(cluster, name)] = e
		idx[e.id] = e
	}
	return idx
}

// lookup finds the entry for a row's cluster and node. An exact match wins even
// when its capacity is empty. A row that names a cluster falls back to a
// descriptor with no cluster tag, as the allocator does; a descriptor tagged
// for another cluster never matches.
func (idx capacityIndex) lookup(cluster, node string) (capacityEntry, bool) {
	if node == "" {
		return capacityEntry{}, false
	}
	if e, ok := idx[nodeKey(cluster, node)]; ok {
		return e, true
	}
	if cluster != "" {
		e, ok := idx[nodeKey("", node)]
		return e, ok
	}
	return capacityEntry{}, false
}

// GroupClusterByCapacity aggregates rows by the capacity_type of their node.
// The tag value is opaque: core keeps no table of capacity types and reports
// the plugin's row notes as they come.
func GroupClusterByCapacity(rows []ClusterRow, capacity capacityIndex) []CapacityGroup {
	groups := map[string]*CapacityGroup{}
	nodes := map[string]map[string]bool{}
	notes := map[string]map[string]bool{}
	for _, r := range rows {
		node := r.Subject[subjectKeyNode]
		cluster := r.Subject[pluginsdk.SubjectCluster]
		nk := nodeKey(cluster, node)
		key := CapacityUnlabeled
		if e, ok := capacity.lookup(cluster, node); ok {
			nk = e.id
			if e.capacity != "" {
				key = e.capacity
			}
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
			nodes[key][nk] = true
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
