package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// CLIHelper provides utilities for testing CLI commands in integration tests.
type CLIHelper struct {
	t      *testing.T
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	cmd    *cobra.Command
}

// NewCLIHelper creates a new CLI test helper.
// It configures the environment to suppress log output to avoid polluting test output.
func NewCLIHelper(t *testing.T) *CLIHelper {
	t.Helper()

	// Set zerolog global level to disabled to suppress all logging during tests.
	// This prevents JSON parsing errors from log output mixing with command output.
	zerolog.SetGlobalLevel(zerolog.Disabled)

	// Restore global level after test completes
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	})

	return &CLIHelper{
		t:      t,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
}

// filterLogLines removes JSON log lines from output.
// Plugin processes may write logs to stderr which can get mixed with stdout in some test
// scenarios. This function filters out lines that look like zerolog JSON output.
func filterLogLines(output string) string {
	var filtered []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip zerolog JSON log lines (start with {"level":)
		if strings.HasPrefix(trimmed, "{\"level\":") {
			continue
		}
		// Skip plugin startup lines (e.g., "PORT=12345")
		if strings.HasPrefix(trimmed, "PORT=") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// Execute runs a CLI command with the given arguments.
// Returns the stdout output and any error.
// Note: Output is filtered to remove log lines from plugin processes that may
// write to stderr (which gets captured by the test runner).
func (h *CLIHelper) Execute(args ...string) (string, error) {
	h.t.Helper()

	// Reset buffers
	h.stdout.Reset()
	h.stderr.Reset()

	// Create root command
	h.cmd = cli.NewRootCmd("test-version")
	h.cmd.SetOut(h.stdout)
	h.cmd.SetErr(h.stderr)
	h.cmd.SetArgs(args)

	// Execute command
	err := h.cmd.Execute()

	// Filter log lines from output - plugin processes may write logs to stderr
	// which can appear in stdout when captured by the test framework
	output := filterLogLines(h.stdout.String())

	return output, err
}

// ExecuteOrFail runs a CLI command and fails the test if it returns an error.
func (h *CLIHelper) ExecuteOrFail(args ...string) string {
	h.t.Helper()

	output, err := h.Execute(args...)
	require.NoError(h.t, err, "Command failed: %v\nStderr: %s", err, h.stderr.String())

	return output
}

// ExecuteExpectError runs a CLI command expecting it to fail.
// Returns the error message.
func (h *CLIHelper) ExecuteExpectError(args ...string) string {
	h.t.Helper()

	_, err := h.Execute(args...)
	require.Error(h.t, err, "Expected command to fail but it succeeded")

	return err.Error()
}

// ExecuteJSON runs a CLI command with JSON output and unmarshals the result.
func (h *CLIHelper) ExecuteJSON(v interface{}, args ...string) error {
	h.t.Helper()

	output, err := h.Execute(args...)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(output), v)
}

// ExecuteJSONOrFail runs a CLI command with JSON output and fails if unmarshaling fails.
func (h *CLIHelper) ExecuteJSONOrFail(v interface{}, args ...string) {
	h.t.Helper()

	err := h.ExecuteJSON(v, args...)
	require.NoError(h.t, err, "Failed to parse JSON output")
}

// Stdout returns the captured stdout output.
func (h *CLIHelper) Stdout() string {
	return h.stdout.String()
}

// Stderr returns the captured stderr output.
func (h *CLIHelper) Stderr() string {
	return h.stderr.String()
}

// EnvSetup sets environment variables and returns a cleanup function.
type EnvSetup func() (cleanup func())

// WithEnv executes a command with temporary environment variables.
func (h *CLIHelper) WithEnv(env map[string]string, fn func()) {
	h.t.Helper()

	// Save original environment
	original := make(map[string]string)
	for key := range env {
		original[key] = os.Getenv(key)
	}

	// Set new environment
	for key, value := range env {
		_ = os.Setenv(key, value)
	}

	// Cleanup function to restore original environment
	h.t.Cleanup(func() {
		for key, value := range original {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
	})

	// Execute function
	fn()
}

// CreateTempFile creates a temporary file with the given content.
// The file is automatically cleaned up after the test.
func (h *CLIHelper) CreateTempFile(content string) string {
	h.t.Helper()

	tmpFile, err := os.CreateTemp("", "finfocus-test-*.json")
	require.NoError(h.t, err, "Failed to create temp file")

	_, err = io.WriteString(tmpFile, content)
	require.NoError(h.t, err, "Failed to write to temp file")

	err = tmpFile.Close()
	require.NoError(h.t, err, "Failed to close temp file")

	h.t.Cleanup(func() {
		_ = os.Remove(tmpFile.Name())
	})

	return tmpFile.Name()
}

// CreateTempDir creates a temporary directory.
// The directory is automatically cleaned up after the test.
func (h *CLIHelper) CreateTempDir() string {
	h.t.Helper()

	tmpDir, err := os.MkdirTemp("", "finfocus-test-*")
	require.NoError(h.t, err, "Failed to create temp directory")

	h.t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	return tmpDir
}

// AssertContains asserts that the output contains the expected substring.
func (h *CLIHelper) AssertContains(output, expected string) {
	h.t.Helper()

	require.Contains(h.t, output, expected, "Output should contain expected string")
}

// AssertNotContains asserts that the output does not contain the substring.
func (h *CLIHelper) AssertNotContains(output, unexpected string) {
	h.t.Helper()

	require.NotContains(h.t, output, unexpected, "Output should not contain string")
}

// AssertJSONField asserts that a JSON field has the expected value.
func (h *CLIHelper) AssertJSONField(output, field string, expected interface{}) {
	h.t.Helper()

	var result map[string]interface{}
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(h.t, err, "Failed to parse JSON output")

	require.Equal(h.t, expected, result[field], "JSON field %s should match expected value", field)
}
