package plugin_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
)

// setupPluginWithConfig installs a mock plugin and adds it to config.
// This is needed because Update requires the plugin to be in config.
// The configDir should be the HOME directory (config is stored at ~/.finfocus/config.yaml).
//
//nolint:unparam // version parameter is intentionally consistent in test setup
func setupPluginWithConfig(t *testing.T, pluginDir, homeDir, name, version, repoURL string) {
	t.Helper()

	// Create plugin directory and binary
	installMockPlugin(t, pluginDir, name, version)

	// Create config directory and file with plugin entry
	// The config package expects "installed_plugins" key at root level
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))

	configPath := filepath.Join(finfocusDir, "config.yaml")
	configContent := `# FinFocus Configuration
installed_plugins:
  - name: ` + name + `
    url: ` + repoURL + `
    version: ` + version + `
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))
}

// TestPluginUpdate_ToLatest tests updating a plugin to the latest version [US3][T021].
func TestPluginUpdate_ToLatest(t *testing.T) {
	// Setup mock registry with v1 and v2
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"updatable": {"v2.0.0", "v1.0.0"}, // v2.0.0 is latest
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Pre-install v1.0.0 with config
	repoURL := "github.com/example/finfocus-plugin-updatable"
	setupPluginWithConfig(t, pluginDir, homeDir, "updatable", "v1.0.0", repoURL)

	// Create installer with mock client
	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.UpdateOptions{
		PluginDir: pluginDir,
	}

	// Update to latest
	result, err := installer.Update(context.Background(), "updatable", opts, nil)

	require.NoError(t, err)
	assert.Equal(t, "updatable", result.Name)
	assert.Equal(t, "v1.0.0", result.OldVersion)
	assert.Equal(t, "v2.0.0", result.NewVersion)
	assert.False(t, result.WasUpToDate)

	// Verify new version directory exists
	newVersionDir := filepath.Join(pluginDir, "updatable", "v2.0.0")
	assert.DirExists(t, newVersionDir)

	// Verify binary exists
	binName := "updatable"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	assert.FileExists(t, filepath.Join(newVersionDir, binName))
}

// TestPluginUpdate_SpecificVersion tests updating to a specific version [US3][T022].
func TestPluginUpdate_SpecificVersion(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"specific-update": {"v3.0.0", "v2.0.0", "v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoURL := "github.com/example/finfocus-plugin-specific-update"
	setupPluginWithConfig(t, pluginDir, homeDir, "specific-update", "v1.0.0", repoURL)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	// Update to v2.0.0 (not latest v3.0.0)
	opts := registry.UpdateOptions{
		Version:   "v2.0.0",
		PluginDir: pluginDir,
	}

	result, err := installer.Update(context.Background(), "specific-update", opts, nil)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.OldVersion)
	assert.Equal(t, "v2.0.0", result.NewVersion)

	// Verify v2.0.0 was installed, not v3.0.0
	v2Dir := filepath.Join(pluginDir, "specific-update", "v2.0.0")
	assert.DirExists(t, v2Dir)

	v3Dir := filepath.Join(pluginDir, "specific-update", "v3.0.0")
	assert.NoDirExists(t, v3Dir)
}

// TestPluginUpdate_DryRun tests dry-run mode where no changes are made [US3][T023].
func TestPluginUpdate_DryRun(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"dryrun-test": {"v2.0.0", "v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoURL := "github.com/example/finfocus-plugin-dryrun-test"
	setupPluginWithConfig(t, pluginDir, homeDir, "dryrun-test", "v1.0.0", repoURL)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.UpdateOptions{
		DryRun:    true,
		PluginDir: pluginDir,
	}

	var progressMessages []string
	progress := func(msg string) {
		progressMessages = append(progressMessages, msg)
	}

	result, err := installer.Update(context.Background(), "dryrun-test", opts, progress)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.OldVersion)
	assert.Equal(t, "v2.0.0", result.NewVersion)
	assert.False(t, result.WasUpToDate)

	// Verify v2.0.0 was NOT actually installed (dry run)
	newVersionDir := filepath.Join(pluginDir, "dryrun-test", "v2.0.0")
	assert.NoDirExists(t, newVersionDir, "dry run should not create new directory")

	// Verify old version still exists
	oldVersionDir := filepath.Join(pluginDir, "dryrun-test", "v1.0.0")
	assert.DirExists(t, oldVersionDir, "old version should still exist")

	// Check progress message mentions dry run
	foundDryRunMsg := false
	for _, msg := range progressMessages {
		if containsHelper(msg, "Would update") {
			foundDryRunMsg = true
			break
		}
	}
	assert.True(t, foundDryRunMsg, "should report dry run action")
}

// TestPluginUpdate_AlreadyUpToDate tests behavior when plugin is already up to date [US3][T024].
func TestPluginUpdate_AlreadyUpToDate(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"uptodate": {"v1.0.0"}, // Only one version
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoURL := "github.com/example/finfocus-plugin-uptodate"
	setupPluginWithConfig(t, pluginDir, homeDir, "uptodate", "v1.0.0", repoURL)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.UpdateOptions{
		PluginDir: pluginDir,
	}

	result, err := installer.Update(context.Background(), "uptodate", opts, nil)

	require.NoError(t, err)
	assert.True(t, result.WasUpToDate, "should report already up to date")
	assert.Equal(t, "v1.0.0", result.OldVersion)
	assert.Equal(t, "v1.0.0", result.NewVersion)
}

// TestPluginUpdate_NonExistent tests error handling for non-existent plugin [US3][T025].
func TestPluginUpdate_NonExistent(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"existing": {"v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create an empty config file (no plugins installed)
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("# Empty config\n"), 0644))

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.UpdateOptions{
		PluginDir: pluginDir,
	}

	_, err := installer.Update(context.Background(), "nonexistent", opts, nil)

	assert.Error(t, err, "should fail for non-existent plugin")
	assert.Contains(t, err.Error(), "not installed")
}

// TestPluginUpdate_ProgressCallback tests that progress is reported during update.
func TestPluginUpdate_ProgressCallback(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"progress-update": {"v2.0.0", "v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoURL := "github.com/example/finfocus-plugin-progress-update"
	setupPluginWithConfig(t, pluginDir, homeDir, "progress-update", "v1.0.0", repoURL)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.UpdateOptions{
		PluginDir: pluginDir,
	}

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	_, err := installer.Update(context.Background(), "progress-update", opts, progress)
	require.NoError(t, err)

	assert.NotEmpty(t, messages, "should receive progress messages")
}

// TestPluginUpdate_RemovesOldVersion tests that old version is removed after update.
func TestPluginUpdate_RemovesOldVersion(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"remove-old": {"v2.0.0", "v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoURL := "github.com/example/finfocus-plugin-remove-old"
	setupPluginWithConfig(t, pluginDir, homeDir, "remove-old", "v1.0.0", repoURL)

	// Verify old version exists before update
	oldDir := filepath.Join(pluginDir, "remove-old", "v1.0.0")
	assert.DirExists(t, oldDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.UpdateOptions{
		PluginDir: pluginDir,
	}

	result, err := installer.Update(context.Background(), "remove-old", opts, nil)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", result.NewVersion)

	// Verify new version exists
	newDir := filepath.Join(pluginDir, "remove-old", "v2.0.0")
	assert.DirExists(t, newDir)

	// Verify old version was removed
	assert.NoDirExists(t, oldDir, "old version should be removed after update")
}

// Ensure config package is imported (needed for side effects in some tests).
var _ = config.New
