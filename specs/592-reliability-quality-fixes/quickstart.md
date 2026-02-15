# Quickstart: Verifying Reliability & Quality Fixes

**Feature**: 592-reliability-quality-fixes

## Prerequisites

- Go 1.25.7 installed
- Repository cloned and on `592-reliability-quality-fixes` branch
- `make build` succeeds

## Verification Steps

### 1. Run All Tests

```bash
make test
make lint
```

Both must pass with zero failures.

### 2. Verify --jobs Flag (#602)

```bash
# Auto mode (default behavior preserved)
./bin/finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json

# Explicit worker count
./bin/finfocus cost projected --jobs 4 --pulumi-json examples/plans/aws-simple-plan.json

# Single-threaded mode
./bin/finfocus cost projected --jobs 1 --pulumi-json examples/plans/aws-simple-plan.json

# Verify timing output appears for table format
# Expected: "Analyzed N resources in X.Xs (Y.Y resources/sec)" on stderr

# Verify timing suppressed for JSON
./bin/finfocus cost projected --jobs 2 --output json --pulumi-json examples/plans/aws-simple-plan.json
```

### 3. Verify Race Condition Safety (#652, #653, #656)

```bash
make test-race
```

Must pass with zero data races detected.

### 4. Verify Test Isolation (#605)

```bash
# Run from repo root (may contain Pulumi files)
go test -v ./internal/cli/... -run TestCostProjectedWithoutPulumiJson
go test -v ./internal/cli/... -run TestStackFlagPassedThrough

# Both should pass regardless of working directory
```

### 5. Verify DRY Helpers (#610)

```bash
# Confirm no duplicate functions
grep -rn "formatRecsColumn" internal/tui/
# Expected: zero matches (function removed)

# Confirm exported helpers exist
grep -rn "CountRecommendations" internal/engine/project.go
grep -rn "FormatRecommendationCount" internal/engine/project.go
# Expected: exported function definitions found
```

### 6. Verify Nightly Workflow (#655)

```bash
# Confirm no || true masking
grep "|| true" .github/workflows/nightly.yml
# Expected: zero matches on fuzz test lines
```

### 7. Verify Context Cancellation (#654)

```bash
# Confirm no http.NewRequest without context
grep -n "http.NewRequest(" internal/registry/github.go
# Expected: zero matches (all replaced with http.NewRequestWithContext)
```
