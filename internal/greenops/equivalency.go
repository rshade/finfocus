package greenops

import (
	"fmt"
	"math"

	"github.com/rs/zerolog/log"
)

// Calculate computes carbon equivalencies for the given carbon input.
//
// It normalizes the input value to kilograms, then calculates equivalencies
// using EPA formulas for miles driven and smartphones charged.
//
// Returns an empty output if the input is below MinEquivalencyThresholdKg.
// Returns an error for invalid units or negative values.
//
// Example:
//
//	input := CarbonInput{Value: 150.0, Unit: "kg"}
//	output, err := Calculate(input)
// Calculate converts a CarbonInput to kilograms and computes EPA-based equivalencies
// expressed as miles driven and smartphones charged.
//
// If normalization to kilograms fails, Calculate returns an empty EquivalencyOutput
// (IsEmpty = true) and the normalization error. If the normalized kilogram value is
// below MinEquivalencyThresholdKg, Calculate returns an empty EquivalencyOutput
// with InputKg set to the normalized value and no error. If numeric overflow or
// invalid results occur during equivalency calculation, Calculate returns an empty
// EquivalencyOutput with ErrCalculationOverflow.
//
// On success, the returned EquivalencyOutput contains InputKg, a Results slice with
// EquivalencyMilesDriven and EquivalencySmartphonesCharged entries (each containing
// the raw and formatted values and a label), a human-readable DisplayText of the
// form "Equivalent to driving ~{miles} miles or charging ~{phones} smartphones",
// a CompactText for diagnostics "(≈ {miles} mi, {phones} phones)", and IsEmpty = false.
func Calculate(input CarbonInput) (EquivalencyOutput, error) {
	// Normalize to kg
	kg, err := NormalizeToKg(input.Value, input.Unit)
	if err != nil {
		return EquivalencyOutput{IsEmpty: true}, err
	}

	// Check threshold
	if kg < MinEquivalencyThresholdKg {
		return EquivalencyOutput{InputKg: kg, IsEmpty: true}, nil
	}

	// Calculate equivalencies using EPA formulas
	miles := kg / EPAMilesDrivenFactor
	phones := kg / EPASmartphoneChargeFactor

	// Defensive check: ensure division results are valid
	if math.IsInf(miles, 0) || math.IsNaN(miles) ||
		math.IsInf(phones, 0) || math.IsNaN(phones) {
		return EquivalencyOutput{IsEmpty: true}, ErrCalculationOverflow
	}

	// Format values for display
	milesFormatted := formatEquivalencyValue(miles)
	phonesFormatted := formatEquivalencyValue(phones)

	// Build results
	results := []EquivalencyResult{
		{
			Type:           EquivalencyMilesDriven,
			Value:          miles,
			FormattedValue: milesFormatted,
			Label:          "miles driven",
		},
		{
			Type:           EquivalencySmartphonesCharged,
			Value:          phones,
			FormattedValue: phonesFormatted,
			Label:          "smartphones charged",
		},
	}

	// Build display text (FR-003: "Equivalent to" labeling)
	displayText := fmt.Sprintf("Equivalent to driving ~%s miles or charging ~%s smartphones",
		milesFormatted, phonesFormatted)

	// Build compact text for analyzer diagnostics
	compactText := fmt.Sprintf("(≈ %s mi, %s phones)", milesFormatted, phonesFormatted)

	return EquivalencyOutput{
		InputKg:     kg,
		Results:     results,
		DisplayText: displayText,
		CompactText: compactText,
		IsEmpty:     false,
	}, nil
}

// CalculateFromMap extracts carbon data from a sustainability metrics map
// and calculates equivalencies.
//
// It looks for the "carbon_footprint" key first (canonical), then falls back
// to the deprecated "gCO2e" key for backward compatibility.
//
// Returns an empty output if no carbon metric is found or if the value is
// below the threshold.
//
// Example:
//
//	metrics := map[string]SustainabilityMetric{
//	    "carbon_footprint": {Value: 150.0, Unit: "kg"},
//	}
// CalculateFromMap extracts a carbon metric from the provided metrics map and computes equivalencies.
// 
// If the map is nil or contains neither the canonical key "carbon_footprint" nor the deprecated key "gCO2e",
// the function returns an empty EquivalencyOutput with IsEmpty set to true.
// The function prefers the canonical key; if only the deprecated key is present a deprecation warning is logged.
// If equivalency computation fails for a found metric, a warning is logged and an empty EquivalencyOutput is returned.
//
// Parameters:
//   - metrics: a map of metric keys to SustainabilityMetric values from which a carbon metric may be read.
//
// Returns:
//   - EquivalencyOutput containing computed equivalencies when successful, or an empty EquivalencyOutput with IsEmpty = true on missing data or computation failure.
func CalculateFromMap(metrics map[string]SustainabilityMetric) EquivalencyOutput {
	if metrics == nil {
		return EquivalencyOutput{IsEmpty: true}
	}

	// Check canonical key first.
	if metric, ok := metrics[CarbonMetricKey]; ok {
		output, err := Calculate(CarbonInput(metric))
		if err != nil {
			log.Warn().Err(err).Msg("equivalency calculation failed for carbon_footprint")
			return EquivalencyOutput{IsEmpty: true}
		}
		return output
	}

	// Fallback to deprecated key with warning (FR-009).
	if metric, ok := metrics[DeprecatedCarbonKey]; ok {
		log.Warn().Msg("deprecated key 'gCO2e' used, prefer 'carbon_footprint'")
		output, err := Calculate(CarbonInput(metric))
		if err != nil {
			log.Warn().Err(err).Msg("equivalency calculation failed for gCO2e")
			return EquivalencyOutput{IsEmpty: true}
		}
		return output
	}

	return EquivalencyOutput{IsEmpty: true}
}

// formatEquivalencyValue formats an equivalency value for display.
// Uses large number scaling for million/billion values, otherwise
// formatEquivalencyValue formats a floating-point equivalency value for display.
// If v is greater than or equal to LargeNumberThreshold it returns a compact
// large-number representation; otherwise it rounds v to the nearest integer and
// returns a comma-separated integer string.
func formatEquivalencyValue(v float64) string {
	if v >= LargeNumberThreshold {
		return FormatLarge(v)
	}
	return FormatNumber(int64(math.Round(v)))
}