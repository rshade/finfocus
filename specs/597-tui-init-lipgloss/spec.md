# Feature Specification: TUI Initializing View Lipgloss Consistency

**Feature Branch**: `597-tui-init-lipgloss`
**Created**: 2026-02-18
**Status**: Draft
**Input**: User description: "feat: use lipgloss styles in renderInitializingView for consistency"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consistent Visual Style During Startup (Priority: P1)

A developer or user launches the `finfocus` overview command and briefly sees the "Initializing..." state while the TUI prepares. This initializing screen should look visually identical in style to the loading and progress states that follow—same font weight, same color scheme, same padding, and the same terminal-width-aware layout.

**Why this priority**: This is the sole change described in the issue. Without it the initializing state is visually inconsistent: it uses plain text formatting while every other TUI state uses the shared lipgloss style system.

**Independent Test**: Can be fully tested by calling `renderInitializingView()` in a unit test and asserting the returned string contains lipgloss-rendered output (styled text via `InfoStyle`) rather than a bare `fmt.Sprintf` string, and that the model's `width` field is reflected in the output.

**Acceptance Scenarios**:

1. **Given** the TUI model is in `ViewStateInitializing` with a non-empty `progressMsg`, **When** `renderInitializingView()` is called, **Then** the returned string is rendered using `InfoStyle` and respects the model's `width` field.
2. **Given** the TUI model is in `ViewStateInitializing` with an empty `progressMsg`, **When** `renderInitializingView()` is called, **Then** the fallback text "Initializing..." is rendered using `InfoStyle`.
3. **Given** the TUI model is in `ViewStateInitializing` with a nil `loadingState`, **When** `renderInitializingView()` is called, **Then** no panic occurs and the message is still rendered correctly.
4. **Given** the TUI model is in `ViewStateInitializing` and the terminal width changes (window resize event), **When** `renderInitializingView()` is called after the resize, **Then** the rendered output respects the new width value stored in the model.

---

### Edge Cases

- What happens when `m.loadingState` is nil? The spinner view is treated as an empty string and the message still renders without panic.
- What happens when `m.width` is 0? Width calculation (`m.width - borderPadding`) must not produce a negative width that breaks layout; the view must degrade gracefully.
- What happens when `m.progressMsg` is empty? The fallback "Initializing..." text is used—preserving existing behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `renderInitializingView` MUST use `InfoStyle` to render the status message instead of inserting it as a plain string via `fmt.Sprintf`.
- **FR-002**: `renderInitializingView` MUST compose the spinner and message horizontally using `lipgloss.JoinHorizontal`, matching the composition pattern used in `renderProgressBanner` and `renderLoadingView`.
- **FR-003**: `renderInitializingView` MUST apply a width constraint derived from the model's `width` field (e.g., `m.width - borderPadding`) so the view adapts to terminal size, consistent with `renderProgressBanner` and `renderDetailView`.
- **FR-004**: `renderInitializingView` MUST apply padding consistent with the visual rhythm of adjacent TUI states.
- **FR-005**: `renderInitializingView` MUST retain the fallback text "Initializing..." when `m.progressMsg` is empty, as in the current implementation.
- **FR-006**: `renderInitializingView` MUST handle a nil `loadingState` without panicking, treating the spinner contribution as an empty string.
- **FR-007**: The `fmt` import in `overview_view.go` MUST be removed if it is no longer used after this change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The initializing view is visually consistent in style (color, weight, padding pattern) with the loading and progress banner views when reviewed in the same terminal session.
- **SC-002**: Unit tests for `renderInitializingView` cover: default fallback text, custom `progressMsg`, nil `loadingState`, and non-zero width scenarios—all passing.
- **SC-003**: `make lint` and `make test` pass with zero new violations after the change.
- **SC-004**: No regression in any other `View()` states (loading, list, detail, error, quitting) as confirmed by existing and updated unit tests.

## Assumptions

- `borderPadding` is an existing package-level constant already used throughout `overview_view.go`; the fix reuses it without modification.
- The fix does not introduce new exported types, interfaces, or public API surface.
- The `fmt` import removal is contingent on no other usages remaining in the file after the change; if other usages exist the import stays.
