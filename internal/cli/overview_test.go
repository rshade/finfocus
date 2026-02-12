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

func TestNewOverviewCmd_MissingRequiredFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var buf bytes.Buffer
	cmd := cli.NewOverviewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
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

	// This will try to open plugins which may fail, but it validates that
	// state loading and merge work correctly
	err := cmd.Execute()
	// May fail at plugin opening stage - that's expected
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
	assert.NotNil(t, cmd.Flags().Lookup("from"))
	assert.NotNil(t, cmd.Flags().Lookup("to"))
	assert.NotNil(t, cmd.Flags().Lookup("adapter"))
	assert.NotNil(t, cmd.Flags().Lookup("output"))
	assert.NotNil(t, cmd.Flags().Lookup("filter"))
	assert.NotNil(t, cmd.Flags().Lookup("plain"))
	assert.NotNil(t, cmd.Flags().Lookup("yes"))
	assert.NotNil(t, cmd.Flags().Lookup("no-pagination"))
}

func TestNewOverviewCmd_YesShortFlag(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	cmd := cli.NewOverviewCmd()
	yesFlag := cmd.Flags().Lookup("yes")
	require.NotNil(t, yesFlag)
	assert.Equal(t, "y", yesFlag.Shorthand)
}
