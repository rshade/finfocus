package engine

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// estimateMockPlugin implements proto.CostSourceClient with a configurable
// EstimateCost function for testing tryEstimateCostRPC.
type estimateMockPlugin struct {
	estimateCostFunc func(ctx context.Context, in *pbc.EstimateCostRequest, opts ...grpc.CallOption) (*pbc.EstimateCostResponse, error)
}

func (m *estimateMockPlugin) Name(
	_ context.Context, _ *proto.Empty, _ ...grpc.CallOption,
) (*proto.NameResponse, error) {
	return &proto.NameResponse{Name: "estimate-mock"}, nil
}

func (m *estimateMockPlugin) GetProjectedCost(
	_ context.Context, _ *proto.GetProjectedCostRequest, _ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	return &proto.GetProjectedCostResponse{}, nil
}

func (m *estimateMockPlugin) GetActualCost(
	_ context.Context, _ *proto.GetActualCostRequest, _ ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	return &proto.GetActualCostResponse{}, nil
}

func (m *estimateMockPlugin) GetRecommendations(
	_ context.Context, _ *proto.GetRecommendationsRequest, _ ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	return &proto.GetRecommendationsResponse{}, nil
}

func (m *estimateMockPlugin) GetPluginInfo(
	_ context.Context, _ *proto.Empty, _ ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	return &pbc.GetPluginInfoResponse{}, nil
}

func (m *estimateMockPlugin) DryRun(
	_ context.Context, _ *pbc.DryRunRequest, _ ...grpc.CallOption,
) (*pbc.DryRunResponse, error) {
	return &pbc.DryRunResponse{}, nil
}

func (m *estimateMockPlugin) GetBudgets(
	_ context.Context, _ *pbc.GetBudgetsRequest, _ ...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	return &pbc.GetBudgetsResponse{}, nil
}

func (m *estimateMockPlugin) DismissRecommendation(
	_ context.Context, _ *proto.DismissRecommendationRequest, _ ...grpc.CallOption,
) (*proto.DismissRecommendationResponse, error) {
	return &proto.DismissRecommendationResponse{Success: true}, nil
}

func (m *estimateMockPlugin) Supports(
	_ context.Context, _ *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (m *estimateMockPlugin) EstimateCost(
	ctx context.Context, in *pbc.EstimateCostRequest, opts ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
	if m.estimateCostFunc != nil {
		return m.estimateCostFunc(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "EstimateCost not implemented")
}

func (m *estimateMockPlugin) BatchCost(
	_ context.Context, _ *pbc.BatchCostRequest, _ ...grpc.CallOption,
) (*pbc.BatchCostResponse, error) {
	return &pbc.BatchCostResponse{}, nil
}

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
		assert.Len(t, result.Deltas, 1)
		assert.Equal(t, combinedDeltaProperty, result.Deltas[0].Property)
	})

	t.Run("no property overrides returns validation error", func(t *testing.T) {
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
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "property overrides")
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

// BenchmarkEstimateCost_MinimalOverride benchmarks estimation with a single minimal property change.
func BenchmarkEstimateCost_MinimalOverride(b *testing.B) {
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
		PropertyOverrides: map[string]string{
			"instanceType": "t3.small",
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

// TestTryEstimateCostRPC_Success verifies the RPC path returns correct baseline/modified
// costs and TotalChange when the plugin implements EstimateCost.
func TestTryEstimateCostRPC_Success(t *testing.T) {
	callCount := 0
	var firstRequest, secondRequest *pbc.EstimateCostRequest
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, in *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			callCount++
			// First call = baseline (original properties), second = modified
			if callCount == 1 {
				firstRequest = in
				return &pbc.EstimateCostResponse{
					Currency:    "USD",
					CostMonthly: 10.0,
				}, nil
			}
			secondRequest = in
			return &pbc.EstimateCostResponse{
				Currency:    "USD",
				CostMonthly: 25.0,
			}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-123",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.UsedFallback, "should NOT use fallback when RPC succeeds")
	assert.Equal(t, 2, callCount, "should call EstimateCost twice (baseline + modified)")
	require.NotNil(t, result.Baseline)
	require.NotNil(t, result.Modified)
	assert.Equal(t, 10.0, result.Baseline.Monthly)
	assert.Equal(t, 25.0, result.Modified.Monthly)
	assert.InDelta(t, 15.0, result.TotalChange, 0.001)

	// Verify baseline request carries original properties
	require.NotNil(t, firstRequest, "baseline request should have been captured")
	require.NotNil(t, firstRequest.GetAttributes(), "baseline request should have attributes")
	baselineAttrs := firstRequest.GetAttributes().AsMap()
	assert.Equal(t, "t3.micro", baselineAttrs["instanceType"],
		"baseline request should contain original instanceType")

	// Verify modified request carries merged overrides
	require.NotNil(t, secondRequest, "modified request should have been captured")
	require.NotNil(t, secondRequest.GetAttributes(), "modified request should have attributes")
	modifiedAttrs := secondRequest.GetAttributes().AsMap()
	assert.Equal(t, "m5.large", modifiedAttrs["instanceType"],
		"modified request should contain overridden instanceType")
}

// TestTryEstimateCostRPC_SinglePropertyDelta verifies that a single override
// produces one CostDelta entry with the correct property name and cost change.
func TestTryEstimateCostRPC_SinglePropertyDelta(t *testing.T) {
	callCount := 0
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			callCount++
			if callCount == 1 {
				return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 50.0}, nil
			}
			return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 120.0}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-456",
			Properties: map[string]interface{}{"instanceType": "t3.small"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.xlarge"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Deltas, 1)
	assert.Equal(t, "instanceType", result.Deltas[0].Property)
	assert.Equal(t, "t3.small", result.Deltas[0].OriginalValue)
	assert.Equal(t, "m5.xlarge", result.Deltas[0].NewValue)
	assert.InDelta(t, 70.0, result.Deltas[0].CostChange, 0.001)
}

// TestTryEstimateCostRPC_MultiPropertyCombinedDelta verifies that multiple
// overrides produce a single "combined" delta entry.
func TestTryEstimateCostRPC_MultiPropertyCombinedDelta(t *testing.T) {
	callCount := 0
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			callCount++
			if callCount == 1 {
				return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 30.0}, nil
			}
			return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 80.0}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-789",
			Properties: map[string]interface{}{"instanceType": "t3.micro", "volumeSize": 8},
		},
		PropertyOverrides: map[string]string{
			"instanceType": "m5.large",
			"volumeSize":   "100",
		},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Deltas, 1)
	assert.Equal(t, combinedDeltaProperty, result.Deltas[0].Property)
	assert.InDelta(t, 50.0, result.Deltas[0].CostChange, 0.001)
}

// TestTryEstimateCostRPC_NilResponse verifies that a nil response from the
// plugin causes an error (not a panic).
func TestTryEstimateCostRPC_NilResponse(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			// Simulate a plugin returning a nil response (no proto, no error).
			// validateEstimateResponse catches this and returns errNilEstimateResponse.
			var resp *pbc.EstimateCostResponse
			return resp, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-nil",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	// nil response should cause the plugin to be skipped (error logged),
	// falling through to fallback
	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.UsedFallback, "nil RPC response should trigger fallback")
}

// TestTryEstimateCostRPC_NegativeCost verifies that a negative CostMonthly
// from the plugin causes the RPC result to be rejected.
func TestTryEstimateCostRPC_NegativeCost(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return &pbc.EstimateCostResponse{
				Currency:    "USD",
				CostMonthly: -5.0,
			}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-neg",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	// Negative cost should cause the plugin to be skipped, falling to fallback
	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.UsedFallback, "negative cost should trigger fallback")
}

// TestTryEstimateCostRPC_EmptyCurrency verifies that an empty currency
// in the plugin response causes RPC rejection and engine falls back.
func TestTryEstimateCostRPC_EmptyCurrency(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return &pbc.EstimateCostResponse{
				Currency:    "",
				CostMonthly: 10.0,
			}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-empty-cur",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.UsedFallback, "empty currency should cause RPC rejection and fallback")
}

// TestTryEstimateCostRPC_CurrencyPassthrough verifies that a non-USD currency
// from the plugin is preserved without conversion.
func TestTryEstimateCostRPC_CurrencyPassthrough(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return &pbc.EstimateCostResponse{
				Currency:    "EUR",
				CostMonthly: 10.0,
			}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-eur",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.UsedFallback)
	assert.Equal(t, "EUR", result.Baseline.Currency)
	assert.Equal(t, "EUR", result.Modified.Currency)
}

// TestTryEstimateCostRPC_NilExpiresAt verifies that ExpiresAt remains nil
// since the EstimateCostResponse proto lacks an expires_at field.
func TestTryEstimateCostRPC_NilExpiresAt(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return &pbc.EstimateCostResponse{
				Currency:    "USD",
				CostMonthly: 10.0,
			}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-exp",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.UsedFallback)
	assert.Nil(t, result.Baseline.ExpiresAt, "ExpiresAt should be nil: EstimateCostResponse proto lacks expires_at")
	assert.Nil(t, result.Modified.ExpiresAt, "ExpiresAt should be nil: EstimateCostResponse proto lacks expires_at")
}

// TestEstimateCost_FallbackOnUnimplemented verifies that when a plugin returns
// Unimplemented for EstimateCost, the engine falls back to double-GetProjectedCost
// and sets UsedFallback = true.
func TestEstimateCost_FallbackOnUnimplemented(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return nil, status.Error(codes.Unimplemented, "EstimateCost not implemented")
		},
	}

	clients := []*pluginhost.Client{{Name: "unimpl-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-fallback",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.UsedFallback, "should use fallback when plugin returns Unimplemented")
	require.NotNil(t, result.Baseline)
	require.NotNil(t, result.Modified)
	require.Len(t, result.Deltas, 1)
	assert.Equal(t, "instanceType", result.Deltas[0].Property)
}

// TestEstimateCost_MultiPlugin_FirstUnimplemented verifies that when the first
// plugin returns Unimplemented but the second implements the RPC, the engine
// uses the second plugin's response with UsedFallback = false.
func TestEstimateCost_MultiPlugin_FirstUnimplemented(t *testing.T) {
	unimplMock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return nil, status.Error(codes.Unimplemented, "EstimateCost not implemented")
		},
	}

	callCount := 0
	implMock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			callCount++
			if callCount == 1 {
				return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 20.0}, nil
			}
			return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 45.0}, nil
		},
	}

	clients := []*pluginhost.Client{
		{Name: "unimpl-plugin", API: unimplMock},
		{Name: "impl-plugin", API: implMock},
	}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-multi",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.False(t, result.UsedFallback, "should NOT use fallback when second plugin implements RPC")
	assert.Equal(t, 2, callCount, "second plugin should be called twice (baseline + modified)")
	require.NotNil(t, result.Baseline)
	require.NotNil(t, result.Modified)
	assert.Equal(t, 20.0, result.Baseline.Monthly)
	assert.Equal(t, 45.0, result.Modified.Monthly)
	assert.InDelta(t, 25.0, result.TotalChange, 0.001)
}

// TestValidateEstimateResponse_NaN verifies that NaN CostMonthly is rejected.
func TestValidateEstimateResponse_NaN(t *testing.T) {
	resp := &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: math.NaN()}
	err := validateEstimateResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNonFiniteEstimateCost)
}

// TestValidateEstimateResponse_Inf verifies that Inf CostMonthly is rejected.
func TestValidateEstimateResponse_Inf(t *testing.T) {
	resp := &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: math.Inf(1)}
	err := validateEstimateResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNonFiniteEstimateCost)
}

// TestValidateEstimateResponse_NegativeInf verifies that -Inf CostMonthly is rejected.
func TestValidateEstimateResponse_NegativeInf(t *testing.T) {
	resp := &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: math.Inf(-1)}
	err := validateEstimateResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNonFiniteEstimateCost)
}

// TestValidateEstimateResponse_EmptyCurrency verifies that empty currency is rejected.
func TestValidateEstimateResponse_EmptyCurrency(t *testing.T) {
	resp := &pbc.EstimateCostResponse{Currency: "", CostMonthly: 10.0}
	err := validateEstimateResponse(resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, errEmptyEstimateCurrency)
}

// TestValidateEstimateResponse_Valid verifies that a valid response passes.
func TestValidateEstimateResponse_Valid(t *testing.T) {
	resp := &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 10.0}
	err := validateEstimateResponse(resp)
	require.NoError(t, err)
}

// TestTryEstimateCostRPC_NaNFallsBackToFallback verifies that NaN CostMonthly
// from a plugin causes RPC rejection and engine falls back.
func TestTryEstimateCostRPC_NaNFallsBackToFallback(t *testing.T) {
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return &pbc.EstimateCostResponse{
				Currency:    "USD",
				CostMonthly: math.NaN(),
			}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-nan",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.UsedFallback, "NaN response should cause fallback")
}

// TestTryEstimateCostRPC_CurrencyMismatchFallsBack verifies that differing
// currencies between baseline and modified responses cause fallback.
func TestTryEstimateCostRPC_CurrencyMismatchFallsBack(t *testing.T) {
	callCount := 0
	mock := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			callCount++
			if callCount == 1 {
				return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 10.0}, nil
			}
			return &pbc.EstimateCostResponse{Currency: "EUR", CostMonthly: 9.0}, nil
		},
	}

	clients := []*pluginhost.Client{{Name: "test-plugin", API: mock}}
	eng := New(clients, &mockSpecLoader{})

	request := &EstimateRequest{
		Resource: &ResourceDescriptor{
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-mismatch",
			Properties: map[string]interface{}{"instanceType": "t3.micro"},
		},
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	}

	result, err := eng.EstimateCost(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.UsedFallback, "currency mismatch should cause fallback")
}

func TestCoerceOverrideValue(t *testing.T) {
	tests := []struct {
		name     string
		override string
		original any
		expected any
	}{
		{"float64 integer string", "100", float64(8), float64(100)},
		{"float64 decimal string", "3.14", float64(2.0), float64(3.14)},
		{"float64 invalid string", "m5.large", float64(8), "m5.large"},
		{"int coercion", "42", int(10), int(42)},
		{"int invalid string", "abc", int(10), "abc"},
		{"int64 coercion", "999", int64(100), int64(999)},
		{"int64 invalid string", "nope", int64(100), "nope"},
		{"bool true", "true", false, true},
		{"bool false", "false", true, false},
		{"bool numeric 1", "1", false, true},
		{"bool invalid", "maybe", true, "maybe"},
		{"string original", "new", "old", "new"},
		{"nil original", "value", nil, "value"},
		{"map original", "value", map[string]any{"k": "v"}, "value"},
		{"slice original", "value", []any{1, 2}, "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceOverrideValue(tt.override, tt.original)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergePropertiesWithOverrides(t *testing.T) {
	t.Run("override preserves float64 type", func(t *testing.T) {
		properties := map[string]any{"volumeSize": float64(8)}
		overrides := map[string]string{"volumeSize": "100"}

		merged := mergePropertiesWithOverrides(properties, overrides)

		require.Contains(t, merged, "volumeSize")
		assert.IsType(t, float64(0), merged["volumeSize"])
		assert.Equal(t, float64(100), merged["volumeSize"])
	})

	t.Run("override preserves bool type", func(t *testing.T) {
		properties := map[string]any{"enabled": true}
		overrides := map[string]string{"enabled": "false"}

		merged := mergePropertiesWithOverrides(properties, overrides)

		assert.IsType(t, false, merged["enabled"])
		assert.Equal(t, false, merged["enabled"])
	})

	t.Run("override preserves string type", func(t *testing.T) {
		properties := map[string]any{"instanceType": "t3.micro"}
		overrides := map[string]string{"instanceType": "m5.large"}

		merged := mergePropertiesWithOverrides(properties, overrides)

		assert.Equal(t, "m5.large", merged["instanceType"])
	})

	t.Run("new key stays string", func(t *testing.T) {
		properties := map[string]any{"existing": float64(1)}
		overrides := map[string]string{"newKey": "value"}

		merged := mergePropertiesWithOverrides(properties, overrides)

		assert.Equal(t, "value", merged["newKey"])
		assert.Equal(t, float64(1), merged["existing"])
	})

	t.Run("unparseable override stays string", func(t *testing.T) {
		properties := map[string]any{"count": float64(5)}
		overrides := map[string]string{"count": "many"}

		merged := mergePropertiesWithOverrides(properties, overrides)

		assert.Equal(t, "many", merged["count"])
	})

	t.Run("nil properties does not panic", func(t *testing.T) {
		overrides := map[string]string{"key": "val"}

		merged := mergePropertiesWithOverrides(nil, overrides)

		assert.Equal(t, "val", merged["key"])
	})

	t.Run("empty overrides preserves originals", func(t *testing.T) {
		properties := map[string]any{"a": float64(1)}

		merged := mergePropertiesWithOverrides(properties, map[string]string{})

		assert.Equal(t, float64(1), merged["a"])
	})

	t.Run("mixed types all preserved", func(t *testing.T) {
		properties := map[string]any{
			"num":  float64(8),
			"flag": true,
			"name": "foo",
		}
		overrides := map[string]string{
			"num":  "16",
			"flag": "false",
			"name": "bar",
		}

		merged := mergePropertiesWithOverrides(properties, overrides)

		assert.Equal(t, float64(16), merged["num"])
		assert.Equal(t, false, merged["flag"])
		assert.Equal(t, "bar", merged["name"])
	})

	t.Run("original map not mutated", func(t *testing.T) {
		properties := map[string]any{"x": float64(1)}
		overrides := map[string]string{"x": "2"}

		_ = mergePropertiesWithOverrides(properties, overrides)

		assert.Equal(t, float64(1), properties["x"])
	})
}
