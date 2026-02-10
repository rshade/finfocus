package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/registry"
)

// setupTestEnv sets up test isolation by configuring log level and using a temporary home directory.
func setupTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("FINFOCUS_HOME", t.TempDir())
}

func TestNewPluginValidateCmd(t *testing.T) {
	setupTestEnv(t)

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no flags",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "plugin flag with nonexistent plugin",
			args:        []string{"--plugin", "test-plugin"},
			expectError: true, // Plugin doesn't exist, so command should error
		},
		{
			name:        "help flag",
			args:        []string{"--help"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := cli.NewPluginValidateCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPluginValidateCmdFlags(t *testing.T) {
	setupTestEnv(t)

	cmd := cli.NewPluginValidateCmd()

	// Check plugin flag
	pluginFlag := cmd.Flags().Lookup("plugin")
	assert.NotNil(t, pluginFlag)
	assert.Equal(t, "string", pluginFlag.Value.Type())
	assert.Empty(t, pluginFlag.DefValue)
	assert.Contains(t, pluginFlag.Usage, "Validate a specific plugin by name")
}

func TestPluginValidateCmdHelp(t *testing.T) {
	setupTestEnv(t)

	var buf bytes.Buffer
	cmd := cli.NewPluginValidateCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Validate that installed plugins can be loaded and respond to basic API calls")
	assert.Contains(t, output, "--plugin")
	assert.Contains(t, output, "Validate a specific plugin by name")
	assert.Equal(t, "Validate installed plugins", cmd.Short)
}

func TestPluginValidateCmdExamples(t *testing.T) {
	setupTestEnv(t)

	cmd := cli.NewPluginValidateCmd()

	// Check that examples are present
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "finfocus plugin validate")
	assert.Contains(t, cmd.Example, "finfocus plugin validate --plugin aws-plugin")
	assert.Contains(t, cmd.Example, "Validate a specific plugin")
	assert.Contains(t, cmd.Example, "kubecost")
}

func TestValidatePlugin(t *testing.T) {
	setupTestEnv(t)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Use platform-appropriate binary name and extension
	pluginName := "test-plugin"
	if runtime.GOOS == "windows" {
		pluginName = "test-plugin.exe"
	}

	// Create a mock plugin binary
	pluginPath := filepath.Join(tmpDir, pluginName)
	err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho test"), 0755)
	require.NoError(t, err)

	// Create a valid manifest
	manifestPath := filepath.Join(tmpDir, "plugin.manifest.json")
	manifestContent := `{
		"name": "test-plugin",
		"version": "1.0.0",
		"description": "Test plugin"
	}`
	err = os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	// Use platform-appropriate name for the non-executable test
	nonExecName := "non-exec"
	if runtime.GOOS == "windows" {
		nonExecName = "non-exec.txt" // Use .txt to ensure it's not recognized as executable
	}

	tests := []struct {
		name        string
		plugin      registry.PluginInfo
		setupFunc   func()
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid plugin",
			plugin: registry.PluginInfo{
				Name:    "test-plugin",
				Version: "1.0.0",
				Path:    pluginPath,
			},
			expectError: false,
		},
		{
			name: "plugin not found",
			plugin: registry.PluginInfo{
				Name:    "missing",
				Version: "1.0.0",
				Path:    filepath.Join(tmpDir, "missing"),
			},
			expectError: true,
			errorMsg:    "plugin binary not found",
		},
		{
			name: "plugin is directory",
			plugin: registry.PluginInfo{
				Name:    "dir-plugin",
				Version: "1.0.0",
				Path:    tmpDir,
			},
			expectError: true,
			errorMsg:    "plugin path is a directory",
		},
		{
			name: "plugin not executable",
			plugin: registry.PluginInfo{
				Name:    "non-exec",
				Version: "1.0.0",
				Path:    filepath.Join(tmpDir, nonExecName),
			},
			setupFunc: func() {
				// Create non-executable file
				nonExecPath := filepath.Join(tmpDir, nonExecName)
				writeErr := os.WriteFile(nonExecPath, []byte("test"), 0644)
				require.NoError(t, writeErr)
			},
			expectError: true,
			errorMsg:    "plugin binary is not executable", // matches both Unix and Windows messages
		},
		{
			name: "manifest name mismatch",
			plugin: registry.PluginInfo{
				Name:    "wrong-name",
				Version: "1.0.0",
				Path:    pluginPath,
			},
			expectError: true,
			errorMsg:    "manifest name mismatch",
		},
		{
			name: "manifest version mismatch",
			plugin: registry.PluginInfo{
				Name:    "test-plugin",
				Version: "2.0.0",
				Path:    pluginPath,
			},
			expectError: true,
			errorMsg:    "manifest version mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			ctx := context.Background()
			validateErr := cli.ValidatePlugin(ctx, tt.plugin)

			if tt.expectError {
				require.Error(t, validateErr)
				if tt.errorMsg != "" {
					assert.Contains(t, validateErr.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, validateErr)
			}
		})
	}
}
