# Implementation Plan: CLI Filter Flag Support

**Branch**: `023-add-cli-filter-flag` | **Date**: 2025-12-23 | **Spec**: [link](../spec.md)
**Input**: Feature specification from `specs/023-add-cli-filter-flag/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This feature adds the missing `--filter` flag to the `finfocus actual-cost` command to resolve integration test failures (`unknown flag: --filter`). It ensures parity with the `projected-cost` command and exposes the existing `engine.FilterResources` capability to users for granular cost analysis by tag or resource type.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `github.com/spf13/cobra` (CLI), `github.com/spf13/pflag`
**Storage**: N/A (CLI logic)
**Testing**: `test/integration/cli/filter_test.go` (existing, failing), Go `testing` package
**Target Platform**: Cross-platform (Linux, macOS, Windows)
**Project Type**: CLI Application
**Performance Goals**: N/A (flag parsing overhead is negligible)
**Constraints**: Must match existing `--filter` behavior in `projected-cost` command.
**Scale/Scope**: affects `internal/cli/cost_actual.go` and potentially `internal/engine` wiring.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Is this feature implemented as a plugin or orchestration logic? (Orchestration/CLI layer)
- [x] **Test-Driven Development**: Are tests planned before implementation? (Existing failing tests serve as TDD start)
- [x] **Cross-Platform Compatibility**: Will this work on Linux, macOS, Windows? (Standard Go/Cobra)
- [x] **Documentation as Code**: Are audience-specific docs planned? (CLI help text + user guide update)
- [x] **Protocol Stability**: Do protocol changes follow semantic versioning? (N/A)
- [x] **Quality Gates**: Are all CI checks (tests, lint, security) passing? (Will ensure on completion)
- [x] **Multi-Repo Coordination**: Are cross-repo dependencies documented? (N/A)

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/023-add-cli-filter-flag/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (N/A for this feature)
├── quickstart.md        # Phase 1 output (CLI usage examples)
├── contracts/           # Phase 1 output (N/A)
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
internal/
└── cli/
    └── cost_actual.go   # Modify: Add --filter flag registration and handling
```

**Structure Decision**: Modifying existing CLI command definition in `internal/cli/cost_actual.go`.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |