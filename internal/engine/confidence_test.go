package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

// TestConfidenceConstants validates that confidence level constants are defined.
func TestConfidenceConstants(t *testing.T) {
	t.Parallel()

	// Verify constants exist and have expected values
	assert.Equal(t, engine.ConfidenceHigh, engine.Confidence("high"))
	assert.Equal(t, engine.ConfidenceMedium, engine.Confidence("medium"))
	assert.Equal(t, engine.ConfidenceLow, engine.Confidence("low"))
	assert.Equal(t, engine.ConfidenceUnknown, engine.Confidence(""))
}

// TestConfidenceIsValid tests the IsValid method on Confidence type.
func TestConfidenceIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		confidence engine.Confidence
		wantValid  bool
	}{
		{
			name:       "high is valid",
			confidence: engine.ConfidenceHigh,
			wantValid:  true,
		},
		{
			name:       "medium is valid",
			confidence: engine.ConfidenceMedium,
			wantValid:  true,
		},
		{
			name:       "low is valid",
			confidence: engine.ConfidenceLow,
			wantValid:  true,
		},
		{
			name:       "unknown is valid (empty string)",
			confidence: engine.ConfidenceUnknown,
			wantValid:  true,
		},
		{
			name:       "invalid confidence string",
			confidence: engine.Confidence("invalid"),
			wantValid:  false,
		},
		{
			name:       "uppercase is invalid (must be lowercase)",
			confidence: engine.Confidence("HIGH"),
			wantValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantValid, tt.confidence.IsValid())
		})
	}
}

// TestConfidenceString tests the String method on Confidence type.
func TestConfidenceString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		confidence engine.Confidence
		want       string
	}{
		{
			name:       "high to string",
			confidence: engine.ConfidenceHigh,
			want:       "high",
		},
		{
			name:       "medium to string",
			confidence: engine.ConfidenceMedium,
			want:       "medium",
		},
		{
			name:       "low to string",
			confidence: engine.ConfidenceLow,
			want:       "low",
		},
		{
			name:       "unknown to empty string",
			confidence: engine.ConfidenceUnknown,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()

	tests := []struct {
		name           string
		hasBillingData bool // true if data came from actual billing API
		isExternal     bool // true if resource was imported
		want           engine.Confidence
	}{
		{
			name:           "high confidence - real billing data",
			hasBillingData: true,
			isExternal:     false,
			want:           engine.ConfidenceHigh,
		},
		{
			name:           "high confidence - billing data for external resource",
			hasBillingData: true,
			isExternal:     true, // External flag is irrelevant when we have billing data
			want:           engine.ConfidenceHigh,
		},
		{
			name:           "medium confidence - runtime estimate, non-external",
			hasBillingData: false,
			isExternal:     false,
			want:           engine.ConfidenceMedium,
		},
		{
			name:           "low confidence - runtime estimate, external/imported",
			hasBillingData: false,
			isExternal:     true,
			want:           engine.ConfidenceLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := engine.DetermineConfidence(tt.hasBillingData, tt.isExternal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDetermineConfidenceFromResult tests confidence determination from a CostResult.
// This is useful when we have a completed cost calculation and need to set confidence.
func TestDetermineConfidenceFromResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     engine.CostResult
		isExternal bool
		want       engine.Confidence
	}{
		{
			name: "high confidence - has TotalCost from billing",
			result: engine.CostResult{
				TotalCost: 150.00,
				Adapter:   "kubecost",
			},
			isExternal: false,
			want:       engine.ConfidenceHigh,
		},
		{
			name: "medium confidence - monthly estimate, non-external",
			result: engine.CostResult{
				Monthly: 50.00,
				Hourly:  0.0685,
				Adapter: "local-spec",
			},
			isExternal: false,
			want:       engine.ConfidenceMedium,
		},
		{
			name: "low confidence - monthly estimate, external resource",
			result: engine.CostResult{
				Monthly: 50.00,
				Hourly:  0.0685,
				Adapter: "local-spec",
			},
			isExternal: true,
			want:       engine.ConfidenceLow,
		},
		{
			name: "high confidence - TotalCost overrides external flag",
			result: engine.CostResult{
				TotalCost: 200.00,
				Adapter:   "vantage",
			},
			isExternal: true, // Ignored when TotalCost > 0
			want:       engine.ConfidenceHigh,
		},
		{
			name: "medium confidence - zero TotalCost, non-external",
			result: engine.CostResult{
				TotalCost: 0.0,
				Monthly:   25.00,
			},
			isExternal: false,
			want:       engine.ConfidenceMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := engine.DetermineConfidenceFromResult(tt.result, tt.isExternal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCostResultConfidenceField tests that CostResult has a Confidence field.
func TestCostResultConfidenceField(t *testing.T) {
	t.Parallel()

	result := engine.CostResult{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-12345",
		Monthly:      50.00,
		Confidence:   engine.ConfidenceHigh,
	}

	assert.Equal(t, engine.ConfidenceHigh, result.Confidence)
	assert.Equal(t, "high", result.Confidence.String())
}

// TestConfidenceDisplayLabel tests human-readable display labels for UI.
func TestConfidenceDisplayLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		confidence engine.Confidence
		want       string
	}{
		{engine.ConfidenceHigh, "HIGH"},
		{engine.ConfidenceMedium, "MEDIUM"},
		{engine.ConfidenceLow, "LOW"},
		{engine.ConfidenceUnknown, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.confidence.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.confidence.DisplayLabel())
		})
	}
}
