package engine

import (
	"context"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// CalculateForecastedSpend predicts end-of-period spending using linear extrapolation.
// Uses current time for calculation.
func CalculateForecastedSpend(currentSpend float64, periodStart time.Time, periodEnd time.Time) float64 {
	return CalculateForecastedSpendAt(currentSpend, periodStart, periodEnd, time.Now())
}

// CalculateForecastedSpendAt predicts end-of-period spending relative to a specific time.
// Exposed for testing.
//
// Formula: forecastedSpend = (currentSpend / elapsedDuration) * totalDuration
//
// Edge cases:
//   - Period not started (now < periodStart): returns currentSpend
//   - No elapsed time: returns currentSpend (avoids division by zero)
//   - Zero current spend: returns 0
func CalculateForecastedSpendAt(
	currentSpend float64,
	periodStart time.Time,
	periodEnd time.Time,
	now time.Time,
) float64 {
	if currentSpend == 0 {
		return 0
	}

	if now.Before(periodStart) {
		return currentSpend
	}

	totalDuration := periodEnd.Sub(periodStart)
	elapsedDuration := now.Sub(periodStart)

	if elapsedDuration <= 0 {
		return currentSpend
	}

	// If period has ended, forecast matches actual
	if elapsedDuration >= totalDuration {
		return currentSpend
	}

	// Linear extrapolation
	// Rate = spend / elapsed
	// Forecast = Rate * total
	rate := currentSpend / float64(elapsedDuration)
	forecasted := rate * float64(totalDuration)

	return forecasted
}

// CalculateForecastedPercentage calculates forecasted utilization percentage.
// Returns 0 if budgetLimit <= 0.
func CalculateForecastedPercentage(forecastedSpend float64, budgetLimit float64) float64 {
	if budgetLimit <= 0 {
		return 0
	}
	return (forecastedSpend / budgetLimit) * PercentageMultiplier
}

// UpdateBudgetForecast updates a budget's status with calculated forecast.
// Modifies budget.Status in place. Creates Status if nil.
func UpdateBudgetForecast(ctx context.Context, budget *pbc.Budget, periodStart, periodEnd time.Time) {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "UpdateBudgetForecast").
		Logger()

	if budget == nil {
		return
	}

	if budget.GetStatus() == nil {
		budget.Status = &pbc.BudgetStatus{}
	}

	currentSpend := budget.GetStatus().GetCurrentSpend()
	limit := 0.0
	if amt := budget.GetAmount(); amt != nil {
		limit = amt.GetLimit()
	}

	forecasted := CalculateForecastedSpend(currentSpend, periodStart, periodEnd)
	percentage := CalculateForecastedPercentage(forecasted, limit)

	budget.Status.ForecastedSpend = forecasted
	budget.Status.PercentageForecasted = percentage

	logger.Debug().
		Str("budget_id", budget.GetId()).
		Float64("current_spend", currentSpend).
		Float64("forecasted_spend", forecasted).
		Float64("percentage_forecasted", percentage).
		Msg("updated budget forecast")
}
