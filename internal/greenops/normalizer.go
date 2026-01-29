package greenops

import (
	"math"
	"strings"
)

// getUnitFactor returns the conversion factor for a unit string.
// Returns (factor, true) for valid units, (0, false) for invalid.
// Unit matching is case-insensitive.
func getUnitFactor(unit string) (float64, bool) {
	switch strings.ToLower(unit) {
	case "g", "gco2e":
		return GramsToKg, true
	case "kg", "kgco2e":
		return KgToKg, true
	case "t", "tco2e":
		return TonsToKg, true
	case "lb", "lbco2e":
		return PoundsToKg, true
	default:
		return 0, false
	}
}

// NormalizeToKg converts a carbon value in any recognized unit to kilograms.
//
// Recognized units: g, kg, t, lb, gCO2e, kgCO2e, tCO2e, lbCO2e
// Unit matching is case-insensitive.
//
// Returns ErrNegativeValue if value is negative.
// Returns ErrInvalidUnit if the unit is not recognized.
// Returns ErrCalculationOverflow if value is Inf, NaN, or multiplication overflows.
func NormalizeToKg(value float64, unit string) (float64, error) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, ErrCalculationOverflow
	}

	if value < 0 {
		return 0, ErrNegativeValue
	}

	factor, ok := getUnitFactor(unit)
	if !ok {
		return 0, ErrInvalidUnit
	}

	result := value * factor

	// Check for overflow after multiplication
	if math.IsInf(result, 0) {
		return 0, ErrCalculationOverflow
	}

	return result, nil
}

// IsRecognizedUnit returns true if the unit string is valid for carbon values.
// Unit matching is case-insensitive.
func IsRecognizedUnit(unit string) bool {
	_, ok := getUnitFactor(unit)
	return ok
}
