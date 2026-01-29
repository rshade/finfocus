// Package greenops provides carbon emission equivalency calculations.
//
// It converts abstract carbon footprint values (kg CO2e) into relatable
// real-world equivalencies like "miles driven" or "smartphones charged"
// using EPA-published conversion factors.
package greenops

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

// CarbonInput represents carbon emission data for equivalency calculation.
type CarbonInput struct {
	// Value is the numeric carbon emission amount.
	Value float64

	// Unit is the measurement unit (g, kg, t, gCO2e, kgCO2e, tCO2e, lb, lbCO2e).
	Unit string
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

// SustainabilityMetric mirrors engine.SustainabilityMetric for interface compatibility.
type SustainabilityMetric struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
