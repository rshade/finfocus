package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestCostFromPluginPricingSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        *pbc.PricingSpec
		properties  map[string]any
		wantOK      bool
		wantMonthly float64
		wantHourly  float64
		wantCur     string
		wantNote    string
	}{
		{
			name: "per hour",
			spec: &pbc.PricingSpec{
				BillingMode: billingPerHour, RatePerUnit: 0.1, Currency: "EUR", Source: "aws",
			},
			wantOK: true, wantMonthly: 73, wantHourly: 0.1, wantCur: "EUR",
			wantNote: "Calculated from plugin pricing spec: aws (per_hour)",
		},
		{
			name:   "per day",
			spec:   &pbc.PricingSpec{BillingMode: billingPerDay, RatePerUnit: 2, Currency: "USD"},
			wantOK: true, wantMonthly: 60, wantHourly: 60.0 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (per_day)",
		},
		{
			name:       "per gb month with size",
			spec:       &pbc.PricingSpec{BillingMode: billingPerGBMonth, RatePerUnit: 0.1, Currency: "USD"},
			properties: map[string]any{"sizeGb": 20},
			wantOK:     true, wantMonthly: 2, wantHourly: 2.0 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (per_gb_month)",
		},
		{
			name:   "per gb month assumes one gb",
			spec:   &pbc.PricingSpec{BillingMode: billingPerGBMonth, RatePerUnit: 0.1, Currency: "USD"},
			wantOK: true, wantMonthly: 0.1, wantHourly: 0.1 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (per_gb_month); assumed 1 GB",
		},
		{
			name:       "per request",
			spec:       &pbc.PricingSpec{BillingMode: billingPerRequest, RatePerUnit: 0.002, Currency: "USD"},
			properties: map[string]any{"requests": 1000},
			wantOK:     true, wantMonthly: 2, wantHourly: 2.0 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (per_request)",
		},
		{
			name:   "flat",
			spec:   &pbc.PricingSpec{BillingMode: billingFlat, RatePerUnit: 12, Currency: "USD"},
			wantOK: true, wantMonthly: 12, wantHourly: 12.0 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (flat)",
		},
		{
			name:       "per cpu hour",
			spec:       &pbc.PricingSpec{BillingMode: billingPerCPUHour, RatePerUnit: 0.05, Currency: "USD"},
			properties: map[string]any{"cpu": 4},
			wantOK:     true, wantMonthly: 146, wantHourly: 0.2, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (per_cpu_hour)",
		},
		{
			name: "tiered hourly uses the tier that contains one unit",
			spec: &pbc.PricingSpec{
				BillingMode: billingTiered,
				Unit:        "hour",
				Currency:    "USD",
				Source:      "plugin-a",
				PricingTiers: []*pbc.PricingTier{
					{MinQuantity: 0, MaxQuantity: 10, RatePerUnit: 0.5},
					{MinQuantity: 10, MaxQuantity: 0, RatePerUnit: 0.1},
				},
			},
			wantOK: true, wantMonthly: 365, wantHourly: 0.5, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin-a (tiered)",
		},
		{
			name: "tiered storage uses the size tier",
			spec: &pbc.PricingSpec{
				BillingMode: billingTiered,
				Unit:        "GB-month",
				Currency:    "USD",
				PricingTiers: []*pbc.PricingTier{
					{MinQuantity: 0, MaxQuantity: 50, RatePerUnit: 0.10},
					{MinQuantity: 50, MaxQuantity: 0, RatePerUnit: 0.05},
				},
			},
			properties: map[string]any{"sizeGb": 100},
			wantOK:     true, wantMonthly: 5, wantHourly: 5.0 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (tiered)",
		},
		{
			name:   "zero rate is a priced result",
			spec:   &pbc.PricingSpec{BillingMode: billingPerHour, RatePerUnit: 0, Currency: "USD", Provider: "aws"},
			wantOK: true, wantMonthly: 0, wantHourly: 0, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: aws (per_hour)",
		},
		{
			name:   "unit hour when billing mode is empty",
			spec:   &pbc.PricingSpec{Unit: "hour", RatePerUnit: 0.2, Currency: "USD"},
			wantOK: true, wantMonthly: 146, wantHourly: 0.2, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: plugin (hour)",
		},
		{
			name:   "empty currency defaults to usd",
			spec:   &pbc.PricingSpec{BillingMode: billingFlat, RatePerUnit: 3},
			wantOK: true, wantMonthly: 3, wantHourly: 3.0 / hoursPerMonth, wantCur: "USD",
			wantNote: "Calculated from plugin pricing spec: named-plugin (flat)",
		},
		{
			name:   "negative rate is unusable",
			spec:   &pbc.PricingSpec{BillingMode: billingPerHour, RatePerUnit: -1},
			wantOK: false,
		},
		{
			name:   "unknown billing mode is unusable",
			spec:   &pbc.PricingSpec{BillingMode: "mystery", RatePerUnit: 1},
			wantOK: false,
		},
		{
			name:   "nil spec is unusable",
			spec:   nil,
			wantOK: false,
		},
		{
			name:   "tiered without tiers is unusable",
			spec:   &pbc.PricingSpec{BillingMode: billingTiered, Unit: "hour"},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := ResourceDescriptor{
				Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws", Properties: tt.properties,
			}
			pluginName := "plugin"
			if tt.name == "empty currency defaults to usd" {
				pluginName = "named-plugin"
			}
			got, ok := costFromPluginPricingSpec(resource, tt.spec, pluginName)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, adapterPluginSpec, got.Adapter)
			assert.Equal(t, tt.wantCur, got.Currency)
			assert.Equal(t, tt.wantNote, got.Notes)
			assert.InDelta(t, tt.wantMonthly, got.Monthly, 1e-9)
			assert.InDelta(t, tt.wantHourly, got.Hourly, 1e-9)
			assert.InDelta(t, tt.wantMonthly, got.Breakdown["base_cost"], 1e-9)
			assert.Equal(t, resource.ID, got.ResourceID)
		})
	}
}

type pricingSpecPlugin struct {
	mockCostSourceClient

	projected    *proto.GetProjectedCostResponse
	projectedErr error
	spec         *pbc.PricingSpec
	specErr      error
	hangSpec     bool
	specCalls    int
	lastSpecID   string
	lastSKU      string
}

func (p *pricingSpecPlugin) GetProjectedCost(
	_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	if p.projectedErr != nil {
		return nil, p.projectedErr
	}
	if p.projected == nil {
		return &proto.GetProjectedCostResponse{}, nil
	}
	return p.projected, nil
}

func (p *pricingSpecPlugin) GetPricingSpec(
	ctx context.Context, in *pbc.GetPricingSpecRequest, _ ...grpc.CallOption,
) (*pbc.GetPricingSpecResponse, error) {
	p.specCalls++
	if in.GetResource() != nil {
		p.lastSpecID = in.GetResource().GetId()
		p.lastSKU = in.GetResource().GetSku()
	}
	if p.hangSpec {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if p.specErr != nil {
		return nil, p.specErr
	}
	if p.spec == nil {
		return &pbc.GetPricingSpecResponse{}, nil
	}
	return &pbc.GetPricingSpecResponse{Spec: p.spec}, nil
}

type mapSpecLoader struct {
	specs map[string]*PricingSpec
}

func (m *mapSpecLoader) LoadSpec(provider, service, sku string) (any, error) {
	if m == nil {
		return nil, ErrNoCostData
	}
	spec, ok := m.specs[provider+"-"+service+"-"+sku]
	if !ok {
		return nil, ErrNoCostData
	}
	return spec, nil
}

func pricingYAMLLoader() *mapSpecLoader {
	return &mapSpecLoader{specs: map[string]*PricingSpec{
		"aws-ec2-t3.micro": {
			Provider: "aws",
			Service:  "ec2",
			SKU:      "t3.micro",
			Currency: "USD",
			Pricing:  map[string]any{"monthlyEstimate": 7.59},
		},
	}}
}

func pricingResource() ResourceDescriptor {
	return ResourceDescriptor{
		Type:       "aws:ec2:Instance",
		ID:         "i-123",
		Provider:   "aws",
		Properties: map[string]any{"instanceType": "t3.micro"},
	}
}

func TestProjectedPricingSpecFallbackChain(t *testing.T) {
	t.Parallel()

	hourlySpec := &pbc.PricingSpec{
		BillingMode: billingPerHour, RatePerUnit: 0.1, Currency: "USD", Source: "aws-public",
	}

	t.Run("disabled skips the rpc and uses yaml", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: hourlySpec}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader())
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "local-spec", results[0].Adapter)
		assert.InDelta(t, 7.59, results[0].Monthly, 0.01)
		assert.Contains(t, results[0].Notes, "local spec")
		assert.Equal(t, 0, plugin.specCalls)
	})

	t.Run("enabled plugin spec wins over yaml", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: hourlySpec}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, adapterPluginSpec, results[0].Adapter)
		assert.InDelta(t, 73.0, results[0].Monthly, 1e-9)
		assert.Contains(t, results[0].Notes, "Calculated from plugin pricing spec: aws-public (per_hour)")
		assert.Equal(t, 1, plugin.specCalls)
		assert.Equal(t, "i-123", plugin.lastSpecID)
	})

	t.Run("enabled with errors path uses plugin spec", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: hourlySpec}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		got, err := eng.GetProjectedCostWithErrors(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Len(t, got.Results, 1)
		assert.Equal(t, adapterPluginSpec, got.Results[0].Adapter)
		assert.InDelta(t, 73.0, got.Results[0].Monthly, 1e-9)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("rpc error falls through to yaml", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{specErr: errors.New("unimplemented")}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "local-spec", results[0].Adapter)
		assert.InDelta(t, 7.59, results[0].Monthly, 0.01)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("unknown billing mode falls through to yaml", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: &pbc.PricingSpec{BillingMode: "mystery", RatePerUnit: 4}}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "local-spec", results[0].Adapter)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("zero rate does not fall through to yaml", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: &pbc.PricingSpec{
			BillingMode: billingPerHour, RatePerUnit: 0, Currency: "USD", Source: "aws-public",
		}}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, adapterPluginSpec, results[0].Adapter)
		assert.InDelta(t, 0.0, results[0].Monthly, 1e-9)
		assert.Contains(t, results[0].Notes, "plugin pricing spec")
	})

	t.Run("second plugin spec is used when the first fails", func(t *testing.T) {
		t.Parallel()
		first := &pricingSpecPlugin{specErr: errors.New("no spec")}
		second := &pricingSpecPlugin{spec: hourlySpec}
		eng := New([]*pluginhost.Client{
			{Name: "first", API: first},
			{Name: "second", API: second},
		}, pricingYAMLLoader()).WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, adapterPluginSpec, results[0].Adapter)
		assert.InDelta(t, 73.0, results[0].Monthly, 1e-9)
		assert.Equal(t, 1, first.specCalls)
		assert.Equal(t, 1, second.specCalls)
	})

	t.Run("projected cost success does not call pricing spec", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{
			spec: hourlySpec,
			projected: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{
				MonthlyCost: 9, HourlyCost: 9.0 / hoursPerMonth, Currency: "USD", Notes: "plugin price",
			}}},
		}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, pricingYAMLLoader()).
			WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "aws-plugin", results[0].Adapter)
		assert.InDelta(t, 9.0, results[0].Monthly, 1e-9)
		assert.Equal(t, 0, plugin.specCalls)
	})

	t.Run("enabled with no usable spec still reaches the placeholder", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{specErr: errors.New("down")}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil).
			WithPricingSpecFallback(true)
		results, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, adapterNone, results[0].Adapter)
		assert.Contains(t, results[0].Notes, noteNoPricingInfo)
		assert.Equal(t, 1, plugin.specCalls)
	})
}
