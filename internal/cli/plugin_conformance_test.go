package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewPluginConformanceCmd(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginConformanceCmd()

	assert.Equal(t, "conformance <plugin-path>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestPluginConformanceCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginConformanceCmd()

	// Check all required flags exist
	expectedFlags := []string{
		"mode",
		"verbosity",
		"output",
		"output-file",
		"timeout",
		"category",
		"filter",
	}

	for _, flag := range expectedFlags {
		assert.NotNil(t, cmd.Flags().Lookup(flag), "flag %q should exist", flag)
	}
}

func TestPluginConformanceCmd_FlagDefaults(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginConformanceCmd()

	assert.Equal(t, "tcp", cmd.Flags().Lookup("mode").DefValue)
	assert.Equal(t, "normal", cmd.Flags().Lookup("verbosity").DefValue)
	assert.Equal(t, "table", cmd.Flags().Lookup("output").DefValue)
	assert.Equal(t, "", cmd.Flags().Lookup("output-file").DefValue)
	assert.Equal(t, "5m", cmd.Flags().Lookup("timeout").DefValue)
	assert.Equal(t, "", cmd.Flags().Lookup("filter").DefValue)
}

func TestPluginConformanceCmd_RequiresArg(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	rootCmd := cli.NewRootCmd("test")

	// Find the conformance command
	conformanceCmd, _, err := rootCmd.Find([]string{"plugin", "conformance"})
	require.NoError(t, err)
	require.NotNil(t, conformanceCmd)

	// Execute without argument should fail
	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "conformance"})

	assert.NotEqual(t, 0, result.ExitCode)
	// axtest.Run should indicate missing argument failure
}

func TestPluginConformanceCmd_InvalidMode(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "conformance", "--mode", "invalid", pluginPath},
	)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "invalid mode")
}

func TestPluginConformanceCmd_InvalidVerbosity(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "conformance", "--verbosity", "invalid", pluginPath},
	)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "invalid verbosity")
}

func TestPluginConformanceCmd_InvalidOutput(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "conformance", "--output", "invalid", pluginPath},
	)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "invalid output format")
}

func TestPluginConformanceCmd_InvalidTimeout(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "conformance", "--timeout", "invalid", pluginPath},
	)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "invalid timeout")
}

func TestPluginConformanceCmd_InvalidCategory(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(
		context.Background(),
		t,
		rootCmd,
		[]string{"plugin", "conformance", "--category", "invalid", pluginPath},
	)

	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "invalid category")
}

func TestPluginConformanceCmd_PluginNotFound(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	rootCmd := cli.NewRootCmd("test")

	result := axtest.Run(context.Background(), t, rootCmd, []string{"plugin", "conformance", "/nonexistent/plugin"})

	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "plugin not found")
}

func TestPluginConformanceCmd_CommandRegistered(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	rootCmd := cli.NewRootCmd("test")

	// Verify the command is properly registered
	conformanceCmd, _, err := rootCmd.Find([]string{"plugin", "conformance"})
	require.NoError(t, err)
	require.NotNil(t, conformanceCmd)

	assert.Equal(t, "conformance <plugin-path>", conformanceCmd.Use)
}
