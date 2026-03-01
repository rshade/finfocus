# Research: Charmbracelet v2 API Migration

**Date**: 2026-02-28
**Feature**: 604-charm-v2-upgrade

## Sources

- [Bubbletea v2 Upgrade Guide](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md)
- [Bubbles v2 Upgrade Guide](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md)
- [Lipgloss v2 Upgrade Guide](https://github.com/charmbracelet/lipgloss/blob/main/UPGRADE_GUIDE_V2.md)
- [Bubbletea v2.0.0 Release Notes](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0)
- [charm.land/bubbletea/v2 on pkg.go.dev](https://pkg.go.dev/charm.land/bubbletea/v2)
- [charm.land/lipgloss/v2 on pkg.go.dev](https://pkg.go.dev/charm.land/lipgloss/v2)
- [charm.land/bubbles/v2/table on pkg.go.dev](https://pkg.go.dev/charm.land/bubbles/v2/table)

## Decision Log

### D-001: Import Path Strategy

**Decision**: Use `charm.land/*` vanity import paths (not `github.com/charmbracelet/*/v2`).
**Rationale**: The canonical stable v2.0.0 releases use `charm.land`. The
`github.com/charmbracelet/*/v2` paths are stuck at beta versions.
**Alternatives considered**: `github.com/charmbracelet/bubbletea/v2` — rejected
because it only has beta releases (v2.0.0-beta.6).

### D-002: View() Return Type Migration

**Decision**: Change `View() string` to `View() tea.View`, wrapping content with
`tea.NewView(content)`.
**Rationale**: The v2 `tea.Model` interface requires `View() tea.View`. The
`tea.View` struct carries declarative terminal state (alt screen, cursor, mouse
mode) alongside rendered content.
**Alternatives considered**: None — this is a mandatory interface change.

### D-003: Key Message Migration Pattern

**Decision**: Migrate `tea.KeyMsg` to `tea.KeyPressMsg` using the `.String()` method
where possible, and `msg.Code` constants where the codebase uses `msg.Type`.
**Rationale**: `.String()` still works in v2 and returns compatible strings for
most keys, minimizing changes to existing switch blocks. The `msg.Code` approach
maps directly from the old `msg.Type` field.
**Alternatives considered**: Converting all key handling to `msg.Code` — rejected
because it would increase diff size for no behavioral benefit in files already
using `.String()`.

### D-004: tea.KeyRunes Replacement

**Decision**: Replace `msg.Type == tea.KeyRunes` with `msg.Text != ""` check,
and `string(msg.Runes)` with `msg.Text`.
**Rationale**: v2 replaces `Runes []rune` with `Text string` and removes the
`KeyRunes` type constant. The `Text` field is non-empty for printable characters.
**Alternatives considered**: Using `msg.String()` for all rune detection — viable
but would require restructuring switch blocks in `list/model.go` and
`estimate_model.go`.

### D-005: Space Bar String Change

**Decision**: Update space bar detection from `" "` to `"space"` in `.String()`
comparisons.
**Rationale**: v2 `KeyPressMsg.String()` returns `"space"` for the space bar,
not `" "` as in v1.
**Alternatives considered**: Using `msg.Code == tea.KeySpace` — acceptable
alternative for Type-based patterns.

### D-006: Spinner Constructor Migration

**Decision**: Use functional options: `spinner.New(spinner.WithSpinner(spinner.Dot),
spinner.WithStyle(style))`.
**Rationale**: v2 replaces field assignment with functional options for
immutable initialization.
**Alternatives considered**: None — field assignment no longer compiles in v2.

### D-007: Spinner Tick Migration

**Decision**: Use `s.Tick` method (v2) instead of `s.Tick` field (v1).
**Rationale**: v2 changes `Tick` from a `tea.Cmd` field to a method that returns
`tea.Cmd`. The call site `l.spinner.Tick` remains syntactically identical if Tick
is now a method returning Cmd.
**Alternatives considered**: None — this is mandatory.

### D-008: Width/Height Getter/Setter Migration

**Decision**: Replace `m.Width = N` with `m.SetWidth(N)` and `m.Width` reads
with `m.Width()` across table, textinput, and viewport components.
**Rationale**: v2 encapsulates width/height behind methods for all bubbles
components.
**Alternatives considered**: None — direct field access no longer compiles.

### D-009: Lipgloss Color Type

**Decision**: Change all `const` color declarations to `var`. Keep `lipgloss.Color("N")`
call syntax unchanged.
**Rationale**: `lipgloss.Color()` in v2 returns `color.Color` (from `image/color`),
which is an interface type. Go `const` requires compile-time constant expressions of
basic types (bool, numeric, string). Interface values cannot be constants, and function
calls are runtime expressions. Therefore `const ColorOK = lipgloss.Color("82")` will
not compile — must be `var ColorOK = lipgloss.Color("82")`.
**Alternatives considered**: Using predefined `lipgloss.Red`, `lipgloss.Green`, etc.
constants for basic ANSI colors (0-15) — not applicable since codebase uses ANSI-256
codes like "82", "196", "208".

### D-010: Lipgloss Style API

**Decision**: No changes needed for `NewStyle()`, `JoinVertical()`, `Width()`,
or style method chaining (`.Bold()`, `.Foreground()`, etc.).
**Rationale**: These APIs are unchanged in v2. The `Foreground(c color.Color)`
parameter type change is transparent since `lipgloss.Color()` returns the correct
type.
**Alternatives considered**: None needed.

### D-011: AdaptiveColor Usage

**Decision**: No migration needed for `AdaptiveColor`.
**Rationale**: The codebase does not use `lipgloss.AdaptiveColor`. All colors are
ANSI 256 constants. No compat package import is required.
**Alternatives considered**: N/A.

### D-012: Textinput Field Access

**Decision**: Only `Width` requires migration to `SetWidth()`/`Width()`. `Placeholder`
and `CharLimit` remain exported struct fields — no changes needed.
**Rationale**: Verified against bubbles v2.0.0 source (`textinput/textinput.go`). The
`Model` struct keeps `Placeholder string` and `CharLimit int` as exported fields. Only
`width` was made private with `SetWidth(int)` and `Width() int` accessor methods.
**Alternatives considered**: None — verified against v2 source code.

### D-013: WindowSizeMsg Unchanged

**Decision**: No migration needed for `tea.WindowSizeMsg`.
**Rationale**: The type name and fields (`Width`, `Height`) are identical in v2.
22 usages across 12 files require no changes beyond the import path update.
**Alternatives considered**: N/A.

### D-014: tea.Tick and tea.Batch Unchanged

**Decision**: No migration needed for `tea.Tick()` or `tea.Batch()`.
**Rationale**: Both function signatures are identical in v2. `tea.Quit` is also
unchanged.
**Alternatives considered**: N/A.

### D-015: Init() Signature Unchanged

**Decision**: No migration needed for `Init() tea.Cmd`.
**Rationale**: The v2.0.0 final release kept the v1 signature. Earlier betas
experimented with `Init() (tea.Model, tea.Cmd)` but this was reverted.
**Alternatives considered**: N/A.

## Codebase Impact Audit

### Summary

| Category | Call Sites | Files |
|----------|-----------|-------|
| Import path changes | 47 | 37 |
| `tea.KeyMsg` → `tea.KeyPressMsg` | ~117 | 14 |
| `View() string` → `View() tea.View` | 5 | 5 |
| Spinner migration | 7 | 4 |
| Table API (unchanged patterns) | 35 | 4 |
| Textinput field → setter | 10 | 3 |
| `lipgloss.Color()` | 45 | 5 |
| `lipgloss.NewStyle()` | 56 | 10 |
| `lipgloss.JoinVertical` | 9 | 4 |
| `tea.WindowSizeMsg` (unchanged) | 22 | 12 |
| `tea.Tick` (unchanged) | 2 | 2 |
| `tea.Batch` / `tea.Quit` (unchanged) | 18 | 4 |
| **Total estimated changes** | **~350** | **37** |

### Key Handling Patterns by File

| File | Pattern | Migration Approach |
|------|---------|-------------------|
| `cost_model.go` | `.String()` switch | Change type assertion only |
| `overview_model.go` | `.String()` switch | Change type assertion only |
| `recommendations_model.go` | `.String()` switch | Change type assertion only |
| `estimate_model.go` | `.Type` field switch | Change to `.Code` + `.Text` |
| `list/model.go` | `.Type` field switch | Change to `.Code` + `.Text` |

### Unchanged APIs (no migration needed)

- `tea.WindowSizeMsg` — same name and fields
- `tea.Tick()` — same signature
- `tea.Batch()` — same signature
- `tea.Quit` — same value
- `Init() tea.Cmd` — same signature
- `table.New()` with functional options — same API
- `table.DefaultStyles()` — same signature
- `lipgloss.NewStyle()` — same signature
- `lipgloss.JoinVertical()` / `lipgloss.JoinHorizontal()` — same signatures
- `lipgloss.Width()` — same signature
