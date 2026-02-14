# Testing Patterns

## Table of Contents

- [Testify Assertions](#testify-assertions)
- [CLI Command Tests](#cli-command-tests)
- [Engine Tests](#engine-tests)
- [Table-Driven Tests](#table-driven-tests)
- [Error Path Testing](#error-path-testing)
- [Expected Failure Patterns](#expected-failure-patterns)
- [Coverage Requirements](#coverage-requirements)

## Testify Assertions

Always import both packages:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**`require.*`** (stops test on failure) - use for:

- Setup operations that must succeed
- Error checks where continuing would panic
- Non-nil checks before property access

```go
result, err := SomeFunction()
require.NoError(t, err)
require.NotNil(t, result)
```

**`assert.*`** (continues on failure) - use for:

- Value comparisons after setup
- Multiple property checks on a result

```go
assert.Equal(t, "expected", result.Name)
assert.Len(t, result.Items, 3)
assert.Contains(t, result.Message, "success")
assert.InDelta(t, 85.0, result.Total, 0.01)
```

**Conversion table**:

| Manual | Testify |
|--------|---------|
| `if err != nil { t.Fatal(err) }` | `require.NoError(t, err)` |
| `if err == nil { t.Error(...) }` | `require.Error(t, err)` |
| `if x != y { t.Errorf(...) }` | `assert.Equal(t, y, x)` |
| `if len(x) != n { ... }` | `assert.Len(t, x, n)` |
| `if !contains { ... }` | `assert.Contains(t, s, sub)` |

## CLI Command Tests

```go
package cli_test

func TestNewCostActualCmd(t *testing.T) {
    tests := []struct {
        name        string
        args        []string
        expectError bool
        errorMsg    string
    }{
        {
            name:        "missing required flag",
            args:        []string{"--from", "2024-01-01"},
            expectError: true,
            errorMsg:    "required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var buf bytes.Buffer
            cmd := cli.NewCostActualCmd()
            cmd.SetOut(&buf)
            cmd.SetErr(&buf)
            cmd.SetArgs(tt.args)

            err := cmd.Execute()

            if tt.expectError {
                require.Error(t, err)
                if tt.errorMsg != "" {
                    assert.Contains(t, err.Error(), tt.errorMsg)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## Engine Tests

```go
func TestAggregateResults(t *testing.T) {
    results := []engine.CostResult{
        {ResourceType: "aws:ec2:Instance", ResourceID: "i-123", Monthly: 10.0, Currency: "USD"},
        {ResourceType: "aws:rds:Instance", ResourceID: "db-456", Monthly: 75.0, Currency: "USD"},
    }

    aggregated := engine.AggregateResults(results)

    assert.InDelta(t, 85.0, aggregated.Summary.TotalMonthly, 0.01)
    assert.Equal(t, "USD", aggregated.Summary.Currency)
    assert.Len(t, aggregated.Summary.ByProvider, 1)
}
```

## Table-Driven Tests

Standard pattern for error conditions:

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

## Error Path Testing

Priority error paths to test:

- File I/O errors (missing files, permission denied)
- Network errors (connection refused, timeout)
- Validation errors (invalid input, out of range)
- Concurrent access errors (race conditions)

## Expected Failure Patterns

For tests that intentionally create failing scenarios (mock plugins, timeouts):

```go
// Use t.Logf() for expected failures (test passes)
client, err := pluginhost.NewClient(ctx, launcher, mockPlugin)
if client != nil {
    client.Close()
}
if err != nil {
    t.Logf("Expected failure (handled): %v", err)
}
```

Never use `t.Errorf()` for expected errors - it causes CI failures.

## Coverage Requirements

- 80% minimum for most packages
- 95% for critical paths (engine, pluginhost)
- All error return paths must be tested
- Check coverage: `go test -coverprofile=coverage.out ./...`
- View: `go tool cover -html=coverage.out`
