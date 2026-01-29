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
//	// output.DisplayText == "Equivalent to driving ~781 miles or charging ~18,248 smartphones"
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
//	output := CalculateFromMap(metrics)
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
// comma-separated integers.
func formatEquivalencyValue(v float64) string {
	if v >= LargeNumberThreshold {
		return FormatLarge(v)
	}
	return FormatNumber(int64(math.Round(v)))
}
