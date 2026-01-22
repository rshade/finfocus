package pagination

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

// Sorter defines the interface for sorting recommendations.
type Sorter interface {
	// Sort sorts a slice of recommendations by the specified field and order.
	Sort(recommendations []engine.Recommendation, field, order string) []engine.Recommendation
	// IsValidField checks if the given field name is valid for sorting.
	IsValidField(field string) bool
	// GetValidFields returns a list of valid field names for sorting.
	GetValidFields() []string
}

// RecommendationSorter implements Sorter for engine.Recommendation.
type RecommendationSorter struct {
	validFields map[string]bool
}

// NewRecommendationSorter creates a new RecommendationSorter with valid sort fields.
func NewRecommendationSorter() *RecommendationSorter {
	return &RecommendationSorter{
		validFields: map[string]bool{
			"savings":      true,
			"cost":         true,
			"name":         true,
			"resourceType": true,
			"provider":     true,
			"actionType":   true,
		},
	}
}

// IsValidField checks if the field is valid for sorting.
func (s *RecommendationSorter) IsValidField(field string) bool {
	return s.validFields[field]
}

// GetValidFields returns all valid sort fields.
func (s *RecommendationSorter) GetValidFields() []string {
	fields := make([]string, 0, len(s.validFields))
	for field := range s.validFields {
		fields = append(fields, field)
	}
	sort.Strings(fields) // Return in consistent order
	return fields
}

// Sort sorts recommendations by the specified field and order.
// Returns a new sorted slice; does not modify the original.
// If field is invalid, returns the original slice unchanged.
func (s *RecommendationSorter) Sort(
	recommendations []engine.Recommendation,
	field, order string,
) []engine.Recommendation {
	// Return early if field is invalid
	if !s.IsValidField(field) {
		return recommendations
	}

	// Make a copy to avoid modifying the original
	sorted := make([]engine.Recommendation, len(recommendations))
	copy(sorted, recommendations)

	// Sort using stable sort
	sort.SliceStable(sorted, func(i, j int) bool {
		// For descending order, swap i and j in comparisons to maintain stability
		if order == SortOrderDesc {
			i, j = j, i
		}

		switch field {
		case "savings":
			return sorted[i].EstimatedSavings < sorted[j].EstimatedSavings
		case "cost":
			// "cost" maps to EstimatedSavings since Recommendation has no separate cost field
			return sorted[i].EstimatedSavings < sorted[j].EstimatedSavings
		case "name":
			return sorted[i].ResourceID < sorted[j].ResourceID
		case "resourceType":
			return sorted[i].Type < sorted[j].Type
		case "provider":
			// Extract provider from ResourceID (format: "provider:service:type/id")
			providerI := extractProvider(sorted[i].ResourceID)
			providerJ := extractProvider(sorted[j].ResourceID)
			return providerI < providerJ
		case "actionType":
			return sorted[i].Type < sorted[j].Type
		default:
			return false
		}
	})

	return sorted
}

// extractProvider extracts the provider name from a resource ID.
// Expected format: "provider:service:type/id" or similar.
// Returns the first component before the first colon.
func extractProvider(resourceID string) string {
	parts := strings.Split(resourceID, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return resourceID
}

// ParseSortExpression parses a sort expression in "field:order" format.
// Supports:
//   - "field" - defaults to desc order
//   - "field:asc" - explicit ascending order
//   - "field:desc" - explicit descending order
//
// Returns field name, order, and error if parsing fails.
//
//nolint:nonamedreturns // Named returns improve readability for this multi-value function.
func ParseSortExpression(expr string) (field, order string, err error) {
	// Empty expression is an error
	if strings.TrimSpace(expr) == "" {
		return "", "", errors.New("empty sort expression")
	}

	parts := strings.Split(expr, ":")

	// Check for too many parts
	if len(parts) > sortPartsMax {
		return "", "", fmt.Errorf("invalid format: too many colons in %q", expr)
	}

	// Extract field
	field = strings.TrimSpace(parts[0])
	if field == "" {
		return "", "", errors.New("empty sort expression")
	}

	// Extract order (default to desc if not specified)
	if len(parts) == sortPartsMax {
		order = strings.ToLower(strings.TrimSpace(parts[1]))
	} else {
		order = "desc"
	}

	// Validate order
	if order != "asc" && order != "desc" {
		return "", "", fmt.Errorf("invalid sort order: %q (must be asc or desc)", order)
	}

	return field, order, nil
}
