package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHome sets up an isolated HOME directory for testing to prevent
// tests from reading/writing the real ~/.finfocus directory.
func stubHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // Windows
}

func TestConfig_NewAndDefaults(t *testing.T) {
	stubHome(t)
	cfg := New()

	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.NotEmpty(t, cfg.Logging.File)
	assert.NotNil(t, cfg.Plugins)
	assert.NotEmpty(t, cfg.PluginDir)
	assert.NotEmpty(t, cfg.SpecDir)
}

func TestConfig_NewStrict(t *testing.T) {
	t.Run("success with no config file", func(t *testing.T) {
		stubHome(t)
		cfg, err := NewStrict()

		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "table", cfg.Output.DefaultFormat)
		assert.Equal(t, "info", cfg.Logging.Level)
	})

	t.Run("success with valid config file", func(t *testing.T) {
		stubHome(t)

		// Create a valid config file
		cfg := New()
		cfg.Output.DefaultFormat = "json"
		err := cfg.Save()
		require.NoError(t, err)

		// NewStrict should load the config successfully
		strictCfg, err := NewStrict()
		require.NoError(t, err)
		assert.NotNil(t, strictCfg)
		assert.Equal(t, "json", strictCfg.Output.DefaultFormat)
	})

	t.Run("failure with corrupted config file", func(t *testing.T) {
		stubHome(t)

		// Create a corrupted config file
		configDir := filepath.Join(os.Getenv("HOME"), ".finfocus")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(configDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("invalid: yaml: content: [unclosed"), 0644)
		require.NoError(t, err)

		// NewStrict should fail with corrupted config
		cfg, err := NewStrict()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "corrupted")
	})
}

func TestGetOutputFormat(t *testing.T) {
	// Reset global config for clean state
	ResetGlobalConfigForTest()

	t.Run("returns user choice when provided", func(t *testing.T) {
		result := GetOutputFormat("json")
		assert.Equal(t, "json", result)
	})

	t.Run("returns config default when no user choice", func(t *testing.T) {
		// Set a custom default format
		cfg := GetGlobalConfig()
		cfg.Output.DefaultFormat = "table"

		result := GetOutputFormat("")
		assert.Equal(t, "table", result)
	})
}

func TestSetLogLevel(t *testing.T) {
	// Reset logger to known state
	_ = InitLogger("info", false)

	t.Run("sets valid log level", func(t *testing.T) {
		SetLogLevel("debug")
		logger := GetLogger()
		// Note: We can't easily test the internal level without exposing it
		// But we can verify the function doesn't panic
		assert.NotNil(t, logger)
	})

	t.Run("defaults to info for invalid level", func(t *testing.T) {
		SetLogLevel("invalid-level")
		logger := GetLogger()
		assert.NotNil(t, logger)
	})
}

func TestGetLogger(t *testing.T) {
	logger := GetLogger()
	assert.NotNil(t, logger)
}

func TestConfig_SetGetValues(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Test output values
	err := cfg.Set("output.default_format", "json")
	require.NoError(t, err)

	value, err := cfg.Get("output.default_format")
	require.NoError(t, err)
	assert.Equal(t, "json", value)

	err = cfg.Set("output.precision", "4")
	require.NoError(t, err)

	value, err = cfg.Get("output.precision")
	require.NoError(t, err)
	assert.Equal(t, 4, value)

	// Test plugin values
	err = cfg.Set("plugins.aws.region", "us-west-2")
	require.NoError(t, err)

	value, err = cfg.Get("plugins.aws.region")
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", value)

	// Test logging values
	err = cfg.Set("logging.level", "debug")
	require.NoError(t, err)

	value, err = cfg.Get("logging.level")
	require.NoError(t, err)
	assert.Equal(t, "debug", value)

	// Test plugin_host values
	err = cfg.Set("plugin_host.strict_compatibility", "true")
	require.NoError(t, err)

	value, err = cfg.Get("plugin_host.strict_compatibility")
	require.NoError(t, err)
	assert.Equal(t, true, value)

	// Test setting to false
	err = cfg.Set("plugin_host.strict_compatibility", "false")
	require.NoError(t, err)

	value, err = cfg.Get("plugin_host.strict_compatibility")
	require.NoError(t, err)
	assert.Equal(t, false, value)
}

func TestConfig_SetErrors(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Invalid section
	err := cfg.Set("invalid.key", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration section")

	// Invalid output key
	err = cfg.Set("output.invalid", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown output setting")

	// Invalid precision value
	err = cfg.Set("output.precision", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "precision must be a number")

	// Invalid plugin key format
	err = cfg.Set("plugins.aws", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin key must be in format")

	// Invalid plugin_host key
	err = cfg.Set("plugin_host.invalid", "true")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown plugin_host setting")

	// Invalid plugin_host strict_compatibility value
	err = cfg.Set("plugin_host.strict_compatibility", "not-a-bool")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "strict_compatibility must be a boolean")
}

func TestConfig_GetErrors(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Unknown section
	_, err := cfg.Get("invalid.key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration section")

	// Unknown output key
	_, err = cfg.Get("output.invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown output setting")

	// Unknown plugin
	_, err = cfg.Get("plugins.nonexistent.key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not found")

	// Unknown plugin_host key
	_, err = cfg.Get("plugin_host.invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown plugin_host setting")
}

func TestConfig_Validation(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Valid configuration should pass
	err := cfg.Validate()
	assert.NoError(t, err)

	// Invalid output format
	cfg.Output.DefaultFormat = "invalid"
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")

	// Reset and test invalid precision
	cfg.Output.DefaultFormat = "table"
	cfg.Output.Precision = -1
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid precision")

	// Reset and test invalid log level
	cfg.Output.Precision = 2
	cfg.Logging.Level = "invalid"
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestConfig_SaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "finfocus-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		Output: OutputConfig{
			DefaultFormat: "json",
			Precision:     4,
		},
		Plugins: map[string]PluginConfig{
			"aws": {Config: map[string]interface{}{"region": "us-west-2"}},
		},
		Logging: LoggingConfig{
			Level: "debug",
			File:  filepath.Join(t.TempDir(), "test.log"),
		},
		configPath: filepath.Join(tmpDir, "config.yaml"),
	}

	// Save configuration
	err = cfg.Save()
	require.NoError(t, err)

	// Load configuration
	cfg2 := &Config{
		configPath: cfg.configPath,
	}
	err = cfg2.Load()
	require.NoError(t, err)

	assert.Equal(t, cfg.Output.DefaultFormat, cfg2.Output.DefaultFormat)
	assert.Equal(t, cfg.Output.Precision, cfg2.Output.Precision)
	assert.Equal(t, cfg.Logging.Level, cfg2.Logging.Level)
	assert.Equal(t, cfg.Logging.File, cfg2.Logging.File)
	assert.Equal(t, len(cfg.Plugins), len(cfg2.Plugins))

	awsConfig, exists := cfg2.Plugins["aws"]
	assert.True(t, exists)
	assert.Equal(t, "us-west-2", awsConfig.Config["region"])
}

func TestConfig_List(t *testing.T) {
	stubHome(t)
	cfg := New()
	cfg.Set("plugins.aws.region", "us-west-2")
	cfg.Set("output.default_format", "json")

	list := cfg.List()

	assert.Contains(t, list, "output")
	assert.Contains(t, list, "plugins")
	assert.Contains(t, list, "logging")

	output := list["output"].(OutputConfig)
	assert.Equal(t, "json", output.DefaultFormat)
}

func TestConfig_PluginMethods(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Test GetPluginConfig for non-existent plugin
	config, err := cfg.GetPluginConfig("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, config)

	// Test SetPluginConfig
	pluginConfig := map[string]interface{}{
		"region":  "us-east-1",
		"profile": "production",
	}
	cfg.SetPluginConfig("aws", pluginConfig)

	// Test GetPluginConfig for existing plugin
	retrievedConfig, err := cfg.GetPluginConfig("aws")
	require.NoError(t, err)
	assert.Equal(t, pluginConfig, retrievedConfig)
}

func TestConfig_EnvironmentOverrides(t *testing.T) {
	// Set environment variables
	customLogFile := filepath.Join(t.TempDir(), "custom.log")
	os.Setenv("FINFOCUS_OUTPUT_FORMAT", "json")
	os.Setenv("FINFOCUS_OUTPUT_PRECISION", "5")
	os.Setenv("FINFOCUS_LOG_LEVEL", "debug")
	os.Setenv("FINFOCUS_LOG_FILE", customLogFile)
	os.Setenv("FINFOCUS_PLUGIN_AWS_REGION", "eu-west-1")
	os.Setenv("FINFOCUS_PLUGIN_AWS_PROFILE", "test")

	defer func() {
		os.Unsetenv("FINFOCUS_OUTPUT_FORMAT")
		os.Unsetenv("FINFOCUS_OUTPUT_PRECISION")
		os.Unsetenv("FINFOCUS_LOG_LEVEL")
		os.Unsetenv("FINFOCUS_LOG_FILE")
		os.Unsetenv("FINFOCUS_PLUGIN_AWS_REGION")
		os.Unsetenv("FINFOCUS_PLUGIN_AWS_PROFILE")
	}()

	stubHome(t)
	cfg := New()

	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 5, cfg.Output.Precision)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, customLogFile, cfg.Logging.File)

	awsConfig, exists := cfg.Plugins["aws"]
	assert.True(t, exists)
	assert.Equal(t, "eu-west-1", awsConfig.Config["region"])
	assert.Equal(t, "test", awsConfig.Config["profile"])
}

func TestConfig_BackwardCompatibility(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Test legacy methods still work
	assert.NotEmpty(t, cfg.PluginDir)
	assert.NotEmpty(t, cfg.SpecDir)

	pluginPath := cfg.PluginPath("test-plugin", "1.0.0")
	assert.Contains(t, pluginPath, "test-plugin")
	assert.Contains(t, pluginPath, "1.0.0")
}

// T044: Unit test for configuration precedence (CLI > env > config).
func TestConfig_Precedence_EnvOverridesConfigFile(t *testing.T) {
	// Create a temporary config file with specific values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".finfocus", "config.yaml")
	err := os.MkdirAll(filepath.Dir(configPath), 0700)
	require.NoError(t, err)

	configContent := `
logging:
  level: error
  format: json
`
	err = os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Set HOME to use our test config
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Set environment variables that should override config file
	t.Setenv("FINFOCUS_LOG_LEVEL", "debug")
	t.Setenv("FINFOCUS_LOG_FORMAT", "text")

	cfg := New()

	// Environment should override config file values
	assert.Equal(t, "debug", cfg.Logging.Level, "env should override config file level")
	assert.Equal(t, "text", cfg.Logging.Format, "env should override config file format")
}

// T045: Unit test for FINFOCUS_LOG_LEVEL environment variable.
func TestConfig_FINFOCUS_LOG_LEVEL_EnvVar(t *testing.T) {
	stubHome(t)

	// Test various log levels via environment variable
	tests := []struct {
		envValue      string
		expectedLevel string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
		{"trace", "trace"},
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			t.Setenv("FINFOCUS_LOG_LEVEL", tt.envValue)

			cfg := New()
			assert.Equal(t, tt.expectedLevel, cfg.Logging.Level)
		})
	}
}

// T046: Unit test for FINFOCUS_LOG_FORMAT environment variable.
func TestConfig_FINFOCUS_LOG_FORMAT_EnvVar(t *testing.T) {
	stubHome(t)

	// Test various log formats via environment variable
	// Note: Only "json" and "text" are valid formats per isValidFormat()
	tests := []struct {
		envValue       string
		expectedFormat string
	}{
		{"json", "json"},
		{"text", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			t.Setenv("FINFOCUS_LOG_FORMAT", tt.envValue)

			cfg := New()
			assert.Equal(t, tt.expectedFormat, cfg.Logging.Format)
		})
	}
}

// T047: Unit test for invalid log level fallback to INFO.
func TestConfig_InvalidLogLevel_Validation(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Set an invalid log level
	cfg.Logging.Level = "invalid_level"

	// Validation should fail with invalid level
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

// Test that defaults work correctly when no env vars are set.
func TestConfig_Defaults_NoEnvVars(t *testing.T) {
	// Clear all relevant env vars
	t.Setenv("FINFOCUS_LOG_LEVEL", "")
	t.Setenv("FINFOCUS_LOG_FORMAT", "")
	t.Setenv("FINFOCUS_LOG_FILE", "")
	t.Setenv("FINFOCUS_OUTPUT_FORMAT", "")
	t.Setenv("FINFOCUS_OUTPUT_PRECISION", "")

	stubHome(t)
	cfg := New()

	// Should have default values
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, "table", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Precision)
}

// T011: Test PULUMI_HOME precedence for config path resolution.
// When PULUMI_HOME is set, config should be loaded from $PULUMI_HOME/finfocus/.
func TestConfig_PULUMI_HOME_Precedence(t *testing.T) {
	t.Run("PULUMI_HOME set - uses finfocus subdir", func(t *testing.T) {
		// Create a fake PULUMI_HOME with a config file
		pulumiHome := t.TempDir()
		finfocusDir := filepath.Join(pulumiHome, "finfocus")
		err := os.MkdirAll(finfocusDir, 0700)
		require.NoError(t, err)

		// Create a config file with a distinctive value
		configContent := `
output:
  default_format: ndjson
  precision: 7
`
		configPath := filepath.Join(finfocusDir, "config.yaml")
		err = os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set up environment
		t.Setenv("PULUMI_HOME", pulumiHome)
		stubHome(t) // Set HOME to something else to ensure PULUMI_HOME takes precedence

		cfg := New()

		// Config should come from PULUMI_HOME/finfocus/
		assert.Equal(t, "ndjson", cfg.Output.DefaultFormat, "should use config from PULUMI_HOME")
		assert.Equal(t, 7, cfg.Output.Precision, "should use precision from PULUMI_HOME config")
	})

	t.Run("PULUMI_HOME not set - uses default HOME path", func(t *testing.T) {
		// Create a fake HOME with a config file
		homeDir := t.TempDir()
		finfocusDir := filepath.Join(homeDir, ".finfocus")
		err := os.MkdirAll(finfocusDir, 0700)
		require.NoError(t, err)

		// Create a config file with a distinctive value
		configContent := `
output:
  default_format: json
  precision: 3
`
		configPath := filepath.Join(finfocusDir, "config.yaml")
		err = os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		// Ensure PULUMI_HOME is NOT set
		t.Setenv("PULUMI_HOME", "")
		t.Setenv("HOME", homeDir)
		t.Setenv("USERPROFILE", homeDir) // Windows

		cfg := New()

		// Config should come from HOME/.finfocus/
		assert.Equal(t, "json", cfg.Output.DefaultFormat, "should use config from HOME/.finfocus")
		assert.Equal(t, 3, cfg.Output.Precision, "should use precision from HOME config")
	})

	t.Run("PULUMI_HOME set but no config file - uses defaults", func(t *testing.T) {
		// Create a fake PULUMI_HOME without config file
		pulumiHome := t.TempDir()

		t.Setenv("PULUMI_HOME", pulumiHome)
		stubHome(t)

		cfg := New()

		// Should use defaults
		assert.Equal(t, "table", cfg.Output.DefaultFormat, "should use default format")
		assert.Equal(t, 2, cfg.Output.Precision, "should use default precision")
	})

	t.Run("PULUMI_HOME takes precedence over HOME", func(t *testing.T) {
		// Create both PULUMI_HOME and HOME with different config files
		pulumiHome := t.TempDir()
		homeDir := t.TempDir()

		// Create PULUMI_HOME config
		finfocusDirPulumi := filepath.Join(pulumiHome, "finfocus")
		err := os.MkdirAll(finfocusDirPulumi, 0700)
		require.NoError(t, err)
		pulumiConfig := `
output:
  default_format: ndjson
`
		err = os.WriteFile(filepath.Join(finfocusDirPulumi, "config.yaml"), []byte(pulumiConfig), 0600)
		require.NoError(t, err)

		// Create HOME config with different value
		finfocusDirHome := filepath.Join(homeDir, ".finfocus")
		err = os.MkdirAll(finfocusDirHome, 0700)
		require.NoError(t, err)
		homeConfig := `
output:
  default_format: json
`
		err = os.WriteFile(filepath.Join(finfocusDirHome, "config.yaml"), []byte(homeConfig), 0600)
		require.NoError(t, err)

		// Set both
		t.Setenv("PULUMI_HOME", pulumiHome)
		t.Setenv("HOME", homeDir)
		t.Setenv("USERPROFILE", homeDir)

		cfg := New()

		// PULUMI_HOME should win
		assert.Equal(t, "ndjson", cfg.Output.DefaultFormat, "PULUMI_HOME should take precedence over HOME")
	})
}

// TestConfig_Compatibility tests legacy environment variable support.
func TestConfig_Compatibility(t *testing.T) {
	t.Run("legacy env vars ignored by default", func(t *testing.T) {
		t.Setenv("FINFOCUS_COMPAT", "")
		t.Setenv("PULUMICOST_LOG_LEVEL", "debug")
		stubHome(t)
		cfg := New()
		assert.Equal(t, "info", cfg.Logging.Level)
	})

	t.Run("legacy env vars respected when FINFOCUS_COMPAT=1", func(t *testing.T) {
		t.Logf("pluginsdk.EnvLogLevel: %s", pluginsdk.EnvLogLevel)
		t.Setenv("FINFOCUS_COMPAT", "1")
		t.Setenv("PULUMICOST_LOG_LEVEL", "debug")
		stubHome(t)
		cfg := New()
		assert.Equal(t, "debug", cfg.Logging.Level)
	})

	t.Run("FINFOCUS_ takes precedence over PULUMICOST_ even with compat", func(t *testing.T) {
		t.Setenv("FINFOCUS_COMPAT", "1")
		t.Setenv("FINFOCUS_LOG_LEVEL", "warn")
		t.Setenv("PULUMICOST_LOG_LEVEL", "debug")
		stubHome(t)
		cfg := New()
		assert.Equal(t, "warn", cfg.Logging.Level)
	})
}

// TestResolveConfigDir tests the config directory resolution logic.
func TestResolveConfigDir(t *testing.T) {
	t.Run("with PULUMI_HOME", func(t *testing.T) {
		pulumiHome := "/fake/pulumi/home"
		t.Setenv("PULUMI_HOME", pulumiHome)
		stubHome(t)

		dir := ResolveConfigDir()
		assert.Equal(t, filepath.Join(pulumiHome, "finfocus"), dir)
	})

	t.Run("without PULUMI_HOME", func(t *testing.T) {
		t.Setenv("PULUMI_HOME", "")
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("USERPROFILE", homeDir)

		dir := ResolveConfigDir()
		assert.Equal(t, filepath.Join(homeDir, ".finfocus"), dir)
	})

	t.Run("fallback when HOME is unset", func(t *testing.T) {
		// This tests the fallback when os.UserHomeDir() fails
		t.Setenv("PULUMI_HOME", "")
		t.Setenv("HOME", "")
		t.Setenv("USERPROFILE", "")

		dir := ResolveConfigDir()
		// Should not return empty string or root directory - should use CWD fallback
		assert.NotEqual(t, "", dir)
		assert.NotEqual(t, "/", dir)
		assert.NotEqual(t, "\\", dir)
		assert.Contains(t, dir, ".finfocus")
	})
}
