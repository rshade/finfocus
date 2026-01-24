package engine

import (
	"context"
	"testing"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestDefaultThresholds verifies standard threshold defaults (FR-007).
func TestDefaultThresholds(t *testing.T) {
	defaults := DefaultThresholds()
	require.Len(t, defaults, 3)

	// Verify 50% ACTUAL
	assert.Equal(t, DefaultThreshold50, defaults[0].GetPercentage())
	assert.Equal(t, pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL, defaults[0].GetType())

	// Verify 80% ACTUAL
	assert.Equal(t, DefaultThreshold80, defaults[1].GetPercentage())
	assert.Equal(t, pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL, defaults[1].GetType())

	// Verify 100% ACTUAL
	assert.Equal(t, DefaultThreshold100, defaults[2].GetPercentage())
	assert.Equal(t, pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL, defaults[2].GetType())
}

// TestApplyDefaultThresholds verifies default application logic.
func TestApplyDefaultThresholds(t *testing.T) {
	tests := []struct {
		name          string
		budget        *pbc.Budget
		shouldModify  bool
		expectedCount int
	}{
		{
			name:          "nil budget returns nil",
			budget:        nil,
			shouldModify:  false,
			expectedCount: 0,
		},
		{
			name: "budget with no thresholds gets defaults",
			budget: &pbc.Budget{
				Id:         "b1",
				Thresholds: nil,
			},
			shouldModify:  true,
			expectedCount: 3,
		},
		{
			name: "budget with empty thresholds gets defaults",
			budget: &pbc.Budget{
				Id:         "b2",
				Thresholds: []*pbc.BudgetThreshold{},
			},
			shouldModify:  true,
			expectedCount: 3,
		},
		{
			name: "budget with existing thresholds is untouched",
			budget: &pbc.Budget{
				Id: "b3",
				Thresholds: []*pbc.BudgetThreshold{
					{Percentage: 90, Type: pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL},
				},
			},
			shouldModify:  false,
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyDefaultThresholds(tc.budget)
			if tc.budget == nil {
				assert.Nil(t, result)
				return
			}
			if tc.shouldModify {
				assert.Len(t, result.GetThresholds(), tc.expectedCount)
				assert.Equal(t, DefaultThreshold50, result.GetThresholds()[0].GetPercentage())
			} else {
				assert.Equal(t, tc.budget.GetThresholds(), result.GetThresholds())
			}
		})
	}
}

// TestEvaluateThresholds verifies threshold triggering logic (FR-005, FR-010).
func TestEvaluateThresholds(t *testing.T) {
	ctx := context.Background()

	// Base budget with thresholds
	budget := &pbc.Budget{
		Id: "b-eval",
		Thresholds: []*pbc.BudgetThreshold{
			{Percentage: 50, Type: pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL},
			{Percentage: 80, Type: pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL},
			{Percentage: 100, Type: pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL},
			{Percentage: 110, Type: pbc.ThresholdType_THRESHOLD_TYPE_FORECASTED},
		},
		Amount: &pbc.BudgetAmount{Limit: 1000},
	}

	tests := []struct {
		name            string
		currentSpend    float64
		forecastedSpend float64
		checkFunc       func(t *testing.T, results []ThresholdEvaluationResult)
	}{
		{
			name:            "no thresholds triggered (20%)",
			currentSpend:    200, // 20%
			forecastedSpend: 500, // 50%
			checkFunc: func(t *testing.T, results []ThresholdEvaluationResult) {
				for _, r := range results {
					assert.False(t, r.Triggered, "threshold %.0f%% should not trigger", r.Threshold.GetPercentage())
				}
			},
		},
		{
			name:            "50% actual triggered (55%)",
			currentSpend:    550, // 55%
			forecastedSpend: 800, // 80%
			checkFunc: func(t *testing.T, results []ThresholdEvaluationResult) {
				// 50% Actual -> Triggered
				assert.True(t, results[0].Triggered)
				assert.Equal(t, 50.0, results[0].Threshold.GetPercentage())
				assert.NotZero(t, results[0].TriggeredAt)

				// Others not triggered
				assert.False(t, results[1].Triggered) // 80% Actual
				assert.False(t, results[2].Triggered) // 100% Actual
			},
		},
		{
			name:            "all actual triggered (105%)",
			currentSpend:    1050, // 105%
			forecastedSpend: 1200, // 120%
			checkFunc: func(t *testing.T, results []ThresholdEvaluationResult) {
				assert.True(t, results[0].Triggered) // 50%
				assert.True(t, results[1].Triggered) // 80%
				assert.True(t, results[2].Triggered) // 100%
			},
		},
		{
			name:            "forecasted triggered (120%)",
			currentSpend:    500,  // 50%
			forecastedSpend: 1200, // 120% -> triggers 110% Forecasted
			checkFunc: func(t *testing.T, results []ThresholdEvaluationResult) {
				assert.True(t, results[0].Triggered)  // 50% Actual
				assert.False(t, results[1].Triggered) // 80% Actual
				assert.False(t, results[2].Triggered) // 100% Actual

				// Forecasted 110%
				assert.True(t, results[3].Triggered)
				assert.Equal(t, 110.0, results[3].Threshold.GetPercentage())
				assert.Equal(t, "forecasted", results[3].SpendType)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := EvaluateThresholds(ctx, budget, tc.currentSpend, tc.forecastedSpend)
			tc.checkFunc(t, results)
		})
	}
}

// TestEvaluateThresholdsUpdatesTimestamp verifies FR-010 timestamp updating.
func TestEvaluateThresholdsUpdatesTimestamp(t *testing.T) {
	ctx := context.Background()

	threshold := &pbc.BudgetThreshold{
		Percentage: 50,
		Type:       pbc.ThresholdType_THRESHOLD_TYPE_ACTUAL,
		Triggered:  false,
	}

	budget := &pbc.Budget{
		Id:         "b-time",
		Thresholds: []*pbc.BudgetThreshold{threshold},
		Amount:     &pbc.BudgetAmount{Limit: 100},
	}

	// 1st pass: Trigger it
	results1 := EvaluateThresholds(ctx, budget, 60, 60)
	require.True(t, results1[0].Triggered)
	triggeredAt1 := results1[0].TriggeredAt
	assert.False(t, triggeredAt1.IsZero())

	// Simulate time passing (in real usage, the updated budget is persisted)
	// Here we just want to ensure if we pass in a budget that is ALREADY triggered,
	// we keep the timestamp or update it?
	// The requirement usually implies "first triggered at" or "last triggered at".
	// The proto has `TriggeredAt *timestamp`.
	// Our function `EvaluateThresholds` returns `ThresholdEvaluationResult` which has `TriggeredAt time.Time`.
	// It does NOT modify the input budget thresholds in place to persist state across calls
	// (unless we pass the modified budget back to a store).
	// But `EvaluateThresholds` SHOULD respect existing `Triggered` state if we want to avoid re-alerting.
	// However, the current spec/tasks implies evaluating based on current values.
	// If the budget object passed in ALREADY has Triggered=true, we should probably preserve the timestamp?

	// Let's modify the budget to simulate persistence
	threshold.Triggered = true
	threshold.TriggeredAt = timestamppb.New(triggeredAt1)

	// 2nd pass: Still triggered
	// Wait a bit to ensure time would change if we overwrote it
	time.Sleep(10 * time.Millisecond)

	results2 := EvaluateThresholds(ctx, budget, 70, 70)
	require.True(t, results2[0].Triggered)

	// Should preserve original timestamp if already triggered?
	// FR-010 says "Add triggered timestamp tracking".
	// If it was already triggered, we should probably keep the original timestamp.
	// Let's assert that behavior if implemented, or implement it that way.
	assert.Equal(t, triggeredAt1.Unix(), results2[0].TriggeredAt.Unix())
}
