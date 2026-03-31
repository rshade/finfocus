# Tasks: State-Only Flag for Overview Command

**Input**: Design documents from `/specs/607-state-only-flag/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths). TUI changes
MUST include golden file snapshot tests and visual render verification.

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Flag Registration)

**Purpose**: Add the `stateOnly` field and register the `--state-only` flag so all downstream tasks can reference it.

- [x] T001 Add `stateOnly bool` field to `overviewParams` struct, register `--state-only` flag via `cmd.Flags().BoolVar(&params.stateOnly, "state-only", false, "skip pulumi preview (faster, but won't detect pending changes)")`, declare `cmd.MarkFlagsMutuallyExclusive("state-only", "pulumi-json")`, and add `--state-only` example to the `Example` string in `NewOverviewCmd()` in `internal/cli/overview.go`

**Checkpoint**: Flag exists and is parseable. `--help` shows the flag. `--state-only --pulumi-json` errors at the Cobra level.

---

## Phase 2: User Story 1 - Fast Cost-Only Overview (Priority: P1) MVP

**Goal**: When `--state-only` is set, skip `pulumi preview` entirely in both TUI and plain text paths, reducing overview time from ~18s to ~3s.

**Independent Test**: Run `finfocus overview --state-only` in a Pulumi project and verify cost data appears without preview subprocess.

### Tests for User Story 1

- [x] T002 [P] [US1] Add table-driven test cases to `TestResolveIsStateOnly` for `params.stateOnly=true`: (1) stateOnly=true with no signal/no error returns true, (2) stateOnly=true with HasLikelyChanges=true returns true, (3) stateOnly=true with yes=true returns true — verifying the flag overrides all other inputs in `internal/cli/overview_phase_internal_test.go`
- [x] T003 [P] [US1] Add `assert.Contains(t, output, "--state-only")` assertion to existing `TestNewOverviewCmd_HelpFlag` in `internal/cli/overview_test.go`

### Implementation for User Story 1

- [x] T004 [US1] Add early return `if params.stateOnly { return true }` at top of `resolveIsStateOnly()` before existing logic in `internal/cli/overview.go` — this handles the TUI/interactive path via `overviewInitAndEnrich()`
- [x] T005 [US1] Add early return in `loadPlainOverviewData()` when `params.stateOnly=true`: after the explicit-files check (`if params.pulumiState != "" || params.pulumiJSON != ""`), add a `stateOnly` guard that loads state via `loadStateForOverview(ctx, params, nil)` and returns `(stateResources, nil, stackName, true, nil)` — skipping change detection, preview prompt, and preview execution in `internal/cli/overview.go`

**Checkpoint**: `resolveIsStateOnly()` returns true when flag is set. Plain text path skips preview. All US1 tests pass.

---

## Phase 3: User Story 2 - Flag Conflict Validation (Priority: P2)

**Goal**: Return a clear error when `--state-only` and `--pulumi-json` are both provided; allow `--state-only` with `--pulumi-state`.

**Independent Test**: Run `finfocus overview --state-only --pulumi-json plan.json` and verify error message.

### Tests for User Story 2

- [x] T006 [P] [US2] Add `TestNewOverviewCmd_StateOnlyMutualExclusion` test: create `NewOverviewCmd()`, set args `["--state-only", "--pulumi-json", "plan.json", "--yes"]`, execute, assert error contains both "state-only" and "pulumi-json" in `internal/cli/overview_test.go`
- [x] T007 [P] [US2] Add `TestNewOverviewCmd_StateOnlyWithPulumiState` test: create `NewOverviewCmd()`, set args `["--state-only", "--pulumi-state", "/nonexistent/state.json", "--yes"]`, execute, assert error does NOT contain "mutually exclusive" (error should be about file not found, not flag conflict) in `internal/cli/overview_test.go`

**Checkpoint**: Mutual exclusion validated at Cobra level. Compatible combinations proceed to execution. All US2 tests pass.

---

## Phase 4: User Story 3 - Interactive TUI With State-Only (Priority: P3)

**Goal**: When `--state-only` is set in TUI mode, skip the `pulumidetect.DetectChanges()` call (it's unnecessary when the user has explicitly requested state-only) and proceed directly to state-only row building with on-demand preview available.

**Independent Test**: Verify `overviewInitAndEnrich()` skips change detection when `stateOnly=true`.

### Implementation for User Story 3

- [x] T008 [US3] Add guard in `overviewInitAndEnrich()` to skip `pulumidetect.DetectChanges()` when `params.stateOnly` is true: before the "Detecting changes..." phase, check the flag and if set, log `"--state-only: skipping change detection"` at Info level, set `isStateOnly = true` directly, and skip to the merge phase in `internal/cli/overview.go`

**Checkpoint**: TUI path skips change detection when flag is set. On-demand preview (`p` key) remains available via existing `OverviewSetStateOnlyMsg` path.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Documentation updates and quality gate verification.

- [x] T009 [P] Add `--state-only` row to Options table (`| \`--state-only\` | Skip pulumi preview (faster, no pending change detection) | false |`) and add a "Cost-only overview (skip preview)" example section in `docs/commands/overview.md`
- [x] T010 Run `make test` and `make lint` to verify all quality gates pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies — start immediately
- **User Story 1 (Phase 2)**: Depends on Phase 1 (flag must exist)
- **User Story 2 (Phase 3)**: Depends on Phase 1 (mutual exclusion must be declared)
- **User Story 3 (Phase 4)**: Depends on Phase 1 (flag must exist in params)
- **Polish (Phase 5)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 1 — no dependencies on other stories
- **User Story 2 (P2)**: Can start after Phase 1 — independent of US1 (uses Cobra-level validation)
- **User Story 3 (P3)**: Can start after Phase 1 — independent of US1/US2 (different code path)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation makes tests PASS
- All assertions use testify (`require`/`assert`)

### Parallel Opportunities

- T002 and T003 can run in parallel (different test files)
- T006 and T007 can run in parallel (independent test functions, same file)
- T009 is independent of all implementation tasks (docs only)
- US1, US2, and US3 can start in parallel after Phase 1 (different code paths)

---

## Parallel Example: User Story 1

```text
# Launch tests in parallel (different files):
Task T002: "Add resolveIsStateOnly test cases in overview_phase_internal_test.go"
Task T003: "Add --state-only help text assertion in overview_test.go"

# Then implement sequentially (same file):
Task T004: "Short-circuit resolveIsStateOnly()"
Task T005: "Short-circuit loadPlainOverviewData()"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational (T001)
2. Complete Phase 2: User Story 1 (T002-T005)
3. **STOP and VALIDATE**: `make test` passes, `--state-only` works in plain text mode
4. This alone delivers the ~15s performance improvement

### Incremental Delivery

1. T001 → Flag registered
2. T002-T005 → US1 complete (fast overview works)
3. T006-T007 → US2 complete (flag conflicts validated)
4. T008 → US3 complete (TUI change detection skipped)
5. T009-T010 → Docs updated, quality gates verified

---

## Notes

- All implementation is in a single file (`internal/cli/overview.go`) — [P] markers apply only to test tasks in different files
- No new files created — all changes are edits to existing files
- No TUI view changes → no golden file updates needed
- The existing `isStateOnly` downstream path handles everything after the decision point
