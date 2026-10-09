package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type selectedEstimatePlugin struct {
	estimateMockPlugin

	rate         float64
	fallback     bool
	failure      error
	projected    *proto.GetProjectedCostResponse
	projectedErr error
	malformed    bool
	projectedFn  func(context.Context, *proto.GetProjectedCostRequest) (*proto.GetProjectedCostResponse, error)
}

func (p *selectedEstimatePlugin) GetPricingSpec(
	context.Context,
	*pbc.GetPricingSpecRequest,
	...grpc.CallOption,
) (*pbc.GetPricingSpecResponse, error) {
	return &pbc.GetPricingSpecResponse{
		Spec: &pbc.PricingSpec{BillingMode: "hourly", Currency: "USD", RatePerUnit: p.rate},
	}, nil
}

func (p *selectedEstimatePlugin) EstimateCost(
	_ context.Context,
	r *pbc.EstimateCostRequest,
	_ ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
	if p.failure != nil {
		return nil, p.failure
	}
	if p.fallback {
		return nil, status.Error(codes.Unimplemented, "unsupported")
	}
	return &pbc.EstimateCostResponse{
		Currency:    "USD",
		CostMonthly: p.rate * r.GetAttributes().GetFields()["size"].GetNumberValue(),
	}, nil
}

func (p *selectedEstimatePlugin) GetProjectedCost(
	ctx context.Context,
	r *proto.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	if p.projectedFn != nil {
		return p.projectedFn(ctx, r)
	}
	if p.malformed {
		return p.projected, p.projectedErr
	}
	return &proto.GetProjectedCostResponse{
		Results: []*proto.CostResult{
			{Currency: "USD", MonthlyCost: p.rate * r.Resources[0].Attributes.GetFields()["size"].GetNumberValue()},
		},
	}, nil
}

// Selecting a discovered provider must change actual prices, not just labels.
func TestEstimateSelectedProvider(t *testing.T) {
	t.Parallel()
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	eng := New(
		[]*pluginhost.Client{
			{Name: "first", API: &selectedEstimatePlugin{rate: 10}},
			{Name: "second", API: &selectedEstimatePlugin{rate: 30}},
		},
		nil,
	)
	request := &EstimateRequest{Resource: resource, PropertyOverrides: map[string]string{"size": "3"}}
	require.NoError(t, json.Unmarshal([]byte(`{"pricingMode":"[\"second\",\"hourly\"]"}`), request))
	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.InDelta(t, 60.0, result.Baseline.Monthly, 1e-9)
	assert.InDelta(t, 90.0, result.Modified.Monthly, 1e-9)
	assert.InDelta(t, 30.0, result.TotalChange, 1e-9)
}

func TestEstimateBaselineAndSelectionValidation(t *testing.T) {
	t.Parallel()
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	eng := New([]*pluginhost.Client{{Name: "selected", API: &selectedEstimatePlugin{rate: 30}}}, nil)
	for _, mode := range []string{"", `["selected","hourly"]`} {
		result, err := eng.EstimateBaseline(context.Background(), resource, mode)
		require.NoError(t, err)
		assert.InDelta(t, 60.0, result.Baseline.Monthly, 1e-9)
		assert.InDelta(t, 60.0, result.Modified.Monthly, 1e-9)
		assert.Zero(t, result.TotalChange)
		assert.Empty(t, result.Deltas)
	}
	_, err := eng.EstimateCost(context.Background(), &EstimateRequest{Resource: resource})
	require.ErrorContains(t, err, "property overrides are required")
	_, err = eng.EstimateBaseline(context.Background(), resource, `["selected","invented"]`)
	require.ErrorIs(t, err, ErrInvalidPricingMode)
	_, err = eng.EstimateBaseline(context.Background(), nil, "")
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = eng.EstimateBaseline(ctx, resource, `["selected","hourly"]`)
	require.ErrorIs(t, err, context.Canceled)
}

func TestEstimateSelectedFallbackIgnoresOtherProviderCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	eng := New(
		[]*pluginhost.Client{
			{Name: "first", API: &selectedEstimatePlugin{rate: 10}},
			{Name: "selected", API: &selectedEstimatePlugin{rate: 30, fallback: true}},
		},
		nil,
	).WithCache(newMockCache(true))
	eng.storeProjectedCostCache(ctx, *resource, []CostResult{{Monthly: 999, Currency: "USD", Adapter: "foreign"}})
	modified := *resource
	modified.Properties = map[string]any{"size": float64(3)}
	eng.storeProjectedCostCache(ctx, modified, []CostResult{{Monthly: 999, Currency: "USD", Adapter: "foreign"}})
	require.InDelta(t, 999.0, eng.tryProjectedCostCache(ctx, *resource)[0].Monthly, 1e-9)
	result, err := eng.EstimateCost(
		ctx,
		&EstimateRequest{
			Resource:          resource,
			PropertyOverrides: map[string]string{"size": "3"},
			PricingMode:       `["selected","hourly"]`,
		},
	)
	require.NoError(t, err)
	assert.True(t, result.UsedFallback)
	assert.InDelta(t, 60.0, result.Baseline.Monthly, 1e-9)
	assert.InDelta(t, 90.0, result.Modified.Monthly, 1e-9)
	assert.Equal(t, "selected", result.Modified.Adapter)
	assert.InDelta(t, 999.0, eng.tryProjectedCostCache(ctx, *resource)[0].Monthly, 1e-9)
}

func TestEstimateSelectedFailureDoesNotUseAnotherProvider(t *testing.T) {
	t.Parallel()
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	eng := New(
		[]*pluginhost.Client{
			{Name: "good", API: &selectedEstimatePlugin{rate: 10}},
			{Name: "bad", API: &selectedEstimatePlugin{failure: status.Error(codes.Unavailable, "offline")}},
		},
		nil,
	)
	result, err := eng.EstimateCost(
		context.Background(),
		&EstimateRequest{
			Resource:          resource,
			PropertyOverrides: map[string]string{"size": "3"},
			PricingMode:       `["bad","hourly"]`,
		},
	)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Equal(t, "Unavailable: offline", err.Error())
}

func TestEstimateSelectedMalformedFallback(t *testing.T) {
	t.Parallel()
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	for _, tc := range []struct {
		name     string
		response *proto.GetProjectedCostResponse
	}{
		{name: "nil"},
		{name: "empty", response: &proto.GetProjectedCostResponse{}},
		{name: "nil result", response: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{nil}}},
		{name: "negative", response: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{MonthlyCost: -1, Currency: "USD"}}}},
		{name: "currency", response: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{MonthlyCost: 1}}}},
		{name: "structured error", response: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{MonthlyCost: 1, Currency: "USD", StructuredError: &proto.StructuredError{Code: "offline", Message: "sensitive"}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := New(
				[]*pluginhost.Client{
					{
						Name: "selected",
						API:  &selectedEstimatePlugin{fallback: true, malformed: true, projected: tc.response},
					},
				},
				nil,
			)
			assert.NotPanics(t, func() {
				result, err := eng.EstimateBaseline(context.Background(), resource, `["selected","hourly"]`)
				require.Error(t, err)
				assert.Nil(t, result)
			})
		})
	}
}

func TestEstimateSelectedFallbackRejectsModifiedFailures(t *testing.T) {
	t.Parallel()
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	for _, tc := range []struct {
		name     string
		currency string
		failure  error
	}{
		{name: "currency mismatch", currency: "EUR"},
		{name: "modified unavailable", failure: status.Error(codes.Unavailable, "offline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plugin := &selectedEstimatePlugin{
				fallback: true,
				projectedFn: func(ctx context.Context, request *proto.GetProjectedCostRequest) (*proto.GetProjectedCostResponse, error) {
					_, bounded := ctx.Deadline()
					assert.True(t, bounded, "selected fallback must have an RPC deadline")
					currency := "USD"
					if request.Resources[0].Attributes.GetFields()["size"].GetNumberValue() == 3 {
						if tc.failure != nil {
							return nil, tc.failure
						}
						currency = tc.currency
					}
					return &proto.GetProjectedCostResponse{
						Results: []*proto.CostResult{{Currency: currency, MonthlyCost: 10}},
					}, nil
				},
			}
			eng := New([]*pluginhost.Client{{Name: "selected", API: plugin}}, nil)
			result, err := eng.EstimateCost(
				context.Background(),
				&EstimateRequest{
					Resource:          resource,
					PropertyOverrides: map[string]string{"size": "3"},
					PricingMode:       `["selected","hourly"]`,
				},
			)
			require.Error(t, err)
			assert.Nil(t, result)
			if tc.failure == nil {
				require.ErrorIs(t, err, errCurrencyMismatch)
			} else {
				assert.Equal(t, codes.Unavailable, status.Code(err))
			}
		})
	}
}

func TestEstimateSelectedFallbackValidatesModifiedProperties(t *testing.T) {
	t.Parallel()
	eng := New([]*pluginhost.Client{{Name: "selected", API: &selectedEstimatePlugin{fallback: true, rate: 30}}}, nil)
	resource := &ResourceDescriptor{
		ID:         "urn:test",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	result, err := eng.EstimateCost(
		context.Background(),
		&EstimateRequest{
			Resource:          resource,
			PropertyOverrides: map[string]string{"": "invalid"},
			PricingMode:       `["selected","hourly"]`,
		},
	)
	require.ErrorIs(t, err, ErrResourceValidation)
	assert.Nil(t, result)
}

func TestEstimateSelectedFallbackOmitsOversizedSecret(t *testing.T) {
	t.Parallel()
	eng := New([]*pluginhost.Client{{Name: "selected", API: &selectedEstimatePlugin{fallback: true, rate: 30}}}, nil)
	resource := &ResourceDescriptor{
		ID:       "urn:test",
		Type:     "aws:ec2:Instance",
		Provider: "aws",
		Properties: map[string]any{
			"size": float64(2),
			"userData": map[string]any{
				"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
				"ciphertext":                       strings.Repeat("x", maxPropertyValLen+1),
			},
		},
	}
	result, err := eng.EstimateCost(
		context.Background(),
		&EstimateRequest{
			Resource:          resource,
			PropertyOverrides: map[string]string{"size": "3"},
			PricingMode:       `["selected","hourly"]`,
		},
	)
	require.NoError(t, err)
	assert.True(t, result.UsedFallback)
	assert.InDelta(t, 90.0, result.Modified.Monthly, 0.001)
}
