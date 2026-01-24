# Implementation Plan: Budget Health Suite

**Branch**: `123-budget-health-suite` | **Date**: 2026-01-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/123-budget-health-suite/spec.md`

## Summary

Implement comprehensive budget health functionality in the FinFocus engine combining issues #263 and #267. This includes:

- **Budget health status calculation** (OK/WARNING/CRITICAL/EXCEEDED based on utilization %)
- **Provider filtering** with case-insensitive matching
- **Currency validation** (ISO 4217 format)
- **Summary aggregation** with health status counts
- **Threshold evaluation** for actual and forecasted spend
- **Forecasted spending** via linear extrapolation

The implementation uses proto definitions from finfocus-spec v0.5.4 and follows the existing engine patterns for plugin orchestration.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: finfocus-spec v0.5.4 (protobuf types), google.golang.org/grpc, github.com/stretchr/testify
**Storage**: N/A (stateless engine functionality)
**Testing**: go test with testify assertions, table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project - CLI tool with engine package
**Performance Goals**: Health calculation for 1000 budgets in <100ms, filtering in <500ms
**Constraints**: Stateless processing, no currency conversion, linear extrapolation only
**Scale/Scope**: Up to 1000 budgets per query, multi-provider environments

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in the engine - budgets come from plugins via GetBudgets RPC
- [x] **Test-Driven Development**: Tests planned before implementation (80% minimum coverage per SC-005)
- [x] **Cross-Platform Compatibility**: Pure Go with no platform-specific code
- [x] **Documentation Synchronization**: Engine package CLAUDE.md will be updated with budget health patterns
- [x] **Protocol Stability**: Uses existing proto types from finfocus-spec v0.5.4 (no protocol changes)
- [x] **Implementation Completeness**: Full implementation of all 10 functional requirements - no stubs
- [x] **Quality Gates**: All CI checks required (tests, lint, security)
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec v0.5.4 (already available)

**Violations Requiring Justification**: None - all principles satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/123-budget-health-suite/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output - proto type analysis
├── data-model.md        # Phase 1 output - Go struct mappings
├── quickstart.md        # Phase 1 output - usage examples
├── contracts/           # Phase 1 output - function signatures
│   └── budget-api.go    # Budget health function contracts
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/engine/
├── budget.go              # MODIFY: Add budget filtering and aggregation
├── budget_test.go         # NEW: Budget filtering tests
├── budget_health.go       # NEW: Health status calculation logic
├── budget_health_test.go  # NEW: Health calculation tests
├── budget_forecast.go     # NEW: Forecasting via linear extrapolation
├── budget_forecast_test.go # NEW: Forecasting tests
├── budget_summary.go      # NEW: Summary statistics aggregation
├── budget_summary_test.go # NEW: Summary tests
├── budget_threshold.go    # NEW: Threshold evaluation logic
├── budget_threshold_test.go # NEW: Threshold tests
└── CLAUDE.md              # UPDATE: Document budget health patterns

test/
├── unit/engine/           # Unit tests (covered above)
└── integration/           # Integration tests for budget flows
    └── budget_health_test.go # NEW: End-to-end budget health tests
```

**Structure Decision**: Follows existing engine package patterns. New budget functionality is split into focused files by responsibility (health, forecast, threshold, summary) matching the existing engine code organization (types.go, project.go, etc.).

## Complexity Tracking

> No violations - this section is intentionally empty.
