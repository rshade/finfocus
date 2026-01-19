package engine

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

var currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

// FilterBudgets filters a list of budgets based on the provided criteria.
// It returns a new slice containing only the matching budgets.
// If the filter is nil or empty, all budgets are returned.
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

// CalculateBudgetSummary aggregates health metrics from the provided list of budgets.
func CalculateBudgetSummary(budgets []*pbc.Budget) *pbc.BudgetSummary {
	summary := &pbc.BudgetSummary{
		TotalBudgets: int32(len(budgets)), //nolint:gosec // length is bounded by practical budget limits
	}

	for _, b := range budgets {
		status := b.GetStatus()
		if status == nil || status.GetHealth() == pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED {
			log.Warn().Str("budget_id", b.GetId()).Msg("Budget missing health status, excluded from health metrics")
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

// validateCurrency checks if the currency code matches ISO 4217 format (3 uppercase letters).
func validateCurrency(currency string) bool {
	return currencyRegex.MatchString(currency)
}

// matchStringSlice checks if target exists in the slice (case-insensitive).
func matchStringSlice(target string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(target, c) {
			return true
		}
	}
	return false
}

// getMetadataValue safely retrieves a value from the budget's metadata.
func getMetadataValue(b *pbc.Budget, key string) string {
	metadata := b.GetMetadata()
	if metadata == nil {
		return ""
	}
	return metadata[key]
}
