package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/spec"
)

func TestProjectedCostIntegration(t *testing.T) {
	// Create temporary directory for specs
	tempDir := t.TempDir()

	// Create test spec file
	specContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  instanceType: t3.micro
  onDemandHourly: 0.0104
  monthlyEstimate: 7.59
  vcpu: 2
  memory: 1
  networkPerformance: "Up to 5 Gigabit"
  ebsOptimized: true
metadata:
  region: us-west-2
  operatingSystem: linux
  tenancy: shared
`

	specPath := filepath.Join(tempDir, "aws-ec2-t3.micro.yaml")
	err := os.WriteFile(specPath, []byte(specContent), 0644)
	require.NoError(t, err)

	// Create loader and engine
	loader := spec.NewLoader(tempDir)
	eng := engine.New(nil, loader) // No plugin clients for this test

	// Test resources
	resources := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			ID:       "i-123456789",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"region":       "us-west-2",
			},
		},
		{
			Type:     "aws:rds:Instance",
			ID:       "db-abcdefghi",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceClass": "db.t3.micro",
				"engine":        "mysql",
			},
		},
	}

	// Get projected costs
	results, err := eng.GetProjectedCost(context.Background(), resources)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Check first result (should use spec)
	ec2Result := results[0]
	assert.Equal(t, "aws:ec2:Instance", ec2Result.ResourceType)
	assert.Equal(t, "i-123456789", ec2Result.ResourceID)
	assert.Equal(t, "local-spec", ec2Result.Adapter)
	assert.Equal(t, "USD", ec2Result.Currency)
	assert.InDelta(t, 7.59, ec2Result.Monthly, 0.01)
	assert.InDelta(t, 0.0104, ec2Result.Hourly, 0.001)

	// Check second result (should use fallback)
	rdsResult := results[1]
	assert.Equal(t, "aws:rds:Instance", rdsResult.ResourceType)
	assert.Equal(t, "db-abcdefghi", rdsResult.ResourceID)
	// Should be "none" because no spec available and no plugins
	assert.Equal(t, "none", rdsResult.Adapter)
}

func TestFilteringIntegration(t *testing.T) {
	resources := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			ID:       "i-123",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
		{
			Type:     "aws:rds:Instance",
			ID:       "db-456",
			Provider: "aws",
		},
		{
			Type:     "azure:compute:VirtualMachine",
			ID:       "vm-789",
			Provider: "azure",
		},
	}

	tests := []struct {
		name     string
		filter   string
		expected []string // Expected resource IDs
	}{
		{
			name:     "all resources",
			filter:   "",
			expected: []string{"i-123", "db-456", "vm-789"},
		},
		{
			name:     "aws only",
			filter:   "provider=aws",
			expected: []string{"i-123", "db-456"},
		},
		{
			name:     "ec2 only",
			filter:   "type=ec2",
			expected: []string{"i-123"},
		},
		{
			name:     "t3.micro instances",
			filter:   "instanceType=t3.micro",
			expected: []string{"i-123"},
		},
		{
			name:     "azure resources",
			filter:   "provider=azure",
			expected: []string{"vm-789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := engine.FilterResources(resources, tt.filter)

			var actualIDs []string
			for _, r := range filtered {
				actualIDs = append(actualIDs, r.ID)
			}

			assert.ElementsMatch(t, tt.expected, actualIDs)
		})
	}
}

func TestAggregationIntegration(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      7.59,
			Hourly:       0.0104,
		},
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-456",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      15.18,
			Hourly:       0.0208,
		},
		{
			ResourceType: "aws:rds:Instance",
			ResourceID:   "db-789",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      50.0,
			Hourly:       0.068,
		},
		{
			ResourceType: "azure:compute:VirtualMachine",
			ResourceID:   "vm-abc",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      30.0,
			Hourly:       0.041,
		},
	}

	aggregated := engine.AggregateResults(results)

	// Test totals
	assert.InDelta(t, 102.77, aggregated.Summary.TotalMonthly, 0.01)
	assert.InDelta(t, 0.1402, aggregated.Summary.TotalHourly, 0.001)

	// Test provider aggregation
	assert.InDelta(t, 72.77, aggregated.Summary.ByProvider["aws"], 0.01)
	assert.InDelta(t, 30.0, aggregated.Summary.ByProvider["azure"], 0.01)

	// Test service aggregation
	assert.InDelta(t, 22.77, aggregated.Summary.ByService["ec2"], 0.01)
	assert.InDelta(t, 50.0, aggregated.Summary.ByService["rds"], 0.01)
	assert.InDelta(t, 30.0, aggregated.Summary.ByService["compute"], 0.01)

	// Test adapter aggregation
	assert.InDelta(t, 22.77, aggregated.Summary.ByAdapter["aws-plugin"], 0.01)
	assert.InDelta(t, 80.0, aggregated.Summary.ByAdapter["local-spec"], 0.01)

	// Test resource preservation
	assert.Len(t, aggregated.Resources, 4)
	assert.Equal(t, results, aggregated.Resources)
}

func TestSpecFallbackIntegration(t *testing.T) {
	// Create temporary directory for specs
	tempDir := t.TempDir()

	// Create multiple spec files to test fallback pattern
	specs := map[string]string{
		"aws-ec2-default.yaml": `provider: aws
service: ec2
sku: default
currency: USD
pricing:
  onDemandHourly: 0.05
`,
		"aws-rds-standard.yaml": `provider: aws
service: rds
sku: standard
currency: USD
pricing:
  monthlyEstimate: 25.0
`,
	}

	for filename, content := range specs {
		specPath := filepath.Join(tempDir, filename)
		err := os.WriteFile(specPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	loader := spec.NewLoader(tempDir)
	eng := engine.New(nil, loader)

	// Test resource that should find default spec
	resources := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			ID:       "i-unknown-type",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "unknown.type", // This won't match specific spec
			},
		},
		{
			Type:     "aws:rds:Instance",
			ID:       "db-standard",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceClass": "db.r5.large",
			},
		},
	}

	results, err := eng.GetProjectedCost(context.Background(), resources)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First should use aws-ec2-default.yaml
	ec2Result := results[0]
	assert.Equal(t, "local-spec", ec2Result.Adapter)
	assert.InDelta(t, 0.05*730, ec2Result.Monthly, 0.01)

	// Second should use aws-rds-standard.yaml (fallback to common SKU)
	rdsResult := results[1]
	assert.Equal(t, "local-spec", rdsResult.Adapter)
	assert.InDelta(t, 25.0, rdsResult.Monthly, 0.01)
}
