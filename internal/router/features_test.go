package router_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/router"
)

func TestIsValidFeature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		feature string
		want    bool
	}{
		// Valid features
		{"ProjectedCosts", "ProjectedCosts", true},
		{"ActualCosts", "ActualCosts", true},
		{"Recommendations", "Recommendations", true},
		{"Carbon", "Carbon", true},
		{"DryRun", "DryRun", true},
		{"Budgets", "Budgets", true},
		{"BatchCost", "BatchCost", true},

		// Invalid features (case-sensitive)
		{"lowercase projected", "projectedcosts", false},
		{"uppercase", "PROJECTEDCOSTS", false},
		{"partial match", "Projected", false},

		// Invalid features
		{"empty string", "", false},
		{"unknown feature", "InvalidFeature", false},
		{"typo", "Recomendations", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := router.IsValidFeature(tt.feature)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidFeatures(t *testing.T) {
	t.Parallel()

	features := router.ValidFeatures()

	require.Len(t, features, 7, "should have 7 valid features")

	expected := []router.Feature{
		router.FeatureProjectedCosts,
		router.FeatureActualCosts,
		router.FeatureRecommendations,
		router.FeatureCarbon,
		router.FeatureDryRun,
		router.FeatureBudgets,
		router.FeatureBatchCost,
	}

	assert.Equal(t, expected, features)
}

func TestValidFeatureNames(t *testing.T) {
	t.Parallel()

	names := router.ValidFeatureNames()

	require.Len(t, names, 7)

	expected := []string{
		"ProjectedCosts",
		"ActualCosts",
		"Recommendations",
		"Carbon",
		"DryRun",
		"Budgets",
		"BatchCost",
	}

	assert.Equal(t, expected, names)
}

func TestParseFeature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantFeat  router.Feature
		wantValid bool
	}{
		{"valid ProjectedCosts", "ProjectedCosts", router.FeatureProjectedCosts, true},
		{"valid ActualCosts", "ActualCosts", router.FeatureActualCosts, true},
		{"valid Recommendations", "Recommendations", router.FeatureRecommendations, true},
		{"invalid lowercase", "projectedcosts", "", false},
		{"invalid empty", "", "", false},
		{"invalid unknown", "Unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			feat, valid := router.ParseFeature(tt.input)
			assert.Equal(t, tt.wantFeat, feat)
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}

func TestFeatureFromMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		method    string
		wantFeat  router.Feature
		wantFound bool
	}{
		// Valid method mappings
		{"GetProjectedCost", "GetProjectedCost", router.FeatureProjectedCosts, true},
		{"GetActualCost", "GetActualCost", router.FeatureActualCosts, true},
		{"GetRecommendations", "GetRecommendations", router.FeatureRecommendations, true},
		{"GetCarbonFootprint", "GetCarbonFootprint", router.FeatureCarbon, true},
		{"PerformDryRun", "PerformDryRun", router.FeatureDryRun, true},
		{"GetBudgetStatus", "GetBudgetStatus", router.FeatureBudgets, true},
		{"GetBudgets", "GetBudgets", router.FeatureBudgets, true},
		{"GetBudgetHealth", "GetBudgetHealth", router.FeatureBudgets, true},
		{"EvaluateBudgetAlert", "EvaluateBudgetAlert", router.FeatureBudgets, true},
		{"BatchCost", "BatchCost", router.FeatureBatchCost, true},

		// Invalid method mappings
		{"empty", "", "", false},
		{"unknown method", "UnknownMethod", "", false},
		{"lowercase", "getprojectedcost", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			feat, found := router.FeatureFromMethod(tt.method)
			assert.Equal(t, tt.wantFeat, feat)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestDefaultFeatures(t *testing.T) {
	t.Parallel()

	defaults := router.DefaultFeatures()
	require.Len(t, defaults, 2)
	assert.Equal(t, []router.Feature{router.FeatureProjectedCosts, router.FeatureActualCosts}, defaults)
}
