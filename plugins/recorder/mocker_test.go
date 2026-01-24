package recorder

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func mockerTestLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).Level(zerolog.Disabled)
}

// T027: Unit test for Mocker.GenerateProjectedCost() range.
func TestMocker_GenerateProjectedCost_Range(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	// Generate many costs and verify they're all within range
	for i := 0; i < 1000; i++ {
		cost := mocker.GenerateProjectedCost()
		assert.GreaterOrEqual(t, cost, MinProjectedCost,
			"cost should be >= %f", MinProjectedCost)
		assert.LessOrEqual(t, cost, MaxProjectedCost,
			"cost should be <= %f", MaxProjectedCost)
	}
}

// T028: Unit test for Mocker.GenerateActualCost() range.
func TestMocker_GenerateActualCost_Range(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	// Generate many costs and verify they're all within range
	for i := 0; i < 1000; i++ {
		cost := mocker.GenerateActualCost()
		assert.GreaterOrEqual(t, cost, MinActualCost,
			"cost should be >= %f", MinActualCost)
		assert.LessOrEqual(t, cost, MaxActualCost,
			"cost should be <= %f", MaxActualCost)
	}
}

// T029: Unit test for mock response structure validity.
func TestMocker_CreateProjectedCostResponse_Structure(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	resp := mocker.CreateProjectedCostResponse()

	require.NotNil(t, resp)
	assert.Greater(t, resp.GetCostPerMonth(), float64(0))
	assert.Greater(t, resp.GetUnitPrice(), float64(0))
	assert.Equal(t, "USD", resp.GetCurrency())
	assert.Contains(t, resp.GetBillingDetail(), "Mock cost")
	assert.Contains(t, resp.GetBillingDetail(), "recorder plugin")

	// Verify unit price is derived from monthly cost
	expectedHourly := resp.GetCostPerMonth() / HoursPerMonth
	assert.InDelta(t, expectedHourly, resp.GetUnitPrice(), 0.0001)
}

func TestMocker_CreateActualCostResponse_Structure(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	resp := mocker.CreateActualCostResponse()

	require.NotNil(t, resp)
	require.Len(t, resp.GetResults(), 1)

	result := resp.GetResults()[0]
	assert.Equal(t, "recorder-mock", result.GetSource())
	assert.Greater(t, result.GetCost(), float64(0))
}

func TestMocker_CreateEstimateCostResponse_Structure(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	resp := mocker.CreateEstimateCostResponse()

	require.NotNil(t, resp)
	assert.Greater(t, resp.GetCostMonthly(), float64(0))
	assert.Equal(t, "USD", resp.GetCurrency())
}

func TestMocker_CostDistribution(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	// Generate costs and check distribution
	// With log-scale, we should have more small costs than large ones
	var smallCount, largeCount int
	threshold := 100.0

	for i := 0; i < 1000; i++ {
		cost := mocker.GenerateProjectedCost()
		if cost < threshold {
			smallCount++
		} else {
			largeCount++
		}
	}

	// Log-scale should produce more small values
	// This is a probabilistic test, but with 1000 samples it should be reliable
	t.Logf("Cost distribution: %d small (<%f), %d large", smallCount, threshold, largeCount)
	assert.Greater(t, smallCount, largeCount,
		"log-scale distribution should produce more small costs than large")
}

func TestMocker_Randomness(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	// Generate several costs and ensure they're not all the same
	costs := make(map[float64]bool)
	for i := 0; i < 100; i++ {
		costs[mocker.GenerateProjectedCost()] = true
	}

	// Should have many unique values - with rounding to cents, some duplicates are possible
	// but we should still have a good variety
	assert.Greater(t, len(costs), 50, "should generate diverse random costs")
}

func TestMockerConstants(t *testing.T) {
	// Verify constants are reasonable
	assert.Equal(t, 0.01, MinProjectedCost)
	assert.Equal(t, 1000.0, MaxProjectedCost)
	assert.Equal(t, 0.001, MinActualCost)
	assert.Equal(t, 100.0, MaxActualCost)
	assert.Equal(t, 730.0, HoursPerMonth)
}

func TestMocker_GenerateRecommendations_Count(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	// Generate recommendations multiple times and verify count is in expected range
	for i := 0; i < 100; i++ {
		recs := mocker.GenerateRecommendations()
		assert.Greater(t, len(recs), 0, "should generate at least 1 recommendation")
		assert.LessOrEqual(
			t,
			len(recs),
			maxRecommendationsPerRequest,
			"should generate at most maxRecommendationsPerRequest recommendations",
		)
	}
}

func TestMocker_GenerateRecommendations_Structure(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	recs := mocker.GenerateRecommendations()
	require.Greater(t, len(recs), 0)

	for _, rec := range recs {
		assert.NotEmpty(t, rec.GetId(), "recommendation should have an ID")
		assert.Equal(t, pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE, rec.GetActionType())
		assert.NotEmpty(t, rec.GetDescription())
		assert.NotNil(t, rec.GetImpact(), "recommendation should have impact")
		assert.Greater(t, rec.GetImpact().GetEstimatedSavings(), float64(0))
		assert.Equal(t, "USD", rec.GetImpact().GetCurrency())
	}
}

func TestMocker_CreateRecommendationsResponse(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	resp := mocker.CreateRecommendationsResponse()

	require.NotNil(t, resp)
	require.Greater(t, len(resp.GetRecommendations()), 0)

	for _, rec := range resp.GetRecommendations() {
		assert.NotEmpty(t, rec.GetId())
		assert.Equal(t, pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE, rec.GetActionType())
		assert.NotEmpty(t, rec.GetDescription())
		assert.Greater(t, rec.GetImpact().GetEstimatedSavings(), float64(0))
	}
}

func TestMocker_RecommendationSavings(t *testing.T) {
	mocker := NewMocker(mockerTestLogger())

	recs := mocker.GenerateRecommendations()
	for _, rec := range recs {
		savings := rec.GetImpact().GetEstimatedSavings()
		// Savings should be derived from projected cost generation
		assert.GreaterOrEqual(t, savings, MinProjectedCost)
		assert.LessOrEqual(t, savings, MaxProjectedCost)
	}
}
