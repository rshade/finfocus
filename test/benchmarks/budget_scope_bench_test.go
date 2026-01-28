package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// BenchmarkBudgetScopeAllocation benchmarks the budget scope allocation for various resource counts.
// Performance target: <500ms for 10,000 resources (SC-003).
func BenchmarkBudgetScopeAllocation(b *testing.B) {
	// Create a comprehensive scoped budget configuration
	cfg := &config.BudgetsConfig{
		Global: &config.ScopedBudget{
			Amount:   100000,
			Currency: "USD",
			Period:   "monthly",
		},
		Providers: map[string]*config.ScopedBudget{
			"aws":   {Amount: 50000, Currency: "USD"},
			"gcp":   {Amount: 30000, Currency: "USD"},
			"azure": {Amount: 20000, Currency: "USD"},
		},
		Tags: []config.TagBudget{
			{
				Selector:     "team:platform",
				Priority:     100,
				ScopedBudget: config.ScopedBudget{Amount: 20000, Currency: "USD"},
			},
			{
				Selector:     "team:backend",
				Priority:     100,
				ScopedBudget: config.ScopedBudget{Amount: 15000, Currency: "USD"},
			},
			{
				Selector:     "team:frontend",
				Priority:     100,
				ScopedBudget: config.ScopedBudget{Amount: 10000, Currency: "USD"},
			},
			{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 40000, Currency: "USD"}},
			{Selector: "env:staging", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 10000, Currency: "USD"}},
			{Selector: "cost-center:*", Priority: 10, ScopedBudget: config.ScopedBudget{Amount: 5000, Currency: "USD"}},
		},
		Types: map[string]*config.ScopedBudget{
			"aws:ec2/instance":     {Amount: 20000, Currency: "USD"},
			"aws:rds/instance":     {Amount: 15000, Currency: "USD"},
			"aws:lambda/function":  {Amount: 5000, Currency: "USD"},
			"gcp:compute/instance": {Amount: 15000, Currency: "USD"},
			"azure:compute/vm":     {Amount: 10000, Currency: "USD"},
		},
	}

	// Resource types to cycle through
	resourceTypes := []string{
		"aws:ec2/instance",
		"aws:rds/instance",
		"aws:lambda/function",
		"aws:s3/bucket",
		"gcp:compute/instance",
		"gcp:storage/bucket",
		"azure:compute/vm",
		"azure:storage/account",
	}

	// Tag sets to cycle through
	tagSets := []map[string]string{
		{"team": "platform", "env": "prod", "cost-center": "eng-001"},
		{"team": "backend", "env": "prod"},
		{"team": "frontend", "env": "staging"},
		{"env": "prod"},
		{},
	}

	benchmarkCases := []struct {
		name          string
		resourceCount int
	}{
		{"100_resources", 100},
		{"1000_resources", 1000},
		{"5000_resources", 5000},
		{"10000_resources", 10000},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			ctx := context.Background()

			// Pre-generate resources to avoid allocation during benchmark
			type resource struct {
				resourceType string
				tags         map[string]string
				cost         float64
			}
			resources := make([]resource, bc.resourceCount)
			for i := range resources {
				resources[i] = resource{
					resourceType: resourceTypes[i%len(resourceTypes)],
					tags:         tagSets[i%len(tagSets)],
					cost:         100.0 + float64(i%1000)/10.0, // Varying costs
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Recreate evaluator per iteration to avoid state accumulation
				eval := engine.NewScopedBudgetEvaluator(cfg)
				for _, res := range resources {
					eval.AllocateCosts(ctx, res.resourceType, res.tags, res.cost)
				}
			}
		})
	}
}

// BenchmarkCalculateOverallHealth benchmarks health calculation with varying scope counts.
func BenchmarkCalculateOverallHealth(b *testing.B) {
	benchmarkCases := []struct {
		name          string
		providerCount int
		tagCount      int
		typeCount     int
	}{
		{"small_10_scopes", 3, 3, 4},
		{"medium_50_scopes", 10, 20, 20},
		{"large_100_scopes", 20, 40, 40},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			// Build a ScopedBudgetResult with the specified number of scopes
			result := &engine.ScopedBudgetResult{
				Global: &engine.ScopedBudgetStatus{
					ScopeType: engine.ScopeTypeGlobal,
					ScopeKey:  "global",
					Budget: config.ScopedBudget{
						Amount:   100000,
						Currency: "USD",
					},
					CurrentSpend: 75000,
					Percentage:   75.0,
				},
				ByProvider: make(map[string]*engine.ScopedBudgetStatus),
				ByTag:      make([]*engine.ScopedBudgetStatus, 0, bc.tagCount),
				ByType:     make(map[string]*engine.ScopedBudgetStatus),
			}

			// Add provider statuses
			providers := []string{"aws", "gcp", "azure", "oci", "digitalocean", "linode"}
			for i := 0; i < bc.providerCount && i < len(providers); i++ {
				result.ByProvider[providers[i]] = &engine.ScopedBudgetStatus{
					ScopeType: engine.ScopeTypeProvider,
					ScopeKey:  providers[i],
					Budget: config.ScopedBudget{
						Amount:   30000,
						Currency: "USD",
					},
					CurrentSpend: float64(10000 + i*5000),
					Percentage:   float64(30 + i*15),
				}
			}

			// Add tag statuses (as slice)
			for i := 0; i < bc.tagCount; i++ {
				key := fmt.Sprintf("team:team-%d", i)
				percentage := float64(40 + i%60)
				result.ByTag = append(result.ByTag, &engine.ScopedBudgetStatus{
					ScopeType: engine.ScopeTypeTag,
					ScopeKey:  key,
					Budget: config.ScopedBudget{
						Amount:   10000,
						Currency: "USD",
					},
					CurrentSpend: percentage * 100,
					Percentage:   percentage,
				})
			}

			// Add type statuses
			for i := 0; i < bc.typeCount; i++ {
				key := fmt.Sprintf("aws:service-%d/resource", i)
				percentage := float64(30 + i%70)
				result.ByType[key] = &engine.ScopedBudgetStatus{
					ScopeType: engine.ScopeTypeType,
					ScopeKey:  key,
					Budget: config.ScopedBudget{
						Amount:   5000,
						Currency: "USD",
					},
					CurrentSpend: percentage * 50,
					Percentage:   percentage,
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = engine.CalculateOverallHealth(result)
			}
		})
	}
}

// BenchmarkTagMatching benchmarks tag selector matching with varying tag counts.
func BenchmarkTagMatching(b *testing.B) {
	benchmarkCases := []struct {
		name     string
		tagCount int
	}{
		{"5_tag_budgets", 5},
		{"20_tag_budgets", 20},
		{"50_tag_budgets", 50},
	}

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			ctx := context.Background()
			tags := make([]config.TagBudget, bc.tagCount)
			for i := 0; i < bc.tagCount; i++ {
				tags[i] = config.TagBudget{
					Selector: fmt.Sprintf("tag-%d:value-%d", i, i),
					Priority: i * 10,
					ScopedBudget: config.ScopedBudget{
						Amount:   float64(1000 + i*100),
						Currency: "USD",
					},
				}
			}

			cfg := &config.BudgetsConfig{
				Global: &config.ScopedBudget{Amount: 100000, Currency: "USD"},
				Tags:   tags,
			}

			eval := engine.NewScopedBudgetEvaluator(cfg)

			// Resource tags that may match
			resourceTags := map[string]string{
				"tag-5":  "value-5",
				"tag-10": "value-10",
				"env":    "prod",
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = eval.MatchTagBudgets(ctx, resourceTags)
			}
		})
	}
}
