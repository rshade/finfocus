package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T004: Unit test for TracingHook trace ID injection.
func TestTracingHook_InjectsTraceID(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	// Create context with trace ID
	ctx := context.Background()
	ctx = ContextWithTraceID(ctx, "test-trace-id-123")

	// Create logger with hook
	hook := TracingHook{}
	loggerWithHook := logger.Hook(hook)

	// Log with context
	loggerWithHook.Info().Ctx(ctx).Msg("test message")

	// Verify trace_id is in output
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "test-trace-id-123", logEntry["trace_id"])
	assert.Equal(t, "test message", logEntry["message"])
}

func TestTracingHook_NoTraceIDWithoutContext(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	hook := TracingHook{}
	loggerWithHook := logger.Hook(hook)

	// Log without context (no trace ID should be added)
	loggerWithHook.Info().Msg("test message without trace")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	_, hasTraceID := logEntry["trace_id"]
	assert.False(t, hasTraceID, "trace_id should not be present without context")
}

// T005: Unit test for NewLogger factory function.
func TestNewLogger_DefaultsToJSON(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "info",
		Format: "json",
	}

	var buf bytes.Buffer
	logger := NewLoggerWithWriter(cfg, &buf)

	logger.Info().Msg("test json output")

	// Should be valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "test json output", logEntry["message"])
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "info",
		Format: "console",
	}

	var buf bytes.Buffer
	logger := NewLoggerWithWriter(cfg, &buf)

	logger.Info().Msg("test console output")

	// Console format should contain the message but not be JSON
	output := buf.String()
	assert.Contains(t, output, "test console output")

	// Should NOT be valid JSON (console format)
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.Error(t, err, "console format should not be valid JSON")
}

func TestNewLogger_LevelConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		configLevel   string
		logLevel      string
		shouldContain bool
	}{
		{"info level logs info", "info", "info", true},
		{"info level skips debug", "info", "debug", false},
		{"debug level logs debug", "debug", "debug", true},
		{"error level logs error", "error", "error", true},
		{"error level skips warn", "error", "warn", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoggingConfig{
				Level:  tt.configLevel,
				Format: "json",
			}

			var buf bytes.Buffer
			logger := NewLoggerWithWriter(cfg, &buf)

			switch tt.logLevel {
			case "debug":
				logger.Debug().Msg("test log")
			case "info":
				logger.Info().Msg("test log")
			case "warn":
				logger.Warn().Msg("test log")
			case "error":
				logger.Error().Msg("test log")
			}

			if tt.shouldContain {
				assert.Contains(t, buf.String(), "test log")
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}

// T006: Unit test for OpenTelemetry trace ID generation.
func TestGenerateTraceID_ReturnsValidOTelFormat(t *testing.T) {
	traceID := GenerateTraceID()

	// OpenTelemetry trace ID is 32 lowercase hex characters
	assert.Len(t, traceID, 32)

	// Should be lowercase hexadecimal (OpenTelemetry format)
	for _, c := range traceID {
		isValid := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		assert.True(t, isValid, "character %c is not valid hex", c)
	}
}

func TestGenerateTraceID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateTraceID()
		assert.False(t, ids[id], "duplicate trace ID generated")
		ids[id] = true
	}
}

// T007: Unit test for context helpers.
func TestContextWithTraceID_StoresAndRetrieves(t *testing.T) {
	ctx := context.Background()
	traceID := "test-trace-456"

	ctx = ContextWithTraceID(ctx, traceID)

	retrieved := TraceIDFromContext(ctx)
	assert.Equal(t, traceID, retrieved)
}

func TestTraceIDFromContext_ReturnsEmptyIfNotSet(t *testing.T) {
	ctx := context.Background()
	retrieved := TraceIDFromContext(ctx)
	assert.Empty(t, retrieved)
}

func TestFromContext_ReturnsLoggerWithTraceID(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := zerolog.New(&buf).Hook(TracingHook{})

	ctx := context.Background()
	ctx = ContextWithTraceID(ctx, "context-trace-789")
	ctx = baseLogger.WithContext(ctx)

	logger := FromContext(ctx)
	// Use Ctx to pass context to the event so TracingHook can extract trace_id
	logger.Info().Ctx(ctx).Msg("from context")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "context-trace-789", logEntry["trace_id"])
}

// T008: Unit test for log level parsing.
func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zerolog.Level
	}{
		{"trace", zerolog.TraceLevel},
		{"TRACE", zerolog.TraceLevel},
		{"debug", zerolog.DebugLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"INFO", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"WARN", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"ERROR", zerolog.ErrorLevel},
		{"invalid", zerolog.InfoLevel}, // defaults to info
		{"", zerolog.InfoLevel},        // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			assert.Equal(t, tt.expected, level)
		})
	}
}

// T009: Unit test for sensitive data protection patterns.
func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{
		"api_key",
		"apikey",
		"api-key",
		"password",
		"passwd",
		"pwd",
		"secret",
		"token",
		"credential",
		"cred",
		"private_key",
		"privatekey",
		"auth",
		"authorization",
		"bearer",
		"AWS_SECRET_ACCESS_KEY",
		"my_api_token",
		"user_password_hash",
	}

	for _, key := range sensitive {
		t.Run(key, func(t *testing.T) {
			assert.True(t, isSensitiveKey(key), "%s should be sensitive", key)
		})
	}

	notSensitive := []string{
		"user_id",
		"resource_name",
		"cost_monthly",
		"instance_type",
		"region",
		"provider",
	}

	for _, key := range notSensitive {
		t.Run(key, func(t *testing.T) {
			assert.False(t, isSensitiveKey(key), "%s should not be sensitive", key)
		})
	}
}

func TestSafeStr_RedactsSensitiveValues(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	event := logger.Info()
	event = SafeStr(event, "api_key", "super-secret-key")
	event.Msg("test")

	output := buf.String()
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "super-secret-key")
}

func TestSafeStr_AllowsNonSensitiveValues(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	event := logger.Info()
	event = SafeStr(event, "resource_name", "my-instance")
	event.Msg("test")

	output := buf.String()
	assert.Contains(t, output, "my-instance")
	assert.NotContains(t, output, "[REDACTED]")
}

// T056: Unit test for FINFOCUS_TRACE_ID environment variable override.
func TestGetOrGenerateTraceID_UsesEnvVar(t *testing.T) {
	// Set env var
	os.Setenv("FINFOCUS_TRACE_ID", "env-trace-id")
	defer os.Unsetenv("FINFOCUS_TRACE_ID")

	ctx := context.Background()
	traceID := GetOrGenerateTraceID(ctx)

	assert.Equal(t, "env-trace-id", traceID)
}

// T057: Unit test for external trace ID appearing in all log entries.
func TestExternalTraceID_AppearsInLogEntries(t *testing.T) {
	// Set external trace ID via environment
	os.Setenv("FINFOCUS_TRACE_ID", "external-trace-12345")
	defer os.Unsetenv("FINFOCUS_TRACE_ID")

	var buf bytes.Buffer
	cfg := LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger := NewLoggerWithWriter(cfg, &buf)

	// Get trace ID (should use env var)
	ctx := context.Background()
	traceID := GetOrGenerateTraceID(ctx)
	ctx = ContextWithTraceID(ctx, traceID)

	// Log multiple entries with context
	loggerWithHook := logger.Hook(TracingHook{})
	loggerWithHook.Info().Ctx(ctx).Msg("first message")
	loggerWithHook.Info().Ctx(ctx).Msg("second message")
	loggerWithHook.Warn().Ctx(ctx).Msg("third message")

	// Parse all log entries
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 3, "should have 3 log entries")

	for i, line := range lines {
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(line), &logEntry)
		require.NoError(t, err, "line %d should be valid JSON", i)

		// All entries should have the external trace ID
		assert.Equal(t, "external-trace-12345", logEntry["trace_id"],
			"line %d should have external trace ID", i)
	}
}

func TestGetOrGenerateTraceID_UsesContextIfNoEnv(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("FINFOCUS_TRACE_ID")

	ctx := ContextWithTraceID(context.Background(), "context-trace-id")
	traceID := GetOrGenerateTraceID(ctx)

	assert.Equal(t, "context-trace-id", traceID)
}

func TestGetOrGenerateTraceID_GeneratesNewIfNone(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("FINFOCUS_TRACE_ID")

	ctx := context.Background()
	traceID := GetOrGenerateTraceID(ctx)

	// Should generate a valid OpenTelemetry trace ID (32 hex chars)
	assert.Len(t, traceID, 32)
}

// Test for timestamp presence in logs.
func TestNewLogger_IncludesTimestamp(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "info",
		Format: "json",
	}

	var buf bytes.Buffer
	logger := NewLoggerWithWriter(cfg, &buf)

	logger.Info().Msg("test timestamp")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	_, hasTime := logEntry["time"]
	assert.True(t, hasTime, "log entry should have timestamp")
}

// Test default stderr output.
func TestCreateWriter_DefaultsToStderr(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stderr",
	}

	writer := createWriter(cfg)
	// We can't easily verify it's stderr, but we can verify it's not nil
	assert.NotNil(t, writer)
}

// Test component logger pattern.
func TestLogger_WithComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	componentLogger := logger.With().Str("component", "engine").Logger()
	componentLogger.Info().Msg("component test")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "engine", logEntry["component"])
}

// Test text format alias.
func TestNewLogger_TextFormatAlias(t *testing.T) {
	cfg := LoggingConfig{
		Level:  "info",
		Format: "text",
	}

	var buf bytes.Buffer
	logger := NewLoggerWithWriter(cfg, &buf)

	logger.Info().Msg("test text output")

	// Text format should be same as console (not JSON)
	output := buf.String()
	assert.Contains(t, output, "test text output")

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &logEntry)
	assert.Error(t, err, "text format should not be valid JSON")
}
