# Research: Retire test/unit/ and Fix Test Discovery

**Branch**: `598-retire-test-unit` | **Date**: 2026-02-20

## Summary

This feature is a pure refactoring: move 34 Go test files from `test/unit/` into
their canonical `internal/[package]/` locations so `make test` automatically discovers
them. No new production code is introduced. No API contracts or data models change.

---

## Decision 1 — Makefile is already correct

**Decision**: The Makefile `test` target does NOT need to be changed.

**Finding**: The current target is:

```makefile
test-unit:
    go test -v ./internal/... ./pkg/...
```

This already uses `./internal/...` which automatically picks up any test file under
`internal/`. The "fix" is migrating the files, not changing the Makefile.

**FR-007 implication**: No comment to remove; no `test/unit/` reference exists in the
Makefile. FR-007 is trivially satisfied as part of the migration.

**Rationale**: Go's `./internal/...` glob is a recursive pattern. Once test files live
in `internal/[pkg]/`, they are automatically included.

**Alternatives considered**: Adding `./test/unit/...` to the Makefile — rejected because
it entrenches the wrong pattern and prevents retirement of the directory.

---

## Decision 2 — Actual duplicate count is much lower than assumed

**Decision**: Perform per-function code comparison before classifying any file as
"duplicate". Do not delete based on filename alone.

**Finding**: Initial function-name comparison of files assumed to be duplicates reveals
different test functions:

| test/unit file | Functions | internal/ file | Functions |
|---|---|---|---|
| `cli/cost_actual_test.go` | `TestCostActualCmd_Success`, `TestCostActualCmd_MissingStartDate` | `cli/cost_actual_test.go` | `TestNewCostActualCmd`, `TestCostActualCmdFlags` |
| `engine/engine_test.go` | `TestGetProjectedCost_WithPlugin`, `TestGetProjectedCost_NoPlugin` | `engine/engine_test.go` | `TestAggregateResults`, `TestFilterResources` |

Both "duplicate" candidates contain unique test functions not present in the internal
counterpart. The spec's assumption that `cost_actual_test.go` and `engine_test.go`
are duplicates is likely **inaccurate**.

**Implication**: Most or all 34 files will need to be **migrated**, not deleted. The
migration plan must account for filename collision resolution (the destination may already
have a same-named file with different content).

**Resolution strategy when filename exists in internal/**:
- If functions are disjoint → merge functions into the existing internal file
- If functions are semantically equivalent → drop the duplicate function
- If functions test identical behavior → keep the internal version, drop the test/unit version

**Rationale**: Spec FR-003 already mandates "code-level comparison will confirm the
classification" before deletion.

---

## Decision 3 — Package naming convention

**Decision**: Default package declaration for all migrated files is `package foo_test`
(black-box). White-box (`package foo`) only when unexported symbols are required.

**Rationale**: Go idiomatic convention; prevents accidental internal API reliance;
enforces public contract testing. See spec Clarifications (2026-02-20).

**Finding**: All sampled test/unit/ files already use `package foo_test` convention:
- `test/unit/cli/cost_actual_test.go` → `package cli_test`
- `test/unit/config/config_test.go` → `package config_test`
- `test/unit/engine/engine_test.go` → `package engine_test`
- `test/unit/pluginhost/client_test.go` → `package pluginhost_test`

No white-box package declarations found in sampled files. Actual migration can
preserve existing declarations without changes.

---

## Decision 4 — Git history preservation via `git mv`

**Decision**: All file migrations use `git mv source destination` (or equivalent).

**Rationale**: Preserves `git log --follow` and `git blame` traceability from new path
back to origin. See spec Clarifications (2026-02-20) and FR-011.

**Note on filename collisions**: When both source and destination exist (same filename),
`git mv` cannot be used directly. Strategy:
1. Copy unique functions from test/unit/ file into the existing internal/ file (Edit)
2. Delete the test/unit/ file (`git rm`)
3. The history of the merged-in functions is lost, which is acceptable since the
   merge itself records the provenance.

---

## Decision 5 — `internal/engine/batch/` exists

**Decision**: `test/unit/engine/batch/processor_test.go` can be migrated directly to
`internal/engine/batch/processor_test.go`.

**Finding**: `internal/engine/batch/` contains `batch_test.go`, `doc.go`,
`processor.go`, and `progress.go`. The `processor_test.go` name does not conflict
with `batch_test.go`, so a clean `git mv` applies.

---

## Decision 6 — Destination directory status for all packages

| test/unit/ package | internal/ destination | Tests already exist? | `git mv` possible? |
|---|---|---|---|
| `cli/` | `internal/cli/` | Yes (38 files) | Filename conflict likely |
| `cli/pagination/` | `internal/cli/pagination/` | Yes (`pagination_test.go`) | Filename conflict for sorter_test if same name |
| `config/` | `internal/config/` | Yes (11 files) | Filename conflict likely |
| `engine/` | `internal/engine/` | Yes (30+ files) | Filename conflict likely |
| `engine/batch/` | `internal/engine/batch/` | Yes (`batch_test.go`) | No conflict (`processor_test.go`) |
| `engine/cache/` | `internal/engine/cache/` | Yes (`cache_test.go`) | No conflict (`key_test.go`, `store_test.go`) |
| `ingest/` | `internal/ingest/` | Yes (5 files) | Filename conflict likely |
| `pluginhost/` | `internal/pluginhost/` | Yes (7 files) | `client_test.go` conflicts |
| `registry/` | `internal/registry/` | Yes (12 files) | No name conflicts found |
| `spec/` | `internal/spec/` | Yes (`loader_test.go`, `fuzz_test.go`) | No `spec_test.go` or `parse_test.go` → no conflict |
| `tui/list/` | `internal/tui/list/` | No | Clean `git mv` for both files |

---

## Decision 7 — tasks-template.md change is minimal and precise

**Decision**: Change exactly one line in `.specify/templates/tasks-template.md`.

**Finding**: Line 160:
```
- [ ] TXXX [P] Additional unit tests (ensure 80% coverage minimum) in tests/unit/
```

**Fix**: Replace `in tests/unit/` with `colocated in `internal/[package]/[feature]_test.go``.

**Rationale**: Minimal diff reduces risk. No other test path references exist in the template.

---

## Decision 8 — No impact on CI coverage threshold

**Decision**: Migration will likely INCREASE coverage percentage (previously uncounted
tests now count). The 61% CI threshold will not regress.

**Rationale**: The migrated tests exercise code in `internal/` packages. When those
tests were in `test/unit/`, they ran separately (if at all) with different coverage
profiles. Now they count in the main `./internal/...` coverage run.

**Risk**: If migrated tests fail due to stale assertions, they must be fixed (FR-005)
before merging. CI blocks on test failure regardless of coverage.

---

## Implementation Order

Based on risk and dependency analysis:

1. **Template fix** (lowest risk, zero test code) — US2
2. **Audit all 34 files** (classify: clean-mv, merge, or drop) — US3 prereq
3. **Clean `git mv` migrations** (no conflict, direct move) — US3 batch A
4. **Merge migrations** (function-level merge into existing file) — US3 batch B
5. **Fix stale assertions** (update error messages, output formats) — US3 inline
6. **Remove `test/unit/`** (after all files processed) — US3 final
7. **Update `test/README.md`** — US4

Each batch: run `make test` before moving to next batch to catch regressions early.
