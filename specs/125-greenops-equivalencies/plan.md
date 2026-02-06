# Implementation Plan: GreenOps Impact Equivalencies

**Branch**: `125-greenops-equivalencies` | **Date**: 2026-01-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/125-greenops-equivalencies/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Add human-readable carbon emission equivalencies to CLI, TUI, and Analyzer output to make GreenOps metrics more understandable. The system will calculate EPA-sourced equivalencies (miles driven, smartphones charged) from aggregated carbon footprint values and display them in summary sections alongside existing sustainability metrics.

**Technical Approach**: Create a new `internal/greenops/` package containing equivalency calculation logic with hardcoded EPA formula constants. The package provides reusable utilities consumed by engine rendering (`renderSustainabilitySummary`), TUI summary (`RenderCostSummary`), and analyzer diagnostics (`formatCostMessage`).

## Technical Context

**Language/Version**: Go 1.25.7 (per go.mod)
**Primary Dependencies**: github.com/spf13/cobra (CLI), github.com/charmbracelet/lipgloss (TUI styling), github.com/rs/zerolog (logging), golang.org/x/text (number formatting)
**Storage**: N/A (pure computation, no persistence)
**Testing**: go test with testify (assert/require), table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module (CLI tool)
**Performance Goals**: < 1ms for equivalency calculations per result set
**Constraints**: No external API calls; EPA formulas hardcoded as constants
**Scale/Scope**: Operates on aggregated sustainability metrics (typically 1-100 resources per analysis)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration/display logic in core, not a data source. Carbon data originates from plugins via existing `SustainabilityMetric` pipeline.
- [x] **Test-Driven Development**: Tests planned before implementation with 80%+ coverage target for `internal/greenops/` package. EPA formula accuracy verified to 1% margin (SC-002).
- [x] **Cross-Platform Compatibility**: Pure Go computation with stdlib + golang.org/x/text for number formatting. No platform-specific code.
- [x] **Documentation Synchronization**: README and docs/ updates planned in same PR (user guide for equivalency display, architecture doc for greenops package).
- [x] **Protocol Stability**: No protocol changes required. Uses existing `SustainabilityMetric` type and `"carbon_footprint"` canonical key.
- [x] **Implementation Completeness**: Full implementation planned with no stubs. EPA constants hardcoded with source comments.
- [x] **Quality Gates**: All CI checks (make test, make lint, coverage thresholds) planned for verification.
- [x] **Multi-Repo Coordination**: No cross-repo dependencies. Feature is core-only using existing spec types.

**Violations Requiring Justification**: None identified.

## Project Structure

### Documentation (this feature)

```text
specs/125-greenops-equivalencies/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/
├── greenops/                    # NEW: Sustainability utilities package
│   ├── equivalency.go           # EPA formula calculations
│   ├── equivalency_test.go      # Unit tests (colocated per Go convention)
│   ├── normalizer.go            # Unit normalization to kg
│   ├── normalizer_test.go       # Normalizer tests (colocated)
│   ├── formatter.go             # Number/text formatting utilities
│   ├── formatter_test.go        # Formatter tests (colocated)
│   ├── types.go                 # Shared types and enums
│   ├── constants.go             # EPA formulas and thresholds
│   └── errors.go                # Error type definitions
├── engine/
│   └── project.go               # MODIFY: renderSustainabilitySummary() integration
├── tui/
│   └── cost_view.go             # MODIFY: RenderCostSummary() integration
├── analyzer/
│   └── diagnostics.go           # MODIFY: formatCostMessage() integration
└── cli/
    └── cost_projected.go        # No changes (uses engine output)

test/
└── integration/
    └── greenops_cli_test.go     # Integration tests for CLI/TUI/Analyzer equivalency display

docs/
├── guides/
│   └── greenops-equivalencies.md  # NEW: User guide for equivalency feature
└── architecture/
    └── greenops-package.md        # NEW: Package documentation
```

**Structure Decision**: Single project structure following existing `internal/` package patterns. New `internal/greenops/` package mirrors established patterns from `internal/registry/`, `internal/pluginhost/`. Integration via function calls from existing rendering code paths.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    | N/A        | N/A                                 |
