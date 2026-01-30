---
layout: default
title: GreenOps Package Architecture
description: Internal greenops package for carbon equivalency calculations
parent: Architecture
nav_order: 5
---

The `internal/greenops/` package provides carbon emission equivalency calculations
for FinFocus, converting abstract CO2e values into human-readable comparisons.

## Package Structure

```text
internal/greenops/
├── equivalency.go      # Core calculation functions
├── equivalency_test.go # Unit tests (colocated)
├── normalizer.go       # Unit conversion utilities
├── normalizer_test.go  # Normalizer tests
├── formatter.go        # Number formatting utilities
├── formatter_test.go   # Formatter tests
├── types.go            # Data structures and enums
├── constants.go        # EPA formulas and thresholds
└── errors.go           # Error type definitions
```

## Core Components

### Types (`types.go`)

```go
// EquivalencyType enumerates supported equivalency categories.
type EquivalencyType int

const (
    EquivalencyMilesDriven EquivalencyType = iota
    EquivalencySmartphonesCharged
    EquivalencyTreeSeedlings  // Optional/future
    EquivalencyHomeDays       // Optional/future
)

// CarbonInput represents carbon emission data for calculation.
type CarbonInput struct {
    Value float64  // Emission amount
    Unit  string   // Original unit (g, kg, t, lb)
}

// EquivalencyOutput contains calculated equivalencies.
type EquivalencyOutput struct {
    InputKg     float64             // Normalized input in kg
    Results     []EquivalencyResult // Calculated equivalencies
    DisplayText string              // Full prose format
    CompactText string              // Abbreviated format
    IsEmpty     bool                // True if below threshold
}

// EquivalencyResult represents a single equivalency calculation.
type EquivalencyResult struct {
    Type           EquivalencyType
    Value          float64
    FormattedValue string
    Label          string
}
```

### Constants (`constants.go`)

```go
// EPA formula constants (2024 edition)
const (
    EPAMilesDrivenFactor      = 0.393   // kg CO2e per mile (8.89÷22.8×1.006)
    EPASmartphoneChargeFactor = 0.00822 // kg CO2e per charge
    EPATreeSeedlingFactor     = 60.0    // kg absorbed per tree/10yr
    EPAHomeDayFactor          = 18.3    // kg per day US home
)

// Display thresholds
const (
    MinDisplayThresholdKg     = 0.001     // Minimum for any display
    MinEquivalencyThresholdKg = 1.0       // Minimum for equivalencies
    LargeNumberThreshold      = 1_000_000 // Abbreviated display
)

// Carbon metric keys
const (
    CarbonMetricKey       = "carbon_footprint" // Canonical
    DeprecatedCarbonKey   = "gCO2e"            // Legacy
)
```

### Core Functions (`equivalency.go`)

```go
// Calculate computes equivalencies from a CarbonInput.
// Returns IsEmpty=true if below MinEquivalencyThresholdKg.
func Calculate(input CarbonInput) (EquivalencyOutput, error)

// CalculateFromMap extracts carbon from a sustainability map.
// Checks canonical key first, falls back to deprecated.
func CalculateFromMap(metrics map[string]SustainabilityMetric) EquivalencyOutput
```

### Normalizer (`normalizer.go`)

```go
// NormalizeToKg converts any recognized unit to kilograms.
func NormalizeToKg(value float64, unit string) (float64, error)

// IsRecognizedUnit returns true for valid unit strings.
func IsRecognizedUnit(unit string) bool
```

### Formatter (`formatter.go`)

```go
// FormatNumber formats integers with thousand separators.
func FormatNumber(n int64) string

// FormatFloat formats floats with specified precision.
func FormatFloat(f float64, precision int) string

// FormatLarge abbreviates large numbers (million, billion).
func FormatLarge(n float64) string
```

## Data Flow

```text
┌─────────────────┐
│   CarbonInput   │
│ Value: 150.0    │
│ Unit: "kg"      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  NormalizeToKg  │───► Converts to kg (150.0)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Threshold Check │───► Skip if < 1.0 kg
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────────┐
│  EPA Formulas   │────►│ Miles: 150/0.393     │
│                 │     │ Phones: 150/0.00822  │
└────────┬────────┘     └──────────────────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────────────────────────┐
│  Format Values  │────►│ FormattedValue: "382", "18,248"      │
│                 │     │ DisplayText: "Equivalent to..."     │
└────────┬────────┘     │ CompactText: "(≈ 382 mi, 18,248...)" │
         │              └──────────────────────────────────────┘
         ▼
┌─────────────────┐
│ EquivalencyOut  │
└─────────────────┘
```

## Integration Points

### Engine (`internal/engine/project.go`)

```go
import "github.com/rshade/finfocus/internal/greenops"

// In renderSustainabilitySummary()
if metric, ok := sustainTotals["carbon_footprint"]; ok {
    input := greenops.CarbonInput{Value: metric.Value, Unit: metric.Unit}
    output, _ := greenops.Calculate(input)
    if !output.IsEmpty {
        fmt.Fprintf(w, "  %s\n", output.DisplayText)
    }
}
```

### TUI (`internal/tui/cost_view.go`)

```go
import "github.com/rshade/finfocus/internal/greenops"

// In RenderCostSummary()
if carbonInput, found := aggregateCarbonFromResults(results); found {
    output, _ := greenops.Calculate(carbonInput)
    if !output.IsEmpty {
        content.WriteString(SubtleStyle.Render(output.DisplayText))
    }
}
```

### Analyzer (`internal/analyzer/diagnostics.go`)

```go
import "github.com/rshade/finfocus/internal/greenops"

// In formatCostMessage()
if metric, ok := sustainability["carbon_footprint"]; ok {
    input := greenops.CarbonInput{Value: metric.Value, Unit: metric.Unit}
    output, _ := greenops.Calculate(input)
    if !output.IsEmpty {
        message += " " + output.CompactText
    }
}
```

## Error Handling

```go
var (
    ErrInvalidUnit         = errors.New("invalid carbon unit")
    ErrNegativeValue       = errors.New("negative carbon value")
    ErrCalculationOverflow = errors.New("calculation overflow")
)
```

All integration points handle errors gracefully:

- Invalid units: Log warning, skip equivalency display
- Negative values: Log warning, skip equivalency display
- Below threshold: Return IsEmpty=true, no display

## Testing Strategy

### Unit Tests

- `TestCalculate`: EPA formula accuracy within 1% margin
- `TestCalculateFromMap`: Canonical and legacy key handling
- `TestNormalizeToKg`: All unit conversions
- `TestFormatNumber/Float/Large`: Number formatting

### Integration Tests

- `test/integration/greenops_cli_test.go`: CLI output verification
- `internal/tui/cost_view_test.go`: TUI rendering tests
- `internal/analyzer/diagnostics_test.go`: Diagnostic format tests

### Coverage Target

- Minimum 80% for greenops package
- 95% for critical calculation paths

## Dependencies

- `golang.org/x/text/language`: Locale-aware formatting
- `golang.org/x/text/message`: Number formatting with separators
- `github.com/rs/zerolog`: Structured logging (for deprecation warnings)

No external APIs or network calls required.

## Future Extensions

The package is designed for extensibility:

1. **New equivalencies**: Add to `EquivalencyType` enum and formulas
2. **Regional variations**: Support locale-specific formulas
3. **Custom thresholds**: Allow user configuration
4. **Tree absorption**: Implement `EquivalencyTreeSeedlings` (commented out)
