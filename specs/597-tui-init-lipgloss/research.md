# Research: TUI Initializing View Lipgloss Consistency

**Branch**: `597-tui-init-lipgloss`
**Date**: 2026-02-18

## Summary

No external research required. All decisions are resolved by inspecting the existing codebase patterns in `internal/tui/`.

---

## Decision 1: Horizontal Composition Pattern

**Decision**: Use `lipgloss.JoinHorizontal(lipgloss.Left, spinnerView, " ", InfoStyle.Render(msg))`.

**Rationale**: `renderLoadingView` uses `lipgloss.JoinVertical(lipgloss.Left, ...)` for vertical stacking. The
horizontal spinner + label pattern is the standard Bubble Tea idiom and is what the issue's suggested code
shows. `lipgloss.Left` alignment is used throughout the file (not `lipgloss.Center`) for consistency.

**Alternatives considered**:

- `lipgloss.Center` alignment — the issue suggested this, but the rest of the file uses `Left`; deviating
  without a reason would introduce inconsistency.
- Inline `fmt.Sprintf("%s %s", spinner, InfoStyle.Render(msg))` — avoids JoinHorizontal but loses lipgloss
  layout integration.

---

## Decision 2: Outer Container Style and Padding

**Decision**: Wrap the composed content in `lipgloss.NewStyle().Width(m.width - borderPadding).Padding(1, 2).Render(content)`.

**Rationale**:

- `renderProgressBanner` uses `Padding(0, 1)` because it appears inline at the top of the screen with other
  content directly below it.
- `renderInitializingView` is the only thing on screen during its state, so vertical padding `Padding(1, 2)`
  provides breathing room matching the visual weight of `renderDetailView` (which uses `BoxStyle` with its own
  padding). This aligns with the issue's suggested code.
- `Width(m.width - borderPadding)` mirrors the pattern in `renderProgressBanner` (line 62) and `renderDetailView`
  (line 165).

**Alternatives considered**:

- `Padding(0, 1)` — matches renderProgressBanner but leaves the initializing view cramped; the issue explicitly
  suggests `Padding(1, 2)`.
- No outer wrapper — skips width responsiveness, which is the core FR-003 requirement.

---

## Decision 3: `borderPadding` Source

**Decision**: Reuse the existing `borderPadding = 2` constant from `internal/tui/cost_view.go`.

**Rationale**: It is already used in `overview_view.go` lines 62 and 165 without redefinition. It is a
package-level constant shared across the `tui` package.

**Alternatives considered**: Defining a local constant — unnecessary duplication.

---

## Decision 4: `fmt` Import

**Decision**: Keep the `fmt` import in `overview_view.go`.

**Rationale**: `fmt` is used in at least 7 other places in the file (`renderStatusBar`, `renderDetailCostDrift`,
`renderDetailRecommendations`, `renderDetailError`, `renderBreakdown`, and the `ViewStateError` case). FR-007
condition ("if no longer used") is not met.

---

## Decision 5: nil `loadingState` Safety

**Decision**: Retain the existing nil guard (`if m.loadingState != nil`) unchanged.

**Rationale**: `NewOverviewModel` always initializes `loadingState`, but the nil guard is a defensive pattern
already present and should not be removed. No additional clamping is needed beyond what is already there.

---

## Decision 6: Width Zero / Negative Clamping

**Decision**: No explicit clamping required.

**Rationale**: lipgloss silently ignores width constraints ≤ 0 — the content renders at its natural width.
`defaultWidth = 80` ensures test models always have a positive width. No guard code is needed.

---

## Decision 7: New Tests Required

**Decision**: Add two tests to `overview_view_test.go`:

1. `TestOverviewView_InitializingRender_WidthRespected` — sets `m.width` to a known value and verifies the
   output length is constrained.
2. `TestOverviewView_InitializingRender_NilLoadingState` — manually sets `m.loadingState = nil` and calls
   `renderInitializingView()` directly, asserting no panic and message present.

Existing tests (`TestOverviewView_InitializingRender`, `TestOverviewView_InitializingDefaultMsg`) continue to
pass because message content is preserved.
