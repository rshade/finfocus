package engine

import (
	"context"
	"testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateBudgetSummary verifies basic summary aggregation (FR-004).
func TestCalculateBudgetSummary_US3(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		budgets  []*pbc.Budget
		expected *pbc.BudgetSummary
	}{
		{
			name:    "empty input",
			budgets: []*pbc.Budget{},
			expected: &pbc.BudgetSummary{
				TotalBudgets: 0,
			},
		},
		{
			name: "mixed health statuses",
			budgets: []*pbc.Budget{
				{Id: "1", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "2", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Id: "3", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
				{Id: "4", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL}},
				{Id: "5", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED}},
				{Id: "6", Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED}},
				{
					Id:     "7",
					Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED},
				}, // Should be ignored in counts
			},
			expected: &pbc.BudgetSummary{
				TotalBudgets:    7,
				BudgetsOk:       2,
				BudgetsWarning:  1,
				BudgetsCritical: 1,
				BudgetsExceeded: 2,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateBudgetSummary(ctx, tc.budgets)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCalculateExtendedSummary verifies detailed breakdown calculation.
func TestCalculateExtendedSummary(t *testing.T) {
	ctx := context.Background()

	budgets := []*pbc.Budget{
		// AWS: 1 OK, 1 CRITICAL
		{
			Id:     "aws-1",
			Source: "aws-budgets",
			Amount: &pbc.BudgetAmount{Currency: "USD"},
			Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
		},
		{
			Id:     "aws-2",
			Source: "aws-budgets",
			Amount: &pbc.BudgetAmount{Currency: "USD"},
			Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL},
		},
		// GCP: 1 WARNING
		{
			Id:     "gcp-1",
			Source: "gcp-billing",
			Amount: &pbc.BudgetAmount{Currency: "EUR"},
			Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING},
		},
		// Azure: 1 EXCEEDED
		{
			Id:     "azure-1",
			Source: "azure-cost",
			Amount: &pbc.BudgetAmount{Currency: "USD"},
			Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED},
		},
	}

	result := CalculateExtendedSummary(ctx, budgets)
	require.NotNil(t, result)

	// Verify basic summary embedding
	assert.Equal(t, int32(4), result.GetTotalBudgets())
	assert.Equal(t, int32(1), result.GetBudgetsOk())
	assert.Equal(t, int32(1), result.GetBudgetsWarning())
	assert.Equal(t, int32(1), result.GetBudgetsCritical())
	assert.Equal(t, int32(1), result.GetBudgetsExceeded())

	// Verify overall health (worst case)
	assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, result.OverallHealth)

	// Verify ByProvider breakdown
	require.Contains(t, result.ByProvider, "aws-budgets")
	aws := result.ByProvider["aws-budgets"]
	assert.Equal(t, int32(2), aws.GetTotalBudgets())
	assert.Equal(t, int32(1), aws.GetBudgetsOk())
	assert.Equal(t, int32(1), aws.GetBudgetsCritical())

	require.Contains(t, result.ByProvider, "gcp-billing")
	gcp := result.ByProvider["gcp-billing"]
	assert.Equal(t, int32(1), gcp.GetTotalBudgets())
	assert.Equal(t, int32(1), gcp.GetBudgetsWarning())

	// Verify ByCurrency breakdown
	require.Contains(t, result.ByCurrency, "USD")
	usd := result.ByCurrency["USD"]
	assert.Equal(t, int32(3), usd.GetTotalBudgets()) // aws-1, aws-2, azure-1
	assert.Equal(t, int32(1), usd.GetBudgetsOk())
	assert.Equal(t, int32(1), usd.GetBudgetsCritical())
	assert.Equal(t, int32(1), usd.GetBudgetsExceeded())

	require.Contains(t, result.ByCurrency, "EUR")
	eur := result.ByCurrency["EUR"]
	assert.Equal(t, int32(1), eur.GetTotalBudgets())
	assert.Equal(t, int32(1), eur.GetBudgetsWarning())

	// Verify CriticalBudgets list
	// Should contain CRITICAL and EXCEEDED budgets
	assert.Len(t, result.CriticalBudgets, 2)
	assert.Contains(t, result.CriticalBudgets, "aws-2")
	assert.Contains(t, result.CriticalBudgets, "azure-1")
	assert.NotContains(t, result.CriticalBudgets, "aws-1")
	assert.NotContains(t, result.CriticalBudgets, "gcp-1")
}
