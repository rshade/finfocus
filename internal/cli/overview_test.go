package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// ---------------------------------------------------------------------------
// NewOverviewCmd - Flag parsing and validation
// ---------------------------------------------------------------------------

func TestNewOverviewCmd_NoArgsAutoDetectFails(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// Run from a temp dir with no Pulumi project so auto-detect fails
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--yes"})

	execErr := cmd.Execute()
	require.Error(t, execErr)
	assert.Contains(t, execErr.Error(), "auto-detecting Pulumi project")
}

func TestNewOverviewCmd_HelpFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "unified cost dashboard")
	assert.Contains(t, output, "--pulumi-state")
	assert.Contains(t, output, "--pulumi-json")
	assert.Contains(t, output, "--stack")
	assert.Contains(t, output, "--from")
	assert.Contains(t, output, "--to")
	assert.Contains(t, output, "--adapter")
	assert.Contains(t, output, "--output")
	assert.Contains(t, output, "--filter")
	assert.Contains(t, output, "--plain")
	assert.Contains(t, output, "--yes")
	assert.Contains(t, output, "--no-pagination")
}

func TestNewOverviewCmd_NonExistentStateFile(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--pulumi-state", "/nonexistent/state.json", "--yes"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi state")
}

func TestNewOverviewCmd_NonExistentPlanFile(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	// Create a valid state file
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--pulumi-json", "/nonexistent/plan.json",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading Pulumi plan")
}

func TestNewOverviewCmd_InvalidDateRange(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	// Create a valid state file
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--from", "2025-12-31",
		"--to", "2025-01-01",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date range")
}

func TestNewOverviewCmd_InvalidFromDate(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--from", "not-a-date",
		"--yes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date range")
}

func TestNewOverviewCmd_ValidStateFileEmptyResources(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--yes",
	})

	// cmd.Execute() may succeed or fail with "opening plugins" depending on
	// the test environment. Both outcomes are acceptable because this test
	// validates state loading and merge behaviour, not plugin connectivity.
	err := cmd.Execute()
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

// ---------------------------------------------------------------------------
// Flag parsing tests
// ---------------------------------------------------------------------------

func TestNewOverviewCmd_AllFlagsAccepted(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()

	// Verify all flags are registered
	assert.NotNil(t, cmd.Flags().Lookup("pulumi-json"))
	assert.NotNil(t, cmd.Flags().Lookup("pulumi-state"))
	assert.NotNil(t, cmd.Flags().Lookup("stack"))
	assert.NotNil(t, cmd.Flags().Lookup("from"))
	assert.NotNil(t, cmd.Flags().Lookup("to"))
	assert.NotNil(t, cmd.Flags().Lookup("adapter"))
	assert.NotNil(t, cmd.Flags().Lookup("output"))
	assert.NotNil(t, cmd.Flags().Lookup("filter"))
	assert.NotNil(t, cmd.Flags().Lookup("plain"))
	assert.NotNil(t, cmd.Flags().Lookup("yes"))
	assert.NotNil(t, cmd.Flags().Lookup("no-pagination"))
}

func TestNewOverviewCmd_StackFlagExists(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()
	stackFlag := cmd.Flags().Lookup("stack")
	require.NotNil(t, stackFlag)
	assert.Equal(t, "", stackFlag.DefValue)
	assert.Contains(t, stackFlag.Usage, "Pulumi stack name")
}

func TestNewOverviewCmd_ExplicitStateStillWorks(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	stateJSON := `{"version":3,"deployment":{"manifest":{"time":"2025-01-01T00:00:00Z","magic":"","version":""},"resources":[]}}`
	require.NoError(t, os.WriteFile(statePath, []byte(stateJSON), 0o600))

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--pulumi-state", statePath,
		"--yes",
	})

	// Should still work with explicit --pulumi-state (backwards compatibility).
	// May fail at "opening plugins" depending on test environment.
	err := cmd.Execute()
	if err != nil {
		assert.Contains(t, err.Error(), "opening plugins")
	}
}

func TestNewOverviewCmd_YesShortFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()
	yesFlag := cmd.Flags().Lookup("yes")
	require.NotNil(t, yesFlag)
	assert.Equal(t, "y", yesFlag.Shorthand)
}
