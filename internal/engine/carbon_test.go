package engine

import (
	"context"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type carbonTestAPI struct {
	mockCostSourceClient

	calls         atomic.Int64
	supportsCalls atomic.Int64
	result        *proto.CostResult
	query         func(context.Context) (*proto.GetProjectedCostResponse, error)
	decline       string
}

func (c *carbonTestAPI) Supports(
	_ context.Context,
	_ *pbc.SupportsRequest,
	_ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.supportsCalls.Add(1)
	return &pbc.SupportsResponse{Supported: c.decline == "", Reason: c.decline}, nil
}

func (c *carbonTestAPI) GetProjectedCost(
	ctx context.Context,
	_ *proto.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	c.calls.Add(1)
	if c.query != nil {
		return c.query(ctx)
	}
	return &proto.GetProjectedCostResponse{Results: []*proto.CostResult{c.result}}, nil
}

func carbonTestClient(name string, api *carbonTestAPI, caps ...string) *pluginhost.Client {
	return &pluginhost.Client{Name: name, API: api, Metadata: &proto.PluginMetadata{Capabilities: caps}}
}

func carbonTestResource() ResourceDescriptor {
	return ResourceDescriptor{
		ID:         "vm",
		Type:       "aws:ec2/instance:Instance",
		Provider:   "aws",
		Properties: map[string]any{"instanceType": "t3.medium", "region": "us-east-1"},
	}
}

func carbonTestResult(value float64, unit string) *proto.CostResult {
	return &proto.CostResult{
		Currency:       "USD",
		Sustainability: map[string]proto.SustainabilityMetric{greenops.CarbonMetricKey: {Value: value, Unit: unit}},
	}
}

func projectedCarbonResults(
	ctx context.Context,
	t *testing.T,
	eng *Engine,
	withErrors bool,
) ([]CostResult, []ErrorDetail, error) {
	t.Helper()
	resources := []ResourceDescriptor{carbonTestResource()}
	if withErrors {
		result, err := eng.GetProjectedCostWithErrors(ctx, resources)
		if result == nil {
			return nil, nil, err
		}
		return result.Results, result.Errors, err
	}
	result, err := eng.GetProjectedCost(ctx, resources)
	return result, nil, err
}

func TestCarbonPreservesMonetary(t *testing.T) {
	t.Parallel()
	for _, withErrors := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "with_errors"}[withErrors], func(t *testing.T) {
			t.Parallel()
			carbon := &carbonTestAPI{result: carbonTestResult(1437, "gCO2e")}
			price := &carbonTestAPI{result: &proto.CostResult{MonthlyCost: 73, HourlyCost: .1, Currency: "EUR"}}
			eng := New(
				[]*pluginhost.Client{
					carbonTestClient("carbon", carbon, "carbon"),
					carbonTestClient("money", price, "projected_costs"),
				},
				nil,
			)
			results, errs, err := projectedCarbonResults(context.Background(), t, eng, withErrors)
			require.NoError(t, err)
			assert.Empty(t, errs)
			require.Len(t, results, 1)
			assert.InDelta(t, 73, results[0].Monthly, 1e-9)
			assert.Equal(t, "EUR", results[0].Currency)
			assert.Equal(t, "money", results[0].Adapter)
			assert.InDelta(t, 1437, results[0].Sustainability[greenops.CarbonMetricKey].Value, 1e-9)
			assert.EqualValues(t, 1, carbon.calls.Load())
			assert.EqualValues(t, 1, price.calls.Load())
		})
	}
}

func TestCarbonOnlyKeepsMissingPrice(t *testing.T) {
	t.Parallel()
	carbon := &carbonTestAPI{result: carbonTestResult(100, "gCO2e")}
	eng := New([]*pluginhost.Client{carbonTestClient("carbon", carbon, "carbon")}, nil)
	results, _, err := projectedCarbonResults(context.Background(), t, eng, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Error)
	assert.Equal(t, ErrCodeNoCostData, results[0].Error.Code)
	assert.InDelta(t, 100, results[0].Sustainability[greenops.CarbonMetricKey].Value, 1e-9)
}

func TestCarbonExistingMetricWins(t *testing.T) {
	t.Parallel()
	for _, key := range []string{greenops.CarbonMetricKey, greenops.DeprecatedCarbonKey} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			carbon := &carbonTestAPI{result: carbonTestResult(999, "g")}
			price := &carbonTestAPI{
				result: &proto.CostResult{
					Currency:       "USD",
					MonthlyCost:    73,
					Sustainability: map[string]proto.SustainabilityMetric{key: {Value: 0, Unit: "kg"}},
				},
			}
			eng := New(
				[]*pluginhost.Client{carbonTestClient("money", price), carbonTestClient("carbon", carbon, "carbon")},
				nil,
			)
			results, _, err := projectedCarbonResults(context.Background(), t, eng, false)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Zero(t, results[0].Sustainability[key].Value)
			assert.Zero(t, carbon.calls.Load())
			assert.Zero(t, carbon.supportsCalls.Load())
		})
	}
}

func TestCarbonFailurePreservesPrice(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		metric  *proto.CostResult
		decline string
		rpcErr  bool
	}{
		{name: "negative", metric: carbonTestResult(-1, "g")},
		{name: "nan", metric: carbonTestResult(math.NaN(), "g")},
		{name: "unit", metric: carbonTestResult(100, "gCO2e/month")},
		{name: "no_metric", metric: &proto.CostResult{}},
		{name: "decline", decline: "unsupported region"}, {name: "rpc_failure", rpcErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			carbon := &carbonTestAPI{result: tt.metric, decline: tt.decline}
			if tt.rpcErr {
				carbon.query = func(context.Context) (*proto.GetProjectedCostResponse, error) {
					return nil, status.Error(codes.Unavailable, "offline")
				}
			}
			price := &carbonTestAPI{result: &proto.CostResult{MonthlyCost: 73, Currency: "USD"}}
			eng := New(
				[]*pluginhost.Client{carbonTestClient("money", price), carbonTestClient("carbon", carbon, "carbon")},
				nil,
			)
			results, errs, err := projectedCarbonResults(context.Background(), t, eng, true)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.InDelta(t, 73, results[0].Monthly, 1e-9)
			assert.NotContains(t, results[0].Sustainability, greenops.CarbonMetricKey)
			assert.Contains(t, results[0].Notes, "carbon")
			if tt.decline == "" {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCarbonSelection(t *testing.T) {
	t.Parallel()
	for _, routed := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic", true: "explicit"}[routed], func(t *testing.T) {
			t.Parallel()
			api := &carbonTestAPI{result: carbonTestResult(100, "g")}
			client := carbonTestClient("carbon", api, "carbon")
			eng := New([]*pluginhost.Client{client}, nil)
			if routed {
				eng.WithRouter(
					&mockRouter{selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
						return []PluginMatch{{Client: client, Fallback: true}}
					}},
				)
			}
			for _, feature := range []string{"ProjectedCosts", "ActualCosts", "BatchCost", "PricingSpec"} {
				matches, _ := eng.selectPluginMatchesForResource(context.Background(), carbonTestResource(), feature)
				assert.Empty(t, matches, feature)
			}
			assert.Zero(t, api.supportsCalls.Load())
			matches, _ := eng.selectPluginMatchesForResource(context.Background(), carbonTestResource(), "Carbon")
			assert.Len(t, matches, 1)
		})
	}
}

func TestCarbonCancellationKeepsPartialResults(t *testing.T) {
	t.Parallel()
	for _, withErrors := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "with_errors"}[withErrors], func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			carbon := &carbonTestAPI{query: func(context.Context) (*proto.GetProjectedCostResponse, error) {
				cancel()
				return nil, status.Error(codes.Canceled, "cancelled")
			}}
			price := &carbonTestAPI{result: &proto.CostResult{MonthlyCost: 73, Currency: "USD"}}
			eng := New(
				[]*pluginhost.Client{carbonTestClient("money", price), carbonTestClient("carbon", carbon, "carbon")},
				nil,
			)
			results, _, err := projectedCarbonResults(ctx, t, eng, withErrors)
			require.ErrorIs(t, err, context.Canceled)
			require.Len(t, results, 1)
			assert.InDelta(t, 73, results[0].Monthly, 1e-9)
		})
	}
}

func TestCarbonFallback(t *testing.T) {
	t.Parallel()
	for _, fallback := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[fallback], func(t *testing.T) {
			t.Parallel()
			bad := carbonTestClient(
				"bad",
				&carbonTestAPI{query: func(context.Context) (*proto.GetProjectedCostResponse, error) {
					return nil, status.Error(codes.Unavailable, "offline")
				}},
				"carbon",
			)
			goodAPI := &carbonTestAPI{result: carbonTestResult(500, "g")}
			good := carbonTestClient("good", goodAPI, "carbon")
			price := carbonTestClient(
				"money",
				&carbonTestAPI{result: &proto.CostResult{MonthlyCost: 73, Currency: "USD"}},
			)
			eng := New(
				[]*pluginhost.Client{price, bad, good},
				nil,
			).WithRouter(&mockRouter{selectPluginsFunc: func(_ context.Context, _ ResourceDescriptor, feature string) []PluginMatch {
				if feature == "Carbon" {
					return []PluginMatch{{Client: bad, Fallback: fallback}, {Client: good, Fallback: true}}
				}
				return []PluginMatch{{Client: price, Fallback: true}}
			}})
			results, _, err := projectedCarbonResults(context.Background(), t, eng, false)
			require.NoError(t, err)
			require.Len(t, results, 1)
			if fallback {
				assert.InDelta(t, 500, results[0].Sustainability[greenops.CarbonMetricKey].Value, 1e-9)
			} else {
				assert.Zero(t, goodAPI.calls.Load())
				assert.NotContains(t, results[0].Sustainability, greenops.CarbonMetricKey)
			}
		})
	}
}

func TestCarbonDiagnosticsAreTruncated(t *testing.T) {
	t.Parallel()
	longMessage := strings.Repeat("x", maxDeclineReasonLen+40)
	for _, tt := range []struct {
		name    string
		api     *carbonTestAPI
		wantErr bool
	}{
		{name: "supports decline", api: &carbonTestAPI{decline: longMessage}},
		{name: "rpc failure", api: &carbonTestAPI{query: func(context.Context) (*proto.GetProjectedCostResponse, error) {
			return nil, status.Error(codes.Unavailable, longMessage)
		}}},
		{name: "plugin note", api: &carbonTestAPI{result: &proto.CostResult{
			Sustainability: map[string]proto.SustainabilityMetric{greenops.CarbonMetricKey: {Value: 50, Unit: "g"}},
			Notes:          longMessage,
		}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			price := &carbonTestAPI{result: &proto.CostResult{MonthlyCost: 73, Currency: "USD"}}
			eng := New([]*pluginhost.Client{
				carbonTestClient("money", price), carbonTestClient("carbon", tt.api, "carbon"),
			}, nil)
			results, errs, err := projectedCarbonResults(context.Background(), t, eng, true)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.NotContains(t, results[0].Notes, longMessage)
			assert.LessOrEqual(t, len(results[0].Notes), len("carbon carbon unavailable: ")+maxDeclineReasonLen+3)
			assert.Contains(t, results[0].Notes, "...")
			if tt.name == "rpc failure" {
				require.Len(t, errs, 1)
				assert.Contains(t, errs[0].Error.Error(), longMessage)
			}
		})
	}
}
