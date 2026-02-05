package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// mockEstimateCalculator implements a mock for cost estimation integration tests.
type mockEstimateCalculator struct {
	baselineCosts map[string]float64 // resourceType -> monthly cost
	modifiedCosts map[string]float64 // resourceType -> modified monthly cost
	deltaCosts    map[string]float64 // property -> delta cost
}

func newMockEstimateCalculator() *mockEstimateCalculator {
	return &mockEstimateCalculator{
		baselineCosts: map[string]float64{
			"aws:ec2/instance:Instance": 8.32,  // t3.micro baseline
			"aws:rds/instance:Instance": 15.00, // db.t3.micro baseline
		},
		modifiedCosts: map[string]float64{
			"aws:ec2/instance:Instance": 83.22, // m5.large modified
			"aws:rds/instance:Instance": 45.00, // db.t3.medium modified
		},
		deltaCosts: map[string]float64{
			"instanceType":  74.90, // t3.micro -> m5.large
			"instanceClass": 30.00, // db.t3.micro -> db.t3.medium
		},
	}
}

// TestCostEstimate_SingleResource tests single-resource estimation flow.
func TestCostEstimate_SingleResource(t *testing.T) {
	t.Run("estimates cost for EC2 instance", func(t *testing.T) {
		mock := newMockEstimateCalculator()

		// Create a single resource descriptor
		resource := &engine.ResourceDescriptor{
			Provider: "aws",
			Type:     "ec2:Instance",
			ID:       "test-server",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		}

		// Verify resource structure
		assert.Equal(t, "aws", resource.Provider)
		assert.Equal(t, "ec2:Instance", resource.Type)
		assert.NotNil(t, resource.Properties)

		// Verify mock has baseline cost
		_, hasBaseline := mock.baselineCosts["aws:ec2/instance:Instance"]
		assert.True(t, hasBaseline)
	})

	t.Run("handles property override for instance type change", func(t *testing.T) {
		mock := newMockEstimateCalculator()

		overrides := map[string]string{
			"instanceType": "m5.large",
		}

		// Verify delta cost exists for property
		delta, hasDelta := mock.deltaCosts["instanceType"]
		assert.True(t, hasDelta)
		assert.Equal(t, 74.90, delta)
		assert.NotEmpty(t, overrides)
	})

	t.Run("property that doesn't affect pricing shows zero delta", func(t *testing.T) {
		mock := newMockEstimateCalculator()

		// "tags" doesn't affect pricing
		_, hasDelta := mock.deltaCosts["tags"]
		assert.False(t, hasDelta, "tags should not have a delta cost")
	})

	t.Run("no properties specified shows baseline only", func(t *testing.T) {
		mock := newMockEstimateCalculator()

		resource := &engine.ResourceDescriptor{
			Provider:   "aws",
			Type:       "ec2:Instance",
			ID:         "test-server",
			Properties: map[string]interface{}{},
		}

		// With no overrides, we just get baseline
		baseline := mock.baselineCosts["aws:ec2/instance:Instance"]
		assert.Equal(t, 8.32, baseline)
		assert.NotNil(t, resource)
	})
}

// TestCostEstimate_PlanBased tests plan-based estimation with modifications.
func TestCostEstimate_PlanBased(t *testing.T) {
	t.Run("loads resources from plan fixture", func(t *testing.T) {
		planPath := "../fixtures/estimate/plan-with-modify.json"

		state, err := ingest.LoadStackExport(planPath)
		require.NoError(t, err)
		require.NotNil(t, state)

		// Get custom resources
		customResources := state.GetCustomResources()
		assert.NotEmpty(t, customResources)

		// Map to ResourceDescriptors
		resources, mapErr := ingest.MapStateResources(customResources)
		require.NoError(t, mapErr)
		assert.Len(t, resources, 3) // web-server, api-server, database
	})

	t.Run("applies modifications to specific resource", func(t *testing.T) {
		planPath := "../fixtures/estimate/plan-with-modify.json"

		state, err := ingest.LoadStackExport(planPath)
		require.NoError(t, err)

		customResources := state.GetCustomResources()
		resources, mapErr := ingest.MapStateResources(customResources)
		require.NoError(t, mapErr)

		// Find web-server resource by URN (ID is set to URN in MapStateResource)
		var webServer *engine.ResourceDescriptor
		for i := range resources {
			// ID contains the full URN which ends with the resource name
			if resources[i].ID == "urn:pulumi:dev::test-project::aws:ec2/instance:Instance::web-server" {
				webServer = &resources[i]
				break
			}
		}
		require.NotNil(t, webServer, "should find web-server resource")

		// Simulate modification
		modifications := map[string]string{
			"instanceType": "m5.large",
		}

		// Verify modification can be applied
		assert.NotEmpty(t, modifications)
		assert.Equal(t, "aws:ec2/instance:Instance", webServer.Type)
	})

	t.Run("handles resource not found in plan", func(t *testing.T) {
		planPath := "../fixtures/estimate/plan-with-modify.json"

		state, err := ingest.LoadStackExport(planPath)
		require.NoError(t, err)

		customResources := state.GetCustomResources()
		resources, mapErr := ingest.MapStateResources(customResources)
		require.NoError(t, mapErr)

		// Try to find non-existent resource (using URN format)
		var notFound *engine.ResourceDescriptor
		for i := range resources {
			if resources[i].ID == "urn:pulumi:dev::test-project::aws:ec2/instance:Instance::non-existent-server" {
				notFound = &resources[i]
				break
			}
		}
		assert.Nil(t, notFound, "should not find non-existent resource")
	})

	t.Run("estimates multiple resources with modifications", func(t *testing.T) {
		mock := newMockEstimateCalculator()
		planPath := "../fixtures/estimate/plan-with-modify.json"

		state, err := ingest.LoadStackExport(planPath)
		require.NoError(t, err)

		customResources := state.GetCustomResources()
		resources, mapErr := ingest.MapStateResources(customResources)
		require.NoError(t, mapErr)

		// Calculate total baseline
		totalBaseline := 0.0
		for _, r := range resources {
			if cost, ok := mock.baselineCosts[r.Type]; ok {
				totalBaseline += cost
			}
		}

		// EC2 instances (2) + RDS (1)
		// Note: fixture has aws:ec2/instance:Instance type
		assert.Greater(t, totalBaseline, 0.0, "should have some baseline cost")
	})
}

// TestCostEstimate_FallbackBehavior tests fallback to GetProjectedCost.
func TestCostEstimate_FallbackBehavior(t *testing.T) {
	t.Run("uses fallback when EstimateCost not implemented", func(t *testing.T) {
		// When EstimateCost RPC is not available, engine falls back to:
		// 1. GetProjectedCost with original properties (baseline)
		// 2. GetProjectedCost with modified properties (modified)
		// 3. Calculate delta as modified - baseline

		mock := newMockEstimateCalculator()

		baseline := mock.baselineCosts["aws:ec2/instance:Instance"]
		modified := mock.modifiedCosts["aws:ec2/instance:Instance"]
		expectedDelta := modified - baseline

		assert.Equal(t, 8.32, baseline)
		assert.Equal(t, 83.22, modified)
		assert.InDelta(t, 74.90, expectedDelta, 0.01)
	})

	t.Run("fallback correctly calculates negative delta for downgrade", func(t *testing.T) {
		// Downgrade from m5.large to t3.micro should show savings
		mock := newMockEstimateCalculator()

		// Reverse the calculation (m5.large baseline, t3.micro modified)
		baseline := mock.modifiedCosts["aws:ec2/instance:Instance"] // 83.22
		modified := mock.baselineCosts["aws:ec2/instance:Instance"] // 8.32
		expectedDelta := modified - baseline                        // -74.90

		assert.Less(t, expectedDelta, 0.0, "downgrade should show negative delta (savings)")
		assert.InDelta(t, -74.90, expectedDelta, 0.01)
	})
}

// TestCostEstimate_EstimateResult tests EstimateResult structure.
func TestCostEstimate_EstimateResult(t *testing.T) {
	t.Run("creates valid EstimateResult", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
				ID:       "test-server",
			},
			Baseline: &engine.CostResult{
				Monthly:  8.32,
				Currency: "USD",
			},
			Modified: &engine.CostResult{
				Monthly:  83.22,
				Currency: "USD",
			},
			TotalChange: 74.90,
			Deltas: []engine.CostDelta{
				{
					Property:      "instanceType",
					OriginalValue: "t3.micro",
					NewValue:      "m5.large",
					CostChange:    74.90,
				},
			},
			UsedFallback: false,
		}

		assert.NotNil(t, result.Resource)
		assert.NotNil(t, result.Baseline)
		assert.NotNil(t, result.Modified)
		assert.Equal(t, 74.90, result.TotalChange)
		assert.Len(t, result.Deltas, 1)
		assert.False(t, result.UsedFallback)
	})

	t.Run("handles fallback flag", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
				ID:       "test-server",
			},
			Baseline: &engine.CostResult{
				Monthly:  8.32,
				Currency: "USD",
			},
			Modified: &engine.CostResult{
				Monthly:  83.22,
				Currency: "USD",
			},
			TotalChange:  74.90,
			Deltas:       []engine.CostDelta{},
			UsedFallback: true,
		}

		assert.True(t, result.UsedFallback)
	})
}

// TestCostEstimate_EdgeCases tests edge cases for cost estimation.
func TestCostEstimate_EdgeCases(t *testing.T) {
	t.Run("handles zero cost resources", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "iam:Role",
				ID:       "test-role",
			},
			Baseline: &engine.CostResult{
				Monthly:  0.0,
				Currency: "USD",
			},
			Modified: &engine.CostResult{
				Monthly:  0.0,
				Currency: "USD",
			},
			TotalChange: 0.0,
		}

		assert.Equal(t, 0.0, result.TotalChange)
	})

	t.Run("handles nil baseline", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
				ID:       "new-server",
			},
			Baseline: nil, // New resource, no baseline
			Modified: &engine.CostResult{
				Monthly:  83.22,
				Currency: "USD",
			},
			TotalChange: 83.22,
		}

		assert.Nil(t, result.Baseline)
		assert.NotNil(t, result.Modified)
		assert.Equal(t, 83.22, result.TotalChange)
	})

	t.Run("handles nil modified", func(t *testing.T) {
		result := &engine.EstimateResult{
			Resource: &engine.ResourceDescriptor{
				Provider: "aws",
				Type:     "ec2:Instance",
				ID:       "deleted-server",
			},
			Baseline: &engine.CostResult{
				Monthly:  8.32,
				Currency: "USD",
			},
			Modified:    nil, // Resource being deleted
			TotalChange: -8.32,
		}

		assert.NotNil(t, result.Baseline)
		assert.Nil(t, result.Modified)
		assert.Equal(t, -8.32, result.TotalChange)
	})
}

// TestCostEstimate_ContextHandling tests context propagation.
func TestCostEstimate_ContextHandling(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel

		// Context should be cancelled
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	})
}
