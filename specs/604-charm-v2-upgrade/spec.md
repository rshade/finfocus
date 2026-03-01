# Feature Specification: Upgrade Charmbracelet Dependencies to v2

**Feature Branch**: `604-charm-v2-upgrade`
**Created**: 2026-02-28
**Status**: Draft
**Input**: GitHub Issue #827 — Upgrade charmbracelet dependencies (bubbles, bubbletea, lipgloss)
from v1 to v2 with charm.land import paths

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Atomic Dependency Migration (Priority: P1)

As a maintainer of the FinFocus TUI, I want all three charmbracelet packages
(bubbletea, bubbles, lipgloss) upgraded from v1 to v2 simultaneously, so that
the codebase compiles and runs against the latest supported major versions.

**Why this priority**: The three packages are tightly coupled — they share types
and interfaces. A partial upgrade will not compile. This is the foundational
prerequisite for all other migration work. Without it, nothing else can proceed.

**Independent Test**: Can be verified by running `go build ./...` successfully
after updating all import paths and `go.mod` entries. The project compiles with
zero errors against the v2 module paths.

**Acceptance Scenarios**:

1. **Given** the project depends on bubbletea v1, bubbles v1, and lipgloss v1,
   **When** all three are upgraded to their v2 equivalents with `charm.land`
   import paths, **Then** `go build ./...` succeeds with no compilation errors.
2. **Given** the `go.mod` file references old `github.com/charmbracelet/*` paths,
   **When** the migration is complete, **Then** `go.mod` contains only the new
   `charm.land/*` module paths for these three packages.
3. **Given** transitive charmbracelet dependencies exist (colorprofile, x/ansi,
   x/cellbuf, x/term), **When** the main packages are upgraded, **Then** all
   transitive dependencies resolve correctly via `go mod tidy`.

---

### User Story 2 - TUI Interaction Fidelity (Priority: P1)

As a user of FinFocus TUI commands (`cost projected --tui`, `overview`), I want
all keyboard interactions to work identically after the upgrade, so that my
muscle memory and workflows are preserved.

**Why this priority**: Keyboard handling is the primary user interaction mechanism.
The v2 API replaces `tea.KeyMsg` with `tea.KeyPressMsg` and restructures key
detection. If this migration is incorrect, the TUI becomes unusable.

**Independent Test**: Can be verified by running all TUI unit tests and integration
tests (state machine, virtual scroll) with the new key message types. All existing
test scenarios pass without modification to expected behaviors.

**Acceptance Scenarios**:

1. **Given** a user is on the cost view, **When** they press `q`, `esc`,
   `enter`, `/`, `s`, `p`, arrow keys, `pgup`/`pgdown`, or `ctrl+c`, **Then**
   each key triggers the same behavior as before the upgrade.
2. **Given** a user is navigating the virtual list, **When** they press `j`/`k`
   (vim keys), `Home`/`End`, or `PageUp`/`PageDown`, **Then** scrolling and
   selection behavior is identical to v1.
3. **Given** existing integration tests for TUI state machine transitions,
   **When** the key message types are migrated to v2 API, **Then** all
   integration tests pass without changes to expected outcomes.

---

### User Story 3 - Visual Output Consistency (Priority: P2)

As a user of FinFocus, I want the TUI visual output (colors, styles, tables,
spinners, progress bars, borders) to look identical after the upgrade, so that
I experience no visual regressions.

**Why this priority**: Visual consistency ensures user trust. The v2 API changes
color types, style construction patterns, and table/spinner constructors. While
not blocking basic functionality, visual regressions degrade the user experience.

**Independent Test**: Can be verified by running `finfocus cost projected --tui`
and `finfocus overview` with sample data and confirming visual output matches
pre-upgrade appearance. Automated unit tests verify style/color definitions
produce valid output.

**Acceptance Scenarios**:

1. **Given** 14 color constants are defined in the TUI theme, **When** they are
   migrated to the v2 color API, **Then** each color renders the same ANSI code
   as before.
2. **Given** 11 global style definitions use lipgloss, **When** they are migrated
   to the v2 style API, **Then** each style applies the same visual formatting
   (bold, foreground, border, padding).
3. **Given** tables are constructed with functional options, **When** the v2
   getter/setter API is applied, **Then** tables render with the same column
   widths, headers, and selected-row highlighting.
4. **Given** spinners use a dot animation with custom color, **When** the v2
   functional-option constructor is applied, **Then** the spinner animates
   with the same dot pattern and color.

---

### User Story 4 - Test Suite Integrity (Priority: P2)

As a maintainer, I want all existing tests (unit, integration) to pass after the
upgrade with at least 80% coverage maintained, so that I have confidence the
migration introduced no behavioral regressions.

**Why this priority**: The test suite is the safety net for this migration. Without
passing tests, the upgrade cannot be merged.

**Independent Test**: Can be verified by running `make test` and confirming all
tests pass. Coverage report shows >= 80% for affected packages.

**Acceptance Scenarios**:

1. **Given** the full test suite, **When** `make test` is run after migration,
   **Then** all tests pass with exit code 0.
2. **Given** test files that construct key message values for input simulation,
   **When** they are migrated to the v2 key message API, **Then** each test
   still exercises the same user interaction it was designed to test.
3. **Given** the linter configuration, **When** `make lint` is run after migration,
   **Then** no new lint errors are introduced.

---

### Edge Cases

- What happens if a v2 component returns a different default style than v1?
  The migration must explicitly set styles rather than relying on defaults.
- How does the generic virtual list model handle the View() return type change?
  The generic `VirtualListModel[T]` must return the correct type for the Model
  interface.
- What happens if inline hardcoded color codes in non-centralized locations
  (e.g., `recommendations_model.go`) are missed during migration?
  A comprehensive search must identify all color usages, not just those in
  `colors.go` and `styles.go`.
- What if the spinner tick mechanism changes behavior in v2?
  The `LoadingState.Init()` function must be updated to use the v2 tick pattern.
- What happens if `DefaultStyles()` returns different default values in v2?
  The `DefaultTableStyles()` wrapper must explicitly override all relevant fields.
- What if key string representations differ between v1 and v2 (e.g., `"pgup"` vs
  `"page up"`)? Both `.String()` and `.Type` based detection patterns must be
  validated against the v2 key naming.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST replace all bubbletea v1 imports with
  the v2 `charm.land/bubbletea/v2` module path across all source and test files.
- **FR-002**: System MUST replace all bubbles v1 imports with
  the v2 `charm.land/bubbles/v2/*` module paths across all source and test files.
- **FR-003**: System MUST replace all lipgloss v1 imports with
  the v2 `charm.land/lipgloss/v2` module path across all source and test files.
- **FR-004**: All `View()` methods implementing the tea.Model interface MUST
  return the v2-correct type (5 models: CostViewModel, OverviewModel,
  EstimateModel, RecommendationsViewModel, VirtualListModel).
- **FR-005**: All key message type assertions and switch statements MUST be
  migrated to the v2 key press message equivalent, preserving identical key
  detection behavior for all handled keys.
- **FR-006**: All key type field comparisons (e.g., KeyUp, KeyDown, KeyEnter,
  KeyEsc, KeyRunes) MUST be migrated to the v2 key code API.
- **FR-007**: All key string comparisons (e.g., `"esc"`, `"enter"`, `"ctrl+c"`,
  `"q"`, `"/"`) MUST be migrated to the v2 key press message equivalent while
  preserving the same detection logic.
- **FR-008**: Spinner construction MUST migrate from field assignment pattern
  to functional options pattern (WithSpinner, WithStyle).
- **FR-009**: All spinner tick references used as command return values MUST be
  updated to the v2 equivalent.
- **FR-010**: Table and other component width/height access MUST migrate from
  exported fields to getter/setter methods (SetWidth, SetHeight, Width, Height).
- **FR-011**: Table default styles usage MUST be validated against v2 API; all
  struct field assignments for Header and Selected styles MUST remain compatible.
- **FR-012**: All textinput width assignments MUST migrate from field access
  to setter method calls.
- **FR-013**: All lipgloss color constant definitions MUST produce the same
  ANSI color codes after migration to the v2 color type.
- **FR-014**: All lipgloss style definitions MUST preserve the same visual
  formatting (bold, italic, foreground, background, border, padding) after
  migration.
- **FR-015**: The `go.mod` file MUST be updated with the new `charm.land` module
  paths and `go.sum` MUST be regenerated cleanly.
- **FR-016**: The migration MUST be atomic — all three packages upgraded in a
  single compilable state. No intermediate state should fail to compile.

### Key Entities

- **TUI Model**: A component implementing Init, Update, and View — 5 distinct
  models in the codebase with different receiver types (pointer vs value).
- **Key Message**: The event type representing a keyboard press — changes from
  the v1 type to a restructured v2 type with different fields for key code,
  text, and modifiers.
- **Style**: A lipgloss visual formatting definition — 11 global styles plus
  inline styles scattered across 12+ files.
- **Color Constant**: A named ANSI color value — 14+ constants centralized in
  `colors.go` plus hardcoded values in other files.
- **Component**: A bubbles widget (spinner, table, textinput) that wraps state
  and rendering — constructors and field access patterns change between v1 and v2.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All TUI keyboard interactions behave identically to pre-upgrade
  behavior — zero user-facing behavioral changes.
- **SC-002**: `make test` passes with all existing tests, maintaining at least
  80% code coverage on affected packages.
- **SC-003**: `make lint` passes cleanly with zero new lint warnings or errors.
- **SC-004**: All 5 TUI model View methods render valid output consumable by the
  Bubble Tea runtime.
- **SC-005**: Integration tests (TUI state machine, virtual scroll) pass without
  modification to their expected behavioral outcomes.
- **SC-006**: Visual output of TUI commands shows no regressions when manually
  inspected against pre-upgrade output.
- **SC-007**: The `go.mod` file contains zero references to the old v1 module
  paths for bubbletea, bubbles, or lipgloss.

## Assumptions

- The v2 releases of all three packages are stable and published to the
  `charm.land` vanity domain.
- The v2 key press message `String()` method returns compatible string
  representations for keys like `"esc"`, `"enter"`, `"ctrl+c"`, `"q"`, etc.,
  allowing string-based key detection to continue working.
- The v2 `lipgloss.Color()` function accepts the same ANSI color code strings
  (e.g., `"82"`, `"208"`, `"#FF0000"`) as v1, producing equivalent visual output.
- The v2 spinner tick field (or equivalent) still functions as a valid command
  for initializing spinner animation.
- The v2 table package preserves the functional-options constructor pattern
  used by the existing codebase.
- No changes to the Pulumi Analyzer gRPC protocol or plugin host are required
  by this upgrade — only TUI-layer code is affected.

## Scope Boundaries

### In Scope

- Migrating all three charmbracelet packages to v2 with new import paths
- Updating all affected source files (~21) and test files (~16)
- Updating `go.mod` and `go.sum`
- Fixing all compilation errors caused by API changes
- Ensuring all existing tests pass

### Out of Scope

- Adopting new v2-only features (cursor control, clipboard, Mode 2026/2027)
- Refactoring TUI architecture beyond what the migration requires
- Upgrading other non-charmbracelet dependencies
- Adding new TUI components or views
- Changing TUI behavior or adding new keyboard shortcuts
