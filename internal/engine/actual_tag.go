package engine

import (
	"fmt"
	"strings"
)

// ParseActualTagFilter parses the CLI's actual-cost key=value tag selector.
func ParseActualTagFilter(tag string) (map[string]string, error) {
	if tag == "" {
		return map[string]string{}, nil
	}
	parts := strings.Split(tag, "=")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid tag filter %q: expected key=value", tag)
	}
	return map[string]string{parts[0]: parts[1]}, nil
}

// SumActualCosts returns the total actual cost using the CLI aggregation rule.
func SumActualCosts(results []CostResult) float64 {
	total := 0.0
	for _, result := range results {
		total += result.TotalCost
	}
	return total
}
