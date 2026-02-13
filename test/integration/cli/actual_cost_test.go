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

// TestActualCost_DateRangeValid tests valid date range parameters.
func TestActualCost_DateRangeValid(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "aws-simple-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-01-31",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)
	assert.NotNil(t, resources, "Should produce valid JSON output")
}

// TestActualCost_DateRangeInvalid tests that to < from produces an error.
func TestActualCost_DateRangeInvalid(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "aws-simple-plan.json")

	// End date before start date
	_, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-12-31", "--to", "2025-01-01",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "to", "Error should mention date range issue")
}

// TestActualCost_DateFormats tests multiple date format support.
func TestActualCost_DateFormats(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "aws-simple-plan.json")

	tests := []struct {
		name string
		from string
		to   string
	}{
		{"YYYY-MM-DD", "2025-01-01", "2025-01-31"},
		{"RFC3339", "2025-01-01T00:00:00Z", "2025-01-31T23:59:59Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := h.Execute(
				"cost", "actual", "--pulumi-json", planFile,
				"--from", tt.from, "--to", tt.to,
				"--output", "json",
			)
			require.NoError(t, err)

			var resources []map[string]any
			err = json.Unmarshal([]byte(output), &resources)
			require.NoError(t, err)
		})
	}
}

// TestActualCost_DefaultToDate tests that only --from is specified (to defaults to now).
func TestActualCost_DefaultToDate(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "aws-simple-plan.json")

	// Only specify --from, --to should default to now
	// Use a recent date to avoid "date range too large" error
	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2026-01-01",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)
}

// TestActualCost_MissingFromDate tests that --from is required.
func TestActualCost_MissingFromDate(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "aws-simple-plan.json")

	// Missing required --from flag
	_, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--to", "2025-12-31",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "from", "Error should mention missing --from flag")
}

// TestActualCost_OutputFormats tests all supported output formats.
func TestActualCost_OutputFormats(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "aws-simple-plan.json")

	formats := []string{"table", "json", "ndjson"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			output, err := h.Execute(
				"cost", "actual", "--pulumi-json", planFile,
				"--from", "2025-01-01", "--to", "2025-12-31",
				"--fallback-estimate",
				"--output", format,
			)
			require.NoError(t, err)
			assert.NotEmpty(t, output, "Should produce non-empty output")

			if format == "json" {
				var resources []map[string]any
				err = json.Unmarshal([]byte(output), &resources)
				assert.NoError(t, err, "JSON output should be valid")
			}

			if format == "ndjson" {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					var obj map[string]any
					err = json.Unmarshal([]byte(line), &obj)
					assert.NoError(t, err, "Each NDJSON line should be valid JSON")
				}
			}
		})
	}
}

// TestActualCost_WithStateFile tests using a state JSON file as input.
func TestActualCost_WithStateFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	stateFile := filepath.Join("..", "..", "..", "test", "fixtures", "state", "valid-state.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", stateFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)
}

// TestActualCost_CombinedFlags tests combining filter + group-by + output.
func TestActualCost_CombinedFlags(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--fallback-estimate",
		"--filter", "provider=aws",
		"--group-by", "type",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)

	// All results should be AWS resources
	for _, res := range resources {
		resType, ok := res["resourceType"].(string)
		require.True(t, ok, "resourceType should be a string")
		assert.True(t, strings.HasPrefix(resType, "aws:"), "Should only have AWS resources")
	}
}

// TestActualCost_EmptyPlan tests behavior with a plan that has no resources.
func TestActualCost_EmptyPlan(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create a minimal empty plan
	emptyPlan := `{"version": 1, "deployment": {"resources": []}}`
	planFile := h.CreateTempFile(emptyPlan)

	output, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
		"--output", "json",
	)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(output), &resources)
	require.NoError(t, err)
	assert.Empty(t, resources, "Should have empty results for empty plan")
}

// TestActualCost_NonExistentFile tests error handling for missing input file.
func TestActualCost_NonExistentFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	_, err := h.Execute(
		"cost", "actual", "--pulumi-json", "/nonexistent/path/file.json",
		"--from", "2025-01-01", "--to", "2025-12-31",
	)
	require.Error(t, err)
	// Error should mention file not found or similar
	assert.True(t,
		strings.Contains(err.Error(), "no such file") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist"),
		"Error should mention file not found: %s", err.Error())
}

// TestActualCost_InvalidJSON tests error handling for invalid JSON input.
func TestActualCost_InvalidJSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create an invalid JSON file
	invalidJSON := `{not valid json}`
	planFile := h.CreateTempFile(invalidJSON)

	_, err := h.Execute(
		"cost", "actual", "--pulumi-json", planFile,
		"--from", "2025-01-01", "--to", "2025-12-31",
	)
	require.Error(t, err)
}
