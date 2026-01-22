package config

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus/internal/logging"
)

// Logger is the global zerolog logger instance.
//
//nolint:gochecknoglobals // Logger is intentionally global for application-wide structured logging
var Logger zerolog.Logger

// InitLogger initializes the package-level Logger with the specified log level and optional file output.
// It sets the global Logger, configures console output, and—when logToFile is true—ensures the log directory
// exists and opens the configured log file (falling back to "/tmp/finfocus.log" if none is set).
//
// level is parsed into a zerolog level and defaults to InfoLevel on parse error.
// logToFile enables writing logs to the configured file in addition to the console.
//
// It returns an error if directory creation or opening the log file fails, otherwise nil.
func InitLogger(level string, logToFile bool) error {
	// Parse log level
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	// Set up output writers
	var writers []io.Writer

	// Console writer with human-readable format
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}
	writers = append(writers, consoleWriter)

	// File writer if enabled
	if logToFile {
		if logDirErr := EnsureLogDir(); logDirErr != nil {
			return logDirErr
		}

		cfg := GetGlobalConfig()
		logPath := cfg.Logging.File
		if logPath == "" {
			logPath = "/tmp/finfocus.log"
		}

		logFile, fileErr := os.OpenFile(
			logPath,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0600,
		)
		if fileErr != nil {
			return fileErr
		}
		writers = append(writers, logFile)
	}

	// Create multi-writer
	multi := zerolog.MultiLevelWriter(writers...)

	// Initialize logger
	Logger = zerolog.New(multi).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Logger()

	return nil
}

// SetLogLevel sets the package global Logger's level to the value parsed from level.
// If the provided level cannot be parsed, the logger level is set to zerolog.InfoLevel.
func SetLogLevel(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	Logger = Logger.Level(lvl)
}

// GetLogger returns the global logger instance.
func GetLogger() zerolog.Logger {
	return Logger
}

// init initializes the package-level default logger to info level with console output only.
// It calls InitLogger("info", false) and deliberately ignores any returned error.
// This init is intentional: the package requires a logger to be available before any
// configuration is loaded.
//
//nolint:gochecknoinits // intentional: package-level logger must be initialized before use
func init() {
	// Default to info level, console only
	_ = InitLogger("info", false)
}

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
