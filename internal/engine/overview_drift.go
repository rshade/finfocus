package engine

import (
	"fmt"
	"math"
)

// driftMinDay is the earliest day-of-month at which drift can be calculated.
// Days 1 and 2 have insufficient data for meaningful extrapolation.
const driftMinDay = 3

// driftPercentMultiplier converts a ratio to a percentage.
const driftPercentMultiplier = 100.0

// defaultDaysPerMonth is used for extrapolation in delta calculations.
const defaultDaysPerMonth = 30.0

// standardProjectedDaysPerMonth converts the canonical 730h projected pricing
// basis into day units for drift normalization.
const standardProjectedDaysPerMonth = float64(HoursPerMonth) / float64(hoursPerDay)

// CalculateCostDrift computes the cost drift between extrapolated actual spend
// and projected monthly cost.
//
// It returns a non-nil CostDriftData only when the absolute percent drift
// exceeds the warning threshold (10%). In all other cases it returns nil.
//
// Special cases:
//   - dayOfMonth <= 2: returns nil and an error (insufficient data).
//   - Both actual and projected are zero: returns nil, nil (nothing to compare).
//   - Only one side is zero (new or deleted resource): returns nil, nil.
//
// Parameters:
//   - actualMTD: the month-to-date actual cost.
//   - projected: the projected monthly cost (730h standard month basis).
//   - dayOfMonth: the current day of the month (1-based).
// CalculateCostDrift computes the cost drift between an extrapolated month-to-date actual spend
// and the projected monthly cost normalized to the current calendar month.
//
// Parameters:
//  - actualMTD: month-to-date actual cost for the resource.
//  - projected: projected monthly cost expressed on the package's canonical month basis.
//  - dayOfMonth: current day of the month (1-31); used to extrapolate actualMTD to a full month.
//  - daysInMonth: total days in the current calendar month (28-31); used to normalize the projection.
//
// Return values and error conditions:
//  - Returns (*CostDriftData, nil) when a meaningful drift is detected and its magnitude exceeds the
//    configured warning threshold. The returned CostDriftData contains the extrapolated monthly actual,
//    the projection normalized to the calendar month, the delta (extrapolated − projected), the percent
//    drift, and IsWarning set to true.
//  - Returns (nil, nil) when no meaningful drift can be determined, including:
//      * both actualMTD and projected are zero,
//      * projected is zero but actualMTD > 0 (deleted resource),
//      * actualMTD is zero but projected > 0 (new resource),
//      * or the computed percent drift is within the warning threshold.
//  - Returns (nil, error) with ErrOverviewValidation when input validation fails:
//      * dayOfMonth < driftMinDay (insufficient data to extrapolate), or
//      * daysInMonth <= 0 (invalid month length).
func CalculateCostDrift(actualMTD, projected float64, dayOfMonth, daysInMonth int) (*CostDriftData, error) {
	if dayOfMonth < driftMinDay {
		return nil, fmt.Errorf("%w: insufficient data (day %d of month)", ErrOverviewValidation, dayOfMonth)
	}
	if daysInMonth <= 0 {
		return nil, fmt.Errorf("%w: invalid daysInMonth %d", ErrOverviewValidation, daysInMonth)
	}

	// Edge cases where drift is not meaningful.
	if actualMTD == 0 && projected == 0 {
		return nil, nil //nolint:nilnil // nil,nil is intentional: no drift data, no error.
	}
	if projected == 0 && actualMTD > 0 {
		// Deleted resource: has actual spend but no projection.
		return nil, nil //nolint:nilnil // nil,nil is intentional: no drift data, no error.
	}
	if actualMTD == 0 && projected > 0 {
		// New resource: has projection but no actual spend yet.
		return nil, nil //nolint:nilnil // nil,nil is intentional: no drift data, no error.
	}

	// The projected value is based on a canonical 730h month. Normalize it to
	// the current calendar month before comparison so drift math compares like
	// with like across 28/29/30/31 day months.
	projectedForCalendarMonth := projected * (float64(daysInMonth) / standardProjectedDaysPerMonth)
	extrapolated := actualMTD * (float64(daysInMonth) / float64(dayOfMonth))
	delta := extrapolated - projectedForCalendarMonth
	percentDrift := (delta / projectedForCalendarMonth) * driftPercentMultiplier

	if math.Abs(percentDrift) <= driftWarningThreshold {
		return nil, nil //nolint:nilnil // nil,nil is intentional: drift below threshold, no error.
	}

	return &CostDriftData{
		ExtrapolatedMonthly: extrapolated,
		Projected:           projectedForCalendarMonth,
		Delta:               delta,
		PercentDrift:        percentDrift,
		IsWarning:           true,
	}, nil
}

// CalculateProjectedDelta computes the aggregate cost delta for a set of
// overview rows by examining pending changes.
//
// For each row with a pending operation:
//   - Updating: delta += projected - extrapolated_actual
//   - Creating: delta += projected
//   - Deleting: delta -= extrapolated_actual
//
// The currency is taken from the first non-nil cost data encountered.
// Mixed currencies cannot reach this function because upstream returns
// ErrMixedCurrencies before aggregation.
// The currentDayOfMonth is used for extrapolation of actual costs; if it is
// less than driftMinDay, actual costs are used without extrapolation.
func CalculateProjectedDelta(rows []OverviewRow, currentDayOfMonth int) (float64, string) {
	var delta float64
	var currency string
	for _, row := range rows {
		switch row.Status { //nolint:exhaustive // StatusActive is intentionally skipped (no delta).
		case StatusUpdating:
			projected := getProjectedMonthlyCost(row)
			actual := getExtrapolatedActual(row, currentDayOfMonth)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta += projected - actual

		case StatusCreating:
			projected := getProjectedMonthlyCost(row)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta += projected

		case StatusDeleting:
			actual := getExtrapolatedActual(row, currentDayOfMonth)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta -= actual

		case StatusReplacing:
			// Replace is delete + create; net effect is projected - actual.
			projected := getProjectedMonthlyCost(row)
			actual := getExtrapolatedActual(row, currentDayOfMonth)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta += projected - actual
		}
	}
	return delta, currency
}

// getProjectedMonthlyCost safely extracts the projected monthly cost from a row.
func getProjectedMonthlyCost(row OverviewRow) float64 {
	if row.ProjectedCost == nil {
		return 0
	}
	return row.ProjectedCost.MonthlyCost
}

// getExtrapolatedActual extrapolates the MTD actual cost to a full month.
// If the day of month is too early for reliable extrapolation, returns the
// raw MTD cost.
//
// Note: This function uses defaultDaysPerMonth (30) for extrapolation, which
// differs from CalculateCostDrift that uses the actual daysInMonth (28-31).
// This is intentional: delta calculations use a standardised month length for
// consistent comparisons, while drift calculations use calendar-accurate
// month length for precision.
func getExtrapolatedActual(row OverviewRow, dayOfMonth int) float64 {
	if row.ActualCost == nil {
		return 0
	}
	mtd := row.ActualCost.MTDCost
	if dayOfMonth < driftMinDay {
		return mtd
	}
	// Use a standard 30-day month for extrapolation in delta calculations.
	return mtd * (defaultDaysPerMonth / float64(dayOfMonth))
}

// pickCurrency returns the first non-empty currency found in a row's cost data.
func pickCurrency(row OverviewRow) string {
	if row.ProjectedCost != nil && row.ProjectedCost.Currency != "" {
		return row.ProjectedCost.Currency
	}
	if row.ActualCost != nil && row.ActualCost.Currency != "" {
		return row.ActualCost.Currency
	}
	return ""
}
