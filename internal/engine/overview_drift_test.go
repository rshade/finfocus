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
				},
			},
			currentDay:      15,
			wantDelta:       50.0, // 150 - (50 * 30/15) = 150 - 100 = 50
			wantCurrency:    "USD",
			deltaComparison: "approx",
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
			name: "early month uses raw MTD for extrapolation",
			rows: []OverviewRow{
				{
					Status:     StatusDeleting,
					ActualCost: &ActualCostData{MTDCost: 10, Currency: "USD"},
				},
			},
			currentDay:      2,     // Below driftMinDay
			wantDelta:       -10.0, // Uses raw MTD, no extrapolation
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
