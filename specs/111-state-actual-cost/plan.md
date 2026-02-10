# Implementation Plan: State-Based Actual Cost Estimation

**Branch**: `111-state-actual-cost` | **Date**: 2025-12-31 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/111-state-actual-cost/spec.md`

## Summary

Implement state-based actual cost estimation for the `cost actual` command,
enabling runtime-based cost calculations from Pulumi state files when billing
API access is unavailable. This feature includes a confidence level system
(HIGH/MEDIUM/LOW) for cost transparency, plus critical bug fixes for CI
reliability (AWS region scoping, deterministic output, test timeouts).

**Technical Approach**: Extend the existing CLI and engine to:

1. Accept `--pulumi-state` flag to load state via existing `ingest.LoadStackExport()`
2. Calculate estimated costs as `hourly_rate × runtime_hours` using projected costs
3. Add `Confidence` field to `CostResult` with deterministic level assignment
4. Fix test reliability issues (timeouts, context usage, deterministic output)

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: cobra v1.10.1, finfocus-spec v0.4.11 (pluginsdk),
zerolog v1.34.0, Bubble Tea/Lip Gloss (TUI)
**Storage**: N/A (stateless CLI tool; reads Pulumi state JSON files)
**Testing**: Go testing stdlib + testify v1.11.1, table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: <100ms latency for 100 resources (SC-004)
**Constraints**: Must not break existing `cost actual` behavior when
`--pulumi-state` is not provided
**Scale/Scope**: Stacks with up to 1000 resources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution
(`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core, not
  a direct provider integration. State parsing and cost estimation are
  orchestration concerns. Plugin `GetActualCost` is tried first.
- [x] **Test-Driven Development**: Tests planned before implementation. Target
  80% coverage for new code, 95% for CLI entry points.
- [x] **Cross-Platform Compatibility**: CLI changes work on all platforms.
  E2E fixes specifically target Windows reliability.
- [x] **Documentation as Code**: CLI help text and CLAUDE.md updates planned.
- [x] **Protocol Stability**: No protocol buffer changes required. Uses
  existing `GetProjectedCost` for hourly rates.
- [x] **Implementation Completeness**: All features fully implemented, no
  TODOs or stubs.
- [x] **Quality Gates**: `make lint` and `make test` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo dependencies. Uses existing
  pluginsdk from finfocus-spec.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/111-state-actual-cost/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # N/A (no new API contracts)
├── checklists/          # Validation checklists
│   └── requirements.md
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── cost_actual.go           # MODIFY: Add --pulumi-state, --estimate-confidence
│   ├── cost_actual_test.go      # MODIFY: Add tests for new flags
│   └── plugin_validate.go       # MODIFY: Return typed error instead of os.Exit
├── engine/
│   ├── types.go                 # MODIFY: Add Confidence field to CostResult
│   ├── state_cost.go            # CREATE: State-based cost calculation
│   ├── state_cost_test.go       # CREATE: Unit tests for state cost
│   ├── confidence.go            # CREATE: Confidence level determination
│   ├── confidence_test.go       # CREATE: Unit tests for confidence
│   ├── project.go               # MODIFY: Sort map keys for deterministic output
│   └── output.go                # MODIFY: Render confidence column
├── proto/
│   └── adapter.go               # MODIFY: Scope AWS region fallback
├── analyzer/
│   └── diagnostics.go           # MODIFY: Sort map keys for deterministic output
├── ingest/
│   └── state.go                 # EXISTS: No changes needed
└── conformance/
    └── context.go               # MODIFY: Fix timeout from 1µs to 10ms

test/
├── e2e/
│   ├── main_test.go             # MODIFY: Separate stdout/stderr
│   ├── errors_test.go           # MODIFY: Add 30s timeout
│   └── aws/
│       └── cost_test.go         # MODIFY: Fix fake implementation
├── integration/
│   └── actual_cost_test.go      # CREATE: Integration tests for state-based cost
└── fixtures/
    └── state/
        ├── valid-state.json     # CREATE: Test fixture with timestamps
        ├── no-timestamps.json   # CREATE: Test fixture without timestamps
        └── imported-resources.json  # CREATE: Test fixture with External=true
```

**Structure Decision**: Standard Go project layout with `internal/` packages
and `test/` directory. All new functionality integrates into existing packages
following established patterns.
