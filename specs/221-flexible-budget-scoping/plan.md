# Implementation Plan: Flexible Budget Scoping

**Branch**: `221-flexible-budget-scoping` | **Date**: 2026-01-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/221-flexible-budget-scoping/spec.md`

## Summary

Extend FinFocus budget configuration to support flexible scoping beyond global budgets,
including per-provider, per-resource-type, and per-tag budgets. Implementation extends
the existing `~/.finfocus/config.yaml` with a hierarchical `budgets:` section, adds
scope-aware cost allocation in the engine, and provides a grouped CLI display with
`--budget-scope` filtering.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: github.com/spf13/cobra, github.com/rs/zerolog, gopkg.in/yaml.v3
**Storage**: ~/.finfocus/config.yaml (YAML file-based configuration)
**Testing**: go test with testify (80% minimum, 95% critical paths)
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: <500ms for budget evaluation with 10,000 resources (SC-003)
**Constraints**: Single currency only (OOS-001), point-in-time evaluation only (OOS-002)
**Scale/Scope**: Typical deployments: 100-10,000 resources, <20 scoped budgets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Feature is orchestration logic in core, not a
      plugin. Budgets are user-defined in config, not provider-specific data.
      Justification: This is local budget configuration, not cost data sourcing.
- [x] **Test-Driven Development**: Tests planned before implementation with 80%
      minimum coverage. Critical paths (config parsing, scope matching, allocation)
      will achieve 95% coverage.
- [x] **Cross-Platform Compatibility**: Pure Go implementation with no platform-
      specific code. File paths use `filepath.Join` for OS compatibility.
- [x] **Documentation Synchronization**: README.md budget section and docs/guides/
      will be updated in the same PR as implementation.
- [x] **Protocol Stability**: No protocol buffer changes required. Uses existing
      plugin APIs for cost data; scoped budgets are config-only.
- [x] **Implementation Completeness**: No stubs or TODOs. All scope types (global,
      provider, tag, type) will be fully implemented before merge.
- [x] **Quality Gates**: CI checks for tests, lint, security will pass. Coverage
      thresholds enforced.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Feature is
      entirely within finfocus-core.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/221-flexible-budget-scoping/
├── plan.md              # This file
├── research.md          # Phase 0 output (complete)
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no new API contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── config/
│   ├── budget.go        # Extend with ScopedBudget, BudgetsConfig
│   ├── budget_scoped.go # NEW: Scoped budget types and parsing
│   └── config.go        # Update LoadConfig for migration
├── engine/
│   ├── budget.go        # Extend with scope-aware GetBudgets
│   ├── budget_scope.go  # NEW: Scope matching and allocation
│   └── budget_cli.go    # Extend BudgetStatus with scope info
├── cli/
│   ├── cost_budget.go   # Extend with --budget-scope flag
│   └── cost_budget_render.go # NEW: Grouped budget display

test/
├── unit/
│   ├── config/
│   │   └── budget_scoped_test.go # Scoped config parsing tests
│   └── engine/
│       └── budget_scope_test.go  # Scope matching tests
└── integration/
    └── budget_scope_test.go      # End-to-end scope tests

docs/
├── guides/
│   └── budgets.md                # User guide for scoped budgets
└── reference/
    └── config-reference.md       # Update with budgets section
```

**Structure Decision**: Single project structure, extending existing packages.
No new packages needed; scope logic integrated into config/ and engine/.

## Complexity Tracking

> No Constitution violations requiring justification.

| Component       | Complexity | Rationale                                   |
| --------------- | ---------- | ------------------------------------------- |
| Config parsing  | Medium     | Hierarchical YAML with migration logic      |
| Scope matching  | Low        | Map lookups with priority sorting           |
| Cost allocation | Medium     | Multi-scope aggregation with tag priority   |
| CLI rendering   | Low        | Grouped table output using existing patterns|
