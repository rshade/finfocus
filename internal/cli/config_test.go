package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go"
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

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "init"})

	// Test successful init
	require.Equal(t, 0, result.ExitCode)

	// Check output message
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized successfully")

	// Verify config can be read
	cfg := config.New()
	_, err := os.Stat(cfg.ConfigPath())
	require.NoError(t, err)
	require.NoError(t, cfg.Load())
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
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

	// Test without force flag should fail
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "init"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "already exists")

	// Test with force flag should succeed
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "init", "--force"})
	assert.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration initialized successfully")
}

func TestConfigSetCmd(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
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

	// Test invalid key
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "invalid.key", "value"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "unknown configuration section")

	// Test invalid precision value
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "output.precision", "invalid"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr = string(result.Stderr)
	assert.Contains(t, stderr, "precision must be a number")
}

func TestConfigSetPatchesFileWithoutEnvironment(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	setupTestConfig(t)
	home := os.Getenv("FINFOCUS_HOME")
	require.NoError(t, os.MkdirAll(home, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(`{
	  "cost": {"budgets": {"global": {"currency": "USD", "period": "monthly"}}}
	}`), 0o600))

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "cost.budgets.amount", "25"})
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "Configuration updated: cost.budgets.amount = 25")
	data, err := os.ReadFile(filepath.Join(home, "config.hujson"))
	require.NoError(t, err)
	assert.Regexp(t, `"amount":\s*25\b`, string(data))
	assert.NotContains(t, string(data), `"level"`)

	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "output.precision", "4", "--dry-run"})
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "Would set: output.precision = 4")
	dry, err := os.ReadFile(filepath.Join(home, "config.hujson"))
	require.NoError(t, err)
	assert.Equal(t, string(data), string(dry))
}

func TestConfigSetAndGetMaskPluginSecret(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	setupTestConfig(t)
	const secret = "super-secret-value"

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "plugins.aws.secret_key", secret})
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "Configuration updated: plugins.aws.secret_key = "+config.RedactedValue)
	assert.NotContains(t, string(result.Stdout), secret)
	assert.NotContains(t, string(result.Stderr), secret)

	data, err := os.ReadFile(filepath.Join(os.Getenv("FINFOCUS_HOME"), "config.hujson"))
	require.NoError(t, err)
	assert.Contains(t, string(data), secret)
	assert.NotContains(t, string(data), config.RedactedValue)

	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "get", "plugins.aws.secret_key"})
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Equal(t, config.RedactedValue+"\n", string(result.Stdout))
	assert.NotContains(t, string(result.Stdout), secret)

	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "list"})
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), config.RedactedValue)
	assert.NotContains(t, string(result.Stdout), secret)
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

	// Test getting simple value
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "get", "output.default_format"})
	require.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "json\n", string(result.Stdout))

	// Test getting plugin value
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
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

	// Test invalid key
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "get", "invalid.key"})
	assert.NotEqual(t, 0, result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "unknown configuration section")

	// Test non-existent plugin
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
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

	// Initialize config first
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "init"})
	require.Equal(t, 0, result.ExitCode)

	// Set some config values
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "output.default_format", "json"})
	require.Equal(t, 0, result.ExitCode)

	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "plugins.aws.region", "us-west-2"})
	require.Equal(t, 0, result.ExitCode)

	// Test the default output style on a terminal.
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "list"}, ax.WithStdoutIsTTY(true))
	require.Equal(t, 0, result.ExitCode)

	yamlOutput := string(result.Stdout)
	assert.Contains(t, yamlOutput, "output:")
	assert.Contains(t, yamlOutput, "default_format: json")
	assert.Contains(t, yamlOutput, "plugins:")
	assert.Contains(t, yamlOutput, "aws:")
	assert.Contains(t, yamlOutput, "region: us-west-2")

	// Test JSON output selected by the global mode flag.
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "list", "--format", "json"})
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

	// Test invalid format
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
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

	// Test valid configuration
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "validate"})
	require.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration is valid")

	// Test with verbose flag
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "validate", "--verbose"})
	require.Equal(t, 0, result.ExitCode)

	verboseOutput := string(result.Stdout)
	assert.Contains(t, verboseOutput, "Configuration is valid")
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

	// Test invalid configuration
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "validate"})
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stdout), "invalid output format")
	assert.Contains(t, string(result.Stderr), "configuration validation failed")
}

func TestConfigCommandsIntegration(t *testing.T) {
	// Set log level to error to avoid cluttering test output with debug logs
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	cleanup := setupTestConfig(t)
	defer cleanup()

	// Test full workflow: init -> set -> get -> validate -> list

	// 1. Initialize config
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "init"})
	require.Equal(t, 0, result.ExitCode)

	// 2. Set some values
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "output.default_format", "json"})
	require.Equal(t, 0, result.ExitCode)

	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "plugins.aws.region", "eu-west-1"})
	require.Equal(t, 0, result.ExitCode)

	// 3. Get values to verify
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "get", "output.default_format"})
	require.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "json\n", string(result.Stdout))

	// 4. Validate configuration
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "validate"})
	require.Equal(t, 0, result.ExitCode)
	output := string(result.Stdout)
	assert.Contains(t, output, "Configuration is valid")

	// 5. List all configuration using the default terminal style.
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "list"}, ax.WithStdoutIsTTY(true))
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

	// Test set command with wrong number of args
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "only-one-arg"})
	assert.NotEqual(t, 0, result.ExitCode)

	// Test get command with wrong number of args
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "get"})
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

	// Test getting plugin section (returns map)
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "get", "plugins.aws"})
	require.Equal(t, 0, result.ExitCode)

	mapOutput := string(result.Stdout)
	assert.Contains(t, mapOutput, "plugins.aws:")
	assert.Contains(t, mapOutput, "region:")

	// Test getting all plugins (returns map of PluginConfig)
	result = axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "get", "plugins"})
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

	// Test getting integer value
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "get", "output.precision"})
	require.Equal(t, 0, result.ExitCode)

	assert.Contains(t, string(result.Stdout), "4")
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via setupTestConfig)
func TestConfigListCmdDirectRun(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()
	cmd := cli.NewConfigListCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, output.String(), "output:")

	require.NoError(t, cmd.Flags().Parse([]string{"-f", "json"}))
	output.Reset()
	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Contains(t, output.String(), `"output":`)
}
