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

func TestNewPluginCertifyCmd(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginCertifyCmd()

	assert.Equal(t, "certify <plugin-path>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
}

func TestPluginCertifyCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginCertifyCmd()

	// Check all required flags exist
	expectedFlags := []string{
		"output",
		"mode",
		"timeout",
	}

	for _, flag := range expectedFlags {
		assert.NotNil(t, cmd.Flags().Lookup(flag), "flag %q should exist", flag)
	}
}

func TestPluginCertifyCmd_FlagDefaults(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginCertifyCmd()

	assert.Equal(t, "", cmd.Flags().Lookup("output").DefValue)
	assert.Equal(t, "tcp", cmd.Flags().Lookup("mode").DefValue)
	assert.Equal(t, "10m", cmd.Flags().Lookup("timeout").DefValue)
}

func TestPluginCertifyCmd_RequiresArg(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	rootCmd := cli.NewRootCmd("test")

	// Find the certify command
	certifyCmd, _, err := rootCmd.Find([]string{"plugin", "certify"})
	require.NoError(t, err)
	require.NotNil(t, certifyCmd)

	// Execute without argument should fail
	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"plugin", "certify"})

	err = rootCmd.Execute()

	assert.Error(t, err)
	// Cobra should report missing argument
}

func TestPluginCertifyCmd_InvalidMode(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"plugin", "certify", "--mode", "invalid", pluginPath})

	err = rootCmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

func TestPluginCertifyCmd_InvalidTimeout(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	// Create a temporary file to act as a plugin
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0o755)
	require.NoError(t, err)

	rootCmd := cli.NewRootCmd("test")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"plugin", "certify", "--timeout", "invalid", pluginPath})

	err = rootCmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout")
}

func TestPluginCertifyCmd_PluginNotFound(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	rootCmd := cli.NewRootCmd("test")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"plugin", "certify", "/nonexistent/plugin"})

	err := rootCmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")
}

func TestPluginCertifyCmd_CommandRegistered(t *testing.T) {
	// Note: Cannot use t.Parallel() - tests that execute rootCmd modify global logger state

	rootCmd := cli.NewRootCmd("test")

	// Verify the command is properly registered
	certifyCmd, _, err := rootCmd.Find([]string{"plugin", "certify"})
	require.NoError(t, err)
	require.NotNil(t, certifyCmd)

	assert.Equal(t, "certify <plugin-path>", certifyCmd.Use)
}

func TestPluginCertifyCmd_OutputShortFlag(t *testing.T) {
	t.Parallel()

	cmd := cli.NewPluginCertifyCmd()

	// Verify short flag exists for output
	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "o", outputFlag.Shorthand)
}
