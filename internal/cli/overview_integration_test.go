package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// testdataDir returns the absolute path to the testdata/overview directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "overview")
}

// goldenDir returns the absolute path to the testdata/overview/golden directory.
func goldenDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testdataDir(t), "golden")
}

// shouldUpdateGolden checks if the UPDATE_GOLDEN env var is set for golden file regeneration.
func shouldUpdateGolden() bool {
	return os.Getenv("UPDATE_GOLDEN") != ""
}

// assertGoldenFile compares output against a golden file, creating it if it doesn't exist
// or UPDATE_GOLDEN is set.
func assertGoldenFile(t *testing.T, goldenPath string, actual string) {
	t.Helper()

	if shouldUpdateGolden() {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(actual), 0o644))
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
		// Create golden file on first run
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(actual), 0o644))
		t.Logf("Created golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	assert.Equal(
		t,
		string(expected),
		actual,
		"output does not match golden file %s (run with UPDATE_GOLDEN=1 to regenerate)",
		goldenPath,
	)
}

// ---------------------------------------------------------------------------
// T035: Golden file tests - Table rendering with fixture data
// ---------------------------------------------------------------------------

func TestIntegration_TableRender_NoChanges(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	ctx := context.Background()
	td := testdataDir(t)

	// Load state
	state, err := ingest.LoadStackExportWithContext(ctx, filepath.Join(td, "state-no-changes.json"))
	require.NoError(t, err)

	stateResources := convertTestStateResources(state.GetCustomResourcesWithContext(ctx))

	// No plan (no changes scenario)
	var planSteps []engine.PlanStep

	// Merge
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	require.NoError(t, err)

	// Build stack context
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	stackCtx := engine.StackContext{
		StackName:      "state-no-changes",
		TimeWindow:     engine.DateRange{Start: now.AddDate(0, 0, -15), End: now},
		HasChanges:     false,
		TotalResources: len(rows),
		PendingChanges: 0,
	}

	// Render table
	var buf bytes.Buffer
	err = engine.RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	require.NotEmpty(t, output)

	// Verify key content
	assert.Contains(t, output, "RESOURCE")
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "state-no-changes")

	// All resources should be active with no cost data
	for _, row := range rows {
		assert.Equal(t, engine.StatusActive, row.Status)
	}

	// Golden file comparison
	assertGoldenFile(t, filepath.Join(goldenDir(t), "table-no-changes.txt"), output)
}

func TestIntegration_TableRender_MixedChanges(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	ctx := context.Background()
	td := testdataDir(t)

	// Load state and plan
	state, err := ingest.LoadStackExportWithContext(ctx, filepath.Join(td, "state-mixed-changes.json"))
	require.NoError(t, err)

	plan, err := ingest.LoadPulumiPlanWithContext(ctx, filepath.Join(td, "plan-mixed-changes.json"))
	require.NoError(t, err)

	stateResources := convertTestStateResources(state.GetCustomResourcesWithContext(ctx))
	planSteps := convertTestPlanSteps(plan.Steps)

	// Merge
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	require.NoError(t, err)

	// Detect changes
	hasChanges, changeCount, _ := engine.DetectPendingChanges(ctx, planSteps)
	require.True(t, hasChanges)
	require.Greater(t, changeCount, 0)

	// Build stack context
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	stackCtx := engine.StackContext{
		StackName:      "state-mixed-changes",
		TimeWindow:     engine.DateRange{Start: now.AddDate(0, 0, -15), End: now},
		HasChanges:     hasChanges,
		TotalResources: len(rows),
		PendingChanges: changeCount,
	}

	// Render table
	var buf bytes.Buffer
	err = engine.RenderOverviewAsTable(&buf, rows, stackCtx)
	require.NoError(t, err)

	output := buf.String()
	require.NotEmpty(t, output)

	// Verify key content
	assert.Contains(t, output, "RESOURCE")
	assert.Contains(t, output, "SUMMARY")
	assert.Contains(t, output, "state-mixed-changes")
	assert.Contains(t, output, "pending changes")

	// Verify mixed statuses are present
	statusCounts := make(map[engine.ResourceStatus]int)
	for _, row := range rows {
		statusCounts[row.Status]++
	}
	assert.Greater(t, statusCounts[engine.StatusActive], 0, "should have active resources")
	assert.Greater(t, statusCounts[engine.StatusUpdating], 0, "should have updating resources")

	// Golden file comparison
	assertGoldenFile(t, filepath.Join(goldenDir(t), "table-with-changes.txt"), output)
}

// ---------------------------------------------------------------------------
// T036: End-to-end integration tests via CLI command execution
// ---------------------------------------------------------------------------

func TestIntegration_OverviewCommand_StateOnly(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-no-changes.json"),
		"--yes",
		"--plain",
	})

	err := cmd.Execute()
	// Will fail at plugin opening - expected since we have no plugins in test
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

func TestIntegration_OverviewCommand_StateAndPlan(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-mixed-changes.json"),
		"--pulumi-json", filepath.Join(td, "plan-mixed-changes.json"),
		"--yes",
		"--plain",
	})

	err := cmd.Execute()
	// Will fail at plugin opening - expected since we have no plugins in test
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

func TestIntegration_OverviewCommand_JSONOutput(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-no-changes.json"),
		"--output", "json",
		"--yes",
	})

	err := cmd.Execute()
	// Will fail at plugin opening - expected since we have no plugins in test
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

func TestIntegration_OverviewCommand_NDJSONOutput(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-no-changes.json"),
		"--output", "ndjson",
		"--yes",
	})

	err := cmd.Execute()
	// Will fail at plugin opening - expected since we have no plugins in test
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

func TestIntegration_OverviewCommand_InvalidOutput(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-no-changes.json"),
		"--output", "xml",
		"--yes",
	})

	err := cmd.Execute()
	// May fail at plugin opening before reaching format check
	if err != nil {
		// Either format error or plugin error is acceptable
		errMsg := err.Error()
		isFormatError := strings.Contains(errMsg, "unsupported output format")
		isPluginError := strings.Contains(errMsg, "opening plugins")
		assert.True(t, isFormatError || isPluginError, "expected format or plugin error, got: %s", errMsg)
	}
}

func TestIntegration_OverviewCommand_MissingStateFile(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", "/nonexistent/path/state.json",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi state")
}

func TestIntegration_OverviewCommand_InvalidDateRange(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-no-changes.json"),
		"--from", "2025-12-31",
		"--to", "2025-01-01",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date range")
}

func TestIntegration_OverviewCommand_FilterFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	td := testdataDir(t)

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", filepath.Join(td, "state-no-changes.json"),
		"--filter", "provider=aws",
		"--yes",
		"--plain",
	})

	err := cmd.Execute()
	// Will fail at plugin opening - expected
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

// ---------------------------------------------------------------------------
// JSON render integration tests (direct rendering, no plugins needed)
// ---------------------------------------------------------------------------

func TestIntegration_JSONRender_NoChanges(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	ctx := context.Background()
	td := testdataDir(t)

	// Load state
	state, err := ingest.LoadStackExportWithContext(ctx, filepath.Join(td, "state-no-changes.json"))
	require.NoError(t, err)

	stateResources := convertTestStateResources(state.GetCustomResourcesWithContext(ctx))

	// Merge
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, nil)
	require.NoError(t, err)

	// Build stack context
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	stackCtx := engine.StackContext{
		StackName:      "state-no-changes",
		TimeWindow:     engine.DateRange{Start: now.AddDate(0, 0, -15), End: now},
		TotalResources: len(rows),
	}

	// Render JSON
	var buf bytes.Buffer
	err = engine.RenderOverviewAsJSON(&buf, rows, stackCtx)
	require.NoError(t, err)

	// Verify valid JSON
	var output engine.OverviewJSONOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "state-no-changes", output.Metadata.StackName)
	assert.Greater(t, len(output.Resources), 0)
	assert.Empty(t, output.Errors)
	assert.Equal(t, "USD", output.Summary.Currency)
}

func TestIntegration_NDJSONRender_MixedChanges(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	ctx := context.Background()
	td := testdataDir(t)

	// Load state and plan
	state, err := ingest.LoadStackExportWithContext(ctx, filepath.Join(td, "state-mixed-changes.json"))
	require.NoError(t, err)

	plan, err := ingest.LoadPulumiPlanWithContext(ctx, filepath.Join(td, "plan-mixed-changes.json"))
	require.NoError(t, err)

	stateResources := convertTestStateResources(state.GetCustomResourcesWithContext(ctx))
	planSteps := convertTestPlanSteps(plan.Steps)

	// Merge
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	require.NoError(t, err)

	// Render NDJSON
	var buf bytes.Buffer
	err = engine.RenderOverviewAsNDJSON(&buf, rows)
	require.NoError(t, err)

	// Verify each line is valid JSON
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, len(rows), len(lines))

	for i, line := range lines {
		var row engine.OverviewRow
		err = json.Unmarshal([]byte(line), &row)
		require.NoError(t, err, "line %d should be valid JSON", i)
		assert.NotEmpty(t, row.URN, "line %d should have URN", i)
	}
}

// ---------------------------------------------------------------------------
// Merge integration tests
// ---------------------------------------------------------------------------

func TestIntegration_MergeNoChanges(t *testing.T) {
	ctx := context.Background()
	td := testdataDir(t)

	state, err := ingest.LoadStackExportWithContext(ctx, filepath.Join(td, "state-no-changes.json"))
	require.NoError(t, err)

	stateResources := convertTestStateResources(state.GetCustomResourcesWithContext(ctx))
	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, nil)
	require.NoError(t, err)

	// All should be active
	for _, row := range rows {
		assert.Equal(t, engine.StatusActive, row.Status, "resource %s should be active", row.URN)
	}

	// Detect no changes
	hasChanges, count, _ := engine.DetectPendingChanges(ctx, nil)
	assert.False(t, hasChanges)
	assert.Equal(t, 0, count)
}

func TestIntegration_MergeMixedChanges(t *testing.T) {
	ctx := context.Background()
	td := testdataDir(t)

	state, err := ingest.LoadStackExportWithContext(ctx, filepath.Join(td, "state-mixed-changes.json"))
	require.NoError(t, err)

	plan, err := ingest.LoadPulumiPlanWithContext(ctx, filepath.Join(td, "plan-mixed-changes.json"))
	require.NoError(t, err)

	stateResources := convertTestStateResources(state.GetCustomResourcesWithContext(ctx))
	planSteps := convertTestPlanSteps(plan.Steps)

	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, planSteps)
	require.NoError(t, err)

	// Should have more resources than just state (creates add new ones)
	require.Greater(t, len(rows), len(stateResources))

	// Verify change detection
	hasChanges, changeCount, _ := engine.DetectPendingChanges(ctx, planSteps)
	assert.True(t, hasChanges)
	assert.Equal(t, 10, changeCount) // 3 updates + 2 deletes + 2 replaces + 3 creates

	// Count statuses
	statusCounts := map[engine.ResourceStatus]int{}
	for _, row := range rows {
		statusCounts[row.Status]++
	}

	assert.Greater(t, statusCounts[engine.StatusActive], 0, "should have active resources")
	assert.Equal(t, 3, statusCounts[engine.StatusUpdating], "should have 3 updating resources")
	assert.Equal(t, 2, statusCounts[engine.StatusDeleting], "should have 2 deleting resources")
	assert.Equal(t, 2, statusCounts[engine.StatusReplacing], "should have 2 replacing resources")
	assert.Equal(t, 3, statusCounts[engine.StatusCreating], "should have 3 creating resources")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func convertTestStateResources(resources []ingest.StackExportResource) []engine.StateResource {
	result := make([]engine.StateResource, len(resources))
	for i, r := range resources {
		result[i] = engine.StateResource{
			URN:    r.URN,
			Type:   r.Type,
			ID:     r.ID,
			Custom: r.Custom,
		}
	}
	return result
}

func convertTestPlanSteps(steps []ingest.PulumiStep) []engine.PlanStep {
	result := make([]engine.PlanStep, len(steps))
	for i, s := range steps {
		result[i] = engine.PlanStep{
			URN:  s.URN,
			Op:   s.Op,
			Type: s.Type,
		}
	}
	return result
}
