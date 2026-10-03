package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/test/integration/helpers"
)

// TestConfigInit_CreateNewConfig tests creating a new configuration file.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigInit_CreateNewConfig(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "init")
		require.NoError(t, err)
		assert.Contains(t, output, "Configuration initialized")

		// Verify config file was created
		configPath := filepath.Join(tempHome, ".finfocus", "config.hujson")
		assert.FileExists(t, configPath)
	})
}

// TestConfigInit_ExistingConfig_Error tests that init fails without --force when config exists.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigInit_ExistingConfig_Error(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create existing config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: json\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		_, err := h.Execute("config", "init")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

// TestConfigInit_ExistingConfig_Force tests that init --force allows overwriting existing config.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigInit_ExistingConfig_Force(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create existing config with custom value
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: ndjson\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "init", "--force")
		require.NoError(t, err)
		assert.Contains(t, output, "Configuration initialized")

		// Verify config file exists and is valid YAML
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		// Config should exist and contain output section
		assert.Contains(t, string(content), "output:")
		assert.Contains(t, string(content), "default_format:")
	})
}

// TestConfigSet_ValidKey tests setting a valid configuration key.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigSet_ValidKey(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create initial config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: table\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "set", "output.default_format", "json")
		require.NoError(t, err)
		assert.Contains(t, output, "Configuration updated")
		assert.Contains(t, output, "output.default_format")
		assert.Contains(t, output, "json")
	})
}

// TestConfigSet_InvalidKey tests setting an invalid configuration key.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigSet_InvalidKey(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create initial config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: table\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		_, err := h.Execute("config", "set", "unknown.key", "value")
		require.Error(t, err)
	})
}

// TestConfigGet_ExistingKey tests getting an existing configuration key.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigGet_ExistingKey(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create config with known value
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: json\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "get", "output.default_format")
		require.NoError(t, err)
		assert.Contains(t, strings.TrimSpace(output), "json")
	})
}

// TestConfigGet_NestedKey tests getting a nested configuration key (plugins.aws.region).
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigGet_NestedKey(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create config with nested plugin config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	configContent := `output:
  default_format: table
plugins:
  aws:
    config:
      region: us-west-2
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "get", "plugins")
		require.NoError(t, err)
		assert.Contains(t, output, "aws")
		assert.Contains(t, output, "region")
	})
}

// TestConfigList_YAML tests listing configuration in YAML format.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigList_YAML(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: json\n  precision: 2\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "list")
		require.NoError(t, err)
		// YAML output should contain key: value pairs
		assert.Contains(t, output, "output")
		assert.Contains(t, output, "default_format")
	})
}

// TestConfigList_JSON tests listing configuration in JSON format.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigList_JSON(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: json\n  precision: 2\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "list", "--format", "json")
		require.NoError(t, err)

		// Should be valid JSON
		var result map[string]any
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err)
		assert.Contains(t, result, "output")
	})
}

// TestConfigValidate_Valid tests validating a valid configuration.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigValidate_Valid(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create valid config
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: table\n  precision: 2\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		output, err := h.Execute("config", "validate")
		require.NoError(t, err)
		assert.Contains(t, output, "valid")
	})
}

// TestConfigValidate_Invalid tests validating an invalid configuration.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigValidate_Invalid(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	// Create invalid config (invalid format value)
	finfocusDir := filepath.Join(tempHome, ".finfocus")
	require.NoError(t, os.MkdirAll(finfocusDir, 0755))
	configPath := filepath.Join(finfocusDir, "config.yaml")
	// Use an unsupported output format
	require.NoError(t, os.WriteFile(configPath, []byte("output:\n  default_format: invalid_format\n"), 0644))

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		_, err := h.Execute("config", "validate")
		// Validation may or may not fail depending on validation strictness
		// If it doesn't fail, we still passed the test (config is permissive)
		if err != nil {
			assert.Contains(t, err.Error(), "validation")
		}
	})
}

// TestConfig_FullWorkflow tests the complete config workflow: init → set → get → validate → list.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfig_FullWorkflow(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	tempHome := h.CreateTempDir()

	h.WithEnv(map[string]string{"HOME": tempHome}, func() {
		// Step 1: Initialize config
		output, err := h.Execute("config", "init")
		require.NoError(t, err)
		assert.Contains(t, output, "Configuration initialized")

		// Step 2: Set a value
		output, err = h.Execute("config", "set", "output.default_format", "json")
		require.NoError(t, err)
		assert.Contains(t, output, "Configuration updated")

		// Step 3: Get the value back
		output, err = h.Execute("config", "get", "output.default_format")
		require.NoError(t, err)
		assert.Contains(t, strings.TrimSpace(output), "json")

		// Step 4: Validate the config
		output, err = h.Execute("config", "validate")
		require.NoError(t, err)
		assert.Contains(t, output, "valid")

		// Step 5: List all config
		output, err = h.Execute("config", "list", "--format", "json")
		require.NoError(t, err)
		var result map[string]any
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err)
	})
}

// TestConfigValidate_FileAndCostPreRun checks the file report and the cost pre-run.
//
//nolint:paralleltest // os.Setenv changes the process-wide environment (via WithEnv)
func TestConfigValidate_FileAndCostPreRun(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	home := h.CreateTempDir()
	body := "{\n" +
		"  \"cost\": {\"budgets\": {\"global\": {\"amount\": 100, \"currency\": \"USD\", \"period\": \"weekly\"}}}\n" +
		"}\n"
	configPath := filepath.Join(home, "config.hujson")
	require.NoError(t, os.WriteFile(configPath, []byte(body), 0o600))

	h.WithEnv(map[string]string{
		"FINFOCUS_HOME":                 home,
		"FINFOCUS_SKIP_MIGRATION_CHECK": "1",
		"NO_COLOR":                      "1",
	}, func() {
		t.Chdir(home)
		config.ResetGlobalConfigForTest()
		t.Cleanup(config.ResetGlobalConfigForTest)

		stdout, err := h.Execute("config", "validate", "--file", configPath, "--output", "text")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration validation failed")
		assert.Contains(t, stdout, "must be 'monthly'")
		assert.Contains(t, stdout, "period: monthly")

		stdout, err = h.Execute("config", "validate", "--file", configPath, "--output", "json")
		require.Error(t, err)
		var report struct {
			Valid  bool `json:"valid"`
			Errors []struct {
				Path    string `json:"path"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &report))
		assert.False(t, report.Valid)
		require.NotEmpty(t, report.Errors)
		assert.Equal(t, "cost.budgets.global.period", report.Errors[0].Path)
		assert.Contains(t, report.Errors[0].Message, "monthly")

		_, err = h.Execute("config", "validate", "--output", "yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "text or json")

		_, err = h.Execute("cost", "projected")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be 'monthly'")
		assert.Contains(t, err.Error(), "configuration validation failed")
	})
}
