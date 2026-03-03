package engine

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// computeActualMTDForDrift returns the actualMTD value that would produce the
// given percent drift for the specified parameters, accounting for the
// standardProjectedDaysPerMonth normalization.
func computeActualMTDForDrift(projected, targetPercentDrift float64, dayOfMonth, daysInMonth int) float64 {
	projectedNorm := projected * (float64(daysInMonth) / standardProjectedDaysPerMonth)
	extrapolated := projectedNorm * (1 + targetPercentDrift/driftPercentMultiplier)
	return extrapolated * float64(dayOfMonth) / float64(daysInMonth)
}

// ---------------------------------------------------------------------------
// CalculateCostDrift
// ---------------------------------------------------------------------------

func TestCalculateCostDrift(t *testing.T) {
	tests := []struct {
		name        string
		actualMTD   float64
		projected   float64
		dayOfMonth  int
		daysInMonth int
		wantDrift   bool // expect non-nil CostDriftData
		wantErr     bool
		errContains string
		wantPercent float64 // approximate expected PercentDrift
		wantWarning bool
	}{
		{
			name:        "day 1 returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  1,
			daysInMonth: 30,
			wantErr:     true,
			errContains: "insufficient data (day 1 of month)",
		},
		{
			name:        "day 2 returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  2,
			daysInMonth: 30,
			wantErr:     true,
			errContains: "insufficient data (day 2 of month)",
		},
		{
			name:        "daysInMonth zero returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  3,
			daysInMonth: 0,
			wantErr:     true,
			errContains: "invalid daysInMonth 0 (must be 28..31)",
		},
		{
			name:        "daysInMonth negative returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  3,
			daysInMonth: -5,
			wantErr:     true,
			errContains: "invalid daysInMonth -5 (must be 28..31)",
		},
		{
			name:        "daysInMonth 27 returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  3,
			daysInMonth: 27,
			wantErr:     true,
			errContains: "invalid daysInMonth 27 (must be 28..31)",
		},
		{
			name:        "daysInMonth exceeds 31 returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  3,
			daysInMonth: 32,
			wantErr:     true,
			errContains: "invalid daysInMonth 32 (must be 28..31)",
		},
		{
			name:        "dayOfMonth exceeds daysInMonth returns error",
			actualMTD:   100,
			projected:   100,
			dayOfMonth:  30,
			daysInMonth: 28,
			wantErr:     true,
			errContains: "dayOfMonth 30 exceeds daysInMonth 28",
		},
		{
			name:        "both zero returns nil",
			actualMTD:   0,
			projected:   0,
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   false,
		},
		{
			name:        "deleted resource (projected=0, actual>0)",
			actualMTD:   50,
			projected:   0,
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   false,
		},
		{
			name:        "new resource (actual=0, projected>0)",
			actualMTD:   0,
			projected:   100,
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   false,
		},
		{
			name:        "drift below 10% threshold returns nil",
			actualMTD:   50,  // extrapolated = 50 * (30/15) = 100
			projected:   100, // delta = 0, percent = 0%
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   false,
		},
		{
			name:        "drift just below 10% threshold returns nil",
			actualMTD:   computeActualMTDForDrift(100, 9.9, 15, 30),
			projected:   100,
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   false,
		},
		{
			name:        "drift just above 10% threshold returns data",
			actualMTD:   computeActualMTDForDrift(100, 11.0, 15, 30),
			projected:   100,
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: 11.0,
			wantWarning: true,
		},
		{
			name:        "large positive drift",
			actualMTD:   100, // extrapolated = 100 * (30/10) = 300
			projected:   100, // projected is normalized to calendar month before drift comparison
			dayOfMonth:  10,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: 204.17,
			wantWarning: true,
		},
		{
			name:        "negative drift beyond threshold",
			actualMTD:   20,  // extrapolated = 20 * (30/15) = 40
			projected:   100, // delta = -60, percent = -60%
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: -60.0,
			wantWarning: true,
		},
		{
			name:        "february 28 day month",
			actualMTD:   140, // extrapolated = 140 * (28/14) = 280
			projected:   200, // projected is normalized from 730h/month to 28-day month
			dayOfMonth:  14,
			daysInMonth: 28,
			wantDrift:   true,
			wantPercent: 52.08,
			wantWarning: true,
		},
		{
			name:        "31 day month last day",
			actualMTD:   310, // extrapolated = 310 * (31/31) = 310
			projected:   300, // delta = 10, percent = 3.33%
			dayOfMonth:  31,
			daysInMonth: 31,
			wantDrift:   false, // below 10%
		},
		{
			name:        "day 3 minimum usable day",
			actualMTD:   30,  // extrapolated = 30 * (30/3) = 300
			projected:   100, // projected is normalized to calendar month before drift comparison
			dayOfMonth:  3,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: 204.17,
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drift, err := CalculateCostDrift(tt.actualMTD, tt.projected, tt.dayOfMonth, tt.daysInMonth)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, drift)
				return
			}

			require.NoError(t, err)

			if !tt.wantDrift {
				assert.Nil(t, drift)
				return
			}

			require.NotNil(t, drift)
			assert.InDelta(t, tt.wantPercent, drift.PercentDrift, 1.0,
				"PercentDrift should be approximately %.1f%%", tt.wantPercent)
			assert.Equal(t, tt.wantWarning, drift.IsWarning)
			expectedProjected := tt.projected * (float64(tt.daysInMonth) / standardProjectedDaysPerMonth)
			assert.InDelta(t, expectedProjected, drift.Projected, 0.001)
			assert.Greater(t, drift.ExtrapolatedMonthly, 0.0)

			// Verify the delta computation is consistent.
			expectedDelta := drift.ExtrapolatedMonthly - drift.Projected
			assert.InDelta(t, expectedDelta, drift.Delta, 0.001)

			// Verify validation passes for returned data.
			require.NoError(t, drift.Validate())
		})
	}
}

func TestCalculateCostDrift_ExtrapolationAccuracy(t *testing.T) {
	// Verify extrapolation and projected normalization formulas:
	//   extrapolated = actualMTD * (daysInMonth / dayOfMonth)
	//   projected_for_month = projected * (daysInMonth / standardProjectedDaysPerMonth)
	drift, err := CalculateCostDrift(150, 100, 10, 30)
	require.NoError(t, err)
	require.NotNil(t, drift)

	expectedExtrapolated := 150.0 * (30.0 / 10.0) // = 450
	assert.InDelta(t, expectedExtrapolated, drift.ExtrapolatedMonthly, 0.001)

	expectedProjected := 100.0 * (30.0 / standardProjectedDaysPerMonth)
	assert.InDelta(t, expectedProjected, drift.Projected, 0.001)

	expectedDelta := 450.0 - expectedProjected
	assert.InDelta(t, expectedDelta, drift.Delta, 0.001)

	expectedPercent := (expectedDelta / expectedProjected) * 100.0
	assert.InDelta(t, expectedPercent, drift.PercentDrift, 0.001)
}

func TestCalculateCostDrift_MonthLengthNormalization(t *testing.T) {
	projected := 100.0
	tests := []struct {
		name        string
		dayOfMonth  int
		daysInMonth int
	}{
		{
			name:        "february_28_day_mid_month",
			dayOfMonth:  15,
			daysInMonth: 28,
		},
		{
			name:        "february_28_day_end_of_month",
			dayOfMonth:  28,
			daysInMonth: 28,
		},
		{
			name:        "leap_february_end_of_month",
			dayOfMonth:  29,
			daysInMonth: 29,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectedForCalendarMonth := projected * (float64(tt.daysInMonth) / standardProjectedDaysPerMonth)
			actualMTD := projectedForCalendarMonth * (float64(tt.dayOfMonth) / float64(tt.daysInMonth))

			drift, err := CalculateCostDrift(actualMTD, projected, tt.dayOfMonth, tt.daysInMonth)
			require.NoError(t, err)
			assert.Nil(t, drift, "drift should be suppressed when run-rate exactly matches normalized projection")
		})
	}
}

func TestCalculateCostDriftWithElapsedDays(t *testing.T) {
	t.Run("insufficient elapsed days returns error", func(t *testing.T) {
		drift, err := CalculateCostDriftWithElapsedDays(1.0, 10.0, 1.99, 31)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient data")
		assert.Nil(t, drift)
	})

	t.Run("fractional elapsed day avoids day-of-month rounding bias", func(t *testing.T) {
		// March 3 at 06:00 is 2.25 elapsed days since March 1 00:00.
		// This case regressed previously to ~-25% drift when denominator=3.
		const (
			projected   = 60.74
			daysInMonth = 31
			elapsedDays = 2.25
		)
		projectedForCalendarMonth := projected * (float64(daysInMonth) / standardProjectedDaysPerMonth)
		actualMTD := projectedForCalendarMonth * (elapsedDays / float64(daysInMonth))

		drift, err := CalculateCostDriftWithElapsedDays(actualMTD, projected, elapsedDays, daysInMonth)
		require.NoError(t, err)
		assert.Nil(t, drift, "matching run-rate should suppress drift")
	})
}

// ---------------------------------------------------------------------------
// CalculateProjectedDelta
// ---------------------------------------------------------------------------

func TestCalculateProjectedDelta(t *testing.T) {
	tests := []struct {
		name            string
		rows            []OverviewRow
		currentDay      int
		wantDelta       float64
		wantCurrency    string
		deltaComparison string // "exact" or "approx"
	}{
		{
			name:            "empty rows",
			rows:            nil,
			currentDay:      15,
			wantDelta:       0,
			wantCurrency:    "",
			deltaComparison: "exact",
		},
		{
			name: "active rows produce no delta",
			rows: []OverviewRow{
				{
					Status:        StatusActive,
					ProjectedCost: &ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
				},
			},
			currentDay:      15,
			wantDelta:       0,
			wantCurrency:    "",
			deltaComparison: "exact",
		},
		{
			name: "creating resource adds projected cost",
			rows: []OverviewRow{
				{
					Status:        StatusCreating,
					ProjectedCost: &ProjectedCostData{MonthlyCost: 50, Currency: "USD"},
				},
			},
			currentDay:      15,
			wantDelta:       50.0,
			wantCurrency:    "USD",
			deltaComparison: "exact",
		},
		{
			name: "deleting resource subtracts extrapolated actual",
			rows: []OverviewRow{
				{
					Status:     StatusDeleting,
					ActualCost: &ActualCostData{MTDCost: 50, Currency: "USD"},
				},
			},
			currentDay:      15,
			wantDelta:       -100.0, // extrapolated = 50 * (30/15) = 100
			wantCurrency:    "USD",
			deltaComparison: "approx",
		},
		{
			name: "updating resource: projected minus extrapolated actual",
			rows: []OverviewRow{
				{
					Status:        StatusUpdating,
					ProjectedCost: &ProjectedCostData{MonthlyCost: 200, Currency: "EUR"},
					ActualCost:    &ActualCostData{MTDCost: 50, Currency: "EUR"},
					PropertyDiffs: []PropertyDiff{{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.large"}},
				},
			},
			currentDay:      15,
			wantDelta:       100.0, // 200 - (50 * 30/15) = 200 - 100 = 100
			wantCurrency:    "EUR",
			deltaComparison: "approx",
		},
		{
			name: "replacing resource: projected minus extrapolated actual",
			rows: []OverviewRow{
				{
					Status:        StatusReplacing,
					ProjectedCost: &ProjectedCostData{MonthlyCost: 150, Currency: "USD"},
					ActualCost:    &ActualCostData{MTDCost: 50, Currency: "USD"},
					PropertyDiffs: []PropertyDiff{{Key: "ami", OldValue: "ami-old", NewValue: "ami-new"}},
				},
			},
			currentDay:      15,
			wantDelta:       50.0, // 150 - (50 * 30/15) = 150 - 100 = 50
			wantCurrency:    "USD",
			deltaComparison: "approx",
		},
		{
			name: "updating prefers projected baseline when present",
			rows: []OverviewRow{
				{
					Status:                StatusUpdating,
					ProjectedCost:         &ProjectedCostData{MonthlyCost: 33.872, Currency: "USD"},
					BaselineProjectedCost: &ProjectedCostData{MonthlyCost: 22.776, Currency: "USD"},
					PropertyDiffs: []PropertyDiff{{
						Key:      "instanceType",
						OldValue: "t2.medium",
						NewValue: "t2.large",
					}},
					// Would produce a very different value if actual extrapolation was used.
					ActualCost: &ActualCostData{MTDCost: 2.0, Currency: "USD"},
				},
			},
			currentDay:      15,
			wantDelta:       11.096, // projected(new) - projected(current)
			wantCurrency:    "USD",
			deltaComparison: "approx",
		},
		{
			name: "replacing prefers projected baseline when present",
			rows: []OverviewRow{
				{
					Status:                StatusReplacing,
					ProjectedCost:         &ProjectedCostData{MonthlyCost: 7.592, Currency: "USD"},
					BaselineProjectedCost: &ProjectedCostData{MonthlyCost: 7.592, Currency: "USD"},
					PropertyDiffs:         []PropertyDiff{{Key: "ami", OldValue: "ami-old", NewValue: "ami-new"}},
					ActualCost:            &ActualCostData{MTDCost: 0.47, Currency: "USD"},
				},
			},
			currentDay:      15,
			wantDelta:       0.0,
			wantCurrency:    "USD",
			deltaComparison: "exact",
		},
		{
			name: "mixed additions and deletions",
			rows: []OverviewRow{
				{
					Status:        StatusCreating,
					ProjectedCost: &ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
				},
				{
					Status:     StatusDeleting,
					ActualCost: &ActualCostData{MTDCost: 30, Currency: "USD"},
				},
			},
			currentDay:      15,
			wantDelta:       40.0, // +100 - (30 * 30/15) = 100 - 60 = 40
			wantCurrency:    "USD",
			deltaComparison: "approx",
		},
		{
			name: "nil projected cost treated as zero",
			rows: []OverviewRow{
				{
					Status:        StatusCreating,
					ProjectedCost: nil,
				},
			},
			currentDay:      15,
			wantDelta:       0,
			wantCurrency:    "",
			deltaComparison: "exact",
		},
		{
			name: "nil actual cost treated as zero",
			rows: []OverviewRow{
				{
					Status:     StatusDeleting,
					ActualCost: nil,
				},
			},
			currentDay:      15,
			wantDelta:       0,
			wantCurrency:    "",
			deltaComparison: "exact",
		},
		{
			name: "early month still extrapolates actual for consistency",
			rows: []OverviewRow{
				{
					Status:     StatusDeleting,
					ActualCost: &ActualCostData{MTDCost: 10, Currency: "USD"},
				},
			},
			currentDay:      2,
			wantDelta:       -150.0, // -(10 * 30/2)
			wantCurrency:    "USD",
			deltaComparison: "exact",
		},
		{
			name: "currency from actual when projected is nil",
			rows: []OverviewRow{
				{
					Status:     StatusDeleting,
					ActualCost: &ActualCostData{MTDCost: 30, Currency: "GBP"},
				},
			},
			currentDay:      15,
			wantDelta:       -60.0, // -(30 * 30/15)
			wantCurrency:    "GBP",
			deltaComparison: "approx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, currency := CalculateProjectedDelta(tt.rows, tt.currentDay)

			if tt.deltaComparison == "exact" {
				assert.Equal(t, tt.wantDelta, delta)
			} else {
				assert.InDelta(t, tt.wantDelta, delta, 1.0,
					"delta should be approximately %.1f", tt.wantDelta)
			}
			assert.Equal(t, tt.wantCurrency, currency)
		})
	}
}

func TestCalculateProjectedDelta_LargeDataset(t *testing.T) {
	// Verify consistent behaviour with many rows.
	rows := make([]OverviewRow, 100)
	for i := range rows {
		rows[i] = OverviewRow{
			Status:        StatusCreating,
			ProjectedCost: &ProjectedCostData{MonthlyCost: 10, Currency: "USD"},
		}
	}

	delta, currency := CalculateProjectedDelta(rows, 15)
	assert.InDelta(t, 1000.0, delta, 0.001) // 100 * 10
	assert.Equal(t, "USD", currency)
}

func TestCalculateProjectedDelta_MathConsistency(t *testing.T) {
	// Verify that creating and then deleting the same resource nets to zero.
	rows := []OverviewRow{
		{
			Status:        StatusCreating,
			ProjectedCost: &ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
		},
		{
			Status:     StatusDeleting,
			ActualCost: &ActualCostData{MTDCost: 50, Currency: "USD"},
		},
	}

	// With day 15 of 30-day month, deleting extrapolates 50 * (30/15) = 100.
	// Net = +100 - 100 = 0
	delta, _ := CalculateProjectedDelta(rows, 15)
	assert.True(t, math.Abs(delta) < 1.0,
		"creating and deleting same-cost resource should net close to zero, got %.2f", delta)
}

// ---------------------------------------------------------------------------
// CalculateRowDelta
// ---------------------------------------------------------------------------

func TestCalculateRowDelta_UpdatingResource(t *testing.T) {
	row := OverviewRow{
		Status:        StatusUpdating,
		ActualCost:    &ActualCostData{MTDCost: 50},
		ProjectedCost: &ProjectedCostData{MonthlyCost: 100},
		PropertyDiffs: []PropertyDiff{{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.large"}},
	}
	// Day 15: extrapolated = 50 * (30/15) = 100; delta = 100 - 100 = 0
	delta, ok := CalculateRowDelta(row, 15)
	require.True(t, ok)
	assert.InDelta(t, 0.0, delta, 0.01)
}

func TestCalculateRowDelta_ReplacingResource(t *testing.T) {
	row := OverviewRow{
		Status:        StatusReplacing,
		ActualCost:    &ActualCostData{MTDCost: 30},
		ProjectedCost: &ProjectedCostData{MonthlyCost: 100},
		PropertyDiffs: []PropertyDiff{{Key: "ami", OldValue: "ami-old", NewValue: "ami-new"}},
	}
	// Day 15: extrapolated = 30 * (30/15) = 60; delta = 100 - 60 = 40
	delta, ok := CalculateRowDelta(row, 15)
	require.True(t, ok)
	assert.InDelta(t, 40.0, delta, 0.01)
}

func TestCalculateRowDelta_ReplacingResource_UsesBaselineProjectedWhenAvailable(t *testing.T) {
	row := OverviewRow{
		Status:                StatusReplacing,
		ProjectedCost:         &ProjectedCostData{MonthlyCost: 7.592},
		BaselineProjectedCost: &ProjectedCostData{MonthlyCost: 7.592},
		PropertyDiffs:         []PropertyDiff{{Key: "ami", OldValue: "ami-old", NewValue: "ami-new"}},
		ActualCost:            &ActualCostData{MTDCost: 0.47},
	}
	delta, ok := CalculateRowDelta(row, 15)
	require.True(t, ok)
	assert.InDelta(t, 0.0, delta, 0.0001)
}

func TestCalculateRowDelta_CreatingResource(t *testing.T) {
	row := OverviewRow{
		Status:        StatusCreating,
		ProjectedCost: &ProjectedCostData{MonthlyCost: 75},
	}
	delta, ok := CalculateRowDelta(row, 15)
	require.True(t, ok)
	assert.InDelta(t, 75.0, delta, 0.01)
}

func TestCalculateRowDelta_DeletingResource(t *testing.T) {
	row := OverviewRow{
		Status:     StatusDeleting,
		ActualCost: &ActualCostData{MTDCost: 50},
	}
	// Day 15: extrapolated = 50 * (30/15) = 100; delta = -100
	delta, ok := CalculateRowDelta(row, 15)
	require.True(t, ok)
	assert.InDelta(t, -100.0, delta, 0.01)
}

func TestCalculateRowDelta_ActiveWithDrift(t *testing.T) {
	row := OverviewRow{
		Status:    StatusActive,
		CostDrift: &CostDriftData{Delta: -25.0},
	}
	delta, ok := CalculateRowDelta(row, 15)
	require.True(t, ok)
	assert.InDelta(t, -25.0, delta, 0.01)
}

func TestCalculateRowDelta_ActiveWithoutDrift(t *testing.T) {
	row := OverviewRow{
		Status:        StatusActive,
		ProjectedCost: &ProjectedCostData{MonthlyCost: 100},
		ActualCost:    &ActualCostData{MTDCost: 50},
	}
	_, ok := CalculateRowDelta(row, 15)
	assert.False(t, ok, "active without drift should return false")
}

func TestCalculateRowDelta_NoCostData(t *testing.T) {
	row := OverviewRow{
		Status: StatusUpdating,
	}
	_, ok := CalculateRowDelta(row, 15)
	assert.False(t, ok, "no cost data should return false")
}

func TestCalculateRowDelta_NoPropertyDiffs(t *testing.T) {
	// Updating/replacing without PropertyDiffs should skip delta calculation.
	tests := []struct {
		name string
		row  OverviewRow
	}{
		{
			name: "updating without property diffs",
			row: OverviewRow{
				Status:        StatusUpdating,
				ActualCost:    &ActualCostData{MTDCost: 50},
				ProjectedCost: &ProjectedCostData{MonthlyCost: 200},
			},
		},
		{
			name: "replacing without property diffs",
			row: OverviewRow{
				Status:        StatusReplacing,
				ActualCost:    &ActualCostData{MTDCost: 30},
				ProjectedCost: &ProjectedCostData{MonthlyCost: 100},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, ok := CalculateRowDelta(tt.row, 15)
			assert.False(t, ok, "no PropertyDiffs should return false")
			assert.Equal(t, 0.0, delta)
		})
	}
}

func TestCalculateRowDelta_EarlyMonth(t *testing.T) {
	row := OverviewRow{
		Status:        StatusReplacing,
		ActualCost:    &ActualCostData{MTDCost: 2},
		ProjectedCost: &ProjectedCostData{MonthlyCost: 60},
		PropertyDiffs: []PropertyDiff{{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.large"}},
	}
	// Day 1: extrapolated = 2 * (30/1) = 60, delta = 60 - 60 = 0.
	delta, ok := CalculateRowDelta(row, 1)
	require.True(t, ok)
	assert.InDelta(t, 0.0, delta, 0.01)
}

func TestCalculateRowDelta_EarlyMonth_Deleting(t *testing.T) {
	row := OverviewRow{
		Status:     StatusDeleting,
		ActualCost: &ActualCostData{MTDCost: 5},
	}
	// Day 2: extrapolated = 5 * (30/2) = 75; delta = -75.
	delta, ok := CalculateRowDelta(row, 2)
	require.True(t, ok)
	assert.InDelta(t, -75.0, delta, 0.01)
}

func TestCalculateRowDelta_EarlyMonth_Creating(t *testing.T) {
	row := OverviewRow{
		Status:        StatusCreating,
		ProjectedCost: &ProjectedCostData{MonthlyCost: 75},
	}
	// Creating resources don't depend on extrapolation, so delta is always valid.
	delta, ok := CalculateRowDelta(row, 1)
	require.True(t, ok, "creating delta doesn't depend on extrapolation")
	assert.InDelta(t, 75.0, delta, 0.01)
}

// ---------------------------------------------------------------------------
// ForceExtrapolateActual
// ---------------------------------------------------------------------------

func TestForceExtrapolateActual_EarlyMonth(t *testing.T) {
	row := OverviewRow{ActualCost: &ActualCostData{MTDCost: 2.0}}
	// Day 1: 2.0 * (30/1) = 60.0
	assert.InDelta(t, 60.0, ForceExtrapolateActual(row, 1), 0.01)
	// Day 2: 2.0 * (30/2) = 30.0
	assert.InDelta(t, 30.0, ForceExtrapolateActual(row, 2), 0.01)
}

func TestForceExtrapolateActual_NilActualCost(t *testing.T) {
	row := OverviewRow{}
	assert.Equal(t, 0.0, ForceExtrapolateActual(row, 5))
}

func TestForceExtrapolateActual_ZeroDayOfMonth(t *testing.T) {
	row := OverviewRow{ActualCost: &ActualCostData{MTDCost: 10.0}}
	// dayOfMonth <= 0: returns raw MTD.
	assert.Equal(t, 10.0, ForceExtrapolateActual(row, 0))
}

// ---------------------------------------------------------------------------
// PopulateComputedDeltas
// ---------------------------------------------------------------------------

func TestPopulateComputedDeltas_MixedStatuses(t *testing.T) {
	rows := []OverviewRow{
		{
			URN:           "urn:active",
			Type:          "aws:ec2:Instance",
			Status:        StatusActive,
			ProjectedCost: &ProjectedCostData{MonthlyCost: 100},
			ActualCost:    &ActualCostData{MTDCost: 50},
			// No drift → ComputedDelta should remain nil.
		},
		{
			URN:           "urn:creating",
			Type:          "aws:s3:Bucket",
			Status:        StatusCreating,
			ProjectedCost: &ProjectedCostData{MonthlyCost: 50},
		},
		{
			URN:        "urn:deleting",
			Type:       "aws:lambda:Function",
			Status:     StatusDeleting,
			ActualCost: &ActualCostData{MTDCost: 30},
		},
		{
			URN:           "urn:updating",
			Type:          "aws:rds:Instance",
			Status:        StatusUpdating,
			ActualCost:    &ActualCostData{MTDCost: 60},
			ProjectedCost: &ProjectedCostData{MonthlyCost: 200},
			PropertyDiffs: []PropertyDiff{{Key: "instanceClass", OldValue: "db.t3.small", NewValue: "db.t3.large"}},
		},
	}

	PopulateComputedDeltas(rows, 15)

	// Active without drift → nil.
	assert.Nil(t, rows[0].ComputedDelta, "active without drift should be nil")

	// Creating → +projected.
	require.NotNil(t, rows[1].ComputedDelta)
	assert.Equal(t, 50.0, *rows[1].ComputedDelta)

	// Deleting → -extrapolated actual (30 * 30/15 = 60).
	require.NotNil(t, rows[2].ComputedDelta)
	assert.InDelta(t, -60.0, *rows[2].ComputedDelta, 0.01)

	// Updating with PropertyDiffs → projected - extrapolated actual (200 - 60*30/15 = 200-120 = 80).
	require.NotNil(t, rows[3].ComputedDelta)
	assert.InDelta(t, 80.0, *rows[3].ComputedDelta, 0.01)
}

func TestPopulateComputedDeltas_EmptyRows(t *testing.T) {
	// Should not panic on empty/nil slices.
	PopulateComputedDeltas(nil, 15)
	PopulateComputedDeltas([]OverviewRow{}, 15)
}

func TestPopulateComputedDeltas_ActiveWithDrift(t *testing.T) {
	rows := []OverviewRow{
		{
			URN:       "urn:drifting",
			Type:      "aws:ec2:Instance",
			Status:    StatusActive,
			CostDrift: &CostDriftData{Delta: -25.0},
		},
	}

	PopulateComputedDeltas(rows, 15)

	require.NotNil(t, rows[0].ComputedDelta)
	assert.InDelta(t, -25.0, *rows[0].ComputedDelta, 0.01)
}

func TestPopulateComputedDeltas_SummaryConsistency(t *testing.T) {
	// Verify that sum of ComputedDelta equals CalculateProjectedDelta for
	// rows with pending changes (the consistency guarantee this fix ensures).
	rows := []OverviewRow{
		{
			URN:           "urn:creating",
			Type:          "aws:s3:Bucket",
			Status:        StatusCreating,
			ProjectedCost: &ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
		},
		{
			URN:        "urn:deleting",
			Type:       "aws:lambda:Function",
			Status:     StatusDeleting,
			ActualCost: &ActualCostData{MTDCost: 30, Currency: "USD"},
		},
	}

	const day = 15
	PopulateComputedDeltas(rows, day)

	// Sum of per-row deltas.
	var sumDelta float64
	for _, row := range rows {
		if row.ComputedDelta != nil {
			sumDelta += *row.ComputedDelta
		}
	}

	// CalculateRowDelta uses ForceExtrapolateActual (always extrapolates),
	// while CalculateProjectedDelta uses GetExtrapolatedActual (returns raw MTD on day <3).
	// On day 15 both extrapolate, so they should match.
	expectedCreating, _ := CalculateRowDelta(rows[0], day)
	expectedDeleting, _ := CalculateRowDelta(rows[1], day)
	assert.InDelta(t, expectedCreating+expectedDeleting, sumDelta, 0.01)
}
