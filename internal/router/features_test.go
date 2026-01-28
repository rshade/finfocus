package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidFeature(t *testing.T) {
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
			got := IsValidFeature(tt.feature)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidFeatures(t *testing.T) {
	features := ValidFeatures()

	require.Len(t, features, 6, "should have 6 valid features")

	expected := []Feature{
		FeatureProjectedCosts,
		FeatureActualCosts,
		FeatureRecommendations,
		FeatureCarbon,
		FeatureDryRun,
		FeatureBudgets,
	}

	assert.Equal(t, expected, features)
}

func TestValidFeatureNames(t *testing.T) {
	names := ValidFeatureNames()

	require.Len(t, names, 6)

	expected := []string{
		"ProjectedCosts",
		"ActualCosts",
		"Recommendations",
		"Carbon",
		"DryRun",
		"Budgets",
	}

	assert.Equal(t, expected, names)
}

func TestParseFeature(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFeat  Feature
		wantValid bool
	}{
		{"valid ProjectedCosts", "ProjectedCosts", FeatureProjectedCosts, true},
		{"valid ActualCosts", "ActualCosts", FeatureActualCosts, true},
		{"valid Recommendations", "Recommendations", FeatureRecommendations, true},
		{"invalid lowercase", "projectedcosts", "", false},
		{"invalid empty", "", "", false},
		{"invalid unknown", "Unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feat, valid := ParseFeature(tt.input)
			assert.Equal(t, tt.wantFeat, feat)
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}

func TestFeatureFromMethod(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		wantFeat  Feature
		wantFound bool
	}{
		// Valid method mappings
		{"GetProjectedCost", "GetProjectedCost", FeatureProjectedCosts, true},
		{"GetActualCost", "GetActualCost", FeatureActualCosts, true},
		{"GetRecommendations", "GetRecommendations", FeatureRecommendations, true},
		{"GetCarbonFootprint", "GetCarbonFootprint", FeatureCarbon, true},
		{"PerformDryRun", "PerformDryRun", FeatureDryRun, true},
		{"GetBudgetStatus", "GetBudgetStatus", FeatureBudgets, true},
		{"GetBudgets", "GetBudgets", FeatureBudgets, true},
		{"GetBudgetHealth", "GetBudgetHealth", FeatureBudgets, true},
		{"EvaluateBudgetAlert", "EvaluateBudgetAlert", FeatureBudgets, true},

		// Invalid method mappings
		{"empty", "", "", false},
		{"unknown method", "UnknownMethod", "", false},
		{"lowercase", "getprojectedcost", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feat, found := FeatureFromMethod(tt.method)
			assert.Equal(t, tt.wantFeat, feat)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestDefaultFeatures(t *testing.T) {
	defaults := DefaultFeatures()

	require.Len(t, defaults, 2)
	assert.Contains(t, defaults, FeatureProjectedCosts)
	assert.Contains(t, defaults, FeatureActualCosts)
}
