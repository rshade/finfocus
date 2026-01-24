package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// TestNew_DefaultConfiguration tests creating a new config with default values.
func TestNew_DefaultConfiguration(t *testing.T) {
	setupTestHome(t)

	cfg := config.New()

	require.NotNil(t, cfg)
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.NotNil(t, cfg.Plugins)
	assert.NotEmpty(t, cfg.PluginDir)
	assert.NotEmpty(t, cfg.SpecDir)
}

// TestNew_CreatesConfigInHomeDirectory tests that config path is in ~/.finfocus/.
func TestNew_CreatesConfigInHomeDirectory(t *testing.T) {
	homeDir := setupTestHome(t)

	cfg := config.New()

	finfocusDir := filepath.Join(homeDir, ".finfocus")
	assert.Contains(t, cfg.PluginDir, finfocusDir)
	assert.Contains(t, cfg.SpecDir, finfocusDir)
}

// TestLoad_ValidConfigFile tests loading a valid YAML configuration file.
func TestLoad_ValidConfigFile(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	// Create config file
	configContent := `output:
  default_format: json
  precision: 4
logging:
  level: debug
  format: json
  file: /tmp/test.log
plugins:
  aws:
    region: us-west-2
    profile: production
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg := config.New()

	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 4, cfg.Output.Precision)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", awsConfig["region"])
	assert.Equal(t, "production", awsConfig["profile"])
}

// TestLoad_NonExistentFile tests that loading a non-existent file uses defaults.
func TestLoad_NonExistentFile(t *testing.T) {
	setupTestHome(t)

	cfg := config.New()

	// Should use defaults when file doesn't exist
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
	assert.Equal(t, "info", cfg.Logging.Level)
}

// TestLoad_CorruptedYAML tests error handling for malformed YAML.
func TestLoad_CorruptedYAML(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	// Create corrupted YAML file
	corruptedContent := `output:
  default_format: json
  precision: invalid_yaml [
logging:
  level: debug
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(corruptedContent), 0600)
	require.NoError(t, err)

	// Should use defaults for corrupted fields, but keep valid fields
	cfg := config.New()

	// Valid fields from YAML are used, corrupted fields fall back to defaults
	assert.Equal(t, "json", cfg.Output.DefaultFormat) // Valid field from YAML
	assert.Equal(t, 2, cfg.Output.Precision)          // Invalid field, uses default
}

// TestLoad_EmptyConfigFile tests loading an empty configuration file.
func TestLoad_EmptyConfigFile(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(""), 0600)
	require.NoError(t, err)

	cfg := config.New()

	// Should use defaults for empty file
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
}

// TestLoad_PartialConfiguration tests loading a partial config with some defaults.
func TestLoad_PartialConfiguration(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	// Only specify output settings, omit logging and plugins
	partialContent := `output:
  default_format: ndjson
  precision: 6
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(partialContent), 0600)
	require.NoError(t, err)

	cfg := config.New()

	// Specified values should be loaded
	assert.Equal(t, "ndjson", cfg.Output.DefaultFormat)
	assert.Equal(t, 6, cfg.Output.Precision)

	// Unspecified values should use defaults
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.NotNil(t, cfg.Plugins)
}

// TestSave_CreatesDirectory tests that Save creates the config directory if it doesn't exist.
func TestSave_CreatesDirectory(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	cfg := config.New()
	cfg.Output.DefaultFormat = "json"

	err := cfg.Save()

	require.NoError(t, err)
	_, err = os.Stat(finfocusDir)
	assert.NoError(t, err, "Directory should be created")
}

// TestSave_CreatesFile tests that Save creates a config file with correct permissions.
func TestSave_CreatesFile(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	configPath := filepath.Join(finfocusDir, "config.yaml")

	cfg := config.New()
	cfg.Output.DefaultFormat = "json"
	cfg.Output.Precision = 5

	err := cfg.Save()

	require.NoError(t, err)
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

// TestSaveLoad_RoundTrip tests save and load cycle preserves configuration.
func TestSaveLoad_RoundTrip(t *testing.T) {
	setupTestHome(t)

	original := config.New()
	original.Output.DefaultFormat = "json"
	original.Output.Precision = 7
	original.Logging.Level = "debug"
	original.SetPluginConfig("kubecost", map[string]interface{}{
		"endpoint": "http://localhost:9090",
		"token":    "secret-token",
	})

	err := original.Save()
	require.NoError(t, err)

	// Create new config and load
	reloaded := config.New()

	assert.Equal(t, original.Output.DefaultFormat, reloaded.Output.DefaultFormat)
	assert.Equal(t, original.Output.Precision, reloaded.Output.Precision)
	assert.Equal(t, original.Logging.Level, reloaded.Logging.Level)

	kubecostConfig, err := reloaded.GetPluginConfig("kubecost")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", kubecostConfig["endpoint"])
	assert.Equal(t, "secret-token", kubecostConfig["token"])
}

// TestLoad_ComplexPluginConfiguration tests loading nested plugin configurations.
func TestLoad_ComplexPluginConfiguration(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	complexContent := `plugins:
  aws:
    region: us-east-1
    profile: production
    timeout: 30
  kubecost:
    endpoint: http://localhost:9090
    namespace: monitoring
    enabled: true
  vantage:
    api_key: test-key
    base_url: https://api.vantage.sh
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(complexContent), 0600)
	require.NoError(t, err)

	cfg := config.New()

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", awsConfig["region"])
	assert.Equal(t, "production", awsConfig["profile"])

	kubecostConfig, err := cfg.GetPluginConfig("kubecost")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", kubecostConfig["endpoint"])
	assert.Equal(t, "monitoring", kubecostConfig["namespace"])
	assert.Equal(t, true, kubecostConfig["enabled"])
}

// TestLoad_LoggingOutputsConfiguration tests loading multiple logging outputs.
func TestLoad_LoggingOutputsConfiguration(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	logDir := filepath.Join(finfocusDir, "logs")

	loggingContent := `logging:
  level: debug
  format: json
  outputs:
    - type: console
      level: info
      format: text
    - type: file
      path: ` + filepath.Join(logDir, "app.log") + `
      level: debug
      format: json
      max_size_mb: 100
      max_files: 5
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(loggingContent), 0600)
	require.NoError(t, err)

	cfg := config.New()

	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	require.Len(t, cfg.Logging.Outputs, 2)

	consoleOutput := cfg.Logging.Outputs[0]
	assert.Equal(t, "console", consoleOutput.Type)
	assert.Equal(t, "info", consoleOutput.Level)
	assert.Equal(t, "text", consoleOutput.Format)

	fileOutput := cfg.Logging.Outputs[1]
	assert.Equal(t, "file", fileOutput.Type)
	assert.Contains(t, fileOutput.Path, "app.log")
	assert.Equal(t, "debug", fileOutput.Level)
	assert.Equal(t, "json", fileOutput.Format)
	assert.Equal(t, 100, fileOutput.MaxSizeMB)
	assert.Equal(t, 5, fileOutput.MaxFiles)
}

// TestLoad_MixedConfiguration tests loading a complete configuration with all sections.
func TestLoad_MixedConfiguration(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")
	logDir := filepath.Join(finfocusDir, "logs")

	fullContent := `output:
  default_format: json
  precision: 3
logging:
  level: warn
  format: json
  file: ` + filepath.Join(logDir, "finfocus.log") + `
  outputs:
    - type: console
      level: error
plugins:
  aws:
    region: ap-south-1
  gcp:
    project_id: my-project
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(fullContent), 0600)
	require.NoError(t, err)

	cfg := config.New()

	// Verify all sections loaded
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 3, cfg.Output.Precision)
	assert.Equal(t, "warn", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)

	awsConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, "ap-south-1", awsConfig["region"])

	gcpConfig, err := cfg.GetPluginConfig("gcp")
	require.NoError(t, err)
	assert.Equal(t, "my-project", gcpConfig["project_id"])
}

// TestLoad_WithUnicodeContent tests loading config with Unicode characters.
func TestLoad_WithUnicodeContent(t *testing.T) {
	homeDir := setupTestHome(t)
	finfocusDir := filepath.Join(homeDir, ".finfocus")

	unicodeContent := `plugins:
  test:
    description: "日本語 テスト 🚀"
    author: "Café ☕"
`
	err := os.MkdirAll(finfocusDir, 0700)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(finfocusDir, "config.yaml"), []byte(unicodeContent), 0600)
	require.NoError(t, err)

	cfg := config.New()

	testConfig, err := cfg.GetPluginConfig("test")
	require.NoError(t, err)
	assert.Contains(t, testConfig["description"], "日本語")
	assert.Contains(t, testConfig["description"], "🚀")
	assert.Contains(t, testConfig["author"], "☕")
}

// Helper functions

// setupTestHome creates an isolated HOME directory for testing.
func setupTestHome(t *testing.T) string {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // Windows compatibility

	return homeDir
}
