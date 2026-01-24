package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TestGetProjectedCost_WithPlugin tests projected cost calculation using a mock plugin.
func TestGetProjectedCost_WithPlugin(t *testing.T) {
	// Setup mock plugin server
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	// Create plugin client
	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	// Create engine with plugin client
	eng := engine.New([]*pluginhost.Client{client}, nil)

	// Test data
	resources := []engine.ResourceDescriptor{
		{
			Type:     "aws:ec2/instance:Instance",
			ID:       "i-1234567890abcdef0",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
	}

	// Execute
	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	// Verify
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "aws:ec2/instance:Instance", result.ResourceType)
	assert.Equal(t, "i-1234567890abcdef0", result.ResourceID)
	assert.Equal(t, "test-plugin", result.Adapter)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, 7.30, result.Monthly)
	assert.Equal(t, 0.01, result.Hourly)
	assert.Contains(t, result.Notes, "t3.micro")
}

// TestGetProjectedCost_MultipleResources tests calculation for multiple resources.
func TestGetProjectedCost_MultipleResources(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
		{Type: "aws:s3/bucket:Bucket", ID: "bucket-001", Provider: "aws"},
		{Type: "aws:rds/instance:Instance", ID: "db-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify all resources got results
	for i, result := range results {
		assert.Equal(t, resources[i].ID, result.ResourceID)
		assert.Equal(t, "test-plugin", result.Adapter)
		assert.Equal(t, "USD", result.Currency)
		assert.Greater(t, result.Monthly, 0.0)
	}
}

// TestGetProjectedCost_NoPlugin tests fallback when no plugin available.
func TestGetProjectedCost_NoPlugin(t *testing.T) {
	// Create engine with no plugins and no spec loader
	eng := engine.New(nil, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 1)

	// Should return placeholder result with "none" adapter
	result := results[0]
	assert.Equal(t, "none", result.Adapter)
	assert.Equal(t, 0.0, result.Monthly)
	assert.Equal(t, 0.0, result.Hourly)
	assert.Contains(t, result.Notes, "No pricing information available")
}

// TestGetProjectedCost_PluginError tests handling of plugin errors.
func TestGetProjectedCost_PluginError(t *testing.T) {
	helper := plugin.NewTestHelper(t)

	// Configure plugin to return errors
	helper.SetError("GetProjectedCost", plugin.ErrorTimeout)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err) // Engine doesn't fail on plugin errors
	require.Len(t, results, 1)

	// Should fall back to "none" adapter
	result := results[0]
	assert.Equal(t, "none", result.Adapter)
}

// TestGetProjectedCost_MultiPluginSupport tests multiple plugins.
func TestGetProjectedCost_MultiPluginSupport(t *testing.T) {
	// Create first plugin
	helper1 := plugin.NewTestHelper(t)
	helper1.Plugin().SetProjectedCostResponse("aws:ec2/instance:Instance",
		plugin.QuickResponse("USD", 10.0, 0.014))

	// Create second plugin
	helper2 := plugin.NewTestHelper(t)
	helper2.Plugin().SetProjectedCostResponse("aws:ec2/instance:Instance",
		plugin.QuickResponse("USD", 12.0, 0.016))

	conn1 := helper1.Dial()
	conn2 := helper2.Dial()

	clients := []*pluginhost.Client{
		{Name: "plugin1", API: proto.NewCostSourceClient(conn1)},
		{Name: "plugin2", API: proto.NewCostSourceClient(conn2)},
	}

	eng := engine.New(clients, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 2) // Should get results from both plugins

	// Verify both plugins contributed
	adapters := make(map[string]bool)
	for _, result := range results {
		adapters[result.Adapter] = true
	}
	assert.True(t, adapters["plugin1"])
	assert.True(t, adapters["plugin2"])
}

// TestGetProjectedCost_PartialData tests scenario with missing data for some resources.
func TestGetProjectedCost_PartialData(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioPartialData)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
		{Type: "aws:s3/bucket:Bucket", ID: "bucket-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 2)

	// First resource should have data
	assert.Equal(t, "test-plugin", results[0].Adapter)
	assert.Greater(t, results[0].Monthly, 0.0)

	// Second resource should fall back to "none"
	assert.Equal(t, "none", results[1].Adapter)
	assert.Equal(t, 0.0, results[1].Monthly)
}

// TestGetProjectedCost_HighCost tests high-cost scenario.
func TestGetProjectedCost_HighCost(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioHighCost)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "test-plugin", result.Adapter)
	assert.Greater(t, result.Monthly, 1000.0) // High cost scenario
	assert.Contains(t, result.Notes, "GPU")   // Should indicate expensive resource
}

// TestGetProjectedCost_ZeroCost tests free-tier resources.
func TestGetProjectedCost_ZeroCost(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioZeroCost)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:s3/bucket:Bucket", ID: "bucket-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "test-plugin", result.Adapter)
	assert.Equal(t, 0.0, result.Monthly)
	assert.Equal(t, 0.0, result.Hourly)
	assert.Contains(t, result.Notes, "Free tier")
}

// TestGetProjectedCost_MultiCurrency tests mixed currency handling.
func TestGetProjectedCost_MultiCurrency(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioMultiCurrency)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
		{Type: "aws:rds/instance:Instance", ID: "db-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 2)

	// Verify different currencies
	currencies := make(map[string]bool)
	for _, result := range results {
		currencies[result.Currency] = true
	}
	assert.True(t, len(currencies) > 1, "Should have multiple currencies")
}

// TestGetProjectedCost_WithBreakdown tests cost breakdown data.
func TestGetProjectedCost_WithBreakdown(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.ConfigureScenario(plugin.ScenarioSuccess)

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:lambda/function:Function", ID: "func-001", Provider: "aws"},
	}

	ctx := context.Background()
	results, err := eng.GetProjectedCost(ctx, resources)

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.NotNil(t, result.Breakdown)
	// Mock plugin returns unit_price breakdown
	assert.Contains(t, result.Breakdown, "unit_price")

	// Verify breakdown exists and has values
	assert.Greater(t, len(result.Breakdown), 0)
}

// TestGetActualCost_WithPlugin tests actual cost retrieval.
func TestGetActualCost_WithPlugin(t *testing.T) {
	helper := plugin.NewTestHelper(t)

	// Configure actual cost response
	helper.Plugin().ConfigureActualCostScenario("i-1234567890abcdef0", 45.67, map[string]float64{
		"compute": 30.00,
		"storage": 15.67,
	})

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-1234567890abcdef0", Provider: "aws"},
	}

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	ctx := context.Background()
	results, err := eng.GetActualCost(ctx, resources, from, to)

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "test-plugin", result.Adapter)
	assert.Equal(t, 45.67, result.TotalCost)
	assert.NotEmpty(t, result.CostPeriod)
	assert.Equal(t, from, result.StartDate)
	assert.Equal(t, to, result.EndDate)
	assert.Contains(t, result.Breakdown, "compute")
	assert.Contains(t, result.Breakdown, "storage")
}

// TestGetActualCost_NoPlugin tests actual cost with no plugin.
func TestGetActualCost_NoPlugin(t *testing.T) {
	eng := engine.New(nil, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	ctx := context.Background()
	results, err := eng.GetActualCost(ctx, resources, from, to)

	require.NoError(t, err)
	require.Len(t, results, 1)

	// Should return placeholder with "none" adapter
	result := results[0]
	assert.Equal(t, "none", result.Adapter)
	assert.Equal(t, 0.0, result.TotalCost)
	assert.Contains(t, result.Notes, "No actual cost data")
}

// TestGetActualCost_TimeRange tests time range handling.
func TestGetActualCost_TimeRange(t *testing.T) {
	helper := plugin.NewTestHelper(t)
	helper.Plugin().ConfigureActualCostScenario("i-001", 100.0, map[string]float64{
		"compute": 100.0,
	})

	conn := helper.Dial()
	client := &pluginhost.Client{
		Name: "test-plugin",
		API:  proto.NewCostSourceClient(conn),
	}

	eng := engine.New([]*pluginhost.Client{client}, nil)

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "i-001", Provider: "aws"},
	}

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 10, 23, 59, 59, 0, time.UTC)

	ctx := context.Background()
	results, err := eng.GetActualCost(ctx, resources, from, to)

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, 100.0, result.TotalCost)
	// Period can be in days or weeks depending on duration
	assert.NotEmpty(t, result.CostPeriod)

	// Verify monthly projection is calculated
	assert.Greater(t, result.Monthly, 0.0)
	assert.Greater(t, result.Hourly, 0.0)
}
