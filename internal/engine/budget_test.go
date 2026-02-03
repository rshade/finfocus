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

// =============================================================================
// Tag-Based Budget Filtering Tests (T007-T011, T015-T018, T021-T022, T026, T030-T032)
// =============================================================================

// TestMatchesBudgetTagsWithGlob_ExactMatch tests exact tag value matching (T007, US1).
func TestMatchesBudgetTagsWithGlob_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		budget   *pbc.Budget
		tags     map[string]string
		expected bool
	}{
		{
			name: "exact match single tag",
			budget: &pbc.Budget{
				Id:       "1",
				Metadata: map[string]string{"namespace": "production"},
			},
			tags:     map[string]string{"namespace": "production"},
			expected: true,
		},
		{
			name: "exact match with tag: prefix in metadata",
			budget: &pbc.Budget{
				Id:       "2",
				Metadata: map[string]string{"tag:namespace": "production"},
			},
			tags:     map[string]string{"namespace": "production"},
			expected: true,
		},
		{
			name: "no match - different value",
			budget: &pbc.Budget{
				Id:       "3",
				Metadata: map[string]string{"namespace": "staging"},
			},
			tags:     map[string]string{"namespace": "production"},
			expected: false,
		},
		{
			name: "no match - case sensitive",
			budget: &pbc.Budget{
				Id:       "4",
				Metadata: map[string]string{"namespace": "Production"},
			},
			tags:     map[string]string{"namespace": "production"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBudgetTagsWithGlob(tc.budget, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMatchesBudgetTagsWithGlob_MissingKey tests budget missing required tag key (T008, US1).
func TestMatchesBudgetTagsWithGlob_MissingKey(t *testing.T) {
	tests := []struct {
		name     string
		budget   *pbc.Budget
		tags     map[string]string
		expected bool
	}{
		{
			name: "budget lacks required key",
			budget: &pbc.Budget{
				Id:       "1",
				Metadata: map[string]string{"other": "value"},
			},
			tags:     map[string]string{"namespace": "production"},
			expected: false,
		},
		{
			name: "budget has nil metadata",
			budget: &pbc.Budget{
				Id:       "2",
				Metadata: nil,
			},
			tags:     map[string]string{"namespace": "production"},
			expected: false,
		},
		{
			name:     "nil budget",
			budget:   nil,
			tags:     map[string]string{"namespace": "production"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBudgetTagsWithGlob(tc.budget, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMatchesBudgetTagsWithGlob_EmptyTags tests empty tags map returns all (T011, US1).
func TestMatchesBudgetTagsWithGlob_EmptyTags(t *testing.T) {
	budget := &pbc.Budget{
		Id:       "1",
		Metadata: map[string]string{"namespace": "production"},
	}

	tests := []struct {
		name     string
		tags     map[string]string
		expected bool
	}{
		{
			name:     "nil tags matches all",
			tags:     nil,
			expected: true,
		},
		{
			name:     "empty tags map matches all",
			tags:     map[string]string{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBudgetTagsWithGlob(budget, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMatchesBudgetTagsWithGlob_GlobPatterns tests glob pattern matching (T015-T018, US2).
func TestMatchesBudgetTagsWithGlob_GlobPatterns(t *testing.T) {
	tests := []struct {
		name     string
		budget   *pbc.Budget
		tags     map[string]string
		expected bool
	}{
		{
			name: "prefix glob pattern matches (prod-*)",
			budget: &pbc.Budget{
				Id:       "1",
				Metadata: map[string]string{"namespace": "prod-us"},
			},
			tags:     map[string]string{"namespace": "prod-*"},
			expected: true,
		},
		{
			name: "suffix glob pattern matches (*-production)",
			budget: &pbc.Budget{
				Id:       "2",
				Metadata: map[string]string{"env": "team-a-production"},
			},
			tags:     map[string]string{"env": "*-production"},
			expected: true,
		},
		{
			name: "both-ends glob pattern matches (*prod*)",
			budget: &pbc.Budget{
				Id:       "3",
				Metadata: map[string]string{"env": "my-production-env"},
			},
			tags:     map[string]string{"env": "*prod*"},
			expected: true,
		},
		{
			name: "glob pattern no match",
			budget: &pbc.Budget{
				Id:       "4",
				Metadata: map[string]string{"namespace": "staging"},
			},
			tags:     map[string]string{"namespace": "prod-*"},
			expected: false,
		},
		{
			name: "character class glob pattern [a-z]",
			budget: &pbc.Budget{
				Id:       "5",
				Metadata: map[string]string{"env": "a"},
			},
			tags:     map[string]string{"env": "[a-z]"},
			expected: true,
		},
		{
			name: "single character glob pattern (?)",
			budget: &pbc.Budget{
				Id:       "6",
				Metadata: map[string]string{"region": "us"},
			},
			tags:     map[string]string{"region": "u?"},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBudgetTagsWithGlob(tc.budget, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMatchesBudgetTagsWithGlob_MultipleTags tests AND logic for multiple tags (T021-T022, US3).
func TestMatchesBudgetTagsWithGlob_MultipleTags(t *testing.T) {
	tests := []struct {
		name     string
		budget   *pbc.Budget
		tags     map[string]string
		expected bool
	}{
		{
			name: "multiple tags all match (AND logic)",
			budget: &pbc.Budget{
				Id:       "1",
				Metadata: map[string]string{"namespace": "production", "cluster": "us-east-1"},
			},
			tags:     map[string]string{"namespace": "production", "cluster": "us-east-1"},
			expected: true,
		},
		{
			name: "multiple tags partial match fails (AND logic)",
			budget: &pbc.Budget{
				Id:       "2",
				Metadata: map[string]string{"namespace": "production", "cluster": "us-west-2"},
			},
			tags:     map[string]string{"namespace": "production", "cluster": "us-east-1"},
			expected: false,
		},
		{
			name: "multiple tags with glob patterns",
			budget: &pbc.Budget{
				Id:       "3",
				Metadata: map[string]string{"namespace": "prod-us", "cluster": "cluster-a-prod"},
			},
			tags:     map[string]string{"namespace": "prod-*", "cluster": "*-prod"},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBudgetTagsWithGlob(tc.budget, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMatchesBudgetTagsWithGlob_EdgeCases tests edge cases (T030-T032, Phase 7).
func TestMatchesBudgetTagsWithGlob_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		budget   *pbc.Budget
		tags     map[string]string
		expected bool
	}{
		{
			name: "tag key with special characters (kubernetes.io/name)",
			budget: &pbc.Budget{
				Id:       "1",
				Metadata: map[string]string{"kubernetes.io/name": "my-app"},
			},
			tags:     map[string]string{"kubernetes.io/name": "my-app"},
			expected: true,
		},
		{
			name: "empty tag value matches empty metadata value",
			budget: &pbc.Budget{
				Id:       "2",
				Metadata: map[string]string{"namespace": ""},
			},
			tags:     map[string]string{"namespace": ""},
			expected: true,
		},
		{
			name: "empty tag value does not match non-empty metadata",
			budget: &pbc.Budget{
				Id:       "3",
				Metadata: map[string]string{"namespace": "production"},
			},
			tags:     map[string]string{"namespace": ""},
			expected: false,
		},
		{
			name: "case-sensitive key matching",
			budget: &pbc.Budget{
				Id:       "4",
				Metadata: map[string]string{"Namespace": "production"},
			},
			tags:     map[string]string{"namespace": "production"},
			expected: false, // Different key case
		},
		{
			name: "invalid glob pattern treated as non-match",
			budget: &pbc.Budget{
				Id:       "5",
				Metadata: map[string]string{"namespace": "production"},
			},
			tags:     map[string]string{"namespace": "[invalid"},
			expected: false, // Invalid pattern syntax
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBudgetTagsWithGlob(tc.budget, tc.tags)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestFilterBudgetsByTags tests the full filtering function (T003, T009).
func TestFilterBudgetsByTags(t *testing.T) {
	budgets := []*pbc.Budget{
		{Id: "1", Metadata: map[string]string{"namespace": "production", "cluster": "us-east-1"}},
		{Id: "2", Metadata: map[string]string{"namespace": "production", "cluster": "us-west-2"}},
		{Id: "3", Metadata: map[string]string{"namespace": "staging", "cluster": "us-east-1"}},
		{Id: "4", Metadata: map[string]string{"namespace": "dev"}},
		{Id: "5", Metadata: nil},
	}

	tests := []struct {
		name        string
		tags        map[string]string
		expectedIDs []string
	}{
		{
			name:        "empty tags returns all",
			tags:        nil,
			expectedIDs: []string{"1", "2", "3", "4", "5"},
		},
		{
			name:        "filter by single exact tag",
			tags:        map[string]string{"namespace": "production"},
			expectedIDs: []string{"1", "2"},
		},
		{
			name:        "filter by multiple tags (AND)",
			tags:        map[string]string{"namespace": "production", "cluster": "us-east-1"},
			expectedIDs: []string{"1"},
		},
		{
			name:        "filter with glob pattern",
			tags:        map[string]string{"namespace": "prod*"},
			expectedIDs: []string{"1", "2"},
		},
		{
			name:        "no matches returns empty",
			tags:        map[string]string{"namespace": "nonexistent"},
			expectedIDs: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			result := FilterBudgetsByTags(ctx, budgets, tc.tags)

			var resultIDs []string
			for _, b := range result {
				resultIDs = append(resultIDs, b.GetId())
			}
			assert.ElementsMatch(t, tc.expectedIDs, resultIDs)
		})
	}
}

// TestFilterBudgetsByTags_CombinedWithProvider tests combined provider and tag filtering (T026, T028-T029, US4).
func TestFilterBudgetsByTags_CombinedWithProvider(t *testing.T) {
	budgets := []*pbc.Budget{
		{Id: "1", Source: "kubecost", Metadata: map[string]string{"namespace": "production"}},
		{Id: "2", Source: "kubecost", Metadata: map[string]string{"namespace": "staging"}},
		{Id: "3", Source: "aws-budgets", Metadata: map[string]string{"namespace": "production"}},
		{Id: "4", Source: "aws-budgets", Metadata: map[string]string{"namespace": "staging"}},
	}

	ctx := context.Background()

	// First filter by provider (OR logic)
	providerFiltered := FilterBudgetsByProvider(ctx, budgets, []string{"kubecost"})
	require.Len(t, providerFiltered, 2) // IDs 1 and 2

	// Then filter by tags (AND logic)
	tagFiltered := FilterBudgetsByTags(ctx, providerFiltered, map[string]string{"namespace": "production"})
	require.Len(t, tagFiltered, 1)
	assert.Equal(t, "1", tagFiltered[0].GetId())
}

// TestFilterBudgetsByTags_BackwardCompatibility tests existing provider filtering unchanged (T013, T042).
func TestFilterBudgetsByTags_BackwardCompatibility(t *testing.T) {
	budgets := []*pbc.Budget{
		{Id: "1", Source: "aws-budgets"},
		{Id: "2", Source: "kubecost"},
		{Id: "3", Source: "gcp-billing"},
	}

	ctx := context.Background()

	// Provider-only filtering should work as before
	result := FilterBudgetsByProvider(ctx, budgets, []string{"aws-budgets"})
	require.Len(t, result, 1)
	assert.Equal(t, "1", result[0].GetId())

	// Empty tags should not affect results
	result = FilterBudgetsByTags(ctx, budgets, nil)
	require.Len(t, result, 3)

	result = FilterBudgetsByTags(ctx, budgets, map[string]string{})
	require.Len(t, result, 3)
}

// =============================================================================
// Tag Filtering Benchmarks
// =============================================================================

// BenchmarkFilterBudgetsByTags1000 benchmarks tag filtering for 1000 budgets.
func BenchmarkFilterBudgetsByTags1000(b *testing.B) {
	ctx := context.Background()
	budgets := generateBudgetsWithTags(1000)
	tags := map[string]string{"namespace": "prod-*", "cluster": "us-*"}

	b.ResetTimer()
	for b.Loop() {
		_ = FilterBudgetsByTags(ctx, budgets, tags)
	}
}

// BenchmarkMatchesBudgetTagsWithGlob benchmarks single budget tag matching.
func BenchmarkMatchesBudgetTagsWithGlob(b *testing.B) {
	budget := &pbc.Budget{
		Id: "1",
		Metadata: map[string]string{
			"namespace": "prod-us-east-1",
			"cluster":   "us-east-1",
			"env":       "production",
		},
	}
	tags := map[string]string{"namespace": "prod-*", "cluster": "us-*"}

	b.ResetTimer()
	for b.Loop() {
		_ = matchesBudgetTagsWithGlob(budget, tags)
	}
}

// generateBudgetsWithTags creates n budgets with varying metadata for benchmarking.
func generateBudgetsWithTags(n int) []*pbc.Budget {
	namespaces := []string{"prod-us", "prod-eu", "staging-us", "dev-local"}
	clusters := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}
	budgets := make([]*pbc.Budget, n)

	for i := 0; i < n; i++ {
		idSuffix := strconv.Itoa(i)
		budgets[i] = &pbc.Budget{
			Id:     "budget-" + idSuffix,
			Name:   "Budget " + idSuffix,
			Source: "kubecost",
			Metadata: map[string]string{
				"namespace": namespaces[i%len(namespaces)],
				"cluster":   clusters[i%len(clusters)],
			},
		}
	}
	return budgets
}
