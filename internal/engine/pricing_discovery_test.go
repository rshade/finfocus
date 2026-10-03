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
)

func TestDiscoverPricingSpec(t *testing.T) {
	t.Parallel()

	spec := &pbc.PricingSpec{
		BillingMode: "per_gb_month",
		RatePerUnit: 0.023,
		Currency:    "USD",
		Unit:        "GB-month",
		Assumptions: []string{"  730 hours/month  ", ""},
		MetricHints: []*pbc.UsageMetricHint{{Metric: "storage_gb", Unit: "GB"}, {Metric: "  "}},
		PricingTiers: []*pbc.PricingTier{
			{MinQuantity: 0, MaxQuantity: 100, RatePerUnit: 0.023, Description: "first 100 GB"},
			{MinQuantity: 100, MaxQuantity: 0, RatePerUnit: 0.021, Description: "over 100 GB"},
		},
	}

	t.Run("caches one rpc per resource type", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: spec}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		first := pricingResource()
		second := first
		second.ID = "i-456"
		got := eng.DiscoverPricingSpec(context.Background(), &first)
		again := eng.DiscoverPricingSpec(context.Background(), &second)

		require.Len(t, got.Modes, 1)
		assert.Equal(t, "aws:ec2:Instance", got.ResourceType)
		assert.Equal(t, "per_gb_month", got.Modes[0].BillingMode)
		assert.Equal(t, "aws-plugin", got.Modes[0].Plugin)
		assert.InDelta(t, 0.023, got.Modes[0].RatePerUnit, 1e-9)
		assert.Equal(t, []string{"730 hours/month"}, got.Modes[0].Assumptions)
		require.Len(t, got.Modes[0].MetricHints, 1)
		assert.Equal(t, "storage_gb", got.Modes[0].MetricHints[0].Metric)
		require.Len(t, got.Modes[0].Tiers, 2)
		assert.Equal(t, "first 100 GB", got.Modes[0].Tiers[0].Description)
		assert.InDelta(t, 0.0, got.Modes[0].Tiers[1].MaxQuantity, 1e-9)
		assert.Equal(t, 1, plugin.specCalls)
		assert.Equal(t, "i-123", plugin.lastSpecID)
		assert.Equal(t, got.Modes[0].BillingMode, again.Modes[0].BillingMode)
		got.Modes[0].Assumptions[0] = "changed"
		cached := eng.DiscoverPricingSpec(context.Background(), &first)
		assert.Equal(t, "730 hours/month", cached.Modes[0].Assumptions[0])
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("different type calls the plugin again", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: spec}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		first := pricingResource()
		other := first
		other.Type = "aws:s3:Bucket"
		_ = eng.DiscoverPricingSpec(context.Background(), &first)
		got := eng.DiscoverPricingSpec(context.Background(), &other)
		assert.Equal(t, "aws:s3:Bucket", got.ResourceType)
		assert.Equal(t, 2, plugin.specCalls)
	})

	t.Run("rpc error is cached as no modes", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{specErr: errors.New("unimplemented")}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		resource := pricingResource()
		got := eng.DiscoverPricingSpec(context.Background(), &resource)
		again := eng.DiscoverPricingSpec(context.Background(), &resource)
		assert.Empty(t, got.Modes)
		assert.Empty(t, again.Modes)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("not implemented is hidden", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: &pbc.PricingSpec{BillingMode: "not_implemented", RatePerUnit: 1}}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		resource := pricingResource()
		got := eng.DiscoverPricingSpec(context.Background(), &resource)
		assert.Empty(t, got.Modes)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("zero rate stays selectable", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: &pbc.PricingSpec{BillingMode: "per_hour", RatePerUnit: 0, Unit: "hour"}}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		resource := pricingResource()
		got := eng.DiscoverPricingSpec(context.Background(), &resource)
		require.Len(t, got.Modes, 1)
		assert.InDelta(t, 0.0, got.Modes[0].RatePerUnit, 1e-9)
	})

	t.Run("second plugin supplies the mode when the first misses", func(t *testing.T) {
		t.Parallel()
		first := &pricingSpecPlugin{specErr: errors.New("no spec")}
		second := &pricingSpecPlugin{spec: spec}
		eng := New([]*pluginhost.Client{
			{Name: "missing", API: first},
			{Name: "aws-plugin", API: second},
		}, nil)
		resource := pricingResource()
		got := eng.DiscoverPricingSpec(context.Background(), &resource)
		require.Len(t, got.Modes, 1)
		assert.Equal(t, "aws-plugin", got.Modes[0].Plugin)
		assert.Equal(t, 1, first.specCalls)
		assert.Equal(t, 1, second.specCalls)
	})

	t.Run("nil resource and cancelled context do not call", func(t *testing.T) {
		t.Parallel()
		plugin := &pricingSpecPlugin{spec: spec}
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		assert.Empty(t, eng.DiscoverPricingSpec(context.Background(), nil).Modes)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resource := pricingResource()
		assert.Empty(t, eng.DiscoverPricingSpec(ctx, &resource).Modes)
		assert.Equal(t, 0, plugin.specCalls)
		assert.Empty(t, (*Engine)(nil).DiscoverPricingSpec(context.Background(), &resource).Modes)
	})
}

type estimateAndSpecPlugin struct {
	pricingSpecPlugin

	monthly float64
}

func (p *estimateAndSpecPlugin) EstimateCost(
	_ context.Context,
	_ *pbc.EstimateCostRequest,
	_ ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
	return &pbc.EstimateCostResponse{CostMonthly: p.monthly, Currency: "USD"}, nil
}

func TestDiscoverPricingSpec_EstimateStillRunsWhenSpecMisses(t *testing.T) {
	t.Parallel()

	plugin := &estimateAndSpecPlugin{
		pricingSpecPlugin: pricingSpecPlugin{specErr: errors.New("unimplemented")},
		monthly:           12.5,
	}
	eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
	resource := pricingResource()
	got := eng.DiscoverPricingSpec(context.Background(), &resource)
	assert.Empty(t, got.Modes)

	result, err := eng.EstimateCost(context.Background(), &EstimateRequest{
		Resource:          &resource,
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Modified)
	assert.InDelta(t, 12.5, result.Modified.Monthly, 1e-9)
	assert.Equal(t, 1, plugin.specCalls)
}
