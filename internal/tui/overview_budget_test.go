package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
)

// testBudget creates a *pbc.Budget with the given health parameters for testing.
func testBudget(
	id, name, currency string,
	limit, spend, utilization float64,
	health pbc.BudgetHealthStatus,
) *pbc.Budget {
	return &pbc.Budget{
		Id:   id,
		Name: name,
		Amount: &pbc.BudgetAmount{
			Limit:    limit,
			Currency: currency,
		},
		Status: &pbc.BudgetStatus{
			CurrentSpend:   spend,
			PercentageUsed: utilization,
			Health:         health,
		},
	}
}

// testBudgetWithThresholds creates a *pbc.Budget with triggered threshold alerts.
func testBudgetWithThresholds(
	id, name, currency string,
	limit, spend, utilization float64,
	health pbc.BudgetHealthStatus,
	thresholds []*pbc.BudgetThreshold,
) *pbc.Budget {
	b := testBudget(id, name, currency, limit, spend, utilization, health)
	b.Thresholds = thresholds
	return b
}

// testBudgetWithForecast creates a *pbc.Budget with forecasted spend.
func testBudgetWithForecast(
	id, name, currency string,
	limit, spend, forecasted, utilization float64,
	health pbc.BudgetHealthStatus,
) *pbc.Budget {
	b := testBudget(id, name, currency, limit, spend, utilization, health)
	b.Status.ForecastedSpend = forecasted
	return b
}

// testBudgetResult creates an *engine.BudgetResult with the given budgets
// and overall health status for testing.
func testBudgetResult(budgets []*pbc.Budget, overallHealth pbc.BudgetHealthStatus) *engine.BudgetResult {
	byCurrency := make(map[string]*pbc.BudgetSummary)
	for _, b := range budgets {
		if b.GetAmount() != nil {
			currency := b.GetAmount().GetCurrency()
			if currency != "" {
				if _, exists := byCurrency[currency]; !exists {
					byCurrency[currency] = &pbc.BudgetSummary{}
				}
				byCurrency[currency].TotalBudgets++
			}
		}
	}
	return &engine.BudgetResult{
		Budgets: budgets,
		Summary: &engine.ExtendedBudgetSummary{
			BudgetSummary: &pbc.BudgetSummary{
				TotalBudgets: int32(len(budgets)),
			},
			OverallHealth:   overallHealth,
			ByCurrency:      byCurrency,
			CriticalBudgets: []string{},
			ByProvider:      make(map[string]*pbc.BudgetSummary),
		},
	}
}

// testModelWithBudget creates an OverviewModel with budget data loaded.
func testModelWithBudget(result *engine.BudgetResult) OverviewModel {
	return OverviewModel{
		budgetLoaded: true,
		budgetResult: result,
		width:        defaultWidth,
		height:       defaultHeight,
	}
}

// TestRenderBudgetFooter verifies budget footer rendering across all health
// states, edge cases, and mixed-currency scenarios.
func TestRenderBudgetFooter(t *testing.T) {
	tests := []struct {
		name           string
		model          OverviewModel
		expectEmpty    bool
		expectContains []string
		expectMissing  []string
	}{
		{
			name:        "not loaded returns empty",
			model:       OverviewModel{budgetLoaded: false},
			expectEmpty: true,
		},
		{
			name:        "nil result returns empty",
			model:       OverviewModel{budgetLoaded: true, budgetResult: nil},
			expectEmpty: true,
		},
		{
			name: "empty budgets returns empty",
			model: OverviewModel{
				budgetLoaded: true,
				budgetResult: &engine.BudgetResult{
					Budgets: []*pbc.Budget{},
				},
			},
			expectEmpty: true,
		},
		{
			name: "single budget OK at 45%",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("b1", "Infra", "USD", 10000, 4500, 45.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			)),
			expectContains: []string{"OK", "$4,500.00", "$10,000.00", "45"},
		},
		{
			name: "single budget WARNING at 85%",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("b1", "Infra", "USD", 10000, 8500, 85.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
			)),
			expectContains: []string{"WARNING", "$8,500.00", "$10,000.00", "85"},
		},
		{
			name: "single budget CRITICAL at 95%",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("b1", "Infra", "USD", 10000, 9500, 95.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
			)),
			expectContains: []string{"CRITICAL", "$9,500.00", "$10,000.00", "95"},
		},
		{
			name: "single budget EXCEEDED at 105%",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("b1", "Infra", "USD", 10000, 10500, 105.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
			)),
			expectContains: []string{"EXCEEDED", "$10,500.00", "$10,000.00", "105"},
		},
		{
			name: "mixed currency shows badge only no dollar amounts",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("b1", "USD Budget", "USD", 10000, 8500, 85.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING),
					testBudget("b2", "EUR Budget", "EUR", 5000, 4000, 80.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
			)),
			expectContains: []string{"WARNING"},
			expectMissing:  []string{"$"},
		},
		{
			name: "single budget direct spend and limit displayed",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("b1", "Monthly Cloud", "USD", 5000, 2250, 45.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			)),
			expectContains: []string{"$2,250.00", "$5,000.00"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderBudgetFooter(tt.model)
			if tt.expectEmpty {
				assert.Empty(t, result)
				return
			}
			assert.NotEmpty(t, result)
			for _, s := range tt.expectContains {
				assert.Contains(t, result, s, "expected footer to contain %q", s)
			}
			for _, s := range tt.expectMissing {
				assert.NotContains(t, result, s, "expected footer to NOT contain %q", s)
			}
		})
	}
}

// TestRenderDetailBudgetStatus verifies the detail view budget section rendering
// including per-budget breakdown, triggered alerts, and edge cases.
func TestRenderDetailBudgetStatus(t *testing.T) {
	tests := []struct {
		name           string
		model          OverviewModel
		expectEmpty    bool
		expectContains []string
		expectMissing  []string
	}{
		{
			name:        "not loaded returns empty",
			model:       OverviewModel{budgetLoaded: false},
			expectEmpty: true,
		},
		{
			name:        "nil result returns empty",
			model:       OverviewModel{budgetLoaded: true, budgetResult: nil},
			expectEmpty: true,
		},
		{
			name: "empty budgets returns empty",
			model: OverviewModel{
				budgetLoaded: true,
				budgetResult: &engine.BudgetResult{Budgets: []*pbc.Budget{}},
			},
			expectEmpty: true,
		},
		{
			name: "multiple budgets one OK one WARNING",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudgetWithForecast("b1", "Dev Infra", "USD", 10000, 4500, 5200, 45.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK),
					testBudgetWithForecast("b2", "Prod Infra", "USD", 20000, 17000, 22000, 85.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
			)),
			expectContains: []string{
				"BUDGET STATUS",
				"Dev Infra", "OK", "$4,500.00", "$10,000.00", "45.0%",
				"Prod Infra", "WARNING", "$17,000.00", "$20,000.00", "85.0%",
				"$5,200.00",  // Dev forecasted
				"$22,000.00", // Prod forecasted
			},
		},
		{
			name: "triggered threshold alerts displayed",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudgetWithThresholds("b1", "Alerting Budget", "USD", 10000, 8500, 85.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
						[]*pbc.BudgetThreshold{
							{
								Percentage: 80,
								Type:       pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL,
								Triggered:  true,
							},
							{
								Percentage: 100,
								Type:       pbc.ThresholdType_THRESHOLD_TYPE_FORECASTED,
								Triggered:  true,
							},
							{
								Percentage: 90,
								Type:       pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL,
								Triggered:  false, // not triggered
							},
						},
					),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
			)),
			expectContains: []string{
				"ALERT:", "80%", "THRESHOLD_TYPE_ACTUAL",
				"ALERT:", "100%", "THRESHOLD_TYPE_FORECASTED",
			},
			expectMissing: []string{
				"90%", // not-triggered threshold should not appear
			},
		},
		{
			name: "budget with no name falls back to ID",
			model: testModelWithBudget(testBudgetResult(
				[]*pbc.Budget{
					testBudget("budget-123", "", "USD", 5000, 2000, 40.0,
						pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK),
				},
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			)),
			expectContains: []string{"budget-123", "OK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderDetailBudgetStatus(tt.model)
			if tt.expectEmpty {
				assert.Empty(t, result)
				return
			}
			assert.NotEmpty(t, result)
			for _, s := range tt.expectContains {
				assert.Contains(t, result, s, "expected detail to contain %q", s)
			}
			for _, s := range tt.expectMissing {
				assert.NotContains(t, result, s, "expected detail to NOT contain %q", s)
			}
		})
	}
}
