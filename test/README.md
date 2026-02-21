# Testing Framework

This directory contains the comprehensive testing framework for the FinFocus system.

## Test Organization

```text
/test
├── README.md             # This file
├── integration/         # Cross-component integration tests
│   ├── plugin/          # Plugin communication tests
│   └── cli/             # CLI integration tests
├── e2e/                 # End-to-end workflow tests
├── fixtures/            # Test data files
│   ├── plans/           # Sample Pulumi plans
│   ├── specs/           # Test pricing specifications
│   ├── configs/         # Test configuration files
│   └── responses/       # Mock plugin responses
├── mocks/               # Mock implementations
│   ├── plugin/          # Mock plugin server
│   └── services/        # Mock external services
└── benchmarks/          # Performance and benchmark tests

```

## Testing Categories

### Unit Tests (colocated with source)

Unit tests live alongside the source code they test:

- Path: `internal/[package]/[name]_test.go`
- Black-box tests (public API only): `package foo_test`
- White-box tests (access to unexported symbols): `package foo`
- Run with: `go test ./internal/...` (included in `make test`)

> The `test/unit/` directory was retired; see issue #732 for details.

### Integration Tests (/test/integration/)

Cross-component interaction testing:

- CLI command execution with real data

- Plugin discovery and launching

- gRPC communication with plugins

- File system operations (specs, configs)

- Cross-component data flow validation

### End-to-End Tests (/test/e2e/)

Complete workflow validation:

- Full workflow: Plan → Ingest → Calculate → Output

- Multiple output formats (table, JSON, NDJSON)

- Error scenarios and recovery

- Real plugin integration scenarios

### Performance Tests (/test/benchmarks/)

- Cost calculation performance

- Plugin communication latency

- Memory usage optimization

- Large plan processing

## Test Fixtures

### Sample Plans (/test/fixtures/plans/)

- AWS infrastructure examples

- Multi-provider scenarios

- Edge cases and error conditions

- Large-scale deployments

### Mock Responses (/test/fixtures/responses/)

- Plugin API responses

- Error scenarios

- Performance test data

- Timeout and retry scenarios

### Test Configurations (/test/fixtures/configs/)

- Various plugin configurations

- Different output format settings

- Authentication scenarios

## Mock Implementations

### Mock Plugin (/test/mocks/plugin/)

Configurable plugin server for testing:

- Implements full gRPC CostSource interface

- Configurable responses for testing scenarios

- Error injection capabilities

- Performance testing support

- Timeout and retry testing

### Mock Services (/test/mocks/services/)

- Mock cloud provider APIs

- Mock file system operations

- Mock network services

## Testing Tools and Utilities

- Go testing framework with testify assertions

- gRPC testing utilities

- Golden file testing for output formats

- Table-driven tests for multiple scenarios

- Test helpers for common setup/teardown

## Coverage Requirements

- **Minimum 80% code coverage** overall

- **Critical paths must have 95% coverage**

- All error handling paths tested

- Performance regression detection

## Running Tests

```bash
# All tests (unit + any additional internal tests)
make test

# Unit tests only (colocated with source)
go test ./internal/...

# Integration tests only
go test ./test/integration/...

# End-to-end tests only
go test ./test/e2e/...

# Benchmarks only
go test -bench=. ./test/benchmarks/...

# Mock plugin tests
go test ./test/mocks/plugin/...

# Coverage report for all tests
go test -coverprofile=coverage.out ./internal/... ./pkg/...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./internal/...

# Run tests with verbose output
go test -v ./internal/...

# Run specific test by name
go test -run TestEngine_GetProjectedCost ./internal/engine/...

# Run benchmarks with memory stats
go test -bench=. -benchmem ./test/benchmarks/...

# Run integration tests with longer timeout
go test -timeout=60s ./test/integration/...
```

## Test Categories and Commands

### Unit Tests

Test individual components in isolation (colocated with source):

```bash
# Engine unit tests
go test ./internal/engine/...

# Config unit tests
go test ./internal/config/...

# Spec unit tests
go test ./internal/spec/...

# All unit tests
go test ./internal/...
```

### Integration Tests

Test cross-component interactions:

```bash
# Plugin communication tests
go test ./test/integration/plugin/...

# CLI integration tests (part of e2e)
go test ./test/e2e/...
```

### End-to-End Tests

Test complete workflows:

```bash
# CLI workflow tests (requires building binary)
go test -tags e2e ./test/e2e/...

# Run with timeout for slow builds
go test -tags e2e -timeout=120s ./test/e2e/...

# Run only Analyzer integration tests
go test -tags e2e -v -run "TestAnalyzer" ./test/e2e/...
```

### Performance Tests

Benchmark cost calculation performance:

```bash
# All benchmarks
go test -bench=. ./test/benchmarks/...

# Engine benchmarks only
go test -bench=BenchmarkEngine ./test/benchmarks/...

# Memory allocation benchmarks
go test -bench=Allocation -benchmem ./test/benchmarks/...

# Concurrent benchmarks
go test -bench=Concurrent ./test/benchmarks/...
```

### Mock Plugin Tests

Test mock implementations:

```bash
# Mock plugin functionality
go test ./test/mocks/plugin/...

# Run with race detection for concurrent tests
go test -race ./test/mocks/plugin/...
```
