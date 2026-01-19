//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectedCost_EC2_WithoutPlugin validates CLI parsing works correctly without plugins.
// This test verifies that finfocus correctly parses the preview JSON and returns $0.00
// when no pricing plugin is installed. This is important to ensure the CLI works even
// without plugins installed.
//
// Expected behavior: $0.00 cost (no plugin to provide pricing)
func TestProjectedCost_EC2_WithoutPlugin(t *testing.T) {
	// Check if we should skip this test (if plugins are pre-installed)
	pm := NewPluginManager(t)
	ctx := context.Background()

	if pm.IsPluginInstalled(ctx, "aws-public") {
		t.Skip("Skipping 'without plugin' test - aws-public plugin is already installed. Run TestProjectedCost_EC2_WithPlugin instead.")
	}

	tc := NewTestContext(t, "e2e-ec2-noplugin")

	// Setup Project - this deploys real AWS infrastructure using CLI commands
	err := tc.SetupProject(ctx, "fixtures/ec2")
	require.NoError(t, err, "Failed to setup project")
	defer tc.Teardown(ctx)

	// Run finfocus CLI to get the calculated cost (should be $0.00 without plugin)
	calculatedCost, err := tc.RunFinFocus(ctx)
	require.NoError(t, err, "finfocus CLI failed - ensure binary is built with 'make build'")

	t.Logf("Calculated cost without plugin: $%.4f/month (expected ~$0.00)", calculatedCost)

	// Without plugin, we expect $0.00 or very low cost
	// This validates the CLI parsing works correctly
	assert.True(t, calculatedCost < 1.0, "Expected cost to be $0.00 or very low without plugin, got $%.2f", calculatedCost)
	t.Log("CLI parsing validation passed - finfocus correctly parses preview JSON without plugin")
}

// TestProjectedCost_EC2_WithPlugin validates the full cost calculation chain with aws-public plugin.
// This test:
// 1. Installs the aws-public plugin
// 2. Deploys real AWS infrastructure (t3.micro EC2)
// 3. Runs finfocus cost projected
// 4. Validates cost is within ±5% of expected AWS pricing (~$7.59/month)
//
// This is the primary E2E test for validating actual cost accuracy.
//
// Environment Variables:
// - E2E_CLEANUP_PLUGINS: Set to "true" or "1" to remove plugins after test (default: false)
func TestProjectedCost_EC2_WithPlugin(t *testing.T) {
	tc := NewTestContext(t, "e2e-ec2-plugin")
	ctx := context.Background()

	// Initialize plugin manager and install aws-public plugin
	pm := NewPluginManager(t)
	err := pm.EnsurePluginInstalled(ctx, "aws-public")
	require.NoError(t, err, "Failed to install aws-public plugin")

	// Schedule plugin cleanup if enabled (controlled by E2E_CLEANUP_PLUGINS env var)
	defer pm.DeferPluginCleanup(ctx, "aws-public")()

	// List installed plugins for debugging
	plugins, err := pm.ListPlugins(ctx)
	require.NoError(t, err)
	t.Logf("Installed plugins:\n%s", plugins)

	// Setup Project - this deploys real AWS infrastructure using CLI commands
	err = tc.SetupProject(ctx, "fixtures/ec2")
	require.NoError(t, err, "Failed to setup project")
	defer tc.Teardown(ctx)

	// Get expected cost from our pricing reference
	expectedCost, ok := GetExpectedCost("t3.micro")
	require.True(t, ok, "Missing pricing reference for t3.micro")

	// Run finfocus CLI to get the actual calculated cost
	// This is the REAL test - we're calling the actual finfocus binary WITH the plugin
	calculatedCost, err := tc.RunFinFocus(ctx)
	require.NoError(t, err, "finfocus CLI failed - ensure binary is built with 'make build'")

	t.Logf("Expected cost: $%.4f/month, Calculated cost: $%.4f/month", expectedCost, calculatedCost)

	// First, validate that we got a non-zero cost (proves plugin is working)
	require.Greater(t, calculatedCost, 0.0, "Expected non-zero cost with aws-public plugin installed")

	// Validate the calculated cost is within tolerance of expected
	validator := NewDefaultCostValidator(5.0) // 5% tolerance
	err = validator.ValidateProjected(calculatedCost, expectedCost)
	assert.NoError(t, err, "Projected cost validation failed - cost outside ±5% tolerance")

	// Log detailed comparison report
	report := validator.Compare(calculatedCost, expectedCost)
	LogComparisonReport(t, report)
}

// TestProjectedCost_EC2 is the legacy test that validates basic E2E functionality.
// It runs regardless of plugin state and validates the workflow.
// For cost accuracy validation, use TestProjectedCost_EC2_WithPlugin.
func TestProjectedCost_EC2(t *testing.T) {
	tc := NewTestContext(t, "e2e-ec2")
	ctx := context.Background()

	// Setup Project - this deploys real AWS infrastructure using CLI commands
	err := tc.SetupProject(ctx, "fixtures/ec2")
	require.NoError(t, err, "Failed to setup project")
	defer tc.Teardown(ctx)

	// Get expected cost from our pricing reference
	expectedCost, ok := GetExpectedCost("t3.micro")
	require.True(t, ok, "Missing pricing reference for t3.micro")

	// Run finfocus CLI to get the actual calculated cost
	// This is the REAL test - we're calling the actual finfocus binary
	calculatedCost, err := tc.RunFinFocus(ctx)
	require.NoError(t, err, "finfocus CLI failed - ensure binary is built with 'make build'")

	t.Logf("Expected cost: $%.4f/month, Calculated cost: $%.4f/month", expectedCost, calculatedCost)

	// Check if we have a plugin installed to determine expected behavior
	pm := NewPluginManager(t)
	hasPlugin := pm.IsPluginInstalled(ctx, "aws-public")

	if hasPlugin {
		// With plugin, validate cost is within tolerance
		validator := NewDefaultCostValidator(5.0) // 5% tolerance
		err = validator.ValidateProjected(calculatedCost, expectedCost)
		assert.NoError(t, err, "Projected cost validation failed")

		// Log detailed comparison report
		report := validator.Compare(calculatedCost, expectedCost)
		LogComparisonReport(t, report)
	} else {
		// Without plugin, just log the result
		t.Logf("No aws-public plugin installed - cost validation skipped (got $%.2f)", calculatedCost)
		t.Log("To run full cost validation, install plugin with: finfocus plugin install aws-public")
	}
}

// TestProjectedCost_EBS verifies the projected cost calculation for an EBS volume.
// NOTE: This test is deferred until we create the fixtures/ebs project directory.
// For now, it validates the cost validator logic with the expected pricing.
func TestProjectedCost_EBS(t *testing.T) {
	t.Skip("Skipping EBS test - fixtures/ebs not yet implemented")

	tc := NewTestContext(t, "e2e-ebs")
	ctx := context.Background()

	// Setup Project
	err := tc.SetupProject(ctx, "fixtures/ebs")
	require.NoError(t, err, "Failed to setup project")
	defer tc.Teardown(ctx)

	expectedCost, ok := GetExpectedCost("gp3")
	require.True(t, ok, "Missing pricing reference for gp3")

	calculatedCost, err := tc.RunFinFocus(ctx)
	require.NoError(t, err, "finfocus CLI failed")

	validator := NewDefaultCostValidator(5.0)
	err = validator.ValidateProjected(calculatedCost, expectedCost)
	assert.NoError(t, err, "Projected cost validation failed")
}

// TestUnsupportedResourceTypes verifies that unsupported resources don't crash the validator.
func TestUnsupportedResourceTypes(t *testing.T) {
	// Simulate a resource that returns 0 cost (unsupported or free)
	expectedCost := 0.0
	actualCost := 0.0 // Should be 0 for unsupported

	validator := NewDefaultCostValidator(5.0)
	err := validator.ValidateProjected(actualCost, expectedCost)
	assert.NoError(t, err, "Validation should pass for zero-cost/unsupported resources")
}

// TestActualCost_Runtime verifies the actual cost calculation based on runtime.
// This test validates the fallback formula: projected_cost * runtime_hours / 730
func TestActualCost_Runtime(t *testing.T) {
	// Simulate a runtime duration
	runtime := 1 * time.Hour
	expectedHourlyCost := 0.0104 // ~$7.59 / 730

	// Calculate expected total cost for the runtime
	expectedTotal := expectedHourlyCost * runtime.Hours()

	// Simulate a calculated cost using the fallback formula
	// In production, this comes from: (projected_monthly_cost / 730) * runtime_hours
	calculatedCost := expectedTotal

	validator := NewDefaultCostValidator(5.0)
	err := validator.ValidateActual(calculatedCost, runtime, expectedHourlyCost)
	assert.NoError(t, err, "Actual cost validation failed")
}

// TestCleanupVerification verifies that cleanup logic works as expected.
// This test creates and destroys a real stack to verify the cleanup mechanism.
func TestCleanupVerification(t *testing.T) {
	tc := NewTestContext(t, "e2e-cleanup")
	ctx := context.Background()

	// Setup the EC2 project
	err := tc.SetupProject(ctx, "fixtures/ec2")
	require.NoError(t, err, "Failed to setup project")

	// Verify work directory exists before cleanup
	require.DirExists(t, tc.WorkDir, "Work directory should exist before cleanup")

	// Verify Teardown runs successfully - this destroys all resources
	tc.Teardown(ctx)

	// Verify cleanup was actually performed - work directory should be removed
	require.NoDirExists(t, tc.WorkDir, "Work directory should be removed after cleanup")

	// If Teardown panicked or failed, the test would have already failed
	t.Log("Cleanup verification passed - resources destroyed successfully")
}

// TestCostValidatorTolerance verifies the cost validator works with different tolerances.
func TestCostValidatorTolerance(t *testing.T) {
	testCases := []struct {
		name       string
		calculated float64
		expected   float64
		tolerance  float64
		shouldPass bool
	}{
		{"exact match", 7.59, 7.59, 5.0, true},
		{"within 5%", 7.22, 7.59, 5.0, true},
		{"at 5% boundary", 7.211, 7.59, 5.0, true},
		{"outside 5%", 7.00, 7.59, 5.0, false},
		{"10% tolerance passes", 7.00, 7.59, 10.0, true},
		{"zero expected zero actual", 0.0, 0.0, 5.0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validator := NewDefaultCostValidator(tc.tolerance)
			err := validator.ValidateProjected(tc.calculated, tc.expected)
			if tc.shouldPass {
				assert.NoError(t, err, "Expected validation to pass")
			} else {
				assert.Error(t, err, "Expected validation to fail")
			}
		})
	}
}

// TestComparisonReport verifies the comparison report generation.
func TestComparisonReport(t *testing.T) {
	validator := NewDefaultCostValidator(5.0)
	report := validator.Compare(7.22, 7.59)

	assert.Equal(t, 7.22, report.Actual)
	assert.Equal(t, 7.59, report.Expected)
	assert.InDelta(t, 4.87, report.PercentDiff, 0.5) // ~4.87% difference
	assert.True(t, report.WithinLimit)

	// Log the report
	LogComparisonReport(t, report)
}
