package plugin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TestNewMockPlugin verifies that a new mock plugin has correct default configuration.
func TestNewMockPlugin(t *testing.T) {
	mock := plugin.NewMockPlugin()

	require.NotNil(t, mock)

	config := mock.GetConfig()
	assert.NotNil(t, config.ProjectedCostResponses)
	assert.NotNil(t, config.ActualCostResponses)
	assert.Empty(t, config.ProjectedCostResponses)
	assert.Empty(t, config.ActualCostResponses)
	assert.Equal(t, plugin.ErrorNone, config.ErrorType)
	assert.Empty(t, config.ErrorMethod)
	assert.Equal(t, 0, config.LatencyMS)
}

// TestScenarioSuccess verifies the success scenario configures typical resource responses.
func TestScenarioSuccess(t *testing.T) {
	mock := plugin.NewMockPlugin()
	mock.ConfigureScenario(plugin.ScenarioSuccess)

	config := mock.GetConfig()

	// Should have multiple resource types configured
	assert.NotEmpty(t, config.ProjectedCostResponses)
	assert.Contains(t, config.ProjectedCostResponses, "aws:ec2/instance:Instance")
	assert.Contains(t, config.ProjectedCostResponses, "aws:s3/bucket:Bucket")
	assert.Contains(t, config.ProjectedCostResponses, "aws:rds/instance:Instance")
	assert.Contains(t, config.ProjectedCostResponses, "aws:lambda/function:Function")

	// Verify EC2 instance response
	ec2Response := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	require.NotNil(t, ec2Response)
	assert.Equal(t, "USD", ec2Response.Currency)
	assert.Greater(t, ec2Response.MonthlyCost, 0.0)
	assert.Greater(t, ec2Response.HourlyCost, 0.0)
	assert.NotEmpty(t, ec2Response.Notes)
	assert.Contains(t, ec2Response.CostBreakdown, "compute")

	// Verify S3 bucket response
	s3Response := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]
	require.NotNil(t, s3Response)
	assert.Equal(t, "USD", s3Response.Currency)
	assert.Greater(t, s3Response.MonthlyCost, 0.0)
	assert.Contains(t, s3Response.CostBreakdown, "storage")

	// Verify Lambda function response has breakdown
	lambdaResponse := config.ProjectedCostResponses["aws:lambda/function:Function"]
	require.NotNil(t, lambdaResponse)
	assert.Contains(t, lambdaResponse.CostBreakdown, "compute")
	assert.Contains(t, lambdaResponse.CostBreakdown, "requests")
}

// TestScenarioPartialData verifies partial data scenario with missing resources.
func TestScenarioPartialData(t *testing.T) {
	mock := plugin.NewMockPlugin()
	mock.ConfigureScenario(plugin.ScenarioPartialData)

	config := mock.GetConfig()

	// Should have some resources configured
	assert.Contains(t, config.ProjectedCostResponses, "aws:ec2/instance:Instance")

	// But not all resources (simulating missing data)
	// Note: The implementation intentionally doesn't configure S3 bucket
	assert.NotContains(t, config.ProjectedCostResponses, "aws:s3/bucket:Bucket")
	assert.NotContains(t, config.ProjectedCostResponses, "aws:rds/instance:Instance")
}

// TestScenarioHighCost verifies high-cost scenario with expensive resources.
func TestScenarioHighCost(t *testing.T) {
	mock := plugin.NewMockPlugin()
	mock.ConfigureScenario(plugin.ScenarioHighCost)

	config := mock.GetConfig()

	// Verify expensive EC2 instance
	ec2Response := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	require.NotNil(t, ec2Response)
	assert.Greater(t, ec2Response.MonthlyCost, 1000.0, "High cost scenario should have expensive resources")
	assert.Contains(t, ec2Response.Notes, "GPU", "Should indicate high-end hardware")

	// Verify expensive RDS instance
	rdsResponse := config.ProjectedCostResponses["aws:rds/instance:Instance"]
	require.NotNil(t, rdsResponse)
	assert.Greater(t, rdsResponse.MonthlyCost, 500.0, "RDS should also be expensive")
}

// TestScenarioZeroCost verifies zero-cost scenario for free tier resources.
func TestScenarioZeroCost(t *testing.T) {
	mock := plugin.NewMockPlugin()
	mock.ConfigureScenario(plugin.ScenarioZeroCost)

	config := mock.GetConfig()

	// Verify S3 bucket is free
	s3Response := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]
	require.NotNil(t, s3Response)
	assert.Equal(t, 0.0, s3Response.MonthlyCost)
	assert.Equal(t, 0.0, s3Response.HourlyCost)
	assert.NotEmpty(t, s3Response.Notes, "Should have notes explaining zero cost")

	// Verify Lambda is free
	lambdaResponse := config.ProjectedCostResponses["aws:lambda/function:Function"]
	require.NotNil(t, lambdaResponse)
	assert.Equal(t, 0.0, lambdaResponse.MonthlyCost)
	assert.Equal(t, 0.0, lambdaResponse.HourlyCost)
}

// TestScenarioMultiCurrency verifies mixed currency scenario.
func TestScenarioMultiCurrency(t *testing.T) {
	mock := plugin.NewMockPlugin()
	mock.ConfigureScenario(plugin.ScenarioMultiCurrency)

	config := mock.GetConfig()

	// Verify different currencies
	ec2Response := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	require.NotNil(t, ec2Response)
	assert.Equal(t, "USD", ec2Response.Currency)

	rdsResponse := config.ProjectedCostResponses["aws:rds/instance:Instance"]
	require.NotNil(t, rdsResponse)
	assert.Equal(t, "EUR", rdsResponse.Currency)

	// Verify they have different currencies
	assert.NotEqual(t, ec2Response.Currency, rdsResponse.Currency, "Should have mixed currencies")
}

// TestSetProjectedCostResponse verifies custom response configuration.
func TestSetProjectedCostResponse(t *testing.T) {
	mock := plugin.NewMockPlugin()

	customResponse := plugin.QuickResponse("USD", 99.99, 0.137)
	mock.SetProjectedCostResponse("custom:resource:Type", customResponse)

	config := mock.GetConfig()
	assert.Contains(t, config.ProjectedCostResponses, "custom:resource:Type")
	assert.Equal(t, customResponse, config.ProjectedCostResponses["custom:resource:Type"])
}

// TestSetActualCostResponse verifies actual cost response configuration.
func TestSetActualCostResponse(t *testing.T) {
	mock := plugin.NewMockPlugin()

	actualResponse := plugin.QuickActualResponse("USD", 150.50)
	mock.SetActualCostResponse("resource-id-123", actualResponse)

	config := mock.GetConfig()
	assert.Contains(t, config.ActualCostResponses, "resource-id-123")
	assert.Equal(t, actualResponse, config.ActualCostResponses["resource-id-123"])
}

// TestConfigureActualCostScenario verifies actual cost scenario setup.
func TestConfigureActualCostScenario(t *testing.T) {
	mock := plugin.NewMockPlugin()

	breakdown := map[string]float64{
		"compute": 100.00,
		"storage": 50.00,
	}
	mock.ConfigureActualCostScenario("resource-456", 150.00, breakdown)

	config := mock.GetConfig()
	actualResponse := config.ActualCostResponses["resource-456"]
	require.NotNil(t, actualResponse)
	assert.Equal(t, "USD", actualResponse.Currency)
	assert.Equal(t, 150.00, actualResponse.TotalCost)
	assert.Equal(t, breakdown, actualResponse.CostBreakdown)
}

// TestReset verifies that reset clears all configuration.
func TestReset(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Configure some responses and errors
	mock.ConfigureScenario(plugin.ScenarioSuccess)
	mock.SetError("GetProjectedCost", plugin.ErrorTimeout)
	mock.SetLatency(500)

	// Verify configuration is set
	config := mock.GetConfig()
	assert.NotEmpty(t, config.ProjectedCostResponses)
	assert.Equal(t, plugin.ErrorTimeout, config.ErrorType)
	assert.Equal(t, 500, config.LatencyMS)

	// Reset
	mock.Reset()

	// Verify everything is cleared
	config = mock.GetConfig()
	assert.Empty(t, config.ProjectedCostResponses)
	assert.Empty(t, config.ActualCostResponses)
	assert.Equal(t, plugin.ErrorNone, config.ErrorType)
	assert.Empty(t, config.ErrorMethod)
	assert.Equal(t, 0, config.LatencyMS)
}

// TestConfigure verifies that Configure sets full configuration.
func TestConfigure(t *testing.T) {
	mock := plugin.NewMockPlugin()

	customConfig := plugin.MockConfig{
		ProjectedCostResponses: map[string]*proto.CostResult{
			"test:resource:Type": {
				Currency:    "EUR",
				MonthlyCost: 25.00,
				HourlyCost:  0.034,
				CostBreakdown: map[string]float64{
					"compute": 25.00,
				},
			},
		},
		ActualCostResponses: map[string]*proto.ActualCostResult{
			"test-id": {
				Currency:  "GBP",
				TotalCost: 75.00,
				CostBreakdown: map[string]float64{
					"total": 75.00,
				},
			},
		},
		ErrorType:   plugin.ErrorProtocol,
		ErrorMethod: "GetActualCost",
		LatencyMS:   250,
	}

	mock.Configure(customConfig)

	config := mock.GetConfig()
	assert.Equal(t, customConfig.ErrorType, config.ErrorType)
	assert.Equal(t, customConfig.ErrorMethod, config.ErrorMethod)
	assert.Equal(t, customConfig.LatencyMS, config.LatencyMS)
	assert.Len(t, config.ProjectedCostResponses, 1)
	assert.Len(t, config.ActualCostResponses, 1)
}

// TestQuickResponse verifies helper function for creating responses.
func TestQuickResponse(t *testing.T) {
	response := plugin.QuickResponse("USD", 100.00, 0.137)

	require.NotNil(t, response)
	assert.Equal(t, "USD", response.Currency)
	assert.Equal(t, 100.00, response.MonthlyCost)
	assert.Equal(t, 0.137, response.HourlyCost)
	assert.Contains(t, response.CostBreakdown, "total")
	assert.Equal(t, 100.00, response.CostBreakdown["total"])
}

// TestQuickActualResponse verifies helper function for actual cost responses.
func TestQuickActualResponse(t *testing.T) {
	response := plugin.QuickActualResponse("EUR", 250.50)

	require.NotNil(t, response)
	assert.Equal(t, "EUR", response.Currency)
	assert.Equal(t, 250.50, response.TotalCost)
	assert.Contains(t, response.CostBreakdown, "total")
	assert.Equal(t, 250.50, response.CostBreakdown["total"])
}

// TestMultipleScenarioChanges verifies scenario can be changed multiple times.
func TestMultipleScenarioChanges(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Start with success scenario
	mock.ConfigureScenario(plugin.ScenarioSuccess)
	config := mock.GetConfig()
	assert.Contains(t, config.ProjectedCostResponses, "aws:lambda/function:Function")

	// Change to high cost scenario
	mock.ConfigureScenario(plugin.ScenarioHighCost)
	config = mock.GetConfig()
	assert.NotContains(t, config.ProjectedCostResponses, "aws:lambda/function:Function")

	ec2Response := config.ProjectedCostResponses["aws:ec2/instance:Instance"]
	assert.Greater(t, ec2Response.MonthlyCost, 1000.0)

	// Change to zero cost scenario
	mock.ConfigureScenario(plugin.ScenarioZeroCost)
	config = mock.GetConfig()
	s3Response := config.ProjectedCostResponses["aws:s3/bucket:Bucket"]
	assert.Equal(t, 0.0, s3Response.MonthlyCost)
}

// TestResponseIsolation verifies responses don't interfere with each other.
func TestResponseIsolation(t *testing.T) {
	mock := plugin.NewMockPlugin()

	// Set projected cost response
	projectedResponse := plugin.QuickResponse("USD", 50.00, 0.068)
	mock.SetProjectedCostResponse("test:type:A", projectedResponse)

	// Set actual cost response with same key structure
	actualResponse := plugin.QuickActualResponse("USD", 100.00)
	mock.SetActualCostResponse("test:type:A", actualResponse)

	config := mock.GetConfig()

	// Verify both are set independently
	assert.Equal(t, projectedResponse, config.ProjectedCostResponses["test:type:A"])
	assert.Equal(t, actualResponse, config.ActualCostResponses["test:type:A"])
	assert.NotEqual(t, projectedResponse.MonthlyCost, actualResponse.TotalCost)
}
