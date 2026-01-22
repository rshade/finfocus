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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiRegion_Projected_USEast1 tests projected cost calculation for us-east-1 region.
func TestMultiRegion_Projected_USEast1(t *testing.T) {
	if shouldSkipRegion(t, "us-east-1") {
		t.Skip("Skipping us-east-1 test per FINFOCUS_E2E_REGIONS configuration")
	}
	testMultiRegionProjected(t, "us-east-1")
}

// TestMultiRegion_Projected_EUWest1 tests projected cost calculation for eu-west-1 region.
func TestMultiRegion_Projected_EUWest1(t *testing.T) {
	if shouldSkipRegion(t, "eu-west-1") {
		t.Skip("Skipping eu-west-1 test per FINFOCUS_E2E_REGIONS configuration")
	}
	testMultiRegionProjected(t, "eu-west-1")
}

// TestMultiRegion_Projected_APNortheast1 tests projected cost calculation for ap-northeast-1 region.
func TestMultiRegion_Projected_APNortheast1(t *testing.T) {
	if shouldSkipRegion(t, "ap-northeast-1") {
		t.Skip("Skipping ap-northeast-1 test per FINFOCUS_E2E_REGIONS configuration")
	}
	testMultiRegionProjected(t, "ap-northeast-1")
}

// testMultiRegionProjected is the common test implementation for projected costs across regions.
func testMultiRegionProjected(t *testing.T, region string) {
	t.Parallel()

	startTime := time.Now()

	// Log plugin versions at test start
	LogPluginVersions(t)

	// Setup test configuration
	config := RegionTestConfig{
		Region:            region,
		FixturePath:       getFixturePath(t, region),
		ExpectedCostsPath: getExpectedCostsPath(t, region),
		Tolerance:         getTolerance(),
		Timeout:           getTimeout(),
		PluginVersion:     "0.1.0", // Expected plugin version
	}

	// Validate configuration
	require.NoError(t, config.Validate(), "Invalid test configuration")

	// Load expected costs
	expectedCosts, err := loadExpectedCosts(config.ExpectedCostsPath, "projected")
	require.NoError(t, err, "Failed to load expected costs")
	require.NotEmpty(t, expectedCosts, "Expected costs cannot be empty")

	// Generate Pulumi plan
	planPath, cleanup := generatePulumiPlan(t, config.FixturePath, region)
	defer cleanup()

	// Run finfocus cost projected
	results := runProjectedCostCommand(t, planPath, config.Timeout)

	// Validate plugin loading (T024)
	validatePluginLoaded(t, "aws-public")

	// Validate region-specific plugin (T025)
	validateRegionSpecificPlugin(t, region)

	// Validate costs with tolerance (T019)
	validationResults := validateCostsWithTolerance(t, results, expectedCosts, config.Tolerance)

	// Check execution time
	executionTime := time.Since(startTime)
	t.Logf("Test execution time for %s: %v", region, executionTime)

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

// runProjectedCostCommand executes the finfocus cost projected command and returns parsed results.
func runProjectedCostCommand(t *testing.T, planPath string, timeout time.Duration) []CostResult {
	t.Helper()

	binary := findFinFocusBinary()
	require.NotEmpty(t, binary, "finfocus binary not found")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "cost", "projected", "--pulumi-json", planPath, "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if context was cancelled/timed out
		if ctx.Err() != nil {
			t.Fatalf("Command timed out after %v: %v", timeout, ctx.Err())
		}
		// Include full output in error message
		t.Fatalf("Command failed with output: %s", string(output))
	}

	// Parse JSON output
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

	// Extract cost results
	return parseCostResults(t, result)
}

// parseCostResults extracts cost results from the command output.
func parseCostResults(t *testing.T, output map[string]interface{}) []CostResult {
	t.Helper()

	resources, ok := output["resources"].([]interface{})
	require.True(t, ok, "Output missing 'resources' field")

	var results []CostResult
	for _, res := range resources {
		resource, ok := res.(map[string]interface{})
		require.True(t, ok, "Invalid resource format")

		result := CostResult{
			ResourceName: getStringField(resource, "name"),
			ResourceType: getStringField(resource, "type"),
			Region:       getStringField(resource, "region"),
			MonthlyCost:  getFloatField(resource, "monthly_cost"),
			Currency:     getStringField(resource, "currency"),
			CostType:     "projected",
			Timestamp:    time.Now(),
		}
		results = append(results, result)
	}

	return results
}

// validateCostsWithTolerance validates actual costs against expected ranges.
func validateCostsWithTolerance(t *testing.T, actual []CostResult, expected []ExpectedCost, tolerance float64) []CostValidationResult {
	t.Helper()

	var validations []CostValidationResult

	// Create validator with tolerance
	validator := NewDefaultCostValidator(tolerance)

	// Create a map of expected costs by resource name for easy lookup
	expectedMap := make(map[string]ExpectedCost)
	for _, exp := range expected {
		expectedMap[exp.ResourceName] = exp
	}

	for _, act := range actual {
		exp, found := expectedMap[act.ResourceName]
		if !found {
			t.Logf("Warning: Unexpected resource in results: %s", act.ResourceName)
			continue
		}

		// Use validator to compute validation result
		validation := validator.ValidateCost(act, exp, tolerance)
		validations = append(validations, validation)
	}

	return validations
}

// validatePluginLoaded checks that a specific plugin is loaded (T024).
func validatePluginLoaded(t *testing.T, pluginName string) {
	t.Helper()

	plugins, err := GetLoadedPlugins(t)
	require.NoError(t, err, "Failed to get loaded plugins")

	found := false
	for _, plugin := range plugins {
		if plugin.Name == pluginName {
			found = true
			t.Logf("Plugin %s (version: %s) is loaded", plugin.Name, plugin.Version)
			break
		}
	}

	require.True(t, found, "Plugin %s should be loaded", pluginName)
}

// validateRegionSpecificPlugin validates that region-specific plugin binary is used (T025).
func validateRegionSpecificPlugin(t *testing.T, region string) {
	t.Helper()

	plugins, err := GetLoadedPlugins(t)
	require.NoError(t, err, "Failed to get loaded plugins")

	// Check if any AWS plugin is loaded
	awsPluginFound := false
	for _, plugin := range plugins {
		if strings.Contains(plugin.Name, "aws") {
			awsPluginFound = true
			t.Logf("AWS plugin loaded for region %s: %s (version: %s, path: %s)",
				region, plugin.Name, plugin.Version, plugin.Path)

			// Validate that plugin path exists
			_, err := os.Stat(plugin.Path)
			require.NoError(t, err, "Plugin binary should exist at path: %s", plugin.Path)
		}
	}

	require.True(t, awsPluginFound, "AWS plugin should be loaded for region %s", region)
}

// Helper functions

// getFixturePath returns the absolute path to the fixture directory for a region.
func getFixturePath(t *testing.T, region string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("fixtures", "multi-region", region))
	require.NoError(t, err)
	require.DirExists(t, path, "Fixture directory should exist: %s", path)
	return path
}

// getExpectedCostsPath returns the absolute path to the expected costs file.
func getExpectedCostsPath(t *testing.T, region string) string {
	t.Helper()
	path := filepath.Join(getFixturePath(t, region), "expected-costs.json")
	require.FileExists(t, path, "Expected costs file should exist: %s", path)
	return path
}

// getTolerance returns the cost tolerance from environment or default.
func getTolerance() float64 {
	toleranceStr := os.Getenv("FINFOCUS_E2E_TOLERANCE")
	if toleranceStr == "" {
		return 0.05 // Default ±5%
	}

	// Use strconv.ParseFloat for proper error handling
	tolerance, err := strconv.ParseFloat(toleranceStr, 64)
	if err != nil {
		return 0.05 // Fallback to default on parse error
	}
	return tolerance
}

// getTimeout returns the test timeout from environment or default.
func getTimeout() time.Duration {
	timeoutStr := os.Getenv("FINFOCUS_E2E_TIMEOUT")
	if timeoutStr == "" {
		return 10 * time.Minute
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return 10 * time.Minute
	}
	return timeout
}

// shouldSkipRegion checks if a region should be skipped based on environment configuration.
func shouldSkipRegion(t *testing.T, region string) bool {
	t.Helper()
	regionsStr := os.Getenv("FINFOCUS_E2E_REGIONS")
	if regionsStr == "" {
		// Default: only run us-east-1
		return region != "us-east-1"
	}

	regions := strings.Split(regionsStr, ",")
	for _, r := range regions {
		if strings.TrimSpace(r) == region {
			return false
		}
	}
	return true
}

// loadExpectedCosts loads expected costs from JSON file.
func loadExpectedCosts(path string, costType string) ([]ExpectedCost, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read expected costs file: %w", err)
	}

	var wrapper struct {
		Region    string         `json:"region"`
		CostType  string         `json:"cost_type"`
		Resources []ExpectedCost `json:"resources"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse expected costs JSON: %w", err)
	}

	// Filter by cost type
	var filtered []ExpectedCost
	for _, cost := range wrapper.Resources {
		if cost.CostType == costType {
			filtered = append(filtered, cost)
		}
	}

	return filtered, nil
}

// generatePulumiPlan generates a Pulumi plan JSON file from the fixture.
// Returns the path to the plan file and a cleanup function.
func generatePulumiPlan(t *testing.T, fixturePath string, region string) (string, func()) {
	t.Helper()

	// For now, use a static plan file if it exists
	// In the future with #177, we would use Pulumi Automation API
	planFile := filepath.Join(fixturePath, "plan.json")

	// Check if plan file exists
	if _, err := os.Stat(planFile); err == nil {
		return planFile, func() {}
	}

	// Generate plan using pulumi CLI (temporary solution until #177)
	t.Logf("Generating Pulumi plan for %s (requires Pulumi CLI)", region)

	// TODO: This requires Pulumi Automation API (#177)
	// For now, skip plan generation and use pre-generated plans
	t.Skip("Plan generation requires Pulumi Automation API (issue #177)")

	return "", func() {}
}

// getStringField safely extracts a string field from a map.
func getStringField(m map[string]interface{}, field string) string {
	if val, ok := m[field].(string); ok {
		return val
	}
	return ""
}

// getFloatField safely extracts a float field from a map.
func getFloatField(m map[string]interface{}, field string) float64 {
	if val, ok := m[field].(float64); ok {
		return val
	}
	return 0.0
}

// TestMultiRegion_Unified_Projected tests projected cost calculation for the unified multi-region fixture.
func TestMultiRegion_Unified_Projected(t *testing.T) {
	t.Parallel()

	startTime := time.Now()

	// Log plugin versions at test start
	LogPluginVersions(t)

	// Setup test configuration
	fixturePath := getUnifiedFixturePath(t)
	expectedCostsPath := filepath.Join(fixturePath, "expected-costs.json")

	// Load unified expected costs
	expectedCosts, err := loadUnifiedExpectedCosts(expectedCostsPath)
	require.NoError(t, err, "Failed to load unified expected costs")
	require.NotEmpty(t, expectedCosts.Resources, "Expected costs cannot be empty")

	// Generate Pulumi plan
	planPath, cleanup := generatePulumiPlan(t, fixturePath, "unified")
	defer cleanup()

	// Run finfocus cost projected
	results := runProjectedCostCommand(t, planPath, getTimeout())

	// Validate plugin loading
	validatePluginLoaded(t, "aws-public")

	// Validate unified costs with per-resource and aggregate validation
	validator := NewDefaultCostValidator(expectedCosts.AggregateValidation.Tolerance)
	validationResult, err := validator.ValidateUnifiedFixtureCosts(results, expectedCosts)
	require.NoError(t, err, "Failed to validate unified fixture costs")

	// Check per-resource validations
	failureCount := 0
	for _, result := range validationResult.PerResourceResults {
		if !result.WithinTolerance {
			t.Errorf("Cost validation failed for %s (%s): actual=$%.2f, expected=[$%.2f, $%.2f], variance=%.2f%%",
				result.ResourceName, result.Region, result.ActualCost, result.ExpectedMin, result.ExpectedMax, result.Variance)
			failureCount++
		} else {
			t.Logf("Resource %s (%s): $%.2f [PASS]", result.ResourceName, result.Region, result.ActualCost)
		}
	}

	// Check aggregate validation
	if !validationResult.TotalWithinTolerance {
		t.Errorf("Aggregate total $%.2f outside expected range [$%.2f, $%.2f]",
			validationResult.TotalCost, validationResult.ExpectedTotalMin, validationResult.ExpectedTotalMax)
		failureCount++
	} else {
		t.Logf("Aggregate total: $%.2f within [$%.2f, $%.2f] [PASS]",
			validationResult.TotalCost, validationResult.ExpectedTotalMin, validationResult.ExpectedTotalMax)
	}

	// Check execution time
	executionTime := time.Since(startTime)
	t.Logf("Test execution time for unified fixture: %v", executionTime)

	require.Equal(t, 0, failureCount, "All cost validations must pass")
}

// getUnifiedFixturePath returns the absolute path to the unified fixture directory.
func getUnifiedFixturePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("fixtures", "multi-region", "unified"))
	require.NoError(t, err)
	require.DirExists(t, path, "Unified fixture directory should exist: %s", path)
	return path
}

// loadUnifiedExpectedCosts loads expected costs for a unified fixture.
func loadUnifiedExpectedCosts(path string) (UnifiedExpectedCosts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UnifiedExpectedCosts{}, fmt.Errorf("failed to read unified expected costs file: %w", err)
	}

	var expected UnifiedExpectedCosts
	if err := json.Unmarshal(data, &expected); err != nil {
		return UnifiedExpectedCosts{}, fmt.Errorf("failed to parse unified expected costs JSON: %w", err)
	}

	return expected, nil
}
