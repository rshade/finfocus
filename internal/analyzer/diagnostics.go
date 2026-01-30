package analyzer

import (
	"fmt"
	"sort"
	"strings"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/rs/zerolog/log"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/greenops"
)

// Policy pack and policy name constants for diagnostic messages.
const (
	policyPackName  = "finfocus"
	policyNameCost  = "cost-estimate"
	policyNameSum   = "stack-cost-summary"
	defaultCurrency = "USD"
)

// CostToDiagnostic converts a CostResult to an AnalyzeDiagnostic.
//
// This function creates a diagnostic message suitable for display in the
// Pulumi CLI output. The diagnostic includes:
//   - Per-resource cost information
//   - Source attribution (which plugin/spec provided the cost)
//   - Severity based on data availability
//
// Per FR-005, all diagnostics use ADVISORY enforcement level to ensure
// CostToDiagnostic converts an engine.CostResult into a Pulumi analysis diagnostic describing an estimated resource cost.
// It formats the cost message, assigns a default low severity, elevates severity to medium when Monthly is zero and Notes are present, and returns an AnalyzeDiagnostic populated with the cost policy name, policy pack name, provided URN, and policy pack version.
func CostToDiagnostic(
	cost engine.CostResult,
	urn string,
	version string,
) *pulumirpc.AnalyzeDiagnostic {
	message := formatCostMessage(cost)
	severity := pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW

	// Elevate severity if no cost data available but notes present
	// This indicates a fallback or partial data scenario
	if cost.Monthly == 0 && cost.Notes != "" {
		severity = pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM
	}

	return &pulumirpc.AnalyzeDiagnostic{
		PolicyName:        policyNameCost,
		PolicyPackName:    policyPackName,
		PolicyPackVersion: version,
		Description:       "Estimated resource cost",
		Message:           message,
		EnforcementLevel:  pulumirpc.EnforcementLevel_ADVISORY,
		Urn:               urn,
		Severity:          severity,
	}
}

// StackSummaryDiagnostic creates a stack-level cost summary diagnostic.
//
// This diagnostic provides an aggregated view of all resource costs:
//   - Total monthly cost across all resources
//   - Count of resources successfully analyzed
//   - Currency (defaults to USD)
//
// StackSummaryDiagnostic creates a stack-level AnalyzeDiagnostic that summarizes the total
// estimated monthly cost across the provided CostResult values.
//
// The `costs` slice is aggregated to compute the total monthly cost, the currency (defaults
// to "USD" if unspecified), and the count of resources with a positive monthly estimate.
// The `version` parameter is used as the PolicyPackVersion on the returned diagnostic.
//
// It returns an *pulumirpc.AnalyzeDiagnostic representing a stack-level cost summary
// (no URN), with EnforcementLevel set to ADVISORY and a low policy severity.
func StackSummaryDiagnostic(
	costs []engine.CostResult,
	version string,
) *pulumirpc.AnalyzeDiagnostic {
	var totalMonthly float64
	currency := defaultCurrency
	analyzed := 0

	for _, c := range costs {
		totalMonthly += c.Monthly
		if c.Currency != "" {
			currency = c.Currency
		}
		if c.Monthly > 0 {
			analyzed++
		}
	}

	message := fmt.Sprintf("Total Estimated Monthly Cost: $%.2f %s (%d resources analyzed)",
		totalMonthly, currency, analyzed)

	// Append recommendation summary if any recommendations exist
	recAgg := AggregateRecommendations(costs)
	if recAgg.Count > 0 {
		message += formatRecommendationSummary(recAgg)
	}

	return &pulumirpc.AnalyzeDiagnostic{
		PolicyName:        policyNameSum,
		PolicyPackName:    policyPackName,
		PolicyPackVersion: version,
		Description:       "Stack cost summary",
		Message:           message,
		EnforcementLevel:  pulumirpc.EnforcementLevel_ADVISORY,
		Severity:          pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW,
		// No URN - stack-level diagnostic
	}
}

// formatCostMessage formats a cost result into a human-readable message.
//
// Message formats:
//   - With cost: "Estimated Monthly Cost: $X.XX USD (source: adapter-name)"
//   - Zero cost with notes: Returns the notes directly
// appended prefixed by " | ".
func formatCostMessage(cost engine.CostResult) string {
	var message string
	switch {
	case cost.Monthly > 0:
		message = fmt.Sprintf("Estimated Monthly Cost: $%.2f %s (source: %s)",
			cost.Monthly, cost.Currency, cost.Adapter)
	case cost.Notes != "":
		message = cost.Notes
	default:
		message = "Unable to estimate cost"
	}

	// Append sustainability metrics if present (sorted for deterministic output - SC-003 fix)
	if len(cost.Sustainability) > 0 {
		var sustainParts []string
		// Sort keys for deterministic output
		keys := make([]string, 0, len(cost.Sustainability))
		for k := range cost.Sustainability {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			m := cost.Sustainability[k]
			if k == "gCO2e" {
				sustainParts = append(sustainParts, fmt.Sprintf("Carbon: %.2f %s", m.Value, m.Unit))
			} else {
				sustainParts = append(sustainParts, fmt.Sprintf("%s: %.2f %s", k, m.Value, m.Unit))
			}
		}

		if len(sustainParts) > 0 {
			message += fmt.Sprintf(" [%s]", getJoinedSustainability(sustainParts))
		}

		// Add carbon equivalencies if carbon_footprint is present
		carbonEquiv := getCarbonEquivalencyText(cost.Sustainability)
		if carbonEquiv != "" {
			message += " " + carbonEquiv
		}
	}

	// Append recommendations if present (follows sustainability pattern)
	if recStr := formatRecommendations(cost.Recommendations); recStr != "" {
		message += " | " + recStr
	}

	return message
}

// getJoinedSustainability joins the provided sustainability metric parts using ", " as the separator to produce a deterministic, human-readable summary string.
func getJoinedSustainability(parts []string) string {
	return strings.Join(parts, ", ")
}

// getCarbonEquivalencyText returns the compact equivalency text for carbon metrics.
// On calculation errors the function logs a warning.
func getCarbonEquivalencyText(sustainability map[string]engine.SustainabilityMetric) string {
	// Check canonical key first
	metric, ok := sustainability[greenops.CarbonMetricKey]
	if !ok {
		// Fallback to deprecated key
		metric, ok = sustainability[greenops.DeprecatedCarbonKey]
	}

	if !ok {
		return ""
	}

	// Calculate equivalencies
	input := greenops.CarbonInput{Value: metric.Value, Unit: metric.Unit}
	output, err := greenops.Calculate(input)
	if err != nil {
		log.Warn().Str("unit", metric.Unit).Err(err).
			Msg("carbon equivalency calculation failed")
		return ""
	}
	if output.IsEmpty {
		return ""
	}

	return output.CompactText
}

// maxRecommendationsToShow is the maximum number of recommendations to display
// before truncating with "and N more" indicator.
const maxRecommendationsToShow = 3

// formatRecommendation formats a single recommendation into a human-readable string.
//
// Format examples:
//   - With savings: "Right-sizing: Switch to t3.small (save $15.00/mo)"
//   - Without savings: "Review: Consider adjusting storage class"
func formatRecommendation(rec engine.Recommendation) string {
	// Base format: "Type: Description"
	result := fmt.Sprintf("%s: %s", rec.Type, rec.Description)

	// Append savings if present and non-zero
	if rec.EstimatedSavings > 0 {
		symbol := getCurrencySymbol(rec.Currency)
		result += fmt.Sprintf(" (save %s%.2f/mo)", symbol, rec.EstimatedSavings)
	}

	return result
}

// getCurrencySymbol returns the appropriate currency symbol for the given code.
// Returns "$" as default for unknown currencies.
func getCurrencySymbol(currency string) string {
	switch currency {
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	case "USD", "":
		return "$"
	default:
		return "$"
	}
}

// RecommendationAggregate holds aggregated recommendation statistics.
type RecommendationAggregate struct {
	Count           int
	TotalSavings    float64
	Currency        string
	MixedCurrencies bool
}

// formatRecommendationSummary formats the aggregate recommendation info for stack summary.
//
// Format examples:
//   - Same currency: " | 3 recommendations with $125.00/mo potential savings"
//   - Mixed currencies: " | 3 recommendations (mixed currencies)"
func formatRecommendationSummary(agg RecommendationAggregate) string {
	if agg.MixedCurrencies {
		return fmt.Sprintf(" | %d recommendations (mixed currencies)", agg.Count)
	}
	symbol := getCurrencySymbol(agg.Currency)
	return fmt.Sprintf(
		" | %d recommendations with %s%.2f/mo potential savings",
		agg.Count, symbol, agg.TotalSavings,
	)
}

// AggregateRecommendations calculates total recommendation count and savings.
//
// If recommendations have mixed currencies, MixedCurrencies is set to true
// and TotalSavings should not be displayed as a numeric value.
//
// Per the engine.Recommendation contract, Currency should be empty only if
// EstimatedSavings is zero. If a recommendation has savings without currency,
// this is treated as a mixed currency scenario since the currency is unknown.
func AggregateRecommendations(costs []engine.CostResult) RecommendationAggregate {
	var result RecommendationAggregate
	currencySet := make(map[string]bool)

	for _, cost := range costs {
		for _, rec := range cost.Recommendations {
			result.Count++
			result.TotalSavings += rec.EstimatedSavings

			// Validate currency/savings consistency per contract:
			// If savings > 0 but currency is empty, treat as mixed currencies
			// since the actual currency is unknown.
			if rec.EstimatedSavings > 0 && rec.Currency == "" {
				result.MixedCurrencies = true
				continue
			}

			if rec.Currency != "" {
				currencySet[rec.Currency] = true
			}
		}
	}

	// Skip currency detection if already marked as mixed
	if result.MixedCurrencies {
		return result
	}

	// Determine currency status
	switch len(currencySet) {
	case 0:
		result.Currency = defaultCurrency // Default
	case 1:
		for curr := range currencySet {
			result.Currency = curr
		}
	default:
		result.MixedCurrencies = true
	}

	return result
}

// formatRecommendations formats a slice of recommendations into a single string.
//
// If there are more than maxRecommendationsToShow (3), only the first 3 are shown
// with an "and N more" indicator.
//
// Returns empty string if recommendations is nil or empty.
//
// Format example:
//
//	"Recommendations: Right-sizing: Switch to t3.small (save $15.00/mo);
//	 Terminate: Remove idle instance (save $100.00/mo)"
func formatRecommendations(recommendations []engine.Recommendation) string {
	if len(recommendations) == 0 {
		return ""
	}

	// Filter to valid recommendations only (skip malformed)
	validRecs := filterValidRecommendations(recommendations)
	if len(validRecs) == 0 {
		return ""
	}

	var parts []string
	displayCount := len(validRecs)
	if displayCount > maxRecommendationsToShow {
		displayCount = maxRecommendationsToShow
	}

	for i := range displayCount {
		parts = append(parts, formatRecommendation(validRecs[i]))
	}

	// Add "and N more" indicator if truncated
	if len(validRecs) > maxRecommendationsToShow {
		remaining := len(validRecs) - maxRecommendationsToShow
		parts = append(parts, fmt.Sprintf("and %d more", remaining))
	}

	return "Recommendations: " + strings.Join(parts, "; ")
}

// isValidRecommendation returns true if the recommendation has valid Type, Description,
// and consistent EstimatedSavings/Currency values per the engine.Recommendation contract.
//
// Per contract:
//   - Type and Description must be non-empty
//   - Currency should be empty only if EstimatedSavings is zero
//   - EstimatedSavings should not be negative
func isValidRecommendation(rec engine.Recommendation) bool {
	if rec.Type == "" || rec.Description == "" {
		return false
	}

	// Per contract: Currency should be empty only if EstimatedSavings is zero
	if rec.EstimatedSavings > 0 && rec.Currency == "" {
		return false
	}

	// EstimatedSavings should not be negative
	if rec.EstimatedSavings < 0 {
		return false
	}

	return true
}

// filterValidRecommendations returns only valid recommendations per isValidRecommendation.
// Malformed recommendations (empty Type/Description, invalid currency/savings) are skipped.
func filterValidRecommendations(recs []engine.Recommendation) []engine.Recommendation {
	var valid []engine.Recommendation
	for _, rec := range recs {
		if isValidRecommendation(rec) {
			valid = append(valid, rec)
		}
		// Silently skip malformed recommendations - logging would require
		// importing zerolog/logging which adds coupling. If detailed logging
		// is needed, it should be done at the plugin boundary where the
		// recommendation data is received.
	}
	return valid
}

// WarningDiagnostic creates a warning-level diagnostic for error conditions.
//
// Use this function when cost calculation fails but the preview should
// continue. Examples include:
//   - Plugin timeout
//   - Network failures
//   - Unsupported resource types
//   - Invalid resource data
//
// Per FR-005, all diagnostics use ADVISORY enforcement to never block
// deployments in MVP mode.
func WarningDiagnostic(message, urn, version string) *pulumirpc.AnalyzeDiagnostic {
	return &pulumirpc.AnalyzeDiagnostic{
		PolicyName:        policyNameCost,
		PolicyPackName:    policyPackName,
		PolicyPackVersion: version,
		Description:       "Cost estimation warning",
		Message:           message,
		EnforcementLevel:  pulumirpc.EnforcementLevel_ADVISORY,
		Urn:               urn,
		Severity:          pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM,
	}
}