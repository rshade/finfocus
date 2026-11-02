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

	capacity := map[string]string{"spot-1": "spot", "od-1": "on-demand"}
	const spotNote = "spot node priced on-demand"

	tests := []struct {
		name     string
		rows     []ClusterRow
		capacity map[string]string
		want     []CapacityGroup
	}{
		{
			name:     "mixed spot, on-demand, untagged, and control plane",
			rows:     capacityRows(),
			capacity: capacity,
			want: []CapacityGroup{
				{CapacityType: CapacityUnlabeled, TotalCost: 83, IdleCost: 2, Nodes: 1, Rows: 3},
				{CapacityType: "on-demand", TotalCost: 40, IdleCost: 10, Nodes: 1, Rows: 2},
				{CapacityType: "spot", TotalCost: 20, IdleCost: 5, Nodes: 1, Rows: 3, Notes: []string{spotNote}},
			},
		},
		{
			name:     "no tags anywhere is one unlabeled group",
			rows:     capacityRows(),
			capacity: nil,
			want: []CapacityGroup{
				{CapacityType: CapacityUnlabeled, TotalCost: 143, IdleCost: 17, Nodes: 3, Rows: 8,
					Notes: []string{spotNote}},
			},
		},
		{
			name:     "empty tag value is unlabeled",
			rows:     capacityRows()[3:5],
			capacity: map[string]string{"od-1": ""},
			want: []CapacityGroup{
				{CapacityType: CapacityUnlabeled, TotalCost: 40, IdleCost: 10, Nodes: 1, Rows: 2},
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
			got := GroupClusterByCapacity(tt.rows, tt.capacity)
			require.Len(t, got, len(tt.want))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].CapacityType, got[i].CapacityType)
				assert.InDelta(t, tt.want[i].TotalCost, got[i].TotalCost, 1e-9, tt.want[i].CapacityType)
				assert.InDelta(t, tt.want[i].IdleCost, got[i].IdleCost, 1e-9, tt.want[i].CapacityType)
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
		idle += g.IdleCost
	}
	require.Len(t, byType, 3)
	assert.InDelta(t, 50, byType["spot"].TotalCost, 1e-9)
	assert.InDelta(t, 20, byType["spot"].IdleCost, 1e-9)
	assert.InDelta(t, 30, byType["on-demand"].TotalCost, 1e-9)
	assert.InDelta(t, 20, byType[CapacityUnlabeled].TotalCost, 1e-9)
	assert.InDelta(t, res.Total, sum, 1e-9, "capacity split sums to the cluster total")
	assert.InDelta(t, res.Idle, idle, 1e-9, "idle is counted once")
}

func TestRunClusterAllocation_CapacitySplitNamespaceScoped(t *testing.T) {
	t.Parallel()

	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{taggedNodeDesc("n1", "spot")},
	}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 50, Currency: "USD"},
	}}

	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer,
		ClusterRequest{Namespace: "app"})
	require.NoError(t, err)

	require.Len(t, res.Capacity, 1)
	assert.Equal(t, "spot", res.Capacity[0].CapacityType)
	assert.InDelta(t, res.Total, res.Capacity[0].TotalCost, 1e-9)
	assert.InDelta(t, 0, res.Capacity[0].IdleCost, 1e-9, "scoped runs omit idle rows")
}
