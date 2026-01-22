package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestGetDefaultMonthlyByType(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		expected     float64
	}{
		// Database types
		{"database_rds", "aws:rds/instance:Instance", defaultDatabaseMonthlyCost},
		{"database_rds_legacy", "aws:rds:dbInstance", defaultDatabaseMonthlyCost},
		{"database_generic", "azure:database:Sql", defaultDatabaseMonthlyCost},
		{"database_case_insensitive", "aws:RDS:dbInstance", defaultDatabaseMonthlyCost},
		{"case_insensitive_upper", "AWS:RDS:INSTANCE", defaultDatabaseMonthlyCost},

		// Storage types
		{"storage_s3", "aws:s3/bucket:Bucket", defaultStorageMonthlyCost},
		{"storage_s3_legacy", "aws:s3:Bucket", defaultStorageMonthlyCost},
		{"storage_generic", "azure:storage:account", defaultStorageMonthlyCost},
		{"storage_gcp", "gcp:storage:Bucket", defaultStorageMonthlyCost},

		// Compute types (default)
		{"compute_ec2", "aws:ec2/instance:Instance", defaultComputeMonthlyCost},
		{"compute_ec2_legacy", "aws:ec2:Instance", defaultComputeMonthlyCost},
		{"compute_generic", "unknown:resource:Type", defaultComputeMonthlyCost},
		{"unknown", "gcp:unknown:resource", defaultComputeMonthlyCost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDefaultMonthlyByType(tt.resourceType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestParseFloatValue tests type conversion for various input types.
// This covers the parseFloatValue function which accepts float64, int, or numeric strings.
func TestParseFloatValue(t *testing.T) {
	tests := []struct {
		name       string
		input      interface{}
		expectedOk bool
		expected   float64
	}{
		// Direct float64 passthrough
		{"float64_positive", 42.5, true, 42.5},
		{"float64_zero", 0.0, true, 0.0},
		{"float64_negative", -123.45, true, -123.45},

		// Integer conversion
		{"int_positive", 100, true, 100.0},
		{"int_zero", 0, true, 0.0},
		{"int_negative", -50, true, -50.0},

		// String parsing
		{"string_integer", "42", true, 42.0},
		{"string_float", "3.14159", true, 3.14159},
		{"string_negative", "-99.9", true, -99.9},
		{"string_scientific", "1.5e10", true, 1.5e10},

		// Invalid inputs return false
		{"string_non_numeric", "not-a-number", false, 0},
		{"string_empty", "", false, 0},
		{"nil_value", nil, false, 0},
		{"bool_true", true, false, 0},
		{"bool_false", false, false, 0},
		{"struct_type", struct{}{}, false, 0},
		{"slice_type", []int{1, 2, 3}, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parseFloatValue(tt.input)
			assert.Equal(t, tt.expectedOk, ok, "unexpected ok value")
			if tt.expectedOk {
				assert.InDelta(t, tt.expected, result, 0.0001, "unexpected result value")
			}
		})
	}
}

func TestGetStorageSize(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]interface{}
		wantSize   float64
		wantFound  bool
	}{
		{"size_prop", map[string]interface{}{"size": 100}, 100.0, true},
		{"sizeGb_prop", map[string]interface{}{"sizeGb": "200"}, 200.0, true},
		{"volumeSize_prop", map[string]interface{}{"volumeSize": 50.5}, 50.5, true},
		{"allocatedStorage_prop", map[string]interface{}{"allocatedStorage": 500}, 500.0, true},
		{"no_size_prop", map[string]interface{}{"name": "test"}, 0, false},
		{"nil_props", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ResourceDescriptor{Properties: tt.properties}
			size, found := getStorageSize(res)
			assert.Equal(t, tt.wantFound, found)
			if found {
				assert.Equal(t, tt.wantSize, size)
			}
		})
	}
}

func TestTryStoragePricing(t *testing.T) {
	tests := []struct {
		name            string
		pricing         map[string]interface{}
		resource        ResourceDescriptor
		expectedMonthly float64
		expectedHourly  float64
		expectedOk      bool
	}{
		{
			name:    "valid_pricing_with_size",
			pricing: map[string]interface{}{"pricePerGBMonth": 0.1},
			resource: ResourceDescriptor{
				Properties: map[string]interface{}{"size": 100.0},
			},
			expectedMonthly: 10.0,         // 100 * 0.1
			expectedHourly:  10.0 / 730.0, // 10 / 730
			expectedOk:      true,
		},
		{
			name:    "valid_storage_pricing_int_size",
			pricing: map[string]interface{}{"pricePerGBMonth": 0.10},
			resource: ResourceDescriptor{
				Properties: map[string]interface{}{"size": 100},
			},
			expectedMonthly: 10.0, // 100 * 0.10
			expectedHourly:  10.0 / hoursPerMonth,
			expectedOk:      true,
		},
		{
			name:    "missing_size",
			pricing: map[string]interface{}{"pricePerGBMonth": 0.1},
			resource: ResourceDescriptor{
				Properties: map[string]interface{}{},
			},
			expectedMonthly: 0,
			expectedHourly:  0,
			expectedOk:      false,
		},
		{
			name:    "missing_pricing_key",
			pricing: map[string]interface{}{"otherKey": 0.1},
			resource: ResourceDescriptor{
				Properties: map[string]interface{}{"size": 100.0},
			},
			expectedMonthly: 0,
			expectedHourly:  0,
			expectedOk:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monthly, hourly, ok := tryStoragePricing(tt.pricing, tt.resource)
			assert.Equal(t, tt.expectedOk, ok)
			if tt.expectedOk {
				assert.InDelta(t, tt.expectedMonthly, monthly, 0.0001)
				assert.InDelta(t, tt.expectedHourly, hourly, 0.0001)
			}
		})
	}
}

func TestTryFallbackNumericValue(t *testing.T) {
	tests := []struct {
		name        string
		pricing     map[string]interface{}
		wantMonthly float64
		wantHourly  float64
		wantFound   bool
	}{
		{
			name:        "single_numeric_value",
			pricing:     map[string]interface{}{"flatRate": 0.5},
			wantMonthly: 0.5 * hoursPerMonth,
			wantHourly:  0.5,
			wantFound:   true,
		},
		{
			name:      "no_numeric_values",
			pricing:   map[string]interface{}{"desc": "text only"},
			wantFound: false,
		},
		{
			name:      "zero_value_ignored",
			pricing:   map[string]interface{}{"free": 0.0},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, h, found := tryFallbackNumericValue(tt.pricing)
			assert.Equal(t, tt.wantFound, found)
			if found {
				assert.InDelta(t, tt.wantMonthly, m, 0.0001)
				assert.InDelta(t, tt.wantHourly, h, 0.0001)
			}
		})
	}
}

func TestGetActualCost_Wrapper(t *testing.T) {
	// Test the simple wrapper function GetActualCost
	// We can't easily mock the full engine here without setting up plugins,
	// but we can test that it calls GetActualCostWithOptions correctly.
	// Since we are in the engine package, we can just create an engine with no clients/loader
	// and expect it to return results (even if empty or placeholders).

	e := New([]*pluginhost.Client{}, nil)

	resources := []ResourceDescriptor{
		{Type: "aws:s3:Bucket", ID: "bucket-1", Provider: "aws"},
	}
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	results, err := e.GetActualCost(context.Background(), resources, from, to)

	// Should return a result (fallback placeholder) and no error
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "none", results[0].Adapter)
	assert.Equal(t, 0.0, results[0].TotalCost)
}
