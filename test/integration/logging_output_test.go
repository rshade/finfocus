package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/logging"
)

// T017: Integration test for file logging with path output.
func TestFileLogging_PathOutput(t *testing.T) {
	t.Run("creates log file and returns path info", func(t *testing.T) {
		tempDir := t.TempDir()
		logFile := filepath.Join(tempDir, "test.log")

		cfg := logging.Config{
			Level:  "info",
			Format: "json",
			Output: "file",
			File:   logFile,
		}

		result := logging.NewLoggerWithPath(cfg)

		assert.NotNil(t, result.Logger)
		assert.Equal(t, logFile, result.FilePath)
		assert.True(t, result.UsingFile)
		assert.False(t, result.FallbackUsed)
		assert.Empty(t, result.FallbackReason)
	})

	t.Run("returns fallback info when file cannot be created", func(t *testing.T) {
		cfg := logging.Config{
			Level:  "info",
			Format: "json",
			Output: "file",
			File:   "/nonexistent/path/that/cannot/exist/log.log",
		}

		result := logging.NewLoggerWithPath(cfg)

		assert.NotNil(t, result.Logger)
		assert.Empty(t, result.FilePath)
		assert.False(t, result.UsingFile)
		assert.True(t, result.FallbackUsed)
		assert.NotEmpty(t, result.FallbackReason)
	})

	t.Run("no path info when output is stderr", func(t *testing.T) {
		cfg := logging.Config{
			Level:  "info",
			Format: "json",
			Output: "stderr",
		}

		result := logging.NewLoggerWithPath(cfg)

		assert.NotNil(t, result.Logger)
		assert.Empty(t, result.FilePath)
		assert.False(t, result.UsingFile)
		assert.False(t, result.FallbackUsed)
	})
}

// T015/T016: Test log path display message formatting.
func TestLogPathDisplay(t *testing.T) {
	t.Run("formats logging to file message correctly", func(t *testing.T) {
		var buf bytes.Buffer
		logPath := "/var/log/finfocus.log"

		logging.PrintLogPathMessage(&buf, logPath)

		output := buf.String()
		assert.Contains(t, output, "Logging to:")
		assert.Contains(t, output, logPath)
	})

	t.Run("formats fallback warning message correctly", func(t *testing.T) {
		var buf bytes.Buffer
		reason := "permission denied"

		logging.PrintFallbackWarning(&buf, reason)

		output := buf.String()
		assert.Contains(t, output, "Warning")
		assert.Contains(t, output, "stderr")
		assert.Contains(t, output, reason)
	})

	t.Run("no message when not using file", func(t *testing.T) {
		var buf bytes.Buffer

		// Empty path means not using file
		logging.PrintLogPathMessage(&buf, "")

		assert.Empty(t, buf.String())
	})
}

// Test that file logging actually writes to the file.
func TestFileLogging_WritesToFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "write-test.log")

	cfg := logging.Config{
		Level:  "info",
		Format: "json",
		Output: "file",
		File:   logFile,
	}

	result := logging.NewLoggerWithPath(cfg)
	require.True(t, result.UsingFile)

	// Write a log message
	result.Logger.Info().Msg("test message for file")

	// Verify it was written
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(content), "test message for file"))
}
