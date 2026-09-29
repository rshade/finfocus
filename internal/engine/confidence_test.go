package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfidenceConstants validates that confidence level constants are defined.
func TestConfidenceConstants(t *testing.T) {
	// Verify constants exist and have expected values
	assert.Equal(t, ConfidenceHigh, Confidence("high"))
	assert.Equal(t, ConfidenceMedium, Confidence("medium"))
	assert.Equal(t, ConfidenceLow, Confidence("low"))
	assert.Equal(t, ConfidenceUnknown, Confidence(""))
}

// TestConfidenceIsValid tests the IsValid method on Confidence type.
func TestConfidenceIsValid(t *testing.T) {
	tests := []struct {
		name       string
		confidence Confidence
		wantValid  bool
	}{
		{
			name:       "high is valid",
			confidence: ConfidenceHigh,
			wantValid:  true,
		},
		{
			name:       "medium is valid",
			confidence: ConfidenceMedium,
			wantValid:  true,
		},
		{
			name:       "low is valid",
			confidence: ConfidenceLow,
			wantValid:  true,
		},
		{
			name:       "unknown is valid (empty string)",
			confidence: ConfidenceUnknown,
			wantValid:  true,
		},
		{
			name:       "invalid confidence string",
			confidence: Confidence("invalid"),
			wantValid:  false,
		},
		{
			name:       "uppercase is invalid (must be lowercase)",
			confidence: Confidence("HIGH"),
			wantValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantValid, tt.confidence.IsValid())
		})
	}
}

// TestConfidenceString tests the String method on Confidence type.
func TestConfidenceString(t *testing.T) {
	tests := []struct {
		name       string
		confidence Confidence
		want       string
	}{
		{
			name:       "high to string",
			confidence: ConfidenceHigh,
			want:       "high",
		},
		{
			name:       "medium to string",
			confidence: ConfidenceMedium,
			want:       "medium",
		},
		{
			name:       "low to string",
			confidence: ConfidenceLow,
			want:       "low",
		},
		{
			name:       "unknown to empty string",
			confidence: ConfidenceUnknown,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.confidence.String())
		})
	}
}

// TestDetermineConfidence tests the confidence level determination logic.
// Per spec:
//   - HIGH: Real billing data from plugin (TotalCost > 0 from actual billing API)
//   - MEDIUM: Runtime-based estimate where External=false
//   - LOW: Runtime-based estimate where External=true (imported resources)
func TestDetermineConfidence(t *testing.T) {
	tests := []struct {
		name           string
		hasBillingData bool // true if data came from actual billing API
		isExternal     bool // true if resource was imported
		want           Confidence
	}{
		{
			name:           "high confidence - real billing data",
			hasBillingData: true,
			isExternal:     false,
			want:           ConfidenceHigh,
		},
		{
			name:           "high confidence - billing data for external resource",
			hasBillingData: true,
			isExternal:     true, // External flag is irrelevant when we have billing data
			want:           ConfidenceHigh,
		},
		{
			name:           "medium confidence - runtime estimate, non-external",
			hasBillingData: false,
			isExternal:     false,
			want:           ConfidenceMedium,
		},
		{
			name:           "low confidence - runtime estimate, external/imported",
			hasBillingData: false,
			isExternal:     true,
			want:           ConfidenceLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineConfidence(tt.hasBillingData, tt.isExternal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDetermineConfidenceFromResult tests confidence determination from a CostResult.
// This is useful when we have a completed cost calculation and need to set confidence.
func TestDetermineConfidenceFromResult(t *testing.T) {
	tests := []struct {
		name       string
		result     CostResult
		isExternal bool
		want       Confidence
	}{
		{
			name: "high confidence - has TotalCost from billing",
			result: CostResult{
				TotalCost: 150.00,
				Adapter:   "kubecost",
			},
			isExternal: false,
			want:       ConfidenceHigh,
		},
		{
			name: "medium confidence - monthly estimate, non-external",
			result: CostResult{
				Monthly: 50.00,
				Hourly:  0.0685,
				Adapter: "local-spec",
			},
			isExternal: false,
			want:       ConfidenceMedium,
		},
		{
			name: "low confidence - monthly estimate, external resource",
			result: CostResult{
				Monthly: 50.00,
				Hourly:  0.0685,
				Adapter: "local-spec",
			},
			isExternal: true,
			want:       ConfidenceLow,
		},
		{
			name: "high confidence - TotalCost overrides external flag",
			result: CostResult{
				TotalCost: 200.00,
				Adapter:   "vantage",
			},
			isExternal: true, // Ignored when TotalCost > 0
			want:       ConfidenceHigh,
		},
		{
			name: "medium confidence - zero TotalCost, non-external",
			result: CostResult{
				TotalCost: 0.0,
				Monthly:   25.00,
			},
			isExternal: false,
			want:       ConfidenceMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineConfidenceFromResult(tt.result, tt.isExternal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCostResultConfidenceField tests that CostResult has a Confidence field.
func TestCostResultConfidenceField(t *testing.T) {
	result := CostResult{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-12345",
		Monthly:      50.00,
		Confidence:   ConfidenceHigh,
	}

	assert.Equal(t, ConfidenceHigh, result.Confidence)
	assert.Equal(t, "high", result.Confidence.String())
}

// TestConfidenceDisplayLabel tests human-readable display labels for UI.
func TestConfidenceDisplayLabel(t *testing.T) {
	tests := []struct {
		confidence Confidence
		want       string
	}{
		{ConfidenceHigh, "HIGH"},
		{ConfidenceMedium, "MEDIUM"},
		{ConfidenceLow, "LOW"},
		{ConfidenceUnknown, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.confidence.String(), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.confidence.DisplayLabel())
		})
	}
}
