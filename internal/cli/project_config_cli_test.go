package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

// projectTierDir returns a Pulumi project directory whose .finfocus/config.hujson
// holds projectCfg, with an empty global config.
func projectTierDir(t *testing.T, projectCfg string) string {
	t.Helper()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: demo\nruntime: go\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".finfocus"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".finfocus", "config.hujson"), []byte(projectCfg), 0o600))
	return dir
}

func TestNewRootCmdDoesNotInitializeConfig(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	cmd := cli.NewRootCmd("test")
	require.NotNil(t, cmd)

	assert.Nil(t, config.GlobalConfig, "building the command tree must not initialize the config singleton")
}

//nolint:paralleltest // sets env, the global config singleton, and builds a root command
func TestProjectTierBudgetApplied(t *testing.T) {
	dir := projectTierDir(t, `{"cost":{"budgets":{"global":{"amount":10,"currency":"USD"}}}}`)

	result := runCLI(t, projectedArgs("--project-dir", dir)...)

	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Contains(t, string(result.Stdout), "BUDGET STATUS")
	assert.Contains(t, string(result.Stdout), "$10.00")
}

//nolint:paralleltest // sets env, the global config singleton, and builds a root command
func TestProjectTierDefaultOutputFormat(t *testing.T) {
	dir := projectTierDir(t, `{"output":{"default_format":"json"}}`)

	result := runCLI(t, projectedArgs("--project-dir", dir)...)

	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.True(t, json.Valid(result.Stdout), "project default_format=json should produce JSON: %s", result.Stdout)

	result = runCLI(t, projectedArgs("--project-dir", dir, "--output", "table")...)

	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.False(t, json.Valid(result.Stdout), "explicit --output table must win over the project default")
}
