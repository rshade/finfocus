# Implementation Plan: Charmbracelet v2 Upgrade

**Branch**: `604-charm-v2-upgrade` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/604-charm-v2-upgrade/spec.md`

## Summary

Upgrade charmbracelet bubbletea, bubbles, and lipgloss from v1 to v2 atomically.
The v2 releases use `charm.land` vanity import paths and introduce breaking changes
to the Model interface (`View()` returns `tea.View`), key message types (`tea.KeyMsg`
→ `tea.KeyPressMsg`), component constructors (functional options for spinner), and
field access patterns (getter/setter methods for width/height). The migration touches
~37 files (21 source + 16 test) with ~400 API call sites to update.

## Technical Context

**Language/Version**: Go 1.25.8 (see `go.mod`)
**Primary Dependencies**:

- `charm.land/bubbletea/v2` (from `github.com/charmbracelet/bubbletea v1.3.10`)
- `charm.land/bubbles/v2` (from `github.com/charmbracelet/bubbles v1.0.0`)
- `charm.land/lipgloss/v2` (from `github.com/charmbracelet/lipgloss v1.1.0`)

**Storage**: N/A (no storage changes)
**Testing**: `go test` via `make test` (testify assert/require)
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module (CLI tool)
**Performance Goals**: No performance regression; v2 Cursed Renderer may improve TUI rendering
**Constraints**: Atomic migration (all three packages in one compilable state)
**Scale/Scope**: ~400 API call sites across 37 files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: N/A — this is a TUI dependency upgrade,
  not a plugin or provider integration. No core architecture changes.
- [x] **Test-Driven Development**: All existing tests will be migrated to v2 API.
  Coverage must remain >= 80%. No new functionality is added.
- [x] **Cross-Platform Compatibility**: charmbracelet v2 packages support the same
  platforms as v1 (Linux, macOS, Windows). Cross-platform builds verified in CI.
- [x] **Documentation Integrity**: No README or docs/ changes required — this is
  an internal dependency upgrade with no user-facing API changes to FinFocus itself.
- [x] **Protocol Stability**: N/A — no protocol buffer or gRPC changes.
- [x] **Implementation Completeness**: The migration will be complete — no stubs,
  no TODOs, no partial upgrades. All v1 API call sites will be fully migrated.
- [x] **Quality Gates**: `make test` and `make lint` must pass after migration.
- [x] **Multi-Repo Coordination**: N/A — charmbracelet packages are not shared
  across the FinFocus multi-repo ecosystem.

**Violations Requiring Justification**: None. All constitution principles are satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/604-charm-v2-upgrade/
├── plan.md              # This file
├── research.md          # Phase 0: v2 API research findings
├── quickstart.md        # Phase 1: Migration reference guide
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
# Files modified by this migration (no new files created)
internal/tui/
├── banner.go                    # lipgloss import only
├── colors.go                    # lipgloss.Color type change
├── colors_test.go               # lipgloss import only
├── components.go                # lipgloss.NewStyle() calls
├── cost_loading.go              # spinner constructor, tick, lipgloss
├── cost_loading_test.go         # spinner.TickMsg type change
├── cost_model.go                # KeyPressMsg, View(), table, textinput, lipgloss
├── cost_model_test.go           # tea.KeyMsg → tea.KeyPressMsg in test helpers
├── cost_view.go                 # table construction (5 table.New calls)
├── delta_view.go                # lipgloss.NewStyle() (22 calls)
├── estimate_model.go            # KeyPressMsg (Type-based), View()
├── estimate_model_test.go       # tea.KeyMsg → tea.KeyPressMsg
├── overview_budget.go           # lipgloss import
├── overview_messages.go         # bubbletea import
├── overview_model.go            # KeyPressMsg, table, textinput
├── overview_model_test.go       # tea.KeyMsg → tea.KeyPressMsg
├── overview_view.go             # View(), lipgloss layout
├── progress.go                  # lipgloss.NewStyle()
├── recommendations_model.go     # KeyPressMsg, View(), textinput, lipgloss
├── recommendations_model_test.go # tea.KeyMsg → tea.KeyPressMsg
├── spinner.go                   # spinner.New() → functional options
├── spinner_test.go              # spinner assertions
├── styles.go                    # lipgloss style definitions
├── table.go                     # table.New(), DefaultStyles()
├── table_test.go                # table assertions
└── list/
    ├── model.go                 # KeyPressMsg (Type-based), View()
    └── model_test.go            # tea.KeyMsg → tea.KeyPressMsg

internal/cli/
├── cost_budget.go               # lipgloss.Color, NewStyle
├── cost_budget_render.go        # lipgloss.Color, NewStyle
├── cost_budget_render_test.go   # lipgloss.Color
├── cost_estimate.go             # bubbletea import
├── cost_recommendations.go      # bubbletea import
├── cost_tui.go                  # bubbletea import
└── overview.go                  # bubbletea import

test/integration/
├── tui_state_machine_test.go    # tea.KeyMsg → tea.KeyPressMsg
└── tui_virtual_scroll_test.go   # tea.KeyMsg → tea.KeyPressMsg

go.mod                           # Module path updates
go.sum                           # Regenerated
```

**Structure Decision**: No new files or directories. This is a pure in-place
migration of existing source files. The existing Go module structure is unchanged.

## Migration Strategy

### Phase 1: Import Path + go.mod (Mechanical)

Update `go.mod` to replace all three charmbracelet dependencies with `charm.land`
equivalents. Run `sed` or `goimports` to update all 47 import statements across
37 files.

### Phase 2: View() Return Type (5 files)

Change `View() string` to `View() tea.View` in 5 model files. Wrap existing
string return values with `tea.NewView(content)`.

### Phase 3: Key Message Migration (12 files, ~117 usages)

Two migration patterns required:

**Pattern A — String-based detection** (cost_model, overview_model,
recommendations_model): Change type assertion from `tea.KeyMsg` to
`tea.KeyPressMsg`. The `.String()` method still works in v2, so switch bodies
are largely preserved. Exception: space bar changes from `" "` to `"space"`.

**Pattern B — Type-field detection** (list/model, estimate_model): Change
`msg.Type` comparisons to `msg.Code` comparisons. Replace `tea.KeyRunes`
checks with `msg.Text != ""` and `msg.Text` string matching.

**Test files**: All test helpers constructing `tea.KeyMsg{Type: ..., Runes: ...}`
must be updated to `tea.KeyPressMsg{Code: ..., Text: ...}`.

### Phase 4: Bubbles Component Migration (7 files)

- **Spinner** (2 files): `spinner.New()` → `spinner.New(spinner.WithSpinner(spinner.Dot),
  spinner.WithStyle(...))`. `l.spinner.Tick` field reference may need verification.
- **Table** (3 files, 35 call sites): `table.New()` with functional options is
  unchanged. `table.DefaultStyles()` is unchanged. Width/Height field access may
  need getter/setter migration.
- **Textinput** (3 files): `ti.Width = N` → `ti.SetWidth(N)`. `ti.Placeholder`
  and `ti.CharLimit` field access needs verification against v2 API.

### Phase 5: Lipgloss Migration (16 files, ~111 call sites)

- `lipgloss.Color("N")` syntax is preserved but now returns `color.Color`.
  Since `const ColorOK = lipgloss.Color("82")` pattern is used, the const
  declarations may need to become `var` if the return type is no longer a
  simple type alias.
- `lipgloss.NewStyle()` is unchanged.
- `lipgloss.JoinVertical()`, `lipgloss.Width()` are unchanged.
- No `AdaptiveColor` usage found in codebase (no compat import needed).

### Phase 6: Validation

- `go build ./...` (compilation check)
- `make test` (80%+ coverage)
- `make lint` (zero new errors)
- Manual TUI smoke test

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `lipgloss.Color` const → var forced change | Medium | Low | Verify v2 type; may stay as function call in const |
| Spinner `.Tick` field vs method change | Medium | Low | Check v2 source; update pattern accordingly |
| Space bar `" "` → `"space"` missed | High | Medium | Grep for `" "` in key switch blocks |
| `textinput` field setters beyond Width | Medium | Low | Check Placeholder, CharLimit in v2 API |
| v2 default styles differ from v1 | Low | Low | Explicit style overrides already in place |
| Key string representations changed | Low | High | Validate all `.String()` values against v2 docs |

## Complexity Tracking

> No constitution violations — this section is intentionally empty.
