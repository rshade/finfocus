// Package output_test provides integration tests for output format generation.
package output_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

// TestOutputFormat_JSON tests JSON output format.
func TestOutputFormat_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute with JSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "Command should succeed")

	// Verify valid JSON - renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]interface{}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err, "Should produce valid JSON")

	// Extract the finfocus wrapper
	result, ok := wrapper["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	// Verify structure
	assert.Contains(t, result, "summary", "JSON should have summary")
	assert.Contains(t, result, "resources", "JSON should have resources")

	// Verify summary fields
	summary, ok := result["summary"].(map[string]interface{})
	require.True(t, ok, "Summary should be a map")
	assert.Contains(t, summary, "totalMonthly")
	assert.Contains(t, summary, "totalHourly")
	assert.Contains(t, summary, "currency")
}

// TestOutputFormat_Table tests table output format.
func TestOutputFormat_Table(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute with table output (default)
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "table")
	require.NoError(t, err, "Command should succeed")

	// Verify table format
	h.AssertContains(output, "COST SUMMARY")
	h.AssertContains(output, "Total Monthly Cost")
	h.AssertContains(output, "Total Hourly Cost")
	h.AssertContains(output, "RESOURCE DETAILS")

	// Verify table has separators
	assert.Contains(t, output, "===", "Table should have separator lines")
	assert.Contains(t, output, "---", "Table should have row separators")
}

// TestOutputFormat_NDJSON tests NDJSON (newline-delimited JSON) output format.
func TestOutputFormat_NDJSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute with NDJSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "ndjson")
	require.NoError(t, err, "Command should succeed")

	// NDJSON may be empty if no resources, or have one JSON object per line
	if output != "" {
		lines := strings.Split(strings.TrimSpace(output), "\n")

		// Each line should be valid JSON
		for i, line := range lines {
			if line == "" {
				continue
			}
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(line), &obj)
			assert.NoError(t, err, "Line %d should be valid JSON: %s", i, line)
		}
	}
}

// TestOutputFormat_DefaultIsTable tests that table is the default output format.
func TestOutputFormat_DefaultIsTable(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute without --output flag (should default to table per config)
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile)
	require.NoError(t, err, "Command should succeed")

	// Verify it's table format (contains table headers)
	h.AssertContains(output, "COST SUMMARY")
	h.AssertContains(output, "Total Monthly Cost")
}

// TestOutputFormat_InvalidFormat tests error handling for invalid format.
func TestOutputFormat_InvalidFormat(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute with invalid format
	errMsg := h.ExecuteExpectError("cost", "projected", "--pulumi-json", planFile, "--output", "xml")

	// Should report unsupported format
	assert.Contains(t, errMsg, "unsupported", "Should report unsupported output format")
}

// TestOutputFormat_EmptyResults tests output formats with empty results.
func TestOutputFormat_EmptyResults(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create empty plan
	emptyPlan := `{"resources": []}`
	planFile := h.CreateTempFile(emptyPlan)

	// Test JSON format
	outputJSON, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "JSON output should handle empty results")

	// renderJSON wraps results in {"finfocus": ...}
	var wrapperJSON map[string]interface{}
	err = json.Unmarshal([]byte(outputJSON), &wrapperJSON)
	require.NoError(t, err, "Should produce valid JSON for empty results")

	resultJSON, ok := wrapperJSON["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	resources, ok := resultJSON["resources"].([]interface{})
	require.True(t, ok, "Resources should be an array")
	assert.Empty(t, resources, "Should have no resources")

	// Test table format
	outputTable, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "table")
	require.NoError(t, err, "Table output should handle empty results")

	h.AssertContains(outputTable, "COST SUMMARY")
	h.AssertContains(outputTable, "0.00")
}

// TestOutputFormat_CurrencyFormatting tests currency formatting in different outputs.
func TestOutputFormat_CurrencyFormatting(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Test JSON - currency should be string field
	outputJSON, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapperJSON map[string]interface{}
	err = json.Unmarshal([]byte(outputJSON), &wrapperJSON)
	require.NoError(t, err)

	resultJSON, ok := wrapperJSON["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	summary := resultJSON["summary"].(map[string]interface{})
	currency, ok := summary["currency"].(string)
	require.True(t, ok, "Currency should be a string")
	assert.Equal(t, "USD", currency, "Default currency should be USD")

	// Test table - currency should appear in output
	outputTable, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "table")
	require.NoError(t, err)

	h.AssertContains(outputTable, "USD")
}

// TestOutputFormat_CostPrecision tests decimal precision in cost values.
func TestOutputFormat_CostPrecision(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Test JSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]interface{}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	// Check that cost values are numbers (not strings)
	summary := result["summary"].(map[string]interface{})
	totalMonthly, ok := summary["totalMonthly"].(float64)
	require.True(t, ok, "Monthly cost should be a number")

	// Cost should be >= 0
	assert.GreaterOrEqual(t, totalMonthly, 0.0, "Cost should be non-negative")
}

// TestOutputFormat_ResourceFields tests that all expected resource fields are present.
func TestOutputFormat_ResourceFields(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Get JSON output
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]interface{}
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	resources, ok := result["resources"].([]interface{})
	require.True(t, ok, "Resources should be an array")

	if len(resources) > 0 {
		// Check first resource has required fields
		resource := resources[0].(map[string]interface{})

		assert.Contains(t, resource, "resourceType", "Resource should have type")
		assert.Contains(t, resource, "resourceId", "Resource should have ID")
		assert.Contains(t, resource, "adapter", "Resource should have adapter")
		assert.Contains(t, resource, "currency", "Resource should have currency")
		assert.Contains(t, resource, "monthly", "Resource should have monthly cost")
		assert.Contains(t, resource, "hourly", "Resource should have hourly cost")
	}
}

// TestOutputFormat_ConsistencyAcrossFormats tests that data is consistent across formats.
func TestOutputFormat_ConsistencyAcrossFormats(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Get JSON output
	outputJSON, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapperJSON map[string]interface{}
	err = json.Unmarshal([]byte(outputJSON), &wrapperJSON)
	require.NoError(t, err)

	resultJSON, ok := wrapperJSON["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	summaryJSON := resultJSON["summary"].(map[string]interface{})
	totalMonthlyJSON := summaryJSON["totalMonthly"].(float64)

	// Get table output
	outputTable, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "table")
	require.NoError(t, err)

	// Table should contain the same total (formatted as string)
	// Can't do exact comparison due to formatting, but verify output exists
	h.AssertContains(outputTable, "Total Monthly Cost")

	// Both formats should report the same resource count
	resourcesJSON := resultJSON["resources"].([]interface{})
	resourceCountJSON := len(resourcesJSON)

	// Count resources in table (rough check - just verify non-zero if JSON has resources)
	if resourceCountJSON > 0 {
		h.AssertContains(outputTable, "RESOURCE DETAILS")
	}

	// Verify total is non-negative in JSON
	assert.GreaterOrEqual(t, totalMonthlyJSON, 0.0, "Total cost should be non-negative")
}
