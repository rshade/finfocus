package greenops

// EPA Formula Constants (2024 Edition)
// Source: https://www.epa.gov/energy/greenhouse-gas-equivalencies-calculator
//
// These constants represent the kg CO2e equivalent for each activity.
// To calculate the equivalency, divide the carbon value by the factor:
//
//	equivalency = kg_CO2e / factor
const (
	// EPAMilesDrivenFactor is kg CO2e per mile for average passenger vehicle.
	// Source: EPA GHG Equivalencies Calculator (2024 edition).
	// Reference: https://www.epa.gov/energy/greenhouse-gas-equivalencies-calculator-calculations-and-references
	//
	// Derivation from EPA 2024 data:
	//   - CO2 per gallon gasoline: 8.89 kg (8.89 × 10⁻³ metric tons)
	//   - Average fuel economy: 22.8 miles per gallon
	//   - GHG adjustment (CH4 + N2O): 1/0.994 = 1.006
	//   - Result: 8.89 ÷ 22.8 × 1.006 = 0.392 ≈ 0.393 kg CO2e/mile
	//   - EPA reference: 3.93 × 10⁻⁴ metric tons/mile = 0.393 kg/mile
	//
	// Note: This is the divisor used in the equivalency formula (kg_CO2e / factor).
	EPAMilesDrivenFactor = 0.393

	// EPASmartphoneChargeFactor is kg CO2e per smartphone charge.
	// Based on average smartphone battery capacity and grid carbon intensity.
	EPASmartphoneChargeFactor = 0.00822

	// EPATreeSeedlingFactor is kg CO2e absorbed per tree seedling over 10 years.
	// Based on urban tree carbon sequestration rates.
	EPATreeSeedlingFactor = 60.0

	// EPAHomeDayFactor is kg CO2e per day of average US home electricity.
	// Based on average residential electricity consumption and grid intensity.
	EPAHomeDayFactor = 18.3
)

// Unit Conversion Constants for normalizing carbon values to kilograms.
const (
	// GramsToKg converts grams to kilograms.
	GramsToKg = 0.001

	// KgToKg is the identity conversion for kilograms.
	KgToKg = 1.0

	// TonsToKg converts metric tons to kilograms.
	TonsToKg = 1000.0

	// PoundsToKg converts pounds to kilograms.
	PoundsToKg = 0.453592
)

// Display Threshold Constants control when equivalencies are shown.
const (
	// MinDisplayThresholdKg is the minimum kg CO2e for any display.
	// Values below this are effectively zero and not displayed.
	MinDisplayThresholdKg = 0.001

	// MinEquivalencyThresholdKg is the minimum kg CO2e for showing equivalencies.
	// Below this threshold, raw values are shown without equivalencies
	// because the equivalencies become meaninglessly small.
	MinEquivalencyThresholdKg = 1.0

	// LargeNumberThreshold is the threshold for using abbreviated display.
	// Values at or above this threshold use "~X.X million" format.
	LargeNumberThreshold = 1_000_000

	// BillionThreshold is the threshold for billion-scale display.
	BillionThreshold = 1_000_000_000
)

// Metric Keys for sustainability maps.
const (
	// CarbonMetricKey is the canonical key for carbon footprint in sustainability maps.
	CarbonMetricKey = "carbon_footprint"

	// DeprecatedCarbonKey is the legacy key (deprecated, for backward compatibility).
	DeprecatedCarbonKey = "gCO2e"
)
