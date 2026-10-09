package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/greenops"
)

// TestAggregateSustainability pins the behavior extracted from the TUI's
// aggregateCarbonFromResults: canonical key preferred over the deprecated key,
// mixed units normalized to kilograms, unnormalizable units skipped.
func TestAggregateSustainability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		results   []CostResult
		wantKg    float64
		wantFound bool
		wantUnit  string
	}{
		{
			name:      "no sustainability data",
			results:   []CostResult{{ResourceID: "i-1"}},
			wantFound: false,
			wantUnit:  "",
		},
		{
			name: "mixed units normalized to kg",
			results: []CostResult{
				{ResourceID: "i-2", Sustainability: map[string]SustainabilityMetric{
					greenops.CarbonMetricKey: {Value: 500, Unit: "g"},
				}},
				{ResourceID: "i-3", Sustainability: map[string]SustainabilityMetric{
					greenops.CarbonMetricKey: {Value: 2.5, Unit: "kg"},
				}},
				{ResourceID: "i-4", Sustainability: map[string]SustainabilityMetric{
					greenops.DeprecatedCarbonKey: {Value: 0.001, Unit: "t"},
				}},
			},
			wantKg:    4.0,
			wantFound: true,
			wantUnit:  "kg",
		},
		{
			name: "canonical key preferred over deprecated",
			results: []CostResult{
				{ResourceID: "i-5", Sustainability: map[string]SustainabilityMetric{
					greenops.CarbonMetricKey:     {Value: 1, Unit: "kg"},
					greenops.DeprecatedCarbonKey: {Value: 100, Unit: "kg"},
				}},
			},
			wantKg:    1.0,
			wantFound: true,
			wantUnit:  "kg",
		},
		{
			name: "invalid units skipped",
			results: []CostResult{
				{ResourceID: "i-6", Sustainability: map[string]SustainabilityMetric{
					greenops.CarbonMetricKey: {Value: 10, Unit: "lightyears"},
				}},
				{ResourceID: "i-7", Sustainability: map[string]SustainabilityMetric{
					greenops.CarbonMetricKey: {Value: 1, Unit: "kg"},
				}},
			},
			wantKg:    1.0,
			wantFound: true,
			wantUnit:  "kg",
		},
		{
			name: "negative values skipped",
			results: []CostResult{
				{ResourceID: "i-8", Sustainability: map[string]SustainabilityMetric{
					greenops.CarbonMetricKey: {Value: -5, Unit: "kg"},
				}},
			},
			wantFound: false,
			wantUnit:  "",
		},
		{
			name: "non-carbon metrics ignored",
			results: []CostResult{
				{ResourceID: "i-9", Sustainability: map[string]SustainabilityMetric{
					"water_usage": {Value: 100, Unit: "L"},
				}},
			},
			wantFound: false,
			wantUnit:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input, found := AggregateSustainability(context.Background(), tt.results)
			assert.Equal(t, tt.wantFound, found)
			assert.InDelta(t, tt.wantKg, input.Value, 1e-9)
			assert.Equal(t, tt.wantUnit, input.Unit)
		})
	}
}

// TestAggregateSustainabilityMatchesLegacyTUI proves the unified aggregation
// produces totals identical to the legacy tui/cost_view.go loop on a shared
// fixture, replicating the legacy semantics independently.
func TestAggregateSustainabilityMatchesLegacyTUI(t *testing.T) {
	t.Parallel()

	results := []CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "web-1",
			Sustainability: map[string]SustainabilityMetric{
				greenops.CarbonMetricKey: {Value: 1500, Unit: "gCO2e"},
			},
		},
		{
			ResourceType: "aws:s3/bucket:Bucket",
			ResourceID:   "data",
			Sustainability: map[string]SustainabilityMetric{
				greenops.CarbonMetricKey: {Value: 0.002, Unit: "t"},
			},
		},
		{
			ResourceType: "aws:ebs/volume:Volume",
			ResourceID:   "old",
			Sustainability: map[string]SustainabilityMetric{
				greenops.DeprecatedCarbonKey: {Value: 750, Unit: "g"},
			},
		},
		{ResourceType: "aws:lambda/function:Function", ResourceID: "fn"},
		{
			ResourceType: "aws:rds/instance:Instance",
			ResourceID:   "db",
			Sustainability: map[string]SustainabilityMetric{
				greenops.CarbonMetricKey: {Value: 42, Unit: "furlongs"},
			},
		},
	}

	// Legacy loop copied from internal/tui/cost_view.go aggregateCarbonFromResults.
	legacyTotal := 0.0
	legacyFound := false
	for _, r := range results {
		if r.Sustainability == nil {
			continue
		}
		metric, ok := r.Sustainability[greenops.CarbonMetricKey]
		if !ok {
			metric, ok = r.Sustainability[greenops.DeprecatedCarbonKey]
		}
		if ok {
			kg, err := greenops.NormalizeToKg(metric.Value, metric.Unit)
			if err != nil {
				continue
			}
			legacyTotal += kg
			legacyFound = true
		}
	}

	input, found := AggregateSustainability(context.Background(), results)
	require.True(t, legacyFound)
	assert.Equal(t, legacyFound, found)
	assert.InDelta(t, legacyTotal, input.Value, 1e-9)
	assert.Equal(t, "kg", input.Unit)
}
