package cli_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

func setupTestConfig(t *testing.T) func() {
	t.Helper()
	testHome := t.TempDir()
	t.Setenv("HOME", testHome)
	t.Setenv("USERPROFILE", testHome) // Windows compatibility
	t.Setenv("FINFOCUS_HOME", filepath.Join(testHome, ".finfocus"))
	// If a reset helper exists, call it here; otherwise noop.
	return func() {}
}

func TestConfigInitCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	root := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, root, []string{"config", "init"})

	// Test successful init
	require.Equal(t, 0, result.ExitCode)

	// Check output message
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized successfully")

	// Verify config can be read
	cfg := config.New()
	require.NotNil(t, cfg)
}

func TestConfigInitCmdForce(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Create existing config
	cfg := config.New()
	err := cfg.Save()
	require.NoError(t, err)

	root := cli.NewRootCmd("test")

	// Test without force flag should fail
	result := axtest.Run(context.Background(), t, root, []string{"config", "init"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "already exists")

	// Test with force flag should succeed
	result = axtest.Run(context.Background(), t, root, []string{"config", "init", "--force"})
	assert.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized successfully")
}

func TestConfigSetCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	root := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "output.default_format", "json"})

	// Test setting output format
	require.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration updated: output.default_format = json")

	// Verify the value was set
	cfg := config.New()
	value, err := cfg.Get("output.default_format")
	require.NoError(t, err)
	assert.Equal(t, "json", value)
}

func TestConfigSetCmdErrors(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	root := cli.NewRootCmd("test")

	// Test invalid key
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "invalid.key", "value"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "unknown configuration section")

	// Test invalid precision value
	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "output.precision", "invalid"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr = string(result.Stderr)
	assert.Contains(t, stderr, "precision must be a number")
}

func TestConfigGetCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Set up some config values
	cfg := config.New()
	require.NoError(t, cfg.Set("output.default_format", "json"))
	require.NoError(t, cfg.Set("plugins.aws.region", "us-west-2"))
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test getting simple value
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "output.default_format"})
	require.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "json\n", string(result.Stdout))

	// Test getting plugin value
	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "plugins.aws.region"})
	require.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "us-west-2\n", string(result.Stdout))
}

func TestConfigGetCmdErrors(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Create initial config file
	cfg := config.New()
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test invalid key
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "invalid.key"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "unknown configuration section")

	// Test non-existent plugin
	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "plugins.nonexistent.key"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr = string(result.Stderr)
	assert.Contains(t, stderr, "plugin not found")
}

func TestConfigListCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	root := cli.NewRootCmd("test")

	// Initialize config first
	result := axtest.Run(context.Background(), t, root, []string{"config", "init"})
	require.Equal(t, 0, result.ExitCode)

	// Set some config values
	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "output.default_format", "json"})
	require.Equal(t, 0, result.ExitCode)

	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "plugins.aws.region", "us-west-2"})
	require.Equal(t, 0, result.ExitCode)

	// Test YAML output. Pass --as explicitly rather than relying on the
	// no-flag TTY-detection default: axtest.Run's stdout is a buffer, never a
	// real terminal, so ax's mode resolution would otherwise fall back to
	// JSON - and ax.WithStdoutIsTTY(true) does not reliably override that
	// once a root command has already served an earlier axtest.Run call in
	// this test (a reused-root quirk in ax-go, not something to work around
	// here; --as sidesteps it since it doesn't depend on mode resolution).
	result = axtest.Run(context.Background(), t, root, []string{"config", "list", "--as", "yaml"})
	require.Equal(t, 0, result.ExitCode)

	yamlOutput := string(result.Stdout)
	assert.Contains(t, yamlOutput, "output:")
	assert.Contains(t, yamlOutput, "default_format: json")
	assert.Contains(t, yamlOutput, "plugins:")
	assert.Contains(t, yamlOutput, "aws:")
	assert.Contains(t, yamlOutput, "region: us-west-2")

	// Test JSON output. Use --as (not --format) here: this root has already
	// served a "config list --as yaml" call above, and Cobra flags carry
	// their Changed() state across Execute() calls on a reused *cobra.Command
	// (axtest.Run's own docs note this - "it does not reset flag values a
	// previous call set"). Passing --format instead would leave --as's
	// Changed() flag stuck true from the earlier call, short-circuiting
	// config_list.go's mode-resolution fallback before --format is ever
	// consulted. Re-asserting --as explicitly avoids relying on that fallback
	// at all, on either call.
	result = axtest.Run(context.Background(), t, root, []string{"config", "list", "--as", "json"})
	require.Equal(t, 0, result.ExitCode)

	jsonOutput := string(result.Stdout)
	assert.Contains(t, jsonOutput, "\"output\":")
	assert.Contains(t, jsonOutput, "\"default_format\": \"json\"")
	assert.Contains(t, jsonOutput, "\"plugins\":")
}

func TestConfigListCmdErrors(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Create initial config file
	cfg := config.New()
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test invalid format
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "list", "--format", "invalid"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "unknown output mode")
}

func TestConfigValidateCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Create initial config file
	cfg := config.New()
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test valid configuration
	result := axtest.Run(context.Background(), t, root, []string{"config", "validate"})
	require.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "✅ Configuration is valid")

	// Test with verbose flag
	result = axtest.Run(context.Background(), t, root, []string{"config", "validate", "--verbose"})
	require.Equal(t, 0, result.ExitCode)

	verboseOutput := string(result.Stdout)
	assert.Contains(t, verboseOutput, "✅ Configuration is valid")
	assert.Contains(t, verboseOutput, "Configuration details:")
	assert.Contains(t, verboseOutput, "Output format:")
	assert.Contains(t, verboseOutput, "Logging level:")
}

func TestConfigValidateCmdErrors(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Set invalid configuration
	cfg := config.New()
	// Ensure file exists with current defaults first.
	require.NoError(t, cfg.Save())
	// Reload, mutate, and save invalid field.
	cfg.Output.DefaultFormat = "invalid"
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test invalid configuration
	result := axtest.Run(context.Background(), t, root, []string{"config", "validate"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "invalid output format")
}

func TestConfigCommandsIntegration(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	root := cli.NewRootCmd("test")

	// Test full workflow: init -> set -> get -> validate -> list

	// 1. Initialize config
	result := axtest.Run(context.Background(), t, root, []string{"config", "init"})
	require.Equal(t, 0, result.ExitCode)

	// 2. Set some values
	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "output.default_format", "json"})
	require.Equal(t, 0, result.ExitCode)

	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "plugins.aws.region", "eu-west-1"})
	require.Equal(t, 0, result.ExitCode)

	// 3. Get values to verify
	result = axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "output.default_format"})
	require.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "json\n", string(result.Stdout))

	// 4. Validate configuration
	result = axtest.Run(context.Background(), t, root, []string{"config", "validate"})
	require.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "✅ Configuration is valid")

	// 5. List all configuration (explicit --as yaml; see the comment in
	// TestConfigListCmd on why the no-flag TTY-detection default isn't used
	// here on a root command already reused by earlier calls in this test).
	result = axtest.Run(context.Background(), t, root, []string{"config", "list", "--as", "yaml"})
	require.Equal(t, 0, result.ExitCode)

	listOutput := string(result.Stdout)
	assert.Contains(t, listOutput, "default_format: json")
	assert.Contains(t, listOutput, "region: eu-west-1")
}

func TestConfigCmdWrongArgs(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	root := cli.NewRootCmd("test")

	// Test set command with wrong number of args
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "set", "only-one-arg"})
	assert.NotEqual(t, 0, result.ExitCode)

	// Test get command with wrong number of args
	result = axtest.Run(context.Background(), t, root, []string{"config", "get"})
	assert.NotEqual(t, 0, result.ExitCode)
}

func TestConfigGetCmdMapOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Set up config with plugins
	cfg := config.New()
	require.NoError(t, cfg.Set("plugins.aws.region", "us-west-2"))
	require.NoError(t, cfg.Set("plugins.aws.account_id", "123456789"))
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test getting plugin section (returns map)
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "plugins.aws"})
	require.Equal(t, 0, result.ExitCode)

	mapOutput := string(result.Stdout)
	assert.Contains(t, mapOutput, "plugins.aws:")
	assert.Contains(t, mapOutput, "region:")

	// Test getting all plugins (returns map of PluginConfig)
	result = axtest.Run(context.Background(), t, root, []string{"config", "get", "plugins"})
	require.Equal(t, 0, result.ExitCode)

	allPluginsOutput := string(result.Stdout)
	assert.Contains(t, allPluginsOutput, "plugins:")
	assert.Contains(t, allPluginsOutput, "aws:")
}

func TestConfigGetCmdIntOutput(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Set up config with precision (integer)
	cfg := config.New()
	require.NoError(t, cfg.Set("output.precision", "4"))
	require.NoError(t, cfg.Save())

	root := cli.NewRootCmd("test")

	// Test getting integer value
	result := axtest.Run(context.Background(), t, root,
		[]string{"config", "get", "output.precision"})
	require.Equal(t, 0, result.ExitCode)

	assert.Contains(t, string(result.Stdout), "4")
}
