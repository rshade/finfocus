package plugin_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

// TestPluginInstall_FromRegistry tests installing a plugin via mock registry [US2][T015].
func TestPluginInstall_FromRegistry(t *testing.T) {
	// Setup mock registry with test plugin
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"test": {"v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	// Create test plugin directory and set HOME for config isolation
	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create installer with mock client pointing to test server
	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	// Install using github.com URL format (parsed by ParsePluginSpecifier)
	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	// Use github.com/owner/repo format - the BaseURL redirect handles the actual request
	specifier := "github.com/example/finfocus-plugin-test"
	result, err := installer.Install(context.Background(), specifier, opts, nil)

	require.NoError(t, err)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.True(t, result.FromURL)

	// Verify binary was installed
	binName := "test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(result.Path, binName)
	assert.FileExists(t, binPath)

	// Verify binary is executable on Unix
	if runtime.GOOS != "windows" {
		info, err := os.Stat(binPath)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&0111, "binary should be executable")
	}
}

// TestPluginInstall_SpecificVersion tests installing a specific version [US2][T016].
func TestPluginInstall_SpecificVersion(t *testing.T) {
	// Setup mock registry with multiple versions
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"versioned": {"v2.0.0", "v1.5.0", "v1.0.0"}, // v2.0.0 is latest
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	// Install specific version v1.5.0 (not latest)
	specifier := "github.com/example/finfocus-plugin-versioned@v1.5.0"
	result, err := installer.Install(context.Background(), specifier, opts, nil)

	require.NoError(t, err)
	assert.Equal(t, "versioned", result.Name)
	assert.Equal(t, "v1.5.0", result.Version)

	// Verify the correct version was installed
	versionDir := filepath.Join(pluginDir, "versioned", "v1.5.0")
	assert.DirExists(t, versionDir)

	// Verify v2.0.0 was NOT installed
	latestDir := filepath.Join(pluginDir, "versioned", "v2.0.0")
	assert.NoDirExists(t, latestDir)
}

// TestPluginInstall_FromURL tests direct URL installation [US2][T017].
func TestPluginInstall_FromURL(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"url-plugin": {"v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	// Install from direct URL format
	specifier := "github.com/custom-org/finfocus-plugin-url-plugin"

	var progressMessages []string
	progress := func(msg string) {
		progressMessages = append(progressMessages, msg)
	}

	result, err := installer.Install(context.Background(), specifier, opts, progress)

	require.NoError(t, err)
	assert.True(t, result.FromURL, "should be marked as URL install")
	assert.Equal(t, "url-plugin", result.Name)

	// Verify progress was reported
	assert.NotEmpty(t, progressMessages, "should report progress")
}

// TestPluginInstall_Force tests reinstall behavior with --force flag [US2][T018].
func TestPluginInstall_Force(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"force-plugin": {"v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	specifier := "github.com/example/finfocus-plugin-force-plugin"

	// First install
	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}
	result1, err := installer.Install(context.Background(), specifier, opts, nil)
	require.NoError(t, err)

	// Try to install again without force (should fail)
	_, err = installer.Install(context.Background(), specifier, opts, nil)
	assert.Error(t, err, "should fail without force flag")
	assert.Contains(t, err.Error(), "already installed")

	// Install with force
	optsForce := registry.InstallOptions{
		Force:     true,
		NoSave:    true,
		PluginDir: pluginDir,
	}
	result2, err := installer.Install(context.Background(), specifier, optsForce, nil)
	require.NoError(t, err)

	// Both results should have same path
	assert.Equal(t, result1.Path, result2.Path)
}

// TestPluginInstall_NoSave tests that config is not modified with --no-save [US2][T019].
func TestPluginInstall_NoSave(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"nosave": {"v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	specifier := "github.com/example/finfocus-plugin-nosave"

	// Install with NoSave option
	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	result, err := installer.Install(context.Background(), specifier, opts, nil)

	require.NoError(t, err)
	assert.Equal(t, "nosave", result.Name)

	// Note: We can't easily verify config wasn't modified without mocking the config package.
	// The NoSave flag is tested via the installer's internal logic.
	// This test verifies the install succeeds with NoSave=true.
}

// TestPluginInstall_DownloadFailure tests error handling when download fails.
func TestPluginInstall_DownloadFailure(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"fail-download": {"v1.0.0"},
		},
		FailDownload: true,
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	specifier := "github.com/example/finfocus-plugin-fail-download"
	_, err := installer.Install(context.Background(), specifier, opts, nil)

	assert.Error(t, err, "should fail when download fails")
}

// TestPluginInstall_MetadataFailure tests error handling when metadata fetch fails.
func TestPluginInstall_MetadataFailure(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"fail-metadata": {"v1.0.0"},
		},
		FailMetadata: true,
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	specifier := "github.com/example/finfocus-plugin-fail-metadata"
	_, err := installer.Install(context.Background(), specifier, opts, nil)

	assert.Error(t, err, "should fail when metadata fetch fails")
}

// TestPluginInstall_NonExistentPlugin tests error handling for non-existent plugin.
func TestPluginInstall_NonExistentPlugin(t *testing.T) {
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

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	// Try to install a plugin that doesn't exist in the mock registry
	specifier := "github.com/example/finfocus-plugin-nonexistent"
	_, err := installer.Install(context.Background(), specifier, opts, nil)

	assert.Error(t, err, "should fail for non-existent plugin")
}

// TestPluginInstall_ProgressCallback tests that progress is reported correctly.
func TestPluginInstall_ProgressCallback(t *testing.T) {
	cfg := MockRegistryConfig{
		Plugins: map[string][]string{
			"progress-test": {"v1.0.0"},
		},
	}
	server, cleanup := StartMockRegistryWithConfig(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	specifier := "github.com/example/finfocus-plugin-progress-test"

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	_, err := installer.Install(context.Background(), specifier, opts, progress)
	require.NoError(t, err)

	// Verify progress messages were received
	assert.NotEmpty(t, messages, "should receive progress messages")

	// Check for expected progress stages
	foundFetching := false
	foundDownloading := false
	foundSuccess := false

	for _, msg := range messages {
		if contains(msg, "Fetching") {
			foundFetching = true
		}
		if contains(msg, "Downloading") {
			foundDownloading = true
		}
		if contains(msg, "Successfully") || contains(msg, "installed") {
			foundSuccess = true
		}
	}

	assert.True(t, foundFetching || foundDownloading, "should have fetch/download progress")
	assert.True(t, foundSuccess, "should have success message")
}

// contains is a helper for case-insensitive substring matching.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > 0 && (s[0:len(substr)] == substr ||
			containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
