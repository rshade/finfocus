# Feature Specification: Retire test/unit/ and Fix Test Discovery

**Feature Branch**: `598-retire-test-unit`
**Created**: 2026-02-20
**Status**: Draft
**Input**: Issue #732 — fix(speckit): tasks-template.md directs tests to test/unit/ causing CI-invisible dead tests

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Developer runs `make test` and all unit tests execute (Priority: P1)

A developer makes a change to `internal/cli/` and runs `make test`. Every unit test
in the repository — including tests that were previously hidden in `test/unit/` —
runs and either passes or fails visibly. No tests silently drift from the
implementation they are supposed to verify.

**Why this priority**: This is the core defect. Tests that are never run provide
zero value and create false confidence. Fixing CI visibility is the highest-impact
change and a prerequisite for everything else.

**Independent Test**: Running `make test` on the merged branch produces a result that
includes coverage from packages previously covered only in `test/unit/` (e.g., cache
key generation, pagination sorter, TUI list model). Any regression caught by those
tests is now surfaced immediately.

**Acceptance Scenarios**:

1. **Given** a clean checkout of the merged branch, **When** `make test` runs,
   **Then** all 34 formerly-orphaned test files contribute to coverage output and
   any failure stops the build.
2. **Given** a developer edits `internal/engine/cache/key.go`, **When** they run
   `make test`, **Then** the test in `internal/engine/cache/key_test.go` (migrated
   from `test/unit/engine/cache/key_test.go`) catches the regression.
3. **Given** the Makefile and CI config, **When** a reviewer searches for `test/unit/`,
   **Then** `grep -rn 'test/unit' Makefile .github/` returns no output — no reference
   to `test/unit/` remains in build or CI scripts after migration.

---

### User Story 2 — Developer generates a new spec with `/speckit.tasks` and tests land in `internal/` (Priority: P2)

A developer runs `/speckit.tasks` for a new feature. The generated `tasks.md` directs
unit tests to `internal/[package]/[feature]_test.go`, not to `test/unit/` or
`tests/unit/`. The tasks are immediately runnable by `make test` without any
additional Makefile changes.

**Why this priority**: Without fixing the template the problem recurs for every future
spec. This is the upstream fix that prevents re-accumulation of dead tests.

**Independent Test**: Generate a tasks list for a trivial feature after the template is
updated. Inspect the generated `tasks.md`. Every unit test task references
`internal/...` paths. No mention of `test/unit/` or `tests/unit/` appears.

**Acceptance Scenarios**:

1. **Given** the updated `.specify/templates/tasks-template.md`, **When**
   `/speckit.tasks` runs for any feature, **Then** the Polish phase task for unit
   tests reads: "Additional unit tests colocated in `internal/[package]/[feature]_test.go`".
2. **Given** the updated template, **When** a developer follows the generated tasks,
   **Then** running `make test` after writing the tests verifies them without any
   extra configuration.

---

### User Story 3 — Duplicate tests are removed, unique tests are migrated and fixed (Priority: P3)

All 34 files under `test/unit/` are either deleted (if they duplicate coverage already
in `internal/`) or migrated to `internal/[package]/` (if they provide unique coverage).
Before migration, broken assertions are updated to match current implementation
behaviour. After migration, the `test/unit/` directory no longer exists.

**Why this priority**: Migration and cleanup is necessary to complete the retirement,
but it depends on the Makefile and template fixes (P1 and P2) being in place first.
The migration itself is safe to do incrementally package by package.

**Independent Test**: After migration, running `find test/unit -name "*.go"` returns
nothing. Running `make test` shows no decrease in total test count vs. the count of
unique tests that existed in `test/unit/`.

**Acceptance Scenarios**:

1. **Given** files identified as duplicates (`cost_projected_test.go`,
   `cost_actual_test.go`, `plugin_test.go` in `test/unit/cli/`), **When** the
   migration completes, **Then** those files are deleted and `internal/cli/` tests
   cover the same commands.
2. **Given** files with stale assertions (`budget_scoped_test.go`, broken error-message
   tests), **When** the migration completes, **Then** assertions match current
   production behaviour and `make test` passes with `-race`.
3. **Given** unique files (`flags_test.go`, `output_test.go`, `prompt_test.go`,
   `pagination/`, `engine/cache/`, `tui/list/`, `registry/`, `spec/`, `ingest/`,
   `pluginhost/`), **When** the migration completes, **Then** each lives in the
   corresponding `internal/[package]/` directory and `make test` runs it.

---

### User Story 4 — `test/README.md` reflects the retired convention (Priority: P4)

A new contributor reads `test/README.md` to understand where tests live. The README
explains that unit tests are colocated with source code, integration tests live in
`test/integration/`, and E2E tests live in `test/e2e/`. There is no mention of
`test/unit/` as an active location.

**Why this priority**: Documentation reinforces the pattern for humans and AI agents
alike. Without updating it, the old convention lives on in written form even after
the directory is deleted.

**Independent Test**: After the change, `grep -r "test/unit" test/README.md` returns
only historical context (if any), not active instructions. All example commands in
the README use `./internal/...` paths.

**Acceptance Scenarios**:

1. **Given** the updated `test/README.md`, **When** a contributor follows the documented
   test commands, **Then** every command they run is valid and produces output.
2. **Given** the updated README, **When** an AI agent reads it for project context,
   **Then** it will not suggest creating files under `test/unit/`.

---

### Edge Cases

- A test in `test/unit/` imports a package that does not exist in `internal/` (e.g.,
  a helper only in `test/unit/`). The helper must be moved alongside the test or
  inlined before deletion of the source directory.
- A unique test in `test/unit/` uses a `package` declaration that conflicts with the
  target `internal/` package (white-box vs. black-box). The **default package declaration
  for all migrated tests is `package foo_test` (black-box)**. Switch to `package foo`
  (white-box) only when the test explicitly requires access to unexported symbols that
  cannot be exposed via the public API.
- After migration, total `make test` coverage percentage may change because previously
  uncounted tests now count. This is expected and desirable; coverage thresholds in
  CI must not regress below current thresholds.
- The `test/unit/engine/batch/processor_test.go` file tests a batch processor; verify
  that `internal/engine/batch/` exists before migrating.
- `plugins/recorder/*_test.go` files were noted with stub methods. These are in
  `plugins/recorder/`, not `test/unit/`, and are out of scope for migration but should
  be audited separately.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `.specify/templates/tasks-template.md` Polish phase task for unit
  tests MUST reference `internal/[package]/[feature]_test.go` as the target path,
  not `test/unit/` or `tests/unit/`.
- **FR-002**: The tasks template MUST include a Go-specific path conventions section
  that explicitly states `test/unit/` is retired and defines when to use
  `internal/`, `test/integration/`, and `test/e2e/`.
- **FR-003**: All 34 files under `test/unit/` MUST be either deleted (duplicates) or
  migrated to the corresponding `internal/[package]/` directory (unique tests).
- **FR-004**: All migrated tests MUST compile and pass `make test -race` before the
  migration of that file is considered complete.
- **FR-005**: Stale assertions in migrated tests MUST be updated to match current
  production behaviour (error messages, output formats, warning counts).
- **FR-006**: The `test/unit/` directory MUST be removed entirely from the repository
  after all files are processed.
- **FR-007**: The Makefile `test` target MUST NOT contain any comment claiming
  `test/unit/` is excluded, and MUST NOT reference `test/unit/` in any test command.
- **FR-008**: `test/README.md` MUST document the colocated unit test pattern as
  authoritative, replace all `go test ./test/unit/...` example commands with
  `go test ./internal/...`, and may retain a historical note explaining the
  retirement.
- **FR-009**: After migration, `make test` and `make test-race` MUST run all unit
  tests in a single invocation with no orphaned tests remaining outside the covered
  paths.
- **FR-010**: The CI pipeline MUST continue to pass with the migrated tests included,
  and the coverage threshold MUST NOT regress below the current minimum.
- **FR-011**: Each test file migration MUST use `git mv` (or equivalent history-preserving
  rename) so that `git log --follow` and `git blame` remain traceable from the new
  `internal/[package]/` path back to the original `test/unit/` source. **Exception**: For
  files where the destination already exists (MERGE classification), unique functions are
  copied into the existing file and the source is removed with `git rm`; the merge commit
  itself serves as the provenance record in lieu of `git mv`.

### Key Entities

- **tasks-template.md**: The speckit template that generates task lists for new
  features. Contains a Polish phase line that must be updated to reference Go-idiomatic
  test paths.
- **test/unit/**: The directory being retired. Contains 34 Go test files across 11
  package subdirectories (cli, cli/pagination, config, engine, engine/batch, engine/cache,
  ingest, pluginhost, registry, spec, tui/list). Must be fully emptied and removed.
- **internal/[package]/**: The canonical location for Go unit tests in this project.
  Tests in this location are automatically discovered by `go test ./internal/...`.
- **test/README.md**: Developer-facing documentation that currently documents
  `test/unit/` as authoritative. Must be updated to reflect the retired convention.
- **Makefile**: Defines the `test` and `test-race` targets. Currently excludes
  `test/unit/` with an inaccurate "environment-dependent" comment.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After the change, `make test` produces output that includes test results
  from every package that previously had unique coverage only in `test/unit/` — zero
  test files remain orphaned outside CI-covered paths.
- **SC-002**: Running `find . -path ./test/unit -prune -o -print | grep test/unit`
  returns no results, confirming the directory is removed.
- **SC-003**: A tasks list generated by `/speckit.tasks` for any new feature contains
  zero references to `test/unit/` or `tests/unit/` in unit test task descriptions.
- **SC-004**: `make test-race` passes with no data-race errors introduced by migrated
  tests, and the total test pass count equals or exceeds the count prior to migration
  (accounting for duplicate removal).
- **SC-005**: The CI coverage percentage after migration is equal to or greater than
  the coverage percentage before migration (previously uncounted tests may increase
  coverage).
- **SC-006**: `test/README.md` contains no `go test ./test/unit/...` commands; all
  unit test example commands reference `./internal/...`.

## Assumptions

- The 3-category classification in the issue (duplicates, unique, broken-unique) is
  accurate. Before deleting any file, a code-level comparison will confirm the
  classification. If a "duplicate" file contains even one unique test function, it
  is reclassified as unique and migrated.
- The `internal/engine/batch/` package directory already exists (referenced by
  `test/unit/engine/batch/processor_test.go`). If it does not, the test's target
  package must be identified from its import declarations before migration.
- The CI coverage minimum (currently 61% per `ci.yml`) will not regress after
  migration. If it does, the PR must not merge until coverage is restored.
- Tests in `plugins/recorder/` are out of scope for this feature; they are in a
  separate directory and tracked by a separate issue.
- The `.specify/templates/tasks-template.md` file is the only template that contains
  the `tests/unit/` path typo. Other templates (spec-template, plan-template) do not
  require path convention updates.

## Clarifications

### Session 2026-02-20

- Q: When migrating a test file, what should the default package declaration be when there is no explicit requirement for unexported symbol access? → A: `package foo_test` (black-box); switch to `package foo` only when unexported access is genuinely required.
- Q: Should test file migrations preserve git history (git mv) or use delete-and-recreate? → A: `git mv` (history-preserving); keeps `git log --follow` and `git blame` traceable from new path.
