# Feature Specification: GreenOps Impact Equivalencies

**Feature Branch**: `125-greenops-equivalencies`
**Created**: 2026-01-27
**Status**: Draft
**Input**: User description: "Add human-readable carbon emission equivalencies to CLI and TUI output to make GreenOps metrics more understandable"

## Clarifications

### Session 2026-01-27

- Q: Which approach should the equivalency logic use to identify carbon emission values from plugin responses? → A: Use proto field name as canonical (`MetricKind_METRIC_KIND_CARBON_FOOTPRINT` → `"carbon_footprint"` key)
- Q: How should the equivalency calculator handle varying carbon units from plugins? → A: Normalize all values to kg internally before applying EPA formulas; preserve original unit for logging
- Q: Where should the carbon equivalency calculation logic be implemented? → A: New `internal/greenops/` package for sustainability/carbon utilities
- Q: How should EPA equivalency formula constants be managed? → A: Hardcode constants with EPA version/date comments in source code

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Carbon Equivalencies in CLI Table Output (Priority: P1)

As a DevOps engineer reviewing infrastructure costs, I want to see real-world equivalencies for carbon emissions in the CLI summary output so I can better understand and communicate the environmental impact of my cloud infrastructure.

**Why this priority**: This is the primary use case - making abstract carbon numbers meaningful. The CLI is the most common interface for cost analysis and provides immediate value to users already viewing sustainability metrics.

**Independent Test**: Can be fully tested by running `finfocus cost projected` with a plan containing resources that emit carbon, and verifying the summary includes equivalency text. Delivers immediate comprehension improvement for carbon data.

**Acceptance Scenarios**:

1. **Given** a Pulumi plan with resources that generate carbon emissions, **When** I run `finfocus cost projected --pulumi-json plan.json`, **Then** the summary section displays carbon emissions with at least one real-world equivalency (e.g., "~781 miles driven" or "~18,000 smartphones charged")

2. **Given** aggregated carbon emissions totaling 150 kg CO2e, **When** the summary is rendered, **Then** I see output like: "Est. Carbon Footprint: 150 kg CO2e (Equivalent to driving ~781 miles or charging ~18,248 smartphones)"

3. **Given** a Pulumi plan with zero carbon emissions, **When** I run the cost command, **Then** no carbon equivalency line appears in the summary (graceful omission)

---

### User Story 2 - View Carbon Equivalencies in TUI Summary (Priority: P2)

As a user exploring costs interactively via the TUI, I want to see the same carbon equivalencies displayed in the TUI summary view so I have consistent environmental context regardless of interface.

**Why this priority**: Ensures feature parity between CLI and TUI interfaces. Users who prefer the interactive TUI should not lose functionality available in CLI.

**Independent Test**: Can be tested by launching the TUI with cost data containing carbon metrics and verifying the summary panel includes equivalency text.

**Acceptance Scenarios**:

1. **Given** I am viewing the TUI cost summary with aggregated carbon data, **When** the summary renders, **Then** the carbon emissions display includes real-world equivalencies in the same format as CLI

2. **Given** carbon emissions are present in the TUI view, **When** displayed, **Then** the equivalency text is styled consistently with existing TUI design patterns (using Lip Gloss styles)

---

### User Story 3 - View Carbon Equivalencies in Analyzer Diagnostics (Priority: P3)

As a developer using `pulumi preview` with the FinFocus analyzer, I want to see carbon equivalencies in the diagnostic output so I can understand environmental impact during my infrastructure planning workflow.

**Why this priority**: Extends equivalency display to the zero-click Pulumi integration. Important for users who rely on `pulumi preview` output rather than standalone FinFocus commands.

**Independent Test**: Can be tested by running `pulumi preview` with the FinFocus analyzer configured and verifying diagnostic messages include carbon equivalencies.

**Acceptance Scenarios**:

1. **Given** a Pulumi preview with resources generating carbon emissions, **When** the FinFocus analyzer processes the stack, **Then** the summary diagnostic includes carbon equivalencies

---

### Edge Cases

- What happens when carbon emissions are very small (< 1 gram CO2e)? Display raw value only, skip equivalencies.
- What happens when carbon emissions are extremely large (> 1,000,000 kg CO2e)? Use appropriate unit scaling (e.g., "~5.2 million miles driven") with number formatting.
- How does the system handle mixed metric units from different plugins? All carbon values are normalized to kg CO2e internally before aggregation. Units are considered "matching" if they can be converted to kg (g, kg, t, gCO2e, kgCO2e, tCO2e, lb, lbCO2e). Unrecognized units trigger a warning log: `"skipping carbon metric with unrecognized unit: {unit}"` and exclude that resource from aggregation.
- What happens when only some resources have carbon metrics? Display equivalencies for the aggregated total of resources that have carbon data.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST calculate carbon equivalencies using EPA-sourced formulas when carbon emission metrics are present
- **FR-002**: System MUST display equivalencies only when total carbon emissions are non-zero
- **FR-003**: System MUST label equivalencies as "Equivalent to" or "Approx." to indicate estimates
- **FR-004**: System MUST support at least two equivalency types: miles driven and smartphones charged
- **FR-005**: System MUST format large numbers with appropriate separators (e.g., "18,248" not "18248")
- **FR-006**: System MUST round equivalency values appropriately (miles to nearest whole number, smartphones to nearest whole number)
- **FR-007**: System MUST keep equivalency display concise (1-2 lines maximum in output)
- **FR-008**: System MUST apply equivalencies to aggregated carbon totals, not per-resource values
- **FR-009**: System MUST identify carbon metrics using the proto-defined canonical key `"carbon_footprint"` (from `MetricKind_METRIC_KIND_CARBON_FOOTPRINT`); legacy `"gCO2e"` key support is deprecated

### Key Entities

- **CarbonEquivalency**: Represents a real-world equivalency calculation with type (miles, smartphones, trees), formula, and formatted output
- **EquivalencyResult**: The computed equivalency value for a given carbon amount, including the formatted display string

### Architecture Constraints

- Equivalency calculation logic MUST reside in new `internal/greenops/` package
- Package provides reusable sustainability utilities for CLI, TUI, and Analyzer consumers
- Follows existing domain-specific package pattern (e.g., `internal/registry/`, `internal/pluginhost/`)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can understand carbon impact through at least 2 real-world comparisons in every CLI/TUI summary that contains carbon data
- **SC-002**: All carbon equivalency calculations match EPA formula outputs within 1% margin
- **SC-003**: Equivalency display adds no more than 2 lines to existing summary output
- **SC-004**: No equivalency text appears when carbon emissions are zero or missing
- **SC-005**: Equivalency feature has 80%+ unit test coverage for calculation and formatting logic

## Assumptions

- EPA equivalency formulas are stable and will be used as the authoritative source
- Formulas are hardcoded as Go constants with EPA source URL and version date comments (no config file override)
- Miles driven formula: `kg_CO2e / 0.192` where 0.192 is the divisor (kg CO2e per mile; inverted from 8.89 kg/gal ÷ 21.6 mpg), EPA 2024
- Smartphones charged formula: `kg_CO2e / 0.00822` where 0.00822 is kg CO2e per smartphone charge, EPA 2024
- Tree seedlings formula: `kg_CO2e / 60.0` where 60.0 is kg CO2e absorbed per tree seedling grown for 10 years (carbon sequestration), EPA 2024. Note: This differs from monthly absorption rate; we use the 10-year cumulative value for meaningful comparisons
- Carbon metrics may arrive in varying units (g, kg, metric tons); equivalency calculator normalizes all values to kg internally before applying formulas, preserving original unit in logs for auditability
- Existing `SustainabilityMetric` structure is sufficient without schema changes
- Primary display will show miles and smartphones; tree absorption is optional/future
