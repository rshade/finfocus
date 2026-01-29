# Research: GreenOps Impact Equivalencies

**Feature**: 125-greenops-equivalencies
**Date**: 2026-01-27
**Status**: Complete

## Overview

This document captures research findings and technology decisions for implementing carbon emission equivalencies in FinFocus CLI, TUI, and Analyzer output.

## Research Areas

### 1. EPA Greenhouse Gas Equivalencies Calculator

**Source**: [EPA GHG Equivalencies Calculator](https://www.epa.gov/energy/greenhouse-gas-equivalencies-calculator)

**Decision**: Use EPA-published conversion factors as authoritative source.

**Rationale**:

- EPA is the standard reference for carbon equivalency calculations in US-based tools
- Formulas are well-documented with methodology explanations
- Values are updated periodically (latest: 2024) with clear versioning

**Alternatives Considered**:

- **Carbon Footprint calculators (third-party APIs)**: Rejected due to external dependency requirement and latency concerns
- **Custom research formulas**: Rejected due to lack of authoritative backing and maintenance burden

**Key Formulas (EPA 2024)**:

| Equivalency | Formula | Source |
|-------------|---------|--------|
| Miles driven | `kg_CO2e / 0.192` | Average passenger vehicle (8.89 kg CO2/gallon × 21.6 mpg) |
| Smartphones charged | `kg_CO2e / 0.00822` | Average smartphone charge (8.22 g CO2) |
| Tree seedlings grown 10 years | `kg_CO2e / 60.0` | Urban tree carbon sequestration |
| Home electricity (days) | `kg_CO2e / 18.3` | Average US home daily consumption |

**Implementation Notes**:

- Constants defined with EPA source URL and version date in code comments
- All calculations normalize to kg CO2e before applying formulas
- Unit conversions: 1 metric ton = 1000 kg, 1 g = 0.001 kg

### 2. Number Formatting for Large Values

**Decision**: Use `golang.org/x/text/message` with `golang.org/x/text/language` for locale-aware number formatting.

**Rationale**:

- Provides proper thousand separators (FR-005: "18,248" not "18248")
- Handles internationalization if needed in future
- Standard Go extended library with minimal dependencies

**Alternatives Considered**:

- **fmt.Sprintf with manual formatting**: Rejected due to complexity of handling edge cases and localization
- **humanize library**: Rejected to minimize external dependencies; golang.org/x/text is quasi-stdlib

**Code Pattern**:

```go
import (
    "golang.org/x/text/language"
    "golang.org/x/text/message"
)

p := message.NewPrinter(language.English)
formatted := p.Sprintf("%d", 18248) // "18,248"
```

### 3. Unit Normalization Strategy

**Decision**: Normalize all carbon values to kilograms (kg) internally before applying EPA formulas.

**Rationale**:

- EPA formulas are expressed in kg CO2e
- Consistent internal representation simplifies calculations
- Original unit preserved in logs for auditability (per spec clarification)

**Alternatives Considered**:

- **Per-unit formula variants**: Rejected due to formula duplication and maintenance burden
- **Calculate in original units then convert**: Rejected due to precision loss potential

**Unit Conversion Table**:

| Input Unit | Multiplier to kg |
|------------|------------------|
| g, gCO2e | 0.001 |
| kg, kgCO2e | 1.0 |
| t, tCO2e, metric ton | 1000.0 |
| lb, lbCO2e | 0.453592 |

**Edge Cases**:

- Unknown units: Log warning, skip equivalency calculation
- Mixed units across resources: Aggregate only matching units (per spec edge case)

### 4. Display Threshold Strategy

**Decision**: Skip equivalency display for very small values (< 1g CO2e).

**Rationale**:

- Equivalencies become meaningless at sub-gram scale (e.g., "~0.005 miles driven")
- FR-002 requires display only when non-zero, extended to "meaningfully non-zero"

**Thresholds**:

| Value Range | Display Behavior |
|-------------|------------------|
| < 0.001 kg (1g) | No equivalency display |
| 0.001 - 1.0 kg | Show raw value only, no equivalencies |
| 1.0 - 1,000,000 kg | Standard equivalency display |
| > 1,000,000 kg | Use scaled units (e.g., "~5.2 million miles") |

### 5. Large Number Scaling

**Decision**: Use abbreviated notation for very large equivalency values.

**Rationale**:

- FR-007 requires concise display (1-2 lines max)
- Large numbers are harder to comprehend without scaling

**Scaling Rules**:

| Value | Display Format |
|-------|---------------|
| < 1,000 | "123" |
| 1,000 - 999,999 | "18,248" (comma separated) |
| 1,000,000 - 999,999,999 | "~5.2 million" |
| ≥ 1,000,000,000 | "~1.2 billion" |

### 6. Integration Points Analysis

**Decision**: Integrate at three rendering locations with shared calculation logic.

**Findings from Codebase Exploration**:

1. **Engine Table Rendering** (`internal/engine/project.go:260-285`):
   - `renderSustainabilitySummary()` aggregates and displays sustainability metrics
   - Inject equivalency calculation after aggregation, before display
   - Modify output to append equivalency text to carbon_footprint line

2. **TUI Summary** (`internal/tui/cost_view.go:72-133`):
   - `RenderCostSummary()` renders styled summary box
   - Add equivalency text after provider breakdown
   - Use existing Lip Gloss styles for consistency

3. **Analyzer Diagnostics** (`internal/analyzer/diagnostics.go:119-160`):
   - `formatCostMessage()` builds diagnostic message string
   - Append equivalency text after sustainability metrics section
   - Keep within diagnostic message length limits

**Rationale**: Shared `internal/greenops/` package provides calculation logic; each consumer handles display formatting appropriate to its output medium.

### 7. Carbon Metric Identification

**Decision**: Use `"carbon_footprint"` as the canonical key with fallback to legacy `"gCO2e"`.

**Findings**:

- Proto field: `MetricKind_METRIC_KIND_CARBON_FOOTPRINT` maps to `"carbon_footprint"`
- Legacy key `"gCO2e"` still referenced in `diagnostics.go:142`
- Spec FR-009 marks legacy support as deprecated

**Implementation**:

```go
// Check canonical key first
if metric, ok := sustainability["carbon_footprint"]; ok {
    return metric.Value, metric.Unit, nil
}
// Fallback to legacy (deprecated)
if metric, ok := sustainability["gCO2e"]; ok {
    log.Warn().Msg("deprecated key 'gCO2e' used, prefer 'carbon_footprint'")
    return metric.Value, metric.Unit, nil
}
```

### 8. Output Format Consistency

**Decision**: Use consistent phrasing across all output modes.

**Format Template**:

```text
CLI/TUI: "Equivalent to driving ~X miles or charging ~Y smartphones"
Analyzer: " (≈ X mi driven, Y phones charged)"
```

**Rationale**:

- FR-003 requires "Equivalent to" or "Approx." labeling
- Analyzer uses compact format due to diagnostic message constraints
- CLI/TUI have more space for readable prose

### 9. Testing Strategy

**Decision**: Table-driven tests with EPA formula verification.

**Test Categories**:

1. **Unit Tests** (`internal/greenops/equivalency_test.go`):
   - Formula accuracy within 1% margin (SC-002)
   - Unit normalization correctness
   - Edge cases (zero, negative, very small, very large values)
   - Number formatting verification

2. **Integration Tests** (`test/integration/greenops_test.go`):
   - End-to-end CLI output verification
   - TUI rendering verification
   - Analyzer diagnostic format verification

**Coverage Target**: 80%+ for greenops package (SC-005).

## Technology Stack Summary

| Component | Technology | Version/Source |
|-----------|------------|----------------|
| Carbon Formulas | EPA GHG Equivalencies | 2024 Edition |
| Number Formatting | golang.org/x/text | Latest stable |
| Testing | testify (assert/require) | Existing dependency |
| Logging | zerolog | Existing dependency |
| TUI Styling | lipgloss | Existing dependency |

## Open Questions (Resolved)

All clarification questions from the spec have been resolved:

- ✅ Carbon metric identification: Use `"carbon_footprint"` canonical key
- ✅ Unit normalization: Normalize to kg internally
- ✅ Package location: New `internal/greenops/` package
- ✅ Formula constants: Hardcode with EPA source comments

## Next Steps

1. Create data-model.md with entity definitions
2. Create contracts/ with interface definitions
3. Create quickstart.md for developer onboarding
4. Proceed to task generation (/speckit.tasks)
