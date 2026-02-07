package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidation_OutputFormat tests validation of output format values.
func TestValidation_OutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		shouldError bool
		errContains string
	}{
		{"valid table", "table", false, ""},
		{"valid json", "json", false, ""},
		{"valid ndjson", "ndjson", false, ""},
		{"invalid format", "xml", true, "invalid output format"},
		{"empty format", "", true, "invalid output format"},
		{"uppercase format", "TABLE", true, "invalid output format"},
		{"mixed case format", "Json", true, "invalid output format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubHome(t)
			cfg := New()
			cfg.Output.DefaultFormat = tt.format

			err := cfg.Validate()
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidation_Precision tests validation of precision values.
func TestValidation_Precision(t *testing.T) {
	tests := []struct {
		name        string
		precision   int
		shouldError bool
		errContains string
	}{
		{"valid 0", 0, false, ""},
		{"valid 2 (default)", 2, false, ""},
		{"valid 10 (max)", 10, false, ""},
		{"invalid negative", -1, true, "invalid precision"},
		{"invalid too high", 11, true, "invalid precision"},
		{"invalid very negative", -100, true, "invalid precision"},
		{"invalid very high", 100, true, "invalid precision"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubHome(t)
			cfg := New()
			cfg.Output.Precision = tt.precision

			err := cfg.Validate()
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidation_LogLevel tests validation of log level values.
func TestValidation_LogLevel(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		shouldError bool
		errContains string
	}{
		{"valid debug", "debug", false, ""},
		{"valid info", "info", false, ""},
		{"valid warn", "warn", false, ""},
		{"valid error", "error", false, ""},
		{"invalid level", "invalid", true, "invalid log level"},
		{"uppercase level", "DEBUG", true, "invalid log level"},
		{"invalid trace", "trace", true, "invalid log level"},
		{"invalid fatal", "fatal", true, "invalid log level"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubHome(t)
			cfg := New()
			cfg.Logging.Level = tt.level

			err := cfg.Validate()
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidation_LogFormat tests validation of log format values.
func TestValidation_LogFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		shouldError bool
		errContains string
	}{
		{"valid json", "json", false, ""},
		{"valid text", "text", false, ""},
		{"empty format", "", false, ""}, // Empty is allowed (uses default)
		{"invalid format", "console", true, "invalid log format"},
		{"invalid xml", "xml", true, "invalid log format"},
		{"uppercase format", "JSON", true, "invalid log format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubHome(t)
			cfg := New()
			cfg.Logging.Format = tt.format

			err := cfg.Validate()
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidation_LogFilePath tests validation of log file paths.
func TestValidation_LogFilePath(t *testing.T) {
	t.Run("relative path is invalid", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.File = "relative/path/to/log.log"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log file path must be absolute")
	})

	t.Run("absolute path is valid", func(t *testing.T) {
		stubHome(t)
		tmpDir := t.TempDir()
		cfg := New()
		cfg.Logging.File = filepath.Join(tmpDir, "logs", "test.log")

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("empty path is valid (uses default)", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.File = ""

		err := cfg.Validate()
		require.NoError(t, err)
	})
}

// TestValidation_LogOutput tests validation of log output configurations.
func TestValidation_LogOutput(t *testing.T) {
	t.Run("valid console output", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "console", Level: "info", Format: "text"},
		}

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("valid file output", func(t *testing.T) {
		stubHome(t)
		tmpDir := t.TempDir()
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{
				Type:      "file",
				Level:     "debug",
				Format:    "json",
				Path:      filepath.Join(tmpDir, "logs", "app.log"),
				MaxSizeMB: 10,
				MaxFiles:  5,
			},
		}

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("valid syslog output", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "syslog"},
		}

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("invalid output type", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "invalid"},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid output type")
	})

	t.Run("file output without path", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "file", Path: ""},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file output requires 'path' field")
	})

	t.Run("file output with relative path", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "file", Path: "relative/path.log"},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file path must be absolute")
	})

	t.Run("file output with negative max_size", func(t *testing.T) {
		stubHome(t)
		tmpDir := t.TempDir()
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "file", Path: filepath.Join(tmpDir, "app.log"), MaxSizeMB: -1},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_size_mb must be non-negative")
	})

	t.Run("file output with negative max_files", func(t *testing.T) {
		stubHome(t)
		tmpDir := t.TempDir()
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "file", Path: filepath.Join(tmpDir, "app.log"), MaxFiles: -1},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_files must be non-negative")
	})

	t.Run("output with invalid level", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "console", Level: "invalid"},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid log level")
	})

	t.Run("output with invalid format", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Logging.Outputs = []LogOutput{
			{Type: "console", Format: "invalid"},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid log format")
	})
}

// TestValidation_PluginConfigurations tests validation of plugin configurations.
func TestValidation_PluginConfigurations(t *testing.T) {
	t.Run("valid plugin name", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Plugins = map[string]PluginConfig{
			"aws-cost": {Config: map[string]interface{}{"region": "us-east-1"}},
		}

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("valid plugin name with underscore", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Plugins = map[string]PluginConfig{
			"aws_cost": {Config: map[string]interface{}{"region": "us-east-1"}},
		}

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("invalid plugin name with special chars", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Plugins = map[string]PluginConfig{
			"aws@cost": {Config: map[string]interface{}{}},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("invalid plugin name with spaces", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Plugins = map[string]PluginConfig{
			"aws cost": {Config: map[string]interface{}{}},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("plugin with empty config key", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.Plugins = map[string]PluginConfig{
			"aws": {Config: map[string]interface{}{"": "value"}},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty configuration key")
	})
}

// TestValidation_MalformedConfigFile tests handling of malformed config files.
func TestValidation_MalformedConfigFile(t *testing.T) {
	t.Run("invalid YAML syntax", func(t *testing.T) {
		stubHome(t)
		configDir := filepath.Join(os.Getenv("HOME"), ".finfocus")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(configDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("invalid: yaml: content: [unclosed"), 0644)
		require.NoError(t, err)

		cfg, err := NewStrict()
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "corrupted")
	})

	t.Run("invalid YAML types", func(t *testing.T) {
		stubHome(t)
		configDir := filepath.Join(os.Getenv("HOME"), ".finfocus")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		// Output precision should be an int, not a string
		configPath := filepath.Join(configDir, "config.yaml")
		err = os.WriteFile(configPath, []byte("output:\n  precision: \"not-a-number\""), 0644)
		require.NoError(t, err)

		cfg, err := NewStrict()
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("empty config file is valid", func(t *testing.T) {
		stubHome(t)
		configDir := filepath.Join(os.Getenv("HOME"), ".finfocus")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(configDir, "config.yaml")
		err = os.WriteFile(configPath, []byte(""), 0644)
		require.NoError(t, err)

		cfg, err := NewStrict()
		require.NoError(t, err)
		assert.NotNil(t, cfg)
	})
}

// TestValidation_SetErrors tests error paths in Set method.
func TestValidation_SetErrors(t *testing.T) {
	t.Run("invalid output key depth", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		err := cfg.Set("output.nested.key", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid output key")
	})

	t.Run("invalid logging key depth", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		err := cfg.Set("logging.nested.key", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logging key")
	})

	t.Run("unknown logging setting", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		err := cfg.Set("logging.unknown", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown logging setting")
	})
}

// TestValidation_GetErrors tests error paths in Get method.
func TestValidation_GetErrors(t *testing.T) {
	t.Run("invalid output key depth", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		_, err := cfg.Get("output.nested.key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid output key")
	})

	t.Run("invalid logging key depth", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		_, err := cfg.Get("logging.nested.key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logging key")
	})

	t.Run("unknown logging setting", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		_, err := cfg.Get("logging.unknown")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown logging setting")
	})

	t.Run("get all plugins", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.SetPluginConfig("test", map[string]interface{}{"key": "value"})

		value, err := cfg.Get("plugins")
		require.NoError(t, err)
		assert.NotNil(t, value)
	})

	t.Run("plugin config key not found", func(t *testing.T) {
		stubHome(t)
		cfg := New()
		cfg.SetPluginConfig("test", map[string]interface{}{"key": "value"})

		_, err := cfg.Get("plugins.test.nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config key not found")
	})
}

// TestValidation_InitLoggerErrors tests error paths in InitLogger.
func TestValidation_InitLoggerErrors(t *testing.T) {
	t.Run("invalid log level falls back to info", func(t *testing.T) {
		err := InitLogger("invalid-level", false)
		require.NoError(t, err)
		// Logger should still be initialized with info level
		assert.NotNil(t, GetLogger())
	})

	t.Run("log to file with valid path", func(t *testing.T) {
		stubHome(t)
		tmpDir := t.TempDir()

		// Set up config with valid log path
		cfg := GetGlobalConfig()
		cfg.Logging.File = filepath.Join(tmpDir, "logs", "test.log")

		err := InitLogger("debug", true)
		require.NoError(t, err)

		// Close log file to release handle (prevents Windows file locking)
		t.Cleanup(CloseLogFile)
	})
}

// TestValidation_SetLoggingFile tests the logging.file setting.
func TestValidation_SetLoggingFile(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Set logging file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	err := cfg.Set("logging.file", logPath)
	require.NoError(t, err)

	// Get logging file
	value, err := cfg.Get("logging.file")
	require.NoError(t, err)
	assert.Equal(t, logPath, value)
}

// TestValidation_GetLoggingFile tests getting logging.file setting.
func TestValidation_GetLoggingFile(t *testing.T) {
	stubHome(t)
	cfg := New()

	// Get default logging file
	value, err := cfg.Get("logging.file")
	require.NoError(t, err)
	assert.NotEmpty(t, value)
}

// TestValidation_SaveErrors tests error paths in Save method.
func TestValidation_SaveErrors(t *testing.T) {
	t.Run("save creates directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{
			Output: OutputConfig{
				DefaultFormat: "json",
				Precision:     2,
			},
			configPath: filepath.Join(tmpDir, "nested", "dir", "config.yaml"),
		}

		err := cfg.Save()
		require.NoError(t, err)

		// Verify file was created
		_, err = os.Stat(cfg.configPath)
		require.NoError(t, err)
	})
}

// TestValidation_SetPluginConfigNilMap tests SetPluginConfig with nil plugins map.
func TestValidation_SetPluginConfigNilMap(t *testing.T) {
	stubHome(t)
	cfg := New()
	cfg.Plugins = nil

	// Should not panic, should create the map
	cfg.SetPluginConfig("test", map[string]interface{}{"key": "value"})

	assert.NotNil(t, cfg.Plugins)
	assert.Contains(t, cfg.Plugins, "test")
}

// TestRoutingConfig_Validate tests RoutingConfig validation.
func TestRoutingConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *RoutingConfig
		wantErr     bool
		errContains string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "empty plugins",
			config:  &RoutingConfig{Plugins: []PluginRouting{}},
			wantErr: false,
		},
		{
			name: "empty plugin name",
			config: &RoutingConfig{Plugins: []PluginRouting{
				{
					Name: "",
					Patterns: []ResourcePattern{
						{Type: PatternTypeGlob, Pattern: "aws:*"},
					},
				},
			}},
			wantErr:     true,
			errContains: "plugin at index 0: name is required",
		},
		{
			name: "valid glob pattern",
			config: &RoutingConfig{
				Plugins: []PluginRouting{
					{
						Name: "test",
						Patterns: []ResourcePattern{
							{Type: PatternTypeGlob, Pattern: "aws:ec2/*"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid regex pattern",
			config: &RoutingConfig{
				Plugins: []PluginRouting{
					{
						Name: "test",
						Patterns: []ResourcePattern{
							{Type: PatternTypeRegex, Pattern: "^aws:(ec2|rds):.*$"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex pattern",
			config: &RoutingConfig{
				Plugins: []PluginRouting{
					{
						Name: "test",
						Patterns: []ResourcePattern{
							{Type: PatternTypeRegex, Pattern: "aws:(ec2|rds"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "invalid regex",
		},
		{
			name: "invalid glob pattern - bad syntax",
			config: &RoutingConfig{
				Plugins: []PluginRouting{
					{
						Name: "test",
						Patterns: []ResourcePattern{
							{Type: PatternTypeGlob, Pattern: "aws:["},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "invalid glob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidatePattern tests the validatePattern helper function.
func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name        string
		pluginName  string
		index       int
		pattern     ResourcePattern
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty pattern string",
			pluginName:  "test",
			index:       0,
			pattern:     ResourcePattern{Type: PatternTypeGlob, Pattern: ""},
			wantErr:     true,
			errContains: "pattern string is required",
		},
		{
			name:        "invalid pattern type",
			pluginName:  "test",
			index:       0,
			pattern:     ResourcePattern{Type: "wildcard", Pattern: "aws:*"},
			wantErr:     true,
			errContains: "invalid type",
		},
		{
			name:       "valid regex",
			pluginName: "test",
			index:      0,
			pattern:    ResourcePattern{Type: PatternTypeRegex, Pattern: "^aws:.*$"},
			wantErr:    false,
		},
		{
			name:        "invalid regex - unclosed group",
			pluginName:  "test",
			index:       0,
			pattern:     ResourcePattern{Type: PatternTypeRegex, Pattern: "aws:(ec2|rds"},
			wantErr:     true,
			errContains: "invalid regex",
		},
		{
			name:       "valid glob",
			pluginName: "test",
			index:      0,
			pattern:    ResourcePattern{Type: PatternTypeGlob, Pattern: "aws:ec2/*"},
			wantErr:    false,
		},
		{
			name:        "invalid glob - unclosed bracket",
			pluginName:  "test",
			index:       0,
			pattern:     ResourcePattern{Type: PatternTypeGlob, Pattern: "aws:[a-z"},
			wantErr:     true,
			errContains: "invalid glob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePattern(tt.pluginName, tt.index, tt.pattern)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
