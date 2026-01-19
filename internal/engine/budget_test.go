package engine

import (
	"testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/stretchr/testify/assert"
)

func TestFilterBudgets(t *testing.T) {
	tests := []struct {
		name     string
		budgets  []*pbc.Budget
		filter   *pbc.BudgetFilter
		expected []string // Expected Budget IDs
	}{
		{
			name: "empty filter returns all",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws"},
				{Id: "2", Source: "gcp"},
			},
			filter:   &pbc.BudgetFilter{},
			expected: []string{"1", "2"},
		},
		{
			name: "filter by provider (exact match)",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws"},
				{Id: "2", Source: "gcp"},
			},
			filter:   &pbc.BudgetFilter{Providers: []string{"aws"}},
			expected: []string{"1"},
		},
		{
			name: "filter by provider (case insensitive)",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws"},
				{Id: "2", Source: "gcp"},
			},
			filter:   &pbc.BudgetFilter{Providers: []string{"AWS"}},
			expected: []string{"1"},
		},
		{
			name: "filter by multiple providers",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws"},
				{Id: "2", Source: "gcp"},
				{Id: "3", Source: "azure"},
			},
			filter:   &pbc.BudgetFilter{Providers: []string{"aws", "azure"}},
			expected: []string{"1", "3"},
		},
		{
			name: "filter by region",
			budgets: []*pbc.Budget{
				{Id: "1", Metadata: map[string]string{"region": "us-east-1"}},
				{Id: "2", Metadata: map[string]string{"region": "us-west-1"}},
			},
			filter:   &pbc.BudgetFilter{Regions: []string{"us-east-1"}},
			expected: []string{"1"},
		},
		{
			name: "filter by resource type",
			budgets: []*pbc.Budget{
				{Id: "1", Metadata: map[string]string{"resourceType": "ec2"}},
				{Id: "2", Metadata: map[string]string{"resourceType": "s3"}},
			},
			filter:   &pbc.BudgetFilter{ResourceTypes: []string{"ec2"}},
			expected: []string{"1"},
		},
		{
			name: "filter by tags (subset match)",
			budgets: []*pbc.Budget{
				{Id: "1", Metadata: map[string]string{"tag:env": "prod", "tag:team": "platform"}},
				{Id: "2", Metadata: map[string]string{"tag:env": "dev"}},
				{Id: "3", Metadata: map[string]string{"tag:env": "prod"}},
			},
			filter:   &pbc.BudgetFilter{Tags: map[string]string{"env": "prod", "team": "platform"}},
			expected: []string{"1"},
		},
		{
			name: "combined filter (provider AND region)",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws", Metadata: map[string]string{"region": "us-east-1"}},
				{Id: "2", Source: "aws", Metadata: map[string]string{"region": "us-west-1"}},
				{Id: "3", Source: "gcp", Metadata: map[string]string{"region": "us-east-1"}},
			},
			filter:   &pbc.BudgetFilter{Providers: []string{"aws"}, Regions: []string{"us-east-1"}},
			expected: []string{"1"},
		},
		{
			name: "no match returns empty",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws"},
			},
			filter:   &pbc.BudgetFilter{Providers: []string{"gcp"}},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FilterBudgets(tc.budgets, tc.filter)
			var resultIDs []string
			for _, b := range result {
				resultIDs = append(resultIDs, b.GetId())
			}
			assert.ElementsMatch(t, tc.expected, resultIDs)
		})
	}
}

func TestCalculateBudgetSummary(t *testing.T) {
	tests := []struct {
		name     string
		budgets  []*pbc.Budget
		expected *pbc.BudgetSummary
	}{
		{
			name:     "empty budgets returns zero summary",
			budgets:  []*pbc.Budget{},
			expected: &pbc.BudgetSummary{},
		},
		{
			name: "single budget OK",
			budgets: []*pbc.Budget{
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
			},
			expected: &pbc.BudgetSummary{TotalBudgets: 1, BudgetsOk: 1},
		},
		{
			name: "mixed health statuses",
			budgets: []*pbc.Budget{
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING}},
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL}},
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED}},
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}},
			},
			expected: &pbc.BudgetSummary{
				TotalBudgets:    5,
				BudgetsOk:       2,
				BudgetsWarning:  1,
				BudgetsCritical: 1,
				BudgetsExceeded: 1,
			},
		},
		{
			name: "missing status counts in total only",
			budgets: []*pbc.Budget{
				{Status: nil},
				{Status: &pbc.BudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED}},
			},
			expected: &pbc.BudgetSummary{
				TotalBudgets: 2,
				// All other fields 0
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateBudgetSummary(tc.budgets)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateCurrency(t *testing.T) {
	tests := []struct {
		currency string
		valid    bool
	}{
		{"USD", true},
		{"EUR", true},
		{"JPY", true},
		{"", false},
		{"US", false},
		{"USDD", false},
		{"usd", false}, // Regex ^[A-Z]{3}$ implies uppercase
		{"123", false},
	}

	for _, tc := range tests {
		t.Run(tc.currency, func(t *testing.T) {
			assert.Equal(t, tc.valid, validateCurrency(tc.currency))
		})
	}
}
