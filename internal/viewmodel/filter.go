package viewmodel

import (
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

// FilterOverviewRows returns the overview rows whose URN or type contains the
// filter text, case-insensitively. An empty filter returns a copy of the
// input so callers may sort the result without reordering the source slice.
func FilterOverviewRows(rows []engine.OverviewRowResult, filterText string) []engine.OverviewRowResult {
	if filterText == "" {
		filtered := make([]engine.OverviewRowResult, len(rows))
		copy(filtered, rows)
		return filtered
	}

	query := strings.ToLower(filterText)
	filtered := []engine.OverviewRowResult{}
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row.URN), query) ||
			strings.Contains(strings.ToLower(row.Type), query) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

// FilterCostResults returns the cost results whose resource ID or resource
// type contains the filter text, case-insensitively. An empty filter returns
// the input slice unchanged.
func FilterCostResults(results []engine.CostResult, filterText string) []engine.CostResult {
	if filterText == "" {
		return results
	}

	var filtered []engine.CostResult
	query := strings.ToLower(filterText)
	for _, r := range results {
		if strings.Contains(strings.ToLower(r.ResourceType), query) ||
			strings.Contains(strings.ToLower(r.ResourceID), query) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// FilterRecommendations returns the recommendations whose resource ID, action
// type, or description contains the filter text, case-insensitively. An empty
// filter returns the input slice unchanged.
func FilterRecommendations(recs []engine.Recommendation, filterText string) []engine.Recommendation {
	if filterText == "" {
		return recs
	}

	var filtered []engine.Recommendation
	query := strings.ToLower(filterText)
	for _, r := range recs {
		if strings.Contains(strings.ToLower(r.ResourceID), query) ||
			strings.Contains(strings.ToLower(r.Type), query) ||
			strings.Contains(strings.ToLower(r.Description), query) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
