# Implementation Plan: Overview Budget Status and Health

**Branch**: `601-overview-budget-status` | **Date**: 2026-02-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/601-overview-budget-status/spec.md`

## Summary

Wire the existing budget health system (`engine.GetBudgets`, `evaluateBudgetStatus`,
`RenderBudgetStatus`) into the overview command across all output modes: TUI (async
footer + detail section), plain text (budget status box after table), and JSON (budgets
array in metadata). The overview command is a top-level command (not under `cost`), so
budget flags (`--exit-on-threshold`, `--exit-code`, `--budget-scope`) must be added
independently to the overview command definition.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Bubble Tea (charmbracelet/bubbletea), Lip Gloss (charmbracelet/lipgloss), Cobra (spf13/cobra), finfocus-spec (pluginsdk, proto types)
**Storage**: N/A (no new persistent storage; reads from existing plugin gRPC + config)
**Testing**: `go test` with testify (assert/require), table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go project (CLI tool)
**Performance Goals**: Budget fetch must not delay initial TUI table render; async loading via Bubble Tea messages
**Constraints**: Budget footer must fit within terminal width; no new dependencies
**Scale/Scope**: ~8 files modified, ~400 lines of new code, ~300 lines of tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic — wires existing plugin
  budget data into the overview display layer. No direct provider integration.
- [x] **Test-Driven Development**: Tests planned for all new code paths: TUI message
  handling, footer rendering, detail view budget section, plain text integration,
  JSON output, and flag behavior. 80%+ coverage target.
- [x] **Cross-Platform Compatibility**: All changes are in Go standard library + existing
  cross-platform dependencies. No platform-specific code.
- [x] **Documentation Integrity**: CLAUDE.md updated with overview budget patterns.
  Godoc comments on all new exported types and functions.
- [x] **Protocol Stability**: No protocol changes. Uses existing `GetBudgets` gRPC API
  and proto types from finfocus-spec.
- [x] **Implementation Completeness**: Fully wired integration — no stubs, no TODOs.
  All four user stories (TUI footer, TUI detail, plain text, JSON) implemented completely.
- [x] **Quality Gates**: `make lint` and `make test` pass. Coverage target met.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Uses existing spec
  types from `finfocus-spec` without modification.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/601-overview-budget-status/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── overview.go              # MODIFY: Add budget flags, wire budget fetch into
│   │                            #   non-TTY path, pass engine to TUI for budget fetch
│   ├── cost_budget.go           # REUSE: RenderBudgetStatus, checkBudgetExitFromResult
│   └── common_execution.go      # REUSE: evaluateBudgetStatus, renderBudgetWithScope
├── tui/
│   ├── overview_messages.go     # MODIFY: Add BudgetDataReadyMsg type
│   ├── overview_model.go        # MODIFY: Add budget fields to OverviewModel,
│   │                            #   handle BudgetDataReadyMsg in Update()
│   ├── overview_view.go         # MODIFY: Add budget footer to renderListView(),
│   │                            #   add BUDGET STATUS section to renderDetailView()
│   └── overview_budget.go       # NEW: Budget footer and detail rendering helpers
└── engine/
    ├── overview_types.go        # MODIFY: Add BudgetHealthSummary type,
    │                            #   add BudgetHealth field to StackContext
    └── overview_render.go       # MODIFY: Include budgets in JSON output
```

**Structure Decision**: Existing single-project Go layout. All changes fit within the
established `internal/cli/`, `internal/tui/`, and `internal/engine/` package boundaries.
One new file (`overview_budget.go`) contains TUI budget rendering helpers, keeping the
existing view file from growing too large.
