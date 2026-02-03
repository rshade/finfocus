package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// fallbackCounter provides uniqueness for last-resort trace ID generation (atomic for thread safety).
//
//nolint:gochecknoglobals // Package-level counter required for fallback trace ID uniqueness across goroutines
var fallbackCounter atomic.Uint32

// traceIDKey is a private type for context keys to avoid collisions.
type traceIDKey struct{}

// Config holds logging configuration settings.
type Config struct {
	Level      string // Log level: trace, debug, info, warn, error
	Format     string // Output format: json, console, text
	Output     string // Output destination: stderr, stdout, file
	File       string // File path when Output is "file"
	Caller     bool   // Include file:line in output
	StackTrace bool   // Include stack trace on errors
}

// TracingHook implements zerolog.Hook to automatically inject trace_id from context.
type TracingHook struct{}

// Run implements zerolog.Hook interface.
// It extracts trace_id from the event's context and adds it to the log entry.
func (h TracingHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	ctx := e.GetCtx()
	if ctx == nil {
		return
	}
	if traceID, ok := ctx.Value(traceIDKey{}).(string); ok && traceID != "" {
		e.Str("trace_id", traceID)
	}
}

// LoggingConfig is an alias for Config for backward compatibility.
//
//nolint:revive // Backward compatibility alias intentionally uses stuttering name
type LoggingConfig = Config

// LogPathResult contains the result of logger creation with file path information.
// This allows the CLI to communicate log file location to operators.
// Callers should call Close() when done to release any file handles.
type LogPathResult struct {
	Logger         zerolog.Logger // The created logger
	FilePath       string         // Path to log file (empty if not using file)
	UsingFile      bool           // True if logging to file
	FallbackUsed   bool           // True if fallback to stderr occurred
	FallbackReason string         // Reason for fallback (if any)
	file           *os.File       // Internal: file handle for cleanup
}

// NewLoggerWithPath creates a zerolog logger according to cfg and reports the chosen log destination.
//
// If cfg.Output is "file" and cfg.File is non-empty, NewLoggerWithPath attempts to open or create
// the specified file and, on success, returns a logger that writes to that file and sets LogPathResult.FilePath
// and LogPathResult.UsingFile = true. If opening the file fails, the function falls back to stderr,
// sets LogPathResult.FallbackUsed = true and LogPathResult.FallbackReason to the error string, and
// returns a logger that writes to stderr. If cfg.File is empty the function uses stderr.
//
// If cfg.Output is "stdout" the returned logger writes to stdout. For any other cfg.Output value
// the returned logger writes to stderr.
//
// The returned LogPathResult contains the constructed Logger and metadata describing whether a file
// was used, the file path (if any), and whether a fallback to stderr occurred with its reason.
func NewLoggerWithPath(cfg Config) LogPathResult {
	result := LogPathResult{}

	switch cfg.Output {
	case "file":
		if cfg.File != "" {
			file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				// Fall back to stderr.
				result.FallbackUsed = true
				result.FallbackReason = err.Error()
				result.Logger = newLoggerWithWriter(cfg, os.Stderr)
			} else {
				result.Logger = newLoggerWithWriter(cfg, file)
				result.FilePath = cfg.File
				result.UsingFile = true
				result.file = file // Store file handle for cleanup
			}
		} else {
			result.Logger = newLoggerWithWriter(cfg, os.Stderr)
		}
	case "stdout":
		result.Logger = newLoggerWithWriter(cfg, os.Stdout)
	default:
		result.Logger = newLoggerWithWriter(cfg, os.Stderr)
	}

	return result
}

// Close releases any resources held by the logger (e.g., file handles).
func (r *LogPathResult) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// NewLogger creates a logger configured according to cfg.
// The logger includes timestamps and trace ID injection, respects cfg.Level, cfg.Format, cfg.Caller and cfg.StackTrace, and writes to the destination specified by cfg.Output (file, stdout, or stderr). If a file destination is selected but cannot be opened, the logger falls back to stderr.
func NewLogger(cfg Config) zerolog.Logger {
	result := NewLoggerWithPath(cfg)
	return result.Logger
}

// NewLoggerWithWriter creates a logger writing to the specified writer.
// This is primarily used for testing to capture log output.
func NewLoggerWithWriter(cfg Config, writer io.Writer) zerolog.Logger {
	return newLoggerWithWriter(cfg, writer)
}

// newLoggerWithWriter is the internal implementation for creating loggers.
func newLoggerWithWriter(cfg Config, writer io.Writer) zerolog.Logger {
	// Apply format transformation
	output := writer
	if cfg.Format == "console" || cfg.Format == "text" {
		output = zerolog.ConsoleWriter{
			Out:        writer,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// Create base logger with timestamp
	logger := zerolog.New(output).
		With().
		Timestamp().
		Logger().
		Hook(TracingHook{}).
		Level(parseLevel(cfg.Level))

	// Add caller info if configured
	if cfg.Caller {
		logger = logger.With().Caller().Logger()
	}

	return logger
}

// createWriter selects an io.Writer based on the provided LoggingConfig.
// If cfg.Output is "stdout" it returns os.Stdout. If cfg.Output is "file" and
// cfg.File is a non-empty path it attempts to open (or create) the file for
// appending and returns the opened *os.File. If opening the file fails or
// cfg.File is empty, it falls back to os.Stderr and emits a warning to stderr.
// For any other cfg.Output value it returns os.Stderr.
func createWriter(cfg LoggingConfig) io.Writer {
	switch cfg.Output {
	case "stdout":
		return os.Stdout
	case "file":
		if cfg.File != "" {
			file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				// Fall back to stderr and log warning
				warnLogger := zerolog.New(os.Stderr)
				warnLogger.Warn().
					Err(err).
					Str("file", cfg.File).
					Msg("cannot open log file, falling back to stderr")
				return os.Stderr
			}
			return file
		}
		return os.Stderr
	default:
		// Default to stderr (keeps stdout clean for command output)
		return os.Stderr
	}
}

// parseLevel converts a string log level to zerolog.Level.
// parseLevel converts a case-insensitive level string into the corresponding zerolog.Level.
// It accepts "trace", "debug", "info" (or empty), "warn"/"warning", and "error".
// For any other value it emits a warning to stderr indicating the provided and fallback levels
// and returns zerolog.InfoLevel.
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info", "":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		// Log warning for invalid level and fall back to info
		warnLogger := zerolog.New(os.Stderr)
		warnLogger.Warn().
			Str("provided_level", level).
			Str("fallback_level", "info").
			Msg("invalid log level, falling back to info")
		return zerolog.InfoLevel
	}
}

// traceIDByteLength is the number of bytes for an OpenTelemetry trace ID (128-bit = 16 bytes).
const traceIDByteLength = 16

// GenerateTraceID creates a new OpenTelemetry-format trace identifier.
// This uses the pluginsdk's GenerateTraceID which produces a 32-character
// lowercase hexadecimal string, compatible with OpenTelemetry and W3C Trace Context.
func GenerateTraceID() string {
	traceID, err := pluginsdk.GenerateTraceID()
	if err != nil {
		// Fallback to simple hex if generation fails (extremely unlikely)
		b := make([]byte, traceIDByteLength)
		if _, readErr := rand.Read(b); readErr != nil {
			// Last resort fallback with timestamp, process ID, and counter for uniqueness
			return fmt.Sprintf("%016x%08x%08x", time.Now().UnixNano(), os.Getpid(), fallbackCounter.Add(1))
		}
		return hex.EncodeToString(b)
	}
	return traceID
}

// GetOrGenerateTraceID returns a trace ID from environment, context, or generates a new one.
// Priority: FINFOCUS_TRACE_ID env var > context > generate new.
func GetOrGenerateTraceID(ctx context.Context) string {
	// 1. Check environment variable (external injection) using pluginsdk constant
	if envTraceID := os.Getenv(pluginsdk.EnvTraceID); envTraceID != "" {
		return envTraceID
	}

	// 2. Check context
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		return traceID
	}

	// 3. Generate new
	return GenerateTraceID()
}

// ContextWithTraceID stores a trace ID in the context.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceIDFromContext extracts the trace ID from context.
// Returns empty string if no trace ID is stored.
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey{}).(string); ok {
		return traceID
	}
	return ""
}

// FromContext returns a logger from context, creating a default if none exists.
// FromContext returns a pointer to a zerolog.Logger associated with ctx, or a new default logger if none is present.
//
// If no logger exists in the context, a default logger is created that writes to stderr, includes timestamps, and has the TracingHook applied so trace IDs from context are injected into log events. The default log level is taken from the environment variable named by pluginsdk.EnvLogLevel when valid; otherwise the level defaults to info. This function never returns nil.
func FromContext(ctx context.Context) *zerolog.Logger {
	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() == zerolog.Disabled {
		// No logger in context, create a default one
		// Check environment variable for log level using pluginsdk constant
		level := zerolog.InfoLevel
		if envLevel := os.Getenv(pluginsdk.EnvLogLevel); envLevel != "" {
			if parsedLevel, err := zerolog.ParseLevel(envLevel); err == nil {
				level = parsedLevel
			} else {
				// Log invalid log level values at error level
				fmt.Fprintf(os.Stderr, "Invalid %s '%s': %v, using default info level\n", pluginsdk.EnvLogLevel, envLevel, err)
			}
		}
		defaultLogger := zerolog.New(os.Stderr).
			Level(level).
			With().
			Timestamp().
			Logger().
			Hook(TracingHook{})
		return &defaultLogger
	}
	return logger
}

// sensitivePatterns defines keys that should have their values redacted.
//
//nolint:gochecknoglobals // Package-level constant patterns for sensitive key detection
var sensitivePatterns = []string{
	"api_key", "apikey", "api-key",
	"password", "passwd", "pwd",
	"secret", "token",
	"credential", "cred",
	"private_key", "privatekey",
	"auth", "authorization", "bearer",
}

// isSensitiveKey checks if a key name contains sensitive patterns.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// SafeStr adds a string field to the event, redacting sensitive values.
// Use this when logging potentially sensitive key-value pairs.
func SafeStr(e *zerolog.Event, key, value string) *zerolog.Event {
	if isSensitiveKey(key) {
		return e.Str(key, "[REDACTED]")
	}
	return e.Str(key, value)
}

// ComponentLogger creates a logger derived from the provided logger with the
// `component` field set to the given component name.
// It returns the derived logger that will include `component` in all emitted entries.
func ComponentLogger(logger zerolog.Logger, component string) zerolog.Logger {
	return logger.With().Str("component", component).Logger()
}

// PrintLogPathMessage writes a "Logging to: <path>" message to the writer.
// PrintLogPathMessage writes "Logging to: <path>" followed by a newline to w.
// If path is empty, PrintLogPathMessage does nothing.
func PrintLogPathMessage(w io.Writer, path string) {
	if path == "" {
		return
	}
	_, _ = io.WriteString(w, "Logging to: "+path+"\n")
}

// PrintFallbackWarning writes a single-line warning to w indicating that logging
// has fallen back to stderr. If reason is non-empty it is appended in
// parentheses after the message. The function ignores any write error.
func PrintFallbackWarning(w io.Writer, reason string) {
	msg := "Warning: Could not write to log file, falling back to stderr"
	if reason != "" {
		msg += " (" + reason + ")"
	}
	_, _ = io.WriteString(w, msg+"\n")
}
