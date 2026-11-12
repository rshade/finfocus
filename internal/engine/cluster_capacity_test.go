package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func capacityRows() []ClusterRow {
	w := func(node, pod string, cost float64, note string) ClusterRow {
		return ClusterRow{
			Subject:   map[string]string{"kind": "workload", "namespace": "app", "pod": pod, "node": node},
			CPUCost:   cost / 2,
			MemCost:   cost / 2,
			TotalCost: cost,
			Note:      note,
		}
	}
	idle := func(node string, cost float64, note string) ClusterRow {
		return ClusterRow{
			Subject:   map[string]string{"kind": "__idle__", "node": node},
			CPUCost:   cost,
			TotalCost: cost,
			Note:      note,
		}
	}
	const spotNote = "spot node priced on-demand"
	return []ClusterRow{
		w("spot-1", "a", 10, spotNote),
		w("spot-1", "b", 5, spotNote),
		idle("spot-1", 5, spotNote),
		w("od-1", "c", 30, ""),
		idle("od-1", 10, ""),
		w("bare-1", "d", 8, ""),
		idle("bare-1", 2, ""),
		{Subject: map[string]string{"kind": "__cluster__", "cluster": "prod"}, TotalCost: 73},
	}
}

func TestGroupClusterByCapacity(t *testing.T) {
	t.Parallel()

	capacity := []*pbc.ResourceDescriptor{
		capNode("", "spot-1", "", "spot"), capNode("", "od-1", "", "on-demand"),
	}
	const spotNote = "spot node priced on-demand"

	tests := []struct {
		name     string
		rows     []ClusterRow
		capacity []*pbc.ResourceDescriptor
		want     []CapacityGroup
	}{
		{
			name:     "mixed spot, on-demand, untagged, and control plane",
			rows:     capacityRows(),
			capacity: capacity,
			want: []CapacityGroup{
				{
					CapacityType: CapacityUnlabeled,
					CPUCost:      6,
					MemCost:      4,
					TotalCost:    83,
					IdleCost:     ptr(2),
					Nodes:        1,
					Rows:         3,
				},
				{
					CapacityType: "on-demand",
					CPUCost:      25,
					MemCost:      15,
					TotalCost:    40,
					IdleCost:     ptr(10),
					Nodes:        1,
					Rows:         2,
				},
				{
					CapacityType: "spot",
					CPUCost:      12.5,
					MemCost:      7.5,
					TotalCost:    20,
					IdleCost:     ptr(5),
					Nodes:        1,
					Rows:         3,
					Notes:        []string{spotNote},
				},
			},
		},
		{
			name:     "no tags anywhere is one unlabeled group",
			rows:     capacityRows(),
			capacity: nil,
			want: []CapacityGroup{
				{
					CapacityType: CapacityUnlabeled,
					CPUCost:      43.5,
					MemCost:      26.5,
					TotalCost:    143,
					IdleCost:     ptr(17),
					Nodes:        3,
					Rows:         8,
					Notes:        []string{spotNote},
				},
			},
		},
		{
			name:     "empty tag value is unlabeled",
			rows:     capacityRows()[3:5],
			capacity: []*pbc.ResourceDescriptor{capNode("", "od-1", "", "")},
			want: []CapacityGroup{
				{
					CapacityType: CapacityUnlabeled,
					CPUCost:      25,
					MemCost:      15,
					TotalCost:    40,
					IdleCost:     ptr(10),
					Nodes:        1,
					Rows:         2,
				},
			},
		},
		{
			name:     "no rows",
			rows:     nil,
			capacity: capacity,
			want:     []CapacityGroup{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GroupClusterByCapacity(tt.rows, capacityByNode(tt.capacity))
			require.Len(t, got, len(tt.want))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].CapacityType, got[i].CapacityType)
				assert.InDelta(t, tt.want[i].TotalCost, got[i].TotalCost, 1e-9, tt.want[i].CapacityType)
				assert.InDelta(t, tt.want[i].CPUCost, got[i].CPUCost, 1e-9, tt.want[i].CapacityType)
				assert.InDelta(t, tt.want[i].MemCost, got[i].MemCost, 1e-9, tt.want[i].CapacityType)
				assert.InDelta(t, idleOf(tt.want[i]), idleOf(got[i]), 1e-9, tt.want[i].CapacityType)
				assert.Equal(t, tt.want[i].Nodes, got[i].Nodes, tt.want[i].CapacityType)
				assert.Equal(t, tt.want[i].Rows, got[i].Rows, tt.want[i].CapacityType)
				assert.Equal(t, tt.want[i].Notes, got[i].Notes, tt.want[i].CapacityType)
			}
			var sumRows, sumGroups float64
			for _, r := range tt.rows {
				sumRows += r.TotalCost
			}
			for _, g := range got {
				sumGroups += g.TotalCost
			}
			assert.InDelta(t, sumRows, sumGroups, 1e-9, "every row lands in exactly one group")
		})
	}
}

func capNode(cluster, id, name, capacity string) *pbc.ResourceDescriptor {
	tags := map[string]string{"kind": "node"}
	if cluster != "" {
		tags["cluster"] = cluster
	}
	if name != "" {
		tags["node"] = name
	}
	if capacity != "" {
		tags["capacity_type"] = capacity
	}
	return &pbc.ResourceDescriptor{Id: id, Tags: tags}
}

func capRow(kind, cluster, node string, cost float64) ClusterRow {
	s := map[string]string{"kind": kind, "node": node}
	if cluster != "" {
		s["cluster"] = cluster
	}
	return ClusterRow{Subject: s, CPUCost: cost, TotalCost: cost}
}

func TestGroupClusterByCapacity_AllocatorShapes(t *testing.T) {
	t.Parallel()

	type want struct {
		total, idle float64
		nodes       int
	}
	tests := []struct {
		name  string
		descs []*pbc.ResourceDescriptor
		rows  []ClusterRow
		want  map[string]want
	}{
		{
			name: "same node name in two clusters, composite ids",
			descs: []*pbc.ResourceDescriptor{
				capNode("c1", "c1\x00n1", "n1", "spot"),
				capNode("c2", "c2\x00n1", "n1", "on-demand"),
			},
			rows: []ClusterRow{
				capRow("workload", "c1", "n1", 17.5), capRow("workload", "c2", "n1", 70),
				capRow("__idle__", "c1", "c1\x00n1", 52.5), capRow("__idle__", "c2", "c2\x00n1", 70),
			},
			want: map[string]want{"spot": {70, 52.5, 1}, "on-demand": {140, 70, 1}},
		},
		{
			name:  "clusterless descriptor with rows stamped with a cluster",
			descs: []*pbc.ResourceDescriptor{capNode("", "spot-1", "", "spot")},
			rows: []ClusterRow{
				capRow("workload", "c", "spot-1", 11.07), capRow("__idle__", "c", "spot-1", 18.93),
				capRow("workload", "c", "fargate-ip-1", 0), capRow("workload", "c", "deleted-node", 0),
			},
			want: map[string]want{"spot": {30, 18.93, 1}, CapacityUnlabeled: {0, 0, 2}},
		},
		{
			name: "exact match with an empty tag beats the clusterless fallback",
			descs: []*pbc.ResourceDescriptor{
				capNode("c1", "c1\x00n1", "n1", ""), capNode("", "n1", "", "spot"),
			},
			rows: []ClusterRow{
				capRow("workload", "c1", "n1", 5), capRow("__idle__", "c1", "c1\x00n1", 5),
				capRow("__idle__", "", "n1", 2),
			},
			want: map[string]want{CapacityUnlabeled: {10, 5, 1}, "spot": {2, 2, 1}},
		},
		{
			name:  "descriptor tagged for another cluster never matches",
			descs: []*pbc.ResourceDescriptor{capNode("c1", "c1\x00n1", "n1", "spot")},
			rows:  []ClusterRow{capRow("workload", "c2", "n1", 5)},
			want:  map[string]want{CapacityUnlabeled: {5, 0, 1}},
		},
		{
			name:  "single cluster run without cluster on rows",
			descs: []*pbc.ResourceDescriptor{capNode("", "n1", "", "spot")},
			rows:  []ClusterRow{capRow("workload", "", "n1", 6), capRow("__idle__", "", "n1", 4)},
			want:  map[string]want{"spot": {10, 4, 1}},
		},
		{
			name: "control plane and Fargate descriptors are not nodes",
			descs: []*pbc.ResourceDescriptor{
				{Id: "eks", Tags: map[string]string{"kind": "cluster", "capacity_type": "spot"}},
				{Id: "fargate-1", Tags: map[string]string{"kind": "fargate", "capacity_type": "spot"}},
			},
			rows: []ClusterRow{capRow("workload", "", "fargate-1", 3)},
			want: map[string]want{CapacityUnlabeled: {3, 0, 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GroupClusterByCapacity(tt.rows, capacityByNode(tt.descs))
			require.Len(t, got, len(tt.want))
			var sumRows, sumGroups float64
			for _, r := range tt.rows {
				sumRows += r.TotalCost
			}
			for _, g := range got {
				w, ok := tt.want[g.CapacityType]
				require.True(t, ok, g.CapacityType)
				assert.InDelta(t, w.total, g.TotalCost, 1e-9, g.CapacityType)
				assert.InDelta(t, w.idle, idleOf(g), 1e-9, g.CapacityType)
				assert.Equal(t, w.nodes, g.Nodes, g.CapacityType)
				sumGroups += g.TotalCost
			}
			assert.InDelta(t, sumRows, sumGroups, 1e-9, "buckets sum to the input")
		})
	}
}

func taggedNodeDesc(id, capacity string) *pbc.ResourceDescriptor {
	d := nodeDesc(id)
	if capacity != "" {
		d.Tags["capacity_type"] = capacity
	}
	return d
}

func TestRunClusterAllocation_CapacitySplit(t *testing.T) {
	t.Parallel()

	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{
			taggedNodeDesc("n1", "spot"), taggedNodeDesc("n2", "on-demand"), taggedNodeDesc("n3", ""),
		},
	}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 50, Currency: "USD"},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 30, Currency: "USD"},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n3", Monthly: 20, Currency: "USD"},
	}}

	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.NoError(t, err)

	byType := map[string]CapacityGroup{}
	var sum, idle float64
	for _, g := range res.Capacity {
		byType[g.CapacityType] = g
		sum += g.TotalCost
		idle += idleOf(g)
	}
	require.Len(t, byType, 3)
	assert.InDelta(t, 50, byType["spot"].TotalCost, 1e-9)
	assert.InDelta(t, 20, idleOf(byType["spot"]), 1e-9)
	assert.InDelta(t, 30, byType["on-demand"].TotalCost, 1e-9)
	assert.InDelta(t, 20, byType[CapacityUnlabeled].TotalCost, 1e-9)
	assert.InDelta(t, res.Total, sum, 1e-9, "capacity split sums to the cluster total")
	assert.InDelta(t, res.Idle, idle, 1e-9, "idle is counted once")
}

func TestRunClusterAllocation_CapacitySplitScoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  ClusterRequest
	}{
		{name: "namespace", req: ClusterRequest{Namespace: "app"}},
		{name: "selector", req: ClusterRequest{Selector: map[string]string{"team": "storefront"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			usage := &fakeUsage{resp: &pbc.GetStatsResponse{
				Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
				Priceable: []*pbc.ResourceDescriptor{taggedNodeDesc("n1", "spot")},
			}}
			pricer := fakePricer{results: []CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 50, Currency: "USD"},
			}}

			res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, tt.req)
			require.NoError(t, err)

			require.Len(t, res.Capacity, 1)
			assert.Equal(t, "spot", res.Capacity[0].CapacityType)
			assert.InDelta(t, res.Total, res.Capacity[0].TotalCost, 1e-9)
			assert.Nil(t, res.Capacity[0].IdleCost, "scoped runs omit idle rows")
		})
	}
}

func ptr(v float64) *float64 { return &v }

func idleOf(g CapacityGroup) float64 {
	if g.IdleCost == nil {
		return 0
	}
	return *g.IdleCost
}
