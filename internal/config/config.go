package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// Duration is a wrapper around time.Duration that supports YAML/JSON parsing.
//
//nolint:recvcheck // MarshalYAML returns value, UnmarshalYAML needs pointer - this is standard YAML pattern
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler for Duration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalYAML implements yaml.Marshaler for Duration.
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// Duration returns the underlying time.Duration value.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

const (
	defaultPrecision     = 2
	envKeySeparatorCount = 2
	minPluginKeyParts    = 2

	// Analyzer timeout defaults.
	defaultPerResourceTimeout   = 5 * time.Second
	defaultTotalTimeout         = 60 * time.Second
	defaultWarnThresholdTimeout = 30 * time.Second

	// Cache defaults.
	defaultCacheTTLSeconds = 3600 // 1 hour default
	defaultCacheMaxSizeMB  = 100  // 100 MB default
)

// ErrConfigCorrupted is returned in strict mode when the config file exists but cannot be parsed.
var ErrConfigCorrupted = errors.New("configuration file appears corrupted")

// Config represents the complete configuration structure.
type Config struct {
	// Legacy fields for backward compatibility
	PluginDir string `yaml:"-" json:"-"`
	SpecDir   string `yaml:"-" json:"-"`

	// New comprehensive configuration
	Output           OutputConfig            `yaml:"output"      json:"output"`
	Plugins          map[string]PluginConfig `yaml:"plugins"     json:"plugins"`
	Logging          LoggingConfig           `yaml:"logging"     json:"logging"`
	Analyzer         AnalyzerConfig          `yaml:"analyzer"    json:"analyzer"`
	PluginHostConfig PluginHostConfig        `yaml:"plugin_host" json:"plugin_host"`
	Cost             CostConfig              `yaml:"cost"        json:"cost"`

	// Internal fields
	configPath string
}

// PluginHostConfig defines plugin host behavior settings.
type PluginHostConfig struct {
	// StrictCompatibility blocks plugin initialization on spec version mismatch.
	// When true, plugins with incompatible spec versions will fail to load.
	// When false (default), a warning is logged but initialization continues.
	StrictCompatibility bool `yaml:"strict_compatibility" json:"strict_compatibility"`
}

// OutputConfig defines output formatting preferences.
type OutputConfig struct {
	DefaultFormat string `yaml:"default_format" json:"default_format"`
	Precision     int    `yaml:"precision"      json:"precision"`
}

// PluginConfig defines plugin-specific configuration.
type PluginConfig struct {
	Config map[string]interface{} `yaml:",inline" json:",inline"`
}

// LoggingConfig defines logging preferences.
type LoggingConfig struct {
	Level   string      `yaml:"level"   json:"level"`
	Format  string      `yaml:"format"  json:"format"`  // "json" or "text"
	Outputs []LogOutput `yaml:"outputs" json:"outputs"` // Multiple output destinations
	File    string      `yaml:"file"    json:"file"`    // Legacy: single file output
	Audit   AuditConfig `yaml:"audit"   json:"audit"`   // Audit logging configuration
}

// AuditConfig defines audit logging settings for cost query operations.
type AuditConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"` // Enable audit logging
	File    string `yaml:"file"    json:"file"`    // Separate audit file (optional, empty = main log)
}

// LogOutput defines a logging output destination.
type LogOutput struct {
	Type      string `yaml:"type"                  json:"type"`                  // "console", "file", "syslog"
	Level     string `yaml:"level,omitempty"       json:"level,omitempty"`       // Optional: override global level
	Path      string `yaml:"path,omitempty"        json:"path,omitempty"`        // For file type
	Format    string `yaml:"format,omitempty"      json:"format,omitempty"`      // Optional: override global format
	MaxSizeMB int    `yaml:"max_size_mb,omitempty" json:"max_size_mb,omitempty"` // File rotation
	MaxFiles  int    `yaml:"max_files,omitempty"   json:"max_files,omitempty"`   // File rotation
}

// AnalyzerConfig defines analyzer-specific configuration for the Pulumi Analyzer plugin.
type AnalyzerConfig struct {
	Timeout AnalyzerTimeout           `yaml:"timeout" json:"timeout"`
	Plugins map[string]AnalyzerPlugin `yaml:"plugins" json:"plugins"`
}

// AnalyzerTimeout defines timeout settings for cost analysis operations.
type AnalyzerTimeout struct {
	PerResource   Duration `yaml:"per_resource"   json:"per_resource"`   // Per-resource timeout (default: 5s)
	Total         Duration `yaml:"total"          json:"total"`          // Overall analysis timeout (default: 60s)
	WarnThreshold Duration `yaml:"warn_threshold" json:"warn_threshold"` // Warning threshold (default: 30s)
}

// AnalyzerPlugin defines a cost plugin configuration for the analyzer.
type AnalyzerPlugin struct {
	Path    string            `yaml:"path"    json:"path"`    // Path to plugin binary
	Enabled bool              `yaml:"enabled" json:"enabled"` // Whether plugin is enabled
	Env     map[string]string `yaml:"env"     json:"env"`     // Environment variables for plugin
}

// ResolveConfigDir determines the configuration directory based on environment variables.
// It follows this precedence order:
// 1. $FINFOCUS_HOME - explicit override.
// 2. $PULUMI_HOME/finfocus/ - if PULUMI_HOME is set (Pulumi ecosystem integration).
// 3. $HOME/.finfocus/ - default fallback (standard behavior).
// 4. ./.finfocus - fallback of last resort if home directory cannot be determined.
func ResolveConfigDir() string {
	// Check explicit FINFOCUS_HOME first
	if ffHome := os.Getenv("FINFOCUS_HOME"); ffHome != "" {
		return ffHome
	}

	// Check PULUMI_HOME (Pulumi ecosystem integration)
	if pulumiHome := os.Getenv("PULUMI_HOME"); pulumiHome != "" {
		return filepath.Join(pulumiHome, "finfocus")
	}

	// Fall back to HOME/.finfocus
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		// Last resort fallback - use current working directory
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return ".finfocus"
		}
		return filepath.Join(cwd, ".finfocus")
	}
	return filepath.Join(homeDir, ".finfocus")
}

// New creates a new configuration with defaults.
// In strict mode (FINFOCUS_CONFIG_STRICT=true), corrupted config files cause a panic.
func New() *Config {
	finfocusDir := ResolveConfigDir()

	cfg := &Config{
		// Legacy fields
		PluginDir: filepath.Join(finfocusDir, "plugins"),
		SpecDir:   filepath.Join(finfocusDir, "specs"),

		// New configuration
		Output: OutputConfig{
			DefaultFormat: "table",
			Precision:     defaultPrecision,
		},
		Plugins: make(map[string]PluginConfig),
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   filepath.Join(finfocusDir, "logs", "finfocus.log"),
			Outputs: []LogOutput{
				{
					Type:   "console",
					Level:  "info",
					Format: "text",
				},
			},
		},
		Analyzer: AnalyzerConfig{
			Timeout: AnalyzerTimeout{
				PerResource:   Duration(defaultPerResourceTimeout),
				Total:         Duration(defaultTotalTimeout),
				WarnThreshold: Duration(defaultWarnThresholdTimeout),
			},
			Plugins: make(map[string]AnalyzerPlugin),
		},
		Cost: CostConfig{
			Cache: CacheConfig{
				Enabled:    true,
				TTLSeconds: defaultCacheTTLSeconds,
				Directory:  filepath.Join(finfocusDir, "cache"),
				MaxSizeMB:  defaultCacheMaxSizeMB,
			},
		},

		configPath: filepath.Join(finfocusDir, "config.yaml"),
	}

	// Check for strict mode
	strictMode := os.Getenv("FINFOCUS_CONFIG_STRICT") == "true" ||
		os.Getenv("FINFOCUS_CONFIG_STRICT") == "1"

	// Load from file if exists
	if err := cfg.Load(); err != nil {
		switch {
		case os.IsNotExist(err):
			// Config file doesn't exist - this is fine, use defaults
		case os.IsPermission(err):
			// Permission error - warn but continue (or fail in strict mode)
			if strictMode {
				panic(fmt.Sprintf("STRICT MODE: Permission denied reading config file: %v", err))
			}
			fmt.Fprintf(os.Stderr, "Warning: Permission denied reading config file: %v\n", err)
		default:
			// Likely a corrupted config file - fail in strict mode
			if strictMode {
				panic(fmt.Sprintf("STRICT MODE: Config file appears corrupted: %v", err))
			}
			fmt.Fprintf(
				os.Stderr,
				"Warning: Config file may be corrupted, using defaults: %v\n",
				err,
			)
		}
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	return cfg
}

// NewStrict creates a new configuration with strict error handling.
// It returns an error instead of using defaults if the config file exists but cannot be parsed.
// It uses ResolveConfigDir() to respect PULUMI_HOME when set.
func NewStrict() (*Config, error) {
	finfocusDir := ResolveConfigDir()

	cfg := &Config{
		// Legacy fields
		PluginDir: filepath.Join(finfocusDir, "plugins"),
		SpecDir:   filepath.Join(finfocusDir, "specs"),

		// New configuration
		Output: OutputConfig{
			DefaultFormat: "table",
			Precision:     defaultPrecision,
		},
		Plugins: make(map[string]PluginConfig),
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   filepath.Join(finfocusDir, "logs", "finfocus.log"),
			Outputs: []LogOutput{
				{
					Type:   "console",
					Level:  "info",
					Format: "text",
				},
			},
		},
		Analyzer: AnalyzerConfig{
			Timeout: AnalyzerTimeout{
				PerResource:   Duration(defaultPerResourceTimeout),
				Total:         Duration(defaultTotalTimeout),
				WarnThreshold: Duration(defaultWarnThresholdTimeout),
			},
			Plugins: make(map[string]AnalyzerPlugin),
		},
		Cost: CostConfig{
			Cache: CacheConfig{
				Enabled:    true,
				TTLSeconds: defaultCacheTTLSeconds,
				Directory:  filepath.Join(finfocusDir, "cache"),
				MaxSizeMB:  defaultCacheMaxSizeMB,
			},
		},

		configPath: filepath.Join(finfocusDir, "config.yaml"),
	}

	// Load from file with strict error handling
	if loadErr := cfg.Load(); loadErr != nil {
		switch {
		case os.IsNotExist(loadErr):
			// Config file doesn't exist - this is fine, use defaults
		case os.IsPermission(loadErr):
			// Permission error - fail immediately
			return nil, fmt.Errorf("permission denied reading config file: %w", loadErr)
		default:
			// Likely a corrupted config file - fail immediately
			return nil, fmt.Errorf("%w: %w", ErrConfigCorrupted, loadErr)
		}
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Validate the configuration
	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", validateErr)
	}

	return cfg, nil
}

// Load loads configuration from the config file.
func (c *Config) Load() error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, c)
}

// Save saves the current configuration to the config file.
func (c *Config) Save() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(c.configPath, data, 0600)
}

// Set sets a configuration value using dot notation.
func (c *Config) Set(key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) < 1 {
		return errors.New("invalid key format")
	}

	switch parts[0] {
	case "output":
		return c.setOutputValue(parts[1:], value)
	case "plugins":
		return c.setPluginValue(parts[1:], value)
	case "logging":
		return c.setLoggingValue(parts[1:], value)
	case "plugin_host":
		return c.setPluginHostValue(parts[1:], value)
	case "cost":
		return c.setCostValue(parts[1:], value)
	default:
		return fmt.Errorf("unknown configuration section: %s", parts[0])
	}
}

// Get gets a configuration value using dot notation.
func (c *Config) Get(key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	if len(parts) < 1 {
		return nil, errors.New("invalid key format")
	}

	switch parts[0] {
	case "output":
		return c.getOutputValue(parts[1:])
	case "plugins":
		return c.getPluginValue(parts[1:])
	case "logging":
		return c.getLoggingValue(parts[1:])
	case "plugin_host":
		return c.getPluginHostValue(parts[1:])
	case "cost":
		return c.getCostValue(parts[1:])
	default:
		return nil, fmt.Errorf("unknown configuration section: %s", parts[0])
	}
}

// List returns all configuration as a map.
func (c *Config) List() map[string]interface{} {
	return map[string]interface{}{
		"output":      c.Output,
		"plugins":     c.Plugins,
		"logging":     c.Logging,
		"analyzer":    c.Analyzer,
		"plugin_host": c.PluginHostConfig,
		"cost":        c.Cost,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate output format
	validFormats := []string{"table", "json", "ndjson"}
	valid := false
	for _, format := range validFormats {
		if c.Output.DefaultFormat == format {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf(
			"invalid output format: %s (must be one of: %v)",
			c.Output.DefaultFormat,
			validFormats,
		)
	}

	// Validate precision
	if c.Output.Precision < 0 || c.Output.Precision > 10 {
		return fmt.Errorf("invalid precision: %d (must be between 0 and 10)", c.Output.Precision)
	}

	// Validate logging configuration
	if err := c.validateLogging(); err != nil {
		return fmt.Errorf("logging configuration validation failed: %w", err)
	}

	// Validate plugin configurations
	if err := c.validatePluginConfigurations(); err != nil {
		return fmt.Errorf("plugin configuration validation failed: %w", err)
	}

	// Validate cost configuration
	if err := c.Cost.Validate(); err != nil {
		return fmt.Errorf("cost configuration validation failed: %w", err)
	}

	return nil
}

// validateLogging validates logging configuration.
func (c *Config) validateLogging() error {
	// Validate logging level
	if c.Logging.Level != "" {
		if err := isValidLevel(c.Logging.Level); err != nil {
			return err
		}
	}

	// Validate logging format
	if c.Logging.Format != "" {
		if err := isValidFormat(c.Logging.Format); err != nil {
			return err
		}
	}

	// Validate legacy file path (if specified)
	if c.Logging.File != "" {
		if err := validateFilePath(c.Logging.File); err != nil {
			return err
		}
	}

	// Validate outputs
	for i, output := range c.Logging.Outputs {
		if err := validateLogOutput(output); err != nil {
			return fmt.Errorf("output %d: %w", i, err)
		}
	}

	// Validate audit configuration
	if err := c.validateAuditConfig(); err != nil {
		return fmt.Errorf("audit configuration: %w", err)
	}

	return nil
}

// validateAuditConfig validates the audit logging configuration.
func (c *Config) validateAuditConfig() error {
	// If audit logging is disabled, no further validation needed
	if !c.Logging.Audit.Enabled {
		return nil
	}

	// Validate audit file path if specified
	if c.Logging.Audit.File != "" {
		if err := validateFilePath(c.Logging.Audit.File); err != nil {
			return fmt.Errorf("audit file: %w", err)
		}
	}

	return nil
}

func isValidLevel(level string) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, validLevel := range validLevels {
		if level == validLevel {
			return nil
		}
	}
	return fmt.Errorf("invalid log level: %s (must be one of: %v)", level, validLevels)
}

func isValidFormat(format string) error {
	validFormats := []string{"json", "text"}
	for _, validFormat := range validFormats {
		if format == validFormat {
			return nil
		}
	}
	return fmt.Errorf("invalid log format: %s (must be one of: %v)", format, validFormats)
}

func validateFilePath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("log file path must be absolute: %s", path)
	}

	// Check if parent directory is accessible
	logDir := filepath.Dir(path)
	if _, statErr := os.Stat(logDir); statErr != nil && os.IsNotExist(statErr) {
		// Try to create the directory
		if mkdirErr := os.MkdirAll(logDir, 0700); mkdirErr != nil {
			return fmt.Errorf("cannot create log directory %s: %w", logDir, mkdirErr)
		}
	}

	return nil
}

const (
	outputTypeFile = "file"
)

// validateLogOutput validates a single log output configuration.
func validateLogOutput(output LogOutput) error {
	// Validate output type
	if err := validateOutputType(output.Type); err != nil {
		return err
	}

	// Validate level if specified
	if output.Level != "" {
		if err := isValidLevel(output.Level); err != nil {
			return err
		}
	}

	// Validate format if specified
	if output.Format != "" {
		if err := isValidFormat(output.Format); err != nil {
			return err
		}
	}

	// File-specific validations
	if output.Type == outputTypeFile {
		return validateFileOutput(output)
	}

	return nil
}

func validateOutputType(outputType string) error {
	validTypes := []string{"console", outputTypeFile, "syslog"}
	for _, t := range validTypes {
		if outputType == t {
			return nil
		}
	}
	return fmt.Errorf("invalid output type: %s (must be one of: %v)", outputType, validTypes)
}

// - MaxSizeMB or MaxFiles is negative (values of 0 are allowed and mean unlimited).
func validateFileOutput(output LogOutput) error {
	if output.Path == "" {
		return errors.New("file output requires 'path' field")
	}
	if !filepath.IsAbs(output.Path) {
		return fmt.Errorf("file path must be absolute: %s", output.Path)
	}

	// Check if parent directory is accessible
	logDir := filepath.Dir(output.Path)
	if _, statErr := os.Stat(logDir); statErr != nil && os.IsNotExist(statErr) {
		// Try to create the directory
		if mkdirErr := os.MkdirAll(logDir, 0700); mkdirErr != nil {
			return fmt.Errorf("cannot create log directory %s: %w", logDir, mkdirErr)
		}
	}

	// Validate rotation settings
	if output.MaxSizeMB < 0 {
		return fmt.Errorf(
			"max_size_mb must be non-negative (0 means unlimited), got: %d",
			output.MaxSizeMB,
		)
	}
	if output.MaxFiles < 0 {
		return fmt.Errorf(
			"max_files must be non-negative (0 means unlimited), got: %d",
			output.MaxFiles,
		)
	}

	return nil
}

// validatePluginConfigurations validates plugin-specific configurations.
func (c *Config) validatePluginConfigurations() error {
	for pluginName, pluginConfig := range c.Plugins {
		// Validate plugin name (must be valid identifier)
		if pluginName == "" {
			return errors.New("plugin name cannot be empty")
		}

		// Check for suspicious characters that might cause issues
		for _, char := range pluginName {
			if (char < 'a' || char > 'z') &&
				(char < 'A' || char > 'Z') &&
				(char < '0' || char > '9') &&
				char != '-' && char != '_' {
				return fmt.Errorf("plugin name contains invalid character: %s", pluginName)
			}
		}

		// Validate plugin configuration keys
		for configKey := range pluginConfig.Config {
			if configKey == "" {
				return fmt.Errorf("plugin %s has empty configuration key", pluginName)
			}
		}
	}

	return nil
}

// PluginPath returns the path for a specific plugin version (backward compatibility).
func (c *Config) PluginPath(name, version string) string {
	return filepath.Join(c.PluginDir, name, version)
}

// GetPluginConfig returns configuration for a specific plugin.
func (c *Config) GetPluginConfig(pluginName string) (map[string]interface{}, error) {
	if plugin, exists := c.Plugins[pluginName]; exists {
		return plugin.Config, nil
	}
	return make(map[string]interface{}), nil
}

// SetPluginConfig sets configuration for a specific plugin.
func (c *Config) SetPluginConfig(pluginName string, config map[string]interface{}) {
	if c.Plugins == nil {
		c.Plugins = make(map[string]PluginConfig)
	}
	c.Plugins[pluginName] = PluginConfig{Config: config}
}

// applyEnvOverrides applies environment variable overrides.
//
//nolint:gocognit // Complexity is inherent to handling multiple env overrides
func (c *Config) applyEnvOverrides() {
	// Compatibility layer for legacy PULUMICOST_ variables
	if os.Getenv("FINFOCUS_COMPAT") == "1" || os.Getenv("FINFOCUS_COMPAT") == "true" {
		c.applyLegacyEnvOverrides()
	}

	// Output overrides
	if format := os.Getenv("FINFOCUS_OUTPUT_FORMAT"); format != "" {
		c.Output.DefaultFormat = format
	}
	if precision := os.Getenv("FINFOCUS_OUTPUT_PRECISION"); precision != "" {
		if p, err := strconv.Atoi(precision); err == nil {
			c.Output.Precision = p
		}
	}

	// Logging overrides using pluginsdk constants for consistency with plugins
	if level := os.Getenv(pluginsdk.EnvLogLevel); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv(pluginsdk.EnvLogFormat); format != "" {
		c.Logging.Format = format
	}
	if logFile := os.Getenv(pluginsdk.EnvLogFile); logFile != "" {
		c.Logging.File = logFile
	}

	// Cache configuration overrides (T067)
	if enabled := os.Getenv("FINFOCUS_CACHE_ENABLED"); enabled != "" {
		if e, err := strconv.ParseBool(enabled); err == nil {
			c.Cost.Cache.Enabled = e
		}
	}
	if ttl := os.Getenv("FINFOCUS_CACHE_TTL_SECONDS"); ttl != "" {
		if t, err := strconv.Atoi(ttl); err == nil {
			c.Cost.Cache.TTLSeconds = t
		}
	}
	if dir := os.Getenv("FINFOCUS_CACHE_DIR"); dir != "" {
		c.Cost.Cache.Directory = dir
	}
	if maxSize := os.Getenv("FINFOCUS_CACHE_MAX_SIZE_MB"); maxSize != "" {
		if m, err := strconv.Atoi(maxSize); err == nil {
			c.Cost.Cache.MaxSizeMB = m
		} else {
			log.Debug().
				Str("env_var", "FINFOCUS_CACHE_MAX_SIZE_MB").
				Str("value", maxSize).
				Err(err).
				Msg("failed to parse cache max size, using default")
		}
	}

	// Budget exit configuration overrides (Issue #219)
	if exitOnThreshold := os.Getenv("FINFOCUS_BUDGET_EXIT_ON_THRESHOLD"); exitOnThreshold != "" {
		if e, err := strconv.ParseBool(exitOnThreshold); err == nil {
			c.ensureBudgetsConfig()
			c.Cost.Budgets.Global.ExitOnThreshold = &e
		}
	}
	if exitCode := os.Getenv("FINFOCUS_BUDGET_EXIT_CODE"); exitCode != "" {
		if code, err := strconv.Atoi(exitCode); err == nil {
			c.ensureBudgetsConfig()
			c.Cost.Budgets.Global.ExitCode = &code
		}
	}

	// Plugin overrides (FINFOCUS_PLUGIN_<NAME>_<KEY>=value)
	c.scanPluginEnvironmentVars()
}

// applyLegacyEnvOverrides handles backward compatibility for PULUMICOST_ variables.
func (c *Config) applyLegacyEnvOverrides() {
	if format := os.Getenv("PULUMICOST_OUTPUT_FORMAT"); format != "" {
		c.Output.DefaultFormat = format
	}
	if precision := os.Getenv("PULUMICOST_OUTPUT_PRECISION"); precision != "" {
		if p, err := strconv.Atoi(precision); err == nil {
			c.Output.Precision = p
		}
	}
	if level := os.Getenv("PULUMICOST_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv("PULUMICOST_LOG_FORMAT"); format != "" {
		c.Logging.Format = format
	}
	if logFile := os.Getenv("PULUMICOST_LOG_FILE"); logFile != "" {
		c.Logging.File = logFile
	}

	// Legacy plugin overrides
	const legacyPluginPrefix = "PULUMICOST_PLUGIN_"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, legacyPluginPrefix) {
			continue
		}
		parts := strings.SplitN(env, "=", envKeySeparatorCount)
		if len(parts) != envKeySeparatorCount {
			continue
		}
		key := parts[0]
		value := parts[1]
		keyParts := strings.Split(strings.TrimPrefix(key, legacyPluginPrefix), "_")
		if len(keyParts) < minPluginKeyParts {
			continue
		}
		pluginName := strings.ToLower(keyParts[0])
		configKey := strings.ToLower(strings.Join(keyParts[1:], "_"))
		if c.Plugins == nil {
			c.Plugins = make(map[string]PluginConfig)
		}
		if _, exists := c.Plugins[pluginName]; !exists {
			c.Plugins[pluginName] = PluginConfig{Config: make(map[string]interface{})}
		}
		c.Plugins[pluginName].Config[configKey] = value
	}
}

// scanPluginEnvironmentVars efficiently scans for plugin-specific environment variables.
func (c *Config) scanPluginEnvironmentVars() {
	const pluginPrefix = "FINFOCUS_PLUGIN_"

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, pluginPrefix) {
			continue
		}

		parts := strings.SplitN(env, "=", envKeySeparatorCount)
		if len(parts) != envKeySeparatorCount {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Extract plugin name and config key
		keyParts := strings.Split(strings.TrimPrefix(key, pluginPrefix), "_")
		if len(keyParts) < minPluginKeyParts {
			continue
		}

		pluginName := strings.ToLower(keyParts[0])
		configKey := strings.ToLower(strings.Join(keyParts[1:], "_"))

		if c.Plugins == nil {
			c.Plugins = make(map[string]PluginConfig)
		}

		if _, exists := c.Plugins[pluginName]; !exists {
			c.Plugins[pluginName] = PluginConfig{Config: make(map[string]interface{})}
		}

		c.Plugins[pluginName].Config[configKey] = value
	}
}

// Helper methods for setting values.
func (c *Config) setOutputValue(parts []string, value string) error {
	if len(parts) != 1 {
		return errors.New("invalid output key")
	}

	switch parts[0] {
	case "default_format":
		c.Output.DefaultFormat = value
	case "precision":
		p, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("precision must be a number: %w", err)
		}
		c.Output.Precision = p
	default:
		return fmt.Errorf("unknown output setting: %s", parts[0])
	}

	return nil
}

func (c *Config) setPluginValue(parts []string, value string) error {
	if len(parts) < minPluginKeyParts {
		return errors.New("plugin key must be in format plugins.<name>.<key>")
	}

	pluginName := parts[0]
	configKey := strings.Join(parts[1:], ".")

	if c.Plugins == nil {
		c.Plugins = make(map[string]PluginConfig)
	}

	if _, exists := c.Plugins[pluginName]; !exists {
		c.Plugins[pluginName] = PluginConfig{Config: make(map[string]interface{})}
	}

	c.Plugins[pluginName].Config[configKey] = value
	return nil
}

func (c *Config) setLoggingValue(parts []string, value string) error {
	if len(parts) != 1 {
		return errors.New("invalid logging key")
	}

	switch parts[0] {
	case "level":
		c.Logging.Level = value
	case "file":
		c.Logging.File = value
	default:
		return fmt.Errorf("unknown logging setting: %s", parts[0])
	}

	return nil
}

// Helper methods for getting values.
func (c *Config) getOutputValue(parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, errors.New("invalid output key")
	}

	switch parts[0] {
	case "default_format":
		return c.Output.DefaultFormat, nil
	case "precision":
		return c.Output.Precision, nil
	default:
		return nil, fmt.Errorf("unknown output setting: %s", parts[0])
	}
}

func (c *Config) getPluginValue(parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return c.Plugins, nil
	}

	pluginName := parts[0]
	plugin, exists := c.Plugins[pluginName]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginName)
	}

	if len(parts) == 1 {
		return plugin.Config, nil
	}

	configKey := strings.Join(parts[1:], ".")
	value, exists := plugin.Config[configKey]
	if !exists {
		return nil, fmt.Errorf("config key not found: %s", configKey)
	}

	return value, nil
}

func (c *Config) getLoggingValue(parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, errors.New("invalid logging key")
	}

	switch parts[0] {
	case "level":
		return c.Logging.Level, nil
	case "file":
		return c.Logging.File, nil
	default:
		return nil, fmt.Errorf("unknown logging setting: %s", parts[0])
	}
}

// Helper methods for plugin_host values.
func (c *Config) setPluginHostValue(parts []string, value string) error {
	if len(parts) != 1 {
		return errors.New("invalid plugin_host key")
	}

	switch parts[0] {
	case "strict_compatibility":
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("strict_compatibility must be a boolean: %w", err)
		}
		c.PluginHostConfig.StrictCompatibility = boolVal
	default:
		return fmt.Errorf("unknown plugin_host setting: %s", parts[0])
	}

	return nil
}

func (c *Config) getPluginHostValue(parts []string) (interface{}, error) {
	if len(parts) != 1 {
		return nil, errors.New("invalid plugin_host key")
	}

	switch parts[0] {
	case "strict_compatibility":
		return c.PluginHostConfig.StrictCompatibility, nil
	default:
		return nil, fmt.Errorf("unknown plugin_host setting: %s", parts[0])
	}
}

// Helper methods for cost values.
func (c *Config) setCostValue(parts []string, value string) error {
	if len(parts) < 1 {
		return errors.New("invalid cost key")
	}

	switch parts[0] {
	case "budgets":
		return c.setCostBudgetsValue(parts[1:], value)
	default:
		return fmt.Errorf("unknown cost setting: %s", parts[0])
	}
}

func (c *Config) setCostBudgetsValue(parts []string, value string) error {
	if len(parts) != 1 {
		return errors.New("invalid cost.budgets key")
	}

	switch parts[0] {
	case "amount":
		amount, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("amount must be a number: %w", err)
		}
		c.ensureBudgetsConfig()
		c.Cost.Budgets.Global.Amount = amount
	case "currency":
		c.ensureBudgetsConfig()
		c.Cost.Budgets.Global.Currency = value
	case "period":
		c.ensureBudgetsConfig()
		c.Cost.Budgets.Global.Period = value
	case "alerts":
		return errors.New("cost.budgets.alerts must be configured via YAML")
	default:
		return fmt.Errorf("unknown cost.budgets setting: %s", parts[0])
	}

	return nil
}

// ensureBudgetsConfig ensures that the Budgets and Global structures are initialized.
func (c *Config) ensureBudgetsConfig() {
	if c.Cost.Budgets == nil {
		c.Cost.Budgets = &BudgetsConfig{}
	}
	if c.Cost.Budgets.Global == nil {
		c.Cost.Budgets.Global = &ScopedBudget{}
	}
}

func (c *Config) getCostValue(parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return c.Cost, nil
	}

	switch parts[0] {
	case "budgets":
		return c.getCostBudgetsValue(parts[1:])
	default:
		return nil, fmt.Errorf("unknown cost setting: %s", parts[0])
	}
}

func (c *Config) getCostBudgetsValue(parts []string) (interface{}, error) {
	if len(parts) < 1 {
		return c.Cost.Budgets, nil
	}

	// Access global budget values for backwards compatibility
	if c.Cost.Budgets == nil || c.Cost.Budgets.Global == nil {
		return nil, errors.New("no budget configured")
	}

	switch parts[0] {
	case "amount":
		return c.Cost.Budgets.Global.Amount, nil
	case "currency":
		return c.Cost.Budgets.Global.Currency, nil
	case "period":
		return c.Cost.Budgets.Global.GetPeriod(), nil
	default:
		return nil, fmt.Errorf("unknown cost.budgets setting: %s", parts[0])
	}
}

// GetOutputFormat selects the output format, preferring the provided userChoice over the configured default.
// It returns userChoice if non-empty; otherwise it returns the DefaultFormat from the global configuration.
func GetOutputFormat(userChoice string) string {
	// If user provided a format, use it
	if userChoice != "" {
		return userChoice
	}

	// Try to get default from configuration (use singleton to avoid side effects)
	cfg := GetGlobalConfig()
	return cfg.Output.DefaultFormat
}
