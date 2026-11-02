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

	capacity := map[string]string{nodeKey("", "spot-1"): "spot", nodeKey("", "od-1"): "on-demand"}
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
				{
					CapacityType: CapacityUnlabeled,
					CPUCost:      6,
					MemCost:      4,
					TotalCost:    83,
					IdleCost:     2,
					Nodes:        1,
					Rows:         3,
				},
				{CapacityType: "on-demand", CPUCost: 25, MemCost: 15, TotalCost: 40, IdleCost: 10, Nodes: 1, Rows: 2},
				{
					CapacityType: "spot",
					CPUCost:      12.5,
					MemCost:      7.5,
					TotalCost:    20,
					IdleCost:     5,
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
					IdleCost:     17,
					Nodes:        3,
					Rows:         8,
					Notes:        []string{spotNote},
				},
			},
		},
		{
			name:     "empty tag value is unlabeled",
			rows:     capacityRows()[3:5],
			capacity: map[string]string{nodeKey("", "od-1"): ""},
			want: []CapacityGroup{
				{
					CapacityType: CapacityUnlabeled,
					CPUCost:      25,
					MemCost:      15,
					TotalCost:    40,
					IdleCost:     10,
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
			got := GroupClusterByCapacity(tt.rows, tt.capacity)
			require.Len(t, got, len(tt.want))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].CapacityType, got[i].CapacityType)
				assert.InDelta(t, tt.want[i].TotalCost, got[i].TotalCost, 1e-9, tt.want[i].CapacityType)
				assert.InDelta(t, tt.want[i].CPUCost, got[i].CPUCost, 1e-9, tt.want[i].CapacityType)
				assert.InDelta(t, tt.want[i].MemCost, got[i].MemCost, 1e-9, tt.want[i].CapacityType)
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

func TestGroupClusterByCapacity_SameNodeNameInTwoClusters(t *testing.T) {
	t.Parallel()

	desc := func(cluster, capacity string) *pbc.ResourceDescriptor {
		return &pbc.ResourceDescriptor{
			Id:   "ip-10-0-1-5",
			Tags: map[string]string{"cluster": cluster, "capacity_type": capacity},
		}
	}
	row := func(kind, cluster string, cost float64) ClusterRow {
		return ClusterRow{
			Subject:   map[string]string{"kind": kind, "cluster": cluster, "node": "ip-10-0-1-5"},
			CPUCost:   cost,
			TotalCost: cost,
		}
	}
	rows := []ClusterRow{
		row("workload", "a", 10), row("__idle__", "a", 5),
		row("workload", "b", 30), row("__idle__", "b", 20),
	}

	got := GroupClusterByCapacity(rows, capacityByNode([]*pbc.ResourceDescriptor{
		desc("a", "spot"), desc("b", "on-demand"),
	}))

	require.Len(t, got, 2)
	assert.Equal(t, "on-demand", got[0].CapacityType)
	assert.InDelta(t, 50, got[0].TotalCost, 1e-9)
	assert.InDelta(t, 20, got[0].IdleCost, 1e-9)
	assert.Equal(t, 1, got[0].Nodes)
	assert.Equal(t, "spot", got[1].CapacityType)
	assert.InDelta(t, 15, got[1].TotalCost, 1e-9)
	assert.InDelta(t, 5, got[1].IdleCost, 1e-9)
	assert.Equal(t, 1, got[1].Nodes)
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
