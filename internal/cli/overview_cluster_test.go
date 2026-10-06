package cli

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

const testClusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"

func testClusterRow(props map[string]any) engine.OverviewRow {
	return engine.OverviewRow{
		URN:        testClusterURN,
		Type:       "aws:eks/cluster:Cluster",
		Status:     engine.StatusActive,
		Properties: props,
	}
}

func TestClusterScopeCandidates(t *testing.T) {
	t.Parallel()

	const arn = "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster"
	named := map[string]any{"name": "prod-cluster"}
	namedWithARN := map[string]any{"name": "prod-cluster", "arn": arn}
	arnOnly := map[string]any{"arn": "arn:aws:eks:us-east-1:123456789012:cluster/arn-cluster"}

	tests := []struct {
		name         string
		props        map[string]any
		clusterCount int
		cfg          *config.Config
		want         []clusterScope
	}{
		{
			name:         "config mapping by URN wins and is the only candidate",
			props:        named,
			clusterCount: 1,
			cfg: &config.Config{Overview: &config.OverviewConfig{ClusterContexts: map[string]string{
				testClusterURN: "urn-ctx",
				"prod-cluster": "name-ctx",
			}}},
			want: []clusterScope{{scope: "urn-ctx", explicit: true}},
		},
		{
			name:         "config mapping by name",
			props:        named,
			clusterCount: 2,
			cfg: &config.Config{Overview: &config.OverviewConfig{ClusterContexts: map[string]string{
				"prod-cluster": "name-ctx",
			}}},
			want: []clusterScope{{scope: "name-ctx", explicit: true}},
		},
		{
			name:         "name then current context in a single-cluster stack",
			props:        named,
			clusterCount: 1,
			cfg:          config.New(),
			want:         []clusterScope{{scope: "prod-cluster"}, {assumed: true}},
		},
		{
			name:         "name, full ARN, then current context; duplicate ARN segment dropped",
			props:        namedWithARN,
			clusterCount: 1,
			want:         []clusterScope{{scope: "prod-cluster"}, {scope: arn}, {assumed: true}},
		},
		{
			name:         "ARN and its name segment",
			props:        arnOnly,
			clusterCount: 2,
			want: []clusterScope{
				{scope: "arn:aws:eks:us-east-1:123456789012:cluster/arn-cluster"},
				{scope: "arn-cluster"},
			},
		},
		{
			name:         "single cluster without identity assumes current context",
			clusterCount: 1,
			want:         []clusterScope{{assumed: true}},
		},
		{
			name:         "multi cluster without identity is not expanded live",
			clusterCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := clusterScopeCandidates(testClusterRow(tt.props), tt.clusterCount, tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// fakeUsageSource and fakeAllocator satisfy engine.UsageSource / engine.Allocator.
type fakeUsageSource struct {
	resp *pbc.GetStatsResponse
	err  error
}

func (f *fakeUsageSource) GetStats(
	_ context.Context, _ *pbc.GetStatsRequest, _ ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	return f.resp, f.err
}

// scopedUsageSource answers GetStats only for the scopes in ok and records
// every scope it was asked for.
type scopedUsageSource struct {
	resp   *pbc.GetStatsResponse
	ok     map[string]bool
	mu     sync.Mutex
	scopes []string
}

func (f *scopedUsageSource) GetStats(
	_ context.Context, in *pbc.GetStatsRequest, _ ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scopes = append(f.scopes, in.GetScope())
	if !f.ok[in.GetScope()] {
		return nil, errors.New("context not found")
	}
	return f.resp, nil
}

type fakeAllocator struct {
	fn func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error)
}

func (f *fakeAllocator) Allocate(
	_ context.Context, in *pbc.AllocateRequest, _ ...grpc.CallOption,
) (*pbc.AllocateResponse, error) {
	return f.fn(in)
}

// conservingAllocator splits each priced resource 60/40 workload/idle so the
// allocation contract's conservation check passes.
func conservingAllocator() *fakeAllocator {
	return &fakeAllocator{fn: func(r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
		resp := &pbc.AllocateResponse{
			PolicyDigest:        "d",
			EffectivePolicyJson: []byte(`{"version":1}`),
		}
		for _, p := range r.GetPriced() {
			if !p.GetPriced() {
				continue
			}
			id := p.GetResource().GetId()
			resp.Rows = append(resp.Rows,
				&pbc.AllocationRow{
					Subject: map[string]string{
						"kind":      "workload",
						"namespace": "payments",
						"pod":       "p-" + id,
					},
					CpuCost:   p.GetCost() * 0.6,
					TotalCost: p.GetCost() * 0.6,
					Currency:  "USD",
				},
				&pbc.AllocationRow{
					Subject:   map[string]string{"kind": "__idle__", "node": id},
					CpuCost:   p.GetCost() * 0.4,
					TotalCost: p.GetCost() * 0.4,
					Currency:  "USD",
				},
			)
		}
		return resp, nil
	}}
}

type fakePricer struct{ results []engine.CostResult }

func (f fakePricer) GetProjectedCostWithErrors(
	_ context.Context, _ []engine.ResourceDescriptor,
) (*engine.CostResultWithErrors, error) {
	return &engine.CostResultWithErrors{Results: f.results}, nil
}

func (f fakePricer) GetWindowCost(
	_ context.Context, _ []engine.ResourceDescriptor, _, _ time.Time,
) ([]engine.CostResult, error) {
	return nil, errors.New("overview cluster expansion never prices a window")
}

func liveTestHarness() (*fakeUsageSource, *fakeAllocator, fakePricer) {
	usage := &fakeUsageSource{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{
			{Provider: "aws", ResourceType: "aws:ec2/instance:Instance",
				Sku: "m5.large", Region: "us-east-1", Id: "n1", Tags: map[string]string{"kind": "node"}},
		},
	}}
	pricer := fakePricer{results: []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "n1",
			Monthly:      100,
			Currency:     "USD",
		},
	}}
	return usage, conservingAllocator(), pricer
}

func TestRunLiveExpansion(t *testing.T) {
	t.Parallel()
	workloadURN := "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api"

	newRows := func() []engine.OverviewRow {
		return engine.ExpandClustersProjected([]engine.OverviewRow{
			testClusterRow(map[string]any{"name": "prod-cluster"}),
			{
				URN:    workloadURN,
				Type:   "kubernetes:apps/v1:Deployment",
				Status: engine.StatusActive,
				ProjectedCost: &engine.ProjectedCostData{
					MonthlyCost: 54.75,
					Currency:    "USD",
				},
			},
		})
	}

	t.Run("live data wins and projected row is suppressed", func(t *testing.T) {
		t.Parallel()
		usage, alloc, pricer := liveTestHarness()
		rows, notes := runLiveExpansion(
			context.Background(), newRows(), usage, alloc, pricer, nil, config.New(),
		)

		// cluster + payments workload group + idle group.
		require.Len(t, rows, 3)
		assert.Equal(t, testClusterURN, rows[0].URN)
		assert.Equal(t,
			[]string{testClusterURN + "#ns/payments", testClusterURN + "#ns/__idle__"},
			rows[0].ChildURNs)
		assert.Equal(t, engine.ExpansionSourceLive, rows[1].ExpansionSource)
		require.Len(t, notes, 2)
		assert.Contains(t, notes[0], "live cluster data preferred for prod-cluster")
		assert.Contains(t, notes[0], "1 projected workload row hidden")
		assert.Contains(t, notes[1], "excluded from summary")
	})

	t.Run("usage failure keeps projected expansion", func(t *testing.T) {
		t.Parallel()
		_, alloc, pricer := liveTestHarness()
		badUsage := &fakeUsageSource{err: errors.New("connection refused")}
		rows, notes := runLiveExpansion(
			context.Background(), newRows(), badUsage, alloc, pricer, nil, config.New(),
		)
		require.Len(t, rows, 2)
		assert.Equal(t, []string{workloadURN}, rows[0].ChildURNs)
		assert.Equal(t, engine.ExpansionSourceProjected, rows[1].ExpansionSource)
		assert.Empty(t, notes)
	})

	t.Run("every cluster expands after an earlier one reorders rows", func(t *testing.T) {
		t.Parallel()
		usage, alloc, pricer := liveTestHarness()
		secondURN := "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::second"
		rows := []engine.OverviewRow{
			{URN: "urn:pulumi:prod::myapp::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket"},
			testClusterRow(map[string]any{"name": "prod-cluster"}),
			{URN: secondURN, Type: "aws:eks/cluster:Cluster", Properties: map[string]any{"name": "second"}},
		}
		rows, _ = runLiveExpansion(
			context.Background(), rows, usage, alloc, pricer, nil, config.New(),
		)
		childrenOf := map[string]int{}
		for _, r := range rows {
			if r.ParentURN != "" {
				childrenOf[r.ParentURN]++
			}
		}
		assert.Equal(t, 2, childrenOf[testClusterURN])
		assert.Equal(t, 2, childrenOf[secondURN])
	})

	t.Run("unmatched name falls back to the current context", func(t *testing.T) {
		t.Parallel()
		base, alloc, pricer := liveTestHarness()
		usage := &scopedUsageSource{resp: base.resp, ok: map[string]bool{"": true}}
		rows, notes := runLiveExpansion(
			context.Background(), newRows(), usage, alloc, pricer, nil, config.New(),
		)
		assert.Equal(t, []string{"prod-cluster", ""}, usage.scopes)
		require.Len(t, rows, 3)
		assert.Equal(t, engine.ExpansionSourceLive, rows[1].ExpansionSource)
		require.Len(t, notes, 3)
		assert.Contains(t, notes[1], "assumed from the current context")
	})

	t.Run("explicit mapping is the only scope tried", func(t *testing.T) {
		t.Parallel()
		base, alloc, pricer := liveTestHarness()
		usage := &scopedUsageSource{resp: base.resp, ok: map[string]bool{"": true}}
		cfg := &config.Config{Overview: &config.OverviewConfig{ClusterContexts: map[string]string{
			"prod-cluster": "missing-ctx",
		}}}
		rows, notes := runLiveExpansion(context.Background(), newRows(), usage, alloc, pricer, nil, cfg)
		assert.Equal(t, []string{"missing-ctx"}, usage.scopes)
		require.Len(t, rows, 2)
		assert.Equal(t, engine.ExpansionSourceProjected, rows[1].ExpansionSource)
		assert.Empty(t, notes)
	})

	t.Run("deleting cluster is not expanded live", func(t *testing.T) {
		t.Parallel()
		usage, alloc, pricer := liveTestHarness()
		cluster := testClusterRow(map[string]any{"name": "prod-cluster"})
		cluster.Status = engine.StatusDeleting
		rows, notes := runLiveExpansion(
			context.Background(), []engine.OverviewRow{cluster}, usage, alloc, pricer, nil, nil,
		)
		require.Len(t, rows, 1)
		assert.Empty(t, rows[0].ChildURNs)
		assert.Empty(t, notes)
	})

	t.Run("multi cluster stack without identity is skipped", func(t *testing.T) {
		t.Parallel()
		base, alloc, pricer := liveTestHarness()
		usage := &scopedUsageSource{resp: base.resp, ok: map[string]bool{"": true}}
		rows := []engine.OverviewRow{
			testClusterRow(nil),
			{URN: "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::other", Type: "aws:eks/cluster:Cluster"},
		}
		out, notes := runLiveExpansion(context.Background(), rows, usage, alloc, pricer, nil, nil)
		assert.Empty(t, usage.scopes)
		assert.Len(t, out, 2)
		assert.Empty(t, notes)
	})

	t.Run(
		"cluster without identity in single-cluster stack notes the assumption",
		func(t *testing.T) {
			t.Parallel()
			usage, alloc, pricer := liveTestHarness()
			rows := []engine.OverviewRow{testClusterRow(nil)}
			rows, notes := runLiveExpansion(
				context.Background(), rows, usage, alloc, pricer, nil, nil,
			)
			require.Len(t, rows, 3)
			require.Len(t, notes, 2)
			assert.Contains(t, notes[0], "assumed from the current context")
		},
	)
}

func TestExpandOverviewClusters_NoPluginsNoRegression(t *testing.T) {
	t.Parallel()
	rows := engine.ExpandClustersProjected([]engine.OverviewRow{
		testClusterRow(map[string]any{"name": "prod-cluster"}),
	})
	// No plugin clients: live expansion is skipped; the (empty) projected
	// grouping stands and the cluster row is unchanged.
	out, notes := expandOverviewClusters(
		context.Background(),
		rows,
		nil,
		fakePricer{},
		config.New(),
	)
	require.Len(t, out, 1)
	assert.Empty(t, out[0].ChildURNs)
	assert.Empty(t, notes)
}

func TestOverviewRowsExpanded(t *testing.T) {
	t.Parallel()
	assert.False(t, overviewRowsExpanded([]engine.OverviewRow{{URN: "a"}, {URN: "b"}}))
	assert.True(t, overviewRowsExpanded([]engine.OverviewRow{{URN: "a", ChildURNs: []string{"b"}}}))
	assert.True(t, overviewRowsExpanded([]engine.OverviewRow{{URN: "b", ParentURN: "a"}}))
}
