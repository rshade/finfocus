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

// ValidFeatures returns all supported Feature values (ProjectedCosts, ActualCosts,
// Recommendations, Carbon, DryRun, Budgets).
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
// IsValidFeature reports whether name exactly matches one of the valid Feature values.
// Comparison is case-sensitive; it returns true if a match is found and false otherwise.
func IsValidFeature(name string) bool {
	for _, f := range ValidFeatures() {
		if string(f) == name {
			return true
		}
	}
	return false
}

// ValidFeatureNames returns the string names of all valid features.
// ValidFeatureNames returns the string names of all supported Feature values.
// The returned slice preserves the order produced by ValidFeatures.
func ValidFeatureNames() []string {
	features := ValidFeatures()
	names := make([]string, len(features))
	for i, f := range features {
		names[i] = string(f)
	}
	return names
}

// ParseFeature parses a string into a Feature.
// ParseFeature parses s into a Feature if it matches a known feature name.
// It returns the parsed Feature and true when s exactly matches a valid feature
// name (case-sensitive), or the empty Feature and false otherwise.
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
// FeatureFromMethod looks up the Feature associated with a gRPC method name.
// It returns the Feature and true if a mapping exists; otherwise the zero Feature and false.
func FeatureFromMethod(method string) (Feature, bool) {
	f, ok := methodToFeature[method]
	return f, ok
}

// DefaultFeatures returns the features assumed for plugins that don't report capabilities.
// DefaultFeatures returns the default feature set assumed for plugins that do not report capabilities.
// The default set includes FeatureProjectedCosts and FeatureActualCosts.
func DefaultFeatures() []Feature {
	return []Feature{
		FeatureProjectedCosts,
		FeatureActualCosts,
	}
}
