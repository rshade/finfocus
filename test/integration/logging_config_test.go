package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/logging"
)

// T009: Integration test for config file loading and logging initialization.
func TestLoggingConfig_Integration(t *testing.T) {
	t.Run("config file logging settings create working logger", func(t *testing.T) {
		// Create a temporary log file
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		// Create logging config as if loaded from config file
		cfg := logging.Config{
			Level:  "debug",
			Format: "json",
			Output: "file",
			File:   logFile,
		}

		// Create logger with config
		logger := logging.NewLogger(cfg)

		// Log a test message
		logger.Debug().Msg("test integration message")

		// Verify log file was created and contains our message
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test integration message")
		assert.Contains(t, string(content), "debug")
	})

	t.Run("stderr fallback works when file cannot be created", func(t *testing.T) {
		// Use an invalid path that will fail
		cfg := logging.Config{
			Level:  "info",
			Format: "json",
			Output: "file",
			File:   "/nonexistent/deeply/nested/path/that/cannot/exist/log.log",
		}

		// Should not panic - falls back to stderr
		logger := logging.NewLogger(cfg)
		assert.NotNil(t, logger)

		// Logger should still work (writing to stderr)
		logger.Info().Msg("fallback test")
	})

	t.Run("trace ID propagation works with configured logger", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "trace.log")

		cfg := logging.Config{
			Level:  "debug",
			Format: "json",
			Output: "file",
			File:   logFile,
		}

		logger := logging.NewLogger(cfg)

		// Create context with trace ID
		ctx := logging.ContextWithTraceID(context.Background(), "test-trace-123")
		ctx = logger.WithContext(ctx)

		// Log with context
		logging.FromContext(ctx).Debug().Ctx(ctx).Msg("traced message")

		// Verify trace ID appears in log
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test-trace-123")
	})
}

// Test console format works.
func TestLoggingConfig_ConsoleFormat(t *testing.T) {
	cfg := logging.Config{
		Level:  "info",
		Format: "console",
		Output: "stderr",
	}

	// Should create logger with console writer
	logger := logging.NewLogger(cfg)
	assert.NotNil(t, logger)
}

// Test text format is alias for console.
func TestLoggingConfig_TextFormat(t *testing.T) {
	cfg := logging.Config{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	}

	// Should create logger with console writer (text is alias)
	logger := logging.NewLogger(cfg)
	assert.NotNil(t, logger)
}
