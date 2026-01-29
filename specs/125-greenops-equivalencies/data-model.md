# Data Model: GreenOps Impact Equivalencies

**Feature**: 125-greenops-equivalencies
**Date**: 2026-01-27
**Status**: Complete

## Overview

This document defines the data structures and entities for the carbon emission equivalencies feature. The design follows Go idioms and integrates with existing FinFocus engine types.

## Entity Definitions

### 1. EquivalencyType

An enumeration of supported carbon equivalency types.

```go
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
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| (enum) | int | Iota-based enumeration for type safety |

**Validation Rules**:

- Only defined constants are valid
- Primary display uses MilesDriven and SmartphonesCharged (per spec FR-004)

### 2. EquivalencyFormula

Represents the conversion formula for a specific equivalency type.

```go
// EquivalencyFormula contains the conversion factor and metadata for an equivalency calculation.
type EquivalencyFormula struct {
    // Type identifies this formula's equivalency category.
    Type EquivalencyType

    // Factor is the divisor applied to kg CO2e to produce the equivalency value.
    // Example: 0.192 for miles driven (kg_CO2e / 0.192 = miles).
    Factor float64

    // Unit is the display unit for the equivalency result (e.g., "miles", "smartphones").
    Unit string

    // Label is the human-readable description used in output.
    // Example: "miles driven" or "smartphones charged".
    Label string

    // Source documents the formula origin for auditability.
    Source string

    // Version indicates the formula version/date.
    Version string
}
```

**Fields**:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| Type | EquivalencyType | Enum identifier | EquivalencyMilesDriven |
| Factor | float64 | Conversion divisor (kg CO2e / Factor = result) | 0.192 |
| Unit | string | Result unit for display | "miles" |
| Label | string | Human-readable phrase | "miles driven" |
| Source | string | Formula source URL | "EPA GHG Equivalencies Calculator" |
| Version | string | Formula version date | "2024" |

**Validation Rules**:

- Factor must be > 0
- Unit and Label must be non-empty strings

### 3. EquivalencyResult

The computed equivalency value for a given carbon amount.

```go
// EquivalencyResult represents a calculated carbon equivalency with formatted display.
type EquivalencyResult struct {
    // Type identifies the equivalency category.
    Type EquivalencyType

    // Value is the raw calculated equivalency value.
    Value float64

    // FormattedValue is the display-ready string with appropriate scaling and separators.
    // Example: "18,248" or "~5.2 million".
    FormattedValue string

    // Label is the descriptive phrase for the equivalency.
    // Example: "miles driven".
    Label string
}
```

**Fields**:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| Type | EquivalencyType | Enum identifier | EquivalencyMilesDriven |
| Value | float64 | Raw numeric result | 18248.456 |
| FormattedValue | string | Display-formatted value | "18,248" |
| Label | string | Descriptive phrase | "miles driven" |

**Validation Rules**:

- Value must be >= 0 (negative CO2e is invalid)
- FormattedValue must be non-empty for display

### 4. CarbonInput

Input structure for equivalency calculations.

```go
// CarbonInput represents carbon emission data to be converted to equivalencies.
type CarbonInput struct {
    // Value is the numeric carbon emission amount.
    Value float64

    // Unit is the original unit of measurement (g, kg, t, etc.).
    Unit string
}
```

**Fields**:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| Value | float64 | Emission amount | 150.5 |
| Unit | string | Original unit | "kg" |

**Validation Rules**:

- Value must be >= 0
- Unit must be a recognized unit (g, kg, t, gCO2e, kgCO2e, tCO2e, lb, lbCO2e)

### 5. EquivalencyOutput

Aggregated output structure for display formatting.

```go
// EquivalencyOutput contains all equivalency results for a carbon input.
type EquivalencyOutput struct {
    // InputKg is the normalized input value in kilograms CO2e.
    InputKg float64

    // Results contains calculated equivalencies, ordered by priority.
    Results []EquivalencyResult

    // DisplayText is the fully formatted output string for CLI/TUI.
    // Example: "Equivalent to driving ~781 miles or charging ~18,248 smartphones".
    DisplayText string

    // CompactText is the abbreviated format for constrained outputs (e.g., Analyzer).
    // Example: "(≈ 781 mi, 18,248 phones)".
    CompactText string

    // IsEmpty indicates no equivalencies were calculated (below threshold or error).
    IsEmpty bool
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| InputKg | float64 | Normalized input in kg |
| Results | []EquivalencyResult | Ordered equivalency calculations |
| DisplayText | string | Full prose format |
| CompactText | string | Abbreviated format |
| IsEmpty | bool | True if no equivalencies calculated |

**State Transitions**: N/A (immutable output structure)

## Relationships

```text
CarbonInput ──────► Calculator ──────► EquivalencyOutput
                        │                    │
                        ▼                    ▼
               EquivalencyFormula[]    EquivalencyResult[]
```

**Cardinality**:

- 1 CarbonInput → 1 EquivalencyOutput
- 1 EquivalencyOutput → N EquivalencyResults (typically 2: miles + smartphones)
- N EquivalencyFormulas → static registry (constants)

## Integration with Existing Types

### engine.SustainabilityMetric

The existing type that provides carbon input:

```go
// From internal/engine/types.go
type SustainabilityMetric struct {
    Value float64 `json:"value"`
    Unit  string  `json:"unit"`
}
```

**Integration Pattern**:

```go
// In internal/engine/project.go (renderSustainabilitySummary)
if metric, ok := sustainTotals["carbon_footprint"]; ok {
    input := greenops.CarbonInput{Value: metric.Value, Unit: metric.Unit}
    output := greenops.Calculate(input)
    // Append output.DisplayText to summary
}
```

### engine.CostResult

Contains the sustainability map:

```go
// From internal/engine/types.go
type CostResult struct {
    // ... other fields
    Sustainability map[string]SustainabilityMetric `json:"sustainability,omitempty"`
}
```

**Access Pattern**: `result.Sustainability["carbon_footprint"]`

## Constants

> **Note**: The authoritative source for all constants is `contracts/equivalency.go`.
> The values below are duplicated for documentation purposes and must stay synchronized.

### EPA Formula Constants

```go
const (
    // EPAMilesDrivenFactor is kg CO2e per mile for average passenger vehicle.
    // Source: EPA GHG Equivalencies Calculator (2024)
    // Calculation: 8.89 kg CO2/gallon ÷ 21.6 mpg = 0.411 kg/mi → 1 mi = 0.192 kg inverted
    EPAMilesDrivenFactor = 0.192

    // EPASmartphoneChargeFactor is kg CO2e per smartphone charge.
    // Source: EPA GHG Equivalencies Calculator (2024)
    EPASmartphoneChargeFactor = 0.00822

    // EPATreeSeedlingFactor is kg CO2e absorbed by tree seedling over 10 years.
    // Source: EPA GHG Equivalencies Calculator (2024)
    EPATreeSeedlingFactor = 60.0

    // EPAHomeDayFactor is kg CO2e per day of average US home electricity.
    // Source: EPA GHG Equivalencies Calculator (2024)
    EPAHomeDayFactor = 18.3
)
```

### Unit Conversion Constants

```go
const (
    // GramsPerKilogram for unit normalization.
    GramsPerKilogram = 1000.0

    // KilogramsPerMetricTon for unit normalization.
    KilogramsPerMetricTon = 1000.0

    // KilogramsPerPound for unit normalization.
    KilogramsPerPound = 0.453592
)
```

### Display Thresholds

```go
const (
    // MinDisplayThresholdKg is the minimum kg CO2e for equivalency display.
    MinDisplayThresholdKg = 0.001

    // MinEquivalencyThresholdKg is the minimum kg CO2e for showing equivalencies.
    MinEquivalencyThresholdKg = 1.0

    // LargeNumberThreshold for abbreviated display (e.g., "5.2 million").
    LargeNumberThreshold = 1_000_000
)
```

## Error Handling

### Error Types

```go
var (
    // ErrInvalidUnit indicates an unrecognized carbon unit.
    ErrInvalidUnit = errors.New("invalid carbon unit")

    // ErrNegativeValue indicates a negative carbon value.
    ErrNegativeValue = errors.New("negative carbon value")

    // ErrCalculationOverflow indicates a value too large to calculate.
    ErrCalculationOverflow = errors.New("calculation overflow")
)
```

### Graceful Degradation

When errors occur, the system degrades gracefully:

| Error | Behavior |
|-------|----------|
| Invalid unit | Log warning, skip equivalency (show raw value only) |
| Negative value | Log warning, skip equivalency |
| Overflow | Cap at max display value, log warning |

## Example Usage

### CLI Table Output

```text
SUSTAINABILITY SUMMARY
======================
carbon_footprint:      150.00 kg
  Equivalent to driving ~781 miles or charging ~18,248 smartphones
energy_consumption:    2000.00 kWh
```

### TUI Summary Box

```text
╭──────────────────────────────────────────────╮
│ Cost Summary                                  │
│ Total: $245.50 USD                           │
│ Resources: 5                                  │
│                                              │
│ Carbon Impact: 150.00 kg CO2e                │
│ (Equivalent to driving ~781 miles)           │
╰──────────────────────────────────────────────╯
```

### Analyzer Diagnostic

```text
Estimated Monthly Cost: $245.50 USD (source: aws-public)
 [carbon_footprint: 150.00 kg (≈ 781 mi, 18,248 phones)]
```
