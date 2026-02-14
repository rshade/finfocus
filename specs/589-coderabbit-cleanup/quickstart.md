# Quickstart: CodeRabbit Cleanup

**Branch**: `589-coderabbit-cleanup` | **Date**: 2026-02-13

## Prerequisites

- Go 1.25.7
- Node.js (for `npx prettier` and `npx markdownlint-cli`)
- `golangci-lint` installed

## Implementation Order

Work through groups in this order. Each group is independently testable.

### Step 1: Doc Comment Cleanup (Group 1)

Edit 5 files, 11 functions. For each function:

1. Read the current doc comment
2. Identify duplicated or stuttered content
3. Write a single concise Go-style comment
4. Verify with `go doc ./internal/proto/ resolveSKUAndRegion` (etc.)

Files: `adapter.go`, `pulumi.go`, `aws.go`, `state.go`, `adapter_test.go`

### Step 2: Code Quality Fixes (Group 3)

Seven independent fixes across 5 files:

1. **3a**: Simplify conditional in `cost_actual.go:565`
2. **3b**: Handle error from `GetString("stack")` in `cost_actual.go:539`
3. **3c**: Add `audit.logFailure(ctx, err)` in `cost_projected.go:159`
4. **3d**: Add comment documenting `--pulumi-json` optionality in `cost_projected.go:81`
5. **3e**: Validate empty keys in `parseMetadataFlags` in `plugin_install.go:482`
6. **3f**: Fix region enrichment in `registry.go:73-77`
7. **3g**: Update `NotFoundError` message in `errors.go:36`

### Step 3: Structured Logging (Group 2)

Add missing `component`/`operation` fields to 4 log call sites:

1. `common_execution.go:215` - Add both fields
2. `engine.go:1093` - Add `operation` field
3. `engine.go:1113` - Add `operation` field
4. `pulumi_plan.go:73` - Add `operation` field

### Step 4: Test Improvements (Group 4)

1. **4a**: Add `t.TempDir()` + `os.Chdir()` isolation to 4 tests in `cost_actual_test.go`
2. **4b**: Replace `time.Sleep` with `cancel()` in 2 tests in `pulumi_test.go`

### Step 5: Prettier Formatting (Group 6)

```bash
npx prettier --write docs/guides/routing.md
npx prettier --check docs/guides/routing.md  # Verify
```

### Step 6: PulumiClient Refactor (Group 5) — Optional

This is the largest change. May be deferred to a separate PR.

1. Create `PulumiClient` struct with `runner` field
2. Add `NewClient()` and `NewClientWithRunner()` constructors
3. Convert `GetCurrentStack`, `Preview`, `StackExport` to methods
4. Update call sites in `common_execution.go` and `overview.go`
5. Update tests in `pulumi_test.go` and `test/integration/pulumi_auto_test.go`

## Validation

After each group:

```bash
make test          # All tests pass
make lint          # All linting passes (use extended timeout)
```

After all groups:

```bash
go test -race -count=10 ./internal/pulumi/...    # Race detector
go test -count=100 ./internal/pulumi/...          # Determinism check
go doc ./internal/proto/ resolveSKUAndRegion      # Verify doc comments
```
