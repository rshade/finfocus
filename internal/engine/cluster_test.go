package engine

import (
	"context"
	"errors"
	"testing"
	"time"

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

// GetWindowCost reports that this fake has no window price. Run-rate tests
// never call it. Historical tests use recordingPricer.
func (fakePricer) GetWindowCost(
	context.Context, []ResourceDescriptor, time.Time, time.Time,
) ([]CostResult, error) {
	return nil, errors.New("GetWindowCost is not used on the run-rate path")
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE, Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}}
	pricer := fakePricer{results: []CostResult{{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1"}}}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no priceable resource could be priced")
}

func TestRunClusterAllocation_NamespaceScopeDropsSharedRows(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	okUsage := &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}
	okPricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 100, Currency: "USD"}}}

	t.Run("usage error wrapped", func(t *testing.T) {
		t.Parallel()
		_, err := RunClusterAllocation(context.Background(), &fakeUsage{err: errors.New("forbidden")},
			conservingAlloc(), okPricer, ClusterRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get usage stats: forbidden")
	})
	t.Run("historical mode rejected", func(t *testing.T) {
		t.Parallel()
		_, err := RunClusterAllocation(context.Background(),
			&fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_HISTORICAL}},
			conservingAlloc(), okPricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrHistoricalUnsupported)
	})
	t.Run("conservation violation", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
	t.Parallel()

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

func TestShowAllocationPolicy_RejectsInvalidResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		resp *pbc.AllocateResponse
		want string
	}{
		{"empty digest", &pbc.AllocateResponse{EffectivePolicyJson: []byte(`{"version":1}`)}, "policy_digest is empty"},
		{"empty policy", &pbc.AllocateResponse{PolicyDigest: "abc"}, "effective_policy_json is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			alloc := &fakeAlloc{fn: func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error) { return tt.resp, nil }}
			_, _, err := ShowAllocationPolicy(context.Background(), alloc, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid allocator response")
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPriceableToResource(t *testing.T) {
	t.Parallel()

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

type windowCall struct {
	from time.Time
	to   time.Time
}

// recordingPricer records which pricing path a cluster run used.
type recordingPricer struct {
	projected      []CostResult
	window         []CostResult
	windowErr      error
	projectedCalls int
	windowCalls    []windowCall
}

func (p *recordingPricer) GetProjectedCostWithErrors(
	_ context.Context, _ []ResourceDescriptor,
) (*CostResultWithErrors, error) {
	p.projectedCalls++
	return &CostResultWithErrors{Results: p.projected}, nil
}

func (p *recordingPricer) GetWindowCost(
	_ context.Context, _ []ResourceDescriptor, from, to time.Time,
) ([]CostResult, error) {
	p.windowCalls = append(p.windowCalls, windowCall{from: from, to: to})
	if p.windowErr != nil {
		return nil, p.windowErr
	}
	return p.window, nil
}

func historicalWindow() (time.Time, time.Time) {
	return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
}

func TestRunClusterAllocation_HistoricalWindow(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	const total = 18.5
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_HISTORICAL,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
	}}
	pricer := &recordingPricer{window: []CostResult{{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "n1",
		TotalCost:    total,
		Monthly:      999,
		Currency:     "USD",
	}}}

	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{
		Scope: "prod", From: from, To: to,
	})
	require.NoError(t, err)
	require.NotNil(t, usage.got.GetStart())
	require.NotNil(t, usage.got.GetEnd())
	assert.True(t, from.Equal(usage.got.GetStart().AsTime()))
	assert.True(t, to.Equal(usage.got.GetEnd().AsTime()))
	require.Len(t, pricer.windowCalls, 1)
	assert.True(t, from.Equal(pricer.windowCalls[0].from))
	assert.True(t, to.Equal(pricer.windowCalls[0].to))
	assert.Zero(t, pricer.projectedCalls, "a window must not ask for a monthly price")
	assert.Equal(t, ModeHistorical, res.Mode)
	assert.Equal(t, FormatPeriod(from, to), res.Period)
	require.Len(t, res.Priced, 1)
	assert.InDelta(t, total, res.Priced[0].Monthly, 1e-9)
	assert.True(t, res.Priced[0].Priced)
	assert.InDelta(t, total, res.Total, 1e-9)
}

func TestRunClusterAllocation_HistoricalNamespaceDropsSharedRows(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	const total = 18.5
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_HISTORICAL,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
	}}
	pricer := &recordingPricer{window: []CostResult{{
		ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", TotalCost: total, Currency: "USD",
	}}}
	alloc := &fakeAlloc{fn: func(r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
		cost := r.GetPriced()[0].GetCost()
		return &pbc.AllocateResponse{
			PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`),
			Rows: []*pbc.AllocationRow{
				{Subject: map[string]string{"kind": "workload", "namespace": "app", "pod": "p", "node": "n1"},
					CpuCost: cost * 0.6, TotalCost: cost * 0.6, Currency: "USD"},
				{Subject: map[string]string{"kind": "__idle__", "node": "n1"},
					CpuCost: cost * 0.3, TotalCost: cost * 0.3, Currency: "USD"},
				{Subject: map[string]string{"kind": "__cluster__"},
					CpuCost: cost * 0.1, TotalCost: cost * 0.1, Currency: "USD"},
			},
		}, nil
	}}

	res, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{
		Namespace: "app", From: from, To: to,
	})
	require.NoError(t, err)
	assert.Equal(t, "app", usage.got.GetSelector()["namespace"])
	assert.True(t, res.NamespaceScoped)
	require.Len(t, res.Rows, 1)
	assert.Equal(t, "workload", res.Rows[0].Subject["kind"])
	assert.InDelta(t, total*0.6, res.Total, 1e-9)
	assert.Zero(t, res.Idle)
}

func TestRunClusterAllocation_IncompleteWarning(t *testing.T) {
	t.Parallel()

	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
		Warnings:  []string{"incomplete: node n1: gap in cpu_usage"},
	}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD"},
	}}
	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.NoError(t, err)
	assert.True(t, res.Incomplete)
	assert.Contains(t, res.Warnings, "incomplete: node n1: gap in cpu_usage")
}

func TestRunClusterAllocation_OtherWarningStaysComplete(t *testing.T) {
	t.Parallel()

	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
		Warnings:  []string{"unknown metric ignored"},
	}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD"},
	}}
	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.NoError(t, err)
	assert.False(t, res.Incomplete)
	assert.Contains(t, res.Warnings, "unknown metric ignored")
}

func TestRunClusterAllocation_NoWindowUsesProjectedMonthly(t *testing.T) {
	t.Parallel()

	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
	}}
	pricer := &recordingPricer{
		projected: []CostResult{{
			ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD",
		}},
		window: []CostResult{{
			ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", TotalCost: 1, Currency: "USD",
		}},
	}
	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, pricer.projectedCalls)
	assert.Empty(t, pricer.windowCalls)
	assert.Nil(t, usage.got.GetStart())
	assert.Nil(t, usage.got.GetEnd())
	assert.Equal(t, ModeRunRate, res.Mode)
	assert.Equal(t, "monthly", res.Period)
	require.Len(t, res.Priced, 1)
	assert.InDelta(t, 70, res.Priced[0].Monthly, 1e-9)
	assert.InDelta(t, 70, res.Total, 1e-9)
}

func TestRunClusterAllocation_WindowRequiresBothBounds(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	tests := []struct {
		name string
		req  ClusterRequest
	}{
		{name: "from only", req: ClusterRequest{From: from}},
		{name: "to only", req: ClusterRequest{To: to}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			usage := &fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_HISTORICAL}}
			_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), fakePricer{}, tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "both")
			assert.Nil(t, usage.got, "GetStats runs only after both bounds are set")
		})
	}
}

func TestRunClusterAllocation_WindowRejectsRunRateStats(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
	}}
	pricer := &recordingPricer{
		projected: []CostResult{{
			ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD",
		}},
		window: []CostResult{{
			ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", TotalCost: 12, Currency: "USD",
		}},
	}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{
		From: from, To: to,
	})
	require.ErrorIs(t, err, ErrStatsModeMismatch)
	assert.Contains(t, err.Error(), ModeHistorical)
	assert.Contains(t, err.Error(), ModeRunRate)
	assert.Zero(t, pricer.projectedCalls)
	assert.Empty(t, pricer.windowCalls)
}

func TestRunClusterAllocation_WindowEndMustBeAfterStart(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_HISTORICAL}}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), fakePricer{}, ClusterRequest{
		From: to, To: from,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end must be after start")
	assert.Nil(t, usage.got)
}

func TestRunClusterAllocation_WindowPriceError(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_HISTORICAL,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
	}}
	pricer := &recordingPricer{windowErr: errors.New("billing down")}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{
		From: from, To: to,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "price cluster resources: billing down")
	assert.Zero(t, pricer.projectedCalls)
	require.Len(t, pricer.windowCalls, 1)
}

func TestRunClusterAllocation_WindowRejectsUnspecifiedMode(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_UNSPECIFIED,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
	}}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), &recordingPricer{}, ClusterRequest{
		From: from, To: to,
	})
	require.ErrorIs(t, err, ErrStatsModeMismatch)
	assert.Contains(t, err.Error(), pbc.StatsMode_STATS_MODE_UNSPECIFIED.String())
}

func TestRunClusterAllocation_HistoricalUnpriced(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	const okCost = 40.0
	tests := []struct {
		name     string
		bad      CostResult
		missing  bool
		wantNote string
	}{
		{
			name: "plugin error",
			bad: CostResult{
				ResourceType: "aws:ec2/instance:Instance", ResourceID: "bad",
				TotalCost: 12, Monthly: 80, Currency: "USD",
				Error: &StructuredError{Code: ErrCodeNoCostData, Message: "billing failed"},
			},
			wantNote: "billing failed",
		},
		{
			name:    "missing result",
			missing: true, wantNote: unpricedMissing,
		},
		{
			name: "zero total ignores monthly",
			bad: CostResult{
				ResourceType: "aws:ec2/instance:Instance", ResourceID: "bad",
				TotalCost: 0, Monthly: 80, Currency: "USD",
			},
			wantNote: unpricedZeroNote,
		},
		{
			name: "negative total",
			bad: CostResult{
				ResourceType: "aws:ec2/instance:Instance", ResourceID: "bad",
				TotalCost: -1, Monthly: 80, Currency: "USD",
			},
			wantNote: unpricedZeroNote,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			usage := &fakeUsage{resp: &pbc.GetStatsResponse{
				Mode:      pbc.StatsMode_STATS_MODE_HISTORICAL,
				Priceable: []*pbc.ResourceDescriptor{nodeDesc("ok"), nodeDesc("bad")},
			}}
			window := []CostResult{{
				ResourceType: "aws:ec2/instance:Instance", ResourceID: "ok",
				TotalCost: okCost, Monthly: 999, Currency: "USD",
			}}
			if !tt.missing {
				window = append(window, tt.bad)
			}
			pricer := &recordingPricer{window: window, projected: []CostResult{{
				ResourceType: "aws:ec2/instance:Instance", ResourceID: "bad", Monthly: 80, Currency: "USD",
			}}}
			res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{
				From: from, To: to,
			})
			require.NoError(t, err)
			assert.Zero(t, pricer.projectedCalls, "a window must not call projected pricing")
			require.Len(t, pricer.windowCalls, 1)

			priced := map[string]PricedSummary{}
			for _, s := range res.Priced {
				priced[s.ID] = s
			}
			require.Contains(t, priced, "ok")
			require.Contains(t, priced, "bad")
			assert.True(t, priced["ok"].Priced)
			assert.InDelta(t, okCost, priced["ok"].Monthly, 1e-9)
			assert.False(t, priced["bad"].Priced)
			assert.Zero(t, priced["bad"].Monthly)
			assert.Contains(t, priced["bad"].Note, tt.wantNote)
			assert.True(t, res.Incomplete)
			assert.InDelta(t, okCost, res.Total, 1e-9)
		})
	}
}

func TestRunClusterAllocation_HistoricalAllUnpricedIsFatal(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_HISTORICAL,
		Priceable: []*pbc.ResourceDescriptor{
			nodeDesc("missing"), nodeDesc("errored"), nodeDesc("zero"),
		},
	}}
	pricer := &recordingPricer{window: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "errored", TotalCost: 5, Monthly: 9,
			Error: &StructuredError{Message: "billing failed"}},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "zero", TotalCost: 0, Monthly: 9, Currency: "USD"},
	}}
	alloc := conservingAlloc()
	_, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{From: from, To: to})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no priceable resource could be priced")
	assert.Zero(t, pricer.projectedCalls)
	assert.Nil(t, alloc.got)
}

func TestRunClusterAllocation_HistoricalConservationAndCurrency(t *testing.T) {
	t.Parallel()

	from, to := historicalWindow()

	t.Run("conservation", func(t *testing.T) {
		t.Parallel()
		usage := &fakeUsage{resp: &pbc.GetStatsResponse{
			Mode:      pbc.StatsMode_STATS_MODE_HISTORICAL,
			Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")},
		}}
		pricer := &recordingPricer{window: []CostResult{{
			ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", TotalCost: 100, Currency: "USD",
		}}}
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
		_, err := RunClusterAllocation(context.Background(), usage, broken, pricer, ClusterRequest{From: from, To: to})
		require.ErrorIs(t, err, ErrConservation)
		assert.Zero(t, pricer.projectedCalls)
	})

	t.Run("mixed currencies", func(t *testing.T) {
		t.Parallel()
		usage := &fakeUsage{resp: &pbc.GetStatsResponse{
			Mode:      pbc.StatsMode_STATS_MODE_HISTORICAL,
			Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2")},
		}}
		pricer := &recordingPricer{window: []CostResult{
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", TotalCost: 5, Currency: "USD"},
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", TotalCost: 5, Currency: "EUR"},
		}}
		alloc := conservingAlloc()
		_, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{From: from, To: to})
		require.ErrorIs(t, err, ErrMixedCurrencies)
		assert.Zero(t, pricer.projectedCalls)
		assert.Nil(t, alloc.got)
	})
}
