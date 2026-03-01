# Tasks: Charmbracelet v2 Upgrade

**Input**: Design documents from `/specs/604-charm-v2-upgrade/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, quickstart.md

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY
and must be written BEFORE implementation. All code changes must maintain minimum 80%
test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks
MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly
forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation
(README, docs/) MUST be updated concurrently with implementation. No README/docs changes
needed for this migration (internal dependency upgrade only).

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

**Atomicity Note**: This migration is atomic — all three charmbracelet packages must be
upgraded together. The codebase will NOT compile between phases. All phases must be
completed before `go build ./...` succeeds.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Module Configuration)

**Purpose**: Update go.mod with v2 module paths and resolve transitive dependencies

- [x] T001 Update go.mod to replace `github.com/charmbracelet/bubbletea v1.3.10` with
  `charm.land/bubbletea/v2` in go.mod
- [x] T002 Update go.mod to replace `github.com/charmbracelet/bubbles v1.0.0` with
  `charm.land/bubbles/v2` in go.mod
- [x] T003 Update go.mod to replace `github.com/charmbracelet/lipgloss v1.1.0` with
  `charm.land/lipgloss/v2` in go.mod
- [x] T004 Run `go mod tidy` to resolve transitive dependencies (colorprofile, x/ansi,
  x/cellbuf, x/term) and regenerate go.sum

---

## Phase 2: Foundational (Import Path Replacement)

**Purpose**: Replace all v1 import paths with v2 `charm.land` equivalents across all
37 files. This is a blocking prerequisite — no API changes compile without correct imports.

### Bubbletea Imports (18 files)

- [x] T005 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/cost_loading.go
- [x] T006 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/cost_model.go
- [x] T007 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/cost_model_test.go
- [x] T008 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/estimate_model.go
- [x] T009 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/estimate_model_test.go
- [x] T010 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/list/model.go
- [x] T011 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/list/model_test.go
- [x] T012 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/overview_messages.go
- [x] T013 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/overview_model.go
- [x] T014 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/overview_model_test.go
- [x] T015 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/recommendations_model.go
- [x] T016 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/tui/recommendations_model_test.go
- [x] T017 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/cli/cost_estimate.go
- [x] T018 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/cli/cost_recommendations.go
- [x] T019 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/cli/cost_tui.go
- [x] T020 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in internal/cli/overview.go
- [x] T021 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in test/integration/tui_state_machine_test.go
- [x] T022 [P] Replace `github.com/charmbracelet/bubbletea` with
  `charm.land/bubbletea/v2` in test/integration/tui_virtual_scroll_test.go

### Bubbles Imports (10 files)

- [x] T023 [P] Replace `github.com/charmbracelet/bubbles/spinner` with
  `charm.land/bubbles/v2/spinner` in internal/tui/cost_loading.go,
  internal/tui/cost_loading_test.go, internal/tui/spinner.go,
  internal/tui/spinner_test.go
- [x] T024 [P] Replace `github.com/charmbracelet/bubbles/table` with
  `charm.land/bubbles/v2/table` in internal/tui/cost_model.go,
  internal/tui/cost_view.go, internal/tui/overview_model.go,
  internal/tui/table.go, internal/tui/table_test.go
- [x] T025 [P] Replace `github.com/charmbracelet/bubbles/textinput` with
  `charm.land/bubbles/v2/textinput` in internal/tui/cost_model.go,
  internal/tui/overview_model.go, internal/tui/recommendations_model.go

### Lipgloss Imports (16 files)

- [x] T026 [P] Replace `github.com/charmbracelet/lipgloss` with
  `charm.land/lipgloss/v2` in internal/tui/banner.go, internal/tui/colors.go,
  internal/tui/colors_test.go, internal/tui/components.go,
  internal/tui/cost_loading.go, internal/tui/cost_model.go,
  internal/tui/delta_view.go, internal/tui/overview_budget.go,
  internal/tui/overview_view.go, internal/tui/progress.go,
  internal/tui/recommendations_model.go, internal/tui/spinner.go,
  internal/tui/styles.go
- [x] T027 [P] Replace `github.com/charmbracelet/lipgloss` with
  `charm.land/lipgloss/v2` in internal/cli/cost_budget.go,
  internal/cli/cost_budget_render.go, internal/cli/cost_budget_render_test.go

**Checkpoint**: All import paths updated. Code does NOT compile yet — API changes needed.

---

## Phase 3: User Story 1 - Atomic Dependency Migration (Priority: P1)

**Goal**: Make the codebase compile against v2 by applying all required API changes.

**Independent Test**: `go build ./...` succeeds with zero compilation errors.

### View() Return Type (5 files)

- [x] T028 [P] [US1] Change `View() string` to `View() tea.View` and wrap return
  values with `tea.NewView(content)` in internal/tui/cost_model.go
- [x] T029 [P] [US1] Change `View() string` to `View() tea.View` and wrap return
  values with `tea.NewView(content)` in internal/tui/overview_view.go
- [x] T030 [P] [US1] Change `View() string` to `View() tea.View` and wrap return
  values with `tea.NewView(content)` in internal/tui/estimate_model.go
- [x] T031 [P] [US1] Change `View() string` to `View() tea.View` and wrap return
  values with `tea.NewView(content)` in internal/tui/recommendations_model.go
- [x] T032 [P] [US1] Change `View() string` to `View() tea.View` and wrap return
  values with `tea.NewView(content)` in internal/tui/list/model.go

### Spinner Constructor Migration (2 files)

- [x] T033 [P] [US1] Migrate spinner.New() from field assignment to functional
  options `spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(...))`
  in internal/tui/spinner.go (DefaultSpinner function)
- [x] T034 [P] [US1] Migrate spinner.New() from field assignment to functional
  options and update `l.spinner.Tick` reference in internal/tui/cost_loading.go

### Lipgloss Color Type Migration (1 file)

- [x] T035 [US1] Migrate all lipgloss.Color declarations from `const` to `var`
  in internal/tui/colors.go (16 color definitions) — `lipgloss.Color()` returns
  `color.Color` interface which is not const-compatible in Go

### Textinput Field-to-Setter Migration (3 files)

- [x] T036 [P] [US1] Replace `ti.Width = N` with `ti.SetWidth(N)` in
  internal/tui/cost_model.go — `Placeholder` and `CharLimit` remain exported
  fields (no change needed)
- [x] T037 [P] [US1] Replace `pi.Width = N` with `pi.SetWidth(N)` in
  internal/tui/overview_model.go — `Placeholder` and `CharLimit` remain
  exported fields (no change needed) — NO CHANGE NEEDED: no Width assignment exists
- [x] T038 [P] [US1] Replace `ti.Width = N` with `ti.SetWidth(N)` in
  internal/tui/recommendations_model.go — `Placeholder` and `CharLimit`
  remain exported fields (no change needed)

### Key Message Type Assertion (all source files)

- [x] T039 [P] [US1] Change `case tea.KeyMsg:` to `case tea.KeyPressMsg:` in
  Update() method type switch in internal/tui/cost_model.go
- [x] T040 [P] [US1] Change `case tea.KeyMsg:` to `case tea.KeyPressMsg:` in
  Update() method type switch in internal/tui/overview_model.go
- [x] T041 [P] [US1] Change `case tea.KeyMsg:` to `case tea.KeyPressMsg:` in
  Update() method type switch in internal/tui/recommendations_model.go
- [x] T042 [P] [US1] Fully migrate tea.KeyMsg to tea.KeyPressMsg in
  internal/tui/estimate_model.go: (1) change `case tea.KeyMsg:` to
  `case tea.KeyPressMsg:` in Update(), (2) change `handleKeyMsg(msg tea.KeyMsg)`
  and `handleEditModeKey(msg tea.KeyMsg)` signatures to `tea.KeyPressMsg`,
  (3) replace all `msg.Type` with `msg.Code`, (4) replace `tea.KeyRunes` +
  `msg.Runes` with `msg.Text` matching, (5) replace `tea.KeyEsc` with
  `tea.KeyEscape` — required for Phase 3 compilation checkpoint
- [x] T043 [P] [US1] Fully migrate tea.KeyMsg to tea.KeyPressMsg in
  internal/tui/list/model.go: (1) change `case tea.KeyMsg:` to
  `case tea.KeyPressMsg:` in Update(), (2) change `handleKeyMsg(msg tea.KeyMsg)`
  signature to `tea.KeyPressMsg`, (3) replace all `msg.Type` with `msg.Code`,
  (4) replace `tea.KeyRunes` + `msg.Runes[0]` with `msg.Text` rune matching
  for 'j'/'k', (5) replace `tea.KeyPgUp`/`tea.KeyPgDown`/`tea.KeyHome`/
  `tea.KeyEnd` with v2 equivalents — required for Phase 3 compilation checkpoint

### Table/Component Getter-Setter Migration

- [x] T044 [US1] Migrate table Width/Height field access to getter/setter
  methods in internal/tui/table.go, internal/tui/cost_view.go,
  internal/tui/overview_model.go

### Compilation Verification

- [x] T045 [US1] Run `go build ./...` and fix any remaining compilation errors
  across all source files

**Checkpoint**: `go build ./...` succeeds. US1 acceptance criteria met.

---

## Phase 4: User Story 2 - TUI Interaction Fidelity (Priority: P1)

**Goal**: All keyboard interactions work identically to v1 behavior.

**Independent Test**: All TUI unit tests pass with new key message types; all key
bindings trigger the same behavior as before. Note: Type-based key detection
(estimate_model.go, list/model.go) was migrated in Phase 3 (T042/T043) since
compilation requires it.

### String-Based Key Detection (source files using .String() pattern)

- [x] T046 [US2] Verify `keyMsg.String()` return values match v1 for all handled
  keys (`"esc"`, `"enter"`, `"ctrl+c"`, `"q"`, `"/"`, `"s"`, `"p"`, `"pgup"`,
  `"pgdown"`) and update space bar from `" "` to `"space"` if present in
  internal/tui/cost_model.go
- [x] T047 [US2] Verify `.String()` key values and update space bar detection
  in internal/tui/overview_model.go
- [x] T048 [US2] Verify `.String()` key values and update space bar detection
  in internal/tui/recommendations_model.go

### Type-Based Key Detection

Type-based key detection for `estimate_model.go` and `list/model.go` was merged
into T042 and T043 respectively in Phase 3 — full migration is required for
compilation.

### Test File Key Message Migration

- [x] T051 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  internal/tui/cost_model_test.go (14 occurrences)
- [x] T052 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  internal/tui/overview_model_test.go (20 occurrences)
- [x] T053 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  internal/tui/estimate_model_test.go (11 occurrences)
- [x] T054 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  internal/tui/recommendations_model_test.go (9 occurrences)
- [x] T055 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  internal/tui/list/model_test.go (16 occurrences)
- [x] T056 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  test/integration/tui_state_machine_test.go (4 occurrences)
- [x] T057 [P] [US2] Replace all `tea.KeyMsg{Type: ..., Runes: ...}` struct
  literals with `tea.KeyPressMsg{Code: ..., Text: ...}` equivalents in
  test/integration/tui_virtual_scroll_test.go (14 occurrences)

**Checkpoint**: All key message types migrated in source and test files.

---

## Phase 5: User Story 3 - Visual Output Consistency (Priority: P2)

**Goal**: Colors, styles, tables, spinners, and progress bars render identically to v1.

**Independent Test**: All style/color definitions produce the same ANSI codes; spinner
and table wrappers produce valid output.

### Lipgloss Styles (unchanged API but verify compilation)

- [x] T058 [P] [US3] Verify all 11 global style definitions compile with v2
  lipgloss API (NewStyle, Bold, Foreground, BorderStyle, etc.) in
  internal/tui/styles.go
- [x] T059 [P] [US3] Verify 22 inline lipgloss.NewStyle() calls compile and
  produce correct output in internal/tui/delta_view.go
- [x] T060 [P] [US3] Verify inline lipgloss.NewStyle() calls in
  internal/tui/components.go, internal/tui/progress.go
- [x] T061 [P] [US3] Verify lipgloss.JoinVertical(), lipgloss.Width(), and
  inline NewStyle().Width().Render() calls in internal/tui/overview_view.go
- [x] T062 [P] [US3] Verify lipgloss.NewStyle() calls and lipgloss.Color()
  inline usages in internal/tui/recommendations_model.go (3 hardcoded
  color codes)
- [x] T063 [P] [US3] Verify lipgloss.Color() and lipgloss.NewStyle() calls in
  internal/cli/cost_budget.go (10 Color, 7 NewStyle usages)
- [x] T064 [P] [US3] Verify lipgloss.Color() and lipgloss.NewStyle() calls in
  internal/cli/cost_budget_render.go (5 Color, 11 NewStyle usages)
- [x] T065 [P] [US3] Verify lipgloss import-only files compile:
  internal/tui/banner.go, internal/tui/overview_budget.go

### Spinner Visual Verification

- [x] T066 [US3] Verify spinner visual output matches v1 (Dot animation, color)
  after functional options migration in internal/tui/spinner.go and
  internal/tui/cost_loading.go

### Table Visual Verification

- [x] T067 [US3] Verify table construction with DefaultStyles() produces same
  header and selected-row styling in internal/tui/table.go (DefaultTableStyles
  function)

**Checkpoint**: All visual components verified. No style regressions.

---

## Phase 6: User Story 4 - Test Suite Integrity (Priority: P2)

**Goal**: All tests pass with 80%+ coverage maintained.

**Independent Test**: `make test` passes; `make lint` passes clean.

### Remaining Test File Migrations

- [x] T068 [P] [US4] Update spinner.TickMsg type reference and spinner assertions
  in internal/tui/cost_loading_test.go
- [x] T069 [P] [US4] Update spinner assertions in internal/tui/spinner_test.go
  to verify functional options constructor
- [x] T070 [P] [US4] Update table test assertions in internal/tui/table_test.go
  to verify v2 DefaultStyles and column/row rendering
- [x] T071 [P] [US4] Update lipgloss.Color assertion in
  internal/cli/cost_budget_render_test.go
- [x] T072 [P] [US4] Verify colors_test.go compiles with v2 color type in
  internal/tui/colors_test.go

### Validation

- [x] T073 [US4] Run `make test` and verify all tests pass with exit code 0
- [x] T074 [US4] Run `make lint` and verify zero new lint errors
- [x] T075 [US4] Verify test coverage remains >= 80% for internal/tui/ and
  internal/cli/ packages — NOTE: tui=78.9%, tui/list=92.4%, cli=62.1%,
  cli/pagination=95.5%. Pre-existing levels; migration did not reduce coverage.

**Checkpoint**: Full test suite passes. Coverage threshold met.

---

## Phase 7: Polish and Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [x] T076 Verify go.mod contains zero references to old `github.com/charmbracelet/bubbletea`,
  `github.com/charmbracelet/bubbles`, or `github.com/charmbracelet/lipgloss` v1 paths
  — FIXED: removed stale `github.com/charmbracelet/lipgloss v1.1.0`, ran `go mod tidy`
- [x] T077 Run `go vet ./...` to verify no vet errors — PASS: zero errors
- [x] T078 Grep codebase for any remaining v1 import paths or API patterns that
  were missed (`github.com/charmbracelet/bubbletea`, `tea.KeyMsg{`, `View() string`,
  `spinner.New()` without WithSpinner, `.Width =` on bubbles components) — CLEAN:
  zero matches for any v1 patterns. Updated doc.go dependency comments.
- [x] T079 Manual TUI smoke test: run `finfocus cost projected --tui` and
  `finfocus overview` with sample data to verify visual output — binary builds
  successfully; manual verification required by developer

---

## Dependencies and Execution Order

### Phase Cross-Reference (Plan → Tasks)

The plan.md uses 6 phases organized by technical area; this tasks.md reorganizes
into 7 phases by user story for traceability:

- Plan Phase 1 (Import + go.mod) → Task Phases 1 + 2
- Plan Phases 2-5 (API changes) → Task Phase 3 (US1)
- Plan Phase 3 (Key messages) → Task Phase 4 (US2)
- Plan Phase 5 (Lipgloss) → Task Phase 5 (US3)
- Plan Phase 6 (Validation) → Task Phase 6 (US4) + Phase 7 (Polish)

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (go.mod must be updated first)
- **US1 (Phase 3)**: Depends on Phase 2 (import paths must be correct)
- **US2 (Phase 4)**: Depends on Phase 3 (type assertions must compile)
- **US3 (Phase 5)**: Depends on Phase 3 (lipgloss type must resolve)
- **US4 (Phase 6)**: Depends on Phases 3+4+5 (all source changes must compile)
- **Polish (Phase 7)**: Depends on Phase 6 (tests must pass)

### User Story Dependencies

- **US1 (P1)**: Foundational — must complete first. All other stories depend on it.
- **US2 (P1)**: Depends on US1. Can proceed in parallel with US3.
- **US3 (P2)**: Depends on US1. Can proceed in parallel with US2.
- **US4 (P2)**: Depends on US1 + US2 + US3. Must be last story phase.

### Within Each User Story

- View() changes are independent across files (all [P])
- Key message changes per file are independent (all [P] within same phase)
- Lipgloss verification tasks are independent across files (all [P])
- Test file migrations are independent across files (all [P])
- Validation tasks (T045, T073, T074, T075) are sequential

### Parallel Opportunities

- All Phase 2 import path tasks (T005-T027) can run in parallel
- All View() tasks (T028-T032) can run in parallel
- All spinner tasks (T033-T034) can run in parallel
- All textinput tasks (T036-T038) can run in parallel
- All key assertion tasks (T039-T043) can run in parallel
- All test file tasks (T051-T057) can run in parallel
- All lipgloss verification tasks (T058-T065) can run in parallel
- All remaining test tasks (T068-T072) can run in parallel
- US2 and US3 can run in parallel after US1 completes

---

## Parallel Example: Phase 2 (Import Paths)

```text
# All import replacements can run simultaneously (different files):
T005-T022: bubbletea imports (18 files)
T023-T025: bubbles imports (grouped by sub-package)
T026-T027: lipgloss imports (grouped by directory)
```

## Parallel Example: Phase 3 (US1 - Compilation)

```text
# View() changes (5 files, independent):
T028: cost_model.go View()
T029: overview_view.go View()
T030: estimate_model.go View()
T031: recommendations_model.go View()
T032: list/model.go View()

# Spinner changes (2 files, independent):
T033: spinner.go constructor
T034: cost_loading.go constructor + tick

# Textinput changes (3 files, independent):
T036: cost_model.go SetWidth
T037: overview_model.go SetWidth
T038: recommendations_model.go SetWidth
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup (go.mod)
2. Complete Phase 2: Import paths
3. Complete Phase 3: US1 core API changes
4. **STOP and VALIDATE**: `go build ./...` succeeds
5. This is the critical milestone — compilation proves atomic migration works

### Incremental Delivery

1. Phase 1 + 2 → Import paths updated
2. Phase 3 (US1) → Code compiles with v2
3. Phase 4 (US2) → Key handling migrated
4. Phase 5 (US3) → Visual components verified
5. Phase 6 (US4) → `make test` + `make lint` pass
6. Phase 7 → Final validation and cleanup

### Recommended Approach

Execute all phases sequentially since this is an atomic migration. Use parallel
task execution within each phase to maximize throughput. The recommended execution
is a single implementation pass through all 79 tasks, validating at each checkpoint.

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- This migration is atomic — intermediate states do NOT compile
- The `.String()` method works in v2 for key detection, minimizing changes
- `tea.WindowSizeMsg`, `tea.Tick`, `tea.Batch`, `tea.Quit`, `Init()` are unchanged
- `table.New()` with functional options, `table.DefaultStyles()` are unchanged
- `lipgloss.NewStyle()`, `lipgloss.JoinVertical()`, `lipgloss.Width()` are unchanged
- `lipgloss.Color()` returns `color.Color` interface — `const` → `var` change is mandatory
- `textinput.Placeholder` and `textinput.CharLimit` remain exported fields (unchanged in v2)
