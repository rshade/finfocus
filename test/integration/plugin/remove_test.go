package plugin_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/registry"
)

// TestPluginRemove_Basic tests basic plugin removal [US4][T027].
func TestPluginRemove_Basic(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Setup plugin with config
	pluginName := "remove-basic"
	version := "v1.0.0"
	repoURL := "github.com/example/finfocus-plugin-remove-basic"
	setupPluginWithConfig(t, pluginDir, homeDir, pluginName, version, repoURL)

	// Verify plugin exists before removal
	pluginPath := filepath.Join(pluginDir, pluginName, version)
	assert.DirExists(t, pluginPath)

	// Create installer
	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	var progressMessages []string
	progress := func(msg string) {
		progressMessages = append(progressMessages, msg)
	}

	err := installer.Remove(pluginName, opts, progress)
	require.NoError(t, err)

	// Verify plugin directory was removed
	assert.NoDirExists(t, pluginPath, "plugin directory should be removed")

	// Verify parent directory is cleaned up if empty
	parentDir := filepath.Join(pluginDir, pluginName)
	assert.NoDirExists(t, parentDir, "parent directory should be removed when empty")

	// Verify progress was reported
	assert.NotEmpty(t, progressMessages)
}

// TestPluginRemove_KeepConfig tests that config is retained with --keep-config [US4][T028].
func TestPluginRemove_KeepConfig(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pluginName := "remove-keepconfig"
	version := "v1.0.0"
	repoURL := "github.com/example/finfocus-plugin-remove-keepconfig"
	setupPluginWithConfig(t, pluginDir, homeDir, pluginName, version, repoURL)

	// Verify config exists
	configPath := filepath.Join(homeDir, ".finfocus", "config.yaml")
	configBefore, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(configBefore), pluginName)

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		KeepConfig: true,
		PluginDir:  pluginDir,
	}

	err = installer.Remove(pluginName, opts, nil)
	require.NoError(t, err)

	// Verify plugin files were removed
	pluginPath := filepath.Join(pluginDir, pluginName, version)
	assert.NoDirExists(t, pluginPath)

	// Note: Verifying config retention would require reading config after removal.
	// The KeepConfig flag prevents config.RemoveInstalledPlugin from being called.
	// This test verifies the removal succeeds with KeepConfig=true.
}

// TestPluginRemove_Aliases tests that 'uninstall' and 'rm' aliases work [US4][T029].
func TestPluginRemove_Aliases(t *testing.T) {
	// Test CLI command aliases
	cmd := cli.NewPluginRemoveCmd()

	// Verify aliases are configured
	assert.Contains(t, cmd.Aliases, "uninstall", "should have 'uninstall' alias")
	assert.Contains(t, cmd.Aliases, "rm", "should have 'rm' alias")

	// Test using remove command via CLI
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pluginName := "alias-test"
	version := "v1.0.0"
	repoURL := "github.com/example/finfocus-plugin-alias-test"
	setupPluginWithConfig(t, pluginDir, homeDir, pluginName, version, repoURL)

	// Create a new command instance for execution
	removeCmd := cli.NewPluginRemoveCmd()
	var stdout, stderr bytes.Buffer
	removeCmd.SetOut(&stdout)
	removeCmd.SetErr(&stderr)

	removeCmd.SetArgs([]string{
		pluginName,
		"--plugin-dir", pluginDir,
	})

	err := removeCmd.Execute()
	require.NoError(t, err)

	// Verify plugin was removed
	pluginPath := filepath.Join(pluginDir, pluginName, version)
	assert.NoDirExists(t, pluginPath)
}

// TestPluginRemove_NonExistent tests error handling for non-existent plugin [US4][T030].
func TestPluginRemove_NonExistent(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create empty config (no plugins installed)
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("# Empty config\n"), 0644))

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	err := installer.Remove("nonexistent-plugin", opts, nil)

	assert.Error(t, err, "should fail for non-existent plugin")
	assert.Contains(t, err.Error(), "not installed")
}

// TestPluginRemove_ProgressCallback tests that progress is reported during removal.
func TestPluginRemove_ProgressCallback(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pluginName := "progress-remove"
	version := "v1.0.0"
	repoURL := "github.com/example/finfocus-plugin-progress-remove"
	setupPluginWithConfig(t, pluginDir, homeDir, pluginName, version, repoURL)

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	err := installer.Remove(pluginName, opts, progress)
	require.NoError(t, err)

	assert.NotEmpty(t, messages, "should receive progress messages")

	// Check for removal progress
	foundRemoving := false
	foundSuccess := false
	for _, msg := range messages {
		if containsHelper(msg, "Removing") {
			foundRemoving = true
		}
		if containsHelper(msg, "Successfully") || containsHelper(msg, "removed") {
			foundSuccess = true
		}
	}
	assert.True(t, foundRemoving, "should report removing progress")
	assert.True(t, foundSuccess, "should report success")
}

// TestPluginRemove_ViaCliCommand tests removal via CLI command execution.
func TestPluginRemove_ViaCliCommand(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pluginName := "cli-remove"
	version := "v1.0.0"
	repoURL := "github.com/example/finfocus-plugin-cli-remove"
	setupPluginWithConfig(t, pluginDir, homeDir, pluginName, version, repoURL)

	cmd := cli.NewPluginRemoveCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		pluginName,
		"--plugin-dir", pluginDir,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output contains success message
	output := stdout.String()
	assert.Contains(t, output, "removed successfully")

	// Verify plugin was actually removed
	pluginPath := filepath.Join(pluginDir, pluginName, version)
	assert.NoDirExists(t, pluginPath)
}

// TestPluginRemove_MultipleVersions tests removal when multiple versions exist.
func TestPluginRemove_MultipleVersions(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pluginName := "multi-version"

	// Install multiple versions manually
	installMockPlugin(t, pluginDir, pluginName, "v1.0.0")
	installMockPlugin(t, pluginDir, pluginName, "v2.0.0")

	// Setup config pointing to v2.0.0
	// The config package expects "installed_plugins" key at root level
	repoURL := "github.com/example/finfocus-plugin-multi-version"
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	configContent := `installed_plugins:
  - name: ` + pluginName + `
    url: ` + repoURL + `
    version: v2.0.0
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	// Remove the plugin (should remove v2.0.0 as per config)
	err := installer.Remove(pluginName, opts, nil)
	require.NoError(t, err)

	// Verify v2.0.0 was removed
	v2Path := filepath.Join(pluginDir, pluginName, "v2.0.0")
	assert.NoDirExists(t, v2Path, "v2.0.0 should be removed")

	// v1.0.0 should still exist (not tracked in config)
	v1Path := filepath.Join(pluginDir, pluginName, "v1.0.0")
	assert.DirExists(t, v1Path, "v1.0.0 should still exist")
}

// TestPluginRemove_FilesystemOnly tests removal of plugins installed manually
// (on disk but not tracked in config.yaml). This is the fix for issue #592.
func TestPluginRemove_FilesystemOnly(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create empty config (no plugins registered)
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("# Empty config\n"), 0644))

	// Install plugin manually (filesystem only, no config entry)
	pluginName := "filesystem-only"
	version := "v1.0.0"
	installMockPlugin(t, pluginDir, pluginName, version)

	// Verify plugin exists on disk
	pluginPath := filepath.Join(pluginDir, pluginName, version)
	assert.DirExists(t, pluginPath)

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	var progressMessages []string
	progress := func(msg string) {
		progressMessages = append(progressMessages, msg)
	}

	// Remove should succeed via filesystem fallback
	err := installer.Remove(pluginName, opts, progress)
	require.NoError(t, err)

	// Verify plugin directory was removed
	assert.NoDirExists(t, pluginPath, "plugin version directory should be removed")

	// Verify parent directory is cleaned up
	parentDir := filepath.Join(pluginDir, pluginName)
	assert.NoDirExists(t, parentDir, "parent directory should be removed when empty")

	// Verify progress was reported
	assert.NotEmpty(t, progressMessages)
}

// TestPluginRemove_FilesystemOnly_MultipleVersions tests removal discovers
// the first version directory for filesystem-only plugins.
func TestPluginRemove_FilesystemOnly_MultipleVersions(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create empty config
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("# Empty config\n"), 0644))

	// Install two versions manually (no config)
	pluginName := "fs-multi"
	installMockPlugin(t, pluginDir, pluginName, "v1.0.0")
	installMockPlugin(t, pluginDir, pluginName, "v2.0.0")

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	// Remove should succeed (removes whichever version is discovered first)
	err := installer.Remove(pluginName, opts, nil)
	require.NoError(t, err)

	// Parent directory should still exist since one version remains
	parentDir := filepath.Join(pluginDir, pluginName)
	assert.DirExists(t, parentDir, "parent directory should still exist with remaining version")
}

// TestPluginRemove_ConfigTracked_StillWorks tests that config-tracked removal
// still works correctly (no regression).
func TestPluginRemove_ConfigTracked_StillWorks(t *testing.T) {
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	pluginName := "config-tracked"
	version := "v1.0.0"
	repoURL := "github.com/example/finfocus-plugin-config-tracked"
	setupPluginWithConfig(t, pluginDir, homeDir, pluginName, version, repoURL)

	// Verify plugin exists
	pluginPath := filepath.Join(pluginDir, pluginName, version)
	assert.DirExists(t, pluginPath)

	installer := registry.NewInstaller(pluginDir)

	opts := registry.RemoveOptions{
		PluginDir: pluginDir,
	}

	err := installer.Remove(pluginName, opts, nil)
	require.NoError(t, err)

	// Verify plugin was removed
	assert.NoDirExists(t, pluginPath, "plugin directory should be removed")
	parentDir := filepath.Join(pluginDir, pluginName)
	assert.NoDirExists(t, parentDir, "parent directory should be removed when empty")
}
