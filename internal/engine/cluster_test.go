package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type fakeUsage struct {
	resp *pbc.GetStatsResponse
	err  error
	got  *pbc.GetStatsRequest
}

func (f *fakeUsage) GetStats(
	_ context.Context, in *pbc.GetStatsRequest, _ ...grpc.CallOption,
) (*pbc.GetStatsResponse, error) {
	f.got = in
	return f.resp, f.err
}

type fakeAlloc struct {
	fn  func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error)
	got *pbc.AllocateRequest
}

func (f *fakeAlloc) Allocate(
	_ context.Context, in *pbc.AllocateRequest, _ ...grpc.CallOption,
) (*pbc.AllocateResponse, error) {
	f.got = in
	return f.fn(in)
}

type fakePricer struct{ results []CostResult }

func (f fakePricer) GetProjectedCostWithErrors(
	_ context.Context, _ []ResourceDescriptor,
) (*CostResultWithErrors, error) {
	return &CostResultWithErrors{Results: f.results}, nil
}

func nodeDesc(id string) *pbc.ResourceDescriptor {
	return &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:ec2/instance:Instance",
		Sku: "m5.large", Region: "us-east-1", Id: id, Tags: map[string]string{"kind": "node"}}
}

// conservingAlloc splits each priced resource 60/40 into a workload row and an idle row.
func conservingAlloc() *fakeAlloc {
	return &fakeAlloc{fn: func(r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
		resp := &pbc.AllocateResponse{PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`)}
		for _, p := range r.GetPriced() {
			if !p.GetPriced() {
				continue
			}
			id := p.GetResource().GetId()
			workloadSubject := map[string]string{"kind": "workload", "namespace": "app", "pod": "p-" + id, "node": id}
			resp.Rows = append(resp.Rows,
				&pbc.AllocationRow{
					Subject:   workloadSubject,
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

func TestRunClusterAllocation_HappyPath(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2")},
		Warnings:  []string{"w1"},
	}}
	pricer := fakePricer{results: []CostResult{ // deliberately out of order
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 30, Currency: "USD"},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD"},
	}}
	alloc := conservingAlloc()

	res, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{
		Scope: "prod", Selector: map[string]string{"app": "web"}, PolicyJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	assert.Equal(t, "prod", usage.got.GetScope())
	assert.Equal(t, "web", usage.got.GetSelector()["app"])
	require.Len(t, alloc.got.GetPriced(), 2)
	byID := map[string]float64{}
	for _, p := range alloc.got.GetPriced() {
		byID[p.GetResource().GetId()] = p.GetCost()
	}
	assert.InDelta(t, 70, byID["n1"], 1e-9, "results out of order must still match by id")
	assert.InDelta(t, 30, byID["n2"], 1e-9)
	assert.Equal(t, []byte(`{}`), alloc.got.GetPolicyJson())

	assert.Equal(t, ModeRunRate, res.Mode)
	assert.InDelta(t, 100, res.Total, 1e-9)
	assert.InDelta(t, 40, res.Idle, 1e-9)
	assert.False(t, res.Incomplete)
	assert.Equal(t, "USD", res.Currency)
	assert.Equal(t, "d", res.PolicyDigest)
	assert.Contains(t, res.Warnings, "w1")
	assert.Len(t, res.Priced, 2)
}

func TestRunClusterAllocation_UnpricedResources(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2"), nodeDesc("n3")},
	}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD"},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 0, Currency: "USD",
			Notes: `EC2 instance type "x9.huge" not found in pricing data`},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n3", Currency: "USD",
			Error: &StructuredError{Code: ErrCodeNoCostData, Message: "No pricing information available"}},
	}}
	alloc := conservingAlloc()
	res, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{})
	require.NoError(t, err)

	priced := map[string]*pbc.PricedResource{}
	for _, p := range alloc.got.GetPriced() {
		priced[p.GetResource().GetId()] = p
	}
	assert.True(t, priced["n1"].GetPriced())
	assert.False(t, priced["n2"].GetPriced(), "zero-cost node is unpriced")
	assert.Contains(t, priced["n2"].GetNote(), "not found in pricing data")
	assert.False(t, priced["n3"].GetPriced())
	assert.True(t, res.Incomplete)
	assert.InDelta(t, 70, res.Total, 1e-9)
}

func TestRunClusterAllocation_AllUnpricedIsFatal(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE, Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}}
	pricer := fakePricer{results: []CostResult{{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1"}}}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no priceable resource could be priced")
}

func TestRunClusterAllocation_NamespaceScopeDropsSharedRows(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE, Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 100, Currency: "USD"}}}
	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer,
		ClusterRequest{Namespace: "app"})
	require.NoError(t, err)
	assert.Equal(t, "app", usage.got.GetSelector()["namespace"])
	assert.True(t, res.NamespaceScoped)
	for _, r := range res.Rows {
		assert.Equal(t, "workload", r.Subject["kind"])
	}
	assert.InDelta(t, 60, res.Total, 1e-9)
	assert.Zero(t, res.Idle)
}

func TestRunClusterAllocation_Errors(t *testing.T) {
	okUsage := &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}
	okPricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 100, Currency: "USD"}}}

	t.Run("usage error wrapped", func(t *testing.T) {
		_, err := RunClusterAllocation(context.Background(), &fakeUsage{err: errors.New("forbidden")},
			conservingAlloc(), okPricer, ClusterRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get usage stats: forbidden")
	})
	t.Run("historical mode rejected", func(t *testing.T) {
		_, err := RunClusterAllocation(context.Background(),
			&fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_HISTORICAL}},
			conservingAlloc(), okPricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrHistoricalUnsupported)
	})
	t.Run("conservation violation", func(t *testing.T) {
		broken := &fakeAlloc{fn: func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
			return &pbc.AllocateResponse{
				PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`),
				Rows: []*pbc.AllocationRow{
					{Subject: map[string]string{"kind": "workload", "namespace": "app", "pod": "p"},
						CpuCost: 88, TotalCost: 88, Currency: "USD"},
					{Subject: map[string]string{"kind": "__idle__", "node": "n1"}, Currency: "USD"},
				},
			}, nil
		}}
		_, err := RunClusterAllocation(
			context.Background(),
			&fakeUsage{resp: okUsage},
			broken,
			okPricer,
			ClusterRequest{},
		)
		require.ErrorIs(t, err, ErrConservation)
		assert.Contains(t, err.Error(), "88", "message reports the actual total")
	})
	t.Run("contract violation", func(t *testing.T) {
		noIdle := &fakeAlloc{fn: func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
			return &pbc.AllocateResponse{
				PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`),
				Rows: []*pbc.AllocationRow{
					{Subject: map[string]string{"kind": "workload", "namespace": "app", "pod": "p"},
						CpuCost: 100, TotalCost: 100, Currency: "USD"},
				},
			}, nil
		}}
		_, err := RunClusterAllocation(
			context.Background(),
			&fakeUsage{resp: okUsage},
			noIdle,
			okPricer,
			ClusterRequest{},
		)
		require.ErrorIs(
			t,
			err,
			ErrConservation,
			"a priced node without an idle row is rejected even when totals balance",
		)
	})
	t.Run("mixed currencies", func(t *testing.T) {
		usage := &fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
			Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2")}}}
		pricer := fakePricer{results: []CostResult{
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 1, Currency: "USD"},
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 1, Currency: "EUR"}}}
		_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrMixedCurrencies)
	})
}

func TestShowAllocationPolicy(t *testing.T) {
	alloc := &fakeAlloc{fn: func(r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
		assert.Empty(t, r.GetUsage())
		assert.Empty(t, r.GetPriced())
		return &pbc.AllocateResponse{EffectivePolicyJson: []byte(`{"version":1}`), PolicyDigest: "abc"}, nil
	}}
	pol, digest, err := ShowAllocationPolicy(context.Background(), alloc, []byte(`{"idle":"separate"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":1}`, string(pol))
	assert.Equal(t, "abc", digest)
	assert.JSONEq(t, `{"idle":"separate"}`, string(alloc.got.GetPolicyJson()))
}

func TestPriceableToResource(t *testing.T) {
	r := PriceableToResource(&pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:eks/cluster:Cluster",
		Sku: "cluster", Region: "us-west-2", Id: "prod", Tags: map[string]string{"kind": "cluster"}})
	assert.Equal(t, "aws", r.Provider)
	assert.Equal(t, "aws:eks/cluster:Cluster", r.Type)
	assert.Equal(t, "prod", r.ID)
	assert.Equal(t, "cluster", r.Properties["sku"])
	assert.Equal(t, "us-west-2", r.Properties["region"])
	assert.Equal(t, "cluster", r.Properties["kind"])
	require.NoError(t, r.Validate())
}
