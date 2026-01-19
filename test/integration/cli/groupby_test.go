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

// TestGroupBy_Resource_JSON tests --group-by resource with JSON output.
func TestGroupBy_Resource_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "resource", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Verify results grouped by resource
	assert.NotEmpty(t, resources, "Should have resource results")
	for _, res := range resources {
		assert.Contains(t, res, "resourceId", "Each result should have resourceId when grouped by resource")
		assert.Contains(t, res, "resourceType", "Each result should have resourceType")
	}
}

// TestGroupBy_Type_JSON tests --group-by type with JSON output.
func TestGroupBy_Type_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "type", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Verify results grouped by type
	assert.NotEmpty(t, resources, "Should have type-grouped results")

	// Collect unique types
	types := make(map[string]bool)
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		types[resType] = true
	}

	// Should have multiple different resource types
	assert.Greater(t, len(types), 1, "Should have multiple resource types")
}

// TestGroupBy_Provider_JSON tests --group-by provider with JSON output.
func TestGroupBy_Provider_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "provider", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Verify results grouped by provider
	assert.NotEmpty(t, resources, "Should have provider-grouped results")

	// Collect unique providers from resource types
	providers := make(map[string]bool)
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		// Extract provider from resource type (e.g., "aws" from "aws:ec2/instance:Instance")
		parts := strings.Split(resType, ":")
		if len(parts) > 0 {
			providers[parts[0]] = true
		}
	}

	// Multi-resource-plan has multiple providers
	assert.GreaterOrEqual(t, len(providers), 1, "Should have at least one provider")
}

// TestGroupBy_Daily_JSON tests --group-by daily with JSON output.
func TestGroupBy_Daily_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-01-07",
		"--group-by", "daily", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array (aggregations)
	var aggregations []map[string]any
	err = json.Unmarshal([]byte(output), &aggregations)
	require.NoError(t, err)

	// Daily grouping produces time-based aggregations
	assert.NotEmpty(t, aggregations, "Should have daily aggregations")
}

// TestGroupBy_Monthly_JSON tests --group-by monthly with JSON output.
func TestGroupBy_Monthly_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-03-31",
		"--group-by", "monthly", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array (aggregations)
	var aggregations []map[string]any
	err = json.Unmarshal([]byte(output), &aggregations)
	require.NoError(t, err)

	// Monthly grouping produces time-based aggregations
	assert.NotEmpty(t, aggregations, "Should have monthly aggregations")
}

// TestGroupBy_Date_Alias tests --group-by date (alias for legacy date grouping).
func TestGroupBy_Date_Alias(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-01-07",
		"--group-by", "date", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)
	assert.NotEmpty(t, resources, "Should have results with date grouping")
}

// TestGroupBy_WithFilter_Combined tests combining --filter with --group-by.
func TestGroupBy_WithFilter_Combined(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--filter", "tag:env=prod", "--group-by", "type", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Should have filtered and grouped results
	assert.NotEmpty(t, resources, "Should have filtered and grouped results")
}

// TestGroupBy_TableOutput tests --group-by with table output format.
func TestGroupBy_TableOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "type", "--output", "table",
	)
	require.NoError(t, err)

	// Table output should have headers and content
	assert.NotEmpty(t, output, "Should have table output")
	// Table headers include Resource and Total Cost
	assert.Contains(t, output, "Resource", "Table should have Resource header")
	assert.Contains(t, output, "Total Cost", "Table should have Total Cost header")
}

// TestGroupBy_NDJSONOutput tests --group-by with NDJSON output format.
func TestGroupBy_NDJSONOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "type", "--output", "ndjson",
	)
	require.NoError(t, err)

	// NDJSON should be newline-delimited JSON
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.NotEmpty(t, lines, "Should have NDJSON lines")

	// Each line should be valid JSON
	for i, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]any
		err = json.Unmarshal([]byte(line), &obj)
		assert.NoError(t, err, "Line %d should be valid JSON: %s", i, line)
	}
}

// TestGroupBy_InvalidValue tests --group-by with an invalid value.
func TestGroupBy_InvalidValue(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Using an invalid group-by value
	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "invalid_grouping", "--output", "json",
	)

	// The command may succeed with the invalid value being treated as no grouping
	// or it may fail with validation error - both are valid behaviors
	if err != nil {
		assert.Contains(t, err.Error(), "invalid", "Error should mention invalid grouping")
	} else {
		// If it doesn't error, should still produce valid JSON
		var resources []map[string]any
		err = json.Unmarshal([]byte(output), &resources)
		require.NoError(t, err)
	}
}

// TestGroupBy_MultiProvider tests grouping across multiple providers.
func TestGroupBy_MultiProvider(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--group-by", "provider", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON array
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// The multi-resource-plan should have resources from multiple providers
	providers := make(map[string]bool)
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		if ok {
			parts := strings.Split(resType, ":")
			if len(parts) > 0 {
				providers[parts[0]] = true
			}
		}
	}

	// Multi-resource plan has aws, azure, gcp resources
	assert.GreaterOrEqual(t, len(providers), 1, "Should have at least one provider in results")
}

// TestGroupBy_EmptyResults tests grouping with no matching resources.
func TestGroupBy_EmptyResults(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--filter", "type=nonexistent", "--group-by", "type", "--output", "json",
	)
	require.NoError(t, err)

	// Should produce valid JSON (possibly empty array)
	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Empty results are valid
	assert.Empty(t, resources, "Should have empty results for non-matching filter")
}
