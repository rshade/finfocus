# Quickstart: GreenOps Impact Equivalencies

**Feature**: 125-greenops-equivalencies
**Date**: 2026-01-27
**Audience**: Developers implementing or extending this feature

## Overview

This guide covers how to implement and integrate carbon emission equivalencies into FinFocus. The feature converts abstract carbon footprint values (kg CO2e) into relatable real-world equivalencies like "miles driven" or "smartphones charged".

## Prerequisites

- Go 1.25.5+
- Familiarity with FinFocus codebase (`internal/` package structure)
- Understanding of existing sustainability metrics pipeline

## Quick Implementation

### Step 1: Create the greenops Package

Create `internal/greenops/` with the core calculator:

```go
// internal/greenops/equivalency.go
package greenops

import (
    "fmt"
    "math"
    "strings"

    "golang.org/x/text/language"
    "golang.org/x/text/message"
)

// EPA Formula Constants (2024 Edition)
// Source: https://www.epa.gov/energy/greenhouse-gas-equivalencies-calculator
const (
    EPAMilesDrivenFactor      = 0.192   // kg CO2e per mile
    EPASmartphoneChargeFactor = 0.00822 // kg CO2e per smartphone charge
)

// Calculate computes carbon equivalencies from a CarbonInput.
func Calculate(input CarbonInput) (EquivalencyOutput, error) {
    // Normalize to kg
    kg, err := normalizeToKg(input.Value, input.Unit)
    if err != nil {
        return EquivalencyOutput{IsEmpty: true}, err
    }

    // Skip if below threshold
    if kg < MinEquivalencyThresholdKg {
        return EquivalencyOutput{InputKg: kg, IsEmpty: true}, nil
    }

    // Calculate equivalencies
    miles := kg / EPAMilesDrivenFactor
    phones := kg / EPASmartphoneChargeFactor

    p := message.NewPrinter(language.English)
    milesStr := formatValue(p, miles)
    phonesStr := formatValue(p, phones)

    return EquivalencyOutput{
        InputKg: kg,
        Results: []EquivalencyResult{
            {Type: EquivalencyMilesDriven, Value: miles, FormattedValue: milesStr, Label: "miles driven"},
            {Type: EquivalencySmartphonesCharged, Value: phones, FormattedValue: phonesStr, Label: "smartphones charged"},
        },
        DisplayText: fmt.Sprintf("Equivalent to driving ~%s miles or charging ~%s smartphones", milesStr, phonesStr),
        CompactText: fmt.Sprintf("(≈ %s mi, %s phones)", milesStr, phonesStr),
        IsEmpty:     false,
    }, nil
}

func formatValue(p *message.Printer, v float64) string {
    if v >= LargeNumberThreshold {
        millions := v / 1_000_000
        return fmt.Sprintf("~%.1f million", millions)
    }
    return p.Sprintf("%d", int64(math.Round(v)))
}
```

### Step 2: Integrate with Engine Rendering

Modify `internal/engine/project.go` to display equivalencies:

```go
// In renderSustainabilitySummary() after aggregation

if metric, ok := sustainTotals["carbon_footprint"]; ok {
    input := greenops.CarbonInput{Value: metric.Value, Unit: metric.Unit}
    output, err := greenops.Calculate(input)
    if err == nil && !output.IsEmpty {
        fmt.Fprintf(w, "  %s\n", output.DisplayText)
    }
}
```

### Step 3: Integrate with TUI

Modify `internal/tui/cost_view.go`:

```go
// In RenderCostSummary() after provider breakdown

// Add carbon equivalency if present
if carbonMetric, ok := aggregateCarbonFromResults(results); ok {
    input := greenops.CarbonInput{Value: carbonMetric.Value, Unit: carbonMetric.Unit}
    output, _ := greenops.Calculate(input)
    if !output.IsEmpty {
        carbonLine := lipgloss.NewStyle().
            Foreground(lipgloss.Color("34")).
            Render(output.DisplayText)
        summary.WriteString("\n" + carbonLine)
    }
}
```

### Step 4: Integrate with Analyzer

Modify `internal/analyzer/diagnostics.go`:

```go
// In formatCostMessage() after sustainability metrics

if carbonMetric, ok := result.Sustainability["carbon_footprint"]; ok {
    input := greenops.CarbonInput{Value: carbonMetric.Value, Unit: carbonMetric.Unit}
    output, _ := greenops.Calculate(input)
    if !output.IsEmpty {
        message += " " + output.CompactText
    }
}
```

## Testing

### Unit Tests

Create `internal/greenops/equivalency_test.go`:

```go
func TestCalculate(t *testing.T) {
    tests := []struct {
        name      string
        input     CarbonInput
        wantMiles float64
        wantErr   bool
    }{
        {
            name:      "150kg carbon",
            input:     CarbonInput{Value: 150.0, Unit: "kg"},
            wantMiles: 781.25, // 150 / 0.192
            wantErr:   false,
        },
        {
            name:      "below threshold",
            input:     CarbonInput{Value: 0.5, Unit: "kg"},
            wantMiles: 0,
            wantErr:   false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            output, err := Calculate(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)

            if tt.wantMiles > 0 {
                // Verify within 1% margin (SC-002)
                assert.InDelta(t, tt.wantMiles, output.Results[0].Value, tt.wantMiles*0.01)
            }
        })
    }
}
```

### Run Tests

```bash
# Run greenops package tests
go test -v ./internal/greenops/...

# Run with coverage
go test -coverprofile=coverage.out ./internal/greenops/...
go tool cover -func=coverage.out

# Verify 80%+ coverage (SC-005)
```

## API Reference

### Primary Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `Calculate` | `(CarbonInput) (EquivalencyOutput, error)` | Compute equivalencies for carbon input |
| `CalculateFromMap` | `(map[string]SustainabilityMetric) EquivalencyOutput` | Extract and calculate from metrics map |

### Types

| Type | Purpose |
|------|---------|
| `CarbonInput` | Input with value and unit |
| `EquivalencyOutput` | Results with formatted display text |
| `EquivalencyResult` | Single equivalency calculation |
| `EquivalencyType` | Enum for equivalency categories |

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `EPAMilesDrivenFactor` | 0.192 | kg CO2e per mile |
| `EPASmartphoneChargeFactor` | 0.00822 | kg CO2e per phone charge |
| `MinEquivalencyThresholdKg` | 1.0 | Minimum kg for equivalency display |

## Common Patterns

### Graceful Degradation

```go
output, err := greenops.Calculate(input)
if err != nil {
    log.Warn().Err(err).Msg("equivalency calculation failed")
    // Continue without equivalency display
}
if output.IsEmpty {
    // No equivalency to display (below threshold or zero)
}
```

### Unit Normalization

```go
// All of these produce the same kg value
greenops.Calculate(CarbonInput{Value: 150, Unit: "kg"})      // 150 kg
greenops.Calculate(CarbonInput{Value: 150000, Unit: "g"})    // 150 kg
greenops.Calculate(CarbonInput{Value: 0.15, Unit: "t"})      // 150 kg
```

## Troubleshooting

### No Equivalency Output

1. Check if carbon value is above threshold (1.0 kg)
2. Verify unit is recognized (g, kg, t, gCO2e, kgCO2e, tCO2e)
3. Check logs for warning messages about invalid units

### Incorrect Formatting

1. Verify `golang.org/x/text` dependency is installed
2. Check locale settings (defaults to English)

### Integration Issues

1. Ensure `greenops` package is imported correctly
2. Verify sustainability metrics contain `"carbon_footprint"` key
3. Check that engine/TUI/analyzer code paths are reached

## Next Steps

After implementing the core feature:

1. Add integration tests in `test/integration/greenops_test.go`
2. Update user documentation in `docs/guides/greenops-equivalencies.md`
3. Add architecture documentation in `docs/architecture/greenops-package.md`
4. Run full test suite: `make test && make lint`
