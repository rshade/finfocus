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
//   - projected: the projected monthly cost.
//   - dayOfMonth: the current day of the month (1-based).
//   - daysInMonth: total days in the current month (28-31).
func CalculateCostDrift(actualMTD, projected float64, dayOfMonth, daysInMonth int) (*CostDriftData, error) {
	if dayOfMonth < driftMinDay {
		return nil, fmt.Errorf("insufficient data (day %d of month)", dayOfMonth)
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

	extrapolated := actualMTD * (float64(daysInMonth) / float64(dayOfMonth))
	delta := extrapolated - projected
	percentDrift := (delta / projected) * driftPercentMultiplier

	if math.Abs(percentDrift) <= driftWarningThreshold {
		return nil, nil //nolint:nilnil // nil,nil is intentional: drift below threshold, no error.
	}

	return &CostDriftData{
		ExtrapolatedMonthly: extrapolated,
		Projected:           projected,
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
