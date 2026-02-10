package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T058: Unit test for createWriter file creation.
func TestCreateWriter_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := LoggingConfig{
		Output: "file",
		File:   logFile,
	}

	writer := createWriter(cfg)
	require.NotNil(t, writer)

	// Verify it's a file
	file, ok := writer.(*os.File)
	require.True(t, ok, "createWriter should return *os.File")
	assert.Equal(t, logFile, file.Name())

	// Verify we can write to it
	_, err := file.WriteString("test log content\n")
	require.NoError(t, err)
	_ = file.Close()

	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "test log content\n", string(content))
}

// T059: Unit test for createWriter fallback to stderr on permission error.
func TestCreateWriter_FallbackOnPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - POSIX permissions not enforced")
	}

	// Create a directory and make it read-only to force permission error
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0500) // Read/Execute only
	require.NoError(t, err)

	logFile := filepath.Join(readOnlyDir, "test.log")

	cfg := LoggingConfig{
		Output: "file",
		File:   logFile,
	}

	// Capture stderr to verify warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w //nolint:reassign // intentional: capturing stderr for test verification

	defer func() {
		os.Stderr = oldStderr //nolint:reassign // intentional: restoring stderr after test
		_ = r.Close()
	}()

	writer := createWriter(cfg)

	// Close write end of pipe to read output
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	stderrOutput := buf.String()

	// Verify fallback
	assert.Equal(t, os.Stderr, writer, "should fallback to stderr")
	assert.Contains(t, stderrOutput, "cannot open log file, falling back to stderr")
	assert.Contains(t, stderrOutput, "permission denied")
}

// T060: Unit test for NewLoggerWithPath file creation.
func TestNewLoggerWithPath_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := LoggingConfig{
		Output: "file",
		File:   logFile,
		Level:  "info",
		Format: "json",
	}

	result := NewLoggerWithPath(cfg)
	defer result.Close() // Release file handle (prevents Windows file locking)

	assert.True(t, result.UsingFile)
	assert.Equal(t, logFile, result.FilePath)
	assert.False(t, result.FallbackUsed)

	// Verify logging works
	result.Logger.Info().Msg("test message")

	// Hack: wait a tiny bit for buffer flush or ensure logger writes immediately?
	// zerolog writes immediately to file.

	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message")
}

// T061: Unit test for NewLoggerWithPath fallback.
func TestNewLoggerWithPath_Fallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - POSIX permissions not enforced")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0500)
	require.NoError(t, err)

	logFile := filepath.Join(readOnlyDir, "test.log")

	cfg := LoggingConfig{
		Output: "file",
		File:   logFile,
	}

	result := NewLoggerWithPath(cfg)

	assert.False(t, result.UsingFile)
	assert.Empty(t, result.FilePath)
	assert.True(t, result.FallbackUsed)
	assert.Contains(t, result.FallbackReason, "permission denied")
}

// T062: Unit test for LogPathResult properties.
func TestLogPathResult_Stdout(t *testing.T) {
	cfg := LoggingConfig{
		Output: "stdout",
	}

	result := NewLoggerWithPath(cfg)

	assert.False(t, result.UsingFile)
	assert.Empty(t, result.FilePath)
	assert.False(t, result.FallbackUsed)
}

// T063: Unit test for PrintLogPathMessage.
func TestPrintLogPathMessage(t *testing.T) {
	var buf bytes.Buffer
	PrintLogPathMessage(&buf, "/test/path.log")
	assert.Equal(t, "Logging to: /test/path.log\n", buf.String())

	buf.Reset()
	PrintLogPathMessage(&buf, "")
	assert.Empty(t, buf.String())
}

// T064: Unit test for PrintFallbackWarning.
func TestPrintFallbackWarning(t *testing.T) {
	var buf bytes.Buffer
	PrintFallbackWarning(&buf, "permission denied")
	assert.Equal(
		t,
		"Warning: Could not write to log file, falling back to stderr (permission denied)\n",
		buf.String(),
	)

	buf.Reset()
	PrintFallbackWarning(&buf, "")
	assert.Equal(t, "Warning: Could not write to log file, falling back to stderr\n", buf.String())
}
