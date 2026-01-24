package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/config"
)

// T007: Unit test for config bridge function.
func TestLoggingConfig_ToLoggingConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          config.LoggingConfig
		expectedLevel  string
		expectedFormat string
		expectedOutput string
		expectedFile   string
	}{
		{
			name: "defaults to stderr when no file specified",
			input: config.LoggingConfig{
				Level:  "info",
				Format: "json",
				File:   "",
			},
			expectedLevel:  "info",
			expectedFormat: "json",
			expectedOutput: "stderr",
			expectedFile:   "",
		},
		{
			name: "sets output to file when file path provided",
			input: config.LoggingConfig{
				Level:  "debug",
				Format: "console",
				File:   "/var/log/finfocus.log",
			},
			expectedLevel:  "debug",
			expectedFormat: "console",
			expectedOutput: "file",
			expectedFile:   "/var/log/finfocus.log",
		},
		{
			name: "handles all log levels",
			input: config.LoggingConfig{
				Level:  "error",
				Format: "text",
				File:   "",
			},
			expectedLevel:  "error",
			expectedFormat: "text",
			expectedOutput: "stderr",
			expectedFile:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := tc.input.ToLoggingConfig()

			assert.Equal(t, tc.expectedLevel, result.Level, "Level mismatch")
			assert.Equal(t, tc.expectedFormat, result.Format, "Format mismatch")
			assert.Equal(t, tc.expectedOutput, result.Output, "Output mismatch")
			assert.Equal(t, tc.expectedFile, result.File, "File mismatch")
		})
	}
}

// T008: Unit test for config override precedence (file < env < flag).
func TestLoggingConfig_OverridePrecedence(t *testing.T) {
	// Test that environment variables override default config values.
	// Note: The --debug flag override is handled in CLI, not in config package.
	// Note: This test validates env var precedence over defaults; file-based
	// overrides are tested in config_test.go which has access to stubHome().

	// Save and restore environment using pluginsdk constants for consistency.
	origLevel := os.Getenv(pluginsdk.EnvLogLevel)
	origFormat := os.Getenv(pluginsdk.EnvLogFormat)
	t.Cleanup(func() {
		if origLevel != "" {
			_ = os.Setenv(pluginsdk.EnvLogLevel, origLevel)
		} else {
			_ = os.Unsetenv(pluginsdk.EnvLogLevel)
		}
		if origFormat != "" {
			_ = os.Setenv(pluginsdk.EnvLogFormat, origFormat)
		} else {
			_ = os.Unsetenv(pluginsdk.EnvLogFormat)
		}
	})

	t.Run("env vars override default values", func(t *testing.T) {
		// Set environment variables to override defaults using pluginsdk constants.
		_ = os.Setenv(pluginsdk.EnvLogLevel, "debug")
		_ = os.Setenv(pluginsdk.EnvLogFormat, "text")

		// Load config - env vars should take precedence over defaults.
		cfg := config.New()

		assert.Equal(t, "debug", cfg.Logging.Level, "Env var should override default level")
		assert.Equal(t, "text", cfg.Logging.Format, "Env var should override default format")
	})
}

// Test AuditConfig validation.
func TestAuditConfig_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		auditConfig config.AuditConfig
		expectError bool
	}{
		{
			name: "disabled audit requires no validation",
			auditConfig: config.AuditConfig{
				Enabled: false,
				File:    "",
			},
			expectError: false,
		},
		{
			name: "enabled audit with empty file is valid",
			auditConfig: config.AuditConfig{
				Enabled: true,
				File:    "",
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Output: config.OutputConfig{
					DefaultFormat: "table",
					Precision:     2,
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "json",
					Audit:  tc.auditConfig,
				},
			}

			err := cfg.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test GetLoggingConfig retrieves from global config.
func TestGetLoggingConfig(t *testing.T) {
	loggingCfg := config.GetLoggingConfig()

	// Should return valid defaults
	assert.NotEmpty(t, loggingCfg.Level, "Level should have a default value")
}
