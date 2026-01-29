// Package contracts defines the public interface for the greenops package.
// This file serves as a contract specification for the equivalency calculator.
//
// NOTE: This is a design contract, not production code. The actual implementation
// will be in internal/greenops/.
package contracts

// Calculator defines the interface for carbon equivalency calculations.
// Implementations must be thread-safe for concurrent use.
type Calculator interface {
	// Calculate computes equivalencies for the given carbon input.
	// Returns an EquivalencyOutput with formatted display text.
	//
	// If the input value is below MinDisplayThresholdKg, returns an empty output.
	// If the unit is unrecognized, returns an error.
	//
	// Example:
	//   input := CarbonInput{Value: 150.0, Unit: "kg"}
	//   output, err := calc.Calculate(input)
	//   // output.DisplayText == "Equivalent to driving ~781 miles or charging ~18,248 smartphones"
	Calculate(input CarbonInput) (EquivalencyOutput, error)

	// CalculateFromMap extracts carbon data from a sustainability metrics map
	// and calculates equivalencies.
	//
	// Looks for "carbon_footprint" key first, falls back to deprecated "gCO2e".
	// Returns empty output if no carbon metric is found.
	//
	// Example:
	//   metrics := map[string]SustainabilityMetric{
	//       "carbon_footprint": {Value: 150.0, Unit: "kg"},
	//   }
	//   output := calc.CalculateFromMap(metrics)
	CalculateFromMap(metrics map[string]SustainabilityMetric) EquivalencyOutput
}

// CarbonInput represents carbon emission data for equivalency calculation.
type CarbonInput struct {
	// Value is the numeric carbon emission amount.
	Value float64

	// Unit is the measurement unit (g, kg, t, gCO2e, kgCO2e, tCO2e, lb).
	Unit string
}

// EquivalencyOutput contains all equivalency results for display.
type EquivalencyOutput struct {
	// InputKg is the normalized input value in kilograms CO2e.
	InputKg float64

	// Results contains calculated equivalencies in priority order.
	Results []EquivalencyResult

	// DisplayText is the full prose format for CLI/TUI output.
	// Example: "Equivalent to driving ~781 miles or charging ~18,248 smartphones"
	DisplayText string

	// CompactText is the abbreviated format for constrained outputs.
	// Example: "(≈ 781 mi, 18,248 phones)"
	CompactText string

	// IsEmpty returns true if no equivalencies were calculated.
	IsEmpty bool
}

// EquivalencyResult represents a single calculated equivalency.
type EquivalencyResult struct {
	// Type identifies the equivalency category.
	Type EquivalencyType

	// Value is the raw calculated equivalency value.
	Value float64

	// FormattedValue is the display-ready string with separators/scaling.
	FormattedValue string

	// Label is the descriptive phrase (e.g., "miles driven").
	Label string
}

// EquivalencyType enumerates supported equivalency categories.
type EquivalencyType int

const (
	// EquivalencyMilesDriven - CO2e to miles in average passenger vehicle.
	EquivalencyMilesDriven EquivalencyType = iota

	// EquivalencySmartphonesCharged - CO2e to smartphone full charges.
	EquivalencySmartphonesCharged

	// EquivalencyTreeSeedlings - CO2e to tree seedlings grown 10 years (optional).
	EquivalencyTreeSeedlings

	// EquivalencyHomeDays - CO2e to days of US home electricity (optional).
	EquivalencyHomeDays
)

// SustainabilityMetric mirrors engine.SustainabilityMetric for interface definition.
type SustainabilityMetric struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// Formatter defines the interface for number formatting utilities.
type Formatter interface {
	// FormatNumber formats a number with thousand separators.
	// Example: FormatNumber(18248) == "18,248"
	FormatNumber(n int64) string

	// FormatFloat formats a float with specified precision and separators.
	// Example: FormatFloat(18248.56, 0) == "18,249"
	FormatFloat(f float64, precision int) string

	// FormatLarge formats large numbers with abbreviated notation.
	// Example: FormatLarge(5200000) == "~5.2 million"
	FormatLarge(n float64) string
}

// UnitNormalizer defines the interface for carbon unit conversion.
type UnitNormalizer interface {
	// NormalizeToKg converts a value in any recognized unit to kilograms.
	// Returns an error for unrecognized units.
	//
	// Recognized units: g, kg, t, gCO2e, kgCO2e, tCO2e, lb, lbCO2e
	NormalizeToKg(value float64, unit string) (float64, error)

	// IsRecognizedUnit returns true if the unit string is valid.
	IsRecognizedUnit(unit string) bool
}

// Error types for equivalency calculations.
var (
	// ErrInvalidUnit indicates an unrecognized carbon unit.
	ErrInvalidUnit = constError("invalid carbon unit")

	// ErrNegativeValue indicates a negative carbon value.
	ErrNegativeValue = constError("negative carbon value")

	// ErrNoCarbon indicates no carbon metric was found in the input.
	ErrNoCarbon = constError("no carbon metric found")

	// ErrCalculationOverflow indicates a value too large to calculate safely.
	ErrCalculationOverflow = constError("calculation overflow")
)

// constError is an immutable error type for sentinel errors.
type constError string

func (e constError) Error() string { return string(e) }

// Constants for EPA formula factors (2024 edition).
const (
	// EPAMilesDrivenFactor: kg CO2e per mile (inverted for division).
	// Source: EPA GHG Equivalencies Calculator 2024
	// https://www.epa.gov/energy/greenhouse-gas-equivalencies-calculator
	EPAMilesDrivenFactor = 0.192

	// EPASmartphoneChargeFactor: kg CO2e per smartphone charge.
	EPASmartphoneChargeFactor = 0.00822

	// EPATreeSeedlingFactor: kg CO2e absorbed per tree seedling over 10 years.
	EPATreeSeedlingFactor = 60.0

	// EPAHomeDayFactor: kg CO2e per day of average US home electricity.
	EPAHomeDayFactor = 18.3
)

// Constants for display thresholds.
const (
	// MinDisplayThresholdKg: minimum kg for any equivalency display.
	MinDisplayThresholdKg = 0.001

	// MinEquivalencyThresholdKg: minimum kg for equivalency calculations.
	MinEquivalencyThresholdKg = 1.0

	// LargeNumberThreshold: threshold for abbreviated display.
	LargeNumberThreshold = 1_000_000
)

// CarbonMetricKey is the canonical key for carbon footprint in sustainability maps.
const CarbonMetricKey = "carbon_footprint"

// DeprecatedCarbonKey is the legacy key (deprecated, for backward compatibility).
const DeprecatedCarbonKey = "gCO2e"
