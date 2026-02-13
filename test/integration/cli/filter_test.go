package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

func TestProjectedCost_FilterByType(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute projected cost command with type filter
	output, err := h.Execute(
		"cost", "projected", "--pulumi-json", planFile, "--filter",
		"type=aws:ec2/instance:Instance", "--output", "json",
	)
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]any
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]any)
	require.True(t, ok, "expected finfocus wrapper")

	resources, ok := result["resources"].([]any)
	require.True(t, ok, "expected resources to be an array")

	// Verify filtered results
	assert.NotEmpty(t, resources, "Expected matches for filter: type=aws:ec2/instance:Instance. Output: %s", output)
	for _, r := range resources {
		res, ok := r.(map[string]any)
		require.True(t, ok, "expected resource to be an object")
		assert.Equal(t, "aws:ec2/instance:Instance", res["resourceType"])
	}
}

func TestProjectedCost_FilterByTypeSubstring(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Filter by "bucket" substring
	output, err := h.Execute(
		"cost", "projected", "--pulumi-json", planFile,
		"--filter", "type=bucket", "--output", "json",
	)
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]any
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]any)
	require.True(t, ok, "expected finfocus wrapper")

	resources, ok := result["resources"].([]any)
	require.True(t, ok, "expected resources to be an array")
	assert.NotEmpty(t, resources)
	for _, r := range resources {
		res, ok := r.(map[string]any)
		require.True(t, ok, "expected resource to be an object")
		assert.Contains(t, res["resourceType"], "Bucket")
	}
}

func TestProjectedCost_FilterByProvider(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Filter by "azure" provider
	output, err := h.Execute(
		"cost", "projected", "--pulumi-json", planFile,
		"--filter", "provider=azure", "--output", "json",
	)
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]any
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]any)
	require.True(t, ok, "expected finfocus wrapper")

	resources, ok := result["resources"].([]any)
	require.True(t, ok, "expected resources to be an array")
	assert.NotEmpty(t, resources)
	for _, r := range resources {
		res, ok := r.(map[string]any)
		require.True(t, ok, "expected resource to be an object")
		typeStr, ok := res["resourceType"].(string)
		require.True(t, ok, "expected resourceType to be a string")
		assert.Contains(t, typeStr, "azure")
	}
}

func TestActualCost_FilterByTag(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Filter by env=prod tag
	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "tag:env=prod", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Expect results filtered to only those with the tag
	assert.NotEmpty(t, resources)

	for _, res := range resources {
		id, ok := res["resourceId"].(string)
		require.True(t, ok, "expected resourceId to be a string")
		// Only these resources have env=prod in the fixture
		assert.True(t,
			strings.HasSuffix(id, "i-1234567890abcdef0") ||
				strings.HasSuffix(id, "vm-azure-1") ||
				strings.HasSuffix(id, "db-1"),
			"Found unexpected resource ID in filtered output: %s", id)
	}
}

func TestActualCost_FilterByTagAndType(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Test filter combined with group-by
	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "tag:env=prod", "--group-by", "type", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	assert.NotEmpty(t, resources)

	// With group-by type, we should see aggregated results for types that have env=prod resources
	foundEC2 := false
	foundAzure := false
	foundRDS := false

	for _, res := range resources {
		rType, ok := res["resourceType"].(string)
		require.True(t, ok, "expected resourceType to be a string")

		switch rType {
		case "aws:ec2/instance:Instance":
			foundEC2 = true
		case "azure:compute/virtualMachine:VirtualMachine":
			foundAzure = true
		case "aws:rds/instance:Instance":
			foundRDS = true
		case "gcp:compute/instance:Instance":
			t.Error("Found GCP resource, should be filtered out")
		case "aws:s3/bucket:Bucket":
			t.Error("Found S3 bucket, should be filtered out")
		}
	}

	assert.True(t, foundEC2, "Should find EC2")
	assert.True(t, foundAzure, "Should find Azure VM")
	assert.True(t, foundRDS, "Should find RDS")
}

func TestProjectedCost_FilterNoMatch(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Filter by something that won't match
	output, err := h.Execute(
		"cost", "projected", "--pulumi-json", planFile,
		"--filter", "type=nonexistent", "--output", "json",
	)
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]any
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]any)
	require.True(t, ok, "expected finfocus wrapper")

	resources, ok := result["resources"].([]any)
	require.True(t, ok, "expected resources to be an array")
	assert.Empty(t, resources, "Expected no resources to match 'type=nonexistent'")
}

func TestProjectedCost_FilterInvalidSyntax(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Invalid filter syntax (missing '=')
	_, err := h.Execute(
		"cost", "projected", "--pulumi-json", planFile,
		"--filter", "invalid-syntax",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter syntax")
}

func TestFilter_CaseSensitivity(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Test that filters are case-insensitive
	output, err := h.Execute(
		"cost", "projected", "--pulumi-json", planFile,
		"--filter", "TYPE=AWS:EC2/INSTANCE:INSTANCE", "--output", "json",
	)
	require.NoError(t, err)

	// renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]any
	err = json.Unmarshal([]byte(output), &wrapper)
	require.NoError(t, err)

	result, ok := wrapper["finfocus"].(map[string]any)
	require.True(t, ok, "expected finfocus wrapper")

	resources, ok := result["resources"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, resources, "Filter should be case-insensitive")
}

func TestFilter_AllOutputFormats(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")
	filter := "type=aws:ec2/instance:Instance"

	formats := []string{"table", "json", "ndjson"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			output, err := h.Execute(
				"cost", "projected", "--pulumi-json", planFile,
				"--filter", filter, "--output", format,
			)
			require.NoError(t, err)
			assert.NotEmpty(t, output)

			if format == "json" {
				// renderJSON wraps results in {"finfocus": ...}
				var wrapper map[string]any
				err = json.Unmarshal([]byte(output), &wrapper)
				assert.NoError(t, err)
				result, ok := wrapper["finfocus"].(map[string]any)
				require.True(t, ok, "expected finfocus wrapper")
				resources, ok := result["resources"].([]any)
				require.True(t, ok, "expected resources to be an array")
				assert.NotEmpty(t, resources)
			}
		})
	}
}

// TestActualCost_FilterByTag_NDJSON tests tag filter with NDJSON output for actual costs.
func TestActualCost_FilterByTag_NDJSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "tag:env=prod", "--output", "ndjson",
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
		assert.NoError(t, err, "Line %d should be valid JSON", i)
	}
}

// TestActualCost_FilterByType_Exact tests exact type match filter.
func TestActualCost_FilterByType_Exact(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "type=aws:ec2/instance:Instance", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// All results should have the exact type
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		assert.Contains(t, resType, "ec2", "Should match EC2 instances")
	}
}

// TestActualCost_FilterByType_Substring tests partial type match filter.
func TestActualCost_FilterByType_Substring(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "type=bucket", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// All results should have bucket in the type
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		assert.Contains(t, strings.ToLower(resType), "bucket", "Should match bucket types")
	}
}

// TestActualCost_FilterByProvider_Actual tests provider filter for actual costs.
func TestActualCost_FilterByProvider_Actual(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "provider=aws", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// All results should be from AWS
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		assert.True(t, strings.HasPrefix(resType, "aws:"), "Should only have AWS resources")
	}
}

// TestActualCost_FilterNoMatch tests filter with no matching results.
func TestActualCost_FilterNoMatch(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "type=nonexistent_type", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Should have empty result set
	assert.Empty(t, resources, "Should have empty results for non-matching filter")
}

// TestActualCost_FilterCaseSensitivity tests case-insensitive filter matching.
func TestActualCost_FilterCaseSensitivity(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Use uppercase filter value
	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "TYPE=AWS:EC2/INSTANCE:INSTANCE", "--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// Should find results with case-insensitive matching
	assert.NotEmpty(t, resources, "Filter should be case-insensitive")
}

// TestActualCost_FilterInvalidSyntax_Actual tests invalid filter syntax for actual costs.
func TestActualCost_FilterInvalidSyntax_Actual(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	_, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--filter", "invalid-no-equals-sign",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter syntax")
}

// TestActualCost_MultipleFilters tests using multiple --filter flags.
func TestActualCost_MultipleFilters(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Use multiple filter flags
	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "provider=aws",
		"--filter", "type=ec2",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// All results should be AWS EC2 instances
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		assert.True(t, strings.HasPrefix(resType, "aws:"), "Should only have AWS resources")
		assert.Contains(t, strings.ToLower(resType), "ec2", "Should only have EC2 resources")
	}
}
