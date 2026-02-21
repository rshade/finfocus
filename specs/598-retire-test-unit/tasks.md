---

description: "Task list for: Retire test/unit/ and Fix Test Discovery"
---

# Tasks: Retire test/unit/ and Fix Test Discovery

**Input**: Design documents from `/specs/598-retire-test-unit/`
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ quickstart.md ✅

**Completeness**: Per Constitution Principle VI, all tasks MUST be fully implemented.
No partial migrations, no TODO comments in migrated files.

**Documentation**: Per Constitution Principle IV, `test/README.md` is updated concurrently
with the migration (US4 phase).

**Organization**: Tasks follow spec.md user story priorities (US1 P1 → US2 P2 → US3 P3 → US4 P4).
Migration is the mechanism that satisfies US1; US3 handles stale assertions and cleanup.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no shared state)
- **[Story]**: User story this task belongs to (US1–US4)
- **No [P] marker**: Must run sequentially (depends on prior task output)

## Path Conventions

- Unit tests: `internal/[package]/[name]_test.go` (colocated with source)
- Migration source: `test/unit/[package]/[name]_test.go`
- Template: `.specify/templates/tasks-template.md`
- Documentation: `test/README.md`

---

## Phase 1: Setup — Baseline Verification

**Purpose**: Establish pre-migration ground truth so regressions can be detected.

- [X] T001 Record pre-migration `make test` output (package list and pass/fail counts) to
  `specs/598-retire-test-unit/baseline.txt` for comparison after migration
- [X] T002 Confirm `go test ./test/unit/...` currently runs independently (not via `make test`)
  and note any files that fail to compile in `specs/598-retire-test-unit/baseline.txt`

---

## Phase 2: Foundational — Function-Level Audit (Blocking)

**Purpose**: Determine the exact merge plan for the 8 MERGE-classified files and 2 VERIFY
files before any migrations begin. MUST complete before Phase 3.

**⚠️ CRITICAL**: Migration tasks in Phase 3 for MERGE files depend on this audit output.

- [X] T003 [P] Audit `test/unit/cli/cost_actual_test.go` vs `internal/cli/cost_actual_test.go`:
  list each function in the test/unit/ file; mark as UNIQUE (not in internal/) or DUPLICATE
  (same name exists); record result in `specs/598-retire-test-unit/audit.md`
- [X] T004 [P] Audit `test/unit/cli/cost_projected_test.go` vs
  `internal/cli/cost_projected_test.go`: list each function; mark UNIQUE or DUPLICATE;
  record in `specs/598-retire-test-unit/audit.md`
- [X] T005 [P] Audit `test/unit/cli/plugin_test.go` vs all `internal/cli/plugin_*_test.go`
  files (`plugin_list_test.go`, `plugin_install_test.go`, `plugin_validate_test.go`, etc.):
  list each function; mark UNIQUE or DUPLICATE; record in `specs/598-retire-test-unit/audit.md`
- [X] T006 [P] Audit `test/unit/config/config_test.go` vs `internal/config/config_test.go`:
  list each function; mark UNIQUE or DUPLICATE; record in `specs/598-retire-test-unit/audit.md`
- [X] T007 [P] Audit `test/unit/engine/engine_test.go` vs `internal/engine/engine_test.go`:
  list each function; mark UNIQUE or DUPLICATE; record in `specs/598-retire-test-unit/audit.md`
- [X] T008 [P] Audit `test/unit/pluginhost/client_test.go` vs
  `internal/pluginhost/client_test.go`: list each function; mark UNIQUE or DUPLICATE;
  record in `specs/598-retire-test-unit/audit.md`
- [X] T009 [P] Audit `test/unit/ingest/plan_test.go` vs `internal/ingest/pulumi_plan_test.go`
  (different filenames — check function-level overlap): list each function; mark UNIQUE or
  DUPLICATE; determine final classification (MV or MERGE); record in
  `specs/598-retire-test-unit/audit.md`

**Checkpoint**: `audit.md` complete — every function in every MERGE/VERIFY file is classified.
Batch-A MV migrations and Batch-B MERGE migrations can now proceed.

---

## Phase 3: User Story 1 — make test Discovers All Orphaned Tests (Priority: P1) 🎯 MVP

**Goal**: Migrate all 34 `test/unit/` files into `internal/[package]/` so
`go test ./internal/... ./pkg/...` (`make test`) discovers every formerly-orphaned unit test.

**Independent Test**: After this phase, `make test` output includes test results from
`internal/cli/pagination`, `internal/engine/batch`, `internal/engine/cache`, and
`internal/tui/list` — packages that previously had zero coverage from `make test`.

### Batch A: Clean MV Migrations (24 files — no filename conflict at destination)

Each task: `git mv <source> <destination>` then immediately `go test -race ./internal/<pkg>/...`
to confirm the file compiles and is race-detector clean (FR-004). Fix import path errors
inline before moving to next package.

- [X] T010 [P] [US1] Migrate `cli/pagination/` test files (4 files): `git mv` each of
  `test/unit/cli/pagination/{edge_cases_test.go,flags_test.go,metadata_test.go,sorter_test.go}`
  to `internal/cli/pagination/`; run `go test -race ./internal/cli/pagination/...` to verify compile
- [X] T011 [P] [US1] Migrate `cli/` MV test files (4 files): `git mv` each of
  `test/unit/cli/{flags_test.go,output_test.go,plugin_install_fallback_test.go,prompt_test.go}`
  to `internal/cli/`; run `go test -race ./internal/cli/...` to verify compile
- [X] T012 [P] [US1] Migrate `config/` MV test files (3 files): `git mv` each of
  `test/unit/config/{budget_scoped_test.go,env_test.go,load_test.go}` to `internal/config/`;
  run `go test -race ./internal/config/...` to verify compile
- [X] T013 [P] [US1] Migrate `engine/` MV test files (2 files): `git mv` each of
  `test/unit/engine/{budget_scope_test.go,render_test.go}` to `internal/engine/`;
  run `go test -race ./internal/engine/...` to verify compile
- [X] T014 [P] [US1] Migrate `engine/batch/processor_test.go`: `git mv
  test/unit/engine/batch/processor_test.go internal/engine/batch/processor_test.go`;
  run `go test -race ./internal/engine/batch/...` to verify compile
- [X] T015 [P] [US1] Migrate `engine/cache/` test files (2 files): `git mv` each of
  `test/unit/engine/cache/{key_test.go,store_test.go}` to `internal/engine/cache/`;
  run `go test -race ./internal/engine/cache/...` to verify compile
- [X] T016 [P] [US1] Migrate `ingest/mapper_test.go`: `git mv
  test/unit/ingest/mapper_test.go internal/ingest/mapper_test.go`;
  run `go test -race ./internal/ingest/...` to verify compile
- [X] T017 [P] [US1] Migrate `pluginhost/` MV test files (2 files): `git mv` each of
  `test/unit/pluginhost/{discovery_test.go,lifecycle_test.go}` to `internal/pluginhost/`;
  run `go test -race ./internal/pluginhost/...` to verify compile
- [X] T018 [P] [US1] Migrate `registry/` test files (3 files): `git mv` each of
  `test/unit/registry/{fallback_test.go,manifest_test.go,scan_test.go}` to
  `internal/registry/`; run `go test -race ./internal/registry/...` to verify compile
- [X] T019 [P] [US1] Migrate `spec/` test files (3 files): `git mv` each of
  `test/unit/spec/{load_test.go,parse_test.go,spec_test.go}` to `internal/spec/`;
  run `go test -race ./internal/spec/...` to verify compile
- [X] T020 [P] [US1] Migrate `tui/list/` test files (2 files): `git mv` each of
  `test/unit/tui/list/{model_test.go,render_test.go}` to `internal/tui/list/`;
  run `go test -race ./internal/tui/list/...` to verify compile

- [X] T021 [US1] Run `go test -race ./internal/... ./pkg/...` (full suite) after all Batch-A
  tasks complete; fix any remaining import path errors or package declaration mismatches in
  migrated files (e.g., update `package cli_test` if file references internal symbols requiring
  `package cli`; default MUST remain `package foo_test` per spec Clarification 2026-02-20)

### Batch B: Function-Merge Migrations (8 files — destination exists with same name)

For each task: read `audit.md` (from Phase 2) to identify UNIQUE functions; copy unique
functions into the destination internal/ file; if no unique functions remain, `git rm` the
source file; if unique functions exist, append them to destination file then `git rm` source.
**Fix stale assertions inline**: if the verify command fails due to mismatched expected values
(error message text, output format, warning count), update those expected values to match
current production behaviour (FR-005) before marking the task complete — do not defer assertion
fixes to Phase 5.

- [X] T022 [US1] Merge `test/unit/cli/cost_actual_test.go` into `internal/cli/cost_actual_test.go`:
  add each UNIQUE function from audit.md to the end of the destination file; drop any DUPLICATE
  function; `git rm test/unit/cli/cost_actual_test.go`; run
  `go test ./internal/cli/... -run TestCostActual` to verify
- [X] T023 [US1] Merge `test/unit/cli/cost_projected_test.go` into
  `internal/cli/cost_projected_test.go`: add each UNIQUE function; drop DUPLICATES;
  `git rm test/unit/cli/cost_projected_test.go`; run
  `go test ./internal/cli/... -run TestCostProjected` to verify
- [X] T024 [US1] Merge `test/unit/cli/plugin_test.go` into the appropriate
  `internal/cli/plugin_*_test.go` file (determined from audit.md — pick the file whose
  package and subject area best matches): add UNIQUE functions; drop DUPLICATES;
  `git rm test/unit/cli/plugin_test.go`; run `go test ./internal/cli/... -run TestPlugin`
  to verify
- [X] T025 [US1] Merge `test/unit/config/config_test.go` into `internal/config/config_test.go`:
  add UNIQUE functions; drop DUPLICATES; `git rm test/unit/config/config_test.go`;
  run `go test ./internal/config/...` to verify
- [X] T026 [US1] Merge `test/unit/engine/engine_test.go` into `internal/engine/engine_test.go`:
  add UNIQUE functions; drop DUPLICATES; `git rm test/unit/engine/engine_test.go`;
  run `go test ./internal/engine/...` to verify
- [X] T027 [US1] Merge `test/unit/pluginhost/client_test.go` into
  `internal/pluginhost/client_test.go`: add UNIQUE functions; drop DUPLICATES;
  `git rm test/unit/pluginhost/client_test.go`; run `go test ./internal/pluginhost/...`
  to verify
- [X] T028 [US1] Process `test/unit/ingest/plan_test.go` per audit.md classification:
  if MV → `git mv test/unit/ingest/plan_test.go internal/ingest/plan_test.go`;
  if MERGE → add UNIQUE functions to `internal/ingest/pulumi_plan_test.go` then
  `git rm test/unit/ingest/plan_test.go`; run `go test ./internal/ingest/...` to verify

- [X] T029 [US1] Run `make test` (full suite) after all Batch-B merges and inline assertion
  fixes; confirm output shows test results from previously-uncovered packages; confirm zero
  test failures — all stale assertions must be resolved here, not deferred to Phase 5

**Checkpoint**: US1 acceptance criteria met — `make test` discovers and runs all formerly-orphaned
tests. Every package that had tests only in `test/unit/` now shows in `go test ./internal/...`
output.

---

## Phase 4: User Story 2 — speckit.tasks Generates Correct Test Paths (Priority: P2)

**Note**: US2 (template fix) is fully independent of US1 migration — both touch separate
files. Placement after Phase 3 is a delivery convenience; T030–T031 can run concurrently
with any Phase 3 task if working in parallel. The spec's ordering guidance (template fix
before migration) is satisfied in practice because both are independent; the implementation
order does not affect correctness.

**Goal**: Update `.specify/templates/tasks-template.md` so future task generation never
again directs tests to `test/unit/` or `tests/unit/`.

**Independent Test**: After this phase, `grep -n "tests/unit\|test/unit"
.specify/templates/tasks-template.md` returns no output.

- [X] T030 [P] [US2] Update `.specify/templates/tasks-template.md` to satisfy FR-001 and FR-002.
  **FR-001 (line 160)**: replace the Polish phase line ending in `in tests/unit/` so that
  it ends with: colocated in `internal/[package]/[feature]_test.go`.
  **FR-002 (new section)**: immediately after the `## Path Conventions` section, insert a
  `## Go Test Path Conventions` section that documents three test locations — unit tests
  colocated with source in `internal/`, integration tests in `test/integration/`, and E2E
  tests in `test/e2e/` — and includes an explicit retirement notice for `test/unit/`
- [X] T031 [US2] Verify the template fix: run
  `grep -n "tests/unit\|test/unit" .specify/templates/tasks-template.md`
  and confirm zero matches; run `markdownlint .specify/templates/tasks-template.md` to
  confirm no lint errors introduced

**Checkpoint**: US2 acceptance criteria met — generated task lists will reference
`internal/[package]/` paths only.

---

## Phase 5: User Story 3 — Full Cleanup and Stale Assertion Fixes (Priority: P3)

**Goal**: Fix any broken test assertions in migrated files, confirm race-detector clean,
and permanently remove the `test/unit/` directory.

**Independent Test**: After this phase, `find . -path ./vendor -prune -o -path ./test/unit
-print | grep test/unit` returns no output, and `make test-race` passes with zero failures.

- [X] T032 [US3] Final assertion check: run `make test` and confirm zero failures remain;
  stale assertions from Batch-B migrations were fixed inline during Phase 3 (T022–T028
  preamble and T029); if any new failures appear here, fix the expected value only (FR-005)
  — do NOT change test intent — and re-run `make test` to confirm
- [X] T033 [US3] Run `make test-race` (`go test -v -race ./internal/... ./pkg/...`); fix
  any data-race errors introduced by migrated tests (race errors indicate the test was
  previously hiding a real concurrency bug — fix the test setup, e.g., add `t.Parallel()`
  guards or isolate shared state)
- [X] T034 [US3] Remove `test/unit/` directory: run `git rm -r test/unit/` to stage all
  remaining files for deletion (any files still present at this point are ones that were
  not migrated — investigate before deleting); verify with
  `find . -path ./test/unit -prune -o -print | grep test/unit` returning no output
- [X] T035 [US3] Run `make test` and compare output against `specs/598-retire-test-unit/baseline.txt`
  (from T001): confirm total test function count equals or exceeds baseline; confirm zero
  packages report fewer tests than before; confirm CI coverage percentage (from
  `go test -coverprofile=coverage.out ./internal/... ./pkg/... && go tool cover -func=coverage.out | grep total`)
  is ≥ 61%

**Checkpoint**: US3 acceptance criteria met — `test/unit/` no longer exists, all migrated
tests pass with the race detector, and coverage has not regressed.

---

## Phase 6: User Story 4 — test/README.md Reflects the Retired Convention (Priority: P4)

**Goal**: Update `test/README.md` so new contributors and AI agents learn the colocated
unit test convention and are not directed to `test/unit/`.

**Independent Test**: After this phase,
`grep "go test ./test/unit" test/README.md` returns no output and
`grep "./internal/..." test/README.md` returns at least one match.

- [X] T036 [US4] Rewrite the `### Unit Tests (/test/unit/)` section in `test/README.md`:
  replace section heading with `### Unit Tests (colocated with source)`;
  update description to explain that unit tests live in `internal/[package]/[name]_test.go`
  and are run with `go test ./internal/...`; add a one-sentence historical note:
  `> The test/unit/ directory was retired; see issue #732 for details.`;
  replace all `go test ./test/unit/...` example commands with `go test ./internal/...`
  equivalents
- [X] T037 [P] [US4] Update the directory tree in `test/README.md` to remove the `unit/`
  subtree entry and reflect the actual current structure:
  `test/integration/`, `test/e2e/`, `test/fixtures/`, `test/mocks/`, `test/benchmarks/`
- [X] T038 [US4] Remove the `### Unit Tests` command examples block from the
  `## Test Categories and Commands` section that references `./test/unit/...` paths;
  replace with `go test ./internal/...` for the general unit test command; verify with
  `grep "go test ./test/unit" test/README.md` returning no output
- [X] T039 [US4] Run `markdownlint test/README.md` and fix any markdown lint errors
  introduced by the README edits

**Checkpoint**: US4 acceptance criteria met — `test/README.md` contains no active
`test/unit/` commands; the colocated unit test pattern is documented as authoritative.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, CLAUDE.md audit, lint, coverage, and full acceptance check.

- [X] T040 Audit all `CLAUDE.md` files for `test/unit/` references: run
  `grep -rn "test/unit" CLAUDE.md internal/*/CLAUDE.md .specify/` and update any found
  references to use `internal/[package]/` paths; run `markdownlint` on any modified files
- [X] T041 Run `make lint` on the full repository; fix all golangci-lint and markdownlint
  errors in modified files (`.specify/templates/tasks-template.md`, `test/README.md`,
  any updated `CLAUDE.md` files, and all migrated `_test.go` files)
- [X] T042 Run `go test -coverprofile=coverage.out ./internal/... ./pkg/...` and verify
  `go tool cover -func=coverage.out | grep total` reports ≥ 61%; if below threshold,
  investigate which migrated tests have stale or uncompilable assertions still in
  `internal/` packages and fix them
- [X] T043 Execute all verification steps from `specs/598-retire-test-unit/quickstart.md`
  in order; confirm each step produces the expected output; confirm SC-001 through SC-006
  from spec.md are all satisfied; additionally verify FR-007 by running
  `grep -rn 'test/unit' Makefile .github/` and confirming zero matches — no reference to
  `test/unit/` in build or CI scripts

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: No dependencies on Phase 1 output — can start immediately;
  BLOCKS Phase 3 MERGE tasks (Batch-B T022–T028)
- **US1 Batch-A (T010–T020)**: No dependency on audit (MV files have no destination conflict)
  — can start immediately after Phase 1
- **US1 Batch-B (T022–T028)**: Blocked by Phase 2 audit (T003–T009)
- **US2 (Phase 4)**: No dependencies — completely independent, can run in parallel with
  any Phase 3 task since it touches a different file
- **US3 (Phase 5)**: Depends on Phase 3 completion (all 34 files processed)
- **US4 (Phase 6)**: Depends on Phase 5 completion (test/unit/ removed first)
- **Polish (Phase 7)**: Depends on all prior phases complete

### User Story Dependencies

- **US1 (P1)**: Can start after baseline (Phase 1) — no story dependencies
- **US2 (P2)**: Independent — can run in parallel with US1 (touches different files)
- **US3 (P3)**: Depends on US1 complete (tests must be in internal/ before cleanup)
- **US4 (P4)**: Depends on US3 complete (test/unit/ must be gone before README finalizes)

### Within Phase 3

- T010–T020 (Batch-A): All [P] — different packages, no shared state; safe to run in
  parallel; execute per-package git operations sequentially within a package to avoid
  staging conflicts
- T022–T028 (Batch-B): Run sequentially — each reads audit.md then modifies a specific file
- T021 (Batch-A verify) must run after T010–T020 all complete
- T029 (Batch-B verify) must run after T022–T028 all complete

---

## Parallel Example: US1 Batch-A

All 11 Batch-A migration tasks (T010–T020) touch different packages and can run
concurrently on separate worktrees or sequentially in rapid succession:

```bash
# Launch all Batch-A migrations in parallel (different packages):
Task: "git mv 4 cli/pagination/ files to internal/cli/pagination/"        # T010
Task: "git mv 4 cli/ MV files to internal/cli/"                          # T011
Task: "git mv 3 config/ MV files to internal/config/"                    # T012
Task: "git mv 2 engine/ MV files to internal/engine/"                    # T013
Task: "git mv engine/batch/processor_test.go"                             # T014
Task: "git mv 2 engine/cache/ files to internal/engine/cache/"           # T015
Task: "git mv ingest/mapper_test.go"                                      # T016
Task: "git mv 2 pluginhost/ MV files to internal/pluginhost/"            # T017
Task: "git mv 3 registry/ MV files to internal/registry/"                # T018
Task: "git mv 3 spec/ MV files to internal/spec/"                        # T019
Task: "git mv 2 tui/list/ files to internal/tui/list/"                   # T020
```

## Parallel Example: Phase 2 Audit + US2 Template Fix

```bash
# Audit tasks (T003–T009) and US2 template fix (T030) have zero shared files:
Task: "Audit cli/cost_actual_test.go functions"                           # T003
Task: "Audit cli/cost_projected_test.go functions"                        # T004
Task: "Audit cli/plugin_test.go functions"                                # T005
Task: "Audit config/config_test.go functions"                             # T006
Task: "Audit engine/engine_test.go functions"                             # T007
Task: "Audit pluginhost/client_test.go functions"                         # T008
Task: "Audit ingest/plan_test.go functions"                               # T009
Task: "Fix tasks-template.md line 160"                                    # T030
```

---

## Implementation Strategy

### MVP: US1 Only (make test discovers all tests)

1. Complete Phase 1: Setup (T001–T002)
2. Complete Phase 2: Audit (T003–T009)
3. Complete Batch-A MV migrations (T010–T021)
4. Complete Batch-B MERGE migrations (T022–T029)
5. **STOP and VALIDATE**: `make test` output includes formerly-orphaned packages
6. This alone satisfies US1 acceptance criteria SC-001

### Full Delivery (all four user stories)

1. MVP steps above → US1 done
2. Template fix (T030–T031) → US2 done (can run during MVP if parallelized)
3. Stale fixes + cleanup + directory removal (T032–T035) → US3 done
4. README update (T036–T039) → US4 done
5. Polish (T040–T043) → all SC-001–SC-006 confirmed

---

## Notes

- `[P]` tasks touch different files — safe to parallelize within the same phase
- Each `git mv` is immediately followed by a targeted `go test ./internal/<pkg>/...` verify
- Default package declaration for migrated files: `package foo_test` (black-box);
  only use `package foo` if the test requires unexported symbol access (spec Clarification)
- All migrations use `git mv` (history-preserving); never delete-and-recreate unless a
  MERGE operation requires it (spec FR-011 and Clarification)
- Stale assertion fixes (T032) update ONLY expected values — never test intent
- `test/mocks/` import paths are valid after migration — that directory is not moved
- The Makefile does NOT need changes — `go test ./internal/... ./pkg/...` already covers
  the destination paths
