package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

// setupConfigInitTest sets common env vars and registers cleanup for global state.
func setupConfigInitTest(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("FINFOCUS_HIDE_ALIAS_HINT", "1")
	t.Cleanup(func() {
		config.ResetGlobalConfigForTest()
		config.SetResolvedProjectDir("")
	})
}

// TestConfigInit_InsidePulumiProject verifies that running "config init" inside
// a directory containing Pulumi.yaml creates project-local .finfocus/config.hujson
// and .finfocus/.gitignore.
func TestConfigInit_InsidePulumiProject(t *testing.T) {
	setupConfigInitTest(t)

	tmpDir := t.TempDir()

	// Create a minimal Pulumi.yaml so the project is detected
	pulumiYAML := filepath.Join(tmpDir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(pulumiYAML, []byte("name: test-project\nruntime: go\n"), 0o644))

	// Use FINFOCUS_PROJECT_DIR env var to simulate being inside the project
	// (avoids leaking the package-level projectDirFlag variable to other tests)
	t.Setenv("FINFOCUS_PROJECT_DIR", tmpDir)

	// Point FINFOCUS_HOME to an isolated global dir so we don't touch the real home
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)

	// Execute config init through the root command
	cmd := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, cmd, []string{"config", "init"})

	require.Equal(t, 0, result.ExitCode, "config init should succeed inside a Pulumi project")

	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized at")

	// Verify project-local config.hujson was created
	configPath := filepath.Join(tmpDir, ".finfocus", "config.hujson")
	_, statErr := os.Stat(configPath)
	require.NoError(t, statErr, ".finfocus/config.hujson should exist")

	// Verify .gitignore was created
	gitignorePath := filepath.Join(tmpDir, ".finfocus", ".gitignore")
	_, statErr = os.Stat(gitignorePath)
	require.NoError(t, statErr, ".finfocus/.gitignore should exist")

	// Verify .gitignore contains expected content
	gitignoreData, readErr := os.ReadFile(gitignorePath)
	require.NoError(t, readErr)
	assert.Equal(t, config.GitignoreContent(), string(gitignoreData),
		".gitignore content should match standard template")
}

// TestConfigInit_ExistingGitignorePreserved verifies that running "config init --force"
// does NOT overwrite an existing .gitignore file (FR-007: never overwrite .gitignore).
func TestConfigInit_ExistingGitignorePreserved(t *testing.T) {
	setupConfigInitTest(t)

	tmpDir := t.TempDir()

	// Create Pulumi.yaml
	pulumiYAML := filepath.Join(tmpDir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(pulumiYAML, []byte("name: test-project\nruntime: go\n"), 0o644))

	// Create .finfocus directory with a pre-existing custom .gitignore
	finfocusDir := filepath.Join(tmpDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0o750))

	customContent := "# My custom gitignore\n*.secret\n"
	gitignorePath := filepath.Join(finfocusDir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte(customContent), 0o644))

	// Use FINFOCUS_PROJECT_DIR env var to simulate being inside the project
	t.Setenv("FINFOCUS_PROJECT_DIR", tmpDir)

	// Point FINFOCUS_HOME to an isolated global dir
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)

	// Execute config init with --force (should overwrite config.hujson but NOT .gitignore)
	cmd := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, cmd, []string{"config", "init", "--force"})

	require.Equal(t, 0, result.ExitCode, "config init --force should succeed")

	// Verify .gitignore was NOT overwritten
	gitignoreData, readErr := os.ReadFile(gitignorePath)
	require.NoError(t, readErr)
	assert.Equal(t, customContent, string(gitignoreData),
		".gitignore should preserve custom content and not be overwritten")
}

// TestConfigInit_GlobalFlag verifies that using --global creates configuration
// in the global FINFOCUS_HOME directory instead of project-local.
func TestConfigInit_GlobalFlag(t *testing.T) {
	setupConfigInitTest(t)

	tmpDir := t.TempDir()

	// Create Pulumi.yaml so we ARE inside a Pulumi project
	pulumiYAML := filepath.Join(tmpDir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(pulumiYAML, []byte("name: test-project\nruntime: go\n"), 0o644))

	// Use FINFOCUS_PROJECT_DIR env var to simulate being inside the project
	t.Setenv("FINFOCUS_PROJECT_DIR", tmpDir)

	// Point FINFOCUS_HOME to an isolated global directory
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)

	// Execute config init with --global flag
	cmd := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, cmd, []string{"config", "init", "--global"})

	require.Equal(t, 0, result.ExitCode, "config init --global should succeed")

	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized successfully")

	// Verify global config was created in FINFOCUS_HOME
	globalConfigPath := filepath.Join(globalDir, "config.hujson")
	_, statErr := os.Stat(globalConfigPath)
	require.NoError(t, statErr, "global config.hujson should exist in FINFOCUS_HOME")

	// Verify NO project-local config was created
	projectConfigPath := filepath.Join(tmpDir, ".finfocus", "config.hujson")
	_, statErr = os.Stat(projectConfigPath)
	assert.True(t, os.IsNotExist(statErr),
		"project-local config.hujson should NOT exist when --global is used")
}

// TestConfigInit_OutsidePulumiProject verifies that running "config init" outside
// a Pulumi project directory falls back to global configuration init.
func TestConfigInit_OutsidePulumiProject(t *testing.T) {
	setupConfigInitTest(t)

	// Point FINFOCUS_HOME to an isolated global directory
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)

	// Ensure FINFOCUS_PROJECT_DIR is NOT set so the resolution falls through
	// to Pulumi project walk-up, which will fail since the test temp dir has
	// no Pulumi.yaml. Use NewConfigInitCmd directly to avoid the root
	// PersistentPreRunE resolving against the real cwd.
	cmd := cli.NewConfigInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.NoError(t, err, "config init should succeed outside a Pulumi project (falls back to global)")

	output := buf.String()
	assert.Contains(t, output, "Configuration initialized successfully",
		"should show global init message when outside Pulumi project")

	// Verify global config was created
	globalConfigPath := filepath.Join(globalDir, "config.hujson")
	_, statErr := os.Stat(globalConfigPath)
	require.NoError(t, statErr, "global config.hujson should be created when outside Pulumi project")
}

// TestConfigInit_ForceOverwritesConfig verifies that running "config init --force"
// overwrites an existing config.hujson file with fresh defaults.
func TestConfigInit_ForceOverwritesConfig(t *testing.T) {
	setupConfigInitTest(t)

	tmpDir := t.TempDir()

	// Create Pulumi.yaml
	pulumiYAML := filepath.Join(tmpDir, "Pulumi.yaml")
	require.NoError(t, os.WriteFile(pulumiYAML, []byte("name: test-project\nruntime: go\n"), 0o644))

	// Create existing config.hujson with custom content
	finfocusDir := filepath.Join(tmpDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0o750))

	existingConfig := filepath.Join(finfocusDir, "config.hujson")
	originalContent := "# old config\noutput:\n  default_format: json\n"
	require.NoError(t, os.WriteFile(existingConfig, []byte(originalContent), 0o644))

	// Use FINFOCUS_PROJECT_DIR env var to simulate being inside the project
	t.Setenv("FINFOCUS_PROJECT_DIR", tmpDir)

	// Point FINFOCUS_HOME to an isolated global dir
	globalDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", globalDir)

	// Execute config init with --force
	cmd := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, cmd, []string{"config", "init", "--force"})

	require.Equal(t, 0, result.ExitCode, "config init --force should succeed")

	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized at")

	// Verify config.hujson was overwritten (content should differ from original)
	newContent, readErr := os.ReadFile(existingConfig)
	require.NoError(t, readErr)
	assert.NotEqual(t, originalContent, string(newContent),
		"config.hujson should be overwritten with new default content")
	assert.NotEmpty(t, string(newContent), "config.hujson should not be empty after force init")
}

func TestConfigInit_LegacyProjectConfigPreserved(t *testing.T) {
	setupConfigInitTest(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	projectDir := t.TempDir()
	config.SetResolvedProjectDir(projectDir)
	legacyPath := filepath.Join(projectDir, "config.yaml")
	content := []byte("output:\n  default_format: json\n")
	require.NoError(t, os.WriteFile(legacyPath, content, 0600))

	cmd := cli.NewConfigInitCmd()
	cmd.SetContext(context.Background())
	err := cmd.RunE(cmd, nil)
	require.ErrorContains(t, err, "configuration file already exists")
	assert.NoFileExists(t, filepath.Join(projectDir, "config.hujson"))
	got, err := os.ReadFile(legacyPath)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}
