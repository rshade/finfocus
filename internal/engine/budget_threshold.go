package engine

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// Default threshold percentages.
const (
	DefaultThreshold50  = 50.0
	DefaultThreshold80  = 80.0
	DefaultThreshold100 = 100.0
)

// DefaultThresholds returns the standard thresholds to apply when none configured.
// Returns: 50% ACTUAL, 80% ACTUAL, 100% ACTUAL.
func DefaultThresholds() []*pbc.BudgetThreshold {
	return []*pbc.BudgetThreshold{
		{
			Percentage: DefaultThreshold50,
			Type:       pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL,
		},
		{
			Percentage: DefaultThreshold80,
			Type:       pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL,
		},
		{
			Percentage: DefaultThreshold100,
			Type:       pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL,
		},
	}
}

// ApplyDefaultThresholds adds default thresholds to a budget if none exist.
//
// This function MUTATES the input budget in place by setting budget.Thresholds
// to DefaultThresholds() when the budget has no thresholds defined.
// The same pointer is returned for convenience (allows chaining).
//
// Returns nil if budget is nil.
// Returns the same budget pointer (unchanged) if thresholds already exist.
// Returns the same budget pointer (modified) if defaults were applied.
func ApplyDefaultThresholds(budget *pbc.Budget) *pbc.Budget {
	if budget == nil {
		return nil
	}
	if len(budget.GetThresholds()) == 0 {
		budget.Thresholds = DefaultThresholds()
	}
	return budget
}

// EvaluateThresholds checks which thresholds have been triggered.
//
// Behavior:
//   - Evaluates against current spend for ACTUAL thresholds
//   - Evaluates against forecasted spend for FORECASTED thresholds
//   - Sets Triggered=true and TriggeredAt for crossed thresholds
//   - Returns all thresholds with updated triggered status
//   - Preserves existing TriggeredAt timestamp if already triggered
func EvaluateThresholds(
	ctx context.Context,
	budget *pbc.Budget,
	currentSpend float64,
	forecastedSpend float64,
) []ThresholdEvaluationResult {
	if budget == nil {
		return nil
	}

	limit := 0.0
	if amt := budget.GetAmount(); amt != nil {
		limit = amt.GetLimit()
	}

	if limit <= 0 {
		return nil // Cannot evaluate thresholds without a limit
	}

	var results []ThresholdEvaluationResult
	for _, t := range budget.GetThresholds() {
		results = append(results, evaluateSingleThreshold(ctx, budget.GetId(), t, limit, currentSpend, forecastedSpend))
	}

	return results
}

func evaluateSingleThreshold(
	ctx context.Context,
	budgetID string,
	t *pbc.BudgetThreshold,
	limit float64,
	currentSpend float64,
	forecastedSpend float64,
) ThresholdEvaluationResult {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "evaluateSingleThreshold").
		Logger()

	isTriggered := false
	spendType := "actual"

	var utilization float64
	if t.GetType() == pbc.ThresholdType_THRESHOLD_TYPE_FORECASTED {
		utilization = (forecastedSpend / limit) * PercentageMultiplier
		spendType = "forecasted"
	} else {
		utilization = (currentSpend / limit) * PercentageMultiplier
	}

	if utilization >= t.GetPercentage() {
		isTriggered = true
	}

	result := ThresholdEvaluationResult{
		Threshold: t,
		Triggered: isTriggered,
		SpendType: spendType,
	}

	if isTriggered {
		handleTriggeredThreshold(logger, budgetID, t, &result, spendType, utilization)
	} else if t.GetTriggered() {
		// Reset if no longer triggered (e.g. limit increased)
		t.Triggered = false
		t.TriggeredAt = nil
	}

	return result
}

func handleTriggeredThreshold(
	logger zerolog.Logger,
	budgetID string,
	t *pbc.BudgetThreshold,
	result *ThresholdEvaluationResult,
	spendType string,
	utilization float64,
) {
	// If already triggered in the input, preserve timestamp
	if t.GetTriggered() && t.GetTriggeredAt() != nil {
		result.TriggeredAt = t.GetTriggeredAt().AsTime()
	} else {
		// Newly triggered
		result.TriggeredAt = time.Now()
	}

	// Update the input threshold object as well (side effect, useful for persistence)
	if !t.GetTriggered() {
		t.Triggered = true
		t.TriggeredAt = timestamppb.New(result.TriggeredAt)

		logger.Debug().
			Str("budget_id", budgetID).
			Float64("threshold", t.GetPercentage()).
			Str("type", spendType).
			Float64("utilization", utilization).
			Msg("budget threshold triggered")
	} else if t.GetTriggeredAt() == nil {
		// Backfill timestamp for already-triggered thresholds missing timestamp
		t.TriggeredAt = timestamppb.New(result.TriggeredAt)
	}
}
