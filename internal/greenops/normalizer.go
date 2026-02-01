package greenops

import (
	"math"
	"strings"
)

// getUnitFactor returns the conversion factor for a unit string.
// Returns (factor, true) for valid units, (0, false) for invalid.
// getUnitFactor returns the conversion factor to kilograms for the provided unit and a boolean indicating whether the unit is recognized.
// The unit matching is case-insensitive. Recognized inputs and their mappings are:
// "g", "gCO2e" -> GramsToKg; "kg", "kgCO2e" -> KgToKg; "t", "tCO2e" -> TonsToKg; "lb", "lbCO2e" -> PoundsToKg.
// For unrecognized units it returns 0 and false.
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
// NormalizeToKg converts a carbon quantity from the provided unit to kilograms.
// Supported units are "g", "kg", "t", "lb" and their "CO2e" variants (case-insensitive).
// Returns the converted value in kilograms. It returns ErrNegativeValue if value is less than zero,
// ErrInvalidUnit if the unit is not recognized, and ErrCalculationOverflow if the input is Inf or NaN
// or if the multiplication results in an overflow.
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
// IsRecognizedUnit reports whether the provided unit string is a supported carbon unit.
// It accepts "g", "kg", "t", "lb" and their "CO2e" variants (for example "gCO2e", "kgCO2e"),
// matching case-insensitively. It returns true if the unit is recognized, false otherwise.
func IsRecognizedUnit(unit string) bool {
	_, ok := getUnitFactor(unit)
	return ok
}
