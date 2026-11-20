package viewmodel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestEstimateDisplay(t *testing.T) {
	t.Parallel()
	resource := &engine.ResourceDescriptor{Properties: map[string]any{"size": 2, "password": "hidden"}}
	result := &engine.EstimateResult{
		Baseline:    &engine.CostResult{Monthly: 60, Currency: "EUR"},
		Modified:    &engine.CostResult{Monthly: 90, Currency: "EUR"},
		TotalChange: 30,
		Deltas:      []engine.CostDelta{{Property: "size", CostChange: 30}},
	}
	got := BuildEstimateDisplay(context.Background(), resource, result, map[string]string{"size": "3"}, nil)
	assert.Equal(t, "€60.00/mo (EUR)", got.Baseline)
	assert.Equal(t, "€90.00/mo (EUR)", got.Modified)
	assert.Equal(t, "+$30.00 ↑", got.Change)
	assert.Equal(t, "↑", got.Arrow)
	assert.Equal(
		t,
		[]EstimateProperty{{
			Key: "size", OriginalValue: "2", CurrentValue: "3", Delta: "+$30.00 ↑",
			Class: PropertyClassOther, Editable: true, DefaultVisible: true, Overridden: true,
		}},
		got.Properties,
	)
	assert.False(t, got.Curated)
	assert.Equal(t, EstimateFallbackNote, got.Note)
	assert.Empty(t, got.Details)
	for _, tc := range []struct {
		delta float64
		want  string
	}{{-1, "$1.00 ↓"}, {0, "$0.00 →"}, {0.001, "$0.00 →"}, {2.345, "+$2.35 ↑"}} {
		assert.Equal(t, tc.want, FormatEstimateDelta(tc.delta).Text)
	}
}

func TestEstimatePricingDisplay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mode engine.PricingMode
		want string
	}{
		{mode: engine.PricingMode{BillingMode: "hourly", RatePerUnit: 0.023}, want: "USD 0.023/hourly"},
		{mode: engine.PricingMode{Currency: "EUR", Unit: "GB-month", RatePerUnit: 2}, want: "EUR 2/GB-month"},
	} {
		assert.Equal(t, tc.want, FormatPricingRate(tc.mode))
	}
	assert.Empty(t, BuildEstimateDisplay(context.Background(), nil, nil, nil, nil).Properties)
	assert.Equal(
		t,
		"$0.00 →",
		BuildEstimateDisplay(context.Background(), nil, &engine.EstimateResult{}, nil, nil).Change,
	)
}

func TestEstimatePricingDetails(t *testing.T) {
	t.Parallel()
	mode := engine.PricingMode{
		Tiers: []engine.PricingTierOption{
			{MinQuantity: 0, MaxQuantity: 100, RatePerUnit: 0.023, Description: "first tier"},
			{MinQuantity: 100, RatePerUnit: 0.02},
		},
		Assumptions: []string{"730 hours"},
		MetricHints: []engine.PricingMetricHint{{Metric: "size", Unit: "GB"}, {Metric: "requests"}},
	}
	assert.Equal(
		t,
		"Pricing tiers:\n  0-100: 0.023 first tier\n  100-+: 0.02 \nAssumptions:\n  - 730 hours\nUsage:\n  size (GB)\n  requests\n",
		FormatPricingDetails(mode),
	)
	assert.Empty(t, FormatPricingDetails(engine.PricingMode{}))
}

func TestEstimatePropertyProjection(t *testing.T) {
	t.Parallel()
	properties := map[string]any{"zeta": "last", "alpha": 2, "middle": true}
	overrides := map[string]string{"middle": "false"}
	for _, tc := range []struct {
		name   string
		deltas []engine.CostDelta
		want   float64
	}{
		{name: "single", deltas: []engine.CostDelta{{Property: "middle", CostChange: 12}}, want: 12},
		{name: "combined", deltas: []engine.CostDelta{{Property: "combined", CostChange: 30}}},
		{name: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rows := BuildEstimatePropertyRows(properties, overrides)
			require.Len(t, rows, 3)
			for i := range rows {
				rows[i].CostDelta = 99
			}
			ApplyEstimateDeltas(rows, tc.deltas)
			assert.Equal(t, []EstimatePropertyRow{
				{Key: "alpha", OriginalValue: "2", CurrentValue: "2"},
				{Key: "middle", OriginalValue: "true", CurrentValue: "false", CostDelta: tc.want},
				{Key: "zeta", OriginalValue: "last", CurrentValue: "last"},
			}, rows)
			display := BuildEstimateDisplay(
				context.Background(),
				&engine.ResourceDescriptor{Properties: properties},
				&engine.EstimateResult{Deltas: tc.deltas},
				overrides,
				nil,
			)
			require.Len(t, display.Properties, 3)
			for i, row := range rows {
				assert.Equal(t, row.Key, display.Properties[i].Key)
				assert.Equal(t, row.OriginalValue, display.Properties[i].OriginalValue)
				assert.Equal(t, row.CurrentValue, display.Properties[i].CurrentValue)
				assert.Equal(t, FormatEstimateDelta(row.CostDelta).Text, display.Properties[i].Delta)
			}
		})
	}
}

func TestEstimateDisplayCuratedCarriesPlanFields(t *testing.T) {
	t.Parallel()
	resource := &engine.ResourceDescriptor{
		Type: "aws:ec2/instance:Instance",
		Properties: map[string]any{
			"instanceType": "t3.micro", "ami": "ami-1", "arn": "arn:aws:ec2:us-east-1:1:instance/i-1",
		},
	}
	result := &engine.EstimateResult{Deltas: []engine.CostDelta{{Property: "instanceType", CostChange: 10}}}

	got := BuildEstimateDisplay(context.Background(), resource, result, nil, nil)

	require.Len(t, got.Properties, 2)
	assert.Equal(t, "instanceType", got.Properties[0].Key)
	assert.Equal(t, PropertyClassPricing, got.Properties[0].Class)
	assert.Equal(t, ProvenanceSKU, got.Properties[0].Provenance)
	assert.True(t, got.Properties[0].DefaultVisible)
	assert.Equal(t, "+$10.00 ↑", got.Properties[0].Delta)
	assert.Equal(t, "ami", got.Properties[1].Key)
	assert.False(t, got.Properties[1].DefaultVisible)
	assert.Equal(t, []EstimateDetail{{Key: "arn", Value: "arn:aws:ec2:us-east-1:1:instance/i-1"}}, got.Details)
	assert.True(t, got.Curated)
	assert.Empty(t, got.Note)
}

func TestNewEstimatePropertyDisplayWithoutDeltas(t *testing.T) {
	t.Parallel()
	plan := planFor(ec2Type, map[string]any{"instanceType": "t3.micro"}, nil, nil)

	got := NewEstimatePropertyDisplay(plan, []engine.CostDelta{{Property: "instanceType", CostChange: 5}}, false)

	require.Len(t, got.Properties, 1)
	assert.Empty(t, got.Properties[0].Delta)
	assert.NotNil(t, got.Details)
	assert.True(t, got.Curated)
}

func TestUsageHintsFlattensEveryMode(t *testing.T) {
	t.Parallel()

	found := engine.PricingDiscovery{Modes: []engine.PricingMode{
		{MetricHints: []engine.PricingMetricHint{{Metric: "a"}, {Metric: "b"}}},
		{},
		{MetricHints: []engine.PricingMetricHint{{Metric: "c", Unit: "GB"}}},
	}}

	assert.Equal(
		t,
		[]engine.PricingMetricHint{{Metric: "a"}, {Metric: "b"}, {Metric: "c", Unit: "GB"}},
		UsageHints(found),
	)
	assert.Empty(t, UsageHints(engine.PricingDiscovery{}))
}
