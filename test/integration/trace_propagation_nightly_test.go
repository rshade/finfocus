//go:build nightly
// +build nightly

// Nightly Justification:
//
// These tests MUST remain in the nightly build tag because they:
// (1) Build the finfocus CLI binary via "go build", which requires the full Go
//     toolchain and takes 10-30 seconds per binary compilation.
// (2) Spawn subprocesses via exec.Command that exercise the built binary, adding
//     process management overhead and potential for flakiness on resource-constrained
//     CI runners.
// (3) External dependencies: Go toolchain for binary build, filesystem for binary
//     output, environment variable injection for subprocess control.
// (4) Approximate execution time: 10-30s per test due to binary compilation.
//
// The context-only tests (ContextHelpers, GetOrGenerateFromContext,
// GeneratesNewTraceID, ExternalTraceIDPrecedence) have been promoted to the
// "integration" build tag in trace_propagation_test.go since they only call
// Go functions with zero external dependencies and complete in milliseconds.

package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTracePropagation_TraceIDInDebugOutput verifies that trace IDs appear
// in debug output when the CLI binary is run with --debug flag.
func TestTracePropagation_TraceIDInDebugOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build the CLI binary
	cmd := exec.Command("go", "build", "-o", "../../bin/finfocus-test", "../../cmd/finfocus")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("failed to build CLI: %v\n%s", err, output)
	}

	// Run with debug flag to capture trace ID in output
	cmd = exec.Command("../../bin/finfocus-test", "cost", "projected", "--debug",
		"--pulumi-json", "../../examples/plans/aws-simple-plan.json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // Don't check error as cost calculation may fail without plugins

	// Check that trace ID appears in debug output
	combined := stdout.String() + stderr.String()

	// Trace ID should be in OpenTelemetry format (32 hex characters)
	assert.Contains(t, combined, "trace_id", "debug output should contain trace_id field")
}

// TestTracePropagation_ConsistentTraceID verifies that trace IDs are
// consistent across log entries when the CLI binary is run.
func TestTracePropagation_ConsistentTraceID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create isolated HOME directory to ensure no plugins are found
	// Plugins have their own trace ID generation which would cause mismatches
	tempHome := t.TempDir()

	// Build the CLI binary
	cmd := exec.Command("go", "build", "-o", "../../bin/finfocus-test", "../../cmd/finfocus")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("failed to build CLI: %v\n%s", err, output)
	}

	// Run with debug flag and force JSON format for parseable output
	cmd = exec.Command("../../bin/finfocus-test", "cost", "projected", "--debug",
		"--pulumi-json", "../../examples/plans/aws-simple-plan.json")
	cmd.Env = append(os.Environ(), "FINFOCUS_LOG_FORMAT=json", "HOME="+tempHome)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()

	// Parse JSON log lines from stderr (logs go to stderr)
	var traceIDs []string
	for _, line := range bytes.Split(stderr.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal(line, &logEntry); err != nil {
			continue // Skip non-JSON lines (console format)
		}
		if traceID, ok := logEntry["trace_id"].(string); ok {
			traceIDs = append(traceIDs, traceID)
		}
	}

	// If we have trace IDs, they should all be the same
	if len(traceIDs) > 0 {
		firstTraceID := traceIDs[0]
		for i, tid := range traceIDs {
			assert.Equal(t, firstTraceID, tid, "trace ID at position %d should match first trace ID", i)
		}
	}
}

// TestTracePropagation_ExternalTraceIDFlow validates that an external trace ID
// set via environment variable flows through the CLI binary to log output.
func TestTracePropagation_ExternalTraceIDFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build the CLI binary
	cmd := exec.Command("go", "build", "-o", "../../bin/finfocus-test", "../../cmd/finfocus")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("failed to build CLI: %v\n%s", err, output)
	}

	// Set external trace ID via environment
	externalTraceID := "external-pipeline-trace-abc123"

	// Run with external trace ID and debug mode
	// Force JSON format via env var (--debug sets console format, we override with env)
	cmd = exec.Command("../../bin/finfocus-test", "cost", "projected", "--debug",
		"--pulumi-json", "../../examples/plans/aws-simple-plan.json")
	cmd.Env = append(os.Environ(),
		"FINFOCUS_TRACE_ID="+externalTraceID,
		"FINFOCUS_LOG_FORMAT=json", // Force JSON format for parsing
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()

	// Parse JSON log lines from stderr
	var foundExternalTraceID bool
	for _, line := range bytes.Split(stderr.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal(line, &logEntry); err != nil {
			continue // Skip non-JSON lines
		}
		if traceID, ok := logEntry["trace_id"].(string); ok {
			if traceID == externalTraceID {
				foundExternalTraceID = true
				break
			}
		}
	}

	assert.True(t, foundExternalTraceID, "external trace ID should appear in log output")
}
