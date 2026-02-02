---
layout: default
title: GreenOps Carbon Equivalencies
description: User guide for carbon emission equivalencies
parent: Guides
nav_order: 5
---

FinFocus displays human-readable carbon emission equivalencies alongside sustainability
metrics, making environmental impact more tangible and understandable.

## Overview

When analyzing cloud infrastructure costs, FinFocus calculates carbon footprint values
from provider data. The equivalencies feature converts abstract CO2e values into
relatable real-world comparisons like "miles driven" or "smartphones charged."

## Where Equivalencies Appear

### CLI Output

When running `finfocus cost projected`, equivalencies appear in the sustainability
summary:

```text
SUSTAINABILITY SUMMARY
======================
carbon_footprint:      150.00 kg
  Equivalent to driving ~382 miles or charging ~18,248 smartphones
energy_consumption:    2000.00 kWh
```

### TUI Summary

In the interactive TUI view, equivalencies appear below the provider breakdown:

```text
╭──────────────────────────────────────────────────╮
│ COST SUMMARY                                     │
│ Total Cost:    $245.50    Resources: 5           │
│ aws: $200.00 (81.5%)  gcp: $45.50 (18.5%)       │
│ Equivalent to driving ~382 miles or charging     │
│ ~18,248 smartphones                              │
╰──────────────────────────────────────────────────╯
```

### Analyzer Diagnostics

During `pulumi preview`, equivalencies appear in compact format:

```text
warning: finfocus: Estimated Monthly Cost: $245.50 USD (source: aws-public)
         [carbon_footprint: 150.00 kg] (≈ 382 mi, 18,248 phones)
```

## EPA Formulas

Equivalencies are calculated using EPA Greenhouse Gas Equivalencies Calculator
formulas (2024 edition):

| Equivalency         | Formula             | Explanation                                                        |
| ------------------- | ------------------- | ------------------------------------------------------------------ |
| Miles Driven        | `kg_CO2e / 0.393`   | Average passenger vehicle: 8.89 kg CO2/gallon ÷ 22.8 mpg × GHG adj |
| Smartphones Charged | `kg_CO2e / 0.00822` | Average smartphone charge: 8.22g CO2                               |

Source: [EPA GHG Equivalencies Calculator](https://www.epa.gov/energy/greenhouse-gas-equivalencies-calculator)

## Display Thresholds

| Carbon Value   | Display Behavior                             |
| -------------- | -------------------------------------------- |
| < 1 kg CO2e    | Raw value only, no equivalencies             |
| 1 - 999,999 kg | Standard equivalencies with comma separators |
| ≥ 1,000,000 kg | Abbreviated (e.g., "~5.2 million miles")     |

## Unit Normalization

FinFocus accepts carbon values in various units and normalizes them to kilograms
internally:

| Input Unit | Conversion |
| ---------- | ---------- |
| g, gCO2e   | × 0.001    |
| kg, kgCO2e | × 1.0      |
| t, tCO2e   | × 1000.0   |
| lb, lbCO2e | × 0.453592 |

## Example Calculations

### Small Project (150 kg CO2e/month)

```text
Miles driven:        150 / 0.393    = 382 miles
Smartphones charged: 150 / 0.00822  = 18,248 smartphones
```

Display: "Equivalent to driving ~382 miles or charging ~18,248 smartphones"

### Data Center Scale (10,000,000 kg CO2e/month)

```text
Miles driven:        10,000,000 / 0.393    = 25,445,293 miles
Smartphones charged: 10,000,000 / 0.00822  = 1,216,545,012 smartphones
```

Display: "Equivalent to driving ~25.4 million miles or charging ~1.2 billion smartphones"

## Integration with Plugins

Carbon footprint data comes from cost plugins through the `Sustainability` map in
`CostResult`. Plugins report carbon using the canonical key `carbon_footprint`:

```go
result := &engine.CostResult{
    Monthly: 245.50,
    Currency: "USD",
    Sustainability: map[string]engine.SustainabilityMetric{
        "carbon_footprint": {Value: 150.0, Unit: "kg"},
    },
}
```

The legacy key `gCO2e` is deprecated but still supported for backward compatibility.

## Troubleshooting

### No Equivalencies Displayed

1. **Carbon value too low**: Values below 1 kg don't show equivalencies
2. **Missing carbon data**: Verify plugin reports `carbon_footprint` metric
3. **Check unit**: Ensure unit is recognized (g, kg, t, lb, or CO2e variants)

### Incorrect Values

1. Verify plugin reports correct units (kg vs g vs metric tons)
2. Check for unit mismatch between providers
3. Use `--debug` flag to see raw carbon values

## Related Documentation

- [Cost Calculation Guide](cost-calculation.md) - How costs are calculated
- [Plugin Development](../plugins/plugin-development.md) - Creating plugins with carbon support
- [Architecture: GreenOps Package](../architecture/greenops-package.md) - Technical details
