# Implementation Plan: TUI Initializing View Lipgloss Consistency

**Branch**: `597-tui-init-lipgloss` | **Date**: 2026-02-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/597-tui-init-lipgloss/spec.md`

## Summary

Replace the raw `fmt.Sprintf` call in `renderInitializingView()` with a lipgloss-composed
view that uses `InfoStyle`, horizontal spinner+message layout, and width-responsive padding.
This is a single-function change to `internal/tui/overview_view.go` with two new unit tests.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `github.com/charmbracelet/lipgloss` (already imported in file)
**Storage**: N/A
**Testing**: `go test` + `github.com/stretchr/testify` (assert/require)
**Target Platform**: Linux, macOS, Windows (cross-platform; lipgloss is terminal-capability-aware)
**Project Type**: Single Go CLI binary
**Performance Goals**: N/A — trivial rendering function, no measurable overhead
**Constraints**: Must pass `make lint` and `make test`; no modification to `.golangci.yml`
**Scale/Scope**: 1 function body, 1 source file, 1 test file, ~15 lines changed

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A — this is a TUI rendering change, not a cost data source. Core orchestration layer unchanged.
- [x] **Test-Driven Development**: Two new tests planned covering nil loadingState and width constraints. Existing 2 tests continue to pass. 80% coverage maintained.
- [x] **Cross-Platform Compatibility**: `lipgloss` uses `golang.org/x/term` for terminal detection; renders correctly on all three platforms.
- [x] **Documentation Integrity**: No exported symbols added or changed. No godoc updates required. No README changes needed.
- [x] **Protocol Stability**: N/A — no protocol buffer definitions touched.
- [x] **Implementation Completeness**: Full replacement of the function body; no stubs, no TODOs.
- [x] **Quality Gates**: `make lint` and `make test` must pass before task completion.
- [x] **Multi-Repo Coordination**: N/A — change is entirely within `finfocus-core`, no cross-repo dependencies.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/597-tui-init-lipgloss/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions on patterns and imports
├── quickstart.md        # Phase 1 output — developer change summary
└── tasks.md             # Phase 2 output (/speckit.tasks — not yet created)
```

No `data-model.md` or `contracts/` — this change involves no data entities and no API surface.

### Source Code (repository root)

```text
internal/tui/
├── overview_view.go          # MODIFY: replace renderInitializingView body (~10 lines)
├── overview_view_test.go     # MODIFY: add 2 new test functions
└── cost_view.go              # READ-ONLY: borderPadding = 2 (shared constant, not changed)
```

**Structure Decision**: Single project layout. The change is isolated to the `internal/tui`
package; no new files are created. The `fmt` import in `overview_view.go` is retained
because it is used by other functions in the file.

## Implementation Design

### Function Replacement

**Current** (`overview_view.go` lines 35–44):

```go
func (m OverviewModel) renderInitializingView() string {
    spinnerView := ""
    if m.loadingState != nil {
        spinnerView = m.loadingState.spinner.View()
    }
    msg := m.progressMsg
    if msg == "" {
        msg = "Initializing..."
    }
    return fmt.Sprintf("\n %s %s\n\n", spinnerView, msg)
}
```

**Replacement**:

```go
func (m OverviewModel) renderInitializingView() string {
    spinnerView := ""
    if m.loadingState != nil {
        spinnerView = m.loadingState.spinner.View()
    }
    msg := m.progressMsg
    if msg == "" {
        msg = "Initializing..."
    }
    content := lipgloss.JoinHorizontal(lipgloss.Left,
        spinnerView,
        " ",
        InfoStyle.Render(msg),
    )
    return lipgloss.NewStyle().
        Width(m.width - borderPadding).
        Padding(1, 2).
        Render(content)
}
```

**Key decisions** (see `research.md` for full rationale):

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Alignment | `lipgloss.Left` | Matches all other JoinHorizontal/JoinVertical calls in the file |
| Padding | `Padding(1, 2)` | Full-screen state needs vertical breathing room; matches issue suggestion |
| Width | `m.width - borderPadding` | Mirrors renderProgressBanner (line 62) and renderDetailView (line 165) |
| `fmt` import | Retained | Still used in 7+ other places in the file |
| Width clamping | None needed | lipgloss silently ignores ≤0 widths; `defaultWidth=80` in tests |

### New Tests

Add to `internal/tui/overview_view_test.go`:

```go
// TestOverviewView_InitializingRender_WidthRespected verifies width constraint.
func TestOverviewView_InitializingRender_WidthRespected(t *testing.T) {
    ctx := context.Background()
    model, _ := NewOverviewModel(ctx, nil, 0)
    model.width = 100
    model.progressMsg = "Loading..."

    output := model.renderInitializingView()
    assert.Contains(t, output, "Loading...")
    // lipgloss pads/truncates to width; output lines should not exceed 100 chars
    for _, line := range strings.Split(output, "\n") {
        assert.LessOrEqual(t, len(line), 100)
    }
}

// TestOverviewView_InitializingRender_NilLoadingState verifies nil safety.
func TestOverviewView_InitializingRender_NilLoadingState(t *testing.T) {
    ctx := context.Background()
    model, _ := NewOverviewModel(ctx, nil, 0)
    model.loadingState = nil

    require.NotPanics(t, func() {
        output := model.renderInitializingView()
        assert.Contains(t, output, "Initializing...")
    })
}
```

## Phase 0: Research

**Status**: Complete — see [research.md](research.md)

All unknowns resolved by codebase inspection. No external dependencies or patterns
required beyond what already exists in the `internal/tui` package.

## Phase 1: Design & Contracts

**Status**: Complete

- `data-model.md`: N/A — no data entities
- `contracts/`: N/A — no API/protocol changes
- `quickstart.md`: Generated — see [quickstart.md](quickstart.md)
- Agent context: Updated below

## Post-Design Constitution Check

Re-verified after Phase 1 design:

- Implementation is complete (no stubs, no TODOs in the replacement code)
- All 4 existing tests continue to pass (message content preserved)
- 2 new tests cover nil safety and width constraint (FR-003, FR-006)
- Cross-platform: lipgloss handles terminal capabilities transparently
- `make lint` will pass: no new exported symbols, imports unchanged except removing the one `fmt.Sprintf` call

## Complexity Tracking

No constitution violations. No complexity justification required.
