# Quickstart: Test Quality and CI/CD Improvements

**Date**: 2026-01-19
**Branch**: `118-test-quality-ci`

## Verification Commands

After implementing this feature, use these commands to verify success:

### 1. Fuzz Test Verification

```bash
# Run fuzz test for 30 seconds (smoke test)
go test -fuzz=FuzzJSON -fuzztime=30s ./internal/ingest

# Run full fuzz test for longer duration
go test -fuzz=FuzzPulumiPlanParse -fuzztime=30s ./internal/ingest

# Verify no panics occurred (exit code 0)
echo "Fuzz tests completed: $?"
```

### 2. Benchmark Verification

```bash
# Run benchmarks
go test -bench=. -benchmem ./test/benchmarks/...

# Expected output should show:
# BenchmarkParse_PulumiPlan-8    XXXXX    XXXX ns/op    XXXX B/op    XX allocs/op
# BenchmarkParse_LargePlan-8    XXXX     XXXXX ns/op   XXXXX B/op   XXX allocs/op
```

### 3. E2E Test Verification

```bash
# Build the binary first
make build

# Run specific GCP test (error handling fix)
go test -v -tags e2e ./test/e2e/... -run TestE2E_GCP_ProjectedCost

# Run NDJSON output test (schema validation)
go test -v -tags e2e ./test/e2e/... -run TestE2E_Output_NDJSON

# Run ActualCost E2E test (requires plugin installed)
go test -v -tags e2e ./test/e2e/... -run TestE2E_ActualCost
```

### 4. Cross-Repo Workflow Verification

```bash
# Validate workflow syntax
gh workflow validate .github/workflows/cross-repo-integration.yml

# Trigger workflow manually (after merging)
gh workflow run cross-repo-integration.yml --field plugin_ref=main --field core_ref=main

# Check workflow status
gh run list --workflow=cross-repo-integration.yml --limit=1
```

### 5. Full Validation Suite

```bash
# Run all tests
make test

# Run linting
make lint

# Both must pass before completion
```

## Success Criteria Checklist

| Criterion                    | Verification Command                                     | Expected Result            |
| ---------------------------- | -------------------------------------------------------- | -------------------------- |
| SC-001: Fuzz tests run 30s   | `go test -fuzz=FuzzJSON -fuzztime=30s ./internal/ingest` | Exit code 0, no panics     |
| SC-002: Benchmarks report    | `go test -bench=. ./test/benchmarks/...`                 | > 0 ns/op reported         |
| SC-003: E2E tests pass       | `go test -tags e2e ./test/e2e/...`                       | All PASS                   |
| SC-004: Workflow < 10min     | `gh run view <id>`                                       | Duration < 600s            |
| SC-005: No ignored errors    | Code review                                              | No `_, _` patterns         |
| SC-006: NDJSON validated     | Test includes `assert.Contains`                          | Schema fields checked      |
| SC-007: ActualCost E2E       | `go test -tags e2e -run TestE2E_ActualCost`              | PASS                       |

## Common Issues

### Fuzz Test Failures

If fuzz tests fail:

1. Check for panics in the fuzzer output
2. Add failing input to seed corpus for regression testing
3. Fix parser to handle edge case gracefully

### Benchmark Issues

If benchmarks fail to run:

1. Verify JSON structure uses `steps` not `resourceChanges`
2. Check that resource format includes `op`, `urn`, `type`

### E2E Test Setup

For ActualCost E2E tests:

1. Build finfocus binary: `make build`
2. Install aws-public plugin to `~/.finfocus/plugins/aws-public/<version>/`
3. Verify plugin: `./bin/finfocus plugin list`

### Workflow Debugging

If cross-repo workflow fails:

1. Check GitHub Actions logs
2. Verify plugin build commands work locally
3. Check that `GITHUB_TOKEN` has required permissions
