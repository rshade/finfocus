# Quick Start: Filter Integration Tests

**Feature**: Integration Tests for --filter Flag
**Date**: Tue Dec 16 2025

## Overview

This guide helps you understand and run the integration tests for the `--filter` flag across cost commands. These tests validate that resource filtering works correctly in real CLI execution scenarios.

## Prerequisites

- Go 1.25.6+ installed
- Project built: `make build`
- Test fixtures available in `test/fixtures/plans/`

## Test Structure

### Test Files

```
test/integration/cli/
├── filter_test.go          # New file: filter integration tests
├── cli_workflow_test.go    # Existing: general CLI tests
└── helpers/
    └── cli_helper.go       # Test infrastructure
```

### Test Categories

#### Projected Cost Filtering (`cost projected --filter`)

- **Type filtering**: `type=aws:ec2/instance`, `type=ec2`
- **Provider filtering**: `provider=aws`
- **Edge cases**: No matches, invalid syntax

#### Actual Cost Tag Filtering (`cost actual --group-by "tag:key=value"`)

- **Tag filtering**: `--group-by "tag:env=prod"`
- **Combined filtering**: Group-by with tag filters

#### Output Format Validation

- **Table output**: Human-readable ASCII tables
- **JSON output**: Machine-parseable structured data
- **NDJSON output**: Streamable line-delimited JSON

## Running Tests

### Run All Filter Tests

```bash
go test ./test/integration/cli -run TestFilter -v
```

### Run Specific Test Categories

```bash
# Projected cost type filtering
go test ./test/integration/cli -run TestProjectedCost_FilterByType -v

# Provider filtering
go test ./test/integration/cli -run TestProjectedCost_FilterByProvider -v

# Edge cases
go test ./test/integration/cli -run TestProjectedCost_FilterNoMatch -v

# Tag filtering (actual costs)
go test ./test/integration/cli -run TestActualCost_FilterByTag -v

# Output format validation
go test ./test/integration/cli -run TestFilter_AllOutputFormats -v
```

### Run with Race Detection

```bash
go test ./test/integration/cli -run TestFilter -race -v
```

## Understanding Test Output

### Successful Test Run

```
=== RUN   TestProjectedCost_FilterByType
--- PASS: TestProjectedCost_FilterByType (0.15s)
```

### Failed Test Example

```
=== RUN   TestProjectedCost_FilterByType
    cli_helper.go:98: Command failed: exit status 1
    filter_test.go:45: Expected 3 EC2 instances, got 5 total resources
--- FAIL: TestProjectedCost_FilterByType (0.12s)
```

## Test Fixtures

### Multi-Resource Test Plan

Located: `test/fixtures/plans/multi-resource-plan.json`

Contains 10-20 resources across providers:

- **AWS**: EC2 instances, S3 buckets, RDS instances, Lambda functions
- **Azure**: Virtual machines, storage accounts
- **GCP**: Compute instances, Cloud Storage buckets

### Using Custom Test Plans

```go
// In test code
planFile := h.CreateTempPlanFile(t, customPlanJSON)
output, err := h.Execute("cost", "projected",
    "--pulumi-json", planFile,
    "--filter", "type=aws:ec2/instance")
```

## Common Issues

### Test Fixture Problems

**Symptom**: Tests fail with "invalid plan JSON"
**Solution**: Verify test plan is valid Pulumi export format

### Filter Logic Errors

**Symptom**: Unexpected resources in filtered output
**Solution**: Check filter syntax and case sensitivity (filters are case-insensitive)

### Output Parsing Issues

**Symptom**: JSON parsing errors in tests
**Solution**: Ensure output format matches expected structure

## Debugging Tips

### Enable Debug Logging

```go
// In test setup
zerolog.SetGlobalLevel(zerolog.DebugLevel) // Temporarily enable logging
```

### Inspect Raw Output

```go
output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--filter", "type=ec2")
t.Logf("Raw output: %s", output) // Log actual command output
```

### Validate Test Plans

```bash
# Check if plan is valid JSON
cat test/fixtures/plans/multi-resource-plan.json | jq .
```

## Performance Expectations

- **Individual tests**: < 2 seconds
- **Full test suite**: < 10 seconds additional to baseline
- **Resource count**: Tests designed for 10-20 resources per plan

## Integration with CI/CD

Tests run automatically in CI with:

```bash
make test
```

Linting verification:

```bash
make lint
```

Coverage reporting includes filter test paths.
