package engine

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name:        "drift exactly at 10% threshold returns nil",
			actualMTD:   55,  // extrapolated = 55 * (30/15) = 110
			projected:   100, // delta = 10, percent = 10%
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   false,
		},
		{
			name:        "drift just above 10% threshold returns data",
			actualMTD:   55.5, // extrapolated = 55.5 * (30/15) = 111
			projected:   100,  // delta = 11, percent = 11%
			dayOfMonth:  15,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: 11.0,
			wantWarning: true,
		},
		{
			name:        "large positive drift",
			actualMTD:   100, // extrapolated = 100 * (30/10) = 300
			projected:   100, // delta = 200, percent = 200%
			dayOfMonth:  10,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: 200.0,
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
			projected:   200, // delta = 80, percent = 40%
			dayOfMonth:  14,
			daysInMonth: 28,
			wantDrift:   true,
			wantPercent: 40.0,
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
			projected:   100, // delta = 200, percent = 200%
			dayOfMonth:  3,
			daysInMonth: 30,
			wantDrift:   true,
			wantPercent: 200.0,
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
			assert.Equal(t, tt.projected, drift.Projected)
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
	// Verify the extrapolation formula: extrapolated = actualMTD * (daysInMonth / dayOfMonth)
	drift, err := CalculateCostDrift(150, 100, 10, 30)
	require.NoError(t, err)
	require.NotNil(t, drift)

	expectedExtrapolated := 150.0 * (30.0 / 10.0) // = 450
	assert.InDelta(t, expectedExtrapolated, drift.ExtrapolatedMonthly, 0.001)

	expectedDelta := 450.0 - 100.0 // = 350
	assert.InDelta(t, expectedDelta, drift.Delta, 0.001)

	expectedPercent := (350.0 / 100.0) * 100.0 // = 350%
	assert.InDelta(t, expectedPercent, drift.PercentDrift, 0.001)
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
			name:         "empty rows",
			rows:         nil,
			currentDay:   15,
			wantDelta:    0,
			wantCurrency: "",
		},
		{
			name: "active rows produce no delta",
			rows: []OverviewRow{
				{
					Status:        StatusActive,
					ProjectedCost: &ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
				},
			},
			currentDay:   15,
			wantDelta:    0,
			wantCurrency: "",
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
			currentDay:   15,
			wantDelta:    0,
			wantCurrency: "",
		},
		{
			name: "nil actual cost treated as zero",
			rows: []OverviewRow{
				{
					Status:     StatusDeleting,
					ActualCost: nil,
				},
			},
			currentDay:   15,
			wantDelta:    0,
			wantCurrency: "",
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
