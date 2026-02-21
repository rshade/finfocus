package registry_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

// TestNewDefault tests default registry creation.
func TestNewDefault(t *testing.T) {
	reg := registry.NewDefault()
	assert.NotNil(t, reg)
}

// TestListPlugins_EmptyDirectory tests listing with no plugins installed.
func TestListPlugins_EmptyDirectory(t *testing.T) {
	homeDir, _ := createTestHome(t)
	setupTestHome(t, homeDir)

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Empty(t, plugins)
}

// TestListPlugins_NonExistentDirectory tests behavior when plugin directory doesn't exist.
func TestListPlugins_NonExistentDirectory(t *testing.T) {
	homeDir := t.TempDir()
	setupTestHome(t, homeDir)
	// Don't create .finfocus/plugins directory

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Empty(t, plugins, "Should return empty list for non-existent directory")
}

// TestListPlugins_SinglePlugin tests discovery of a single plugin.
func TestListPlugins_SinglePlugin(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)
	createPlugin(t, pluginDir, "test-plugin", "v1.0.0")

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "test-plugin", plugins[0].Name)
	assert.Equal(t, "v1.0.0", plugins[0].Version)
	assert.Contains(t, plugins[0].Path, "test-plugin")
}

// TestListPlugins_MultiplePlugins tests discovery of multiple plugins.
func TestListPlugins_MultiplePlugins(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)
	createPlugin(t, pluginDir, "plugin-a", "v1.0.0")
	createPlugin(t, pluginDir, "plugin-b", "v2.1.0")
	createPlugin(t, pluginDir, "plugin-c", "v0.5.0")

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Len(t, plugins, 3)

	names := []string{plugins[0].Name, plugins[1].Name, plugins[2].Name}
	assert.Contains(t, names, "plugin-a")
	assert.Contains(t, names, "plugin-b")
	assert.Contains(t, names, "plugin-c")
}

// TestListPlugins_MultipleVersions tests discovery of a plugin with multiple versions.
func TestListPlugins_MultipleVersions(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)
	createPlugin(t, pluginDir, "kubecost", "v1.0.0")
	createPlugin(t, pluginDir, "kubecost", "v2.0.0")
	createPlugin(t, pluginDir, "kubecost", "v2.1.0")

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Len(t, plugins, 3, "Should discover all versions")

	for _, plugin := range plugins {
		assert.Equal(t, "kubecost", plugin.Name)
	}

	versions := []string{plugins[0].Version, plugins[1].Version, plugins[2].Version}
	assert.Contains(t, versions, "v1.0.0")
	assert.Contains(t, versions, "v2.0.0")
	assert.Contains(t, versions, "v2.1.0")
}

// TestListPlugins_MixedStructure tests complex plugin directory with multiple plugins and versions.
func TestListPlugins_MixedStructure(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)
	createPlugin(t, pluginDir, "aws-plugin", "v1.0.0")
	createPlugin(t, pluginDir, "aws-plugin", "v1.1.0")
	createPlugin(t, pluginDir, "kubecost", "v2.0.0")
	createPlugin(t, pluginDir, "vantage", "v0.1.0")

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Len(t, plugins, 4)
}

// TestListPlugins_NonExecutableFiles tests that non-executable files are ignored.
func TestListPlugins_NonExecutableFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)

	versionDir := filepath.Join(pluginDir, "test-plugin", "v1.0.0")
	err := os.MkdirAll(versionDir, 0755)
	require.NoError(t, err)

	// Create non-executable file
	nonExecPath := filepath.Join(versionDir, "test-plugin")
	err = os.WriteFile(nonExecPath, []byte("#!/bin/sh\necho test"), 0644) // No execute permission
	require.NoError(t, err)

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Empty(t, plugins, "Should not discover non-executable files")
}

// TestListPlugins_FilesInPluginRoot tests that files in plugin root directory are ignored.
func TestListPlugins_FilesInPluginRoot(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)

	// Create a valid plugin
	createPlugin(t, pluginDir, "valid-plugin", "v1.0.0")

	// Create files in root (should be ignored)
	err := os.WriteFile(filepath.Join(pluginDir, "README.md"), []byte("test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pluginDir, "config.yaml"), []byte("test"), 0644)
	require.NoError(t, err)

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Len(t, plugins, 1, "Should only discover valid plugin, not files")
	assert.Equal(t, "valid-plugin", plugins[0].Name)
}

// TestListPlugins_EmptyVersionDirectory tests behavior with version directory containing no binaries.
func TestListPlugins_EmptyVersionDirectory(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)

	versionDir := filepath.Join(pluginDir, "test-plugin", "v1.0.0")
	err := os.MkdirAll(versionDir, 0755)
	require.NoError(t, err)

	// Directory exists but no binary inside
	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Empty(t, plugins, "Should not discover plugins without binaries")
}

// TestListPlugins_MultipleBinariesInVersionDir tests discovery when version dir has multiple executables.
func TestListPlugins_MultipleBinariesInVersionDir(t *testing.T) {
	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)

	versionDir := filepath.Join(pluginDir, "test-plugin", "v1.0.0")
	err := os.MkdirAll(versionDir, 0755)
	require.NoError(t, err)

	// Create multiple executable files
	createExecutable(t, filepath.Join(versionDir, "binary1"))
	createExecutable(t, filepath.Join(versionDir, "binary2"))
	createExecutable(t, filepath.Join(versionDir, "binary3"))

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	// Should find first executable
	assert.Len(t, plugins, 1, "Should discover plugin with any executable")
}

// TestListPlugins_WindowsExeExtension tests .exe file detection on Windows.
func TestListPlugins_WindowsExeExtension(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)

	versionDir := filepath.Join(pluginDir, "test-plugin", "v1.0.0")
	err := os.MkdirAll(versionDir, 0755)
	require.NoError(t, err)

	// Create .exe file
	exePath := filepath.Join(versionDir, "test-plugin.exe")
	err = os.WriteFile(exePath, []byte("test binary"), 0644)
	require.NoError(t, err)

	// Create non-.exe file (should be ignored on Windows)
	nonExePath := filepath.Join(versionDir, "other-binary")
	err = os.WriteFile(nonExePath, []byte("test"), 0644)
	require.NoError(t, err)

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Contains(t, plugins[0].Path, ".exe")
}

// TestListPlugins_SymlinksHandled tests that symbolic links are handled.
func TestListPlugins_SymlinksHandled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	homeDir, pluginDir := createTestHome(t)
	setupTestHome(t, homeDir)

	// Create real plugin
	createPlugin(t, pluginDir, "real-plugin", "v1.0.0")

	// Create symlink to version directory
	realPath := filepath.Join(pluginDir, "real-plugin", "v1.0.0")
	symlinkPath := filepath.Join(pluginDir, "real-plugin", "latest")
	err := os.Symlink(realPath, symlinkPath)
	if err != nil {
		t.Skip("Symlink creation not supported on this system")
	}

	reg := registry.NewDefault()
	plugins, err := reg.ListPlugins()

	require.NoError(t, err)
	// Should discover at least the real version
	assert.GreaterOrEqual(t, len(plugins), 1)
}

// Helper functions for creating test plugin structures

// createTestHome creates a temporary HOME directory with .finfocus/plugins structure.
// Returns: (homeDir, pluginDir).
func createTestHome(t *testing.T) (string, string) {
	t.Helper()

	homeDir := t.TempDir()
	pluginDir := filepath.Join(homeDir, ".finfocus", "plugins")
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)

	return homeDir, pluginDir
}

// setupTestHome sets HOME environment variable to the test home directory.
func setupTestHome(t *testing.T, homeDir string) {
	t.Helper()

	t.Setenv("HOME", homeDir)
}

// createPlugin creates a plugin directory structure with an executable binary.
func createPlugin(t *testing.T, pluginRoot, name, version string) {
	t.Helper()

	versionDir := filepath.Join(pluginRoot, name, version)
	err := os.MkdirAll(versionDir, 0755)
	require.NoError(t, err)

	var binaryName string
	if runtime.GOOS == "windows" {
		binaryName = name + ".exe"
	} else {
		binaryName = name
	}

	binaryPath := filepath.Join(versionDir, binaryName)
	createExecutable(t, binaryPath)
}

// createExecutable creates an executable file at the specified path.
func createExecutable(t *testing.T, path string) {
	t.Helper()

	content := []byte("#!/bin/sh\necho test")
	err := os.WriteFile(path, content, 0755)
	require.NoError(t, err)
}
