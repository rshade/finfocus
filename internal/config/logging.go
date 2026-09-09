package config

import (
	"github.com/rshade/finfocus/internal/logging"
)

// ToLoggingConfig converts config.LoggingConfig to logging.Config for use with
// the internal/logging package. This bridges the configuration system to the
// logging infrastructure.
//
// The conversion applies these rules:
//   - Level, Format are copied directly
//   - If File is set, Output becomes "file" and File is passed through
//   - If File is empty, Output defaults to "stderr"
func (lc *LoggingConfig) ToLoggingConfig() logging.Config {
	output := "stderr"
	if lc.File != "" {
		output = outputTypeFile
	}

	return logging.Config{
		Level:  lc.Level,
		Format: lc.Format,
		Output: output,
		File:   lc.File,
		Caller: false, // Default, can be extended if needed
	}
}

// GetLoggingConfig returns the Logging section of the global configuration.
// The returned value is a copy of the current global config's Logging settings.
// Any environment-level overrides (for example a --debug flag) are expected to
// be applied by the caller after retrieving this value.
func GetLoggingConfig() LoggingConfig {
	cfg := GetGlobalConfig()
	return cfg.Logging
}
