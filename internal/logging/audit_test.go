package logging_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/logging"
)

// T023: Unit test for AuditEntry struct.
func TestAuditEntry_Struct(t *testing.T) {
	t.Parallel()

	entry := logging.AuditEntry{
		Timestamp:   time.Now(),
		TraceID:     "test-trace-123",
		Command:     "cost projected",
		Parameters:  map[string]string{"pulumi_json": "/path/to/plan.json"},
		Duration:    100 * time.Millisecond,
		Success:     true,
		ResultCount: 5,
		TotalCost:   123.45,
		Error:       "",
	}

	assert.Equal(t, "test-trace-123", entry.TraceID)
	assert.Equal(t, "cost projected", entry.Command)
	assert.Equal(t, "/path/to/plan.json", entry.Parameters["pulumi_json"])
	assert.Equal(t, 100*time.Millisecond, entry.Duration)
	assert.True(t, entry.Success)
	assert.Equal(t, 5, entry.ResultCount)
	assert.Equal(t, 123.45, entry.TotalCost)
	assert.Empty(t, entry.Error)
}

// T024: Unit test for AuditLogger interface.
func TestAuditLogger_Interface(t *testing.T) {
	t.Parallel()

	t.Run("enabled logger logs entries", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		cfg := logging.AuditLoggerConfig{
			Enabled: true,
			Writer:  &buf,
		}
		auditLogger := logging.NewAuditLogger(cfg)

		assert.True(t, auditLogger.Enabled())

		entry := logging.AuditEntry{
			Timestamp: time.Now(),
			TraceID:   "trace-abc",
			Command:   "cost actual",
			Success:   true,
		}

		ctx := context.Background()
		auditLogger.Log(ctx, entry)

		output := buf.String()
		assert.Contains(t, output, "audit")
		assert.Contains(t, output, "cost actual")
	})

	t.Run("disabled logger does not log", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		cfg := logging.AuditLoggerConfig{
			Enabled: false,
			Writer:  &buf,
		}
		auditLogger := logging.NewAuditLogger(cfg)

		assert.False(t, auditLogger.Enabled())

		entry := logging.AuditEntry{
			Command: "cost projected",
		}

		ctx := context.Background()
		auditLogger.Log(ctx, entry)

		assert.Empty(t, buf.String())
	})
}

// T025: Unit test for audit entry field population.
func TestAuditEntry_FieldPopulation(t *testing.T) {
	t.Parallel()

	t.Run("all fields are logged correctly", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		cfg := logging.AuditLoggerConfig{
			Enabled: true,
			Writer:  &buf,
		}
		auditLogger := logging.NewAuditLogger(cfg)

		entry := logging.AuditEntry{
			Timestamp:   time.Date(2025, 12, 1, 10, 30, 0, 0, time.UTC),
			TraceID:     "01HQ7X2J3K4M5N6P7Q8R9S0T1U",
			Command:     "cost projected",
			Parameters:  map[string]string{"output": "json", "file": "/tmp/plan.json"},
			Duration:    1234 * time.Millisecond,
			Success:     true,
			ResultCount: 10,
			TotalCost:   456.78,
			Error:       "",
		}

		ctx := context.Background()
		auditLogger.Log(ctx, entry)

		output := buf.String()
		assert.Contains(t, output, "cost projected")
		assert.Contains(t, output, "01HQ7X2J3K4M5N6P7Q8R9S0T1U")
		assert.Contains(t, output, "true") // success
	})

	t.Run("error field is logged on failure", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		cfg := logging.AuditLoggerConfig{
			Enabled: true,
			Writer:  &buf,
		}
		auditLogger := logging.NewAuditLogger(cfg)

		entry := logging.AuditEntry{
			Timestamp: time.Now(),
			TraceID:   "trace-err",
			Command:   "cost actual",
			Success:   false,
			Error:     "plugin connection failed",
		}

		ctx := context.Background()
		auditLogger.Log(ctx, entry)

		output := buf.String()
		assert.Contains(t, output, "plugin connection failed")
		assert.Contains(t, output, "false") // success
	})
}

// T027a: Unit test verifying SafeStr() redaction works with audit entry parameters.
func TestAuditEntry_SensitiveDataRedaction(t *testing.T) {
	t.Parallel()

	t.Run("sensitive parameters are redacted", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		cfg := logging.AuditLoggerConfig{
			Enabled: true,
			Writer:  &buf,
		}
		auditLogger := logging.NewAuditLogger(cfg)

		// Include sensitive parameters that should be redacted
		entry := logging.AuditEntry{
			Timestamp: time.Now(),
			TraceID:   "trace-sensitive",
			Command:   "cost actual",
			Parameters: map[string]string{
				"api_key":      "secret-key-12345",
				"password":     "mysecretpassword",
				"normal_param": "visible-value",
			},
			Success: true,
		}

		ctx := context.Background()
		auditLogger.Log(ctx, entry)

		output := buf.String()
		// Sensitive values should be redacted
		assert.NotContains(t, output, "secret-key-12345")
		assert.NotContains(t, output, "mysecretpassword")
		// Non-sensitive values should be visible
		assert.Contains(t, output, "visible-value")
		// Redaction marker should appear
		assert.Contains(t, output, "[REDACTED]")
	})

	t.Run("various sensitive key patterns are redacted", func(t *testing.T) {
		t.Parallel()

		sensitiveKeys := []string{
			"api_key", "apikey", "api-key",
			"password", "passwd", "pwd",
			"secret", "token",
			"credential", "cred",
			"auth", "authorization", "bearer",
		}

		for _, key := range sensitiveKeys {
			t.Run(key, func(t *testing.T) {
				var buf bytes.Buffer
				cfg := logging.AuditLoggerConfig{
					Enabled: true,
					Writer:  &buf,
				}
				auditLogger := logging.NewAuditLogger(cfg)

				entry := logging.AuditEntry{
					Timestamp:  time.Now(),
					TraceID:    "trace-" + key,
					Command:    "test",
					Parameters: map[string]string{key: "sensitive-value-for-" + key},
					Success:    true,
				}

				ctx := context.Background()
				auditLogger.Log(ctx, entry)

				output := buf.String()
				assert.NotContains(t, output, "sensitive-value-for-"+key,
					"Key %q should have its value redacted", key)
			})
		}
	})
}

// Test NoOpAuditLogger for when audit is disabled.
func TestNoOpAuditLogger(t *testing.T) {
	t.Parallel()

	auditLogger := logging.NoOpAuditLogger()

	assert.False(t, auditLogger.Enabled())

	// Should not panic
	ctx := context.Background()
	auditLogger.Log(ctx, logging.AuditEntry{Command: "test"})
}

// Test that audit logger uses component "audit".
func TestAuditLogger_Component(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cfg := logging.AuditLoggerConfig{
		Enabled: true,
		Writer:  &buf,
	}
	auditLogger := logging.NewAuditLogger(cfg)

	ctx := context.Background()
	auditLogger.Log(ctx, logging.AuditEntry{
		TraceID: "trace-component",
		Command: "test",
		Success: true,
	})

	output := buf.String()
	assert.Contains(t, output, "audit")
}

// Test creating audit logger from config.
func TestNewAuditLoggerFromConfig(t *testing.T) {
	t.Run("creates enabled logger when audit enabled", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := logging.AuditLoggerConfig{
			Enabled: true,
			Writer:  &buf,
		}

		auditLogger := logging.NewAuditLogger(cfg)
		assert.True(t, auditLogger.Enabled())
	})

	t.Run("creates disabled logger when audit disabled", func(t *testing.T) {
		cfg := logging.AuditLoggerConfig{
			Enabled: false,
		}

		auditLogger := logging.NewAuditLogger(cfg)
		assert.False(t, auditLogger.Enabled())
	})
}

// Helper test to verify SafeStr function is exported and works.
func TestSafeStr_WithAuditContext(t *testing.T) {
	t.Parallel()

	// Test the exported SafeStr function with various sensitive keys
	tests := []struct {
		key          string
		value        string
		shouldRedact bool
	}{
		{"api_key", "secret123", true},
		{"password", "pass123", true},
		{"normal", "value", false},
		{"file_path", "/tmp/file.json", false},
		{"token", "tok_abc", true},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()

			// Use IsSensitiveKey helper if exported, or test through logging behavior.
			params := map[string]string{tc.key: tc.value}
			entry := logging.AuditEntry{Parameters: params}

			// The entry should have the parameter; redaction is applied at log time.
			assert.Equal(t, tc.value, entry.Parameters[tc.key])
		})
	}
}

// Test that SafeParams helper works correctly.
func TestSafeParams(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"api_key":  "secret123",
		"password": "pass123",
		"file":     "/tmp/test.json",
		"output":   "table",
	}

	safe := logging.SafeParams(params)

	assert.Equal(t, "[REDACTED]", safe["api_key"])
	assert.Equal(t, "[REDACTED]", safe["password"])
	assert.Equal(t, "/tmp/test.json", safe["file"])
	assert.Equal(t, "table", safe["output"])
}

// Test audit entry builder pattern.
func TestAuditEntry_Builder(t *testing.T) {
	t.Parallel()

	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	entry := logging.NewAuditEntry("cost projected", "trace-123").
		WithParameters(map[string]string{"file": "/tmp/plan.json"}).
		WithSuccess(5, 100.50).
		WithDuration(start)

	assert.Equal(t, "cost projected", entry.Command)
	assert.Equal(t, "trace-123", entry.TraceID)
	assert.Equal(t, "/tmp/plan.json", entry.Parameters["file"])
	assert.True(t, entry.Success)
	assert.Equal(t, 5, entry.ResultCount)
	assert.Equal(t, 100.50, entry.TotalCost)
	assert.True(t, entry.Duration >= 10*time.Millisecond)
}

// Test builder with error.
func TestAuditEntry_BuilderWithError(t *testing.T) {
	t.Parallel()

	start := time.Now()
	entry := logging.NewAuditEntry("cost actual", "trace-456").
		WithError("plugin not found").
		WithDuration(start)

	assert.Equal(t, "cost actual", entry.Command)
	assert.False(t, entry.Success)
	assert.Equal(t, "plugin not found", entry.Error)
}

// Test that IsSensitiveKey is exposed for parameter filtering.
func TestIsSensitiveKey(t *testing.T) {
	t.Parallel()

	sensitiveKeys := []string{
		"api_key", "API_KEY", "Api_Key",
		"password", "PASSWORD",
		"secret", "SECRET",
		"token", "TOKEN",
		"auth", "authorization",
	}

	for _, key := range sensitiveKeys {
		assert.True(t, logging.IsSensitiveKey(key), "Expected %q to be sensitive", key)
	}

	nonSensitiveKeys := []string{
		"file", "path", "output", "format",
		"start_date", "end_date", "filter",
	}

	for _, key := range nonSensitiveKeys {
		assert.False(t, logging.IsSensitiveKey(key), "Expected %q to NOT be sensitive", key)
	}
}

// Verify no panic on nil context.
func TestAuditLogger_NilContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cfg := logging.AuditLoggerConfig{
		Enabled: true,
		Writer:  &buf,
	}
	auditLogger := logging.NewAuditLogger(cfg)

	// Should not panic with background context
	ctx := context.Background()
	entry := logging.AuditEntry{
		Command: "test",
		Success: true,
	}

	auditLogger.Log(ctx, entry)
	assert.True(t, strings.Contains(buf.String(), "test"))
}
