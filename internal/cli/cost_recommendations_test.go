package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

// T001: Test NewCostRecommendationsCmd() creates a valid command.
func TestNewCostRecommendationsCmd(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "recommendations", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

// T001: Test command has required flags.
func TestNewCostRecommendationsCmd_Flags(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	// Check pulumi-json flag exists and is required
	pulumiJSONFlag := cmd.Flags().Lookup("pulumi-json")
	require.NotNil(t, pulumiJSONFlag, "pulumi-json flag should exist")

	// Check other expected flags
	adapterFlag := cmd.Flags().Lookup("adapter")
	require.NotNil(t, adapterFlag, "adapter flag should exist")

	outputFlag := cmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag, "output flag should exist")

	filterFlag := cmd.Flags().Lookup("filter")
	require.NotNil(t, filterFlag, "filter flag should exist")
}

// T001: Test command fails without required pulumi-json flag.
func TestNewCostRecommendationsCmd_RequiredFlags(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	// Execute without required flag should fail
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pulumi-json")
}

// T002: Test table output format rendering.
func TestCostRecommendationsCmd_TableOutput(t *testing.T) {
	// Create a temporary plan file
	planJSON := `{
		"version": 3,
		"steps": [
			{
				"op": "create",
				"urn": "urn:pulumi:test::test::aws:ec2/instance:Instance::test-instance",
				"type": "aws:ec2/instance:Instance",
				"newState": {
					"type": "aws:ec2/instance:Instance",
					"inputs": {
						"instanceType": "t3.micro",
						"availabilityZone": "us-east-1a"
					}
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Execute with table output (default)
	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "table"})
	// Command will fail without plugins, but that's expected
	// We're testing that the command infrastructure works
	_ = cmd.Execute()

	// The output should contain some text (even if empty recommendations)
	// In a full integration test with plugins, we'd check for table headers
}

// T002: Test JSON output format rendering.
func TestCostRecommendationsCmd_JSONOutput(t *testing.T) {
	planJSON := `{
		"version": 3,
		"steps": [
			{
				"op": "create",
				"urn": "urn:pulumi:test::test::aws:ec2/instance:Instance::test-instance",
				"type": "aws:ec2/instance:Instance",
				"newState": {
					"type": "aws:ec2/instance:Instance",
					"inputs": {
						"instanceType": "t3.micro"
					}
				}
			}
		]
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "json"})
	_ = cmd.Execute()

	// If output is produced, it should be valid JSON
	output := strings.TrimSpace(outBuf.String())
	if output != "" && strings.HasPrefix(output, "{") {
		var result map[string]interface{}
		jsonErr := json.Unmarshal([]byte(output), &result)
		assert.NoError(t, jsonErr, "JSON output should be valid JSON")
	}
}

// T002: Test NDJSON output format rendering.
func TestCostRecommendationsCmd_NDJSONOutput(t *testing.T) {
	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "ndjson"})
	_ = cmd.Execute()

	// NDJSON output should have each line as valid JSON (if any output)
	output := strings.TrimSpace(outBuf.String())
	if output != "" {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && strings.HasPrefix(line, "{") {
				var item map[string]interface{}
				jsonErr := json.Unmarshal([]byte(line), &item)
				assert.NoError(t, jsonErr, "each NDJSON line should be valid JSON")
			}
		}
	}
}

// T003: Test --filter flag parsing with action types.
func TestCostRecommendationsCmd_FilterFlag(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	// Check filter flag accepts array values
	filterFlag := cmd.Flags().Lookup("filter")
	require.NotNil(t, filterFlag)

	// Test filter flag can be set
	err := cmd.Flags().Set("filter", "action=MIGRATE")
	assert.NoError(t, err)
}

// T003: Test multiple filter values.
func TestCostRecommendationsCmd_MultipleFilters(t *testing.T) {
	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Test with multiple filter values
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "action=MIGRATE,RIGHTSIZE",
		"--output", "json",
	})
	_ = cmd.Execute()

	// Command should process without panic
}

// T003: Test case-insensitive filter matching.
func TestCostRecommendationsCmd_CaseInsensitiveFilter(t *testing.T) {
	planJSON := `{
		"version": 3,
		"steps": []
	}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Test lowercase filter values
	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "action=migrate",
		"--output", "json",
	})
	_ = cmd.Execute()

	// Command should process without panic (case insensitivity tested in proto package)
}

// Test invalid plan path error handling.
func TestCostRecommendationsCmd_InvalidPlanPath(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{"--pulumi-json", "/nonexistent/path/plan.json"})
	err := cmd.Execute()

	assert.Error(t, err)
}

// Test unsupported output format error.
func TestCostRecommendationsCmd_UnsupportedOutputFormat(t *testing.T) {
	planJSON := `{"version": 3, "steps": []}`

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{"--pulumi-json", planPath, "--output", "invalid"})
	err = cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

// T022: Test action type filter parsing in CLI command with valid types.
func TestCostRecommendationsCmd_ValidActionTypeFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter string
	}{
		{"single type MIGRATE", "action=MIGRATE"},
		{"single type RIGHTSIZE", "action=RIGHTSIZE"},
		{"single type TERMINATE", "action=TERMINATE"},
		{"single type CONSOLIDATE", "action=CONSOLIDATE"},
		{"single type SCHEDULE", "action=SCHEDULE"},
		{"single type REFACTOR", "action=REFACTOR"},
		{"single type OTHER", "action=OTHER"},
		{"multiple types", "action=MIGRATE,RIGHTSIZE,TERMINATE"},
		{"lowercase", "action=migrate"},
		{"mixed case", "action=Migrate,RIGHTSIZE"},
		{"with spaces", "action=MIGRATE , RIGHTSIZE"},
	}

	planJSON := `{"version": 3, "steps": []}`
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewCostRecommendationsCmd()
			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&outBuf)

			cmd.SetArgs([]string{
				"--pulumi-json", planPath,
				"--filter", tt.filter,
				"--output", "json",
			})
			// Should not error due to filter parsing
			// (may error due to no plugins, but filter is valid)
			_ = cmd.Execute()
		})
	}
}

// T023: Test invalid action type filter error message listing all 11 valid types.
func TestCostRecommendationsCmd_InvalidActionTypeFilterError(t *testing.T) {
	planJSON := `{"version": 3, "steps": []}`
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "action=INVALID_TYPE",
		"--output", "json",
	})
	err = cmd.Execute()

	// Should error with invalid action type
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action type")

	// Error message should list valid types
	errMsg := err.Error()
	validTypes := []string{"RIGHTSIZE", "TERMINATE", "MIGRATE", "CONSOLIDATE", "SCHEDULE", "REFACTOR", "OTHER"}
	for _, vt := range validTypes {
		assert.Contains(t, errMsg, vt, "error should list valid type: %s", vt)
	}
}

// T023: Test empty action type filter error.
func TestCostRecommendationsCmd_EmptyActionTypeFilter(t *testing.T) {
	planJSON := `{"version": 3, "steps": []}`
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	err := os.WriteFile(planPath, []byte(planJSON), 0o600)
	require.NoError(t, err)

	cmd := cli.NewCostRecommendationsCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	cmd.SetArgs([]string{
		"--pulumi-json", planPath,
		"--filter", "action=",
		"--output", "json",
	})
	err = cmd.Execute()

	// Should error with empty filter value
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action type filter")
}

// ============================================================================
// User Story 1: Quick Overview of Recommendations (T011-T019)
// ============================================================================

// T011: Test renderRecommendationsSummary renders summary section correctly.
func TestRenderRecommendationsSummary(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{Type: "RIGHTSIZE", EstimatedSavings: 87.60, Currency: "USD"},
		{Type: "TERMINATE", EstimatedSavings: 175.20, Currency: "USD"},
		{Type: "RIGHTSIZE", EstimatedSavings: 50.00, Currency: "USD"},
	}

	var buf bytes.Buffer
	cli.RenderRecommendationsSummaryForTest(&buf, recs)
	output := buf.String()

	// Verify summary section exists
	assert.Contains(t, output, "RECOMMENDATIONS SUMMARY")

	// Verify total count
	assert.Contains(t, output, "3") // total recommendations

	// Verify total savings
	assert.Contains(t, output, "312.80") // 87.60 + 175.20 + 50.00

	// Verify action type breakdown (labels are human-readable)
	assert.Contains(t, output, "Rightsize")
	assert.Contains(t, output, "Terminate")
}

// T011: Test renderRecommendationsSummary with empty recommendations.
func TestRenderRecommendationsSummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	cli.RenderRecommendationsSummaryForTest(&buf, nil)
	output := buf.String()

	// Should show "No recommendations" or empty state
	assert.Contains(t, output, "0")
}

// T012: Test sortRecommendationsBySavings sorts correctly.
func TestSortRecommendationsBySavings(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", EstimatedSavings: 10.00},
		{ResourceID: "r2", EstimatedSavings: 100.00},
		{ResourceID: "r3", EstimatedSavings: 50.00},
		{ResourceID: "r4", EstimatedSavings: 25.00},
	}

	sorted := cli.SortRecommendationsBySavingsForTest(recs)

	require.Len(t, sorted, 4)
	assert.Equal(t, "r2", sorted[0].ResourceID) // 100.00
	assert.Equal(t, "r3", sorted[1].ResourceID) // 50.00
	assert.Equal(t, "r4", sorted[2].ResourceID) // 25.00
	assert.Equal(t, "r1", sorted[3].ResourceID) // 10.00
}

// T012: Test sortRecommendationsBySavings with empty input.
func TestSortRecommendationsBySavings_Empty(t *testing.T) {
	sorted := cli.SortRecommendationsBySavingsForTest(nil)
	assert.Empty(t, sorted)
}

// T012: Test sortRecommendationsBySavings with equal savings (stable sort).
func TestSortRecommendationsBySavings_EqualValues(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", EstimatedSavings: 50.00},
		{ResourceID: "r2", EstimatedSavings: 50.00},
		{ResourceID: "r3", EstimatedSavings: 50.00},
	}

	sorted := cli.SortRecommendationsBySavingsForTest(recs)

	require.Len(t, sorted, 3)
	// All should have same savings
	assert.Equal(t, 50.00, sorted[0].EstimatedSavings)
	assert.Equal(t, 50.00, sorted[1].EstimatedSavings)
	assert.Equal(t, 50.00, sorted[2].EstimatedSavings)
}

// ============================================================================
// User Story 2: Verbose Mode (T020-T025)
// ============================================================================

// T020: Test --verbose flag is recognized by the command.
func TestNewCostRecommendationsCmd_VerboseFlag(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	// Check verbose flag exists
	verboseFlag := cmd.Flags().Lookup("verbose")
	require.NotNil(t, verboseFlag, "verbose flag should exist")

	// Default should be false
	assert.Equal(t, "false", verboseFlag.DefValue)
}

// T021: Test verbose mode shows all recommendations.
func TestRenderRecommendationsVerbose(t *testing.T) {
	// Create 7 recommendations (more than default 5)
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
		{ResourceID: "r3", Type: "RIGHTSIZE", EstimatedSavings: 80.00, Currency: "USD"},
		{ResourceID: "r4", Type: "DELETE_UNUSED", EstimatedSavings: 70.00, Currency: "USD"},
		{ResourceID: "r5", Type: "MODIFY", EstimatedSavings: 60.00, Currency: "USD"},
		{ResourceID: "r6", Type: "RIGHTSIZE", EstimatedSavings: 50.00, Currency: "USD"},
		{ResourceID: "r7", Type: "TERMINATE", EstimatedSavings: 40.00, Currency: "USD"},
	}

	var buf bytes.Buffer
	cli.RenderRecommendationsTableVerboseForTest(&buf, recs, true)
	output := buf.String()

	// All 7 resources should be shown in verbose mode
	assert.Contains(t, output, "r1")
	assert.Contains(t, output, "r2")
	assert.Contains(t, output, "r3")
	assert.Contains(t, output, "r4")
	assert.Contains(t, output, "r5")
	assert.Contains(t, output, "r6")
	assert.Contains(t, output, "r7")

	// Should NOT show "Use --verbose" hint
	assert.NotContains(t, output, "Use --verbose")
}

// T021: Test non-verbose mode shows only top 5.
func TestRenderRecommendationsNonVerbose(t *testing.T) {
	// Create 7 recommendations (more than default 5)
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
		{ResourceID: "r3", Type: "RIGHTSIZE", EstimatedSavings: 80.00, Currency: "USD"},
		{ResourceID: "r4", Type: "DELETE_UNUSED", EstimatedSavings: 70.00, Currency: "USD"},
		{ResourceID: "r5", Type: "MODIFY", EstimatedSavings: 60.00, Currency: "USD"},
		{ResourceID: "r6", Type: "RIGHTSIZE", EstimatedSavings: 50.00, Currency: "USD"},
		{ResourceID: "r7", Type: "TERMINATE", EstimatedSavings: 40.00, Currency: "USD"},
	}

	var buf bytes.Buffer
	cli.RenderRecommendationsTableVerboseForTest(&buf, recs, false)
	output := buf.String()

	// Top 5 (by savings) should be shown: r1, r2, r3, r4, r5
	assert.Contains(t, output, "r1")
	assert.Contains(t, output, "r2")
	assert.Contains(t, output, "r3")
	assert.Contains(t, output, "r4")
	assert.Contains(t, output, "r5")

	// r6 and r7 should NOT be shown
	assert.NotContains(t, output, "r6")
	assert.NotContains(t, output, "r7")

	// Should show "Use --verbose" hint
	assert.Contains(t, output, "--verbose")
}

// ============================================================================
// Phase 5: US3 - Filter Interaction Tests (T026-T030)
// ============================================================================

// T026: Test filter + summary mode interaction.
// Verifies that filtering applies BEFORE summary calculation.
func TestFilterWithSummaryMode(t *testing.T) {
	// Create 7 recommendations with different action types
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
		{ResourceID: "r3", Type: "RIGHTSIZE", EstimatedSavings: 80.00, Currency: "USD"},
		{ResourceID: "r4", Type: "DELETE_UNUSED", EstimatedSavings: 70.00, Currency: "USD"},
		{ResourceID: "r5", Type: "MODIFY", EstimatedSavings: 60.00, Currency: "USD"},
		{ResourceID: "r6", Type: "RIGHTSIZE", EstimatedSavings: 50.00, Currency: "USD"},
		{ResourceID: "r7", Type: "TERMINATE", EstimatedSavings: 40.00, Currency: "USD"},
	}

	// Apply filter for RIGHTSIZE only
	filtered, err := cli.ApplyActionTypeFilterForTest(recs, "action=RIGHTSIZE")
	require.NoError(t, err)

	// Should only have 3 RIGHTSIZE recommendations
	assert.Len(t, filtered, 3)

	// Render in non-verbose mode (summary mode)
	var buf bytes.Buffer
	err = cli.RenderRecommendationsTableVerboseForTest(&buf, filtered, false)
	require.NoError(t, err)
	output := buf.String()

	// Summary should show Total Recommendations: 3 (not 7)
	assert.Contains(t, output, "Total Recommendations: 3")

	// Summary should only show RIGHTSIZE savings (100 + 80 + 50 = 230)
	assert.Contains(t, output, "230.00 USD")

	// Only RIGHTSIZE resources should appear
	assert.Contains(t, output, "r1")
	assert.Contains(t, output, "r3")
	assert.Contains(t, output, "r6")

	// Other types should NOT appear
	assert.NotContains(t, output, "r2") // TERMINATE
	assert.NotContains(t, output, "r4") // DELETE_UNUSED
	assert.NotContains(t, output, "r5") // MODIFY
	assert.NotContains(t, output, "r7") // TERMINATE
}

// T027: Test filter + verbose mode interaction.
// Verifies that filtering + verbose shows ALL filtered recommendations.
func TestFilterWithVerboseMode(t *testing.T) {
	// Create 10 recommendations - 6 RIGHTSIZE, 4 TERMINATE
	recs := []cli.TestableRecommendation{
		{ResourceID: "right1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "term1", Type: "TERMINATE", EstimatedSavings: 95.00, Currency: "USD"},
		{ResourceID: "right2", Type: "RIGHTSIZE", EstimatedSavings: 90.00, Currency: "USD"},
		{ResourceID: "term2", Type: "TERMINATE", EstimatedSavings: 85.00, Currency: "USD"},
		{ResourceID: "right3", Type: "RIGHTSIZE", EstimatedSavings: 80.00, Currency: "USD"},
		{ResourceID: "term3", Type: "TERMINATE", EstimatedSavings: 75.00, Currency: "USD"},
		{ResourceID: "right4", Type: "RIGHTSIZE", EstimatedSavings: 70.00, Currency: "USD"},
		{ResourceID: "term4", Type: "TERMINATE", EstimatedSavings: 65.00, Currency: "USD"},
		{ResourceID: "right5", Type: "RIGHTSIZE", EstimatedSavings: 60.00, Currency: "USD"},
		{ResourceID: "right6", Type: "RIGHTSIZE", EstimatedSavings: 55.00, Currency: "USD"},
	}

	// Apply filter for RIGHTSIZE only (6 items)
	filtered, err := cli.ApplyActionTypeFilterForTest(recs, "action=RIGHTSIZE")
	require.NoError(t, err)
	assert.Len(t, filtered, 6)

	// Render in verbose mode - should show ALL 6 filtered items
	var buf bytes.Buffer
	err = cli.RenderRecommendationsTableVerboseForTest(&buf, filtered, true)
	require.NoError(t, err)
	output := buf.String()

	// All 6 RIGHTSIZE resources should appear
	assert.Contains(t, output, "right1")
	assert.Contains(t, output, "right2")
	assert.Contains(t, output, "right3")
	assert.Contains(t, output, "right4")
	assert.Contains(t, output, "right5")
	assert.Contains(t, output, "right6")

	// No TERMINATE resources should appear
	assert.NotContains(t, output, "term1")
	assert.NotContains(t, output, "term2")
	assert.NotContains(t, output, "term3")
	assert.NotContains(t, output, "term4")

	// Should NOT show "Use --verbose" since we're already in verbose mode
	assert.NotContains(t, output, "Use --verbose")

	// Summary should show Total Recommendations: 6
	assert.Contains(t, output, "Total Recommendations: 6")
}

// T028: Test applyActionTypeFilter applies correctly.
func TestApplyActionTypeFilter(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
		{ResourceID: "r3", Type: "MIGRATE", EstimatedSavings: 80.00, Currency: "USD"},
	}

	tests := []struct {
		name        string
		filter      string
		wantCount   int
		wantTypes   []string
		wantErr     bool
		errContains string
	}{
		{
			name:      "filter single action type",
			filter:    "action=RIGHTSIZE",
			wantCount: 1,
			wantTypes: []string{"RIGHTSIZE"},
		},
		{
			name:      "filter multiple action types",
			filter:    "action=RIGHTSIZE,TERMINATE",
			wantCount: 2,
			wantTypes: []string{"RIGHTSIZE", "TERMINATE"},
		},
		{
			name:      "filter no matches",
			filter:    "action=DELETE_UNUSED",
			wantCount: 0,
			wantTypes: []string{},
		},
		{
			name:      "not an action filter - unchanged",
			filter:    "something=else",
			wantCount: 3, // unchanged
			wantTypes: []string{"RIGHTSIZE", "TERMINATE", "MIGRATE"},
		},
		{
			name:        "invalid action type",
			filter:      "action=INVALID_TYPE",
			wantErr:     true,
			errContains: "invalid action type",
		},
		{
			name:      "case insensitive filter",
			filter:    "action=rightsize",
			wantCount: 1,
			wantTypes: []string{"RIGHTSIZE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, err := cli.ApplyActionTypeFilterForTest(recs, tt.filter)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Len(t, filtered, tt.wantCount)

			// Verify all returned recommendations have expected types
			for _, rec := range filtered {
				found := false
				for _, wantType := range tt.wantTypes {
					if rec.Type == wantType {
						found = true
						break
					}
				}
				assert.True(t, found, "unexpected type %s in filtered results", rec.Type)
			}
		})
	}
}

// T030: Test invalid action type error message is clear.
func TestInvalidActionTypeErrorMessage(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
	}

	_, err := cli.ApplyActionTypeFilterForTest(recs, "action=NOT_A_REAL_TYPE")
	require.Error(t, err)

	// Error message should be descriptive
	errMsg := err.Error()
	assert.Contains(t, errMsg, "invalid action type")
}

// ============================================================================
// Phase 6: US5 - JSON Enhancement Tests (T031-T038)
// ============================================================================

// T031: Test JSON output includes summary structure.
func TestRenderRecommendationsJSON_WithSummary(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
		{ResourceID: "r3", Type: "RIGHTSIZE", EstimatedSavings: 80.00, Currency: "USD"},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Parse JSON output
	var output map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	// Check summary exists
	summary, ok := output["summary"].(map[string]interface{})
	require.True(t, ok, "summary field should exist and be an object")

	// Verify summary fields
	assert.Equal(t, float64(3), summary["total_count"])
	assert.Equal(t, float64(270), summary["total_savings"])
	assert.Equal(t, "USD", summary["currency"])

	// Verify count_by_action_type breakdown
	countByAction, ok := summary["count_by_action_type"].(map[string]interface{})
	require.True(t, ok, "count_by_action_type should exist")
	assert.Equal(t, float64(2), countByAction["RIGHTSIZE"])
	assert.Equal(t, float64(1), countByAction["TERMINATE"])

	// Verify savings_by_action_type breakdown
	savingsByAction, ok := summary["savings_by_action_type"].(map[string]interface{})
	require.True(t, ok, "savings_by_action_type should exist")
	assert.Equal(t, float64(180), savingsByAction["RIGHTSIZE"]) // 100 + 80
	assert.Equal(t, float64(90), savingsByAction["TERMINATE"])

	// Verify recommendations array still exists
	recommendations, ok := output["recommendations"].([]interface{})
	require.True(t, ok, "recommendations array should exist")
	assert.Len(t, recommendations, 3)
}

// T032: Test NDJSON output includes summary as first line.
func TestRenderRecommendationsNDJSON_WithSummary(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsNDJSONForTest(&buf, recs)
	require.NoError(t, err)

	// Split by newlines
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.GreaterOrEqual(t, len(lines), 3, "should have at least 3 lines (1 summary + 2 recs)")

	// First line should be summary
	var summary map[string]interface{}
	err = json.Unmarshal([]byte(lines[0]), &summary)
	require.NoError(t, err)

	// Verify it's a summary line (has type: "summary")
	assert.Equal(t, "summary", summary["type"])
	assert.Equal(t, float64(2), summary["total_count"])
	assert.Equal(t, float64(190), summary["total_savings"])

	// Verify count_by_action_type exists
	countByAction, ok := summary["count_by_action_type"].(map[string]interface{})
	require.True(t, ok, "count_by_action_type should exist")
	assert.Equal(t, float64(1), countByAction["RIGHTSIZE"])
	assert.Equal(t, float64(1), countByAction["TERMINATE"])

	// Remaining lines should be recommendations
	for i := 1; i < len(lines); i++ {
		var rec map[string]interface{}
		err = json.Unmarshal([]byte(lines[i]), &rec)
		require.NoError(t, err)
		assert.Contains(t, rec, "resource_id", "line %d should be a recommendation", i)
	}
}

// ============================================================================
// Phase 5: US3 - Include Dismissed Tests (T018)
// ============================================================================

// T018: Test --include-dismissed flag exists on the command.
func TestCostRecommendationsCmd_IncludeDismissedFlag(t *testing.T) {
	cmd := cli.NewCostRecommendationsCmd()

	flag := cmd.Flags().Lookup("include-dismissed")
	require.NotNil(t, flag, "include-dismissed flag should exist")
	assert.Equal(t, "false", flag.DefValue, "default should be false")
}

// T018: Test hasStatusAnnotations detects status annotations.
func TestHasStatusAnnotations(t *testing.T) {
	tests := []struct {
		name   string
		recs   []cli.TestableRecommendation
		expect bool
	}{
		{
			name:   "no recs",
			recs:   nil,
			expect: false,
		},
		{
			name: "all active (no status)",
			recs: []cli.TestableRecommendation{
				{ResourceID: "r1", Type: "RIGHTSIZE"},
				{ResourceID: "r2", Type: "TERMINATE"},
			},
			expect: false,
		},
		{
			name: "all active (explicit)",
			recs: []cli.TestableRecommendation{
				{ResourceID: "r1", Status: "Active"},
				{ResourceID: "r2", Status: "Active"},
			},
			expect: false,
		},
		{
			name: "has dismissed",
			recs: []cli.TestableRecommendation{
				{ResourceID: "r1", Status: "Active"},
				{ResourceID: "r2", Status: "Dismissed"},
			},
			expect: true,
		},
		{
			name: "has snoozed",
			recs: []cli.TestableRecommendation{
				{ResourceID: "r1", Status: "Active"},
				{ResourceID: "r2", Status: "Snoozed"},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cli.HasStatusAnnotationsForTest(tt.recs)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// T018: Test merge of dismissed/snoozed records with active recommendations.
func TestMergeDismissedRecommendations(t *testing.T) {
	// Create a temp dismissal store with test data
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	// Add dismissed record with LastKnown
	now := time.Now()
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: "rec-dismissed-1",
		Status:           config.StatusDismissed,
		Reason:           "BUSINESS_CONSTRAINT",
		DismissedAt:      now,
		LastKnown: &config.LastKnownRecommendation{
			Description:      "Rightsize web-server",
			EstimatedSavings: 45.00,
			Currency:         "USD",
			Type:             "RIGHTSIZE",
			ResourceID:       "aws:ec2:web-server",
		},
		History: []config.LifecycleEvent{
			{Action: config.ActionDismissed, Reason: "BUSINESS_CONSTRAINT", Timestamp: now},
		},
	}))

	// Add snoozed record with LastKnown
	future := now.Add(30 * 24 * time.Hour)
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: "rec-snoozed-1",
		Status:           config.StatusSnoozed,
		Reason:           "DEFERRED",
		DismissedAt:      now,
		ExpiresAt:        &future,
		LastKnown: &config.LastKnownRecommendation{
			Description:      "Terminate idle db",
			EstimatedSavings: 120.00,
			Currency:         "USD",
			Type:             "TERMINATE",
			ResourceID:       "aws:rds:idle-db",
		},
		History: []config.LifecycleEvent{
			{Action: config.ActionSnoozed, Reason: "DEFERRED", Timestamp: now, ExpiresAt: &future},
		},
	}))

	require.NoError(t, store.Save())

	// Active recommendations
	activeRecs := []cli.TestableRecommendation{
		{ResourceID: "aws:ec2:active-1", Type: "MIGRATE", EstimatedSavings: 30.00, Currency: "USD"},
		{ResourceID: "aws:s3:active-2", Type: "DELETE_UNUSED", EstimatedSavings: 5.00, Currency: "USD"},
	}

	// Merge
	merged, mergeErr := cli.MergeDismissedRecommendationsForTest(activeRecs, storePath)
	require.NoError(t, mergeErr)

	// Should have 4 total: 2 active + 1 dismissed + 1 snoozed
	require.Len(t, merged, 4)

	// Verify active recs are present (no status set in test data)
	activeCount := 0
	dismissedCount := 0
	snoozedCount := 0
	for _, rec := range merged {
		switch rec.Status {
		case "Dismissed":
			dismissedCount++
			assert.Equal(t, "aws:ec2:web-server", rec.ResourceID)
			assert.Equal(t, "RIGHTSIZE", rec.Type)
			assert.Equal(t, 45.00, rec.EstimatedSavings)
		case "Snoozed":
			snoozedCount++
			assert.Equal(t, "aws:rds:idle-db", rec.ResourceID)
			assert.Equal(t, "TERMINATE", rec.Type)
			assert.Equal(t, 120.00, rec.EstimatedSavings)
		default:
			activeCount++
		}
	}

	assert.Equal(t, 2, activeCount, "should have 2 active recommendations")
	assert.Equal(t, 1, dismissedCount, "should have 1 dismissed recommendation")
	assert.Equal(t, 1, snoozedCount, "should have 1 snoozed recommendation")
}

// T018: Test merge with empty dismissal store.
func TestMergeDismissedRecommendations_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	activeRecs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
	}

	merged, err := cli.MergeDismissedRecommendationsForTest(activeRecs, storePath)
	require.NoError(t, err)
	assert.Len(t, merged, 1, "no dismissed records should be added")
}

// T018: Test merge does not duplicate active recommendations.
func TestMergeDismissedRecommendations_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	// Add a dismissed record whose LastKnown.ResourceID matches an active rec's ResourceID.
	// The RecommendationID is different from the ResourceID (realistic scenario).
	now := time.Now()
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: "rec-dismiss-001", // Plugin-assigned recommendation ID
		Status:           config.StatusDismissed,
		Reason:           "NOT_APPLICABLE",
		DismissedAt:      now,
		LastKnown: &config.LastKnownRecommendation{
			ResourceID: "aws:ec2:active-1", // Matches active rec below
			Type:       "RIGHTSIZE",
		},
		History: []config.LifecycleEvent{
			{Action: config.ActionDismissed, Reason: "NOT_APPLICABLE", Timestamp: now},
		},
	}))
	require.NoError(t, store.Save())

	activeRecs := []cli.TestableRecommendation{
		{ResourceID: "aws:ec2:active-1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
	}

	merged, mergeErr := cli.MergeDismissedRecommendationsForTest(activeRecs, storePath)
	require.NoError(t, mergeErr)
	// Should NOT duplicate — dismissed record's LastKnown.ResourceID matches the active rec's ResourceID,
	// even though the RecommendationID ("rec-dismiss-001") differs from the ResourceID ("aws:ec2:active-1")
	assert.Len(t, merged, 1, "should not duplicate active recommendation")
}

// T018: Test table rendering with status column shows Status header.
func TestRenderRecommendationsTable_WithStatusColumn(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD", Status: "Active"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 45.00, Currency: "USD", Status: "Dismissed"},
		{ResourceID: "r3", Type: "MIGRATE", EstimatedSavings: 120.00, Currency: "USD", Status: "Snoozed"},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsTableVerboseForTest(&buf, recs, true)
	require.NoError(t, err)
	output := buf.String()

	// Status column header should be present
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "Active")
	assert.Contains(t, output, "Dismissed")
	assert.Contains(t, output, "Snoozed")
}

// T018: Test table rendering without status column (all active).
func TestRenderRecommendationsTable_WithoutStatusColumn(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 90.00, Currency: "USD"},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsTableVerboseForTest(&buf, recs, true)
	require.NoError(t, err)
	output := buf.String()

	// STATUS header should NOT be present when all recs are active
	assert.NotContains(t, output, "STATUS")
}

// T018: Test JSON output includes status field when present.
func TestRenderRecommendationsJSON_WithStatus(t *testing.T) {
	recs := []cli.TestableRecommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD", Status: "Active"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 45.00, Currency: "USD", Status: "Dismissed"},
	}

	var buf bytes.Buffer
	err := cli.RenderRecommendationsJSONForTest(&buf, recs)
	require.NoError(t, err)

	var output map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	recommendations, ok := output["recommendations"].([]interface{})
	require.True(t, ok)
	require.Len(t, recommendations, 2)

	// First rec should have status "Active"
	rec1, ok := recommendations[0].(map[string]interface{})
	require.True(t, ok, "recommendations[0] should be a map")
	assert.Equal(t, "Active", rec1["status"])

	// Second rec should have status "Dismissed"
	rec2, ok := recommendations[1].(map[string]interface{})
	require.True(t, ok, "recommendations[1] should be a map")
	assert.Equal(t, "Dismissed", rec2["status"])
}

// T018: Test dismissed record without LastKnown shows minimal info.
func TestMergeDismissedRecommendations_NoLastKnown(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: "rec-no-snapshot",
		Status:           config.StatusDismissed,
		Reason:           "INACCURATE",
		DismissedAt:      now,
		// No LastKnown
		History: []config.LifecycleEvent{
			{Action: config.ActionDismissed, Reason: "INACCURATE", Timestamp: now},
		},
	}))
	require.NoError(t, store.Save())

	activeRecs := []cli.TestableRecommendation{}

	merged, mergeErr := cli.MergeDismissedRecommendationsForTest(activeRecs, storePath)
	require.NoError(t, mergeErr)
	require.Len(t, merged, 1)

	assert.Equal(t, "Dismissed", merged[0].Status)
	assert.Equal(t, "rec-no-snapshot", merged[0].ResourceID)
	assert.Contains(t, merged[0].Description, "Dismissed recommendation")
}
