# Mock Plugin

This package provides a configurable mock plugin for testing FinFocus's plugin communication layer.

## Features

- **Full gRPC CostSourceService implementation** - Compatible with the real plugin protocol
- **Configurable responses** - Set custom responses for different resource types
- **Error injection** - Simulate timeout, protocol, and data errors
- **Performance simulation** - Add latency to test performance scenarios
- **Pre-configured scenarios** - Common test scenarios ready to use
- **Test helpers** - Convenient functions for test setup and teardown

## Quick Start

### Basic Usage

```go
package mytest

import (
    "testing"
    "github.com/rshade/finfocus/test/mocks/plugin"
)

func TestWithMockPlugin(t *testing.T) {
    // Create test helper (automatically starts server and handles cleanup)
    helper := plugin.NewTestHelper(t)

    // Configure a response
    helper.SetProjectedCost("aws:ec2/instance:Instance", 7.30, 0.01)

    // Get a client connection
    conn := helper.Dial()

    // Use conn with your code under test
    // ...
}
```

### Using Pre-configured Scenarios

```go
func TestSuccessScenario(t *testing.T) {
    helper := plugin.NewTestHelper(t)

    // Configure common AWS resources with realistic pricing
    helper.ConfigureScenario(plugin.ScenarioSuccess)

    // Your test code here
}
```

## Available Scenarios

- **ScenarioSuccess** - Typical successful responses for AWS resources (EC2, S3, RDS, Lambda)
- **ScenarioPartialData** - Some resources have cost data, others don't
- **ScenarioHighCost** - Expensive resources for testing cost warnings
- **ScenarioZeroCost** - Free-tier resources
- **ScenarioMultiCurrency** - Mixed currencies (USD + EUR)

## Error Injection

### Simulating Timeouts

```go
func TestTimeout(t *testing.T) {
    helper := plugin.NewTestHelper(t)

    // Make GetProjectedCost return a timeout error
    helper.SetError("GetProjectedCost", plugin.ErrorTimeout)

    // Your test code should handle the timeout
}
```

### Available Error Types

- `ErrorNone` - No error (default)
- `ErrorTimeout` - Simulates timeout/deadline exceeded
- `ErrorProtocol` - Simulates gRPC protocol error
- `ErrorInvalidData` - Simulates invalid data from plugin
- `ErrorUnavailable` - Simulates service unavailable

## Performance Testing

### Adding Latency

```go
func TestWithLatency(t *testing.T) {
    mock := plugin.NewMockPlugin()

    // Add 100ms latency to all requests
    mock.SetLatency(100)

    helper := plugin.NewTestHelperWithPlugin(t, mock)

    // Your performance test code here
}
```

## Advanced Usage

### Manual Server Management

If you need more control over the server lifecycle:

```go
func TestManualServer(t *testing.T) {
    // Start server manually
    server, err := plugin.StartMockServer()
    if err != nil {
        t.Fatal(err)
    }
    defer server.Stop()

    // Configure the plugin
    server.Plugin.ConfigureScenario(plugin.ScenarioSuccess)

    // Get connection
    ctx := context.Background()
    conn, err := server.Dial(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close()

    // Use connection
}
```

### TCP Server for Integration Tests

For integration tests that need a real TCP connection:

```go
func TestTCPServer(t *testing.T) {
    server, err := plugin.StartMockServerTCP()
    if err != nil {
        t.Fatal(err)
    }
    defer server.Stop()

    // Get the actual TCP address
    address := server.Address()  // e.g., "127.0.0.1:12345"

    // Use address in your integration test
}
```

### Custom Response Configuration

```go
func TestCustomResponse(t *testing.T) {
    helper := plugin.NewTestHelper(t)

    // Set a custom response for a specific resource type
    helper.Plugin().SetProjectedCostResponse("aws:ec2/instance:Instance", &proto.CostResult{
        Currency:    "USD",
        MonthlyCost: 150.00,
        HourlyCost:  0.205,
        Notes:       "Custom pricing for test",
        CostBreakdown: map[string]float64{
            "compute": 120.00,
            "storage": 30.00,
        },
    })

    // Your test code here
}
```

### Actual Cost Responses

```go
func TestActualCost(t *testing.T) {
    helper := plugin.NewTestHelper(t)

    // Configure actual cost response
    helper.Plugin().ConfigureActualCostScenario("i-1234567890abcdef0", 45.67, map[string]float64{
        "compute": 30.00,
        "storage": 15.67,
    })

    // Your test code here
}
```

### Terraform Type Resolution

The mock implements `ResolveResourceTypes`. `ConfigureTerraformResolver`
advertises the `aws` provider and the `resolve_resource_types` capability
through `GetPluginInfo`. Requested types without a configured mapping are
left out of the response, which simulates partial resolution.

```go
func TestResolve(t *testing.T) {
    helper := plugin.NewTestHelper(t)
    helper.Plugin().ConfigureTerraformResolver(plugin.AWSTerraformTypeMappings())
    helper.Plugin().SetError("ResolveResourceTypes", plugin.ErrorUnavailable) // optional

    // Resolve against the server, then check the call count
    _ = helper.Plugin().GetResolveCallCount()
}
```

`StartMockServerTCPAt(plugin, "127.0.0.1:<port>")` serves the mock on a fixed
address. `test/integration/terraform_resolver_plugin_test.go` uses it to run
the mock as a real plugin process on the `--port` the plugin host assigns.

## Reset Between Tests

The TestHelper automatically handles cleanup, but if you're managing the plugin manually:

```go
func TestMultipleScenarios(t *testing.T) {
    helper := plugin.NewTestHelper(t)

    // Test scenario 1
    helper.ConfigureScenario(plugin.ScenarioSuccess)
    // ... test code ...

    // Reset before scenario 2
    helper.Reset()

    // Test scenario 2
    helper.ConfigureScenario(plugin.ScenarioHighCost)
    // ... test code ...
}
```

## Integration with Engine Tests

Example of testing the engine with mock plugin:

```go
func TestEngineWithMockPlugin(t *testing.T) {
    helper := plugin.NewTestHelper(t)
    helper.ConfigureScenario(plugin.ScenarioSuccess)

    conn := helper.Dial()
    defer conn.Close()

    // Create engine client
    client := proto.NewClient(conn)

    // Test engine methods
    request := &proto.GetProjectedCostRequest{
        Resources: []*proto.ResourceDescriptor{
            {Type: "aws:ec2/instance:Instance", Provider: "aws"},
        },
    }

    response, err := client.GetProjectedCost(context.Background(), request)
    require.NoError(t, err)
    require.Len(t, response.Results, 1)
    require.Equal(t, "USD", response.Results[0].Currency)
}
```

## Best Practices

1. **Use TestHelper** - Simplifies setup and ensures proper cleanup
2. **Reset between scenarios** - Prevents test interference
3. **Use pre-configured scenarios** - Saves time and ensures consistency
4. **Configure before dialing** - Set up responses before connecting clients
5. **Handle errors** - Test both success and failure paths
6. **Test isolation** - Each test should be independent

## Troubleshooting

### Plugin Not Responding

If your test hangs, check:

- Did you call `Dial()` before the test timeout?
- Is the server started before dialing?
- Did you configure a response for the resource type you're querying?

### No Response Configured Error

If you get "mock plugin: no response configured for resource":

- The resource type doesn't match any configured response
- Use `ConfigureScenario` or `SetProjectedCostResponse` to add it
- Check the exact resource type string (case-sensitive)

### Tests Interfering

If tests affect each other:

- Use `helper.Reset()` between test cases
- Each test should get its own TestHelper instance
- Don't share MockPlugin instances across tests

## API Reference

### Core Types

- `MockPlugin` - Main plugin configuration and state
- `MockServer` - Running gRPC server instance
- `TestHelper` - Test convenience wrapper

### Key Methods

- `NewTestHelper(t)` - Create helper with automatic cleanup
- `ConfigureScenario(scenario)` - Apply pre-configured scenario
- `SetProjectedCost(type, monthly, hourly)` - Quick response setup
- `SetError(method, errorType)` - Inject errors
- `SetLatency(ms)` - Add simulated latency
- `ConfigureTerraformResolver(mappings)` - Advertise and serve `ResolveResourceTypes`
- `SetResolveResourceTypesHook(fn)` - Observe each `ResolveResourceTypes` request
- `Reset()` - Clear all configuration

### Scenarios

- `ScenarioSuccess` - Normal operation
- `ScenarioPartialData` - Missing data
- `ScenarioHighCost` - Expensive resources
- `ScenarioZeroCost` - Free tier
- `ScenarioMultiCurrency` - Mixed currencies

### Error Types

- `ErrorTimeout` - Deadline exceeded
- `ErrorProtocol` - gRPC/transport error
- `ErrorInvalidData` - Bad response data
- `ErrorUnavailable` - Service down

## Examples

See the test files in this package for comprehensive usage examples:

### Runnable Examples

- `examples_test.go` - 18 runnable examples demonstrating all mock plugin features:
  - Basic usage and custom responses
  - All 5 scenarios (Success, PartialData, HighCost, ZeroCost, MultiCurrency)
  - All 4 error types (Timeout, Protocol, InvalidData, Unavailable)
  - Latency simulation for performance testing
  - Combined configurations (scenario + error + latency)
  - Actual cost responses with detailed breakdowns
  - Reset and test isolation patterns
  - Dynamic configuration changes during tests

### Test Coverage (Phase 4 - Mock Plugin Enhancement)

- `config_test.go` - 15 tests for response configuration and scenarios
- `errors_test.go` - 21 tests for error injection capabilities
- `perf_test.go` - 17 tests for performance simulation and latency
- `api_test.go` - API configuration tests
- `server_test.go` - Server functionality tests
- `helpers_test.go` - Test helper examples

**Total: 53+ tests with 100% coverage of mock plugin functionality**
