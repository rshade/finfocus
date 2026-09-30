package engine_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestCalculateStateCost(t *testing.T) {
	t.Parallel()

	// Fix a reference time for deterministic tests
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		input         engine.StateCostInput
		expectedCost  float64
		expectedHours float64
		expectedNotes string
		expectWarning bool
	}{
		{
			name: "standard resource running for 24 hours",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-1234567890",
					Provider: "aws",
				},
				HourlyRate: 0.10,
				CreatedAt:  now.Add(-24 * time.Hour),
				IsExternal: false,
			},
			expectedCost:  2.40, // 0.10 * 24 hours
			expectedHours: 24.0,
			expectedNotes: "",
			expectWarning: false,
		},
		{
			name: "resource running for 7 days",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:rds/instance:Instance",
					ID:       "db-production",
					Provider: "aws",
				},
				HourlyRate: 0.50,
				CreatedAt:  now.Add(-7 * 24 * time.Hour),
				IsExternal: false,
			},
			expectedCost:  84.0, // 0.50 * 168 hours
			expectedHours: 168.0,
			expectedNotes: "",
			expectWarning: false,
		},
		{
			name: "imported (external) resource",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-imported-123",
					Provider: "aws",
				},
				HourlyRate: 0.10,
				CreatedAt:  now.Add(-48 * time.Hour),
				IsExternal: true,
			},
			expectedCost:  4.80, // 0.10 * 48 hours
			expectedHours: 48.0,
			expectedNotes: "Note: Imported resource - timestamp reflects import time, not actual creation",
			expectWarning: true,
		},
		{
			name: "zero hourly rate",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:s3/bucket:Bucket",
					ID:       "my-bucket",
					Provider: "aws",
				},
				HourlyRate: 0.0,
				CreatedAt:  now.Add(-24 * time.Hour),
				IsExternal: false,
			},
			expectedCost:  0.0,
			expectedHours: 24.0,
			expectedNotes: "",
			expectWarning: false,
		},
		{
			name: "resource created just now (< 1 hour)",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-new",
					Provider: "aws",
				},
				HourlyRate: 0.10,
				CreatedAt:  now.Add(-30 * time.Minute),
				IsExternal: false,
			},
			expectedCost:  0.05, // 0.10 * 0.5 hours
			expectedHours: 0.5,
			expectedNotes: "",
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := engine.CalculateStateCost(tt.input, now)

			assert.InDelta(t, tt.expectedCost, result.TotalCost, 0.01, "TotalCost mismatch")
			assert.InDelta(t, tt.expectedHours, result.RuntimeHours, 0.01, "RuntimeHours mismatch")

			if tt.expectWarning {
				assert.Contains(t, result.Notes, "Imported resource")
			} else if tt.expectedNotes != "" {
				assert.Equal(t, tt.expectedNotes, result.Notes)
			}
		})
	}
}

func TestCalculateStateCost_UptimeAssumption(t *testing.T) {
	t.Parallel()

	// Per spec T023a: All estimates should document 100% uptime assumption
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	input := engine.StateCostInput{
		Resource: engine.ResourceDescriptor{
			Type:     "aws:ec2/instance:Instance",
			ID:       "i-test",
			Provider: "aws",
		},
		HourlyRate: 0.10,
		CreatedAt:  now.Add(-24 * time.Hour),
		IsExternal: false,
	}

	result := engine.CalculateStateCost(input, now)

	// The uptime assumption note is added by the higher-level function
	// Here we just verify the calculation is correct
	assert.InDelta(t, 2.40, result.TotalCost, 0.01)
}

func TestStateCostInput_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     engine.StateCostInput
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid input",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-123",
					Provider: "aws",
				},
				HourlyRate: 0.10,
				CreatedAt:  time.Now().Add(-24 * time.Hour),
			},
			expectErr: false,
		},
		{
			name: "missing created timestamp",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-123",
					Provider: "aws",
				},
				HourlyRate: 0.10,
				// CreatedAt is zero value
			},
			expectErr: true,
			errMsg:    "created timestamp is required",
		},
		{
			name: "negative hourly rate",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-123",
					Provider: "aws",
				},
				HourlyRate: -0.10,
				CreatedAt:  time.Now().Add(-24 * time.Hour),
			},
			expectErr: true,
			errMsg:    "hourly rate cannot be negative",
		},
		{
			name: "future created timestamp",
			input: engine.StateCostInput{
				Resource: engine.ResourceDescriptor{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-123",
					Provider: "aws",
				},
				HourlyRate: 0.10,
				CreatedAt:  time.Now().Add(24 * time.Hour), // Future
			},
			expectErr: true,
			errMsg:    "created timestamp cannot be in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.input.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractCreatedTimestamp(t *testing.T) {
	t.Parallel()

	// Test extracting created timestamp from resource properties
	tests := []struct {
		name       string
		properties map[string]interface{}
		expectTime bool
		expectErr  bool
	}{
		{
			name: "valid RFC3339 timestamp",
			properties: map[string]interface{}{
				"pulumi:created": "2025-01-10T10:30:00Z",
			},
			expectTime: true,
			expectErr:  false,
		},
		{
			name: "missing timestamp",
			properties: map[string]interface{}{
				"someOtherProperty": "value",
			},
			expectTime: false,
			expectErr:  false, // Returns zero time, not an error
		},
		{
			name: "invalid timestamp format",
			properties: map[string]interface{}{
				"pulumi:created": "not-a-timestamp",
			},
			expectTime: false,
			expectErr:  true,
		},
		{
			name:       "nil properties",
			properties: nil,
			expectTime: false,
			expectErr:  false,
		},
		{
			name: "timestamp as time.Time",
			properties: map[string]interface{}{
				"pulumi:created": time.Date(2025, 1, 10, 10, 30, 0, 0, time.UTC),
			},
			expectTime: true,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := engine.ResourceDescriptor{
				Type:       "aws:ec2/instance:Instance",
				ID:         "i-test",
				Provider:   "aws",
				Properties: tt.properties,
			}

			ts, err := engine.ExtractCreatedTimestamp(resource)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expectTime {
					assert.False(t, ts.IsZero(), "expected non-zero timestamp")
				} else {
					assert.True(t, ts.IsZero(), "expected zero timestamp")
				}
			}
		})
	}
}

func TestFindEarliestCreatedTimestamp(t *testing.T) {
	t.Parallel()

	// Test finding the earliest Created timestamp from a set of resources
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		resources    []engine.ResourceDescriptor
		expectedTime time.Time
		expectErr    bool
	}{
		{
			name: "multiple resources with timestamps",
			resources: []engine.ResourceDescriptor{
				{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-1",
					Provider: "aws",
					Properties: map[string]interface{}{
						"pulumi:created": t2.Format(time.RFC3339),
					},
				},
				{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-2",
					Provider: "aws",
					Properties: map[string]interface{}{
						"pulumi:created": t1.Format(time.RFC3339), // Earliest
					},
				},
				{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-3",
					Provider: "aws",
					Properties: map[string]interface{}{
						"pulumi:created": t3.Format(time.RFC3339),
					},
				},
			},
			expectedTime: t1,
			expectErr:    false,
		},
		{
			name: "some resources without timestamps",
			resources: []engine.ResourceDescriptor{
				{
					Type:       "aws:ec2/instance:Instance",
					ID:         "i-1",
					Provider:   "aws",
					Properties: map[string]interface{}{}, // No timestamp
				},
				{
					Type:     "aws:ec2/instance:Instance",
					ID:       "i-2",
					Provider: "aws",
					Properties: map[string]interface{}{
						"pulumi:created": t2.Format(time.RFC3339),
					},
				},
			},
			expectedTime: t2,
			expectErr:    false,
		},
		{
			name: "no resources with timestamps",
			resources: []engine.ResourceDescriptor{
				{
					Type:       "aws:ec2/instance:Instance",
					ID:         "i-1",
					Provider:   "aws",
					Properties: map[string]interface{}{},
				},
			},
			expectedTime: time.Time{},
			expectErr:    true,
		},
		{
			name:         "empty resources",
			resources:    []engine.ResourceDescriptor{},
			expectedTime: time.Time{},
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			earliest, err := engine.FindEarliestCreatedTimestamp(tt.resources)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedTime.UTC(), earliest.UTC())
			}
		})
	}
}

// BenchmarkCalculateStateCost benchmarks state-based cost calculation for 100 resources.
// Per SC-004: State-based cost calculation should complete in <100ms for 100 resources.
func BenchmarkCalculateStateCost(b *testing.B) {
	now := time.Now()
	input := engine.StateCostInput{
		Resource: engine.ResourceDescriptor{
			Type:     "aws:ec2/instance:Instance",
			ID:       "i-benchmark",
			Provider: "aws",
		},
		HourlyRate: 0.10,
		CreatedAt:  now.Add(-24 * time.Hour),
		IsExternal: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.CalculateStateCost(input, now)
	}
}

// BenchmarkCalculateStateCost100Resources tests SC-004 requirement.
// Verifies that calculating costs for 100 resources completes in <100ms.
func BenchmarkCalculateStateCost100Resources(b *testing.B) {
	now := time.Now()

	// Create 100 resources with varying creation times
	inputs := make([]engine.StateCostInput, 100)
	for i := 0; i < 100; i++ {
		inputs[i] = engine.StateCostInput{
			Resource: engine.ResourceDescriptor{
				Type:     "aws:ec2/instance:Instance",
				ID:       fmt.Sprintf("i-%03d", i),
				Provider: "aws",
			},
			HourlyRate: 0.10 * float64(i%10+1),
			CreatedAt:  now.Add(-time.Duration(i*24) * time.Hour),
			IsExternal: i%10 == 0, // Every 10th resource is external
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			engine.CalculateStateCost(inputs[j], now)
		}
	}
}

// TestCalculateStateCost100Resources_Performance validates SC-004 requirement.
// Ensures 100 resource calculations complete in <100ms.
func TestCalculateStateCost100Resources_Performance(t *testing.T) {
	t.Parallel()

	now := time.Now()

	// Create 100 resources
	inputs := make([]engine.StateCostInput, 100)
	for i := 0; i < 100; i++ {
		inputs[i] = engine.StateCostInput{
			Resource: engine.ResourceDescriptor{
				Type:     "aws:ec2/instance:Instance",
				ID:       "i-perf-test",
				Provider: "aws",
			},
			HourlyRate: 0.10,
			CreatedAt:  now.Add(-time.Duration(i) * time.Hour),
			IsExternal: false,
		}
	}

	start := time.Now()
	for _, input := range inputs {
		engine.CalculateStateCost(input, now)
	}
	elapsed := time.Since(start)

	// SC-004: Must complete in <100ms for 100 resources
	assert.Less(t, elapsed, 100*time.Millisecond,
		"state-based cost calculation for 100 resources should complete in <100ms, got %v", elapsed)
}

func TestIsExternalResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		properties map[string]interface{}
		expected   bool
	}{
		{
			name: "external true string",
			properties: map[string]interface{}{
				"pulumi:external": "true",
			},
			expected: true,
		},
		{
			name: "external false string",
			properties: map[string]interface{}{
				"pulumi:external": "false",
			},
			expected: false,
		},
		{
			name: "external true bool",
			properties: map[string]interface{}{
				"pulumi:external": true,
			},
			expected: true,
		},
		{
			name: "external false bool",
			properties: map[string]interface{}{
				"pulumi:external": false,
			},
			expected: false,
		},
		{
			name:       "missing external property",
			properties: map[string]interface{}{},
			expected:   false,
		},
		{
			name:       "nil properties",
			properties: nil,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := engine.ResourceDescriptor{
				Type:       "aws:ec2/instance:Instance",
				ID:         "i-test",
				Provider:   "aws",
				Properties: tt.properties,
			}

			result := engine.IsExternalResource(resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}
