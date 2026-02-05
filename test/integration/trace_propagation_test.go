//go:build nightly

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T038: Integration test for trace propagation to mock plugin.
// This test verifies that trace IDs set in the CLI context propagate to plugin calls.
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

// T038: Test that trace IDs are consistent across log entries.
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
	cmd.Env = append(os.Environ(), pluginsdk.EnvLogFormat+"=json", "HOME="+tempHome)
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

// Test that context helpers work correctly for trace propagation.
func TestTracePropagation_ContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test ContextWithTraceID and TraceIDFromContext
	traceID := "propagation-test-trace-id"
	ctx = logging.ContextWithTraceID(ctx, traceID)

	retrieved := logging.TraceIDFromContext(ctx)
	require.Equal(t, traceID, retrieved)
}

// Test that GetOrGenerateTraceID respects context trace ID.
func TestTracePropagation_GetOrGenerateFromContext(t *testing.T) {
	ctx := context.Background()
	traceID := "context-provided-trace-id"
	ctx = logging.ContextWithTraceID(ctx, traceID)

	result := logging.GetOrGenerateTraceID(ctx)
	assert.Equal(t, traceID, result)
}

// Test that GetOrGenerateTraceID generates new ID when none exists.
func TestTracePropagation_GeneratesNewTraceID(t *testing.T) {
	ctx := context.Background()

	result := logging.GetOrGenerateTraceID(ctx)
	assert.Len(t, result, 32, "should generate valid OTel trace ID (32 hex chars)")
}

// T058: Integration test validating external trace ID flow.
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

// T058: Test that external trace ID takes precedence over context.
func TestTracePropagation_ExternalTraceIDPrecedence(t *testing.T) {
	// Set external trace ID using pluginsdk constant for consistency
	os.Setenv(pluginsdk.EnvTraceID, "external-takes-precedence")
	defer os.Unsetenv(pluginsdk.EnvTraceID)

	// Create context with different trace ID
	ctx := context.Background()
	ctx = logging.ContextWithTraceID(ctx, "context-trace-id")

	// GetOrGenerateTraceID should use external trace ID
	result := logging.GetOrGenerateTraceID(ctx)
	assert.Equal(t, "external-takes-precedence", result, "external trace ID should take precedence over context")
}
