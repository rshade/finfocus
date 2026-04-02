---
title: Testing Guide
description: Comprehensive guide to testing in FinFocus Core
---

This guide covers the testing philosophy, strategy, and practical commands for
contributors working on FinFocus Core.

## Testing Philosophy

1. **Test-Driven Development (TDD)**: Write tests before implementation.
2. **High Coverage**: CI enforces a 60% minimum; aim for 80% overall and 95% on
   critical paths.
3. **Isolation**: Unit tests must not depend on external systems. Use mocks and
   table-driven patterns.
4. **Integration**: Verify component interactions with dedicated integration tests
   in `test/integration/`.
5. **Performance**: Benchmarks must catch regressions on critical paths.

## Test Layout

Unit tests follow the standard Go convention: each `foo_test.go` file lives
beside the `foo.go` it tests. There is no separate `test/unit/` directory.

Additional test infrastructure lives under `test/`:

| Directory          | Contents                                                         |
| ------------------ | ---------------------------------------------------------------- |
| `test/integration/`| Cross-component tests (CLI, Engine, Plugin communication).       |
| `test/e2e/`        | End-to-end tests. Separate Go module. Requires AWS + Pulumi CLI. |
| `test/fixtures/`   | Shared test data: plans, specs, configs, mock responses.         |
| `test/mocks/`      | Mock plugin server implementations.                              |
| `test/benchmarks/` | Performance benchmarks for regression detection.                 |

## Running Tests

### All Unit Tests

```bash
make test
```

### With Race Detector

```bash
make test-race
```

### Integration Tests

```bash
make test-integration
```

### E2E Tests

E2E tests require a built binary, AWS credentials, and the Pulumi CLI.

```bash
make build
export PATH="$HOME/.pulumi/bin:$PATH"
export PULUMI_CONFIG_PASSPHRASE="e2e-test-passphrase"
make test-e2e
```

### Single Package

```bash
go test -v ./internal/cli/...
go test -v ./internal/engine/...
```

### Single Test Function

```bash
go test -run TestSpecificFunction ./...
```

### Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Linting

```bash
make lint
```

## Writing Tests

### Framework: testify

All tests use `github.com/stretchr/testify/require` and
`github.com/stretchr/testify/assert`. Do not write manual `if x != y { t.Errorf(...) }`
checks.

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

Use `require.*` when a failure makes the rest of the test invalid:

```go
result, err := SomeFunction(input)
require.NoError(t, err)
require.NotNil(t, result)
```

Use `assert.*` for value comparisons where seeing all failures is helpful:

```go
assert.Equal(t, "expected", result.Name)
assert.Len(t, result.Items, 3)
assert.Contains(t, result.Message, "success")
```

### Table-Driven Tests

Prefer table-driven tests for functions with multiple input variations:

```go
func TestFunction_Errors(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        wantErr     bool
        errContains string
    }{
        {"empty input", "", true, "input required"},
        {"invalid format", "bad", true, "invalid format"},
        {"valid input", "good", false, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Function(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContains)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Error Path Coverage

Every error return must have a test that triggers it. Priority paths:

- File I/O errors (missing files, permission denied)
- Network errors (connection refused, timeout)
- Validation errors (invalid input, out-of-range values)
- Resource exhaustion (goroutine leaks, unclosed handles)

### Expected-Failure Patterns

Tests that intentionally create failing plugin scenarios must use `t.Logf()`
rather than `t.Errorf()`, so CI does not flag expected errors as failures:

```go
client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)
if client != nil {
    client.Close()
}
if err != nil {
    t.Logf("Expected failure (handled): %v", err)
}
```

Use `require.Error` when an error is required for the test to be valid:

```go
import "github.com/stretchr/testify/require"

_, err := launcher.Start(ctx, "/nonexistent/binary")
require.Error(t, err, "expected error for invalid command")
```

## Coverage Requirements

| Scope          | Minimum |
| -------------- | ------- |
| CI gate        | 60%     |
| General target | 80%     |
| Critical paths | 95%     |

Critical paths include the Engine cost calculation pipeline, Plugin host
lifecycle, and CLI command dispatch.
