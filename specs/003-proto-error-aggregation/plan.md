# Implementation Plan: Error Aggregation in Proto Adapter

**Branch**: `003-proto-error-aggregation` | **Date**: 2025-11-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-proto-error-aggregation/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement error aggregation for `GetProjectedCost` and `GetActualCost` in the proto adapter to replace silent failures with comprehensive error tracking. Changes include new `ErrorDetail` and `CostResultWithErrors` types, updated adapter and engine return types, structured logging with zerolog, and CLI display of errors both inline and as summaries.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: gRPC, zerolog (new), finfocus-spec proto SDK
**Storage**: N/A (in-memory error aggregation)
**Testing**: go test with race detection, 80% coverage minimum
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI project
**Performance Goals**: Minimal overhead for error tracking (<1ms per resource)
**Constraints**: PR size ~150-200 lines, zerolog dependency acceptable
**Scale/Scope**: Handle error aggregation for hundreds of resources per calculation

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Yes - this is orchestration logic in core that handles plugin errors
- [x] **Test-Driven Development**: Yes - comprehensive tests planned (FR testing, edge cases, error truncation)
- [x] **Cross-Platform Compatibility**: Yes - pure Go, no platform-specific code
- [x] **Documentation as Code**: Yes - inline code comments and updated CLAUDE.md
- [x] **Protocol Stability**: Yes - no proto changes, only internal Go types
- [x] **Quality Gates**: Yes - all CI checks will be verified before merge
- [x] **Multi-Repo Coordination**: N/A - no cross-repo changes required (SDK unchanged)

**Violations Requiring Justification**: None - all principles satisfied

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
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
├── proto/
│   ├── adapter.go          # ErrorDetail, CostResultWithErrors types + updated methods
│   └── adapter_test.go     # Comprehensive error aggregation tests
├── engine/
│   ├── engine.go           # Updated return types for GetProjectedCost/GetActualCost
│   └── engine_test.go      # Updated tests for new return types
└── cli/
    ├── cost_projected.go   # Handle CostResultWithErrors, display errors
    ├── cost_actual.go      # Handle CostResultWithErrors, display errors
    └── *_test.go           # CLI integration tests
```

**Structure Decision**: Single project using existing Go internal package structure. All changes are internal to core - no new packages required.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation                  | Why Needed         | Simpler Alternative Rejected Because |
| -------------------------- | ------------------ | ------------------------------------ |
| [e.g., 4th project]        | [current need]     | [why 3 projects insufficient]        |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient]  |
