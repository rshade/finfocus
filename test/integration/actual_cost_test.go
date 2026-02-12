package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// TestStateBasedActualCost_LoadAndMapResources tests loading resources from state file.
func TestStateBasedActualCost_LoadAndMapResources(t *testing.T) {
	statePath := "../fixtures/state/valid-state.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Verify state structure
	assert.Equal(t, 3, state.Version)
	assert.NotEmpty(t, state.Deployment.Resources)

	// Get custom resources (exclude providers and component resources)
	customResources := state.GetCustomResources()
	assert.NotEmpty(t, customResources)

	// Map to ResourceDescriptors
	resources, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)
	assert.NotEmpty(t, resources)

	// Verify timestamp injection
	foundCreated := false
	for _, r := range resources {
		if r.Properties == nil {
			continue
		}
		if created, ok := r.Properties[ingest.PropertyPulumiCreated]; ok {
			assert.NotEmpty(t, created)
			foundCreated = true
		}
	}
	assert.True(t, foundCreated, "expected at least one resource with a created timestamp")
}

// TestStateBasedActualCost_ExtractTimestamps tests timestamp extraction from resources.
func TestStateBasedActualCost_ExtractTimestamps(t *testing.T) {
	statePath := "../fixtures/state/valid-state.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	customResources := state.GetCustomResources()
	resources, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)

	// Find earliest timestamp
	earliest, findErr := engine.FindEarliestCreatedTimestamp(resources)
	require.NoError(t, findErr)
	assert.False(t, earliest.IsZero(), "should find earliest timestamp")

	// Verify timestamp is in the past
	assert.True(t, earliest.Before(time.Now()), "earliest timestamp should be in the past")
}

// TestStateBasedActualCost_NoTimestamps tests handling of state without timestamps.
func TestStateBasedActualCost_NoTimestamps(t *testing.T) {
	statePath := "../fixtures/state/no-timestamps.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	customResources := state.GetCustomResources()
	resources, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)

	// Should fail to find earliest timestamp
	_, findErr := engine.FindEarliestCreatedTimestamp(resources)
	assert.Error(t, findErr)
	assert.ErrorIs(t, findErr, engine.ErrNoTimestampedResources)
}

// TestStateBasedActualCost_ImportedResources tests detection of imported resources.
func TestStateBasedActualCost_ImportedResources(t *testing.T) {
	statePath := "../fixtures/state/imported-resources.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	customResources := state.GetCustomResources()
	resources, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)

	// At least one resource should be marked as external
	foundExternal := false
	for _, r := range resources {
		if engine.IsExternalResource(r) {
			foundExternal = true
			break
		}
	}
	assert.True(t, foundExternal, "should find at least one imported/external resource")
}

// TestStateBasedActualCost_CostCalculation tests runtime-based cost calculation.
func TestStateBasedActualCost_CostCalculation(t *testing.T) {
	// Use a fixed reference time
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-48 * time.Hour) // 48 hours ago

	input := engine.StateCostInput{
		Resource: engine.ResourceDescriptor{
			Type:     "aws:ec2/instance:Instance",
			ID:       "i-test",
			Provider: "aws",
		},
		HourlyRate: 0.10,
		CreatedAt:  createdAt,
		IsExternal: false,
	}

	result := engine.CalculateStateCost(input, now)

	// Verify calculation: 0.10 * 48 = 4.80
	assert.InDelta(t, 4.80, result.TotalCost, 0.01)
	assert.InDelta(t, 48.0, result.RuntimeHours, 0.01)
	assert.Empty(t, result.Notes, "non-external resource should have no warning")
}

// TestStateBasedActualCost_ImportedResourceWarning tests warning for imported resources.
func TestStateBasedActualCost_ImportedResourceWarning(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-24 * time.Hour)

	input := engine.StateCostInput{
		Resource: engine.ResourceDescriptor{
			Type:     "aws:ec2/instance:Instance",
			ID:       "i-imported",
			Provider: "aws",
		},
		HourlyRate: 0.10,
		CreatedAt:  createdAt,
		IsExternal: true, // Imported resource
	}

	result := engine.CalculateStateCost(input, now)

	// Should have warning about imported resource
	assert.Contains(t, result.Notes, "Imported resource")
	assert.Contains(t, result.Notes, "timestamp reflects import time")
}

// TestStateBasedActualCost_HasTimestamps tests the HasTimestamps method.
func TestStateBasedActualCost_HasTimestamps(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "valid state with timestamps",
			path:     "../fixtures/state/valid-state.json",
			expected: true,
		},
		{
			name:     "state without timestamps",
			path:     "../fixtures/state/no-timestamps.json",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := ingest.LoadStackExport(tt.path)
			require.NoError(t, err)

			result := state.HasTimestamps()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStateBasedActualCost_InputValidation tests StateCostInput validation.
func TestStateBasedActualCost_InputValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     engine.StateCostInput
		expectErr bool
	}{
		{
			name: "valid input",
			input: engine.StateCostInput{
				HourlyRate: 0.10,
				CreatedAt:  time.Now().Add(-24 * time.Hour),
			},
			expectErr: false,
		},
		{
			name: "missing timestamp",
			input: engine.StateCostInput{
				HourlyRate: 0.10,
				// CreatedAt is zero
			},
			expectErr: true,
		},
		{
			name: "negative hourly rate",
			input: engine.StateCostInput{
				HourlyRate: -0.10,
				CreatedAt:  time.Now().Add(-24 * time.Hour),
			},
			expectErr: true,
		},
		{
			name: "future timestamp",
			input: engine.StateCostInput{
				HourlyRate: 0.10,
				CreatedAt:  time.Now().Add(24 * time.Hour),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStateBasedActualCost_CloudIdentifiers tests that cloud IDs and ARNs are extracted from state.
func TestStateBasedActualCost_CloudIdentifiers(t *testing.T) {
	statePath := "../fixtures/state/golden-eks-state.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)
	require.NotNil(t, state)

	customResources := state.GetCustomResources()
	resources, mapErr := ingest.MapStateResources(customResources)
	require.NoError(t, mapErr)

	// Every custom resource should have a cloud ID
	foundCloudID := false
	foundARN := false
	for _, r := range resources {
		if r.Properties == nil {
			continue
		}
		if cloudID, ok := r.Properties[ingest.PropertyPulumiCloudID]; ok {
			assert.NotEmpty(t, cloudID, "cloud ID should not be empty for %s", r.Type)
			foundCloudID = true
		}
		if arn, ok := r.Properties[ingest.PropertyPulumiARN]; ok {
			assert.NotEmpty(t, arn, "ARN should not be empty for %s", r.Type)
			foundARN = true
		}
		// Every resource should have URN
		urn, hasURN := r.Properties[ingest.PropertyPulumiURN]
		assert.True(t, hasURN, "URN should be present for %s", r.Type)
		assert.NotEmpty(t, urn)
	}

	assert.True(t, foundCloudID, "expected at least one resource with a cloud ID")
	assert.True(t, foundARN, "expected at least one resource with an ARN")
}

// TestMultiProviderAggregation_LoadAndMapResources tests loading multi-provider state file.
func TestMultiProviderAggregation_LoadAndMapResources(t *testing.T) {
	statePath := "../fixtures/state/multi-provider.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)
	require.NotNil(t, state)

	// Verify state structure
	assert.Equal(t, 3, state.Version)
	assert.NotEmpty(t, state.Deployment.Resources)

	// Get custom resources
	customResources := state.GetCustomResources()
	assert.NotEmpty(t, customResources)

	// Map to ResourceDescriptors
	resources, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)
	assert.NotEmpty(t, resources)

	// Verify multiple providers are present
	providers := make(map[string]bool)
	for _, r := range resources {
		if r.Provider != "" {
			providers[r.Provider] = true
		}
	}

	// Should have AWS, Azure, and GCP providers
	assert.True(t, len(providers) >= 3, "expected at least 3 providers, got %d", len(providers))
}

// TestMultiProviderAggregation_CrossProviderCostCalculation tests cross-provider cost aggregation.
func TestMultiProviderAggregation_CrossProviderCostCalculation(t *testing.T) {
	// Create mock cost results from multiple providers
	jan1 := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	jan2 := time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)

	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-0123456789abcdef0",
			TotalCost:    100.0,
			Currency:     "USD",
			StartDate:    jan1,
			EndDate:      jan2,
		},
		{
			ResourceType: "azure-native:compute:VirtualMachine",
			ResourceID:   "azure-web-server",
			TotalCost:    150.0,
			Currency:     "USD",
			StartDate:    jan1,
			EndDate:      jan2,
		},
		{
			ResourceType: "gcp:compute/instance:Instance",
			ResourceID:   "gcp-web-server",
			TotalCost:    75.0,
			Currency:     "USD",
			StartDate:    jan1,
			EndDate:      jan2,
		},
	}

	// Aggregate by daily
	aggregations, err := engine.CreateCrossProviderAggregation(results, engine.GroupByDaily)
	require.NoError(t, err)
	require.Len(t, aggregations, 1)

	agg := aggregations[0]
	assert.Equal(t, "2025-12-01", agg.Period)
	assert.Equal(t, 325.0, agg.Total) // 100 + 150 + 75
	assert.Equal(t, "USD", agg.Currency)

	// Verify all providers are represented
	assert.Contains(t, agg.Providers, "aws")
	assert.Contains(t, agg.Providers, "azure-native")
	assert.Contains(t, agg.Providers, "gcp")

	assert.Equal(t, 100.0, agg.Providers["aws"])
	assert.Equal(t, 150.0, agg.Providers["azure-native"])
	assert.Equal(t, 75.0, agg.Providers["gcp"])
}

// TestMultiProviderAggregation_MonthlyGrouping tests monthly aggregation across providers.
func TestMultiProviderAggregation_MonthlyGrouping(t *testing.T) {
	dec1 := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	dec31 := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-0123456789abcdef0",
			TotalCost:    3100.0, // Full month AWS cost
			Currency:     "USD",
			StartDate:    dec1,
			EndDate:      dec31,
		},
		{
			ResourceType: "azure-native:compute:VirtualMachine",
			ResourceID:   "azure-web-server",
			TotalCost:    4650.0, // Full month Azure cost
			Currency:     "USD",
			StartDate:    dec1,
			EndDate:      dec31,
		},
		{
			ResourceType: "gcp:compute/instance:Instance",
			ResourceID:   "gcp-web-server",
			TotalCost:    2325.0, // Full month GCP cost
			Currency:     "USD",
			StartDate:    dec1,
			EndDate:      dec31,
		},
	}

	// Aggregate by monthly
	aggregations, err := engine.CreateCrossProviderAggregation(results, engine.GroupByMonthly)
	require.NoError(t, err)
	require.Len(t, aggregations, 1)

	agg := aggregations[0]
	assert.Equal(t, "2025-12", agg.Period)
	assert.Equal(t, 10075.0, agg.Total) // 3100 + 4650 + 2325
	assert.Equal(t, "USD", agg.Currency)
}
