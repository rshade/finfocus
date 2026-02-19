# Tasks: TUI Initializing View Lipgloss Consistency

**Input**: Design documents from `specs/597-tui-init-lipgloss/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, quickstart.md ✓

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and
must be written BEFORE implementation. Tests must FAIL with the current code before
implementation begins.

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks
MUST be fully implemented. No stubs, no TODOs.

**Documentation**: No exported symbols added or changed; no README/docs update required.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- Exact file paths are included in all descriptions

## Phase 1: Setup (Baseline Verification)

**Purpose**: Confirm the existing test suite passes before any changes are made.

- [X] T001 Run `go test ./internal/tui/...` and confirm all tests pass — establishes clean baseline

---

## Phase 2: User Story 1 — Consistent Visual Style During Startup (Priority: P1) 🎯 MVP

**Goal**: Replace the `fmt.Sprintf` call in `renderInitializingView()` with a
lipgloss-composed view that uses `InfoStyle`, horizontal spinner+message layout,
and `m.width`-aware padding — making the initializing state visually consistent
with every other TUI state.

**Independent Test**: Run `go test ./internal/tui/... -run TestOverviewView_Initializing`
and verify all 4 tests pass after implementation, with the width test having failed before it.

### Tests for User Story 1 (TDD — Write FIRST, Verify They FAIL Before Implementing) ⚠️

> **CONSTITUTION REQUIREMENT**: Write T002 and T003 before touching `overview_view.go`.
> T002 MUST fail with the current `fmt.Sprintf` implementation. T003 tests existing nil
> safety which already works; confirm it passes (it documents a guarantee, not new behaviour).

- [X] T002 [US1] Add `TestOverviewView_InitializingRender_UsesLipglossWidth` to
  `internal/tui/overview_view_test.go` — asserts that the longest line in the output
  is `>= model.width - borderPadding` (proves lipgloss Width() padding is applied);
  this test MUST fail before T004 is implemented

  ```go
  func TestOverviewView_InitializingRender_UsesLipglossWidth(t *testing.T) {
      ctx := context.Background()
      model, _ := NewOverviewModel(ctx, nil, 0)
      model.progressMsg = "test"

      output := model.renderInitializingView()
      lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
      maxLen := 0
      for _, line := range lines {
          if len(line) > maxLen {
              maxLen = len(line)
          }
      }
      assert.GreaterOrEqual(t, maxLen, model.width-borderPadding,
          "expected output to be padded to width by lipgloss")
  }
  ```

- [X] T003 [P] [US1] Add `TestOverviewView_InitializingRender_NilLoadingState` to
  `internal/tui/overview_view_test.go` — sets `m.loadingState = nil`, calls
  `renderInitializingView()` directly, and asserts no panic and "Initializing..." present
  (documents nil-safety guarantee explicitly; should PASS before implementation)

  ```go
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

- [X] T004 [P] [US1] Verify T002 fails: run
  `go test ./internal/tui/... -run TestOverviewView_InitializingRender_UsesLipglossWidth`
  and confirm it exits with a test failure — confirms TDD baseline is correct

### Implementation for User Story 1

- [X] T005 [US1] Replace the body of `renderInitializingView()` in
  `internal/tui/overview_view.go` (lines 35–44) with the lipgloss implementation:
  remove the `fmt.Sprintf(...)` return and replace with `lipgloss.JoinHorizontal` +
  `InfoStyle.Render(msg)` wrapped in `lipgloss.NewStyle().Width(m.width - borderPadding).Padding(1, 2).Render(content)`
  — the spinner/nil guard and message fallback logic remain unchanged

- [X] T006 [US1] Verify `make test` passes (`go test ./internal/tui/...`) — all 4 existing
  tests (`TestOverviewView_InitializingRender`, `TestOverviewView_InitializingDefaultMsg`,
  `TestOverviewView_ErrorStateRender`) plus both new tests (T002, T003) must pass

- [X] T007 [US1] Verify `make lint` passes with zero new violations — confirm
  `golangci-lint` is clean and the `fmt` import is still present (it is still used by other
  functions in the file)

**Checkpoint**: User Story 1 complete — `renderInitializingView()` now uses lipgloss styles,
all 5 tests pass, lint is clean.

---

## Phase 3: Polish & Cross-Cutting Concerns

**Purpose**: Final validation across the full test suite.

- [X] T008 [P] Run `make test` (full suite, not just tui package) to confirm no regressions
  in any other package
- [X] T009 Run `make lint` on the full codebase to confirm zero violations

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1** (T001): No dependencies — run immediately
- **Phase 2** (T002–T007): Depends on Phase 1 passing
  - T002 and T003 are the TDD tests; write both before T005
  - T004 confirms TDD baseline (T002 fails with current code)
  - T005 is the implementation; blocked by T002+T003 existing
  - T006 and T007 confirm correctness; blocked by T005
- **Phase 3** (T008, T009): Depends on Phase 2 completion

### Within User Story 1

```text
T002 (write failing test) ──┐
T003 (write nil test)    ──┤── T004 (verify T002 fails) ──→ T005 (implement) ──→ T006 (test) ──→ T007 (lint)
```

T002 and T003 can be written in parallel (same file, no functional dependency).

### Parallel Opportunities

- T002 and T003 can be written at the same time (both add new functions to the same file)
- T008 and T009 in Phase 3 can run in parallel

---

## Parallel Example: User Story 1

```bash
# Write both tests concurrently (same file, append operations):
# Developer A: TestOverviewView_InitializingRender_UsesLipglossWidth (T002)
# Developer B: TestOverviewView_InitializingRender_NilLoadingState (T003)

# Then sequentially:
go test ./internal/tui/... -run TestOverviewView_InitializingRender_UsesLipglossWidth
# → Must FAIL (T004 baseline check)

# Implement T005, then:
go test ./internal/tui/... -run TestOverviewView_Initializing
# → Must PASS (T006)
make lint  # → Must pass (T007)
```

---

## Implementation Strategy

### MVP (This Feature = One Story)

1. **T001**: Confirm baseline
2. **T002+T003**: Write tests (TDD)
3. **T004**: Confirm T002 fails (TDD baseline)
4. **T005**: Implement `renderInitializingView()` replacement
5. **T006+T007**: Validate
6. **T008+T009**: Final full-suite check

This feature is complete with a single delivery increment.

---

## Notes

- [P] tasks = different files or no execution-order dependency
- [US1] label maps tasks to User Story 1 for traceability
- The `fmt` import is NOT removed — it is still used by `renderStatusBar`,
  `renderDetailCostDrift`, `renderDetailRecommendations`, `renderDetailError`,
  and `renderBreakdown` in the same file
- `borderPadding = 2` is defined in `internal/tui/cost_view.go`; do not redefine it
- `defaultWidth = 80` means test models have `m.width = 80` unless overridden
