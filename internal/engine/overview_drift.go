package engine

import (
	"fmt"
	"math"
)

// driftMinDay is the earliest day-of-month at which drift can be calculated.
// Days 1 and 2 have insufficient data for meaningful extrapolation.
const driftMinDay = 3

// driftMinElapsedDays is the minimum elapsed runtime window (in days) needed
// to compute a meaningful run-rate drift. It preserves the intent of
// driftMinDay (skip day 1 and day 2) while allowing fractional-day precision.
const driftMinElapsedDays = float64(driftMinDay - 1)

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
	if err := validateDaysInMonth(daysInMonth); err != nil {
		return nil, err
	}
	if dayOfMonth > daysInMonth {
		return nil, fmt.Errorf(
			"%w: dayOfMonth %d exceeds daysInMonth %d",
			ErrOverviewValidation, dayOfMonth, daysInMonth,
		)
	}

	return CalculateCostDriftWithElapsedDays(actualMTD, projected, float64(dayOfMonth), daysInMonth)
}

// CalculateCostDriftWithElapsedDays computes run-rate drift using a fractional
// elapsed day denominator. This avoids early-month bias from integer dayOfMonth
// rounding (for example, March 3 at 06:00 should use 2.25 elapsed days, not 3).
//
// Parameters:
//   - actualMTD: actual cost accumulated over the elapsed window.
//   - projected: projected monthly cost expressed on the canonical 730h basis.
//   - elapsedDays: elapsed runtime window in days; may be fractional.
//   - daysInMonth: total days in the reference calendar month (28-31).
//
// Return values match CalculateCostDrift.
func CalculateCostDriftWithElapsedDays(
	actualMTD, projected, elapsedDays float64,
	daysInMonth int,
) (*CostDriftData, error) {
	if err := validateDaysInMonth(daysInMonth); err != nil {
		return nil, err
	}
	if elapsedDays <= 0 {
		return nil, fmt.Errorf("%w: elapsedDays must be > 0, got %.4f", ErrOverviewValidation, elapsedDays)
	}
	if math.IsNaN(elapsedDays) || math.IsInf(elapsedDays, 0) {
		return nil, fmt.Errorf("%w: invalid elapsedDays %.4f", ErrOverviewValidation, elapsedDays)
	}
	if elapsedDays < driftMinElapsedDays {
		return nil, fmt.Errorf("%w: insufficient data (elapsed %.2f days)", ErrOverviewValidation, elapsedDays)
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
	extrapolated := actualMTD * (float64(daysInMonth) / elapsedDays)
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

func validateDaysInMonth(daysInMonth int) error {
	const minCalendarDays = 28
	const maxCalendarDays = 31
	if daysInMonth < minCalendarDays || daysInMonth > maxCalendarDays {
		return fmt.Errorf(
			"%w: invalid daysInMonth %d (must be %d..%d)",
			ErrOverviewValidation, daysInMonth, minCalendarDays, maxCalendarDays,
		)
	}
	return nil
}

// CalculateProjectedDelta computes the aggregate change-impact delta for a set
// of overview rows by examining pending changes.
//
// For each row with a pending operation:
//   - Updating/Replacing: delta += projected(after) - projected(current)
//     when baseline projected data is available; otherwise falls back to
//     projected(after) - extrapolated_actual.
//   - Creating: delta += projected
//   - Deleting: delta -= extrapolated_actual
//
// The currency is taken from the first non-empty cost currency in rows that
// contributed a delta. Mixed currencies cannot reach this function because
// upstream returns ErrMixedCurrencies before aggregation.
//
// To match per-row delta behavior, extrapolation always uses
// ForceExtrapolateActual, including day 1-2.
func CalculateProjectedDelta(rows []OverviewRow, currentDayOfMonth int) (float64, string) {
	var delta float64
	var currency string
	for _, row := range rows {
		rowDelta, ok := calculateProjectedDeltaForRow(row, currentDayOfMonth)
		if !ok {
			continue
		}
		delta += rowDelta
		if currency == "" {
			currency = pickCurrency(row)
		}
	}
	return delta, currency
}

// calculateProjectedDeltaForRow returns the change-impact delta for a single
// pending row and whether that row contributes to projected delta aggregation.
func calculateProjectedDeltaForRow(row OverviewRow, dayOfMonth int) (float64, bool) {
	switch row.Status { //nolint:exhaustive // Active/default intentionally excluded.
	case StatusUpdating, StatusReplacing:
		if len(row.PropertyDiffs) == 0 {
			return 0, false
		}
		projected := GetProjectedMonthlyCost(row)
		if baseline, ok := GetBaselineProjectedMonthlyCost(row); ok {
			return projected - baseline, true
		}
		return projected - ForceExtrapolateActual(row, dayOfMonth), true
	case StatusCreating:
		return GetProjectedMonthlyCost(row), true
	case StatusDeleting:
		return -ForceExtrapolateActual(row, dayOfMonth), true
	default:
		return 0, false
	}
}

// GetProjectedMonthlyCost safely extracts the projected monthly cost from a row.
func GetProjectedMonthlyCost(row OverviewRow) float64 {
	if row.ProjectedCost == nil {
		return 0
	}
	return row.ProjectedCost.MonthlyCost
}

// GetBaselineProjectedMonthlyCost extracts current-state projected monthly cost.
func GetBaselineProjectedMonthlyCost(row OverviewRow) (float64, bool) {
	if row.BaselineProjectedCost == nil {
		return 0, false
	}
	return row.BaselineProjectedCost.MonthlyCost, true
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

// ForceExtrapolateActual extrapolates the MTD actual cost to a full month,
// even on early-month days (day 1-2). Used for resources with PropertyDiffs
// where the config change is a known signal and an approximate delta is
// more useful than suppressing it entirely.
func ForceExtrapolateActual(row OverviewRow, dayOfMonth int) float64 {
	if row.ActualCost == nil {
		return 0
	}
	mtd := row.ActualCost.MTDCost
	if dayOfMonth <= 0 {
		return mtd
	}
	return mtd * (defaultDaysPerMonth / float64(dayOfMonth))
}

// CalculateRowDelta computes a per-row cost delta for display purposes.
//
// For resources with pending changes (updating, replacing, creating, deleting),
// the delta represents the cost impact of the change. For updating/replacing
// rows, this prefers projected(after) - projected(current) when baseline
// projected data is available; otherwise it falls back to
// projected(after) - extrapolated actual.
//
// For active resources, CostDrift.Delta is used if available.
//
// Returns (delta, true) when a meaningful delta can be computed, or (0, false)
// when no delta is available (e.g., active resource without drift data).
func CalculateRowDelta(row OverviewRow, dayOfMonth int) (float64, bool) {
	switch row.Status { //nolint:exhaustive // StatusActive handled in default.
	case StatusUpdating, StatusReplacing:
		if len(row.PropertyDiffs) == 0 {
			return 0, false
		}
		projected := GetProjectedMonthlyCost(row)
		if baseline, ok := GetBaselineProjectedMonthlyCost(row); ok {
			if projected == 0 && baseline == 0 {
				return 0, false
			}
			return projected - baseline, true
		}
		actual := ForceExtrapolateActual(row, dayOfMonth)
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
		actual := ForceExtrapolateActual(row, dayOfMonth)
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

// PopulateComputedDeltas computes and stores per-row delta values.
// Must be called after enrichment, before any rendering. All renderers
// (table, JSON, NDJSON, TUI) then read row.ComputedDelta instead of
// independently recomputing deltas — guaranteeing consistency.
func PopulateComputedDeltas(rows []OverviewRow, dayOfMonth int) {
	for i := range rows {
		if d, ok := CalculateRowDelta(rows[i], dayOfMonth); ok {
			val := d
			rows[i].ComputedDelta = &val
		}
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
