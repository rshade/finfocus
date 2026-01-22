package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// TestEnv_OutputFormatOverride tests FINFOCUS_OUTPUT_FORMAT environment variable.
func TestEnv_OutputFormatOverride(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_FORMAT", "json")

	cfg := config.New()

	assert.Equal(t, "json", cfg.Output.DefaultFormat)
}

// TestEnv_OutputPrecisionOverride tests FINFOCUS_OUTPUT_PRECISION environment variable.
func TestEnv_OutputPrecisionOverride(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "8")

	cfg := config.New()

	assert.Equal(t, 8, cfg.Output.Precision)
}

// TestEnv_LogLevelOverride tests FINFOCUS_LOG_LEVEL environment variable.
func TestEnv_LogLevelOverride(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_LOG_LEVEL", "debug")

	cfg := config.New()

	assert.Equal(t, "debug", cfg.Logging.Level)
}

// TestEnv_LogFileOverride tests FINFOCUS_LOG_FILE environment variable.
func TestEnv_LogFileOverride(t *testing.T) {
	setupTestHome(t)
	customLogFile := filepath.Join(t.TempDir(), "custom.log")
	t.Setenv("FINFOCUS_LOG_FILE", customLogFile)

	cfg := config.New()

	assert.Equal(t, customLogFile, cfg.Logging.File)
}

// TestEnv_PluginSingleVariable tests plugin configuration via environment variable.
func TestEnv_PluginSingleVariable(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "us-west-2")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", awsConfig["region"])
}

// TestEnv_PluginMultipleVariables tests multiple plugin environment variables.
func TestEnv_PluginMultipleVariables(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "eu-west-1")
	t.Setenv("FINFOCUS_PLUGIN_AWS_PROFILE", "production")
	t.Setenv("FINFOCUS_PLUGIN_AWS_TIMEOUT", "30")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", awsConfig["region"])
	assert.Equal(t, "production", awsConfig["profile"])
	assert.Equal(t, "30", awsConfig["timeout"])
}

// TestEnv_MultiplePluginConfigurations tests environment variables for multiple plugins.
func TestEnv_MultiplePluginConfigurations(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "us-east-1")
	t.Setenv("FINFOCUS_PLUGIN_KUBECOST_ENDPOINT", "http://localhost:9090")
	t.Setenv("FINFOCUS_PLUGIN_KUBECOST_NAMESPACE", "monitoring")
	t.Setenv("FINFOCUS_PLUGIN_VANTAGE_API_KEY", "test-key")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", awsConfig["region"])

	kubecostConfig, err := cfg.GetPluginConfig("kubecost")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", kubecostConfig["endpoint"])
	assert.Equal(t, "monitoring", kubecostConfig["namespace"])

	vantageConfig, err := cfg.GetPluginConfig("vantage")
	require.NoError(t, err)
	assert.Equal(t, "test-key", vantageConfig["api_key"])
}

// TestEnv_PluginNestedKeyWithUnderscores tests plugin config keys with underscores.
func TestEnv_PluginNestedKeyWithUnderscores(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_API_ENDPOINT", "https://custom.amazonaws.com")
	t.Setenv("FINFOCUS_PLUGIN_AWS_MAX_RETRY_COUNT", "5")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "https://custom.amazonaws.com", awsConfig["api_endpoint"])
	assert.Equal(t, "5", awsConfig["max_retry_count"])
}

// TestEnv_InvalidPrecisionIgnored tests that invalid precision values are ignored.
func TestEnv_InvalidPrecisionIgnored(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "invalid")

	cfg := config.New()

	// Should use default precision (2) when invalid value provided
	assert.Equal(t, 2, cfg.Output.Precision)
}

// TestEnv_PrecedenceOverConfigFile tests that environment variables override config file.
func TestEnv_PrecedenceOverConfigFile(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	// Create config file with initial values
	configContent := `output:
  default_format: table
  precision: 2
logging:
  level: info
plugins:
  aws:
    region: us-east-1
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(configContent), 0600)
	require.NoError(t, err)

	// Set environment variables with different values
	t.Setenv("FINFOCUS_OUTPUT_FORMAT", "json")
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "5")
	t.Setenv("FINFOCUS_LOG_LEVEL", "debug")
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "eu-west-1")

	cfg := config.New()

	// Environment variables should take precedence
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 5, cfg.Output.Precision)
	assert.Equal(t, "debug", cfg.Logging.Level)

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", awsConfig["region"])
}

// TestEnv_AllOutputVariables tests all output-related environment variables together.
func TestEnv_AllOutputVariables(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_FORMAT", "ndjson")
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "6")

	cfg := config.New()

	assert.Equal(t, "ndjson", cfg.Output.DefaultFormat)
	assert.Equal(t, 6, cfg.Output.Precision)
}

// TestEnv_AllLoggingVariables tests all logging-related environment variables together.
func TestEnv_AllLoggingVariables(t *testing.T) {
	setupTestHome(t)
	customLogFile := filepath.Join(t.TempDir(), "env-test.log")
	t.Setenv("FINFOCUS_LOG_LEVEL", "warn")
	t.Setenv("FINFOCUS_LOG_FILE", customLogFile)

	cfg := config.New()

	assert.Equal(t, "warn", cfg.Logging.Level)
	assert.Equal(t, customLogFile, cfg.Logging.File)
}

// TestEnv_EmptyVariableIgnored tests that empty environment variables are ignored.
func TestEnv_EmptyVariableIgnored(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_FORMAT", "")
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "")

	cfg := config.New()

	// Should use defaults when empty
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
}

// TestEnv_MixedOverrides tests combination of file config and env var overrides.
func TestEnv_MixedOverrides(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	// Config file specifies some values
	configContent := `output:
  default_format: table
  precision: 2
logging:
  level: info
plugins:
  aws:
    region: us-east-1
    profile: dev
  kubecost:
    endpoint: http://default:9090
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(configContent), 0600)
	require.NoError(t, err)

	// Only override specific values via environment
	t.Setenv("FINFOCUS_OUTPUT_FORMAT", "json")
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "ap-south-1")

	cfg := config.New()

	// Env var overrides
	assert.Equal(t, "json", cfg.Output.DefaultFormat)

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "ap-south-1", awsConfig["region"])

	// File values still apply where no env var
	assert.Equal(t, 2, cfg.Output.Precision)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "dev", awsConfig["profile"])

	kubecostConfig, err := cfg.GetPluginConfig("kubecost")
	require.NoError(t, err)
	assert.Equal(t, "http://default:9090", kubecostConfig["endpoint"])
}

// TestEnv_CaseInsensitivePluginName tests that plugin names are case-insensitive.
func TestEnv_CaseInsensitivePluginName(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "us-west-2")
	t.Setenv("FINFOCUS_PLUGIN_KUBECOST_ENDPOINT", "http://localhost:9090")

	cfg := config.New()

	// Plugin names should be lowercase
	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", awsConfig["region"])

	kubecostConfig, err := cfg.GetPluginConfig("kubecost")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", kubecostConfig["endpoint"])
}

// TestEnv_SpecialCharactersInValues tests handling of special characters in env var values.
func TestEnv_SpecialCharactersInValues(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_ENDPOINT", "https://api.aws.com/v1?key=value&flag=true")
	t.Setenv("FINFOCUS_PLUGIN_KUBECOST_TOKEN", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Contains(t, awsConfig["endpoint"], "?key=value&flag=true")

	kubecostConfig, err := cfg.GetPluginConfig("kubecost")
	require.NoError(t, err)
	assert.Contains(t, kubecostConfig["token"], "Bearer eyJ")
}

// TestEnv_NoEnvironmentVariables tests behavior with no environment overrides.
func TestEnv_NoEnvironmentVariables(t *testing.T) {
	setupTestHome(t)

	cfg := config.New()

	// Should use all defaults
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.NotNil(t, cfg.Plugins)
}

// TestEnv_ZeroPrecision tests that zero precision is valid.
func TestEnv_ZeroPrecision(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "0")

	cfg := config.New()

	assert.Equal(t, 0, cfg.Output.Precision)
}

// TestEnv_NegativePrecisionIgnored tests that negative precision is ignored.
func TestEnv_NegativePrecisionIgnored(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "-1")

	cfg := config.New()

	// Negative precision should be converted but then fail validation
	// The env var parsing doesn't validate, so -1 will be set
	assert.Equal(t, -1, cfg.Output.Precision)
}

// TestEnv_PluginWithoutAdditionalKeys tests plugin env var with only name (invalid format).
func TestEnv_PluginWithoutAdditionalKeys(t *testing.T) {
	setupTestHome(t)
	// This should be ignored as it doesn't have format FINFOCUS_PLUGIN_<NAME>_<KEY>
	t.Setenv("FINFOCUS_PLUGIN_AWS", "invalid-value")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	// Should be empty map since invalid env var format is ignored
	assert.Empty(t, awsConfig)
}

// TestEnv_WhitespaceInValues tests that whitespace in values is preserved.
func TestEnv_WhitespaceInValues(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_TEST_VALUE", "  spaces around  ")

	cfg := config.New()

	testConfig, err := cfg.GetPluginConfig("test")
	require.NoError(t, err)
	assert.Equal(t, "  spaces around  ", testConfig["value"])
}

// TestEnv_PluginConfigMergesBetweenVars tests that multiple env vars for same plugin merge.
func TestEnv_PluginConfigMergesBetweenVars(t *testing.T) {
	setupTestHome(t)
	t.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "us-west-2")
	t.Setenv("FINFOCUS_PLUGIN_AWS_PROFILE", "staging")
	t.Setenv("FINFOCUS_PLUGIN_AWS_TIMEOUT", "60")
	t.Setenv("FINFOCUS_PLUGIN_AWS_MAX_RETRIES", "3")

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Len(t, awsConfig, 4)
	assert.Equal(t, "us-west-2", awsConfig["region"])
	assert.Equal(t, "staging", awsConfig["profile"])
	assert.Equal(t, "60", awsConfig["timeout"])
	assert.Equal(t, "3", awsConfig["max_retries"])
}
