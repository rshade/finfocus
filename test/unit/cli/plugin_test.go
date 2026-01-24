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

// TestPluginListCmd_NoPlugins tests listing with no plugins installed.
func TestPluginListCmd_NoPlugins(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set HOME to temp directory to ensure no plugins are found
	t.Setenv("HOME", t.TempDir())

	cmd := cli.NewPluginListCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	// Should succeed even with no plugins
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "No plugins")
}

// TestPluginListCmd_WithPlugins tests listing with mock plugins.
func TestPluginListCmd_WithPlugins(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Create temporary plugin directory structure
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, ".finfocus", "plugins")

	// Create mock plugin directories
	kubecostDir := filepath.Join(pluginDir, "kubecost", "v0.1.0")
	err := os.MkdirAll(kubecostDir, 0755)
	require.NoError(t, err)

	// Create mock plugin binary
	pluginBinary := filepath.Join(kubecostDir, "finfocus-plugin-kubecost")
	err = os.WriteFile(pluginBinary, []byte("#!/bin/sh\necho test"), 0755)
	require.NoError(t, err)

	// Set HOME to temp directory
	t.Setenv("HOME", tempDir)

	cmd := cli.NewPluginListCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "kubecost")
	assert.Contains(t, output, "v0.1.0")
}

// TestPluginValidateCmd_NoPlugins tests validation with no plugins.
func TestPluginValidateCmd_NoPlugins(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Set HOME to temp directory to ensure no plugins are found
	t.Setenv("HOME", t.TempDir())

	cmd := cli.NewPluginValidateCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()

	// Should succeed and report no plugins
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "No plugins")
}

// TestPluginValidateCmd_ValidPlugin tests validation with valid plugin.
func TestPluginValidateCmd_ValidPlugin(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Create temporary plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, ".finfocus", "plugins")
	kubecostDir := filepath.Join(pluginDir, "kubecost", "v0.1.0")
	err := os.MkdirAll(kubecostDir, 0755)
	require.NoError(t, err)

	// Create valid executable
	pluginBinary := filepath.Join(kubecostDir, "finfocus-plugin-kubecost")
	err = os.WriteFile(pluginBinary, []byte("#!/bin/sh\necho test"), 0755)
	require.NoError(t, err)

	// Set HOME to temp directory
	t.Setenv("HOME", tempDir)

	cmd := cli.NewPluginValidateCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "kubecost")
	assert.Contains(t, output, "valid") // Or "✓" depending on implementation
}

// TestPluginValidateCmd_NonExecutable tests validation skips non-executable files.
func TestPluginValidateCmd_NonExecutable(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Create temporary plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, ".finfocus", "plugins")
	testDir := filepath.Join(pluginDir, "test-plugin", "v0.1.0")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	// Create non-executable file (no execute permissions)
	pluginBinary := filepath.Join(testDir, "finfocus-plugin-test")
	err = os.WriteFile(pluginBinary, []byte("#!/bin/sh\necho test"), 0644) // 0644 = not executable
	require.NoError(t, err)

	// Set HOME to temp directory
	t.Setenv("HOME", tempDir)

	cmd := cli.NewPluginValidateCmd()

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Registry filters out non-executable files during discovery, so no plugins found
	assert.Contains(t, output, "No plugins")
}

// TestPluginListCmd_VerboseOutput tests verbose output for plugin list.
func TestPluginListCmd_VerboseOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	// Create temporary plugin directory
	tempDir := t.TempDir()
	pluginDir := filepath.Join(tempDir, ".finfocus", "plugins")
	kubecostDir := filepath.Join(pluginDir, "kubecost", "v0.1.0")
	err := os.MkdirAll(kubecostDir, 0755)
	require.NoError(t, err)

	// Create plugin binary
	pluginBinary := filepath.Join(kubecostDir, "finfocus-plugin-kubecost")
	err = os.WriteFile(pluginBinary, []byte("#!/bin/sh\necho test"), 0755)
	require.NoError(t, err)

	// Set HOME
	t.Setenv("HOME", tempDir)

	cmd := cli.NewPluginListCmd()
	cmd.SetArgs([]string{"--verbose"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err = cmd.Execute()
	require.NoError(t, err)

	// Verbose output should have extra column
	output := out.String()
	assert.Contains(t, output, "Executable")
	assert.Contains(t, output, "kubecost")
}
