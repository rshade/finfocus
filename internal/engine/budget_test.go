package engine

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestFilterBudgets(t *testing.T) {
	tests := []struct {
		name     string
		budgets  []*pbc.Budget
		filter   *pbc.BudgetFilter
		expected []string // Expected Budget IDs
	}{
		{
			name: "nil filter returns all budgets unchanged",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws"},
				{Id: "2", Source: "gcp"},
				{Id: "3", Source: "azure"},
			},
			filter:   nil,
			expected: []string{"1", "2", "3"},
		},
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
			result := CalculateBudgetSummary(context.Background(), tc.budgets)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsValidCurrency(t *testing.T) {
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
		{"usd", false}, // ASCII check requires uppercase A-Z
		{"123", false},
	}

	for _, tc := range tests {
		t.Run(tc.currency, func(t *testing.T) {
			assert.Equal(t, tc.valid, isValidCurrency(tc.currency))
		})
	}
}

// TestValidateCurrencyExported tests the exported ValidateCurrency function (FR-003).
func TestValidateCurrencyExported(t *testing.T) {
	tests := []struct {
		name        string
		currency    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid USD",
			currency: "USD",
			wantErr:  false,
		},
		{
			name:     "valid EUR",
			currency: "EUR",
			wantErr:  false,
		},
		{
			name:     "valid GBP",
			currency: "GBP",
			wantErr:  false,
		},
		{
			name:     "valid JPY",
			currency: "JPY",
			wantErr:  false,
		},
		{
			name:        "empty string",
			currency:    "",
			wantErr:     true,
			errContains: "currency code is required",
		},
		{
			name:        "too short (2 chars)",
			currency:    "US",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name:        "too long (4 chars)",
			currency:    "USDD",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name:        "lowercase",
			currency:    "usd",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name:        "mixed case",
			currency:    "Usd",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name:        "numeric",
			currency:    "123",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name:        "special characters",
			currency:    "US$",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name:        "whitespace",
			currency:    "US ",
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCurrency(tc.currency)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidCurrency), "expected ErrInvalidCurrency")
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateBudgetCurrency tests the ValidateBudgetCurrency function.
func TestValidateBudgetCurrency(t *testing.T) {
	tests := []struct {
		name        string
		budget      *pbc.Budget
		wantErr     bool
		errContains string
	}{
		{
			name:    "nil budget",
			budget:  nil,
			wantErr: false,
		},
		{
			name:    "budget with nil amount",
			budget:  &pbc.Budget{Id: "test-1"},
			wantErr: false,
		},
		{
			name: "budget with empty currency (allowed)",
			budget: &pbc.Budget{
				Id:     "test-2",
				Amount: &pbc.BudgetAmount{Limit: 1000, Currency: ""},
			},
			wantErr: false,
		},
		{
			name: "budget with valid USD currency",
			budget: &pbc.Budget{
				Id:     "test-3",
				Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "USD"},
			},
			wantErr: false,
		},
		{
			name: "budget with valid EUR currency",
			budget: &pbc.Budget{
				Id:     "test-4",
				Amount: &pbc.BudgetAmount{Limit: 2000, Currency: "EUR"},
			},
			wantErr: false,
		},
		{
			name: "budget with invalid currency (lowercase)",
			budget: &pbc.Budget{
				Id:     "test-5",
				Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "usd"},
			},
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name: "budget with invalid currency (too short)",
			budget: &pbc.Budget{
				Id:     "test-6",
				Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "US"},
			},
			wantErr:     true,
			errContains: "must be 3 uppercase letters",
		},
		{
			name: "budget with invalid currency (contains budget ID in error)",
			budget: &pbc.Budget{
				Id:     "my-budget-123",
				Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "invalid"},
			},
			wantErr:     true,
			errContains: "my-budget-123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBudgetCurrency(tc.budget)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFilterBudgetsByProvider tests provider filtering with BudgetFilterOptions (FR-002, FR-009).
func TestFilterBudgetsByProvider(t *testing.T) {
	tests := []struct {
		name      string
		budgets   []*pbc.Budget
		providers []string
		expected  []string // Expected Budget IDs
	}{
		{
			name: "empty providers list returns all budgets",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws-budgets"},
				{Id: "2", Source: "kubecost"},
				{Id: "3", Source: "gcp-billing"},
			},
			providers: []string{},
			expected:  []string{"1", "2", "3"},
		},
		{
			name: "nil providers returns all budgets",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws-budgets"},
				{Id: "2", Source: "kubecost"},
			},
			providers: nil,
			expected:  []string{"1", "2"},
		},
		{
			name: "single provider filter (exact match)",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws-budgets"},
				{Id: "2", Source: "kubecost"},
				{Id: "3", Source: "gcp-billing"},
			},
			providers: []string{"aws-budgets"},
			expected:  []string{"1"},
		},
		{
			name: "single provider filter (case-insensitive)",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "AWS-Budgets"},
				{Id: "2", Source: "kubecost"},
			},
			providers: []string{"aws-budgets"},
			expected:  []string{"1"},
		},
		{
			name: "multiple providers (OR logic)",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws-budgets"},
				{Id: "2", Source: "kubecost"},
				{Id: "3", Source: "gcp-billing"},
				{Id: "4", Source: "azure-cost"},
			},
			providers: []string{"aws-budgets", "gcp-billing"},
			expected:  []string{"1", "3"},
		},
		{
			name: "no matches returns empty slice",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws-budgets"},
				{Id: "2", Source: "kubecost"},
			},
			providers: []string{"non-existent-provider"},
			expected:  []string{},
		},
		{
			name:      "empty budgets slice",
			budgets:   []*pbc.Budget{},
			providers: []string{"aws-budgets"},
			expected:  []string{},
		},
		{
			name: "mixed case providers in filter",
			budgets: []*pbc.Budget{
				{Id: "1", Source: "aws-budgets"},
				{Id: "2", Source: "KUBECOST"},
				{Id: "3", Source: "Gcp-Billing"},
			},
			providers: []string{"AWS-BUDGETS", "kubecost", "GCP-BILLING"},
			expected:  []string{"1", "2", "3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FilterBudgetsByProvider(context.Background(), tc.budgets, tc.providers)
			var resultIDs []string
			for _, b := range result {
				resultIDs = append(resultIDs, b.GetId())
			}
			assert.ElementsMatch(t, tc.expected, resultIDs)
		})
	}
}

// TestMatchesProvider tests the MatchesProvider function.
func TestMatchesProvider(t *testing.T) {
	tests := []struct {
		name      string
		budget    *pbc.Budget
		providers []string
		want      bool
	}{
		{
			name:      "nil budget",
			budget:    nil,
			providers: []string{"aws"},
			want:      false,
		},
		{
			name:      "empty providers matches any",
			budget:    &pbc.Budget{Id: "1", Source: "aws-budgets"},
			providers: []string{},
			want:      true,
		},
		{
			name:      "exact match",
			budget:    &pbc.Budget{Id: "1", Source: "aws-budgets"},
			providers: []string{"aws-budgets"},
			want:      true,
		},
		{
			name:      "case-insensitive match",
			budget:    &pbc.Budget{Id: "1", Source: "AWS-Budgets"},
			providers: []string{"aws-budgets"},
			want:      true,
		},
		{
			name:      "no match",
			budget:    &pbc.Budget{Id: "1", Source: "aws-budgets"},
			providers: []string{"gcp-billing"},
			want:      false,
		},
		{
			name:      "match in multiple providers",
			budget:    &pbc.Budget{Id: "1", Source: "kubecost"},
			providers: []string{"aws-budgets", "kubecost", "gcp-billing"},
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchesProvider(tc.budget, tc.providers)
			assert.Equal(t, tc.want, got)
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

// generateBudgetsWithProviders creates n budgets with rotating providers for benchmarking.
func generateBudgetsWithProviders(n int) []*pbc.Budget {
	providers := []string{"aws-budgets", "kubecost", "gcp-billing", "azure-cost"}
	budgets := make([]*pbc.Budget, n)
	for i := 0; i < n; i++ {
		idSuffix := strconv.Itoa(i)
		budgets[i] = &pbc.Budget{
			Id:     "budget-" + idSuffix,
			Name:   "Budget " + idSuffix,
			Source: providers[i%len(providers)],
			Amount: &pbc.BudgetAmount{
				Limit:    1000.0,
				Currency: "USD",
			},
			Status: &pbc.BudgetStatus{
				PercentageUsed: float64(20 + (i % 80)),
				CurrentSpend:   float64(200 + (i % 800)),
			},
		}
	}
	return budgets
}

// BenchmarkFilterBudgetsByProvider1000 benchmarks provider filtering for 1000 budgets.
// Target: < 500ms per spec (T079).
func BenchmarkFilterBudgetsByProvider1000(b *testing.B) {
	ctx := context.Background()
	budgets := generateBudgetsWithProviders(1000)
	providers := []string{"aws-budgets", "gcp-billing"}

	b.ResetTimer()
	for b.Loop() {
		_ = FilterBudgetsByProvider(ctx, budgets, providers)
	}
}

// BenchmarkMatchesProvider benchmarks single provider matching.
func BenchmarkMatchesProvider(b *testing.B) {
	budget := &pbc.Budget{Id: "1", Source: "aws-budgets"}
	providers := []string{"aws-budgets", "gcp-billing", "kubecost"}

	b.ResetTimer()
	for b.Loop() {
		_ = MatchesProvider(budget, providers)
	}
}
