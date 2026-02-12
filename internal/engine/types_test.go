package engine

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/proto"
)

// Test GroupBy validation.
func TestGroupBy_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		groupBy  GroupBy
		expected bool
	}{
		{"valid resource", GroupByResource, true},
		{"valid type", GroupByType, true},
		{"valid provider", GroupByProvider, true},
		{"valid date", GroupByDate, true},
		{"valid daily", GroupByDaily, true},
		{"valid monthly", GroupByMonthly, true},
		{"valid none", GroupByNone, true},
		{"invalid empty string not GroupByNone", GroupBy(""), true}, // Empty string is GroupByNone
		{"invalid random", GroupBy("random"), false},
		{"invalid uppercase", GroupBy("DAILY"), false},
		{"invalid mixed case", GroupBy("Daily"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.groupBy.IsValid()
			assert.Equal(t, tt.expected, got, "IsValid() mismatch")
		})
	}
}

// Test time-based grouping detection.
func TestGroupBy_IsTimeBasedGrouping(t *testing.T) {
	tests := []struct {
		name     string
		groupBy  GroupBy
		expected bool
	}{
		{"daily is time-based", GroupByDaily, true},
		{"monthly is time-based", GroupByMonthly, true},
		{"resource is not time-based", GroupByResource, false},
		{"type is not time-based", GroupByType, false},
		{"provider is not time-based", GroupByProvider, false},
		{"date is not time-based", GroupByDate, false},
		{"none is not time-based", GroupByNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.groupBy.IsTimeBasedGrouping()
			assert.Equal(t, tt.expected, got, "IsTimeBasedGrouping() mismatch")
		})
	}
}

// Test String() method.
func TestGroupBy_String(t *testing.T) {
	tests := []struct {
		name     string
		groupBy  GroupBy
		expected string
	}{
		{"resource", GroupByResource, "resource"},
		{"type", GroupByType, "type"},
		{"provider", GroupByProvider, "provider"},
		{"date", GroupByDate, "date"},
		{"daily", GroupByDaily, "daily"},
		{"monthly", GroupByMonthly, "monthly"},
		{"none", GroupByNone, ""},
		{"empty", GroupBy(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.groupBy.String()
			assert.Equal(t, tt.expected, got, "String() mismatch")
		})
	}
}

// Test ResourceDescriptor creation.
func TestResourceDescriptor(t *testing.T) {
	rd := ResourceDescriptor{
		Type:     "aws:ec2:Instance",
		ID:       "i-123456",
		Provider: "aws",
		Properties: map[string]interface{}{
			"instanceType": "t3.micro",
			"region":       "us-east-1",
		},
	}

	assert.Equal(t, "aws:ec2:Instance", rd.Type)
	assert.Equal(t, "i-123456", rd.ID)
	assert.Equal(t, "aws", rd.Provider)
	assert.Len(t, rd.Properties, 2)

	// Verify properties
	assert.Equal(t, "t3.micro", rd.Properties["instanceType"])
	assert.Equal(t, "us-east-1", rd.Properties["region"])
}

// Test CostResult creation and defaults.
func TestCostResult(t *testing.T) {
	now := time.Now()
	endDate := now.AddDate(0, 1, 0)

	cr := CostResult{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-123456",
		Adapter:      "kubecost",
		Currency:     "USD",
		Monthly:      100.50,
		Hourly:       0.1377,
		TotalCost:    100.50,
		Notes:        "Test cost result",
		StartDate:    now,
		EndDate:      endDate,
		Breakdown: map[string]float64{
			"compute": 80.00,
			"storage": 20.50,
		},
		DailyCosts: []float64{3.35, 3.35, 3.35},
		CostPeriod: "monthly",
	}

	// Verify all fields
	assert.Equal(t, "aws:ec2:Instance", cr.ResourceType)
	assert.Equal(t, "i-123456", cr.ResourceID)
	assert.Equal(t, "kubecost", cr.Adapter)
	assert.Equal(t, "USD", cr.Currency)
	assert.Equal(t, 100.50, cr.Monthly)
	assert.Equal(t, 0.1377, cr.Hourly)
	assert.Equal(t, 100.50, cr.TotalCost)
	assert.Len(t, cr.Breakdown, 2)
	assert.Len(t, cr.DailyCosts, 3)
	assert.Equal(t, "monthly", cr.CostPeriod)
	assert.Equal(t, "Test cost result", cr.Notes)
	assert.False(t, cr.StartDate.IsZero(), "StartDate should not be zero")
	assert.False(t, cr.EndDate.IsZero(), "EndDate should not be zero")

	// Verify breakdown
	assert.Equal(t, 80.00, cr.Breakdown["compute"])
	assert.Equal(t, 20.50, cr.Breakdown["storage"])
}

// Test CrossProviderAggregation.
func TestCrossProviderAggregation(t *testing.T) {
	agg := CrossProviderAggregation{
		Period: "2024-01-15",
		Providers: map[string]float64{
			"aws":   250.00,
			"azure": 180.50,
			"gcp":   95.25,
		},
		Total:    525.75,
		Currency: "USD",
	}

	assert.Equal(t, "2024-01-15", agg.Period)
	assert.Equal(t, 525.75, agg.Total)
	assert.Equal(t, "USD", agg.Currency)
	assert.Len(t, agg.Providers, 3)

	// Verify provider costs
	assert.Equal(t, 250.00, agg.Providers["aws"])
	assert.Equal(t, 180.50, agg.Providers["azure"])
	assert.Equal(t, 95.25, agg.Providers["gcp"])

	// Verify total matches sum
	var sum float64
	for _, cost := range agg.Providers {
		sum += cost
	}
	assert.Equal(t, agg.Total, sum, "Provider sum should equal Total")
}

// Test error types.
func TestErrorTypes(t *testing.T) {
	errTests := []struct {
		name string
		err  error
	}{
		{"ErrNoCostData", ErrNoCostData},
		{"ErrMixedCurrencies", ErrMixedCurrencies},
		{"ErrInvalidGroupBy", ErrInvalidGroupBy},
		{"ErrEmptyResults", ErrEmptyResults},
		{"ErrInvalidDateRange", ErrInvalidDateRange},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.err, "Error should not be nil")
			assert.NotEmpty(t, tt.err.Error(), "Error message should not be empty")
		})
	}
}

// Test CostResultWithErrors edge cases.
func TestCostResultWithErrors_EdgeCases(t *testing.T) {
	t.Run("nil errors slice", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []CostResult{},
			Errors:  nil,
		}

		assert.False(t, result.HasErrors(), "HasErrors() should return false for nil errors")
		assert.Empty(t, result.ErrorSummary(), "ErrorSummary() should return empty string for nil errors")
	})

	t.Run("exactly 5 errors shows all", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []CostResult{},
			Errors:  make([]ErrorDetail, 5),
		}
		for i := 0; i < 5; i++ {
			result.Errors[i] = ErrorDetail{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-" + string(rune('0'+i)),
				Error:        ErrNoCostData,
				Timestamp:    time.Now(),
			}
		}

		summary := result.ErrorSummary()
		assert.NotEmpty(t, summary, "ErrorSummary should not be empty for 5 errors")
		// Should not contain "and X more" since exactly at limit
		assert.LessOrEqual(t, len(summary), 500, "ErrorSummary should not be excessively long for 5 errors")
	})

	t.Run("nil results slice", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: nil,
			Errors:  []ErrorDetail{},
		}

		assert.False(t, result.HasErrors(), "HasErrors() should return false for empty errors with nil results")
	})

	t.Run("error with empty resource type", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []CostResult{},
			Errors: []ErrorDetail{
				{
					ResourceType: "",
					ResourceID:   "",
					Error:        ErrNoCostData,
					Timestamp:    time.Now(),
				},
			},
		}

		assert.True(t, result.HasErrors(), "HasErrors() should return true")
		assert.NotEmpty(t, result.ErrorSummary(), "ErrorSummary should handle empty resource type")
	})
}

// Test ErrorDetail creation and fields.
func TestErrorDetail_Fields(t *testing.T) {
	timestamp := time.Now()
	detail := ErrorDetail{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-1234567890abcdef0",
		PluginName:   "test-plugin",
		Error:        ErrNoCostData,
		Timestamp:    timestamp,
	}

	assert.Equal(t, "aws:ec2:Instance", detail.ResourceType)
	assert.Equal(t, "i-1234567890abcdef0", detail.ResourceID)
	assert.Equal(t, "test-plugin", detail.PluginName)
	assert.True(t, errors.Is(detail.Error, ErrNoCostData), "Error should be ErrNoCostData")
	assert.True(t, detail.Timestamp.Equal(timestamp), "Timestamp mismatch")
}

// Test EstimateResult creation and fields.
func TestEstimateResult(t *testing.T) {
	t.Run("positive cost change", func(t *testing.T) {
		resource := &ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "i-123",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		result := EstimateResult{
			Resource: resource,
			Baseline: &CostResult{
				Monthly:  8.32,
				Hourly:   0.0114,
				Currency: "USD",
			},
			Modified: &CostResult{
				Monthly:  83.22,
				Hourly:   0.114,
				Currency: "USD",
			},
			TotalChange: 74.90,
			Deltas: []CostDelta{
				{
					Property:      "instanceType",
					OriginalValue: "t3.micro",
					NewValue:      "m5.large",
					CostChange:    74.90,
				},
			},
			UsedFallback: false,
		}

		assert.Equal(t, "ec2:Instance", result.Resource.Type)
		assert.Equal(t, 8.32, result.Baseline.Monthly)
		assert.Equal(t, 83.22, result.Modified.Monthly)
		assert.Equal(t, 74.90, result.TotalChange)
		assert.Len(t, result.Deltas, 1)
		assert.False(t, result.UsedFallback)
	})

	t.Run("negative cost change (savings)", func(t *testing.T) {
		result := EstimateResult{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			Baseline: &CostResult{
				Monthly:  83.22,
				Currency: "USD",
			},
			Modified: &CostResult{
				Monthly:  8.32,
				Currency: "USD",
			},
			TotalChange: -74.90,
			Deltas: []CostDelta{
				{
					Property:      "instanceType",
					OriginalValue: "m5.large",
					NewValue:      "t3.micro",
					CostChange:    -74.90,
				},
			},
			UsedFallback: true,
		}

		assert.Less(t, result.TotalChange, 0.0, "TotalChange should be negative")
		assert.Less(t, result.Deltas[0].CostChange, 0.0, "CostChange should be negative")
		assert.True(t, result.UsedFallback)
	})

	t.Run("nil baseline and modified", func(t *testing.T) {
		result := EstimateResult{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			Baseline:    nil,
			Modified:    nil,
			TotalChange: 0,
			Deltas:      nil,
		}

		assert.Nil(t, result.Baseline)
		assert.Nil(t, result.Modified)
		assert.Equal(t, 0.0, result.TotalChange)
	})

	t.Run("multiple deltas", func(t *testing.T) {
		result := EstimateResult{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			Baseline: &CostResult{
				Monthly:  8.32,
				Currency: "USD",
			},
			Modified: &CostResult{
				Monthly:  92.42,
				Currency: "USD",
			},
			TotalChange: 84.10,
			Deltas: []CostDelta{
				{
					Property:      "instanceType",
					OriginalValue: "t3.micro",
					NewValue:      "m5.large",
					CostChange:    74.90,
				},
				{
					Property:      "volumeSize",
					OriginalValue: "8",
					NewValue:      "100",
					CostChange:    9.20,
				},
			},
		}

		assert.Len(t, result.Deltas, 2)

		// Sum of deltas should approximately equal total change
		var sumDeltas float64
		for _, delta := range result.Deltas {
			sumDeltas += delta.CostChange
		}
		assert.InDelta(t, 84.10, sumDeltas, 0.001, "Sum of deltas should approximately equal total change")
	})
}

// Test CostDelta creation and fields.
func TestCostDelta(t *testing.T) {
	t.Run("cost increase", func(t *testing.T) {
		delta := CostDelta{
			Property:      "instanceType",
			OriginalValue: "t3.micro",
			NewValue:      "m5.large",
			CostChange:    65.70,
		}

		assert.Equal(t, "instanceType", delta.Property)
		assert.Equal(t, "t3.micro", delta.OriginalValue)
		assert.Equal(t, "m5.large", delta.NewValue)
		assert.Equal(t, 65.70, delta.CostChange)
	})

	t.Run("cost decrease (savings)", func(t *testing.T) {
		delta := CostDelta{
			Property:      "instanceType",
			OriginalValue: "m5.large",
			NewValue:      "t3.micro",
			CostChange:    -65.70,
		}

		assert.Less(t, delta.CostChange, 0.0, "CostChange should be negative")
	})

	t.Run("zero cost change", func(t *testing.T) {
		delta := CostDelta{
			Property:      "tags",
			OriginalValue: "old-tag",
			NewValue:      "new-tag",
			CostChange:    0.0,
		}

		assert.Equal(t, 0.0, delta.CostChange)
	})

	t.Run("combined delta", func(t *testing.T) {
		// When multiple properties change and per-property attribution is not possible
		delta := CostDelta{
			Property:      "combined",
			OriginalValue: "",
			NewValue:      "",
			CostChange:    84.10,
		}

		assert.Equal(t, "combined", delta.Property)
	})
}

// Test EstimateRequest creation and fields.
func TestEstimateRequest(t *testing.T) {
	t.Run("with single override", func(t *testing.T) {
		request := EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
				ID:       "i-123",
				Properties: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
			},
			UsageProfile: "production",
		}

		assert.Equal(t, "ec2:Instance", request.Resource.Type)
		assert.Len(t, request.PropertyOverrides, 1)
		assert.Equal(t, "m5.large", request.PropertyOverrides["instanceType"])
		assert.Equal(t, "production", request.UsageProfile)
	})

	t.Run("with multiple overrides", func(t *testing.T) {
		request := EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
				"volumeSize":   "100",
			},
		}

		assert.Len(t, request.PropertyOverrides, 2)
	})

	t.Run("with nil overrides", func(t *testing.T) {
		request := EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			PropertyOverrides: nil,
		}

		assert.Nil(t, request.PropertyOverrides)
	})

	t.Run("with empty usage profile", func(t *testing.T) {
		request := EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
			},
			UsageProfile: "",
		}

		assert.Empty(t, request.UsageProfile)
	})
}

// TestConvertProtoRecommendationReasoning verifies that convertProtoRecommendation
// copies the Reasoning field from proto.Recommendation to engine.Recommendation.
func TestConvertProtoRecommendationReasoning(t *testing.T) {
	t.Run("multi-entry reasoning preserved in order", func(t *testing.T) {
		protoRec := &proto.Recommendation{
			ResourceID:  "my-instance",
			ActionType:  "MIGRATE",
			Description: "Switch to Graviton",
			Impact: &proto.RecommendationImpact{
				EstimatedSavings: 8.00,
				Currency:         "USD",
			},
			Reasoning: []string{
				"Ensure application compatibility with ARM64 architecture",
				"Test workloads before full migration",
			},
		}

		engineRec := convertProtoRecommendation(protoRec)

		require.Len(t, engineRec.Reasoning, 2)
		assert.Equal(t, "Ensure application compatibility with ARM64 architecture", engineRec.Reasoning[0])
		assert.Equal(t, "Test workloads before full migration", engineRec.Reasoning[1])
		assert.Equal(t, "my-instance", engineRec.ResourceID)
		assert.Equal(t, "MIGRATE", engineRec.Type)
		assert.Equal(t, "Switch to Graviton", engineRec.Description)
		assert.Equal(t, 8.00, engineRec.EstimatedSavings)
		assert.Equal(t, "USD", engineRec.Currency)
	})

	t.Run("empty reasoning produces nil", func(t *testing.T) {
		protoRec := &proto.Recommendation{
			ResourceID:  "my-instance",
			ActionType:  "RIGHTSIZE",
			Description: "Switch to t3.small",
			Reasoning:   nil,
		}

		engineRec := convertProtoRecommendation(protoRec)

		assert.Nil(t, engineRec.Reasoning)
	})

	t.Run("empty slice reasoning produces empty slice", func(t *testing.T) {
		protoRec := &proto.Recommendation{
			ResourceID:  "my-instance",
			ActionType:  "TERMINATE",
			Description: "Resource is idle",
			Reasoning:   []string{},
		}

		engineRec := convertProtoRecommendation(protoRec)

		assert.Empty(t, engineRec.Reasoning)
	})

	t.Run("single reasoning entry", func(t *testing.T) {
		protoRec := &proto.Recommendation{
			ResourceID:  "db-instance",
			ActionType:  "RIGHTSIZE",
			Description: "Reduce instance size",
			Impact: &proto.RecommendationImpact{
				EstimatedSavings: 15.50,
				Currency:         "EUR",
			},
			Reasoning: []string{"Check connection pool limits before resizing"},
		}

		engineRec := convertProtoRecommendation(protoRec)

		require.Len(t, engineRec.Reasoning, 1)
		assert.Equal(t, "Check connection pool limits before resizing", engineRec.Reasoning[0])
		assert.Equal(t, 15.50, engineRec.EstimatedSavings)
		assert.Equal(t, "EUR", engineRec.Currency)
	})
}

// TestCostResultJSONRecommendations verifies JSON serialization of CostResult with recommendations (US4).
func TestCostResultJSONRecommendations(t *testing.T) {
	t.Run("populated Recommendations includes recommendations array in JSON", func(t *testing.T) {
		costResult := CostResult{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Monthly:      50.0,
			Currency:     "USD",
			Recommendations: []Recommendation{
				{
					Type:             "RIGHTSIZE",
					Description:      "Switch to t3.small",
					EstimatedSavings: 5.0,
					Currency:         "USD",
					Reasoning:        []string{"Ensure app supports smaller instances"},
				},
				{
					Type:        "TERMINATE",
					Description: "Resource is idle",
				},
			},
		}

		jsonBytes, err := json.Marshal(costResult)
		require.NoError(t, err)
		jsonStr := string(jsonBytes)

		// Verify recommendations array is present
		assert.Contains(t, jsonStr, "\"recommendations\"")
		// Verify individual fields are present
		assert.Contains(t, jsonStr, "\"type\":\"RIGHTSIZE\"")
		assert.Contains(t, jsonStr, "\"type\":\"TERMINATE\"")
		assert.Contains(t, jsonStr, "\"description\":\"Switch to t3.small\"")
		assert.Contains(t, jsonStr, "\"description\":\"Resource is idle\"")
		assert.Contains(t, jsonStr, "\"estimatedSavings\":5")
		assert.Contains(t, jsonStr, "\"currency\":\"USD\"")
		assert.Contains(t, jsonStr, "\"reasoning\"")
		assert.Contains(t, jsonStr, "\"Ensure app supports smaller instances\"")
	})

	t.Run("nil Recommendations omits recommendations key entirely", func(t *testing.T) {
		costResult := CostResult{
			ResourceType:    "aws:ec2:Instance",
			ResourceID:      "i-456",
			Monthly:         75.0,
			Currency:        "USD",
			Recommendations: nil,
		}

		jsonBytes, err := json.Marshal(costResult)
		require.NoError(t, err)
		jsonStr := string(jsonBytes)

		// Verify recommendations key is NOT present
		assert.NotContains(t, jsonStr, "\"recommendations\"")
		// Verify other fields are present
		assert.Contains(t, jsonStr, "\"resourceType\":\"aws:ec2:Instance\"")
		assert.Contains(t, jsonStr, "\"monthly\":75")
	})

	t.Run("empty Recommendations slice omits recommendations key", func(t *testing.T) {
		costResult := CostResult{
			ResourceType:    "aws:s3:Bucket",
			ResourceID:      "my-bucket",
			Monthly:         25.0,
			Currency:        "USD",
			Recommendations: []Recommendation{},
		}

		jsonBytes, err := json.Marshal(costResult)
		require.NoError(t, err)
		jsonStr := string(jsonBytes)

		// Verify recommendations key is NOT present due to omitempty
		assert.NotContains(t, jsonStr, "\"recommendations\"")
	})

	t.Run("recommendation with empty Reasoning omits reasoning key", func(t *testing.T) {
		costResult := CostResult{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-789",
			Recommendations: []Recommendation{
				{
					Type:             "MIGRATE",
					Description:      "Switch to Graviton",
					EstimatedSavings: 8.0,
					Currency:         "USD",
					Reasoning:        nil,
				},
			},
		}

		jsonBytes, err := json.Marshal(costResult)
		require.NoError(t, err)
		jsonStr := string(jsonBytes)

		// Verify recommendations is present but reasoning is omitted for empty entries
		assert.Contains(t, jsonStr, "\"recommendations\"")
		assert.Contains(t, jsonStr, "\"type\":\"MIGRATE\"")
		// The reasoning field should be omitted when empty
		assert.NotContains(t, jsonStr, "\"reasoning\":[]")
	})
}
