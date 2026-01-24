package recorder

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/rs/zerolog"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// Cost range constants for mock generation.
const (
	// MinProjectedCost is the minimum projected cost: $0.01 per month.
	MinProjectedCost = 0.01
	// MaxProjectedCost is the maximum projected cost: $1000 per month.
	MaxProjectedCost = 1000.0

	// MinActualCost is the minimum actual cost: $0.001 per day.
	MinActualCost = 0.001
	// MaxActualCost is the maximum actual cost: $100 per day.
	MaxActualCost = 100.0

	// HoursPerMonth is the standard hours per month for cost conversions.
	HoursPerMonth = 730.0

	// centsMultiplier is the multiplier for rounding costs to cents.
	centsMultiplier = 100
	// milliMultiplier is the multiplier for rounding costs to 3 decimal places.
	milliMultiplier = 1000

	// maxRecommendationsPerRequest is the maximum number of recommendations to generate per request.
	maxRecommendationsPerRequest = 5
)

// Mocker generates randomized but valid cost responses for testing.
type Mocker struct {
	logger zerolog.Logger
}

// NewMocker creates a new Mocker instance.
func NewMocker(logger zerolog.Logger) *Mocker {
	return &Mocker{
		logger: logger.With().Str("component", "mocker").Logger(),
	}
}

// GenerateProjectedCost generates a randomized monthly cost.
// Uses log-scale distribution for realistic cost spread.
func (m *Mocker) GenerateProjectedCost() float64 {
	// Log-scale distribution: more small costs, fewer large costs
	//nolint:gosec // G404: math/rand/v2 is appropriate for non-cryptographic mock data generation
	cost := MinProjectedCost * math.Pow(MaxProjectedCost/MinProjectedCost, rand.Float64())
	return math.Round(cost*centsMultiplier) / centsMultiplier // Round to cents
}

// GenerateActualCost generates a randomized daily cost.
// Uses log-scale distribution for realistic cost spread.
func (m *Mocker) GenerateActualCost() float64 {
	//nolint:gosec // G404: math/rand/v2 is appropriate for non-cryptographic mock data generation
	cost := MinActualCost * math.Pow(MaxActualCost/MinActualCost, rand.Float64())
	return math.Round(cost*milliMultiplier) / milliMultiplier // Round to 3 decimal places
}

// CreateProjectedCostResponse creates a mock GetProjectedCostResponse.
func (m *Mocker) CreateProjectedCostResponse() *pbc.GetProjectedCostResponse {
	monthlyCost := m.GenerateProjectedCost()
	hourlyCost := monthlyCost / HoursPerMonth

	m.logger.Debug().
		Float64("monthly_cost", monthlyCost).
		Float64("hourly_cost", hourlyCost).
		Msg("generated mock projected cost")

	return &pbc.GetProjectedCostResponse{
		CostPerMonth:  monthlyCost,
		UnitPrice:     hourlyCost,
		Currency:      "USD",
		BillingDetail: fmt.Sprintf("Mock cost: $%.2f/month (recorder plugin)", monthlyCost),
	}
}

// CreateActualCostResponse creates a mock GetActualCostResponse.
func (m *Mocker) CreateActualCostResponse() *pbc.GetActualCostResponse {
	cost := m.GenerateActualCost()

	m.logger.Debug().
		Float64("cost", cost).
		Msg("generated mock actual cost")

	return &pbc.GetActualCostResponse{
		Results: []*pbc.ActualCostResult{
			{
				Source: "recorder-mock",
				Cost:   cost,
			},
		},
	}
}

// CreateEstimateCostResponse creates a mock EstimateCostResponse.
func (m *Mocker) CreateEstimateCostResponse() *pbc.EstimateCostResponse {
	monthlyCost := m.GenerateProjectedCost()

	m.logger.Debug().
		Float64("estimated_cost", monthlyCost).
		Msg("generated mock estimated cost")

	return &pbc.EstimateCostResponse{
		CostMonthly: monthlyCost,
		Currency:    "USD",
	}
}

// GenerateRecommendations generates a randomized list of recommendations.
func (m *Mocker) GenerateRecommendations() []*pbc.Recommendation {
	// Generate 1 to maxRecommendationsPerRequest recommendations
	//nolint:gosec // G404: math/rand/v2 is appropriate for non-cryptographic mock data generation
	count := 1 + rand.IntN(maxRecommendationsPerRequest)
	recs := make([]*pbc.Recommendation, count)
	for i := range count {
		savings := m.GenerateProjectedCost()
		recs[i] = &pbc.Recommendation{
			Id:          fmt.Sprintf("mock-rec-%d", i),
			ActionType:  pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
			Description: "Mock recommendation from recorder plugin",
			Impact: &pbc.RecommendationImpact{
				EstimatedSavings: savings,
				Currency:         "USD",
			},
		}
	}
	return recs
}

// CreateRecommendationsResponse creates a mock GetRecommendationsResponse.
func (m *Mocker) CreateRecommendationsResponse() *pbc.GetRecommendationsResponse {
	recs := m.GenerateRecommendations()
	m.logger.Debug().Int("count", len(recs)).Msg("generated mock recommendations")
	return &pbc.GetRecommendationsResponse{
		Recommendations: recs,
	}
}
