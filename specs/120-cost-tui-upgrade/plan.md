# Implementation Plan: Cost Commands TUI Upgrade

**Branch**: `106-cost-tui-upgrade` | **Date**: 2025-12-25 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/106-cost-tui-upgrade/spec.md`

## Summary

Upgrade cost commands (`cost projected`, `cost actual`) to use Bubble Tea for
interactive navigation and Lip Gloss for styled output. The implementation
leverages the existing `internal/tui` package (Spinner, Table, styles, colors,
detect) and adds new Bubble Tea models for cost display with interactive
resource tables, detail views, and loading states.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**:

- `github.com/charmbracelet/bubbletea` v1.2.4 (already transitive, promote to direct)
- `github.com/charmbracelet/bubbles` v0.20.0 (table, spinner components)
- `github.com/charmbracelet/lipgloss` v1.1.0 (styling)
- `golang.org/x/term` v0.38.0 (TTY detection)

**Storage**: N/A (no persistent storage for TUI state)
**Testing**: Go testing with `testify`, mock terminal environments
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project (CLI tool)
**Performance Goals**: <100ms keypress response for 100+ resources
**Constraints**: Plain text fallback when TTY unavailable, NO_COLOR support
**Scale/Scope**: Typical usage <1000 resources per cost calculation

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution:

- [x] **Plugin-First Architecture**: This is orchestration/display layer, not a
      plugin. Cost data comes from existing plugin system. ✓
- [x] **Test-Driven Development**: Tests planned for all new TUI components and
      output modes. Target 80% coverage for new code. ✓
- [x] **Cross-Platform Compatibility**: Bubble Tea/Lip Gloss are cross-platform.
      TTY detection handles platform differences. ✓
- [x] **Documentation as Code**: Quickstart.md included. User guide updates
      planned for new interactive features. ✓
- [x] **Protocol Stability**: No protocol changes - uses existing CostResult
      structure. Delta field addition is backward-compatible. ✓
- [x] **Implementation Completeness**: Full implementation planned with no
      stubs. All user stories have concrete acceptance scenarios. ✓
- [x] **Quality Gates**: make lint, make test, coverage checks enforced. ✓
- [x] **Multi-Repo Coordination**: No cross-repo dependencies - uses existing
      engine types from core. Delta field is optional in CostResult. ✓

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/106-cost-tui-upgrade/
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
├── cli/
│   ├── cost_projected.go     # MODIFY: Add TUI rendering integration
│   ├── cost_actual.go        # MODIFY: Add TUI rendering integration
│   └── cost_tui.go           # NEW: Output mode router, TUI entry point
├── engine/
│   ├── types.go              # MODIFY: Add Delta field to CostResult
│   └── project.go            # MODIFY: Add styled rendering functions
└── tui/
    ├── cost_model.go         # NEW: Bubble Tea model for cost display
    ├── cost_view.go          # NEW: View rendering (summary, table, detail)
    ├── cost_loading.go       # NEW: Loading state with spinner
    ├── cost_model_test.go    # NEW: Model tests
    ├── cost_view_test.go     # NEW: View tests
    └── cost_loading_test.go  # NEW: Loading state tests

test/
├── unit/
│   └── tui/
│       └── cost_*.go         # NEW: Unit tests for cost TUI components
└── integration/
    └── cli_tui_test.go       # NEW: Integration tests for TUI output modes
```

**Structure Decision**: Extends existing `internal/tui/` package with
cost-specific Bubble Tea models. CLI commands integrate via new `cost_tui.go`
router that selects output mode based on terminal detection.

## Complexity Tracking

> No violations requiring justification.

| Component | Complexity | Rationale |
|-----------|------------|-----------|
| Bubble Tea models | Medium | Standard tea.Model pattern, well-documented |
| Output mode router | Low | Simple switch on DetectOutputMode result |
| Delta field addition | Low | Optional field, backward-compatible |
