package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errNoSpec is a sentinel error for missing specs in tests.
var errNoSpec = errors.New("no spec available")

// mockSpecLoader is a minimal spec loader for testing.
type mockSpecLoader struct{}

func (m *mockSpecLoader) LoadSpec(_, _, _ string) (interface{}, error) {
	return nil, errNoSpec
}

// TestEstimateCost_Fallback tests the fallback behavior when EstimateCost RPC is not implemented.
func TestEstimateCost_Fallback(t *testing.T) {
	t.Run("single property override with fallback", func(t *testing.T) {
		// Create engine with no plugins (forces fallback to spec)
		engine := New(nil, &mockSpecLoader{})

		request := &EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "aws:ec2:Instance",
				ID:       "i-123",
				Properties: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
			},
		}

		result, err := engine.EstimateCost(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.UsedFallback, "should use fallback when no plugins available")
		assert.NotNil(t, result.Resource)
		assert.NotNil(t, result.Baseline)
		assert.NotNil(t, result.Modified)
		assert.Len(t, result.Deltas, 1)
		assert.Equal(t, "instanceType", result.Deltas[0].Property)
		assert.Equal(t, "t3.micro", result.Deltas[0].OriginalValue)
		assert.Equal(t, "m5.large", result.Deltas[0].NewValue)
	})

	t.Run("multiple property overrides with combined delta", func(t *testing.T) {
		engine := New(nil, &mockSpecLoader{})

		request := &EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "aws:ec2:Instance",
				ID:       "i-123",
				Properties: map[string]interface{}{
					"instanceType": "t3.micro",
					"volumeSize":   8,
				},
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
				"volumeSize":   "100",
			},
		}

		result, err := engine.EstimateCost(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.UsedFallback)
		// Multiple overrides should result in a "combined" delta
		assert.Len(t, result.Deltas, 1)
		assert.Equal(t, "combined", result.Deltas[0].Property)
	})

	t.Run("no property overrides", func(t *testing.T) {
		engine := New(nil, &mockSpecLoader{})

		request := &EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "aws:ec2:Instance",
				ID:       "i-123",
				Properties: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			PropertyOverrides: map[string]string{},
		}

		result, err := engine.EstimateCost(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)

		// With no overrides, baseline and modified should be the same
		assert.Equal(t, result.Baseline.Monthly, result.Modified.Monthly)
		assert.Equal(t, 0.0, result.TotalChange)
		assert.Empty(t, result.Deltas)
	})
}

// TestEstimateCost_ResourceValidation tests that invalid resources are rejected.
func TestEstimateCost_ResourceValidation(t *testing.T) {
	t.Run("empty resource type", func(t *testing.T) {
		engine := New(nil, &mockSpecLoader{})

		request := &EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider:   "aws",
				Type:       "", // Empty type should fail validation
				ID:         "i-123",
				Properties: map[string]interface{}{},
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
			},
		}

		result, err := engine.EstimateCost(context.Background(), request)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource type is required")
	})
}

// TestEstimateCost_TotalChange tests that TotalChange is correctly calculated.
func TestEstimateCost_TotalChange(t *testing.T) {
	t.Run("total change equals modified minus baseline", func(t *testing.T) {
		engine := New(nil, &mockSpecLoader{})

		request := &EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "aws:ec2:Instance",
				ID:       "i-123",
				Properties: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
			},
		}

		result, err := engine.EstimateCost(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)

		// TotalChange should be Modified.Monthly - Baseline.Monthly
		expectedChange := result.Modified.Monthly - result.Baseline.Monthly
		assert.Equal(t, expectedChange, result.TotalChange)
	})
}

// TestFormatPropertyValue tests the property value formatting helper.
func TestFormatPropertyValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string value", "t3.micro", "t3.micro"},
		{"int value", 42, "42"},
		{"int64 value", int64(1000), "1000"},
		{"float64 value", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil value", nil, "<nil>"},
		{"slice value", []string{"a", "b"}, "[a b]"},
		{"map value", map[string]string{"key": "val"}, "map[key:val]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPropertyValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEstimateCost_Context tests context cancellation handling.
func TestEstimateCost_Context(t *testing.T) {
	t.Run("cancelled context returns context.Canceled error", func(t *testing.T) {
		eng := New(nil, &mockSpecLoader{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		request := &EstimateRequest{
			Resource: &ResourceDescriptor{
				Provider: "aws",
				Type:     "aws:ec2:Instance",
				ID:       "i-123",
				Properties: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			PropertyOverrides: map[string]string{
				"instanceType": "m5.large",
			},
		}

		_, err := eng.EstimateCost(ctx, request)
		// With cancelled context, should return context.Canceled or wrapped error
		assert.Error(t, err)
		// The error could be context.Canceled itself or wrapped
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
			"expected context cancellation error, got: %v", err)
	})
}

// TestEstimateRequest_Validation tests EstimateRequest field validation.
func TestEstimateRequest_Validation(t *testing.T) {
	t.Run("nil resource returns error", func(t *testing.T) {
		engine := New(nil, &mockSpecLoader{})

		request := &EstimateRequest{
			Resource:          nil,
			PropertyOverrides: map[string]string{},
		}

		// Should return error, not panic
		result, err := engine.EstimateCost(context.Background(), request)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "resource cannot be nil")
	})

	t.Run("nil request returns error", func(t *testing.T) {
		engine := New(nil, &mockSpecLoader{})

		// Should return error, not panic
		result, err := engine.EstimateCost(context.Background(), nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "request cannot be nil")
	})
}

// BenchmarkEstimateCost_SingleResource validates SC-004:
// "90% of cost estimate requests return results within 5 seconds for single-resource estimation"
// This benchmark measures the performance of single-resource estimation with fallback.
func BenchmarkEstimateCost_SingleResource(b *testing.B) {
	engine := New(nil, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider: "aws",
			Type:     "aws:ec2:Instance",
			ID:       "benchmark-instance",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		},
		PropertyOverrides: map[string]string{
			"instanceType": "m5.large",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.EstimateCost(context.Background(), request)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkEstimateCost_MultipleOverrides benchmarks estimation with multiple property overrides.
func BenchmarkEstimateCost_MultipleOverrides(b *testing.B) {
	engine := New(nil, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider: "aws",
			Type:     "aws:ec2:Instance",
			ID:       "benchmark-instance",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"volumeSize":   8,
				"volumeType":   "gp2",
			},
		},
		PropertyOverrides: map[string]string{
			"instanceType": "m5.large",
			"volumeSize":   "100",
			"volumeType":   "gp3",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.EstimateCost(context.Background(), request)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkEstimateCost_NoOverrides benchmarks baseline-only estimation (no property changes).
func BenchmarkEstimateCost_NoOverrides(b *testing.B) {
	engine := New(nil, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider: "aws",
			Type:     "aws:ec2:Instance",
			ID:       "benchmark-instance",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
		PropertyOverrides: map[string]string{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.EstimateCost(context.Background(), request)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// TestEstimateCost_PerformanceWithin5Seconds validates that single-resource estimation
// completes within the 5-second SLA defined in SC-004.
func TestEstimateCost_PerformanceWithin5Seconds(t *testing.T) {
	eng := New(nil, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider: "aws",
			Type:     "aws:ec2:Instance",
			ID:       "performance-test-instance",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		},
		PropertyOverrides: map[string]string{
			"instanceType": "m5.large",
		},
	}

	// Per SC-004, 90% of cost estimate requests should return within 5 seconds
	const iterations = 100
	const maxDuration = 5 * time.Second
	var totalDuration time.Duration
	slowCount := 0

	for i := 0; i < iterations; i++ {
		start := time.Now()
		result, err := eng.EstimateCost(context.Background(), request)
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.NotNil(t, result)

		totalDuration += elapsed
		if elapsed > maxDuration {
			slowCount++
		}
	}

	t.Logf("Completed %d iterations", iterations)
	t.Logf("Total duration: %v, Average: %v", totalDuration, totalDuration/iterations)
	t.Logf("Slow requests (>5s): %d", slowCount)

	slowPercentage := float64(slowCount) / float64(iterations) * 100
	assert.LessOrEqual(t, slowPercentage, 10.0,
		"more than 10%% of requests exceeded 5 second SLA")
}
