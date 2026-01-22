//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiRegion_Actual_USEast1 tests actual cost calculation for us-east-1 region.
func TestMultiRegion_Actual_USEast1(t *testing.T) {
	if shouldSkipActualCosts(t) || shouldSkipRegion(t, "us-east-1") {
		t.Skip("Skipping us-east-1 actual cost test")
	}
	testMultiRegionActual(t, "us-east-1")
}

// TestMultiRegion_Actual_EUWest1 tests actual cost calculation for eu-west-1 region.
func TestMultiRegion_Actual_EUWest1(t *testing.T) {
	if shouldSkipActualCosts(t) || shouldSkipRegion(t, "eu-west-1") {
		t.Skip("Skipping eu-west-1 actual cost test")
	}
	testMultiRegionActual(t, "eu-west-1")
}

// TestMultiRegion_Actual_APNortheast1 tests actual cost calculation for ap-northeast-1 region.
func TestMultiRegion_Actual_APNortheast1(t *testing.T) {
	if shouldSkipActualCosts(t) || shouldSkipRegion(t, "ap-northeast-1") {
		t.Skip("Skipping ap-northeast-1 actual cost test")
	}
	testMultiRegionActual(t, "ap-northeast-1")
}

// testMultiRegionActual is the common test implementation for actual costs across regions.
func testMultiRegionActual(t *testing.T, region string) {
	t.Parallel()

	startTime := time.Now()

	// Log plugin versions at test start
	LogPluginVersions(t)

	// Check dependencies
	checkDependencies(t)

	// Setup test configuration
	config := RegionTestConfig{
		Region:            region,
		FixturePath:       getFixturePath(t, region),
		ExpectedCostsPath: getExpectedCostsPath(t, region),
		Tolerance:         getTolerance(),
		Timeout:           getTimeout(),
		PluginVersion:     "0.1.0",
	}

	// Validate configuration
	require.NoError(t, config.Validate(), "Invalid test configuration")

	// Load expected costs for actual cost type
	expectedCosts, err := loadExpectedCosts(config.ExpectedCostsPath, "actual")
	if err != nil || len(expectedCosts) == 0 {
		// Fallback to projected costs if actual costs not defined
		expectedCosts, err = loadExpectedCosts(config.ExpectedCostsPath, "projected")
		require.NoError(t, err, "Failed to load expected costs")
	}
	require.NotEmpty(t, expectedCosts, "Expected costs cannot be empty")

	// Deploy resources using Pulumi Automation API (requires #177)
	stateFile, cleanup := deployPulumiResources(t, config.FixturePath, region)
	defer cleanup()

	// Run finfocus cost actual with retry logic (T020)
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	var results []CostResult
	err = RetryWithBackoff(ctx, 3, func() error {
		var retryErr error
		results, retryErr = runActualCostCommand(ctx, t, stateFile)
		return retryErr
	})
	require.NoError(t, err, "Failed to get actual costs after retries")

	// Validate plugin loading
	validatePluginLoaded(t, "aws-public")

	// Validate region-specific plugin (T025)
	validateRegionSpecificPlugin(t, region)

	// Validate costs with tolerance
	validationResults := validateCostsWithTolerance(t, results, expectedCosts, config.Tolerance)

	// Check execution time (T030)
	executionTime := time.Since(startTime)
	t.Logf("Test execution time for %s: %v", region, executionTime)

	// Performance validation: actual cost tests should complete in <5 minutes
	if executionTime > 5*time.Minute {
		t.Logf("WARNING: Test execution time (%v) exceeded 5-minute target", executionTime)
	}

	// Assert all validations passed
	assert.Equal(t, len(expectedCosts), len(validationResults), "Number of results should match expectations")

	failureCount := 0
	for _, result := range validationResults {
		if !result.WithinTolerance {
			t.Errorf("Cost validation failed for %s: actual=$%.2f, expected=[$%.2f, $%.2f], variance=%.2f%%",
				result.ResourceName, result.ActualCost, result.ExpectedMin, result.ExpectedMax, result.Variance)
			failureCount++
		}
	}

	require.Equal(t, 0, failureCount, "All cost validations must pass")
}

// runActualCostCommand executes the finfocus cost actual command and returns parsed results.
func runActualCostCommand(ctx context.Context, t *testing.T, stateFile string) ([]CostResult, error) {
	t.Helper()

	binary := findFinFocusBinary()
	if binary == "" {
		return nil, fmt.Errorf("finfocus binary not found")
	}

	cmd := exec.CommandContext(ctx, binary, "cost", "actual", "--state-file", stateFile, "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
		}
		if _, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("command failed with output: %s", string(output))
		}
		return nil, fmt.Errorf("command execution failed: %w (output: %s)", err, string(output))
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (output: %s)", err, string(output))
	}

	// Extract cost results and set CostType to "actual"
	results := parseCostResults(t, result)
	for i := range results {
		results[i].CostType = "actual"
	}
	return results, nil
}

// deployPulumiResources deploys resources using Pulumi Automation API.
// Returns the path to the state file and a cleanup function.
// NOTE: This requires issue #177 (Pulumi Automation API) to be implemented.
func deployPulumiResources(t *testing.T, fixturePath string, region string) (string, func()) {
	t.Helper()

	// Check if we should use a static state file for testing
	stateFile := filepath.Join(fixturePath, "test-state.json")
	if _, err := os.Stat(stateFile); err == nil {
		t.Logf("Using existing state file: %s", stateFile)
		return stateFile, func() {}
	}

	// Deployment requires Pulumi Automation API (issue #177)
	t.Logf("Resource deployment requires Pulumi Automation API (issue #177)")

	// For now, skip if no state file exists
	t.Skip("Actual cost tests require Pulumi Automation API (issue #177) or pre-generated state file")

	return "", func() {}
}

// checkDependencies verifies that required dependencies are available.
func checkDependencies(t *testing.T) {
	t.Helper()

	// Allow skipping dependency check via environment variable
	if os.Getenv("FINFOCUS_E2E_SKIP_DEPENDENCY_CHECK") == "true" {
		return
	}

	// Check if Pulumi CLI is available (required for Automation API)
	_, err := exec.LookPath("pulumi")
	if err != nil {
		t.Skip("Pulumi CLI not found in PATH - skipping actual cost tests (install from https://www.pulumi.com/docs/get-started/install/)")
	}

	t.Logf("Note: Actual cost tests depend on issue #177 (Pulumi Automation API) and issue #24 (GetActualCost fallback)")
}

// shouldSkipActualCosts checks if actual cost tests should be skipped.
func shouldSkipActualCosts(t *testing.T) bool {
	t.Helper()
	return os.Getenv("FINFOCUS_E2E_SKIP_ACTUAL_COSTS") == "true"
}
