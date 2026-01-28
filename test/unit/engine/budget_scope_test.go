package engine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// TestScopeType tests ScopeType constants and methods.
func TestScopeType(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "global", engine.ScopeTypeGlobal.String())
		assert.Equal(t, "provider", engine.ScopeTypeProvider.String())
		assert.Equal(t, "tag", engine.ScopeTypeTag.String())
		assert.Equal(t, "type", engine.ScopeTypeType.String())
	})

	t.Run("IsValid", func(t *testing.T) {
		assert.True(t, engine.ScopeTypeGlobal.IsValid())
		assert.True(t, engine.ScopeTypeProvider.IsValid())
		assert.True(t, engine.ScopeTypeTag.IsValid())
		assert.True(t, engine.ScopeTypeType.IsValid())
		assert.False(t, engine.ScopeType("invalid").IsValid())
		assert.False(t, engine.ScopeType("").IsValid())
	})
}

// TestExtractProvider tests provider extraction from resource types.
func TestExtractProvider(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		{"aws ec2", "aws:ec2/instance", "aws"},
		{"aws rds", "aws:rds/instance", "aws"},
		{"gcp compute", "gcp:compute/instance", "gcp"},
		{"azure vm", "azure:compute/virtualMachine", "azure"},
		{"uppercase provider", "AWS:ec2/instance", "aws"},
		{"mixed case", "Azure:Compute/VM", "azure"},
		{"no colon", "unknown", "unknown"},
		{"empty string", "", ""},
		{"colon at start", ":ec2/instance", ""},
		{"multiple colons", "aws:ec2:instance:extra", "aws"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.ExtractProvider(tt.resourceType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCalculateHealthFromPercentage tests health calculation.
func TestCalculateHealthFromPercentage(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		want       pbc.BudgetHealthStatus
	}{
		{"zero percent", 0, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
		{"50 percent", 50, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
		{"79 percent", 79, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
		{"80 percent (warning threshold)", 80, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING},
		{"85 percent", 85, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING},
		{"89 percent", 89, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING},
		{"90 percent (critical threshold)", 90, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL},
		{"95 percent", 95, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL},
		{"99 percent", 99, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL},
		{"100 percent (exceeded)", 100, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED},
		{"110 percent", 110, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED},
		{"negative percent", -10, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.CalculateHealthFromPercentage(tt.percentage)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAggregateHealthStatuses tests health aggregation.
func TestAggregateHealthStatuses(t *testing.T) {
	tests := []struct {
		name     string
		statuses []pbc.BudgetHealthStatus
		want     pbc.BudgetHealthStatus
	}{
		{
			name:     "empty list",
			statuses: []pbc.BudgetHealthStatus{},
			want:     pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED,
		},
		{
			name:     "single OK",
			statuses: []pbc.BudgetHealthStatus{pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			want:     pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name: "all OK",
			statuses: []pbc.BudgetHealthStatus{
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			},
			want: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
		},
		{
			name: "warning wins over OK",
			statuses: []pbc.BudgetHealthStatus{
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
			},
			want: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
		},
		{
			name: "critical wins over warning",
			statuses: []pbc.BudgetHealthStatus{
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			},
			want: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		},
		{
			name: "exceeded wins over all",
			statuses: []pbc.BudgetHealthStatus{
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
				pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
			},
			want: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.AggregateHealthStatuses(tt.statuses)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestScopedBudgetStatus_Methods tests ScopedBudgetStatus helper methods.
func TestScopedBudgetStatus_Methods(t *testing.T) {
	t.Run("IsOverBudget", func(t *testing.T) {
		assert.False(t, (&engine.ScopedBudgetStatus{Percentage: 99}).IsOverBudget())
		assert.True(t, (&engine.ScopedBudgetStatus{Percentage: 100}).IsOverBudget())
		assert.True(t, (&engine.ScopedBudgetStatus{Percentage: 150}).IsOverBudget())
	})

	t.Run("HasExceededAlerts", func(t *testing.T) {
		status := &engine.ScopedBudgetStatus{
			Alerts: []engine.ThresholdStatus{
				{Status: engine.ThresholdStatusOK},
				{Status: engine.ThresholdStatusApproaching},
			},
		}
		assert.False(t, status.HasExceededAlerts())

		status.Alerts = append(status.Alerts, engine.ThresholdStatus{Status: engine.ThresholdStatusExceeded})
		assert.True(t, status.HasExceededAlerts())
	})

	t.Run("ScopeIdentifier", func(t *testing.T) {
		assert.Equal(t, "global", (&engine.ScopedBudgetStatus{ScopeType: engine.ScopeTypeGlobal}).ScopeIdentifier())
		assert.Equal(
			t,
			"provider:aws",
			(&engine.ScopedBudgetStatus{ScopeType: engine.ScopeTypeProvider, ScopeKey: "aws"}).ScopeIdentifier(),
		)
		assert.Equal(
			t,
			"tag:team:platform",
			(&engine.ScopedBudgetStatus{ScopeType: engine.ScopeTypeTag, ScopeKey: "team:platform"}).ScopeIdentifier(),
		)
		assert.Equal(
			t,
			"type:aws:ec2/instance",
			(&engine.ScopedBudgetStatus{ScopeType: engine.ScopeTypeType, ScopeKey: "aws:ec2/instance"}).ScopeIdentifier(),
		)
	})
}

// TestScopedBudgetResult_Methods tests ScopedBudgetResult helper methods.
func TestScopedBudgetResult_Methods(t *testing.T) {
	t.Run("HasExceededBudgets", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}
		assert.False(t, result.HasExceededBudgets())

		result.OverallHealth = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL
		assert.False(t, result.HasExceededBudgets())

		result.OverallHealth = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
		assert.True(t, result.HasExceededBudgets())
	})

	t.Run("HasCriticalBudgets", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK}
		assert.False(t, result.HasCriticalBudgets())

		result.OverallHealth = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING
		assert.False(t, result.HasCriticalBudgets())

		result.OverallHealth = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL
		assert.True(t, result.HasCriticalBudgets())

		result.OverallHealth = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED
		assert.True(t, result.HasCriticalBudgets())
	})

	t.Run("AllScopes", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{ScopeType: engine.ScopeTypeGlobal},
			ByProvider: map[string]*engine.ScopedBudgetStatus{
				"gcp": {ScopeType: engine.ScopeTypeProvider, ScopeKey: "gcp"},
				"aws": {ScopeType: engine.ScopeTypeProvider, ScopeKey: "aws"},
			},
			ByTag: []*engine.ScopedBudgetStatus{
				{ScopeType: engine.ScopeTypeTag, ScopeKey: "team:platform"},
			},
			ByType: map[string]*engine.ScopedBudgetStatus{
				"aws:ec2/instance": {ScopeType: engine.ScopeTypeType, ScopeKey: "aws:ec2/instance"},
			},
		}

		scopes := result.AllScopes()
		require.Len(t, scopes, 5)

		// Check order: global, then providers (sorted), then tags, then types (sorted)
		assert.Equal(t, engine.ScopeTypeGlobal, scopes[0].ScopeType)
		assert.Equal(t, "aws", scopes[1].ScopeKey) // aws before gcp alphabetically
		assert.Equal(t, "gcp", scopes[2].ScopeKey)
		assert.Equal(t, engine.ScopeTypeTag, scopes[3].ScopeType)
		assert.Equal(t, engine.ScopeTypeType, scopes[4].ScopeType)
	})
}

// TestScopedBudgetEvaluator tests the evaluator initialization and lookups.
func TestScopedBudgetEvaluator(t *testing.T) {
	t.Run("NewScopedBudgetEvaluator with nil config", func(t *testing.T) {
		eval := engine.NewScopedBudgetEvaluator(nil)
		require.NotNil(t, eval)

		assert.Nil(t, eval.GetProviderBudget("aws"))
		assert.Nil(t, eval.GetTypeBudget("aws:ec2/instance"))
	})

	t.Run("NewScopedBudgetEvaluator with full config", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"AWS": {Amount: 5000},
				"GCP": {Amount: 3000},
			},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000}},
				{Selector: "team:backend", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2500}},
			},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		require.NotNil(t, eval)

		// Test case-insensitive provider lookup
		awsBudget := eval.GetProviderBudget("aws")
		require.NotNil(t, awsBudget)
		assert.Equal(t, 5000.0, awsBudget.Amount)

		awsBudget2 := eval.GetProviderBudget("AWS")
		require.NotNil(t, awsBudget2)
		assert.Equal(t, 5000.0, awsBudget2.Amount)

		gcpBudget := eval.GetProviderBudget("gcp")
		require.NotNil(t, gcpBudget)
		assert.Equal(t, 3000.0, gcpBudget.Amount)

		// Test non-existent provider
		assert.Nil(t, eval.GetProviderBudget("azure"))

		// Test type lookup
		ec2Budget := eval.GetTypeBudget("aws:ec2/instance")
		require.NotNil(t, ec2Budget)
		assert.Equal(t, 1000.0, ec2Budget.Amount)

		assert.Nil(t, eval.GetTypeBudget("aws:rds/instance"))
	})

	t.Run("MatchTagBudgets", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000}},
				{Selector: "cost-center:*", Priority: 10, ScopedBudget: config.ScopedBudget{Amount: 500}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		ctx := context.Background()

		// Test single match
		matches := eval.MatchTagBudgets(ctx, map[string]string{"team": "platform"})
		require.Len(t, matches, 1)
		assert.Equal(t, "team:platform", matches[0].Selector)

		// Test multiple matches (should be in priority order)
		matches = eval.MatchTagBudgets(ctx, map[string]string{
			"team":        "platform",
			"env":         "prod",
			"cost-center": "finance",
		})
		require.Len(t, matches, 3)
		assert.Equal(t, "team:platform", matches[0].Selector) // Priority 100
		assert.Equal(t, "env:prod", matches[1].Selector)      // Priority 50
		assert.Equal(t, "cost-center:*", matches[2].Selector) // Priority 10

		// Test wildcard match
		matches = eval.MatchTagBudgets(ctx, map[string]string{"cost-center": "engineering"})
		require.Len(t, matches, 1)
		assert.Equal(t, "cost-center:*", matches[0].Selector)

		// Test no matches
		matches = eval.MatchTagBudgets(ctx, map[string]string{"unknown": "tag"})
		assert.Empty(t, matches)

		// Test empty tags
		matches = eval.MatchTagBudgets(ctx, map[string]string{})
		assert.Empty(t, matches)

		// Test nil tags
		matches = eval.MatchTagBudgets(ctx, nil)
		assert.Empty(t, matches)
	})

	t.Run("SelectHighestPriorityTagBudget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Tags: []config.TagBudget{
				{Selector: "team:backend", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2500}},
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		ctx := context.Background()

		// Test with no matches
		selected, warnings := eval.SelectHighestPriorityTagBudget(ctx, nil)
		assert.Nil(t, selected)
		assert.Empty(t, warnings)

		selected, warnings = eval.SelectHighestPriorityTagBudget(ctx, []config.TagBudget{})
		assert.Nil(t, selected)
		assert.Empty(t, warnings)

		// Test single match
		matches := []config.TagBudget{
			{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000}},
		}
		selected, warnings = eval.SelectHighestPriorityTagBudget(ctx, matches)
		require.NotNil(t, selected)
		assert.Equal(t, "env:prod", selected.Selector)
		assert.Empty(t, warnings)

		// Test priority tie - should select alphabetically first and emit warning
		matches = []config.TagBudget{
			{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
			{Selector: "team:backend", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2500}},
		}
		selected, warnings = eval.SelectHighestPriorityTagBudget(ctx, matches)
		require.NotNil(t, selected)
		assert.Equal(t, "team:backend", selected.Selector) // "backend" comes before "platform" alphabetically
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "overlapping")
		assert.Contains(t, warnings[0], "priority 100")

		// Test clear priority winner
		matches = []config.TagBudget{
			{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
			{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000}},
		}
		selected, warnings = eval.SelectHighestPriorityTagBudget(ctx, matches)
		require.NotNil(t, selected)
		assert.Equal(t, "team:platform", selected.Selector)
		assert.Empty(t, warnings)
	})
}

// TestAllocateCostToProvider tests provider-level cost allocation (T019).
func TestAllocateCostToProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("allocates to matching provider budget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 5000, Currency: "USD"},
				"gcp": {Amount: 3000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToProvider(ctx, "aws:ec2/instance", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "aws", allocation.Provider)
		assert.Equal(t, 100.0, allocation.Cost)
		assert.Contains(t, allocation.AllocatedScopes, "provider:aws")
	})

	t.Run("no allocation for missing provider budget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 5000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToProvider(ctx, "gcp:compute/instance", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "gcp", allocation.Provider)
		assert.NotContains(t, allocation.AllocatedScopes, "provider:gcp")
	})

	t.Run("case insensitive provider matching", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"AWS": {Amount: 5000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToProvider(ctx, "aws:ec2/instance", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "aws", allocation.Provider)
		assert.Contains(t, allocation.AllocatedScopes, "provider:aws")
	})

	t.Run("handles empty provider in resource type", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToProvider(ctx, ":unknown/resource", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "", allocation.Provider)
		assert.Empty(t, allocation.AllocatedScopes)
	})
}

// TestGetProviderBudgetStatus tests provider budget status calculation (T024).
func TestGetProviderBudgetStatus(t *testing.T) {
	t.Run("calculates provider budget status correctly", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   1000,
			Currency: "USD",
		}

		status := engine.CalculateProviderBudgetStatus("aws", budget, 850.0)

		require.NotNil(t, status)
		assert.Equal(t, engine.ScopeTypeProvider, status.ScopeType)
		assert.Equal(t, "aws", status.ScopeKey)
		assert.Equal(t, 850.0, status.CurrentSpend)
		assert.Equal(t, 85.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING, status.Health)
	})

	t.Run("handles zero budget amount", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   0,
			Currency: "USD",
		}

		status := engine.CalculateProviderBudgetStatus("aws", budget, 100.0)

		require.NotNil(t, status)
		// When budget is 0, percentage should be calculated safely
		assert.Equal(t, 0.0, status.Percentage)
	})

	t.Run("exceeded budget health", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   1000,
			Currency: "USD",
		}

		status := engine.CalculateProviderBudgetStatus("gcp", budget, 1200.0)

		require.NotNil(t, status)
		assert.Equal(t, 120.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, status.Health)
		assert.True(t, status.IsOverBudget())
	})
}

// TestAllocateCostToTag tests tag-level cost allocation (T039).
func TestAllocateCostToTag(t *testing.T) {
	ctx := context.Background()

	t.Run("allocates to matching tag budget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		tags := map[string]string{"team": "platform", "env": "prod"}
		allocation := eval.AllocateCostToTag(ctx, "aws:ec2/instance", tags, 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, 100.0, allocation.Cost)
		assert.Equal(t, "team:platform", allocation.SelectedTagBudget) // Higher priority
		assert.Contains(t, allocation.AllocatedScopes, "tag:team:platform")
		assert.Len(t, allocation.MatchedTags, 2)
		assert.Contains(t, allocation.MatchedTags, "team:platform")
		assert.Contains(t, allocation.MatchedTags, "env:prod")
		assert.Empty(t, allocation.Warnings)
	})

	t.Run("no allocation when no tags match", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		tags := map[string]string{"team": "backend"}
		allocation := eval.AllocateCostToTag(ctx, "aws:ec2/instance", tags, 100.0)

		require.NotNil(t, allocation)
		assert.Empty(t, allocation.AllocatedScopes)
		assert.Empty(t, allocation.MatchedTags)
		assert.Equal(t, "", allocation.SelectedTagBudget)
	})

	t.Run("no allocation with empty tags", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToTag(ctx, "aws:ec2/instance", nil, 100.0)

		require.NotNil(t, allocation)
		assert.Empty(t, allocation.AllocatedScopes)
	})

	t.Run("wildcard tag matching", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "cost-center:*", Priority: 10, ScopedBudget: config.ScopedBudget{Amount: 500}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		tags := map[string]string{"cost-center": "engineering"}
		allocation := eval.AllocateCostToTag(ctx, "aws:ec2/instance", tags, 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "cost-center:*", allocation.SelectedTagBudget)
		assert.Contains(t, allocation.AllocatedScopes, "tag:cost-center:*")
	})

	t.Run("emits warning for priority tie", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "env:*", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 5000}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		tags := map[string]string{"team": "platform", "env": "prod"}
		allocation := eval.AllocateCostToTag(ctx, "aws:ec2/instance", tags, 100.0)

		require.NotNil(t, allocation)
		// Should select alphabetically first (env:* < team:platform)
		assert.Equal(t, "env:*", allocation.SelectedTagBudget)
		require.Len(t, allocation.Warnings, 1)
		assert.Contains(t, allocation.Warnings[0], "overlapping")
	})
}

// TestCalculateTagBudgetStatus tests tag budget status calculation.
func TestCalculateTagBudgetStatus(t *testing.T) {
	t.Run("calculates tag budget status correctly", func(t *testing.T) {
		tagBudget := &config.TagBudget{
			Selector: "team:platform",
			Priority: 100,
			ScopedBudget: config.ScopedBudget{
				Amount:   1000,
				Currency: "USD",
			},
		}

		status := engine.CalculateTagBudgetStatus(tagBudget, 850.0)

		require.NotNil(t, status)
		assert.Equal(t, engine.ScopeTypeTag, status.ScopeType)
		assert.Equal(t, "team:platform", status.ScopeKey)
		assert.Equal(t, 850.0, status.CurrentSpend)
		assert.Equal(t, 85.0, status.Percentage)
	})

	t.Run("handles zero budget amount", func(t *testing.T) {
		tagBudget := &config.TagBudget{
			Selector:     "team:platform",
			ScopedBudget: config.ScopedBudget{Amount: 0, Currency: "USD"},
		}

		status := engine.CalculateTagBudgetStatus(tagBudget, 100.0)

		require.NotNil(t, status)
		assert.Equal(t, 0.0, status.Percentage)
	})

	t.Run("exceeded budget health", func(t *testing.T) {
		tagBudget := &config.TagBudget{
			Selector: "team:platform",
			ScopedBudget: config.ScopedBudget{
				Amount:   1000,
				Currency: "USD",
			},
		}

		status := engine.CalculateTagBudgetStatus(tagBudget, 1200.0)

		require.NotNil(t, status)
		assert.Equal(t, 120.0, status.Percentage)
		assert.True(t, status.IsOverBudget())
	})
}

// TestGetTypeBudget tests resource type budget lookup (T044).
func TestGetTypeBudget(t *testing.T) {
	t.Run("returns budget for configured type", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance":     {Amount: 1000, Currency: "USD"},
				"aws:rds/instance":     {Amount: 2000, Currency: "USD"},
				"gcp:compute/instance": {Amount: 1500, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Test exact type match
		ec2Budget := eval.GetTypeBudget("aws:ec2/instance")
		require.NotNil(t, ec2Budget)
		assert.Equal(t, 1000.0, ec2Budget.Amount)

		rdsBudget := eval.GetTypeBudget("aws:rds/instance")
		require.NotNil(t, rdsBudget)
		assert.Equal(t, 2000.0, rdsBudget.Amount)

		gcpBudget := eval.GetTypeBudget("gcp:compute/instance")
		require.NotNil(t, gcpBudget)
		assert.Equal(t, 1500.0, gcpBudget.Amount)
	})

	t.Run("returns nil for unconfigured type", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Type not in config
		assert.Nil(t, eval.GetTypeBudget("aws:rds/instance"))
		assert.Nil(t, eval.GetTypeBudget("azure:compute/virtualMachine"))
	})

	t.Run("type matching is case-sensitive", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Exact case works
		require.NotNil(t, eval.GetTypeBudget("aws:ec2/instance"))

		// Different case does not match (unlike providers)
		assert.Nil(t, eval.GetTypeBudget("AWS:ec2/instance"))
		assert.Nil(t, eval.GetTypeBudget("aws:EC2/instance"))
	})

	t.Run("handles nil config", func(t *testing.T) {
		eval := engine.NewScopedBudgetEvaluator(nil)
		assert.Nil(t, eval.GetTypeBudget("aws:ec2/instance"))
	})

	t.Run("handles empty types map", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types:  map[string]*config.ScopedBudget{},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		assert.Nil(t, eval.GetTypeBudget("aws:ec2/instance"))
	})
}

// TestAllocateCostToType tests resource type cost allocation (T045).
func TestAllocateCostToType(t *testing.T) {
	ctx := context.Background()

	t.Run("allocates to matching type budget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
				"aws:rds/instance": {Amount: 2000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToType(ctx, "aws:ec2/instance", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "aws:ec2/instance", allocation.ResourceType)
		assert.Equal(t, "aws", allocation.Provider)
		assert.Equal(t, 100.0, allocation.Cost)
		assert.Contains(t, allocation.AllocatedScopes, "type:aws:ec2/instance")
	})

	t.Run("no allocation for unconfigured type", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToType(ctx, "aws:rds/instance", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "aws:rds/instance", allocation.ResourceType)
		assert.Equal(t, "aws", allocation.Provider)
		assert.NotContains(t, allocation.AllocatedScopes, "type:aws:rds/instance")
		assert.Empty(t, allocation.AllocatedScopes)
	})

	t.Run("handles empty config", func(t *testing.T) {
		eval := engine.NewScopedBudgetEvaluator(nil)
		allocation := eval.AllocateCostToType(ctx, "aws:ec2/instance", 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "aws:ec2/instance", allocation.ResourceType)
		assert.Empty(t, allocation.AllocatedScopes)
	})

	t.Run("case sensitive type matching", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Exact match works
		allocation := eval.AllocateCostToType(ctx, "aws:ec2/instance", 100.0)
		assert.Contains(t, allocation.AllocatedScopes, "type:aws:ec2/instance")

		// Case mismatch does NOT match
		allocation2 := eval.AllocateCostToType(ctx, "AWS:ec2/instance", 100.0)
		assert.NotContains(t, allocation2.AllocatedScopes, "type:AWS:ec2/instance")
		assert.Empty(t, allocation2.AllocatedScopes)
	})

	t.Run("extracts provider correctly", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"gcp:compute/instance": {Amount: 1500, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCostToType(ctx, "gcp:compute/instance", 200.0)

		require.NotNil(t, allocation)
		assert.Equal(t, "gcp", allocation.Provider)
		assert.Contains(t, allocation.AllocatedScopes, "type:gcp:compute/instance")
	})
}

// TestCalculateTypeBudgetStatus tests type budget status calculation (T045).
func TestCalculateTypeBudgetStatus(t *testing.T) {
	t.Run("calculates type budget status correctly", func(t *testing.T) {
		resourceType := "aws:ec2/instance"
		budget := &config.ScopedBudget{
			Amount:   1000,
			Currency: "USD",
		}

		status := engine.CalculateTypeBudgetStatus(resourceType, budget, 850.0)

		require.NotNil(t, status)
		assert.Equal(t, engine.ScopeTypeType, status.ScopeType)
		assert.Equal(t, "aws:ec2/instance", status.ScopeKey)
		assert.Equal(t, 850.0, status.CurrentSpend)
		assert.Equal(t, 85.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING, status.Health)
		assert.Equal(t, "USD", status.Currency)
	})

	t.Run("handles zero budget amount", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   0,
			Currency: "USD",
		}

		status := engine.CalculateTypeBudgetStatus("aws:ec2/instance", budget, 100.0)

		require.NotNil(t, status)
		assert.Equal(t, 0.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK, status.Health)
	})

	t.Run("exceeded budget health", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   1000,
			Currency: "USD",
		}

		status := engine.CalculateTypeBudgetStatus("aws:rds/instance", budget, 1200.0)

		require.NotNil(t, status)
		assert.Equal(t, 120.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, status.Health)
		assert.True(t, status.IsOverBudget())
	})

	t.Run("OK status under threshold", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   1000,
			Currency: "USD",
		}

		status := engine.CalculateTypeBudgetStatus("aws:ec2/instance", budget, 500.0)

		require.NotNil(t, status)
		assert.Equal(t, 50.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK, status.Health)
	})

	t.Run("critical status near limit", func(t *testing.T) {
		budget := &config.ScopedBudget{
			Amount:   1000,
			Currency: "USD",
		}

		status := engine.CalculateTypeBudgetStatus("aws:ec2/instance", budget, 950.0)

		require.NotNil(t, status)
		assert.Equal(t, 95.0, status.Percentage)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL, status.Health)
	})
}

// TestBudgetAllocation tests the BudgetAllocation struct.
func TestBudgetAllocation(t *testing.T) {
	allocation := engine.BudgetAllocation{
		ResourceID:   "i-1234567890abcdef0",
		ResourceType: "aws:ec2/instance",
		Provider:     "aws",
		Cost:         100.50,
		AllocatedScopes: []string{
			"global",
			"provider:aws",
			"tag:team:platform",
			"type:aws:ec2/instance",
		},
		MatchedTags:       []string{"team:platform", "env:prod"},
		SelectedTagBudget: "team:platform",
		Warnings:          []string{},
	}

	assert.Equal(t, "i-1234567890abcdef0", allocation.ResourceID)
	assert.Equal(t, "aws:ec2/instance", allocation.ResourceType)
	assert.Equal(t, "aws", allocation.Provider)
	assert.Equal(t, 100.50, allocation.Cost)
	assert.Len(t, allocation.AllocatedScopes, 4)
	assert.Len(t, allocation.MatchedTags, 2)
	assert.Equal(t, "team:platform", allocation.SelectedTagBudget)
	assert.Empty(t, allocation.Warnings)
}

// TestAllocateCosts tests multi-scope cost allocation (T054).
func TestAllocateCosts(t *testing.T) {
	ctx := context.Background()

	t.Run("allocates to global, provider, tag, and type scopes", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 5000, Currency: "USD"},
			},
			Tags: []config.TagBudget{
				{
					Selector:     "team:platform",
					Priority:     100,
					ScopedBudget: config.ScopedBudget{Amount: 2000, Currency: "USD"},
				},
			},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Allocate cost for a resource that matches all scopes
		allocation := eval.AllocateCosts(ctx, "aws:ec2/instance", map[string]string{"team": "platform"}, 100.0)

		require.NotNil(t, allocation)
		assert.Equal(t, 100.0, allocation.Cost)
		assert.Equal(t, "aws", allocation.Provider)
		assert.Contains(t, allocation.AllocatedScopes, "global")
		assert.Contains(t, allocation.AllocatedScopes, "provider:aws")
		assert.Contains(t, allocation.AllocatedScopes, "tag:team:platform")
		assert.Contains(t, allocation.AllocatedScopes, "type:aws:ec2/instance")
	})

	t.Run("allocates to global and provider only when no tag or type match", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 5000, Currency: "USD"},
			},
			Tags: []config.TagBudget{
				{
					Selector:     "team:backend",
					Priority:     100,
					ScopedBudget: config.ScopedBudget{Amount: 2000, Currency: "USD"},
				},
			},
			Types: map[string]*config.ScopedBudget{
				"aws:rds/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Allocate cost for a resource that only matches global and provider
		allocation := eval.AllocateCosts(ctx, "aws:ec2/instance", map[string]string{"team": "platform"}, 100.0)

		require.NotNil(t, allocation)
		assert.Contains(t, allocation.AllocatedScopes, "global")
		assert.Contains(t, allocation.AllocatedScopes, "provider:aws")
		assert.NotContains(t, allocation.AllocatedScopes, "tag:team:backend")
		assert.NotContains(t, allocation.AllocatedScopes, "type:aws:rds/instance")
	})

	t.Run("allocates to global only when nothing else configured", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		allocation := eval.AllocateCosts(ctx, "aws:ec2/instance", nil, 100.0)

		require.NotNil(t, allocation)
		assert.Contains(t, allocation.AllocatedScopes, "global")
		assert.Len(t, allocation.AllocatedScopes, 1)
	})

	t.Run("handles nil config", func(t *testing.T) {
		eval := engine.NewScopedBudgetEvaluator(nil)
		allocation := eval.AllocateCosts(ctx, "aws:ec2/instance", nil, 100.0)

		require.NotNil(t, allocation)
		assert.Empty(t, allocation.AllocatedScopes)
	})
}

// TestCalculateOverallHealth tests worst-wins health aggregation (T055).
func TestCalculateOverallHealth(t *testing.T) {
	t.Run("returns worst health status", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			ByProvider: map[string]*engine.ScopedBudgetStatus{
				"aws": {Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING},
				"gcp": {Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			},
			ByTag: []*engine.ScopedBudgetStatus{
				{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL},
			},
			ByType: map[string]*engine.ScopedBudgetStatus{
				"aws:ec2/instance": {Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			},
		}

		overall := engine.CalculateOverallHealth(result)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL, overall)
	})

	t.Run("exceeded wins over all", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			ByProvider: map[string]*engine.ScopedBudgetStatus{
				"aws": {Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED},
			},
			ByTag: []*engine.ScopedBudgetStatus{
				{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL},
			},
		}

		overall := engine.CalculateOverallHealth(result)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, overall)
	})

	t.Run("all OK returns OK", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			ByProvider: map[string]*engine.ScopedBudgetStatus{
				"aws": {Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			},
		}

		overall := engine.CalculateOverallHealth(result)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK, overall)
	})

	t.Run("empty result returns unspecified", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{}
		overall := engine.CalculateOverallHealth(result)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED, overall)
	})

	t.Run("nil result returns unspecified", func(t *testing.T) {
		overall := engine.CalculateOverallHealth(nil)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED, overall)
	})
}

// TestIdentifyCriticalScopes tests critical scope identification (T056).
func TestIdentifyCriticalScopes(t *testing.T) {
	t.Run("identifies critical and exceeded scopes", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{
				ScopeType: engine.ScopeTypeGlobal,
				Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			},
			ByProvider: map[string]*engine.ScopedBudgetStatus{
				"aws": {
					ScopeType: engine.ScopeTypeProvider,
					ScopeKey:  "aws",
					Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
				},
				"gcp": {
					ScopeType: engine.ScopeTypeProvider,
					ScopeKey:  "gcp",
					Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				},
			},
			ByTag: []*engine.ScopedBudgetStatus{
				{
					ScopeType: engine.ScopeTypeTag,
					ScopeKey:  "team:platform",
					Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
				},
			},
			ByType: map[string]*engine.ScopedBudgetStatus{
				"aws:ec2/instance": {
					ScopeType: engine.ScopeTypeType,
					ScopeKey:  "aws:ec2/instance",
					Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				},
			},
		}

		criticalScopes := engine.IdentifyCriticalScopes(result)

		assert.Len(t, criticalScopes, 2)
		assert.Contains(t, criticalScopes, "provider:aws")
		assert.Contains(t, criticalScopes, "tag:team:platform")
		// OK and WARNING scopes should not be included
		assert.NotContains(t, criticalScopes, "global")
		assert.NotContains(t, criticalScopes, "provider:gcp")
		assert.NotContains(t, criticalScopes, "type:aws:ec2/instance")
	})

	t.Run("returns empty for all OK", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			ByProvider: map[string]*engine.ScopedBudgetStatus{
				"aws": {Health: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK},
			},
		}

		criticalScopes := engine.IdentifyCriticalScopes(result)
		assert.Empty(t, criticalScopes)
	})

	t.Run("handles nil result", func(t *testing.T) {
		criticalScopes := engine.IdentifyCriticalScopes(nil)
		assert.Empty(t, criticalScopes)
	})

	t.Run("includes all critical/exceeded types", func(t *testing.T) {
		result := &engine.ScopedBudgetResult{
			Global: &engine.ScopedBudgetStatus{
				ScopeType: engine.ScopeTypeGlobal,
				Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
			},
			ByType: map[string]*engine.ScopedBudgetStatus{
				"aws:ec2/instance": {
					ScopeType: engine.ScopeTypeType,
					ScopeKey:  "aws:ec2/instance",
					Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
				},
				"aws:rds/instance": {
					ScopeType: engine.ScopeTypeType,
					ScopeKey:  "aws:rds/instance",
					Health:    pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
				},
			},
		}

		criticalScopes := engine.IdentifyCriticalScopes(result)
		assert.Len(t, criticalScopes, 3)
		assert.Contains(t, criticalScopes, "global")
		assert.Contains(t, criticalScopes, "type:aws:ec2/instance")
		assert.Contains(t, criticalScopes, "type:aws:rds/instance")
	})
}
