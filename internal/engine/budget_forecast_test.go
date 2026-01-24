package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// TestCalculateForecastedSpend verifies linear extrapolation logic (FR-006).
func TestCalculateForecastedSpend(t *testing.T) {
	now := time.Now()
	// Create a standard month: Jan 1 to Jan 31 (31 days)
	start := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond) // Jan 31 23:59:59...

	tests := []struct {
		name         string
		currentSpend float64
		// We pass a "now" time to simulate position in the month
		simulatedNow time.Time
		expected     float64
		delta        float64
	}{
		{
			name:         "period not started (now < start)",
			currentSpend: 100,
			simulatedNow: start.Add(-24 * time.Hour),
			expected:     100, // No extrapolation, return current
			delta:        0.01,
		},
		{
			name:         "start of period (elapsed ~0)",
			currentSpend: 100,
			simulatedNow: start.Add(1 * time.Second), // Very little elapsed
			// Extrapolation would be huge (100 / 1sec * 31days), probably capped or raw math
			// Let's assume raw math for FR-006, but practical limits might apply.
			// 1 sec elapsed out of ~2.6M sec.
			// Forecast = 100 * (2.6M / 1) = 260M.
			// Hard to test exact float match without large delta.
			// Let's test "mid-period" for better stability.
			expected: -1, // Skip exact check, just ensure > current
			delta:    -1,
		},
		{
			name:         "mid-period (15.5 days elapsed, 50% time)",
			currentSpend: 500,
			// Jan 16 12:00 = 15.5 days from Jan 1
			simulatedNow: start.Add(15 * 24 * time.Hour).Add(12 * time.Hour),
			expected:     1000, // Should double
			delta:        1.0,
		},
		{
			name:         "end of period (100% time)",
			currentSpend: 1000,
			simulatedNow: end,
			expected:     1000, // No extrapolation needed (or factor is 1.0)
			delta:        0.01,
		},
		{
			name:         "zero current spend",
			currentSpend: 0,
			simulatedNow: start.Add(10 * 24 * time.Hour),
			expected:     0,
			delta:        0.01,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// CalculateForecastedSpend expects (current, start, end).
			// But it internally uses time.Now() unless we inject it?
			// The signature in `contracts/budget-api.go` is:
			// func CalculateForecastedSpend(currentSpend float64, periodStart time.Time, periodEnd time.Time) float64
			// This implies it calculates "now" internally.
			// To test deterministic time, we might need a time provider or pass "now" explicitly.
			// For this test suite, let's assume we can't easily mock time.Now() inside the function
			// unless we change the signature or use a global override (which is messy).
			// OR, we can change the signature to accept `now`.
			// `contracts/budget-api.go` didn't specify `now`.
			// Let's verify `contracts/budget-api.go` again.
			// It says: "func CalculateForecastedSpend(currentSpend float64, periodStart time.Time, periodEnd time.Time) float64"
			// If it uses time.Now(), we can't test "mid-period" easily without mocking.

			// Strategy: Add `now` parameter to the function for testability?
			// Or make `CalculateForecastedSpendAt(..., now)` and have the main one call it.
			// I'll implement `CalculateForecastedSpendAt` in the code and test THAT.

			result := CalculateForecastedSpendAt(tc.currentSpend, start, end, tc.simulatedNow)

			if tc.expected != -1 {
				assert.InDelta(t, tc.expected, result, tc.delta)
			} else {
				assert.Greater(t, result, tc.currentSpend)
			}
		})
	}
}

// TestCalculateForecastedPercentage verifies percentage calculation.
func TestCalculateForecastedPercentage(t *testing.T) {
	tests := []struct {
		name       string
		forecasted float64
		limit      float64
		expected   float64
	}{
		{"normal case", 1200, 1000, 120.0},
		{"zero limit", 500, 0, 0.0},
		{"zero forecast", 0, 1000, 0.0},
		{"exact limit", 1000, 1000, 100.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CalculateForecastedPercentage(tc.forecasted, tc.limit)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestUpdateBudgetForecast verifies integration of forecast logic.
func TestUpdateBudgetForecast(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	// Period: Starts 15 days ago, ends 15 days from now (30 days total)
	start := now.Add(-15 * 24 * time.Hour)
	end := now.Add(15 * 24 * time.Hour)

	budget := &pbc.Budget{
		Id:     "b-forecast",
		Amount: &pbc.BudgetAmount{Limit: 1000},
		Status: &pbc.BudgetStatus{
			CurrentSpend:   500, // 50% spent in 50% time -> Forecast should be ~1000
			PercentageUsed: 50.0,
		},
	}

	UpdateBudgetForecast(ctx, budget, start, end)

	status := budget.GetStatus()
	require.NotNil(t, status)

	// Forecast should be approx 1000
	assert.InDelta(t, 1000.0, status.GetForecastedSpend(), 5.0) // Allow small delta for time execution drift
	assert.InDelta(t, 100.0, status.GetPercentageForecasted(), 0.5)
}
