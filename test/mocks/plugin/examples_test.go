package plugin_test

import (
	"fmt"
	"sort"

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
	config := mock.GetConfig()
	resp := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	fmt.Println(resp.Currency, resp.MonthlyCost, resp.HourlyCost)

	// Output: USD 7.3 0.01
}

// Example_scenarioSuccess demonstrates the success scenario with realistic costs.
func Example_scenarioSuccess() {
	mock := plugin.NewMockPlugin()

	// Configure typical AWS resources
	mock.ConfigureScenario(plugin.ScenarioSuccess)

	// Now the mock has responses for EC2, S3, RDS, and Lambda
	config := mock.GetConfig()
	ec2 := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	s3 := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]
	rds := config.ProjectedCostResponses["aws:rds/instance:Instance"]
	lambda := config.ProjectedCostResponses["aws:lambda/function:Function"]

	fmt.Println("EC2:", ec2.Currency, ec2.MonthlyCost)
	fmt.Println("S3:", s3.Currency, s3.MonthlyCost)
	fmt.Println("RDS:", rds.Currency, rds.MonthlyCost)
	fmt.Println("Lambda:", lambda.Currency, lambda.MonthlyCost)

	// Output:
	// EC2: USD 7.3
	// S3: USD 2.3
	// RDS: USD 12.41
	// Lambda: USD 0.2
}

// Example_scenarioPartialData demonstrates testing with missing cost data.
func Example_scenarioPartialData() {
	mock := plugin.NewMockPlugin()

	// Simulate a scenario where only some resources have cost data
	mock.ConfigureScenario(plugin.ScenarioPartialData)

	// Your code should handle missing cost data gracefully
	config := mock.GetConfig()
	ec2, hasEC2 := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	_, hasS3 := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]

	fmt.Println("has EC2:", hasEC2, "has S3:", hasS3)
	fmt.Println("EC2 monthly:", ec2.MonthlyCost)

	// Output:
	// has EC2: true has S3: false
	// EC2 monthly: 7.3
}

// Example_scenarioHighCost demonstrates testing cost warnings with expensive resources.
func Example_scenarioHighCost() {
	mock := plugin.NewMockPlugin()

	// Configure expensive resources for testing cost alerts
	mock.ConfigureScenario(plugin.ScenarioHighCost)

	// Resources will have high monthly costs (>$1000)
	config := mock.GetConfig()
	ec2 := config.ProjectedCostResponses["aws:ec2/instance:Instance"]

	fmt.Println("EC2 monthly:", ec2.MonthlyCost)
	fmt.Println("exceeds $1000:", ec2.MonthlyCost > 1000)

	// Output:
	// EC2 monthly: 2500
	// exceeds $1000: true
}

// Example_scenarioZeroCost demonstrates testing free-tier resources.
func Example_scenarioZeroCost() {
	mock := plugin.NewMockPlugin()

	// Configure free-tier resources
	mock.ConfigureScenario(plugin.ScenarioZeroCost)

	// All costs will be $0.00
	config := mock.GetConfig()
	s3 := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]
	lambda := config.ProjectedCostResponses["aws:lambda/function:Function"]

	fmt.Println("S3 monthly:", s3.MonthlyCost)
	fmt.Println("Lambda monthly:", lambda.MonthlyCost)

	// Output:
	// S3 monthly: 0
	// Lambda monthly: 0
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

	fmt.Println("EC2 currency:", ec2.Currency)
	fmt.Println("RDS currency:", rds.Currency)

	// Output:
	// EC2 currency: USD
	// RDS currency: EUR
}

// Example_errorTimeout demonstrates timeout error injection.
func Example_errorTimeout() {
	mock := plugin.NewMockPlugin()

	// Make GetProjectedCost return a timeout error
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)

	// Your code should handle timeout gracefully
	config := mock.GetConfig()
	fmt.Println("error type:", config.ErrorType)
	fmt.Println("error method:", config.ErrorMethod)

	// Output:
	// error type: timeout
	// error method: GetProjectedCost
}

// Example_errorProtocol demonstrates protocol error injection.
func Example_errorProtocol() {
	mock := plugin.NewMockPlugin()

	// Simulate a gRPC protocol error
	mock.SetError("GetActualCost", plugin.ErrorProtocol)

	// Test your error handling
	config := mock.GetConfig()
	fmt.Println("error type:", config.ErrorType)
	fmt.Println("error method:", config.ErrorMethod)

	// Output:
	// error type: protocol
	// error method: GetActualCost
}

// Example_errorInvalidData demonstrates invalid data error injection.
func Example_errorInvalidData() {
	mock := plugin.NewMockPlugin()

	// Simulate plugin returning invalid data
	mock.SetError("GetProjectedCost", plugin.ErrorInvalidData)

	// Your code should validate plugin responses
	config := mock.GetConfig()
	fmt.Println("error type:", config.ErrorType)

	// Output:
	// error type: invalid_data
}

// Example_errorUnavailable demonstrates service unavailable error injection.
func Example_errorUnavailable() {
	mock := plugin.NewMockPlugin()

	// Simulate plugin service being unavailable
	mock.SetError("GetActualCost", plugin.ErrorUnavailable)

	// Test retry logic or fallback behavior
	config := mock.GetConfig()
	fmt.Println("error type:", config.ErrorType)

	// Output:
	// error type: unavailable
}

// Example_latencySimulation demonstrates performance testing with latency.
func Example_latencySimulation() {
	mock := plugin.NewMockPlugin()

	// Add 100ms latency to simulate network delay
	mock.SetLatency(100)

	// Your performance tests can measure total time
	config := mock.GetConfig()
	fmt.Println("latency (ms):", config.LatencyMS)

	// Output:
	// latency (ms): 100
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
	config := mock.GetConfig()
	_, hasEC2 := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	fmt.Println("has EC2 response:", hasEC2)
	fmt.Println("error:", config.ErrorMethod, config.ErrorType)
	fmt.Println("latency (ms):", config.LatencyMS)

	// Output:
	// has EC2 response: true
	// error: GetActualCost timeout
	// latency (ms): 50
}

// Example_customResponse demonstrates creating custom cost responses.
func Example_customResponse() {
	mock := plugin.NewMockPlugin()

	// Create a custom response for your specific test
	customResponse := plugin.QuickResponse("USD", 99.99, 0.137)
	mock.SetProjectedCostResponse("custom:service:Type", customResponse)

	// Test with your custom resource type
	config := mock.GetConfig()
	resp := config.ProjectedCostResponses["custom:service:Type"]
	fmt.Println(resp.Currency, resp.MonthlyCost, resp.HourlyCost)

	// Output: USD 99.99 0.137
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

	fmt.Println(actual.Currency, actual.TotalCost)
	keys := make([]string, 0, len(actual.CostBreakdown))
	for key := range actual.CostBreakdown {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Println(key, actual.CostBreakdown[key])
	}

	// Output:
	// USD 175
	// compute 100
	// network 25
	// storage 50
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
	fmt.Println("responses:", len(config.ProjectedCostResponses))
	fmt.Println("no error:", config.ErrorType == plugin.ErrorNone)
	fmt.Println("latency (ms):", config.LatencyMS)

	// Output:
	// responses: 0
	// no error: true
	// latency (ms): 0
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
	fmt.Println("mock1 responses:", len(mock1.GetConfig().ProjectedCostResponses))
	fmt.Println("mock1 error:", mock1.GetConfig().ErrorType == plugin.ErrorNone)
	fmt.Println("mock2 responses:", len(mock2.GetConfig().ProjectedCostResponses))
	fmt.Println("mock2 error:", mock2.GetConfig().ErrorType)

	// Output:
	// mock1 responses: 4
	// mock1 error: true
	// mock2 responses: 0
	// mock2 error: timeout
}

// Example_dynamicConfiguration demonstrates changing configuration during a test.
func Example_dynamicConfiguration() {
	mock := plugin.NewMockPlugin()

	// Start with normal costs
	mock.ConfigureScenario(plugin.ScenarioSuccess)
	normal := mock.GetConfig().ProjectedCostResponses["aws:ec2/instance:Instance"].MonthlyCost
	// ... test normal behavior ...

	// Change to high costs
	mock.ConfigureScenario(plugin.ScenarioHighCost)
	high := mock.GetConfig().ProjectedCostResponses["aws:ec2/instance:Instance"].MonthlyCost
	// ... test high cost alerts ...

	// Change to errors
	mock.SetError("GetActualCost", plugin.ErrorUnavailable)
	// ... test error handling ...

	fmt.Println("normal EC2 monthly:", normal)
	fmt.Println("high EC2 monthly:", high)
	fmt.Println("error:", mock.GetConfig().ErrorMethod, mock.GetConfig().ErrorType)

	// Output:
	// normal EC2 monthly: 7.3
	// high EC2 monthly: 2500
	// error: GetActualCost unavailable
}
