// Package config_test provides integration tests for configuration loading across components.
package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/test/integration/helpers"
)

// TestConfigLoading_DefaultValues tests that default configuration values are loaded correctly.
func TestConfigLoading_DefaultValues(t *testing.T) {
	cfg := config.New()

	// Verify default values
	assert.Equal(t, "table", cfg.Output.DefaultFormat, "Default output format should be table")
	assert.NotEmpty(t, cfg.PluginDir, "Plugin directory should have a default")
	assert.NotEmpty(t, cfg.SpecDir, "Spec directory should have a default")
}

// TestConfigLoading_EnvironmentVariables tests configuration from environment variables.
func TestConfigLoading_EnvironmentVariables(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Set environment variables
	env := map[string]string{
		"FINFOCUS_OUTPUT_FORMAT": "json",
	}

	h.WithEnv(env, func() {
		cfg := config.New()

		// Verify environment variables are loaded
		assert.Equal(t, "json", cfg.Output.DefaultFormat, "Should load output format from env")
	})
}

// TestConfigLoading_ConfigFile tests loading configuration from file.
func TestConfigLoading_ConfigFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temporary home directory
	tempHome := h.CreateTempDir()
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	err := os.MkdirAll(finfocusDir, 0755)
	require.NoError(t, err, "Failed to create finfocus directory")

	// Create config file
	configFile := filepath.Join(finfocusDir, "config.yaml")
	configContent := `output:
  default_format: ndjson
  precision: 3
logging:
  level: debug
`
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err, "Failed to write config file")

	// Set HOME to temp directory and load config
	env := map[string]string{
		"HOME": tempHome,
	}

	h.WithEnv(env, func() {
		cfg := config.New()

		// Verify values from file
		assert.Equal(t, "ndjson", cfg.Output.DefaultFormat, "Should load output format from file")
		assert.Equal(t, 3, cfg.Output.Precision, "Should load precision from file")
		assert.Equal(t, "debug", cfg.Logging.Level, "Should load log level from file")
	})
}

// TestConfigLoading_PrecedenceOrder tests configuration precedence (flags > env > file > defaults).
func TestConfigLoading_PrecedenceOrder(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temporary home directory
	tempHome := h.CreateTempDir()
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	err := os.MkdirAll(finfocusDir, 0755)
	require.NoError(t, err)

	// Create config file with json format
	configFile := filepath.Join(finfocusDir, "config.yaml")
	configContent := `output:
  default_format: json
`
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable (should override file)
	env := map[string]string{
		"HOME":                   tempHome,
		"FINFOCUS_OUTPUT_FORMAT": "ndjson",
	}

	h.WithEnv(env, func() {
		cfg := config.New()

		// Environment should override file
		assert.Equal(t, "ndjson", cfg.Output.DefaultFormat, "Env var should override config file")
	})
}

// TestConfigLoading_InvalidConfigFile tests error handling for invalid config file.
func TestConfigLoading_InvalidConfigFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temporary home directory
	tempHome := h.CreateTempDir()
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	err := os.MkdirAll(finfocusDir, 0755)
	require.NoError(t, err)

	// Create invalid config file
	invalidConfig := `invalid: [yaml content`
	configFile := filepath.Join(finfocusDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(invalidConfig), 0644)
	require.NoError(t, err)

	// Set HOME to temp directory
	env := map[string]string{
		"HOME": tempHome,
	}

	h.WithEnv(env, func() {
		// config.New() should handle invalid config gracefully by using defaults
		cfg := config.New()

		// Should still have default values (config errors are handled gracefully)
		assert.Equal(t, "table", cfg.Output.DefaultFormat, "Should use defaults on invalid config")
	})
}

// TestConfigLoading_MissingConfigFile tests handling of missing config file.
func TestConfigLoading_MissingConfigFile(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temporary home directory without config file
	tempHome := h.CreateTempDir()

	// Set HOME to temp directory (no config file exists)
	env := map[string]string{
		"HOME": tempHome,
	}

	h.WithEnv(env, func() {
		// config.New() should handle missing config gracefully by using defaults
		cfg := config.New()

		// Should have default values
		assert.Equal(t, "table", cfg.Output.DefaultFormat, "Should use defaults when config file missing")
		assert.NotEmpty(t, cfg.PluginDir, "Plugin directory should have a default")
	})
}

// TestConfigLoading_CLIFlags tests that CLI flags override all other configuration sources.
func TestConfigLoading_CLIFlags(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Set environment variable
	env := map[string]string{
		"FINFOCUS_OUTPUT_FORMAT": "table",
	}

	h.WithEnv(env, func() {
		planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

		// Execute with --output flag (should override env)
		output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
		require.NoError(t, err)

		// Should be valid JSON (not table format)
		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		assert.NoError(t, err, "Flag should override environment variable to produce JSON")
	})
}

// TestConfigLoading_PluginDirectory tests plugin directory configuration.
func TestConfigLoading_PluginDirectory(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temporary home directory
	tempHome := h.CreateTempDir()

	// Set HOME to temp directory
	env := map[string]string{
		"HOME": tempHome,
	}

	h.WithEnv(env, func() {
		cfg := config.New()

		// Verify plugin directory is set correctly (should be ~/.finfocus/plugins)
		expectedPluginDir := filepath.Join(tempHome, ".finfocus", "plugins")
		assert.Equal(t, expectedPluginDir, cfg.PluginDir, "Should use default plugin directory")
	})
}

// TestConfigLoading_SpecDirectory tests spec directory configuration.
func TestConfigLoading_SpecDirectory(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	// Create temporary home directory
	tempHome := h.CreateTempDir()

	// Set HOME to temp directory
	env := map[string]string{
		"HOME": tempHome,
	}

	h.WithEnv(env, func() {
		cfg := config.New()

		// Verify spec directory is set correctly (should be ~/.finfocus/specs)
		expectedSpecDir := filepath.Join(tempHome, ".finfocus", "specs")
		assert.Equal(t, expectedSpecDir, cfg.SpecDir, "Should use default spec directory")
	})
}

// TestConfigLoading_Integration_FullWorkflow tests complete configuration flow in CLI.
func TestConfigLoading_Integration_FullWorkflow(t *testing.T) {
	h := helpers.NewCLIHelper(t)

	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	// Execute command with explicit output format
	output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
	require.NoError(t, err, "Command should execute successfully")

	// Verify JSON output - renderJSON wraps results in {"finfocus": ...}
	var wrapper map[string]interface{}
	err = json.Unmarshal([]byte(output), &wrapper)
	assert.NoError(t, err, "Should produce JSON output")

	// Extract the finfocus wrapper
	result, ok := wrapper["finfocus"].(map[string]interface{})
	require.True(t, ok, "Should have finfocus wrapper")

	// Verify basic structure
	assert.Contains(t, result, "summary", "JSON output should have summary")
	assert.Contains(t, result, "resources", "JSON output should have resources")
}
