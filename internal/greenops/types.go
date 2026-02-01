// Package greenops provides carbon emission equivalency calculations.
//
// It converts abstract carbon footprint values (kg CO2e) into relatable
// real-world equivalencies like "miles driven" or "smartphones charged"
// using EPA-published conversion factors.
package greenops

import "fmt"

// EquivalencyType represents a category of carbon emission equivalency.
type EquivalencyType int

const (
	// EquivalencyMilesDriven converts CO2e to miles driven in an average passenger vehicle.
	EquivalencyMilesDriven EquivalencyType = iota

	// EquivalencySmartphonesCharged converts CO2e to smartphone full charges.
	EquivalencySmartphonesCharged

	// EquivalencyTreeSeedlings converts CO2e to tree seedlings grown for 10 years.
	// Note: Optional/future use per spec assumptions.
	EquivalencyTreeSeedlings

	// EquivalencyHomeDays converts CO2e to days of average US home electricity use.
	// Note: Optional/future use.
	EquivalencyHomeDays
)

// String returns a human-readable representation of the EquivalencyType.
func (e EquivalencyType) String() string {
	switch e {
	case EquivalencyMilesDriven:
		return "MilesDriven"
	case EquivalencySmartphonesCharged:
		return "SmartphonesCharged"
	case EquivalencyTreeSeedlings:
		return "TreeSeedlings"
	case EquivalencyHomeDays:
		return "HomeDays"
	default:
		return fmt.Sprintf("EquivalencyType(%d)", e)
	}
}

// CarbonInput represents carbon emission data for equivalency calculation.
type CarbonInput struct {
	// Value is the numeric carbon emission amount.
	Value float64 `json:"value"`

	// Unit is the measurement unit (g, kg, t, gCO2e, kgCO2e, tCO2e, lb, lbCO2e).
	Unit string `json:"unit"`
}

// EquivalencyResult represents a single calculated equivalency.
type EquivalencyResult struct {
	// Type identifies the equivalency category.
	Type EquivalencyType `json:"type"`

	// Value is the raw calculated equivalency value.
	Value float64 `json:"value"`

	// FormattedValue is the display-ready string with separators/scaling.
	FormattedValue string `json:"formatted_value"`

	// Label is the descriptive phrase (e.g., "miles driven").
	Label string `json:"label"`
}

// EquivalencyOutput contains all equivalency results for display.
type EquivalencyOutput struct {
	// InputKg is the normalized input value in kilograms CO2e.
	InputKg float64 `json:"input_kg"`

	// Results contains calculated equivalencies in priority order.
	Results []EquivalencyResult `json:"results"`

	// DisplayText is the full prose format for CLI/TUI output.
	// Example: "Equivalent to driving ~781 miles or charging ~18,248 smartphones"
	DisplayText string `json:"display_text"`

	// CompactText is the abbreviated format for constrained outputs.
	// Example: "(≈ 781 mi, 18,248 phones)"
	CompactText string `json:"compact_text"`

	// IsEmpty returns true if no equivalencies were calculated.
	IsEmpty bool `json:"is_empty"`
}

// SustainabilityMetric mirrors engine.SustainabilityMetric for interface compatibility.
type SustainabilityMetric struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
