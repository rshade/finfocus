package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCostCalculationPrecedence verifies the precedence logic in calculateCostForPeriod.
// This acts as a regression test for logic that might be subtle and missed by integration tests.
func TestCostCalculationPrecedence(t *testing.T) {
	// Case 1: DailyCosts present -> Sum of DailyCosts
	res1 := CostResult{
		DailyCosts: []float64{10, 20, 30},
		TotalCost:  500,  // Should be ignored
		Monthly:    1000, // Should be ignored
	}
	cost1 := calculateCostForPeriod(res1, GroupByDaily)
	assert.InDelta(t, 60.0, cost1, 1e-9, "Should use sum of DailyCosts")

	// Case 2: TotalCost present -> TotalCost
	res2 := CostResult{
		TotalCost: 500,
		Monthly:   1000, // Should be ignored
	}
	cost2 := calculateCostForPeriod(res2, GroupByDaily)
	assert.InDelta(t, 500.0, cost2, 1e-9, "Should use TotalCost over Monthly")

	// Case 3: Only Monthly present, GroupByDaily -> Monthly / 30.44
	res3 := CostResult{
		Monthly: 3044,
	}
	cost3 := calculateCostForPeriod(res3, GroupByDaily)
	assert.InDelta(t, 100.0, cost3, 0.01, "Should convert Monthly to Daily")

	// Case 4: Only Monthly present, GroupByMonthly -> Monthly
	cost4 := calculateCostForPeriod(res3, GroupByMonthly)
	assert.InDelta(t, 3044.0, cost4, 1e-9, "Should use Monthly as is")
}
