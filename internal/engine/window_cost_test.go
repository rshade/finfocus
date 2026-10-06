package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const windowBillHourly = 0.25

// windowBillPlugin is a fake actual-cost plugin. A zero period total is
// estimable: the resource carries pulumi:created and projected cost returns
// an hourly rate, so the state-based path would replace that zero.
type windowBillPlugin struct {
	mockBatchCostSourceClient

	mu             sync.Mutex
	actualTotal    float64
	projectedCalls int
	actualReq      *proto.GetActualCostRequest
}

func (p *windowBillPlugin) GetActualCost(
	_ context.Context, in *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	p.mu.Lock()
	p.actualReq = in
	total := p.actualTotal
	p.mu.Unlock()
	return &proto.GetActualCostResponse{
		Results: []*proto.ActualCostResult{{Currency: "USD", TotalCost: total}},
	}, nil
}

func (p *windowBillPlugin) GetProjectedCost(
	_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	p.mu.Lock()
	p.projectedCalls++
	p.mu.Unlock()
	return &proto.GetProjectedCostResponse{
		Results: []*proto.CostResult{{
			Currency:    "USD",
			HourlyCost:  windowBillHourly,
			MonthlyCost: windowBillHourly * hoursPerMonth,
		}},
	}, nil
}

func (p *windowBillPlugin) calls() (int, *proto.GetActualCostRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.projectedCalls, p.actualReq
}

// windowBillFixture builds one estimable resource over a 7-day past window.
// State-based estimation of that window is 0.25 * 168 hours = 42.
func windowBillFixture(total float64) (*windowBillPlugin, *Engine, ResourceDescriptor, time.Time, time.Time) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	plugin := &windowBillPlugin{actualTotal: total}
	eng := New([]*pluginhost.Client{makeNonBatchClient("window-bill", plugin)}, nil)
	resource := ResourceDescriptor{
		ID:       "i-window",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		Properties: map[string]any{
			PropertyPulumiCreated: from.Add(-time.Hour).Format(time.RFC3339),
			"instanceType":        "m5.large",
			"availabilityZone":    "us-east-1a",
		},
	}
	return plugin, eng, resource, from, to
}

func TestSkipStateEstimate_DefaultsFalse(t *testing.T) {
	t.Parallel()

	var request ActualCostRequest
	assert.False(t, request.SkipStateEstimate)
}

func TestSkipStateEstimate_ZeroTotalUsesStateEstimate(t *testing.T) {
	t.Parallel()

	plugin, eng, resource, from, to := windowBillFixture(0)
	result, errs := eng.getActualCostForResource(context.Background(), resource, ActualCostRequest{
		Resources: []ResourceDescriptor{resource},
		From:      from,
		To:        to,
	})
	require.Empty(t, errs)
	require.NotNil(t, result)
	assert.Equal(t, "estimate", result.Adapter)
	assert.InDelta(t, 42.0, result.TotalCost, 1e-9)
	projected, _ := plugin.calls()
	assert.Equal(t, 1, projected)
}

func TestSkipStateEstimate_KeepsPluginZero(t *testing.T) {
	t.Parallel()

	plugin, eng, resource, from, to := windowBillFixture(0)
	result, errs := eng.getActualCostForResource(context.Background(), resource, ActualCostRequest{
		Resources:         []ResourceDescriptor{resource},
		From:              from,
		To:                to,
		SkipStateEstimate: true,
	})
	require.Empty(t, errs)
	require.NotNil(t, result)
	assert.Zero(t, result.TotalCost)
	assert.Equal(t, "window-bill", result.Adapter)
	projected, _ := plugin.calls()
	assert.Zero(t, projected)
}

func TestWindowCost_ReturnsPluginPeriodTotal(t *testing.T) {
	t.Parallel()

	const periodTotal = 18.5
	plugin, eng, resource, from, to := windowBillFixture(periodTotal)
	results, err := eng.GetWindowCost(context.Background(), []ResourceDescriptor{resource}, from, to)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.InDelta(t, periodTotal, results[0].TotalCost, 1e-9)
	_, actual := plugin.calls()
	require.NotNil(t, actual)
	assert.Equal(t, from.Unix(), actual.StartTime)
	assert.Equal(t, to.Unix(), actual.EndTime)
}

func TestWindowCost_SkipStateEstimateKeepsZero(t *testing.T) {
	t.Parallel()

	plugin, eng, resource, from, to := windowBillFixture(0)
	results, err := eng.GetWindowCost(context.Background(), []ResourceDescriptor{resource}, from, to)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Zero(t, results[0].TotalCost)
	assert.Equal(t, "window-bill", results[0].Adapter)
	projected, actual := plugin.calls()
	assert.Zero(t, projected)
	require.NotNil(t, actual)
	assert.Equal(t, from.Unix(), actual.StartTime)
	assert.Equal(t, to.Unix(), actual.EndTime)
}
