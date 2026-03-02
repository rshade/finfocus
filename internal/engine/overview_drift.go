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

// CalculateCostDrift computes the cost drift between an extrapolated month-to-date actual spend
// and the projected monthly cost normalized to the current calendar month.
//
// Parameters:
//   - actualMTD: month-to-date actual cost for the resource.
//   - projected: projected monthly cost expressed on the package's canonical month basis.
//   - dayOfMonth: current day of the month (1-31); used to extrapolate actualMTD to a full month.
//   - daysInMonth: total days in the current calendar month (28-31); used to normalize the projection.
//
// Return values:
//   - (*CostDriftData, nil) when drift exceeds the warning threshold (10%).
//   - (nil, nil) when drift is not meaningful: both sides zero, one side zero
//     (new/deleted resource), or drift within the warning threshold.
//   - (nil, error) with ErrOverviewValidation when inputs are invalid:
//     dayOfMonth < driftMinDay, daysInMonth outside 28..31, or dayOfMonth > daysInMonth.
func CalculateCostDrift(actualMTD, projected float64, dayOfMonth, daysInMonth int) (*CostDriftData, error) {
	if dayOfMonth < driftMinDay {
		return nil, fmt.Errorf("%w: insufficient data (day %d of month)", ErrOverviewValidation, dayOfMonth)
	}
	const minCalendarDays = 28
	const maxCalendarDays = 31
	if daysInMonth < minCalendarDays || daysInMonth > maxCalendarDays {
		return nil, fmt.Errorf(
			"%w: invalid daysInMonth %d (must be %d..%d)",
			ErrOverviewValidation, daysInMonth, minCalendarDays, maxCalendarDays,
		)
	}
	if dayOfMonth > daysInMonth {
		return nil, fmt.Errorf(
			"%w: dayOfMonth %d exceeds daysInMonth %d",
			ErrOverviewValidation, dayOfMonth, daysInMonth,
		)
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
			projected := GetProjectedMonthlyCost(row)
			actual := GetExtrapolatedActual(row, currentDayOfMonth)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta += projected - actual

		case StatusCreating:
			projected := GetProjectedMonthlyCost(row)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta += projected

		case StatusDeleting:
			actual := GetExtrapolatedActual(row, currentDayOfMonth)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta -= actual

		case StatusReplacing:
			// Replace is delete + create; net effect is projected - actual.
			projected := GetProjectedMonthlyCost(row)
			actual := GetExtrapolatedActual(row, currentDayOfMonth)
			if currency == "" {
				currency = pickCurrency(row)
			}
			delta += projected - actual
		}
	}
	return delta, currency
}

// GetProjectedMonthlyCost safely extracts the projected monthly cost from a row.
func GetProjectedMonthlyCost(row OverviewRow) float64 {
	if row.ProjectedCost == nil {
		return 0
	}
	return row.ProjectedCost.MonthlyCost
}

// GetExtrapolatedActual extrapolates the MTD actual cost to a full month.
// If the day of month is too early for reliable extrapolation, returns the
// raw MTD cost.
//
// Note: This function uses defaultDaysPerMonth (30) for extrapolation, which
// differs from CalculateCostDrift that uses the actual daysInMonth (28-31).
// This is intentional: delta calculations use a standardised month length for
// consistent comparisons, while drift calculations use calendar-accurate
// month length for precision.
func GetExtrapolatedActual(row OverviewRow, dayOfMonth int) float64 {
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

// CalculateRowDelta computes a per-row cost delta for display purposes.
//
// For resources with pending changes (updating, replacing, creating, deleting),
// the delta represents the cost impact of the change using the same logic as
// CalculateProjectedDelta: projected minus extrapolated actual.
//
// For active resources, CostDrift.Delta is used if available.
//
// Returns (delta, true) when a meaningful delta can be computed, or (0, false)
// when no delta is available (e.g., active resource without drift data).
func CalculateRowDelta(row OverviewRow, dayOfMonth int) (float64, bool) {
	switch row.Status { //nolint:exhaustive // StatusActive handled in default.
	case StatusUpdating, StatusReplacing:
		projected := GetProjectedMonthlyCost(row)
		actual := GetExtrapolatedActual(row, dayOfMonth)
		if projected == 0 && actual == 0 {
			return 0, false
		}
		return projected - actual, true

	case StatusCreating:
		projected := GetProjectedMonthlyCost(row)
		if projected == 0 {
			return 0, false
		}
		return projected, true

	case StatusDeleting:
		actual := GetExtrapolatedActual(row, dayOfMonth)
		if actual == 0 {
			return 0, false
		}
		return -actual, true

	default:
		// Active resources: use drift delta if available.
		if row.CostDrift != nil {
			return row.CostDrift.Delta, true
		}
		return 0, false
	}
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
