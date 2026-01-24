package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/test/mocks/plugin"
)

// Example_basicUsage demonstrates basic mock plugin configuration.
func Example_basicUsage() {
	mock := plugin.NewMockPlugin()

	// Configure a simple response
	mock.SetProjectedCostResponse(
		"aws:ec2/instance:Instance",
		plugin.QuickResponse("USD", 7.30, 0.01),
	)

	// Use the mock in your tests
	_ = mock
}

// Example_scenarioSuccess demonstrates the success scenario with realistic costs.
func Example_scenarioSuccess() {
	mock := plugin.NewMockPlugin()

	// Configure typical AWS resources
	mock.ConfigureScenario(plugin.ScenarioSuccess)

	// Now the mock has responses for EC2, S3, RDS, and Lambda
	config := mock.GetConfig()
	_ = config.ProjectedCostResponses["aws:ec2/instance:Instance"]    // EC2 instance
	_ = config.ProjectedCostResponses["aws:s3/bucket:Bucket"]         // S3 bucket
	_ = config.ProjectedCostResponses["aws:rds/instance:Instance"]    // RDS instance
	_ = config.ProjectedCostResponses["aws:lambda/function:Function"] // Lambda function
}

// Example_scenarioPartialData demonstrates testing with missing cost data.
func Example_scenarioPartialData() {
	mock := plugin.NewMockPlugin()

	// Simulate a scenario where only some resources have cost data
	mock.ConfigureScenario(plugin.ScenarioPartialData)

	// Your code should handle missing cost data gracefully
	config := mock.GetConfig()
	_ = config.ProjectedCostResponses["aws:ec2/instance:Instance"] // Has data
	// aws:s3/bucket:Bucket intentionally not configured
}

// Example_scenarioHighCost demonstrates testing cost warnings with expensive resources.
func Example_scenarioHighCost() {
	mock := plugin.NewMockPlugin()

	// Configure expensive resources for testing cost alerts
	mock.ConfigureScenario(plugin.ScenarioHighCost)

	// Resources will have high monthly costs (>$1000)
	config := mock.GetConfig()
	ec2 := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	_ = ec2.MonthlyCost // Will be > $1000 for GPU instance
}

// Example_scenarioZeroCost demonstrates testing free-tier resources.
func Example_scenarioZeroCost() {
	mock := plugin.NewMockPlugin()

	// Configure free-tier resources
	mock.ConfigureScenario(plugin.ScenarioZeroCost)

	// All costs will be $0.00
	config := mock.GetConfig()
	s3 := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]
	_ = s3.MonthlyCost // Will be 0.00
}

// Example_scenarioMultiCurrency demonstrates testing currency aggregation.
func Example_scenarioMultiCurrency() {
	mock := plugin.NewMockPlugin()

	// Configure resources with different currencies
	mock.ConfigureScenario(plugin.ScenarioMultiCurrency)

	// Some resources in USD, others in EUR
	config := mock.GetConfig()
	ec2 := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	rds := config.ProjectedCostResponses["aws:rds/instance:Instance"]

	_ = ec2.Currency // "USD"
	_ = rds.Currency // "EUR"
}

// Example_errorTimeout demonstrates timeout error injection.
func Example_errorTimeout() {
	mock := plugin.NewMockPlugin()

	// Make GetProjectedCost return a timeout error
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	// Your code should handle timeout gracefully
	config := mock.GetConfig()
	_ = config.ErrorType   // ErrorTimeout
	_ = config.ErrorMethod // "GetProjectedCost"
}

// Example_errorProtocol demonstrates protocol error injection.
func Example_errorProtocol() {
	mock := plugin.NewMockPlugin()

	// Simulate a gRPC protocol error
	mock.SetError("GetActualCost", plugin.ErrorProtocol)

	// Test your error handling
	config := mock.GetConfig()
	_ = config.ErrorType // ErrorProtocol
}

// Example_errorInvalidData demonstrates invalid data error injection.
func Example_errorInvalidData() {
	mock := plugin.NewMockPlugin()

	// Simulate plugin returning invalid data
	mock.SetError("GetProjectedCost", plugin.ErrorInvalidData)

	// Your code should validate plugin responses
	config := mock.GetConfig()
	_ = config.ErrorType // ErrorInvalidData
}

// Example_errorUnavailable demonstrates service unavailable error injection.
func Example_errorUnavailable() {
	mock := plugin.NewMockPlugin()

	// Simulate plugin service being unavailable
	mock.SetError("GetActualCost", plugin.ErrorUnavailable)

	// Test retry logic or fallback behavior
	config := mock.GetConfig()
	_ = config.ErrorType // ErrorUnavailable
}

// Example_latencySimulation demonstrates performance testing with latency.
func Example_latencySimulation() {
	mock := plugin.NewMockPlugin()

	// Add 100ms latency to simulate network delay
	mock.SetLatency(100)

	// Your performance tests can measure total time
	config := mock.GetConfig()
	_ = config.LatencyMS // 100
}

// Example_combinedConfiguration demonstrates complex test scenarios.
func Example_combinedConfiguration() {
	mock := plugin.NewMockPlugin()

	// Configure a complete test scenario
	mock.ConfigureScenario(plugin.ScenarioSuccess)      // Realistic costs
	mock.SetError("GetActualCost", plugin.ErrorTimeout) // Timeout on actual costs
	mock.SetLatency(50)                                 // 50ms latency

	// Now you can test complex scenarios like:
	// - Projected costs work (scenario configured)
	// - Actual costs fail with timeout (error configured)
	// - Everything has 50ms delay (latency configured)
}

// Example_customResponse demonstrates creating custom cost responses.
func Example_customResponse() {
	mock := plugin.NewMockPlugin()

	// Create a custom response for your specific test
	customResponse := plugin.QuickResponse("USD", 99.99, 0.137)
	mock.SetProjectedCostResponse("custom:service:Type", customResponse)

	// Test with your custom resource type
	config := mock.GetConfig()
	_ = config.ProjectedCostResponses["custom:service:Type"]
}

// Example_actualCostResponse demonstrates configuring actual cost responses.
func Example_actualCostResponse() {
	mock := plugin.NewMockPlugin()

	// Configure historical actual costs
	breakdown := map[string]float64{
		"compute": 100.00,
		"storage": 50.00,
		"network": 25.00,
	}
	mock.ConfigureActualCostScenario("resource-id-123", 175.00, breakdown)

	// Test actual cost queries
	config := mock.GetConfig()
	actual := config.ActualCostResponses["resource-id-123"]
	_ = actual.TotalCost     // 175.00
	_ = actual.CostBreakdown // Map with detailed breakdown
}

// Example_reset demonstrates resetting mock state between tests.
func Example_reset() {
	mock := plugin.NewMockPlugin()

	// Configure for first test
	mock.ConfigureScenario(plugin.ScenarioSuccess)
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)
	mock.SetLatency(100)

	// ... run test ...

	// Reset for next test
	mock.Reset()

	// Mock is now back to default state
	config := mock.GetConfig()
	_ = len(config.ProjectedCostResponses) // 0 (empty)
	_ = config.ErrorType                   // ErrorNone
	_ = config.LatencyMS                   // 0
}

// Example_testIsolation demonstrates proper test isolation.
func Example_testIsolation() {
	// Test 1: Success scenario
	mock1 := plugin.NewMockPlugin()
	mock1.ConfigureScenario(plugin.ScenarioSuccess)
	// ... test with mock1 ...

	// Test 2: Error scenario (completely independent)
	mock2 := plugin.NewMockPlugin()
	mock2.SetError("GetProjectedCost", plugin.ErrorTimeout)
	// ... test with mock2 ...

	// Each mock is isolated and doesn't affect the other
}

// Example_dynamicConfiguration demonstrates changing configuration during a test.
func Example_dynamicConfiguration() {
	mock := plugin.NewMockPlugin()

	// Start with normal costs
	mock.ConfigureScenario(plugin.ScenarioSuccess)
	// ... test normal behavior ...

	// Change to high costs
	mock.ConfigureScenario(plugin.ScenarioHighCost)
	// ... test high cost alerts ...

	// Change to errors
	mock.SetError("GetActualCost", plugin.ErrorUnavailable)
	// ... test error handling ...
}

// TestExamplesCompile verifies all examples compile and run without panics.
func TestExamplesCompile(t *testing.T) {
	// Basic usage
	Example_basicUsage()

	// Scenarios
	Example_scenarioSuccess()
	Example_scenarioPartialData()
	Example_scenarioHighCost()
	Example_scenarioZeroCost()
	Example_scenarioMultiCurrency()

	// Error injection
	Example_errorTimeout()
	Example_errorProtocol()
	Example_errorInvalidData()
	Example_errorUnavailable()

	// Performance
	Example_latencySimulation()

	// Advanced
	Example_combinedConfiguration()
	Example_customResponse()
	Example_actualCostResponse()
	Example_reset()
	Example_testIsolation()
	Example_dynamicConfiguration()

	// All examples should compile and run
	assert.True(t, true, "All examples compiled and ran successfully")
}
