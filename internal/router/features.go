// Package router provides intelligent plugin routing for FinFocus cost calculations.
// It implements automatic provider-based routing and declarative pattern-based routing,
// enabling multi-cloud cost analysis with configurable plugin selection.
package router

// Feature represents a plugin capability type.
// Features determine which operations a plugin can handle (e.g., ProjectedCosts, Recommendations).
type Feature string

// Feature constants define all supported plugin capabilities.
// These map to gRPC methods in the CostSource service.
const (
	// FeatureProjectedCosts enables projected cost estimation from infrastructure specs.
	FeatureProjectedCosts Feature = "ProjectedCosts"

	// FeatureActualCosts enables historical cost retrieval from cloud provider APIs.
	FeatureActualCosts Feature = "ActualCosts"

	// FeatureRecommendations enables cost optimization recommendations.
	FeatureRecommendations Feature = "Recommendations"

	// FeatureCarbon enables carbon footprint estimation for resources.
	FeatureCarbon Feature = "Carbon"

	// FeatureDryRun enables dry run simulation of cost changes.
	FeatureDryRun Feature = "DryRun"

	// FeatureBudgets enables budget tracking and alerts.
	FeatureBudgets Feature = "Budgets"
)

// ValidFeatures returns all valid feature names.
// This is the authoritative list of features supported by the routing system.
func ValidFeatures() []Feature {
	return []Feature{
		FeatureProjectedCosts,
		FeatureActualCosts,
		FeatureRecommendations,
		FeatureCarbon,
		FeatureDryRun,
		FeatureBudgets,
	}
}

// IsValidFeature checks if a feature name is valid.
// Comparison is case-sensitive; use the exact constant values.
func IsValidFeature(name string) bool {
	for _, f := range ValidFeatures() {
		if string(f) == name {
			return true
		}
	}
	return false
}

// ValidFeatureNames returns the string names of all valid features.
// Useful for error messages and documentation.
func ValidFeatureNames() []string {
	features := ValidFeatures()
	names := make([]string, len(features))
	for i, f := range features {
		names[i] = string(f)
	}
	return names
}

// ParseFeature parses a string into a Feature.
// Returns the Feature and true if valid, or empty Feature and false if invalid.
func ParseFeature(s string) (Feature, bool) {
	if IsValidFeature(s) {
		return Feature(s), true
	}
	return "", false
}

// InferCapabilitiesFromMethods maps gRPC method names to features.
// This enables capability detection from plugin method availability
// until finfocus-spec#287 adds explicit capability reporting.
//
//nolint:gochecknoglobals // Immutable lookup table for method-to-feature mapping.
var methodToFeature = map[string]Feature{
	"GetProjectedCost":    FeatureProjectedCosts,
	"GetActualCost":       FeatureActualCosts,
	"GetRecommendations":  FeatureRecommendations,
	"GetCarbonFootprint":  FeatureCarbon,
	"PerformDryRun":       FeatureDryRun,
	"GetBudgetStatus":     FeatureBudgets,
	"GetBudgets":          FeatureBudgets,
	"GetBudgetHealth":     FeatureBudgets,
	"EvaluateBudgetAlert": FeatureBudgets,
}

// FeatureFromMethod returns the Feature corresponding to a gRPC method name.
// Returns the Feature and true if the method maps to a feature, empty and false otherwise.
func FeatureFromMethod(method string) (Feature, bool) {
	f, ok := methodToFeature[method]
	return f, ok
}

// DefaultFeatures returns the features assumed for plugins that don't report capabilities.
// Per the research document, ProjectedCosts and ActualCosts are assumed available.
func DefaultFeatures() []Feature {
	return []Feature{
		FeatureProjectedCosts,
		FeatureActualCosts,
	}
}
