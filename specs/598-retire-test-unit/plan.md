# Implementation Plan: Retire test/unit/ and Fix Test Discovery

**Branch**: `598-retire-test-unit` | **Date**: 2026-02-20 | **Spec**: `specs/598-retire-test-unit/spec.md`
**Input**: Feature specification from `specs/598-retire-test-unit/spec.md`

## Summary

Retire the `test/unit/` directory by migrating all 34 orphaned Go test files into
their canonical `internal/[package]/` locations, fix the speckit tasks template to
stop generating `test/unit/` paths, and update `test/README.md` to document the
colocated unit test convention. After migration, `make test` automatically discovers
all unit tests via the existing `go test ./internal/... ./pkg/...` target — no
Makefile changes required.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `github.com/stretchr/testify` (assertions, already a dep)
**Storage**: N/A — file system only (source migration, no new storage)
**Testing**: `go test ./internal/... ./pkg/...` via `make test` and `make test-race`
**Target Platform**: Linux, macOS, Windows (cross-platform Go test tooling)
**Project Type**: Single Go module (`github.com/rshade/finfocus`)
**Performance Goals**: `make test` runtime increase < 30 s (migrated tests were always there, just hidden)
**Constraints**: CI coverage threshold ≥ 61% must not regress; `make lint` must pass
**Scale/Scope**: 34 test files across 10 package subdirectories

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Not applicable — this is a test file reorganization,
  not a new feature or cost data source. No plugin changes.
- [x] **Test-Driven Development**: The migrated tests ARE the test suite. All migrated
  tests must compile and pass `make test -race` before migration of that file is marked
  complete (FR-004).
- [x] **Cross-Platform Compatibility**: `go test` and `git mv` work on all supported
  platforms. No platform-specific code introduced.
- [x] **Documentation Integrity**: `test/README.md` is planned for update (US4).
  `CLAUDE.md` references to `test/unit/` will be audited as part of the plan.
- [x] **Protocol Stability**: No protocol buffer changes.
- [x] **Implementation Completeness**: All 34 files must be fully processed. No partial
  migration is acceptable. No TODO comments in migrated test files.
- [x] **Quality Gates**: `make lint` and `make test` must pass after each migration batch.
- [x] **Multi-Repo Coordination**: Single-repo change only. No cross-repo dependencies.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/598-retire-test-unit/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions and findings
├── data-model.md        # Phase 1 output — 34-file inventory and classification
├── quickstart.md        # Phase 1 output — verification guide
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (affected paths)

```text
# Files being REMOVED (after migration):
test/unit/                           # Entire directory deleted after all files processed
├── cli/                             # 11 files → internal/cli/ and internal/cli/pagination/
├── config/                          # 4 files → internal/config/
├── engine/                          # 3 files → internal/engine/
│   ├── batch/                       # 1 file → internal/engine/batch/
│   └── cache/                       # 2 files → internal/engine/cache/
├── ingest/                          # 2 files → internal/ingest/
├── pluginhost/                      # 3 files → internal/pluginhost/
├── registry/                        # 3 files → internal/registry/
├── spec/                            # 3 files → internal/spec/
└── tui/list/                        # 2 files → internal/tui/list/

# Files being MODIFIED (not moved):
.specify/templates/tasks-template.md # Line 160: fix test path reference
test/README.md                       # Replace test/unit/ commands with internal/ commands
CLAUDE.md                            # Audit for test/unit/ references; update if found
```

**Structure Decision**: Go single-module layout. All unit tests colocated with source
in `internal/[package]/`. Existing `make test` target unchanged.

## Complexity Tracking

No constitution violations. Section left empty per template instructions.

## Phase 0: Research

*Status: COMPLETE — see `research.md`*

**Key findings**:

1. Makefile is already correct (`./internal/...`); no Makefile changes needed.
2. File-name-match does not imply function-level duplication. Most "duplicate" files
   contain different test functions — they will be MERGED, not deleted.
3. 24 of 34 files are clean MV candidates (no filename conflict in destination).
4. 8 files require per-function merge into existing internal/ files.
5. 2 files need deeper verification before classification.
6. All sampled test/unit/ files already use `package foo_test` convention.
7. `internal/engine/batch/` exists; `processor_test.go` can be clean-moved.
8. Migration will likely increase coverage (previously uncounted tests now count).

**Resolved unknowns**: None remain. All NEEDS CLARIFICATION items resolved.

## Phase 1: Design & Contracts

### Migration Strategy

Three migration modes (from data-model.md):

**Mode MV (24 files)** — Clean `git mv`:

```bash
# Example: clean move where no same-named file exists in destination
git mv test/unit/engine/cache/key_test.go internal/engine/cache/key_test.go
go test ./internal/engine/cache/...   # Verify immediately
```

**Mode MERGE (8 files)** — Function-level merge:

```text
1. Compare functions in test/unit/ file vs internal/ file
2. For each function in test/unit/ file:
   a. If same function name exists in internal/ → compare; keep better version
   b. If function is unique → copy into internal/ file
3. git rm test/unit/ source file
4. Run make test to verify no regression
```

**Mode VERIFY (2 files)** — Deepen comparison first, then apply MV or MERGE:

```text
- ingest/plan_test.go vs internal/ingest/pulumi_plan_test.go (different names, possible overlap)
- Check if any test function name appears in both files → MERGE; else → MV
```

### Template Fix (US2 — Lowest Risk, Do First)

File: `.specify/templates/tasks-template.md`, line 160

Current:

```text
- [ ] TXXX [P] Additional unit tests (ensure 80% coverage minimum) in tests/unit/
```

Required:

```text
- [ ] TXXX [P] Additional unit tests (ensure 80% coverage minimum) colocated in `internal/[package]/[feature]_test.go`
```

### test/README.md Update (US4 — Do Last)

Replace the `### Unit Tests (/test/unit/)` section and all `go test ./test/unit/...`
commands with equivalent `./internal/...` commands. Retain a historical note:

```markdown
> **Note**: The `test/unit/` directory was retired. Unit tests are now colocated
> with source code in `internal/[package]/`. See issue #732 for details.
```

Remove from the running commands section:

```bash
# Remove these:
go test ./test/unit/...
go test ./test/unit/engine/...
go test ./test/unit/config/...
go test ./test/unit/spec/...

# Replace with:
go test ./internal/...
go test ./internal/engine/...
go test ./internal/config/...
go test ./internal/spec/...
```

### Stale Assertion Fix Pattern (inline during migration)

During each file migration, if `make test` fails due to assertion mismatch:

```go
// BEFORE (stale):
assert.ErrorContains(t, err, "old error message format")

// AFTER (updated to match current production behavior):
assert.ErrorContains(t, err, "current error message format")
```

Do not change test intent — only update expected values to match current reality.

### CLAUDE.md Audit

Search CLAUDE.md files for `test/unit/` references:

```bash
grep -rn "test/unit" CLAUDE.md internal/*/CLAUDE.md .specify/
```

Update any found references to use `internal/[package]/` paths.

## Phase 2: Implementation Sequence

*(Executed by `/speckit.tasks` → `tasks.md`)*

### Recommended Execution Order

```text
US2  →  Audit  →  Batch-A (MV)  →  Batch-B (MERGE)  →  Batch-C (VERIFY)  →  Cleanup  →  US4
```

**US2 (template fix)** — standalone, 1 file, 1 line change, zero risk

**Audit phase** — per-function comparison for all MERGE and VERIFY files; produces
definitive list of functions to keep vs. drop; no file changes

**Batch-A: Clean MV (24 files)** — grouped by package; verify compile after each package:

1. `cli/pagination/` (4 files) — all MV
2. `config/` (3 MV files: `budget_scoped`, `env`, `load`)
3. `engine/` (2 MV files: `budget_scope`, `render`)
4. `engine/batch/` (1 MV file: `processor`)
5. `engine/cache/` (2 MV files: `key`, `store`)
6. `ingest/mapper_test.go` (1 MV file)
7. `pluginhost/` (2 MV files: `discovery`, `lifecycle`)
8. `registry/` (3 MV files: `fallback`, `manifest`, `scan`)
9. `spec/` (3 MV files: `load`, `parse`, `spec`)
10. `tui/list/` (2 MV files: `model`, `render`)
11. `cli/` MV files (3 MV: `flags`, `output`, `prompt`, `plugin_install_fallback`)

**Batch-B: MERGE (8 files)** — per file, audit + merge + verify:

1. `cli/cost_actual_test.go` (functions into `internal/cli/cost_actual_test.go`)
2. `cli/cost_projected_test.go`
3. `cli/plugin_test.go` (into appropriate internal/cli/plugin_*.go test)
4. `config/config_test.go`
5. `engine/engine_test.go`
6. `pluginhost/client_test.go`
7. `ingest/plan_test.go` (post-VERIFY)
8. Any VERIFY files reclassified as MERGE

**Cleanup** — remove `test/unit/` directory entirely with `git rm -r test/unit/`

**US4** — update `test/README.md`

### Quality Gate After Each Batch

```bash
make test       # Must pass
make lint       # Must pass
go test -race ./internal/...  # Must pass (no data races)
```

## Post-Design Constitution Check

- [x] **No TODOs**: Migration tasks are fully specified; no deferred work
- [x] **No Stubs**: All migrated tests exercise real behavior (they were real tests before migration)
- [x] **Implementation Completeness**: All 34 files processed in tasks.md before marking complete
- [x] **Documentation**: test/README.md updated as part of the plan
- [x] **Quality Gates**: make test + make lint gate after each batch
