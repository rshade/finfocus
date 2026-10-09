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
	got := BuildEstimateDisplay(context.Background(), resource, result, map[string]string{"size": "3"})
	assert.Equal(t, "€60.00/mo (EUR)", got.Baseline)
	assert.Equal(t, "€90.00/mo (EUR)", got.Modified)
	assert.Equal(t, "+$30.00 ↑", got.Change)
	assert.Equal(t, "↑", got.Arrow)
	assert.Equal(
		t,
		[]EstimateProperty{{Key: "size", OriginalValue: "2", CurrentValue: "3", Delta: "+$30.00 ↑"}},
		got.Properties,
	)
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
	assert.Empty(t, BuildEstimateDisplay(context.Background(), nil, nil, nil).Properties)
	assert.Equal(t, "$0.00 →", BuildEstimateDisplay(context.Background(), nil, &engine.EstimateResult{}, nil).Change)
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
