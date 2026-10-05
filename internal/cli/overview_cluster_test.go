package cli

import (
	"context"
	"errors"
	"testing"

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

func TestResolveClusterScope(t *testing.T) {
	t.Parallel()

	named := map[string]any{"name": "prod-cluster"}
	arnOnly := map[string]any{"arn": "arn:aws:eks:us-east-1:123456789012:cluster/arn-cluster"}

	t.Run("config mapping by URN wins", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Overview: &config.OverviewConfig{ClusterContexts: map[string]string{
			testClusterURN: "urn-ctx",
			"prod-cluster": "name-ctx",
		}}}
		scope, assumed := resolveClusterScope(testClusterRow(named), 1, cfg)
		assert.Equal(t, "urn-ctx", scope)
		assert.False(t, assumed)
	})

	t.Run("config mapping by name", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Overview: &config.OverviewConfig{ClusterContexts: map[string]string{
			"prod-cluster": "name-ctx",
		}}}
		scope, assumed := resolveClusterScope(testClusterRow(named), 2, cfg)
		assert.Equal(t, "name-ctx", scope)
		assert.False(t, assumed)
	})

	t.Run("name property becomes the context", func(t *testing.T) {
		t.Parallel()
		scope, assumed := resolveClusterScope(testClusterRow(named), 1, config.New())
		assert.Equal(t, "prod-cluster", scope)
		assert.False(t, assumed)
	})

	t.Run("arn name segment becomes the context", func(t *testing.T) {
		t.Parallel()
		scope, assumed := resolveClusterScope(testClusterRow(arnOnly), 1, nil)
		assert.Equal(t, "arn-cluster", scope)
		assert.False(t, assumed)
	})

	t.Run("single cluster without identity assumes current context", func(t *testing.T) {
		t.Parallel()
		scope, assumed := resolveClusterScope(testClusterRow(nil), 1, nil)
		assert.Empty(t, scope)
		assert.True(t, assumed)
	})

	t.Run("multi cluster without identity uses URN segment", func(t *testing.T) {
		t.Parallel()
		scope, assumed := resolveClusterScope(testClusterRow(nil), 2, nil)
		assert.Equal(t, "cluster", scope)
		assert.False(t, assumed)
	})
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
			context.Background(), newRows(), []int{0}, usage, alloc, pricer, nil, config.New(),
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
			context.Background(), newRows(), []int{0}, badUsage, alloc, pricer, nil, config.New(),
		)
		require.Len(t, rows, 2)
		assert.Equal(t, []string{workloadURN}, rows[0].ChildURNs)
		assert.Equal(t, engine.ExpansionSourceProjected, rows[1].ExpansionSource)
		assert.Empty(t, notes)
	})

	t.Run(
		"cluster without identity in single-cluster stack notes the assumption",
		func(t *testing.T) {
			t.Parallel()
			usage, alloc, pricer := liveTestHarness()
			rows := []engine.OverviewRow{testClusterRow(nil)}
			rows, notes := runLiveExpansion(
				context.Background(), rows, []int{0}, usage, alloc, pricer, nil, nil,
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
