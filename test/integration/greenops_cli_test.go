// Package integration contains integration tests for FinFocus components.
package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// TestGreenOps_CLIEquivalencyOutput tests User Story 1: Carbon equivalencies in CLI output.
// This integration test verifies that when cost results contain carbon_footprint metrics,
// the CLI table output displays real-world equivalencies using EPA formulas.
func TestGreenOps_CLIEquivalencyOutput(t *testing.T) {
	// Create sample cost results with carbon footprint data
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-12345",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      73.00,
			Hourly:       0.10,
			Sustainability: map[string]engine.SustainabilityMetric{
				"carbon_footprint": {Value: 150.0, Unit: "kg"},
			},
		},
	}

	// Render to table format
	var buf strings.Builder
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()

	// Verify the output structure contains expected sections
	assert.Contains(t, output, "COST SUMMARY")
	assert.Contains(t, output, "SUSTAINABILITY SUMMARY")

	// FR-001: Verify equivalencies are displayed
	assert.Contains(t, output, "Equivalent to")

	// FR-004: Verify primary equivalencies (miles and smartphones)
	assert.Contains(t, output, "miles")
	assert.Contains(t, output, "smartphones")

	// FR-005: Verify number formatting with commas
	// 150 kg → ~18,248 smartphones (150 / 0.00822)
	assert.Contains(t, output, "18,248")

	// Verify carbon metric is displayed
	assert.Contains(t, output, "carbon_footprint")
	assert.Contains(t, output, "150.00 kg")
}

// TestGreenOps_CLIEquivalencyOmittedWhenZero tests FR-002: graceful omission.
// Equivalencies should not be displayed when carbon emissions are zero or below threshold.
func TestGreenOps_CLIEquivalencyOmittedWhenZero(t *testing.T) {
	tests := []struct {
		name        string
		carbonValue float64
		expectEquiv bool
		description string
	}{
		{
			name:        "zero carbon",
			carbonValue: 0.0,
			expectEquiv: false,
			description: "Zero carbon should show no equivalencies",
		},
		{
			name:        "below threshold",
			carbonValue: 0.5,
			expectEquiv: false,
			description: "Below 1kg threshold should show no equivalencies",
		},
		{
			name:        "at threshold",
			carbonValue: 1.0,
			expectEquiv: true,
			description: "Exactly at threshold should show equivalencies",
		},
		{
			name:        "above threshold",
			carbonValue: 150.0,
			expectEquiv: true,
			description: "Above threshold should show equivalencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []engine.CostResult{
				{
					ResourceType: "aws:ec2:Instance",
					ResourceID:   "i-12345",
					Adapter:      "kubecost",
					Currency:     "USD",
					Monthly:      73.00,
					Sustainability: map[string]engine.SustainabilityMetric{
						"carbon_footprint": {Value: tt.carbonValue, Unit: "kg"},
					},
				},
			}

			var buf strings.Builder
			err := engine.RenderResults(&buf, engine.OutputTable, results)
			require.NoError(t, err)

			output := buf.String()

			if tt.expectEquiv {
				assert.Contains(t, output, "Equivalent to", tt.description)
			} else {
				assert.NotContains(t, output, "Equivalent to", tt.description)
			}
		})
	}
}

// TestGreenOps_CLINoSustainabilityData tests graceful handling when no sustainability data exists.
func TestGreenOps_CLINoSustainabilityData(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType:   "aws:ec2:Instance",
			ResourceID:     "i-12345",
			Adapter:        "kubecost",
			Currency:       "USD",
			Monthly:        73.00,
			Sustainability: nil, // No sustainability data
		},
	}

	var buf strings.Builder
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()

	// Should not crash and should not show sustainability section
	assert.Contains(t, output, "COST SUMMARY")
	assert.NotContains(t, output, "SUSTAINABILITY SUMMARY")
	assert.NotContains(t, output, "Equivalent to")
}

// TestGreenOps_CLIMultipleResourcesAggregation tests aggregation of carbon from multiple resources.
func TestGreenOps_CLIMultipleResourcesAggregation(t *testing.T) {
	// Two resources, each with 75 kg carbon = 150 kg total
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-12345",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      73.00,
			Sustainability: map[string]engine.SustainabilityMetric{
				"carbon_footprint": {Value: 75.0, Unit: "kg"},
			},
		},
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-67890",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      73.00,
			Sustainability: map[string]engine.SustainabilityMetric{
				"carbon_footprint": {Value: 75.0, Unit: "kg"},
			},
		},
	}

	var buf strings.Builder
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()

	// Total should be 150 kg
	assert.Contains(t, output, "150.00 kg")

	// Equivalencies should be calculated on aggregated total
	assert.Contains(t, output, "Equivalent to")
	assert.Contains(t, output, "18,248") // ~18,248 smartphones for 150 kg
}

// TestGreenOps_CLILargeNumberFormatting tests FR-005: large number scaling.
func TestGreenOps_CLILargeNumberFormatting(t *testing.T) {
	// 10 million kg of carbon should produce "million" formatted output
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "data-center-1",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      10000000.00,
			Sustainability: map[string]engine.SustainabilityMetric{
				"carbon_footprint": {Value: 10000000.0, Unit: "kg"}, // 10 million kg
			},
		},
	}

	var buf strings.Builder
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()

	// Large values should use "million" scaling
	assert.Contains(t, output, "million")
}

// TestGreenOps_CLIOnlyEnergyNoCarbon tests behavior with only energy metrics, no carbon.
func TestGreenOps_CLIOnlyEnergyNoCarbon(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-12345",
			Adapter:      "kubecost",
			Currency:     "USD",
			Monthly:      73.00,
			Sustainability: map[string]engine.SustainabilityMetric{
				"energy_consumption": {Value: 2000.0, Unit: "kWh"},
			},
		},
	}

	var buf strings.Builder
	err := engine.RenderResults(&buf, engine.OutputTable, results)
	require.NoError(t, err)

	output := buf.String()

	// Should show sustainability section with energy
	assert.Contains(t, output, "SUSTAINABILITY SUMMARY")
	assert.Contains(t, output, "energy_consumption")
	assert.Contains(t, output, "2000.00 kWh")

	// Should NOT show equivalencies (no carbon data)
	assert.NotContains(t, output, "Equivalent to")
	assert.NotContains(t, output, "miles")
}
