# Implementation Plan: Budget Threshold Exit Codes

**Branch**: `124-budget-exit-codes` | **Date**: 2026-01-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/124-budget-exit-codes/spec.md`

## Summary

Add configurable exit codes when budget thresholds are exceeded, enabling CI/CD pipeline integration for automated cost governance. The implementation extends `BudgetConfig` with `exit_on_threshold` and `exit_code` fields, adds environment variable and CLI flag support, and integrates exit code evaluation into the cost command post-render flow.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: github.com/spf13/cobra (CLI), github.com/rs/zerolog (logging), github.com/rshade/finfocus-spec (proto definitions)
**Storage**: YAML config file (~/.finfocus/config.yaml)
**Testing**: go test with testify (require/assert), 80% minimum coverage
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: Exit code evaluation adds negligible overhead (<1ms)
**Constraints**: Exit codes must be 0-255 (Unix standard), output must render before exit
**Scale/Scope**: Feature-scoped change affecting 3-4 packages (config, engine, cli)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is core orchestration logic (exit behavior), not a data source—appropriate for core.
- [x] **Test-Driven Development**: Tests planned for config parsing, environment variable precedence, exit code evaluation, and CLI integration (80%+ coverage).
- [x] **Cross-Platform Compatibility**: Exit codes 0-255 are universal across Linux, macOS, Windows.
- [x] **Documentation Synchronization**: README and docs/ updates included in implementation scope.
- [x] **Protocol Stability**: No protocol changes—this is CLI-only behavior.
- [x] **Implementation Completeness**: Feature will be fully implemented with no stubs or TODOs.
- [x] **Quality Gates**: All CI checks (tests, lint, security) will pass before merge.
- [x] **Multi-Repo Coordination**: No cross-repo changes required—feature is core-only.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/124-budget-exit-codes/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no API contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── config/
│   └── budget.go        # Add ExitOnThreshold, ExitCode fields to BudgetConfig
├── engine/
│   └── budget_cli.go    # Add ShouldExit() and GetExitCode() methods to BudgetStatus
└── cli/
    ├── cost.go            # Add --exit-on-threshold, --exit-code flags to parent command
    ├── cost_budget.go     # Add checkBudgetExit() helper function
    ├── cost_projected.go  # Call exit logic after render
    └── cost_actual.go     # Call exit logic after render

test/
├── unit/
│   └── config/          # Exit code config parsing tests
├── integration/
│   └── budget_exit_test.go  # CI/CD simulation tests
└── fixtures/
    └── configs/         # Test config files with exit settings
```

**Structure Decision**: Single CLI application pattern. Changes are localized to config (data model), engine (exit logic), and cli (flag handling + exit invocation).

## Complexity Tracking

> **No violations requiring justification**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| None      | N/A        | N/A                                  |
