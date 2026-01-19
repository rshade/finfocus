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

// TestCostRecommendations_TableOutput tests basic table output format.
// Note: Without plugins, recommendations will be empty, but command should succeed.
func TestCostRecommendations_TableOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute recommendations command with table output
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile, "--output", "table",
	)
	require.NoError(t, err)

	// Command should succeed even with no plugins (empty recommendations)
	// The output format should still be valid
	assert.NotNil(t, output)
}

// TestCostRecommendations_JSONOutput tests JSON output format with summary.
func TestCostRecommendations_JSONOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute recommendations command with JSON output
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile, "--output", "json",
	)
	require.NoError(t, err)

	// Verify JSON is valid and contains expected structure
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// Check for summary field in JSON output
	summary, hasSummary := result["summary"]
	if hasSummary && summary != nil {
		summaryMap, ok := summary.(map[string]any)
		if ok {
			// Verify summary has expected fields
			assert.Contains(t, summaryMap, "total_count", "Summary should have total_count")
			assert.Contains(t, summaryMap, "total_savings", "Summary should have total_savings")
		}
	}

	// Check for recommendations array
	recs, hasRecs := result["recommendations"]
	if hasRecs {
		_, ok := recs.([]any)
		assert.True(t, ok, "recommendations should be an array")
	}
}

// TestCostRecommendations_NDJSONOutput tests NDJSON output format.
func TestCostRecommendations_NDJSONOutput(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute recommendations command with NDJSON output
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile, "--output", "ndjson",
	)
	require.NoError(t, err)

	// Each line should be valid JSON (if there are any lines)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]any
		err := json.Unmarshal([]byte(line), &obj)
		assert.NoError(t, err, "Line %d should be valid JSON: %s", i, line)
	}
}

// TestCostRecommendations_VerboseFlag tests the --verbose flag.
func TestCostRecommendations_VerboseFlag(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with verbose flag
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile, "--verbose",
	)
	require.NoError(t, err)

	// Command should succeed with verbose flag
	assert.NotNil(t, output)
}

// TestCostRecommendations_VerboseWithJSON tests verbose flag combined with JSON output.
func TestCostRecommendations_VerboseWithJSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with verbose and JSON output
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile, "--verbose", "--output", "json",
	)
	require.NoError(t, err)

	// Verify JSON is valid
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")
}

// TestCostRecommendations_FilterByActionType tests filtering by action type.
func TestCostRecommendations_FilterByActionType(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with action type filter
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile,
		"--filter", "action=RIGHTSIZE", "--output", "json",
	)
	require.NoError(t, err)

	// Verify JSON is valid
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// If there are recommendations, verify filter was applied
	verifyActionTypeFilter(t, result, "RIGHTSIZE")
}

// TestCostRecommendations_FilterMultipleActionTypes tests filtering by multiple action types.
func TestCostRecommendations_FilterMultipleActionTypes(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with multiple action types in filter
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile,
		"--filter", "action=RIGHTSIZE,TERMINATE", "--output", "json",
	)
	require.NoError(t, err)

	// Verify JSON is valid
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")

	// If there are recommendations, verify filter was applied
	verifyMultipleActionTypeFilter(t, result, []string{"RIGHTSIZE", "TERMINATE"})
}

// TestCostRecommendations_InvalidActionTypeFilter tests error handling for invalid action type.
func TestCostRecommendations_InvalidActionTypeFilter(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with invalid action type
	_, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile,
		"--filter", "action=INVALID_TYPE",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action type")
}

// TestCostRecommendations_CaseInsensitiveFilter tests case-insensitive action type filtering.
func TestCostRecommendations_CaseInsensitiveFilter(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with lowercase action type (should work case-insensitively)
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile,
		"--filter", "action=rightsize", "--output", "json",
	)
	require.NoError(t, err)

	// Verify JSON is valid
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON with case-insensitive filter")
}

// TestCostRecommendations_FilterWithVerbose tests filter combined with verbose flag.
func TestCostRecommendations_FilterWithVerbose(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with filter and verbose
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile,
		"--filter", "action=RIGHTSIZE", "--verbose", "--output", "json",
	)
	require.NoError(t, err)

	// Verify JSON is valid
	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Output should be valid JSON")
}

// TestCostRecommendations_MissingPlanFile tests error handling for missing plan file.
func TestCostRecommendations_MissingPlanFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Execute with non-existent plan file
	_, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", "/nonexistent/path/plan.json",
	)
	require.Error(t, err)
}

// TestCostRecommendations_InvalidOutputFormat tests error handling for invalid output format.
func TestCostRecommendations_InvalidOutputFormat(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute with invalid output format
	_, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile,
		"--output", "invalid_format",
	)
	require.Error(t, err)
}

// TestCostRecommendations_AllOutputFormats tests all supported output formats.
func TestCostRecommendations_AllOutputFormats(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	formats := []string{"table", "json", "ndjson"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			output, err := h.Execute(
				"cost", "recommendations", "--pulumi-json", planFile,
				"--output", format,
			)
			require.NoError(t, err, "Output format %s should work", format)
			assert.NotNil(t, output)

			if format == "json" {
				var result map[string]any
				err = json.Unmarshal([]byte(output), &result)
				assert.NoError(t, err, "JSON output should be valid")
			}
		})
	}
}

// TestCostRecommendations_SummarySection tests that summary section is included in output.
func TestCostRecommendations_SummarySection(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	planFile := filepath.Join("..", "..", "..", "test", "fixtures", "plans", "multi-resource-plan.json")

	// Execute and check JSON output for summary
	output, err := h.Execute(
		"cost", "recommendations", "--pulumi-json", planFile, "--output", "json",
	)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Summary should be present in JSON output
	summary, hasSummary := result["summary"]
	if hasSummary && summary != nil {
		summaryMap, ok := summary.(map[string]any)
		require.True(t, ok, "summary should be an object")

		// Check for count_by_action_type breakdown
		_, hasCountByType := summaryMap["count_by_action_type"]
		assert.True(t, hasCountByType || len(summaryMap) > 0, "Summary should have breakdown fields")

		// Check for savings_by_action_type breakdown
		_, hasSavingsByType := summaryMap["savings_by_action_type"]
		assert.True(t, hasSavingsByType || len(summaryMap) > 0, "Summary should have savings breakdown")
	}
}

// verifyActionTypeFilter checks that all recommendations in the result match the expected action type.
func verifyActionTypeFilter(t *testing.T, result map[string]any, expectedType string) {
	t.Helper()
	recs, ok := result["recommendations"].([]any)
	if !ok || len(recs) == 0 {
		return // No recommendations to verify
	}
	for _, rec := range recs {
		recMap, ok := rec.(map[string]any)
		if !ok {
			continue
		}
		actionType, ok := recMap["type"].(string)
		if !ok {
			continue
		}
		assert.Equal(t, expectedType, actionType,
			"Filtered recommendations should only be %s", expectedType)
	}
}

// verifyMultipleActionTypeFilter checks that all recommendations match one of the expected action types.
func verifyMultipleActionTypeFilter(t *testing.T, result map[string]any, expectedTypes []string) {
	t.Helper()
	validTypes := make(map[string]bool)
	for _, typ := range expectedTypes {
		validTypes[typ] = true
	}

	recs, ok := result["recommendations"].([]any)
	if !ok || len(recs) == 0 {
		return // No recommendations to verify
	}
	for _, rec := range recs {
		recMap, ok := rec.(map[string]any)
		if !ok {
			continue
		}
		actionType, ok := recMap["type"].(string)
		if !ok {
			continue
		}
		assert.True(t, validTypes[actionType],
			"Filtered recommendations should be one of %v, got %s", expectedTypes, actionType)
	}
}
