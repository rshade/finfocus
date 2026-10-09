package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
)

func TestBudgetDisplaySafeSections(t *testing.T) {
	t.Parallel()
	assert.Empty(t, BuildBudgetDisplay(nil).FooterDisplay)
	assert.Empty(t, BuildBudgetDisplay(&engine.BudgetResult{}).Details)
	result := &engine.BudgetResult{
		Budgets: []*pbc.Budget{
			nil,
			{Id: "incomplete"},
			{
				Id:     "fallback",
				Amount: &pbc.BudgetAmount{Limit: 100.1234, Currency: "USD"},
				Status: &pbc.BudgetStatus{
					CurrentSpend:    75.6789,
					ForecastedSpend: 90.1234,
					PercentageUsed:  75.5856,
					Health:          pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				},
				Thresholds: []*pbc.BudgetThreshold{
					{Percentage: 75, Triggered: true},
					{Percentage: 100, Triggered: true},
					{Percentage: 110},
				},
			},
			{
				Name:   "disabled",
				Amount: &pbc.BudgetAmount{Limit: 0},
				Status: &pbc.BudgetStatus{CurrentSpend: 900},
			},
		},
	}
	display := BuildBudgetDisplay(result)
	assert.Equal(t, "Budget: OK $75.68 / $100.12 (76%)", display.FooterDisplay)
	require.Len(t, display.Details, 2)
	assert.Equal(t, "fallback", display.Details[0].Name)
	assert.Equal(t, "$90.12", display.Details[0].ForecastedDisplay)
	assert.Equal(t, "75.6%", display.Details[0].UtilizationDisplay)
	require.Len(t, display.Details[0].Thresholds, 2)
	assert.False(t, display.Details[0].Thresholds[0].Critical)
	assert.True(t, display.Details[0].Thresholds[1].Critical)
	assert.Empty(t, display.Details[1].ForecastedDisplay)
	result.Summary = &engine.ExtendedBudgetSummary{
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		ByCurrency:    map[string]*pbc.BudgetSummary{"USD": {}, "EUR": {}},
	}
	mixed := BuildBudgetDisplay(result)
	assert.Equal(t, "Budget: WARNING", mixed.FooterDisplay)
	assert.Empty(t, mixed.AmountsDisplay)
	result.Summary = nil
	result.Budgets = result.Budgets[3:]
	assert.Equal(t, "Budget: OK", BuildBudgetDisplay(result).FooterDisplay)
}
