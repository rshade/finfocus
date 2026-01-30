package engine_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestAggregateResults(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      10.0,
			Hourly:       0.014,
		},
		{
			ResourceType: "aws:rds:Instance",
			ResourceID:   "db-456",
			Adapter:      "aws-plugin",
			Currency:     "USD",
			Monthly:      50.0,
			Hourly:       0.068,
		},
		{
			ResourceType: "azure:compute:VirtualMachine",
			ResourceID:   "vm-789",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      25.0,
			Hourly:       0.034,
		},
	}

	aggregated := engine.AggregateResults(results)

	// Check summary totals
	assert.InDelta(t, 85.0, aggregated.Summary.TotalMonthly, 0.01)
	assert.InDelta(t, 0.116, aggregated.Summary.TotalHourly, 0.001)
	assert.Equal(t, "USD", aggregated.Summary.Currency)

	// Check provider breakdown
	assert.InDelta(t, 60.0, aggregated.Summary.ByProvider["aws"], 0.01)
	assert.InDelta(t, 25.0, aggregated.Summary.ByProvider["azure"], 0.01)

	// Check service breakdown
	assert.InDelta(t, 10.0, aggregated.Summary.ByService["ec2"], 0.01)
	assert.InDelta(t, 50.0, aggregated.Summary.ByService["rds"], 0.01)
	assert.InDelta(t, 25.0, aggregated.Summary.ByService["compute"], 0.01)

	// Check adapter breakdown
	assert.InDelta(t, 60.0, aggregated.Summary.ByAdapter["aws-plugin"], 0.01)
	assert.InDelta(t, 25.0, aggregated.Summary.ByAdapter["local-spec"], 0.01)

	// Check resources are preserved
	assert.Len(t, aggregated.Resources, 3)
}

func TestFilterResources(t *testing.T) {
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
		expected int
	}{
		{
			name:     "no filter",
			filter:   "",
			expected: 3,
		},
		{
			name:     "filter by provider",
			filter:   "provider=aws",
			expected: 2,
		},
		{
			name:     "filter by type",
			filter:   "type=ec2",
			expected: 1,
		},
		{
			name:     "filter by service",
			filter:   "service=rds",
			expected: 1,
		},
		{
			name:     "filter by property",
			filter:   "instanceType=t3.micro",
			expected: 1,
		},
		{
			name:     "invalid filter",
			filter:   "invalid",
			expected: 3, // Should include all on invalid filter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := engine.FilterResources(resources, tt.filter)
			assert.Len(t, filtered, tt.expected)
		})
	}
}

func TestValidateFilter(t *testing.T) {
	tests := []struct {
		name      string
		filter    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "empty filter is valid",
			filter:    "",
			wantError: false,
		},
		{
			name:      "valid filter",
			filter:    "type=aws:ec2/instance",
			wantError: false,
		},
		{
			name:      "valid tag filter",
			filter:    "tag:env=prod",
			wantError: false,
		},
		{
			name:      "missing equals sign",
			filter:    "type",
			wantError: true,
			errorMsg:  "invalid filter syntax",
		},
		{
			name:      "empty value",
			filter:    "type=",
			wantError: true,
			errorMsg:  "filter key and value must be non-empty",
		},
		{
			name:      "empty key",
			filter:    "=value",
			wantError: true,
			errorMsg:  "filter key and value must be non-empty",
		},
		{
			name:      "whitespace only value",
			filter:    "type=   ",
			wantError: true,
			errorMsg:  "filter key and value must be non-empty",
		},
		{
			name:      "whitespace only key",
			filter:    "   =value",
			wantError: true,
			errorMsg:  "filter key and value must be non-empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidateFilter(tt.filter)
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetProjectedCostEmpty(t *testing.T) {
	// Test with no clients and no loader
	eng := engine.New(nil, nil)

	results, err := eng.GetProjectedCost(context.Background(), []engine.ResourceDescriptor{
		{
			Type: "aws:ec2:Instance",
			ID:   "i-123",
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "aws:ec2:Instance", result.ResourceType)
	assert.Equal(t, "i-123", result.ResourceID)
	assert.Equal(t, "none", result.Adapter)
	assert.Equal(t, "USD", result.Currency)
	assert.InDelta(t, 0.0, result.Monthly, 0.01)
	assert.InDelta(t, 0.0, result.Hourly, 0.01)
	assert.Contains(t, result.Notes, "No pricing information available")
}

// MockSpecLoader for testing.
type MockSpecLoader struct {
	specs map[string]*engine.PricingSpec
}

func (m *MockSpecLoader) LoadSpec(provider, service, sku string) (interface{}, error) {
	key := provider + "-" + service + "-" + sku
	if spec, ok := m.specs[key]; ok {
		return spec, nil
	}
	return nil, engine.ErrNoCostData
}

func TestGetProjectedCostWithSpecLoader(t *testing.T) {
	loader := &MockSpecLoader{
		specs: map[string]*engine.PricingSpec{
			"aws-ec2-t3.micro": {
				Provider: "aws",
				Service:  "ec2",
				SKU:      "t3.micro",
				Currency: "USD",
				Pricing: map[string]interface{}{
					"monthlyEstimate": 7.59,
				},
			},
		},
	}

	eng := engine.New(nil, loader)

	results, err := eng.GetProjectedCost(context.Background(), []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			ID:       "i-123",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "aws:ec2:Instance", result.ResourceType)
	assert.Equal(t, "i-123", result.ResourceID)
	assert.Equal(t, "local-spec", result.Adapter)
	assert.Equal(t, "USD", result.Currency)
	assert.InDelta(t, 7.59, result.Monthly, 0.01)
	assert.InDelta(t, 7.59/730, result.Hourly, 0.001)
	assert.Contains(t, result.Notes, "local spec")
}

func TestGetProjectedCost_PropagatesContextError(t *testing.T) {
	eng := engine.New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := eng.GetProjectedCost(ctx, []engine.ResourceDescriptor{
		{Type: "aws:ec2:Instance", ID: "i-123"},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Len(t, results, 0)
}

func TestFormatPeriod(t *testing.T) {
	tests := []struct {
		name     string
		from     time.Time
		to       time.Time
		expected string
	}{
		{
			name:     "1 day",
			from:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			expected: "1 day",
		},
		{
			name:     "3 days",
			from:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC),
			expected: "3 days",
		},
		{
			name:     "1 week",
			from:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC),
			expected: "1 weeks",
		},
		{
			name:     "2 weeks",
			from:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: "2 weeks",
		},
		{
			name:     "1 month",
			from:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			expected: "1 months",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.FormatPeriod(tt.from, tt.to)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesTags(t *testing.T) {
	tests := []struct {
		name     string
		resource engine.ResourceDescriptor
		tags     map[string]string
		expected bool
	}{
		{
			name:     "empty tags match any resource",
			resource: engine.ResourceDescriptor{Properties: map[string]interface{}{"env": "prod"}},
			tags:     map[string]string{},
			expected: true,
		},
		{
			name: "matching tag",
			resource: engine.ResourceDescriptor{
				Properties: map[string]interface{}{"env": "prod", "team": "backend"},
			},
			tags:     map[string]string{"env": "prod"},
			expected: true,
		},
		{
			name: "multiple matching tags",
			resource: engine.ResourceDescriptor{
				Properties: map[string]interface{}{"env": "prod", "team": "backend"},
			},
			tags:     map[string]string{"env": "prod", "team": "backend"},
			expected: true,
		},
		{
			name: "non-matching tag",
			resource: engine.ResourceDescriptor{
				Properties: map[string]interface{}{"env": "staging"},
			},
			tags:     map[string]string{"env": "prod"},
			expected: false,
		},
		{
			name: "missing tag",
			resource: engine.ResourceDescriptor{
				Properties: map[string]interface{}{"team": "backend"},
			},
			tags:     map[string]string{"env": "prod"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.MatchesTags(tt.resource, tt.tags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroupResults(t *testing.T) {
	eng := &engine.Engine{}

	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-123",
			Adapter:      "aws",
			TotalCost:    10.0,
		},
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-456",
			Adapter:      "aws",
			TotalCost:    15.0,
		},
		{
			ResourceType: "gcp:compute/instance:Instance",
			ResourceID:   "vm-789",
			Adapter:      "gcp",
			TotalCost:    20.0,
		},
	}

	tests := []struct {
		name             string
		groupBy          engine.GroupBy
		expectedGroups   int
		expectedTotalSum float64
	}{
		{
			name:             "no grouping",
			groupBy:          engine.GroupByNone,
			expectedGroups:   3,
			expectedTotalSum: 45.0,
		},
		{
			name:             "group by type",
			groupBy:          engine.GroupByType,
			expectedGroups:   2, // aws:ec2 and gcp:compute grouped separately
			expectedTotalSum: 45.0,
		},
		{
			name:             "group by provider",
			groupBy:          engine.GroupByProvider,
			expectedGroups:   2, // aws and gcp
			expectedTotalSum: 45.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grouped := eng.GroupResults(results, tt.groupBy)
			assert.Len(t, grouped, tt.expectedGroups)

			// Calculate total cost across all groups
			var totalCost float64
			for _, result := range grouped {
				totalCost += result.TotalCost
			}
			assert.InDelta(t, tt.expectedTotalSum, totalCost, 0.01)
		})
	}
}

func TestAggregateResultsFunction(t *testing.T) {
	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-123",
			TotalCost:    10.0,
			Monthly:      100.0,
			Breakdown:    map[string]float64{"compute": 10.0},
		},
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "i-456",
			TotalCost:    15.0,
			Monthly:      150.0,
			Breakdown:    map[string]float64{"compute": 15.0},
		},
	}

	aggregated := engine.AggregateResultsInternal(results, "test-group")

	assert.Equal(t, "test-group", aggregated.ResourceType)
	assert.Equal(t, "aggregated", aggregated.Adapter)
	assert.InDelta(t, 25.0, aggregated.TotalCost, 0.01)
	assert.InDelta(t, 250.0, aggregated.Monthly, 0.01)
	assert.InDelta(t, 25.0, aggregated.Breakdown["compute"], 0.01)
	assert.Contains(t, aggregated.Notes, "2 resources")
}

func TestCreateCrossProviderAggregation(t *testing.T) {
	tests := []struct {
		name        string
		results     []engine.CostResult
		groupBy     engine.GroupBy
		expected    []engine.CrossProviderAggregation
		expectError bool
	}{
		{
			name:        "daily aggregation with multiple providers",
			expectError: false,
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2:Instance",
					TotalCost:    100.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ResourceType: "azure:compute:VirtualMachine",
					TotalCost:    50.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ResourceType: "aws:rds:Instance",
					TotalCost:    75.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
				},
			},
			groupBy: engine.GroupByDaily,
			expected: []engine.CrossProviderAggregation{
				{
					Period: "2025-01-01",
					Providers: map[string]float64{
						"aws":   100.0,
						"azure": 50.0,
					},
					Total:    150.0,
					Currency: "USD",
				},
				{
					Period: "2025-01-02",
					Providers: map[string]float64{
						"aws": 75.0,
					},
					Total:    75.0,
					Currency: "USD",
				},
			},
		},
		{
			name:        "monthly aggregation",
			expectError: false,
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2:Instance",
					TotalCost:    1500.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				},
				{
					ResourceType: "aws:rds:Instance",
					TotalCost:    800.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC),
				},
			},
			groupBy: engine.GroupByMonthly,
			expected: []engine.CrossProviderAggregation{
				{
					Period: "2025-01",
					Providers: map[string]float64{
						"aws": 1500.0,
					},
					Total:    1500.0,
					Currency: "USD",
				},
				{
					Period: "2025-02",
					Providers: map[string]float64{
						"aws": 800.0,
					},
					Total:    800.0,
					Currency: "USD",
				},
			},
		},
		{
			name:        "invalid groupby returns error",
			results:     []engine.CostResult{{ResourceType: "aws:ec2:Instance", Currency: "USD"}},
			groupBy:     engine.GroupByResource,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty results returns error",
			results:     []engine.CostResult{},
			groupBy:     engine.GroupByDaily,
			expected:    nil,
			expectError: true,
		},
		{
			name: "mixed currencies returns error",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2:Instance",
					TotalCost:    100.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					ResourceType: "azure:compute:VirtualMachine",
					TotalCost:    50.0,
					Currency:     "EUR",
					StartDate:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			groupBy:     engine.GroupByDaily,
			expected:    nil,
			expectError: true,
		},
		{
			name: "invalid date range returns error",
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2:Instance",
					TotalCost:    100.0,
					Currency:     "USD",
					StartDate:    time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
					EndDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), // End before start
				},
			},
			groupBy:     engine.GroupByDaily,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.CreateCrossProviderAggregation(tt.results, tt.groupBy)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil result, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d aggregations, got %d", len(tt.expected), len(result))
				return
			}

			// Convert to map for easier comparison
			resultMap := make(map[string]engine.CrossProviderAggregation)
			for _, agg := range result {
				resultMap[agg.Period] = agg
			}

			for _, expected := range tt.expected {
				actual, exists := resultMap[expected.Period]
				if !exists {
					t.Errorf("Expected period %s not found in results", expected.Period)
					continue
				}

				if actual.Total != expected.Total {
					t.Errorf(
						"Period %s: expected total %.2f, got %.2f",
						expected.Period,
						expected.Total,
						actual.Total,
					)
				}

				if actual.Currency != expected.Currency {
					t.Errorf(
						"Period %s: expected currency %s, got %s",
						expected.Period,
						expected.Currency,
						actual.Currency,
					)
				}

				for provider, expectedCost := range expected.Providers {
					actualCost, providerExists := actual.Providers[provider]
					if !providerExists {
						t.Errorf(
							"Period %s: expected provider %s not found",
							expected.Period,
							provider,
						)
						continue
					}

					if actualCost != expectedCost {
						t.Errorf("Period %s, provider %s: expected cost %.2f, got %.2f",
							expected.Period, provider, expectedCost, actualCost)
					}
				}
			}
		})
	}
}

func TestGetActualCostWithOptions(t *testing.T) {
	eng := engine.New(nil, nil) // No plugins for this test

	resources := []engine.ResourceDescriptor{
		{
			Type:       "aws:ec2/instance:Instance",
			ID:         "i-123",
			Provider:   "aws",
			Properties: map[string]interface{}{"env": "prod"},
		},
		{
			Type:       "aws:s3/bucket:Bucket",
			ID:         "bucket-456",
			Provider:   "aws",
			Properties: map[string]interface{}{"env": "staging"},
		},
	}

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		request         engine.ActualCostRequest
		expectedResults int
	}{
		{
			name: "no filters",
			request: engine.ActualCostRequest{
				Resources: resources,
				From:      from,
				To:        to,
			},
			expectedResults: 2,
		},
		{
			name: "filter by tag",
			request: engine.ActualCostRequest{
				Resources: resources,
				From:      from,
				To:        to,
				Tags:      map[string]string{"env": "prod"},
			},
			expectedResults: 1,
		},
		{
			name: "group by provider",
			request: engine.ActualCostRequest{
				Resources: resources,
				From:      from,
				To:        to,
				GroupBy:   "provider",
			},
			expectedResults: 1, // Both are AWS resources, should be grouped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results, err := eng.GetActualCostWithOptions(ctx, tt.request)

			require.NoError(t, err)
			assert.Len(t, results, tt.expectedResults)

			// Check results based on test case
			for _, result := range results {
				if tt.name == "group by provider" {
					// When grouped, adapter should be "aggregated"
					assert.Equal(t, "aggregated", result.Adapter)
				} else {
					// Individual results should have "none" adapter
					assert.Equal(t, "none", result.Adapter)
				}
				assert.Equal(t, "1 weeks", result.CostPeriod)
			}
		})
	}
}

func TestGetActualCostWithOptions_PropagatesContextError(t *testing.T) {
	eng := engine.New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := engine.ActualCostRequest{
		Resources: []engine.ResourceDescriptor{
			{Type: "aws:ec2:Instance", ID: "i-123", Provider: "aws"},
		},
		From: time.Now().Add(-1 * time.Hour),
		To:   time.Now(),
	}

	results, err := eng.GetActualCostWithOptions(ctx, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Len(t, results, 0)
}

func TestGroupByValidation(t *testing.T) {
	tests := []struct {
		name        string
		groupBy     engine.GroupBy
		isValid     bool
		isTimeBased bool
	}{
		{"resource grouping", engine.GroupByResource, true, false},
		{"type grouping", engine.GroupByType, true, false},
		{"provider grouping", engine.GroupByProvider, true, false},
		{"date grouping", engine.GroupByDate, true, false},
		{"daily grouping", engine.GroupByDaily, true, true},
		{"monthly grouping", engine.GroupByMonthly, true, true},
		{"none grouping", engine.GroupByNone, true, false},
		{"invalid grouping", engine.GroupBy("invalid"), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isValid, tt.groupBy.IsValid())
			assert.Equal(t, tt.isTimeBased, tt.groupBy.IsTimeBasedGrouping())
			assert.Equal(t, string(tt.groupBy), tt.groupBy.String())
		})
	}
}

func TestCurrencySymbolMapping(t *testing.T) {
	// Test via the renderCrossProviderTable function - create minimal aggregations to test currency symbol
	aggregations := []engine.CrossProviderAggregation{
		{
			Period:    "2025-01-01",
			Total:     100.0,
			Currency:  "USD",
			Providers: map[string]float64{"aws": 100.0},
		},
		{
			Period:    "2025-01-02",
			Total:     200.0,
			Currency:  "EUR",
			Providers: map[string]float64{"azure": 200.0},
		},
	}

	// We can't easily test the output formatting without mocking stdout,
	// but we can at least verify the function doesn't panic with different currencies
	err := engine.RenderCrossProviderAggregation(
		io.Discard,
		engine.OutputJSON,
		aggregations,
		engine.GroupByDaily,
	)
	require.NoError(t, err)
}

func TestGetProjectedCostWithErrorsReturnType(t *testing.T) {
	// Test that GetProjectedCostWithErrors returns *CostResultWithErrors
	eng := engine.New(nil, nil)

	result, err := eng.GetProjectedCostWithErrors(context.Background(), []engine.ResourceDescriptor{
		{
			Type: "aws:ec2:Instance",
			ID:   "i-123",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result, "Result should not be nil")

	// Verify the result contains expected fields
	assert.NotNil(t, result.Results, "Results slice should not be nil")
	assert.NotNil(t, result.Errors, "Errors slice should not be nil")
	assert.Len(t, result.Results, 1, "Should have one result for the input resource")

	// Verify the result has the expected resource info
	assert.Equal(t, "aws:ec2:Instance", result.Results[0].ResourceType)
	assert.Equal(t, "i-123", result.Results[0].ResourceID)
	assert.Equal(t, "none", result.Results[0].Adapter, "Should have 'none' adapter when no plugins")
	assert.Contains(t, result.Results[0].Notes, "No pricing information available")

	// Verify HasErrors works correctly
	assert.False(t, result.HasErrors(), "Should not have errors for this scenario")
	assert.Empty(t, result.ErrorSummary())
}

func TestGetProjectedCostWithErrorsMultipleResources(t *testing.T) {
	// Test with multiple resources to ensure all are processed
	eng := engine.New(nil, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2:Instance", ID: "i-123"},
		{Type: "aws:rds:Instance", ID: "db-456"},
		{Type: "azure:compute:VirtualMachine", ID: "vm-789"},
	}

	result, err := eng.GetProjectedCostWithErrors(context.Background(), resources)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Results, 3, "Should have one result per resource")
	assert.Empty(t, result.Errors, "Should have no errors with no plugins")

	// Verify each resource type is preserved
	for i, res := range resources {
		assert.Equal(t, res.Type, result.Results[i].ResourceType)
		assert.Equal(t, res.ID, result.Results[i].ResourceID)
	}
}

func TestGetActualCostWithOptionsAndErrors(t *testing.T) {
	t.Run("returns results for resources without plugins", func(t *testing.T) {
		eng := engine.New(nil, nil)
		request := engine.ActualCostRequest{
			Resources: []engine.ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-123"},
				{Type: "aws:rds:Instance", ID: "db-456"},
			},
			From: time.Now().Add(-24 * time.Hour),
			To:   time.Now(),
		}

		result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Results, 2, "Should have one result per resource")
		assert.False(t, result.HasErrors(), "Should have no errors with no plugins")
	})

	t.Run("filters resources by tags", func(t *testing.T) {
		eng := engine.New(nil, nil)
		request := engine.ActualCostRequest{
			Resources: []engine.ResourceDescriptor{
				{
					Type: "aws:ec2:Instance",
					ID:   "i-123",
					Properties: map[string]interface{}{
						"env": "prod",
					},
				},
				{
					Type: "aws:ec2:Instance",
					ID:   "i-456",
					Properties: map[string]interface{}{
						"env": "dev",
					},
				},
			},
			From: time.Now().Add(-24 * time.Hour),
			To:   time.Now(),
			Tags: map[string]string{"env": "prod"},
		}

		result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Results, 1, "Should only include prod resources")
		assert.Equal(t, "i-123", result.Results[0].ResourceID)
	})

	t.Run("empty resources returns empty results", func(t *testing.T) {
		eng := engine.New(nil, nil)
		request := engine.ActualCostRequest{
			Resources: []engine.ResourceDescriptor{},
			From:      time.Now().Add(-24 * time.Hour),
			To:        time.Now(),
		}

		result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Results, 0)
		assert.False(t, result.HasErrors())
	})

	t.Run("groups results when groupBy is specified", func(t *testing.T) {
		eng := engine.New(nil, nil)
		request := engine.ActualCostRequest{
			Resources: []engine.ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-123"},
				{Type: "aws:ec2:Instance", ID: "i-456"},
			},
			From:    time.Now().Add(-24 * time.Hour),
			To:      time.Now(),
			GroupBy: "type",
		}

		result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)

		require.NoError(t, err)
		require.NotNil(t, result)
		// Results should be grouped by type
		assert.GreaterOrEqual(t, len(result.Results), 1)
	})
}

// T050: Test Engine.GetRecommendationsForResources method returns empty results when no plugins available.
func TestGetRecommendationsForResources_NoPlugins(t *testing.T) {
	eng := engine.New(nil, nil) // No plugins

	resources := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			ID:       "i-123",
			Provider: "aws",
		},
	}

	result, err := eng.GetRecommendationsForResources(context.Background(), resources)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Recommendations, "Should return empty recommendations when no plugins")
	assert.Empty(t, result.Errors, "Should have no errors when no plugins")
}

// T050: Test Engine.GetRecommendationsForResources with empty resource list.
func TestGetRecommendationsForResources_EmptyResources(t *testing.T) {
	eng := engine.New(nil, nil)

	result, err := eng.GetRecommendationsForResources(context.Background(), []engine.ResourceDescriptor{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Recommendations)
}

// T051: Test MergeRecommendations function combines recommendations from multiple sources.
func TestMergeRecommendations(t *testing.T) {
	costResults := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Monthly:      100.0,
		},
		{
			ResourceType: "aws:rds:Instance",
			ResourceID:   "db-456",
			Monthly:      200.0,
		},
	}

	recommendations := []engine.Recommendation{
		{
			ResourceID:       "i-123",
			Type:             "Right-sizing",
			Description:      "Downsize instance to t3.small",
			EstimatedSavings: 25.0,
			Currency:         "USD",
		},
		{
			ResourceID:       "db-456",
			Type:             "Terminate",
			Description:      "Terminate idle instance",
			EstimatedSavings: 100.0,
			Currency:         "USD",
		},
	}

	// Test merging recommendations based on resource IDs
	merged := engine.MergeRecommendations(costResults, recommendations)

	require.Len(t, merged, 2)
	// First resource should have first recommendation
	assert.Len(t, merged[0].Recommendations, 1)
	assert.Equal(t, "Right-sizing", merged[0].Recommendations[0].Type)
	// Second resource should have second recommendation
	assert.Len(t, merged[1].Recommendations, 1)
	assert.Equal(t, "Terminate", merged[1].Recommendations[0].Type)
}

// T051: Test MergeRecommendations with no recommendations.
func TestMergeRecommendations_Empty(t *testing.T) {
	costResults := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance",
			ResourceID:   "i-123",
			Monthly:      100.0,
		},
	}

	merged := engine.MergeRecommendations(costResults, nil)

	require.Len(t, merged, 1)
	assert.Empty(t, merged[0].Recommendations)
}

// T052: Test RecommendationsResult type structure.
func TestRecommendationsResult_Structure(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{
			{
				Type:             "Right-sizing",
				Description:      "Switch to t3.small",
				EstimatedSavings: 15.0,
				Currency:         "USD",
			},
		},
		Errors: []engine.RecommendationError{
			{
				PluginName: "aws-plugin",
				Error:      "connection timeout",
			},
		},
		TotalSavings: 15.0,
		Currency:     "USD",
	}

	assert.True(t, result.HasErrors())
	assert.Len(t, result.Recommendations, 1)
	assert.Equal(t, 15.0, result.TotalSavings)
}

// T053: Test error handling for recommendation failures.
func TestRecommendationsResult_ErrorHandling(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{},
		Errors: []engine.RecommendationError{
			{
				PluginName: "plugin-1",
				Error:      "error 1",
			},
			{
				PluginName: "plugin-2",
				Error:      "error 2",
			},
		},
	}

	assert.True(t, result.HasErrors())
	summary := result.ErrorSummary()
	assert.Contains(t, summary, "plugin-1")
	assert.Contains(t, summary, "plugin-2")
	assert.Contains(t, summary, "error 1")
}

// T053: Test RecommendationsResult without errors.
func TestRecommendationsResult_NoErrors(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{
			{Type: "Right-sizing", EstimatedSavings: 50.0, Currency: "USD"},
		},
		TotalSavings: 50.0,
		Currency:     "USD",
	}

	assert.False(t, result.HasErrors())
	assert.Empty(t, result.ErrorSummary())
}

// TestConvertValueToString tests the convertValueToString helper function.
func TestConvertValueToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
		},
		{
			name:     "string value",
			input:    "t2.micro",
			expected: "t2.micro",
		},
		{
			name:     "integer as float64",
			input:    float64(42),
			expected: "42",
		},
		{
			name:     "float value",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "nested map with value key",
			input:    map[string]interface{}{"value": "t3.medium"},
			expected: "t3.medium",
		},
		{
			name:     "nested map with id key",
			input:    map[string]interface{}{"id": "subnet-12345"},
			expected: "subnet-12345",
		},
		{
			name:     "nested map with name key",
			input:    map[string]interface{}{"name": "my-resource"},
			expected: "my-resource",
		},
		{
			name:     "nested map with single arbitrary key",
			input:    map[string]interface{}{"instanceType": "t2.large"},
			expected: "t2.large",
		},
		{
			name:     "nested map with multiple keys (no value/id/name)",
			input:    map[string]interface{}{"foo": "bar", "baz": "qux"},
			expected: "map[baz:qux foo:bar]",
		},
		{
			name:     "deeply nested value",
			input:    map[string]interface{}{"value": map[string]interface{}{"value": "deep"}},
			expected: "deep",
		},
		{
			name:     "single element array",
			input:    []interface{}{"only-one"},
			expected: "only-one",
		},
		{
			name:     "multi element array",
			input:    []interface{}{"a", "b", "c"},
			expected: "a,b,c",
		},
		{
			name:     "empty array",
			input:    []interface{}{},
			expected: "",
		},
		{
			name:     "array with nested map",
			input:    []interface{}{map[string]interface{}{"value": "nested-in-array"}},
			expected: "nested-in-array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.ConvertValueToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConvertToProto tests the convertToProto function with various property types.
func TestConvertToProto(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]interface{}
		expected   map[string]string
	}{
		{
			name:       "nil properties",
			properties: nil,
			expected:   map[string]string{},
		},
		{
			name:       "empty properties",
			properties: map[string]interface{}{},
			expected:   map[string]string{},
		},
		{
			name: "simple string properties",
			properties: map[string]interface{}{
				"instanceType": "t2.micro",
				"region":       "us-east-1",
			},
			expected: map[string]string{
				"instanceType": "t2.micro",
				"region":       "us-east-1",
			},
		},
		{
			name: "nested value pattern from Pulumi",
			properties: map[string]interface{}{
				"instanceType": map[string]interface{}{"value": "t3.large"},
				"ami":          map[string]interface{}{"id": "ami-12345678"},
				"subnetId":     map[string]interface{}{"id": "subnet-abcdef"},
			},
			expected: map[string]string{
				"instanceType": "t3.large",
				"ami":          "ami-12345678",
				"subnetId":     "subnet-abcdef",
			},
		},
		{
			name: "mixed property types",
			properties: map[string]interface{}{
				"instanceType":   "t2.micro",
				"availableZone":  map[string]interface{}{"value": "us-east-1a"},
				"count":          float64(3),
				"enableMonitor":  true,
				"securityGroups": []interface{}{"sg-1", "sg-2"},
				"singleSG":       []interface{}{"sg-only"},
			},
			expected: map[string]string{
				"instanceType":   "t2.micro",
				"availableZone":  "us-east-1a",
				"count":          "3",
				"enableMonitor":  "true",
				"securityGroups": "sg-1,sg-2",
				"singleSG":       "sg-only",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.ConvertToProto(tt.properties)
			assert.Equal(t, tt.expected, result)
		})
	}
}
