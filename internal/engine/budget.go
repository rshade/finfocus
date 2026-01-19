package engine

import (
	"context"
	"regexp"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// currencyPattern is the regex pattern for valid ISO 4217 currency codes.
const currencyPattern = `^[A-Z]{3}$`

// FilterBudgets filters a list of budgets based on the provided criteria.
// It returns a new slice containing only the matching budgets.
// FilterBudgets returns the subset of budgets that satisfy the given BudgetFilter.
// If filter is nil, FilterBudgets returns the original budgets slice unchanged and preserves the original order.
func FilterBudgets(budgets []*pbc.Budget, filter *pbc.BudgetFilter) []*pbc.Budget {
	if filter == nil {
		return budgets
	}

	var filtered []*pbc.Budget
	for _, b := range budgets {
		if matchesBudgetFilter(b, filter) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// matchesBudgetFilter reports whether the given budget b satisfies all criteria specified in filter.
// Providers, Regions, and ResourceTypes are applied only if present in the filter: the budget must match at least one value in each of those lists (case-insensitive comparison). Tags are applied conjunctively: for each key/value pair in filter.Tags the budget's metadata must contain an entry "tag:<key>" equal to the corresponding value.
// The function reads region and resourceType from budget metadata keys "region" and "resourceType" respectively.
// Returns true if the budget meets every specified criterion in the filter, false otherwise.
func matchesBudgetFilter(b *pbc.Budget, filter *pbc.BudgetFilter) bool {
	// Provider (OR logic)
	if len(filter.GetProviders()) > 0 {
		if !matchStringSlice(b.GetSource(), filter.GetProviders()) {
			return false
		}
	}

	// Region (OR logic) - checks Metadata["region"]
	if len(filter.GetRegions()) > 0 {
		region := getMetadataValue(b, "region")
		if !matchStringSlice(region, filter.GetRegions()) {
			return false
		}
	}

	// ResourceType (OR logic) - checks Metadata["resourceType"]
	if len(filter.GetResourceTypes()) > 0 {
		resType := getMetadataValue(b, "resourceType")
		if !matchStringSlice(resType, filter.GetResourceTypes()) {
			return false
		}
	}

	// Tags (AND logic) - checks Metadata["tag:<key>"]
	if len(filter.GetTags()) > 0 {
		for key, val := range filter.GetTags() {
			// Construct the tag key as stored in metadata (e.g., "tag:env")
			metaKey := "tag:" + key
			metaVal := getMetadataValue(b, metaKey)
			if metaVal != val {
				return false
			}
		}
	}

	return true
}

// CalculateBudgetSummary computes a BudgetSummary for the given budgets.
// It sets TotalBudgets to the number of provided budgets and increments
// the appropriate health counters (BudgetsOk, BudgetsWarning, BudgetsCritical,
// BudgetsExceeded) based on each budget's status.Health. Budgets with a nil
// status or a health of UNSPECIFIED are logged and excluded from the health
// counts. It returns a pointer to the populated BudgetSummary.
func CalculateBudgetSummary(ctx context.Context, budgets []*pbc.Budget) *pbc.BudgetSummary {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "CalculateBudgetSummary").
		Logger()

	summary := &pbc.BudgetSummary{
		TotalBudgets: int32(len(budgets)), //nolint:gosec // length is bounded by practical budget limits
	}

	for _, b := range budgets {
		status := b.GetStatus()
		if status == nil || status.GetHealth() == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED {
			logger.Warn().Str("budget_id", b.GetId()).Msg("Budget missing health status, excluded from health metrics")
			continue
		}

		switch status.GetHealth() {
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED:
			// Already handled above, no-op for exhaustive switch
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK:
			summary.BudgetsOk++
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING:
			summary.BudgetsWarning++
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL:
			summary.BudgetsCritical++
		case pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED:
			summary.BudgetsExceeded++
		}
	}

	return summary
}

// validateCurrency reports whether currency is a three-letter ISO 4217 currency code (uppercase A–Z).
// It returns true if the input matches the required pattern, false otherwise.
func validateCurrency(currency string) bool {
	matched, _ := regexp.MatchString(currencyPattern, currency)
	return matched
}

// matchStringSlice reports whether target is present in candidates using a case-insensitive comparison.
// It returns true if any element of candidates equals target ignoring letter case, and false otherwise.
func matchStringSlice(target string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(target, c) {
			return true
		}
	}
	return false
}

// getMetadataValue returns the value for the given key from the budget's metadata.
// If the metadata map is nil or the key does not exist, it returns the empty string.
func getMetadataValue(b *pbc.Budget, key string) string {
	metadata := b.GetMetadata()
	if metadata == nil {
		return ""
	}
	return metadata[key]
}
