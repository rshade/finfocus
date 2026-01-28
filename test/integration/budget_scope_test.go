package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// TestProviderBudget_EndToEnd tests provider budget flow from config to evaluation (T020).
func TestProviderBudget_EndToEnd(t *testing.T) {
	ctx := context.Background()

	t.Run("multi-provider budget configuration and evaluation", func(t *testing.T) {
		// Step 1: Parse YAML configuration
		configYAML := `
global:
  amount: 10000
  currency: USD
providers:
  aws:
    amount: 5000
    currency: USD
  gcp:
    amount: 3000
    currency: USD
  azure:
    amount: 2000
    currency: USD
`
		var budgetsCfg config.BudgetsConfig
		err := yaml.Unmarshal([]byte(configYAML), &budgetsCfg)
		require.NoError(t, err)

		// Step 2: Validate configuration
		warnings, err := budgetsCfg.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		// Step 3: Create evaluator
		eval := engine.NewScopedBudgetEvaluator(&budgetsCfg)
		require.NotNil(t, eval)

		// Step 4: Simulate cost allocations for AWS resources
		awsResources := []struct {
			resourceType string
			cost         float64
		}{
			{"aws:ec2/instance", 1000.0},
			{"aws:rds/instance", 2000.0},
			{"aws:s3/bucket", 500.0},
		}

		var awsTotalCost float64
		for _, res := range awsResources {
			allocation := eval.AllocateCostToProvider(ctx, res.resourceType, res.cost)
			require.NotNil(t, allocation)
			assert.Equal(t, "aws", allocation.Provider)
			assert.Contains(t, allocation.AllocatedScopes, "provider:aws")
			awsTotalCost += res.cost
		}
		assert.Equal(t, 3500.0, awsTotalCost)

		// Step 5: Calculate provider budget status
		awsBudget := eval.GetProviderBudget("aws")
		require.NotNil(t, awsBudget)

		awsStatus := engine.CalculateProviderBudgetStatus("aws", awsBudget, awsTotalCost)
		require.NotNil(t, awsStatus)

		// At 3500/5000 = 70%, health should be OK
		assert.Equal(t, engine.ScopeTypeProvider, awsStatus.ScopeType)
		assert.Equal(t, "aws", awsStatus.ScopeKey)
		assert.Equal(t, 3500.0, awsStatus.CurrentSpend)
		assert.Equal(t, 70.0, awsStatus.Percentage)
		assert.False(t, awsStatus.IsOverBudget())
	})

	t.Run("provider budget warning threshold", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Simulate 850/1000 = 85% spend (WARNING threshold)
		awsBudget := eval.GetProviderBudget("aws")
		require.NotNil(t, awsBudget)

		status := engine.CalculateProviderBudgetStatus("aws", awsBudget, 850.0)
		assert.Equal(t, 85.0, status.Percentage)
		// 85% should be WARNING (between 80% and 90%)
		assert.False(t, status.IsOverBudget())
	})

	t.Run("provider budget exceeded threshold", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"gcp": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Simulate 1500/1000 = 150% spend (EXCEEDED threshold)
		gcpBudget := eval.GetProviderBudget("gcp")
		require.NotNil(t, gcpBudget)

		status := engine.CalculateProviderBudgetStatus("gcp", gcpBudget, 1500.0)
		assert.Equal(t, 150.0, status.Percentage)
		assert.True(t, status.IsOverBudget())
	})

	t.Run("AWS costs only count toward AWS budget, not GCP", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 5000, Currency: "USD"},
				"gcp": {Amount: 3000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Allocate AWS resource cost
		awsAllocation := eval.AllocateCostToProvider(ctx, "aws:ec2/instance", 1000.0)
		assert.Equal(t, "aws", awsAllocation.Provider)
		assert.Contains(t, awsAllocation.AllocatedScopes, "provider:aws")
		assert.NotContains(t, awsAllocation.AllocatedScopes, "provider:gcp")

		// Allocate GCP resource cost
		gcpAllocation := eval.AllocateCostToProvider(ctx, "gcp:compute/instance", 500.0)
		assert.Equal(t, "gcp", gcpAllocation.Provider)
		assert.Contains(t, gcpAllocation.AllocatedScopes, "provider:gcp")
		assert.NotContains(t, gcpAllocation.AllocatedScopes, "provider:aws")

		// Verify each provider's budget sees only its costs
		awsStatus := engine.CalculateProviderBudgetStatus("aws", eval.GetProviderBudget("aws"), 1000.0)
		assert.Equal(t, 20.0, awsStatus.Percentage) // 1000/5000 = 20%

		gcpStatus := engine.CalculateProviderBudgetStatus("gcp", eval.GetProviderBudget("gcp"), 500.0)
		assert.InDelta(t, 16.67, gcpStatus.Percentage, 0.01) // 500/3000 ≈ 16.67%
	})
}

// TestTagBudget_EndToEnd tests tag budget flow from config to evaluation (T033).
func TestTagBudget_EndToEnd(t *testing.T) {
	ctx := context.Background()

	t.Run("tag budget configuration and matching", func(t *testing.T) {
		// Step 1: Parse YAML configuration with tag budgets
		configYAML := `
global:
  amount: 10000
  currency: USD
tags:
  - selector: "team:platform"
    priority: 100
    amount: 2000
    currency: USD
  - selector: "team:backend"
    priority: 100
    amount: 2500
    currency: USD
  - selector: "env:prod"
    priority: 50
    amount: 5000
    currency: USD
  - selector: "cost-center:*"
    priority: 10
    amount: 1000
    currency: USD
`
		var budgetsCfg config.BudgetsConfig
		err := yaml.Unmarshal([]byte(configYAML), &budgetsCfg)
		require.NoError(t, err)

		// Step 2: Validate configuration
		warnings, err := budgetsCfg.Validate()
		require.NoError(t, err)
		// Should have warning about duplicate priority 100
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "priority 100")

		// Step 3: Create evaluator
		eval := engine.NewScopedBudgetEvaluator(&budgetsCfg)
		require.NotNil(t, eval)

		// Step 4: Test matching for resource with single tag
		singleTagResource := map[string]string{"team": "platform"}
		matches := eval.MatchTagBudgets(ctx, singleTagResource)
		require.Len(t, matches, 1)
		assert.Equal(t, "team:platform", matches[0].Selector)

		// Step 5: Test matching for resource with multiple tags
		multiTagResource := map[string]string{
			"team":        "platform",
			"env":         "prod",
			"cost-center": "engineering",
		}
		matches = eval.MatchTagBudgets(ctx, multiTagResource)
		require.Len(t, matches, 3)
		// Verify ordering by priority (highest first)
		assert.Equal(t, "team:platform", matches[0].Selector) // Priority 100
		assert.Equal(t, "env:prod", matches[1].Selector)      // Priority 50
		assert.Equal(t, "cost-center:*", matches[2].Selector) // Priority 10 (wildcard)

		// Step 6: Test priority selection with tie-breaking
		tieResource := map[string]string{"team": "backend"}
		tieMatches := eval.MatchTagBudgets(ctx, tieResource)
		require.Len(t, tieMatches, 1)

		// Now test with both priority-100 teams
		bothTeamsResource := map[string]string{
			"team": "platform",
		}
		matches = eval.MatchTagBudgets(ctx, bothTeamsResource)
		require.Len(t, matches, 1)
		// Only platform matches because tags are exact match

		// Step 7: Test SelectHighestPriorityTagBudget with overlapping priorities
		// Create matches that would have overlapping priorities
		allMatches := eval.MatchTagBudgets(ctx, map[string]string{
			"team":        "backend", // Matches team:backend (priority 100)
			"cost-center": "ops",     // Matches cost-center:* (priority 10)
		})

		selected, selectWarnings := eval.SelectHighestPriorityTagBudget(ctx, allMatches)
		require.NotNil(t, selected)
		assert.Equal(t, "team:backend", selected.Selector)
		assert.Empty(t, selectWarnings) // No warning because clear priority winner
	})

	t.Run("wildcard tag selector matching", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "env:*", Priority: 10, ScopedBudget: config.ScopedBudget{Amount: 1000}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Wildcard should match any value for the key
		ctx := context.Background()
		devMatches := eval.MatchTagBudgets(ctx, map[string]string{"env": "dev"})
		require.Len(t, devMatches, 1)
		assert.Equal(t, "env:*", devMatches[0].Selector)

		prodMatches := eval.MatchTagBudgets(ctx, map[string]string{"env": "prod"})
		require.Len(t, prodMatches, 1)
		assert.Equal(t, "env:*", prodMatches[0].Selector)

		// Key not present should not match
		noKeyMatches := eval.MatchTagBudgets(ctx, map[string]string{"team": "platform"})
		assert.Empty(t, noKeyMatches)
	})

	t.Run("overlapping tag budgets with warning emission", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "team:backend", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2500}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		ctx := context.Background()

		// Resource matches both team:platform and team:backend cannot happen with exact match
		// But we can test the tie-breaking logic directly
		matches := []config.TagBudget{
			{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
			{Selector: "team:backend", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2500}},
		}

		selected, warnings := eval.SelectHighestPriorityTagBudget(ctx, matches)
		require.NotNil(t, selected)
		// "backend" comes before "platform" alphabetically
		assert.Equal(t, "team:backend", selected.Selector)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "overlapping")
		assert.Contains(t, warnings[0], "priority 100")
	})

	t.Run("resources with team tag count only toward team budget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Tags: []config.TagBudget{
				{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000}},
				{Selector: "team:backend", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2500}},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		ctx := context.Background()

		// Platform team resource
		platformMatches := eval.MatchTagBudgets(ctx, map[string]string{"team": "platform"})
		require.Len(t, platformMatches, 1)
		assert.Equal(t, "team:platform", platformMatches[0].Selector)

		// Backend team resource
		backendMatches := eval.MatchTagBudgets(ctx, map[string]string{"team": "backend"})
		require.Len(t, backendMatches, 1)
		assert.Equal(t, "team:backend", backendMatches[0].Selector)

		// Resource with no team tag matches neither
		noTeamMatches := eval.MatchTagBudgets(ctx, map[string]string{"env": "prod"})
		assert.Empty(t, noTeamMatches)
	})
}

// TestScopedBudgets_YAMLConfigParsing tests full YAML config parsing for scoped budgets.
func TestScopedBudgets_YAMLConfigParsing(t *testing.T) {
	t.Run("parse full scoped budget config", func(t *testing.T) {
		configYAML := `
global:
  amount: 10000
  currency: USD
  period: monthly
  alerts:
    - threshold: 50
      type: actual
    - threshold: 80
      type: actual
    - threshold: 100
      type: forecasted
providers:
  aws:
    amount: 5000
    currency: USD
  gcp:
    amount: 3000
    currency: USD
tags:
  - selector: "team:platform"
    priority: 100
    amount: 2000
    currency: USD
  - selector: "env:prod"
    priority: 50
    amount: 5000
    currency: USD
types:
  "aws:ec2/instance":
    amount: 1000
    currency: USD
exit_on_threshold: true
exit_code: 2
`
		var cfg config.BudgetsConfig
		err := yaml.Unmarshal([]byte(configYAML), &cfg)
		require.NoError(t, err)

		// Validate structure
		require.NotNil(t, cfg.Global)
		assert.Equal(t, 10000.0, cfg.Global.Amount)
		assert.Equal(t, "USD", cfg.Global.Currency)
		assert.Len(t, cfg.Global.Alerts, 3)

		// Validate providers
		assert.Len(t, cfg.Providers, 2)
		assert.Equal(t, 5000.0, cfg.Providers["aws"].Amount)
		assert.Equal(t, 3000.0, cfg.Providers["gcp"].Amount)

		// Validate tags
		assert.Len(t, cfg.Tags, 2)
		assert.Equal(t, "team:platform", cfg.Tags[0].Selector)
		assert.Equal(t, 100, cfg.Tags[0].Priority)

		// Validate types
		assert.Len(t, cfg.Types, 1)
		assert.Equal(t, 1000.0, cfg.Types["aws:ec2/instance"].Amount)

		// Validate exit settings
		assert.True(t, cfg.ExitOnThreshold)
		require.NotNil(t, cfg.ExitCode)
		assert.Equal(t, 2, *cfg.ExitCode)

		// Run full validation
		warnings, err := cfg.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)
	})

	t.Run("legacy to scoped budget migration", func(t *testing.T) {
		// Legacy format
		legacyYAML := `
budgets:
  amount: 5000
  currency: USD
  period: monthly
  alerts:
    - threshold: 80
      type: actual
  exit_on_threshold: true
  exit_code: 1
`
		var costCfg config.CostConfig
		err := yaml.Unmarshal([]byte(legacyYAML), &costCfg)
		require.NoError(t, err)

		// Verify legacy config is loaded
		assert.True(t, costCfg.Budgets.IsEnabled())
		assert.Equal(t, 5000.0, costCfg.Budgets.Amount)

		// Get effective budgets (should migrate to scoped format)
		effectiveBudgets := costCfg.GetEffectiveBudgets()
		require.NotNil(t, effectiveBudgets)
		require.NotNil(t, effectiveBudgets.Global)
		assert.Equal(t, 5000.0, effectiveBudgets.Global.Amount)
		assert.Equal(t, "USD", effectiveBudgets.Global.Currency)
	})
}

// TestFullScopedBudgetStatus_EndToEnd tests complete multi-scope budget evaluation (T057).
func TestFullScopedBudgetStatus_EndToEnd(t *testing.T) {
	ctx := context.Background()

	t.Run("complete multi-scope budget evaluation", func(t *testing.T) {
		// Step 1: Parse comprehensive YAML configuration
		configYAML := `
global:
  amount: 10000
  currency: USD
  period: monthly
providers:
  aws:
    amount: 5000
    currency: USD
  gcp:
    amount: 3000
    currency: USD
tags:
  - selector: "team:platform"
    priority: 100
    amount: 2000
    currency: USD
  - selector: "env:prod"
    priority: 50
    amount: 4000
    currency: USD
types:
  "aws:ec2/instance":
    amount: 1000
    currency: USD
  "aws:rds/instance":
    amount: 2500
    currency: USD
`
		var budgetsCfg config.BudgetsConfig
		err := yaml.Unmarshal([]byte(configYAML), &budgetsCfg)
		require.NoError(t, err)

		// Step 2: Validate configuration
		warnings, err := budgetsCfg.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		// Step 3: Create evaluator
		eval := engine.NewScopedBudgetEvaluator(&budgetsCfg)
		require.NotNil(t, eval)

		// Step 4: Allocate costs for multiple resources
		resources := []struct {
			resourceType string
			tags         map[string]string
			cost         float64
		}{
			{"aws:ec2/instance", map[string]string{"team": "platform", "env": "prod"}, 800.0},
			{"aws:ec2/instance", map[string]string{"team": "platform"}, 500.0},
			{"aws:rds/instance", map[string]string{"env": "prod"}, 2000.0},
			{"gcp:compute/instance", map[string]string{"team": "platform"}, 1500.0},
		}

		// Track allocations and total spend per scope
		globalSpend := 0.0
		providerSpend := map[string]float64{}
		tagSpend := map[string]float64{}
		typeSpend := map[string]float64{}

		for _, res := range resources {
			allocation := eval.AllocateCosts(ctx, res.resourceType, res.tags, res.cost)
			require.NotNil(t, allocation)

			// Track global spend
			if slices.Contains(allocation.AllocatedScopes, "global") {
				globalSpend += res.cost
			}

			// Track provider spend
			for _, scope := range allocation.AllocatedScopes {
				if len(scope) > len("provider:") && scope[:len("provider:")] == "provider:" {
					provider := scope[len("provider:"):]
					providerSpend[provider] += res.cost
				}
				if len(scope) > len("tag:") && scope[:len("tag:")] == "tag:" {
					tag := scope[len("tag:"):]
					tagSpend[tag] += res.cost
				}
				if len(scope) > len("type:") && scope[:len("type:")] == "type:" {
					resType := scope[len("type:"):]
					typeSpend[resType] += res.cost
				}
			}
		}

		// Step 5: Build ScopedBudgetResult
		result := &engine.ScopedBudgetResult{
			ByProvider: make(map[string]*engine.ScopedBudgetStatus),
			ByTag:      make([]*engine.ScopedBudgetStatus, 0),
			ByType:     make(map[string]*engine.ScopedBudgetStatus),
		}

		// Calculate global status
		if budgetsCfg.Global != nil {
			result.Global = &engine.ScopedBudgetStatus{
				ScopeType:    engine.ScopeTypeGlobal,
				Budget:       *budgetsCfg.Global,
				CurrentSpend: globalSpend,
				Percentage:   globalSpend / budgetsCfg.Global.Amount * 100,
				Health:       engine.CalculateHealthFromPercentage(globalSpend / budgetsCfg.Global.Amount * 100),
				Currency:     budgetsCfg.Global.Currency,
			}
		}

		// Calculate provider statuses
		for provider, budget := range budgetsCfg.Providers {
			spend := providerSpend[provider]
			result.ByProvider[provider] = engine.CalculateProviderBudgetStatus(provider, budget, spend)
		}

		// Calculate tag statuses
		for i := range budgetsCfg.Tags {
			tagBudget := &budgetsCfg.Tags[i]
			spend := tagSpend[tagBudget.Selector]
			result.ByTag = append(result.ByTag, engine.CalculateTagBudgetStatus(tagBudget, spend))
		}

		// Calculate type statuses
		for resType, budget := range budgetsCfg.Types {
			spend := typeSpend[resType]
			result.ByType[resType] = engine.CalculateTypeBudgetStatus(resType, budget, spend)
		}

		// Step 6: Calculate overall health and critical scopes
		result.OverallHealth = engine.CalculateOverallHealth(result)
		result.CriticalScopes = engine.IdentifyCriticalScopes(result)

		// Step 7: Verify results
		// Global: 4800/10000 = 48% (OK)
		require.NotNil(t, result.Global)
		assert.Equal(t, 4800.0, result.Global.CurrentSpend)
		assert.InDelta(t, 48.0, result.Global.Percentage, 0.1)

		// AWS: 3300/5000 = 66% (OK)
		awsStatus := result.ByProvider["aws"]
		require.NotNil(t, awsStatus)
		assert.Equal(t, 3300.0, awsStatus.CurrentSpend)
		assert.InDelta(t, 66.0, awsStatus.Percentage, 0.1)

		// GCP: 1500/3000 = 50% (OK)
		gcpStatus := result.ByProvider["gcp"]
		require.NotNil(t, gcpStatus)
		assert.Equal(t, 1500.0, gcpStatus.CurrentSpend)
		assert.InDelta(t, 50.0, gcpStatus.Percentage, 0.1)

		// EC2 instances: 1300/1000 = 130% (EXCEEDED)
		ec2Status := result.ByType["aws:ec2/instance"]
		require.NotNil(t, ec2Status)
		assert.Equal(t, 1300.0, ec2Status.CurrentSpend)
		assert.InDelta(t, 130.0, ec2Status.Percentage, 0.1)
		assert.True(t, ec2Status.IsOverBudget())

		// RDS instances: 2000/2500 = 80% (WARNING)
		rdsStatus := result.ByType["aws:rds/instance"]
		require.NotNil(t, rdsStatus)
		assert.Equal(t, 2000.0, rdsStatus.CurrentSpend)
		assert.InDelta(t, 80.0, rdsStatus.Percentage, 0.1)

		// Overall health should be EXCEEDED (worst wins)
		assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, result.OverallHealth)

		// Critical scopes should include EC2 type
		assert.Contains(t, result.CriticalScopes, "type:aws:ec2/instance")
	})
}

// TestTypeBudget_EndToEnd tests resource type budget flow from config to evaluation (T046).
func TestTypeBudget_EndToEnd(t *testing.T) {
	ctx := context.Background()

	t.Run("type budget configuration and evaluation", func(t *testing.T) {
		// Step 1: Parse YAML configuration with type budgets
		configYAML := `
global:
  amount: 10000
  currency: USD
types:
  "aws:ec2/instance":
    amount: 2000
    currency: USD
  "aws:rds/instance":
    amount: 3000
    currency: USD
  "gcp:compute/instance":
    amount: 1500
    currency: USD
`
		var budgetsCfg config.BudgetsConfig
		err := yaml.Unmarshal([]byte(configYAML), &budgetsCfg)
		require.NoError(t, err)

		// Step 2: Validate configuration
		warnings, err := budgetsCfg.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		// Step 3: Verify types are parsed correctly
		assert.Len(t, budgetsCfg.Types, 3)
		assert.Equal(t, 2000.0, budgetsCfg.Types["aws:ec2/instance"].Amount)
		assert.Equal(t, 3000.0, budgetsCfg.Types["aws:rds/instance"].Amount)
		assert.Equal(t, 1500.0, budgetsCfg.Types["gcp:compute/instance"].Amount)

		// Step 4: Create evaluator
		eval := engine.NewScopedBudgetEvaluator(&budgetsCfg)
		require.NotNil(t, eval)

		// Step 5: Test type budget lookups
		ec2Budget := eval.GetTypeBudget("aws:ec2/instance")
		require.NotNil(t, ec2Budget)
		assert.Equal(t, 2000.0, ec2Budget.Amount)

		rdsBudget := eval.GetTypeBudget("aws:rds/instance")
		require.NotNil(t, rdsBudget)
		assert.Equal(t, 3000.0, rdsBudget.Amount)

		// Unconfigured type returns nil
		assert.Nil(t, eval.GetTypeBudget("aws:lambda/function"))

		// Step 6: Test cost allocation to type budgets
		ec2Allocation := eval.AllocateCostToType(ctx, "aws:ec2/instance", 500.0)
		require.NotNil(t, ec2Allocation)
		assert.Equal(t, "aws:ec2/instance", ec2Allocation.ResourceType)
		assert.Equal(t, "aws", ec2Allocation.Provider)
		assert.Contains(t, ec2Allocation.AllocatedScopes, "type:aws:ec2/instance")

		// Step 7: Calculate type budget status
		ec2Status := engine.CalculateTypeBudgetStatus("aws:ec2/instance", ec2Budget, 1700.0)
		require.NotNil(t, ec2Status)
		assert.Equal(t, engine.ScopeTypeType, ec2Status.ScopeType)
		assert.Equal(t, "aws:ec2/instance", ec2Status.ScopeKey)
		assert.Equal(t, 1700.0, ec2Status.CurrentSpend)
		assert.Equal(t, 85.0, ec2Status.Percentage) // 1700/2000 = 85%
		assert.False(t, ec2Status.IsOverBudget())
	})

	t.Run("EC2 resources count only toward EC2 type budget", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 2000, Currency: "USD"},
				"aws:rds/instance": {Amount: 3000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// EC2 allocation should only go to EC2 type budget
		ec2Allocation := eval.AllocateCostToType(ctx, "aws:ec2/instance", 500.0)
		assert.Contains(t, ec2Allocation.AllocatedScopes, "type:aws:ec2/instance")
		assert.NotContains(t, ec2Allocation.AllocatedScopes, "type:aws:rds/instance")

		// RDS allocation should only go to RDS type budget
		rdsAllocation := eval.AllocateCostToType(ctx, "aws:rds/instance", 1000.0)
		assert.Contains(t, rdsAllocation.AllocatedScopes, "type:aws:rds/instance")
		assert.NotContains(t, rdsAllocation.AllocatedScopes, "type:aws:ec2/instance")

		// Verify status calculations are independent
		ec2Status := engine.CalculateTypeBudgetStatus(
			"aws:ec2/instance",
			eval.GetTypeBudget("aws:ec2/instance"),
			500.0,
		)
		assert.Equal(t, 25.0, ec2Status.Percentage) // 500/2000 = 25%

		rdsStatus := engine.CalculateTypeBudgetStatus(
			"aws:rds/instance",
			eval.GetTypeBudget("aws:rds/instance"),
			1000.0,
		)
		assert.InDelta(t, 33.33, rdsStatus.Percentage, 0.01) // 1000/3000 ≈ 33.33%
	})

	t.Run("type budget exceeded threshold", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)
		ec2Budget := eval.GetTypeBudget("aws:ec2/instance")
		require.NotNil(t, ec2Budget)

		// 1500/1000 = 150% - EXCEEDED
		status := engine.CalculateTypeBudgetStatus("aws:ec2/instance", ec2Budget, 1500.0)
		assert.Equal(t, 150.0, status.Percentage)
		assert.True(t, status.IsOverBudget())
	})

	t.Run("type matching is case-sensitive", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Exact case matches
		require.NotNil(t, eval.GetTypeBudget("aws:ec2/instance"))

		// Different case does NOT match (unlike providers which are case-insensitive)
		assert.Nil(t, eval.GetTypeBudget("AWS:ec2/instance"))
		assert.Nil(t, eval.GetTypeBudget("aws:EC2/instance"))
		assert.Nil(t, eval.GetTypeBudget("Aws:Ec2/Instance"))
	})

	t.Run("type budget with combined provider and type scopes", func(t *testing.T) {
		cfg := &config.BudgetsConfig{
			Global: &config.ScopedBudget{Amount: 10000, Currency: "USD"},
			Providers: map[string]*config.ScopedBudget{
				"aws": {Amount: 5000, Currency: "USD"},
			},
			Types: map[string]*config.ScopedBudget{
				"aws:ec2/instance": {Amount: 1000, Currency: "USD"},
			},
		}

		eval := engine.NewScopedBudgetEvaluator(cfg)

		// Both provider and type budgets should be available
		awsBudget := eval.GetProviderBudget("aws")
		require.NotNil(t, awsBudget)
		assert.Equal(t, 5000.0, awsBudget.Amount)

		ec2Budget := eval.GetTypeBudget("aws:ec2/instance")
		require.NotNil(t, ec2Budget)
		assert.Equal(t, 1000.0, ec2Budget.Amount)

		// Resource cost can be allocated to both scopes
		providerAlloc := eval.AllocateCostToProvider(ctx, "aws:ec2/instance", 500.0)
		assert.Contains(t, providerAlloc.AllocatedScopes, "provider:aws")

		typeAlloc := eval.AllocateCostToType(ctx, "aws:ec2/instance", 500.0)
		assert.Contains(t, typeAlloc.AllocatedScopes, "type:aws:ec2/instance")
	})
}
