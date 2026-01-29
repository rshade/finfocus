package greenops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name           string
		input          CarbonInput
		wantMiles      float64
		wantPhones     float64
		wantIsEmpty    bool
		wantErr        bool
		errType        error
		checkDisplay   bool
		displayContain string
		compactContain string
	}{
		// Reference values from spec (SC-002: 1% margin verification)
		{
			name:           "150kg reference value - miles",
			input:          CarbonInput{Value: 150.0, Unit: "kg"},
			wantMiles:      781.25, // 150 / 0.192 = 781.25
			wantPhones:     18248.18,
			wantIsEmpty:    false,
			checkDisplay:   true,
			displayContain: "driving",
			compactContain: "mi",
		},
		{
			name:           "150kg reference value - smartphones",
			input:          CarbonInput{Value: 150.0, Unit: "kg"},
			wantMiles:      781.25,
			wantPhones:     18248.18, // 150 / 0.00822 = 18248.18
			wantIsEmpty:    false,
			checkDisplay:   true,
			displayContain: "smartphones",
			compactContain: "phones",
		},
		// Unit normalization verification
		{
			name:        "grams normalized correctly",
			input:       CarbonInput{Value: 150000.0, Unit: "g"},
			wantMiles:   781.25,
			wantPhones:  18248.18,
			wantIsEmpty: false,
		},
		{
			name:        "metric tons normalized correctly",
			input:       CarbonInput{Value: 0.15, Unit: "t"},
			wantMiles:   781.25,
			wantPhones:  18248.18,
			wantIsEmpty: false,
		},
		// Edge cases
		{
			name:        "below threshold returns empty",
			input:       CarbonInput{Value: 0.5, Unit: "kg"},
			wantIsEmpty: true,
		},
		{
			name:        "exactly at threshold",
			input:       CarbonInput{Value: 1.0, Unit: "kg"},
			wantMiles:   5.208333, // 1 / 0.192
			wantPhones:  121.65,   // 1 / 0.00822
			wantIsEmpty: false,
		},
		{
			name:        "zero value returns empty",
			input:       CarbonInput{Value: 0.0, Unit: "kg"},
			wantIsEmpty: true,
		},
		{
			name:    "negative value returns error",
			input:   CarbonInput{Value: -100.0, Unit: "kg"},
			wantErr: true,
			errType: ErrNegativeValue,
		},
		{
			name:    "invalid unit returns error",
			input:   CarbonInput{Value: 100.0, Unit: "invalid"},
			wantErr: true,
			errType: ErrInvalidUnit,
		},
		// Large values
		{
			name:        "large value (1 million kg)",
			input:       CarbonInput{Value: 1000000.0, Unit: "kg"},
			wantMiles:   5208333.33, // ~5.2 million miles
			wantPhones:  121654501.22,
			wantIsEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calculate(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				assert.True(t, got.IsEmpty, "IsEmpty should be true on error")
				return
			}

			require.NoError(t, err)

			if tt.wantIsEmpty {
				assert.True(t, got.IsEmpty, "expected IsEmpty to be true")
				return
			}

			assert.False(t, got.IsEmpty, "expected IsEmpty to be false")
			require.Len(t, got.Results, 2, "expected 2 equivalency results")

			// Verify miles driven (1% margin per SC-002)
			milesResult := got.Results[0]
			assert.Equal(t, EquivalencyMilesDriven, milesResult.Type)
			assert.InDelta(t, tt.wantMiles, milesResult.Value, tt.wantMiles*0.01,
				"miles should be within 1%% margin")
			assert.Equal(t, "miles driven", milesResult.Label)

			// Verify smartphones charged (1% margin per SC-002)
			phonesResult := got.Results[1]
			assert.Equal(t, EquivalencySmartphonesCharged, phonesResult.Type)
			assert.InDelta(t, tt.wantPhones, phonesResult.Value, tt.wantPhones*0.01,
				"phones should be within 1%% margin")
			assert.Equal(t, "smartphones charged", phonesResult.Label)

			// Verify display text format
			if tt.checkDisplay {
				assert.Contains(t, got.DisplayText, tt.displayContain)
				assert.Contains(t, got.CompactText, tt.compactContain)
			}
		})
	}
}

func TestCalculateFromMap(t *testing.T) {
	tests := []struct {
		name        string
		metrics     map[string]SustainabilityMetric
		wantMiles   float64
		wantIsEmpty bool
	}{
		{
			name: "canonical key carbon_footprint",
			metrics: map[string]SustainabilityMetric{
				"carbon_footprint": {Value: 150.0, Unit: "kg"},
			},
			wantMiles:   781.25,
			wantIsEmpty: false,
		},
		{
			name: "deprecated key gCO2e with warning",
			metrics: map[string]SustainabilityMetric{
				"gCO2e": {Value: 150.0, Unit: "kg"},
			},
			wantMiles:   781.25,
			wantIsEmpty: false,
		},
		{
			name: "canonical takes precedence over deprecated",
			metrics: map[string]SustainabilityMetric{
				"carbon_footprint": {Value: 150.0, Unit: "kg"},
				"gCO2e":            {Value: 300.0, Unit: "kg"}, // Should be ignored
			},
			wantMiles:   781.25, // Uses carbon_footprint value
			wantIsEmpty: false,
		},
		{
			name: "no carbon metric returns empty",
			metrics: map[string]SustainabilityMetric{
				"energy_consumption": {Value: 2000.0, Unit: "kWh"},
			},
			wantIsEmpty: true,
		},
		{
			name:        "empty map returns empty",
			metrics:     map[string]SustainabilityMetric{},
			wantIsEmpty: true,
		},
		{
			name:        "nil map returns empty",
			metrics:     nil,
			wantIsEmpty: true,
		},
		{
			name: "below threshold returns empty",
			metrics: map[string]SustainabilityMetric{
				"carbon_footprint": {Value: 0.5, Unit: "kg"},
			},
			wantIsEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateFromMap(tt.metrics)

			if tt.wantIsEmpty {
				assert.True(t, got.IsEmpty, "expected IsEmpty to be true")
				return
			}

			assert.False(t, got.IsEmpty, "expected IsEmpty to be false")
			require.Len(t, got.Results, 2)

			// Verify miles calculation within 1% margin
			milesResult := got.Results[0]
			assert.InDelta(t, tt.wantMiles, milesResult.Value, tt.wantMiles*0.01)
		})
	}
}

func TestCalculate_DisplayTextFormat(t *testing.T) {
	// Test display text formatting per FR-003 and FR-007
	input := CarbonInput{Value: 150.0, Unit: "kg"}
	got, err := Calculate(input)
	require.NoError(t, err)

	// FR-003: Must use "Equivalent to" or "Approx." labeling
	assert.Contains(t, got.DisplayText, "Equivalent to")

	// FR-005: Number formatting with thousand separators
	assert.Contains(t, got.DisplayText, "18,248") // smartphones with comma

	// FR-007: Concise display (verify reasonable length)
	assert.Less(t, len(got.DisplayText), 100, "display text should be concise")

	// Compact format for analyzer
	assert.Contains(t, got.CompactText, "≈")
	assert.Contains(t, got.CompactText, "mi")
	assert.Contains(t, got.CompactText, "phones")
}

func TestCalculate_LargeNumberFormatting(t *testing.T) {
	// Test large number scaling per research.md thresholds
	input := CarbonInput{Value: 10000000.0, Unit: "kg"} // 10 million kg
	got, err := Calculate(input)
	require.NoError(t, err)

	// Should use "million" scaling for large values
	assert.Contains(t, got.DisplayText, "million")
}

func TestCalculate_VeryLargeNumberFormatting(t *testing.T) {
	// Test billion-scale formatting
	input := CarbonInput{Value: 1000000000.0, Unit: "kg"} // 1 billion kg
	got, err := Calculate(input)
	require.NoError(t, err)

	// Should use "billion" scaling
	assert.Contains(t, got.DisplayText, "billion")
}

// Benchmarks for equivalency calculations

func BenchmarkCalculate(b *testing.B) {
	input := CarbonInput{Value: 150.0, Unit: "kg"}
	for b.Loop() {
		_, _ = Calculate(input)
	}
}

func BenchmarkCalculateFromMap(b *testing.B) {
	metrics := map[string]SustainabilityMetric{
		"carbon_footprint": {Value: 150.0, Unit: "kg"},
	}
	for b.Loop() {
		_ = CalculateFromMap(metrics)
	}
}
