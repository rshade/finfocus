package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rshade/finfocus/test/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIWorkflow_ProjectedCost tests the complete CLI → Engine flow for projected costs.
func TestCLIWorkflow_ProjectedCost(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Use existing test fixture
	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute projected cost command with JSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "Command should succeed")

	// Parse JSON output - renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]interface{}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err, "Should return valid JSON")

	// Extract the finfocus wrapper
	result, ok := wrapper["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	// Verify structure
	assert.Contains(t, result, "summary", "Should have summary section")
	assert.Contains(t, result, "resources", "Should have resources section")

	// Verify summary structure
	summary, ok := result["summary"].(map[string]interface{})
	require.True(t, ok, "Summary should be a map")
	assert.Contains(t, summary, "totalMonthly")
	assert.Contains(t, summary, "totalHourly")
	assert.Contains(t, summary, "currency")
}

// TestCLIWorkflow_ProjectedCost_TableOutput tests table output format.
func TestCLIWorkflow_ProjectedCost_TableOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute with table output (default)
	output := h.ExecuteOrFail("cost", "projected", "--pulumi-json", planFile)

	// Verify table format contains expected headers
	h.AssertContains(output, "COST SUMMARY")
	h.AssertContains(output, "Total Monthly Cost")
	h.AssertContains(output, "RESOURCE DETAILS")
}

// TestCLIWorkflow_ProjectedCost_MissingPlan tests error handling for missing plan file.
func TestCLIWorkflow_ProjectedCost_MissingPlan(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Execute with non-existent plan file
	errMsg := h.ExecuteExpectError("cost", "projected", "--pulumi-json", "/nonexistent/plan.json")

	// Verify error message
	assert.Contains(t, errMsg, "no such file", "Should report missing file")
}

// TestCLIWorkflow_ProjectedCost_InvalidJSON tests error handling for invalid JSON.
func TestCLIWorkflow_ProjectedCost_InvalidJSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temp file with invalid JSON
	invalidJSON := `{"invalid": json}`
	planFile := h.CreateTempFile(invalidJSON)

	// Execute command
	errMsg := h.ExecuteExpectError("cost", "projected", "--pulumi-json", planFile)

	// Verify error message mentions JSON parsing
	assert.Contains(t, errMsg, "invalid", "Should report invalid JSON")
}

// TestCLIWorkflow_ProjectedCost_EmptyPlan tests handling of empty plan.
func TestCLIWorkflow_ProjectedCost_EmptyPlan(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temp file with empty plan
	emptyPlan := `{"resources": []}`
	planFile := h.CreateTempFile(emptyPlan)

	// Execute command with JSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "Should handle empty plan gracefully")

	// Parse output - renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]interface{}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err, "Should return valid JSON")

	// Extract the finfocus wrapper
	result, ok := wrapper["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	// Verify empty resources
	resources, ok := result["resources"].([]interface{})
	require.True(t, ok, "Resources should be an array")
	assert.Empty(t, resources, "Should have no resources")
}

// TestCLIWorkflow_ProjectedCost_NDJSONOutput tests NDJSON output format.
func TestCLIWorkflow_ProjectedCost_NDJSONOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute with NDJSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "ndjson")
	require.NoError(t, err, "Command should succeed")

	// NDJSON should have multiple lines (one JSON object per line)
	// Each line should be valid JSON
	if output != "" {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		// Each line should be valid JSON
		for i, line := range lines {
			if line == "" {
				continue
			}
			var obj map[string]interface{}
			err = json.Unmarshal([]byte(line), &obj)
			assert.NoError(t, err, "NDJSON line %d should be valid JSON", i)
		}
	}
}

// TestCLIWorkflow_Help tests help command output.
func TestCLIWorkflow_Help(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Test root help
	output := h.ExecuteOrFail("--help")
	h.AssertContains(output, "finfocus")
	h.AssertContains(output, "cost")
	h.AssertContains(output, "plugin")

	// Test cost command help
	output = h.ExecuteOrFail("cost", "--help")
	h.AssertContains(output, "projected")
	h.AssertContains(output, "actual")

	// Test projected cost help
	output = h.ExecuteOrFail("cost", "projected", "--help")
	h.AssertContains(output, "--pulumi-json")
	h.AssertContains(output, "--output")
}

// TestCLIWorkflow_Version tests version command.
func TestCLIWorkflow_Version(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	output := h.ExecuteOrFail("--version")

	// Should contain version information
	assert.NotEmpty(t, output, "Version output should not be empty")
}

// TestCLIWorkflow_PluginList tests plugin list command.
func TestCLIWorkflow_PluginList(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Use isolated HOME directory to ensure no plugins are found
	tempHome := t.TempDir()
	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		// Execute plugin list (will be empty without plugins)
		output, err := h.Execute("plugin", "list")
		require.NoError(t, err, "Command should succeed even with no plugins")

		// Should indicate no plugins installed
		assert.Contains(t, output, "No plugins installed", "Should report no plugins")
	})
}

// TestCLIWorkflow_PluginValidate tests plugin validate command.
func TestCLIWorkflow_PluginValidate(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Use isolated HOME directory to ensure no plugins are found
	tempHome := t.TempDir()
	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		// Execute plugin validate (will be empty without plugins)
		output, err := h.Execute("plugin", "validate")
		require.NoError(t, err, "Command should succeed even with no plugins")

		// Should indicate no plugins to validate
		assert.Contains(t, output, "No plugins to validate", "Should report no plugins to validate")
	})
}
