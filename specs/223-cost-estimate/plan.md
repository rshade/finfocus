# Implementation Plan: Cost Estimate Command for What-If Scenario Modeling

**Branch**: `223-cost-estimate` | **Date**: 2026-02-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/223-cost-estimate/spec.md`

## Summary

Implement a new `finfocus cost estimate` command that enables developers to perform "what-if" cost analysis on cloud resources without modifying Pulumi code. The command uses the `EstimateCost` RPC from finfocus-spec v0.5.5 to calculate baseline and modified costs, displaying per-property cost deltas. The implementation follows the existing CLI architecture with engine orchestration and plugin communication patterns.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: Cobra v1.10.2 (CLI), gRPC v1.78.0 (plugins), finfocus-spec v0.5.5 (protocol), Bubble Tea v1.3.10 (TUI), Lip Gloss v1.1.0 (styling)
**Storage**: N/A (stateless command)
**Testing**: go test with testify/assert, testify/require; 80% minimum coverage
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project (Go CLI with plugin architecture)
**Performance Goals**: 90% of single-resource estimates complete within 5 seconds (SC-004)
**Constraints**: Cross-platform compatible, uses existing plugin infrastructure
**Scale/Scope**: Single resources or plan-based (typically <100 resources per plan)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core; cost calculations delegated to plugins via EstimateCost RPC
- [x] **Test-Driven Development**: Tests planned before implementation with 80% coverage target
- [x] **Cross-Platform Compatibility**: Uses existing Go cross-platform patterns; no platform-specific code
- [x] **Documentation Integrity**: CLI docs and README updates planned with implementation
- [x] **Protocol Stability**: Uses existing EstimateCost RPC from finfocus-spec v0.5.5; no protocol changes
- [x] **Implementation Completeness**: Full implementation of all 3 user stories; no stubs or TODOs
- [x] **Quality Gates**: make lint and make test will be run before completion
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec v0.5.5 (already released); no plugin changes required for core functionality

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/223-cost-estimate/
├── plan.md              # This file
├── research.md          # Phase 0: Architecture research findings
├── data-model.md        # Phase 1: Data structures and types
├── quickstart.md        # Phase 1: Developer quickstart guide
├── contracts/           # Phase 1: gRPC contracts documentation
└── tasks.md             # Phase 2: Task breakdown (via /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── cost_estimate.go          # NEW: Cost estimate command implementation
│   ├── cost_estimate_test.go     # NEW: Unit tests for cost estimate command
│   └── common_execution.go       # EXTEND: Add estimate-specific helpers
├── engine/
│   ├── engine.go                 # EXTEND: Add EstimateCost method
│   ├── estimate.go               # NEW: Estimate-specific orchestration
│   ├── estimate_test.go          # NEW: Unit tests for estimate engine
│   └── types.go                  # EXTEND: Add EstimateResult type
├── proto/
│   ├── adapter.go                # EXTEND: Add EstimateCost request builder
│   └── adapter_test.go           # EXTEND: Add validation tests
└── tui/
    ├── estimate_model.go         # NEW: Interactive TUI model for estimates
    ├── estimate_model_test.go    # NEW: TUI model tests
    └── delta_view.go             # NEW: Delta visualization component

test/
├── integration/
│   └── cost_estimate_test.go     # NEW: Integration tests with mock plugin
└── fixtures/
    └── estimate/                 # NEW: Test fixtures for estimate scenarios
        ├── single-resource.json
        └── plan-with-modify.json
```

**Structure Decision**: Follows existing Go project structure with internal packages. New files for estimate-specific logic while extending existing files for shared functionality.

## Complexity Tracking

No constitution violations to justify.
