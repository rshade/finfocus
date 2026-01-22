package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// TestCalculateBudgetHealthFromPercentage tests the core health calculation function (FR-001).
func TestCalculateBudgetHealthFromPercentage(t *testing.T) {
	tests := []struct {
		name           string
		percentageUsed float64
		expected       pbc.BudgetHealthStatus
	}{
		// OK range: 0-79%
		{
			name:           "0% is OK",
			percentageUsed: 0,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name:           "50% is OK",
			percentageUsed: 50,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name:           "79% is OK",
			percentageUsed: 79,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name:           "79.99% is OK",
			percentageUsed: 79.99,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		// WARNING range: 80-89%
		{
			name:           "80% is WARNING (boundary)",
			percentageUsed: 80,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name:           "85% is WARNING",
			percentageUsed: 85,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name:           "89% is WARNING",
			percentageUsed: 89,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name:           "89.99% is WARNING",
			percentageUsed: 89.99,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		// CRITICAL range: 90-99%
		{
			name:           "90% is CRITICAL (boundary)",
			percentageUsed: 90,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		{
			name:           "95% is CRITICAL",
			percentageUsed: 95,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		{
			name:           "99% is CRITICAL",
			percentageUsed: 99,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		{
			name:           "99.99% is CRITICAL",
			percentageUsed: 99.99,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		// EXCEEDED range: 100%+
		{
			name:           "100% is EXCEEDED (boundary)",
			percentageUsed: 100,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		{
			name:           "105% is EXCEEDED",
			percentageUsed: 105,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		{
			name:           "150% is EXCEEDED",
			percentageUsed: 150,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		{
			name:           "200% is EXCEEDED",
			percentageUsed: 200,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		// Edge cases
		{
			name:           "negative percentage treated as OK",
			percentageUsed: -10,
			expected:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateBudgetHealthFromPercentage(tc.percentageUsed)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCalculateBudgetHealth tests health calculation from a budget struct.
func TestCalculateBudgetHealth(t *testing.T) {
	tests := []struct {
		name     string
		budget   *pbc.Budget
		expected pbc.BudgetHealthStatus
	}{
		{
			name:     "nil budget returns UNSPECIFIED",
			budget:   nil,
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name:     "budget with nil status returns UNSPECIFIED",
			budget:   &pbc.Budget{Id: "test-1"},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name: "budget with existing health status uses that value",
			budget: &pbc.Budget{
				Id: "test-2",
				Status: &pbc.BudgetStatus{
					Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
					PercentageUsed: 50, // Would normally be OK, but health is set
				},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name: "budget with UNSPECIFIED health calculates from percentage (OK)",
			budget: &pbc.Budget{
				Id: "test-3",
				Status: &pbc.BudgetStatus{
					Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
					PercentageUsed: 50,
				},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name: "budget with UNSPECIFIED health calculates from percentage (WARNING)",
			budget: &pbc.Budget{
				Id: "test-4",
				Status: &pbc.BudgetStatus{
					Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
					PercentageUsed: 85,
				},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name: "budget with UNSPECIFIED health calculates from percentage (CRITICAL)",
			budget: &pbc.Budget{
				Id: "test-5",
				Status: &pbc.BudgetStatus{
					Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
					PercentageUsed: 95,
				},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		{
			name: "budget with UNSPECIFIED health calculates from percentage (EXCEEDED)",
			budget: &pbc.Budget{
				Id: "test-6",
				Status: &pbc.BudgetStatus{
					Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
					PercentageUsed: 110,
				},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		{
			name: "budget with zero limit edge case",
			budget: &pbc.Budget{
				Id:     "test-7",
				Amount: &pbc.BudgetAmount{Limit: 0, Currency: "USD"},
				Status: &pbc.BudgetStatus{
					Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
					PercentageUsed: 0,
				},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateBudgetHealth(tc.budget)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestAggregateHealth tests worst-case health aggregation (FR-008).
func TestAggregateHealth(t *testing.T) {
	tests := []struct {
		name     string
		budgets  []*pbc.Budget
		expected pbc.BudgetHealthStatus
	}{
		{
			name:     "empty slice returns UNSPECIFIED",
			budgets:  []*pbc.Budget{},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name:     "nil slice returns UNSPECIFIED",
			budgets:  nil,
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name: "single OK budget",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name: "all OK budgets",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name: "worst is WARNING",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name: "worst is CRITICAL",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		{
			name: "worst is EXCEEDED",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL}},
				{Id: "4", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		{
			name: "single EXCEEDED among many OK",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED}},
				{Id: "4", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "5", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
		{
			name: "handles nil status in list",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: nil}, // nil status
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name: "all nil status returns UNSPECIFIED",
			budgets: []*pbc.Budget{
				{Id: "1", Status: nil},
				{Id: "2", Status: nil},
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name: "UNSPECIFIED health with 0% spend calculates to OK",
			budgets: []*pbc.Budget{
				{Id: "1", Status: nil}, // Returns UNSPECIFIED
				{
					Id:     "2",
					Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED},
				}, // Calculates to OK (0%)
			},
			expected: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := AggregateHealth(tc.budgets)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCalculateBudgetHealthResults tests the batch health calculation function.
func TestCalculateBudgetHealthResults(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		budgets       []*pbc.Budget
		expectedCount int
		checkFunc     func(t *testing.T, results []BudgetHealthResult)
	}{
		{
			name:          "empty budgets returns empty results",
			budgets:       []*pbc.Budget{},
			expectedCount: 0,
		},
		{
			name: "nil budgets in slice are skipped",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				nil,
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
			},
			expectedCount: 2,
		},
		{
			name: "populates all fields correctly",
			budgets: []*pbc.Budget{
				{
					Id:     "budget-1",
					Name:   "Production Budget",
					Source: "aws-budgets",
					Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "USD"},
					Status: &pbc.BudgetStatus{
						Health:               pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
						PercentageUsed:       85.5,
						PercentageForecasted: 95.0,
						CurrentSpend:         855,
					},
				},
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, results []BudgetHealthResult) {
				require.Len(t, results, 1)
				r := results[0]
				assert.Equal(t, "budget-1", r.BudgetID)
				assert.Equal(t, "Production Budget", r.BudgetName)
				assert.Equal(t, "aws-budgets", r.Provider)
				assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING, r.Health)
				assert.Equal(t, 85.5, r.Utilization)
				assert.Equal(t, 95.0, r.Forecasted)
				assert.Equal(t, "USD", r.Currency)
				assert.Equal(t, 1000.0, r.Limit)
				assert.Equal(t, 855.0, r.CurrentSpend)
			},
		},
		{
			name: "handles missing amount gracefully",
			budgets: []*pbc.Budget{
				{
					Id:     "budget-2",
					Name:   "No Amount Budget",
					Source: "kubecost",
					Amount: nil,
					Status: &pbc.BudgetStatus{
						Health:         pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
						PercentageUsed: 50,
						CurrentSpend:   500,
					},
				},
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, results []BudgetHealthResult) {
				require.Len(t, results, 1)
				r := results[0]
				assert.Equal(t, "", r.Currency)
				assert.Equal(t, 0.0, r.Limit)
			},
		},
		{
			name: "handles missing status gracefully",
			budgets: []*pbc.Budget{
				{
					Id:     "budget-3",
					Name:   "No Status Budget",
					Source: "gcp-billing",
					Amount: &pbc.BudgetAmount{Limit: 2000, Currency: "EUR"},
					Status: nil,
				},
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, results []BudgetHealthResult) {
				require.Len(t, results, 1)
				r := results[0]
				assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED, r.Health)
				assert.Equal(t, 0.0, r.Utilization)
				assert.Equal(t, 0.0, r.Forecasted)
				assert.Equal(t, 0.0, r.CurrentSpend)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := CalculateBudgetHealthResults(ctx, tc.budgets)
			assert.Len(t, results, tc.expectedCount)
			if tc.checkFunc != nil {
				tc.checkFunc(t, results)
			}
		})
	}
}

// TestHealthThresholdConstants verifies the threshold constants match spec.
func TestHealthThresholdConstants(t *testing.T) {
	assert.Equal(t, 80.0, HealthThresholdWarning)
	assert.Equal(t, 90.0, HealthThresholdCritical)
	assert.Equal(t, 100.0, HealthThresholdExceeded)
}

// =============================================================================
// Benchmarks
// =============================================================================

// generateBudgets creates n budgets with random health statuses for benchmarking.
func generateBudgets(n int) []*pbc.Budget {
	budgets := make([]*pbc.Budget, n)
	statuses := []pbc.BudgetHealthStatus{
		pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
	}
	for i := range n {
		budgets[i] = &pbc.Budget{
			Id:     "budget-" + string(rune('0'+i%10)),
			Name:   "Budget " + string(rune('0'+i%10)),
			Source: "aws-budgets",
			Amount: &pbc.BudgetAmount{
				Limit:    1000.0,
				Currency: "USD",
			},
			Status: &pbc.BudgetStatus{
				Health:         statuses[i%4],
				PercentageUsed: float64(20 + (i%4)*25), // 20, 45, 70, 95
				CurrentSpend:   float64(200 + (i%4)*250),
			},
		}
	}
	return budgets
}

// BenchmarkCalculateBudgetHealthFromPercentage benchmarks the core health calculation.
func BenchmarkCalculateBudgetHealthFromPercentage(b *testing.B) {
	percentages := []float64{0, 50, 79.99, 80, 85, 89.99, 90, 95, 99.99, 100, 150}

	b.ResetTimer()
	for b.Loop() {
		for _, p := range percentages {
			_ = CalculateBudgetHealthFromPercentage(p)
		}
	}
}

// BenchmarkCalculateBudgetHealth benchmarks health calculation for a single budget.
func BenchmarkCalculateBudgetHealth(b *testing.B) {
	budget := &pbc.Budget{
		Id:     "bench-budget",
		Name:   "Benchmark Budget",
		Source: "aws-budgets",
		Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "USD"},
		Status: &pbc.BudgetStatus{
			PercentageUsed: 85,
			CurrentSpend:   850,
		},
	}

	b.ResetTimer()
	for b.Loop() {
		_ = CalculateBudgetHealth(budget)
	}
}

// BenchmarkAggregateHealth1000 benchmarks aggregation for 1000 budgets.
// Target: < 100ms per spec (T079).
func BenchmarkAggregateHealth1000(b *testing.B) {
	budgets := generateBudgets(1000)

	b.ResetTimer()
	for b.Loop() {
		_ = AggregateHealth(budgets)
	}
}

// BenchmarkCalculateBudgetHealthResults1000 benchmarks batch health calculation for 1000 budgets.
// Target: < 100ms per spec (T079).
func BenchmarkCalculateBudgetHealthResults1000(b *testing.B) {
	ctx := context.Background()
	budgets := generateBudgets(1000)

	b.ResetTimer()
	for b.Loop() {
		_ = CalculateBudgetHealthResults(ctx, budgets)
	}
}
