# Quickstart: Verify the test/unit/ Retirement

**Branch**: `598-retire-test-unit` | **Date**: 2026-02-20

This guide shows how to verify the migration is complete and correct.

## Prerequisites

- Go 1.25.7 installed
- Repository cloned and dependencies fetched (`go mod download`)
- `make` available

## Step 1 — Confirm test/unit/ is gone

```bash
find . -path ./test/unit -prune -o -print | grep test/unit
# Expected: no output
```

## Step 2 — Run all unit tests

```bash
make test
# Expected: all tests pass, zero failures
# Look for test output from migrated packages:
#   ok  github.com/rshade/finfocus/internal/cli/pagination
#   ok  github.com/rshade/finfocus/internal/engine/batch
#   ok  github.com/rshade/finfocus/internal/engine/cache
#   ok  github.com/rshade/finfocus/internal/tui/list
```

## Step 3 — Run with race detector

```bash
make test-race
# Expected: all tests pass with -race, no data-race errors
```

## Step 4 — Verify template is updated

```bash
grep "tests/unit" .specify/templates/tasks-template.md
# Expected: no output (reference has been removed)

grep "internal/\[package\]" .specify/templates/tasks-template.md
# Expected: at least one match on the Polish phase line
```

## Step 5 — Verify test/README.md is updated

```bash
grep "go test ./test/unit/" test/README.md
# Expected: no output

grep "./internal/..." test/README.md
# Expected: at least one match (unit test command)
```

## Step 6 — Run linting

```bash
make lint
# Expected: no lint errors, no markdownlint errors
```

## Step 7 — Check coverage does not regress

```bash
go test -coverprofile=coverage.out ./internal/... ./pkg/...
go tool cover -func=coverage.out | grep total
# Expected: total coverage >= 61% (CI threshold)
```

## Step 8 — Verify speckit.tasks generates correct paths

After completing the migration, generate a test tasks list:

```bash
# In a separate branch, create a trivial spec and run /speckit.tasks
# Inspect the generated tasks.md for any test/unit/ references:
grep "test/unit\|tests/unit" specs/*/tasks.md
# Expected: no output
```

## Troubleshooting

### A migrated test fails to compile

Most common causes:
1. **Import path mismatch** — The test imports from `test/unit/somehelper` that was moved
   or deleted. Update the import to `internal/somepackage` or `test/mocks/`.
2. **Package declaration wrong** — If the test uses unexported symbols, it needs
   `package foo` not `package foo_test`. Verify against the source file.
3. **Stale assertion** — Error message or output format changed. Update the assertion
   to match current production behavior (see FR-005).

### A MERGE file has duplicate function names

If `internal/cli/cost_actual_test.go` already has `TestCostActualCmd_Success` AND the
test/unit/ version also defines it:
1. Compare both implementations side by side
2. Keep the version with better coverage/assertions
3. Drop the weaker version
4. Ensure the kept function compiles and passes

### Coverage drops below 61%

If migration of stale tests surfaces production bugs (tests fail, not just stale):
1. Do NOT lower the threshold
2. Fix the production bug inline (preferred) or create a tracked issue
3. The PR must not merge until `make test` is green
