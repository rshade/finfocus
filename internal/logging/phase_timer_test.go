package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartPhase_ReturnsNonZeroStart(t *testing.T) {
	ctx := context.Background()
	before := time.Now()
	pt := StartPhase(ctx, "cli", "overview", "test_phase")
	after := time.Now()

	elapsed := pt.Elapsed()
	assert.GreaterOrEqual(t, elapsed.Nanoseconds(), int64(0))
	assert.LessOrEqual(t, pt.start.UnixNano(), after.UnixNano())
	assert.GreaterOrEqual(t, pt.start.UnixNano(), before.UnixNano())
}

func TestPhaseTimer_Done_DoesNotPanic(t *testing.T) {
	ctx := context.Background()
	pt := StartPhase(ctx, "cli", "overview", "safe_phase")

	// Should not panic
	assert.NotPanics(t, func() {
		pt.Done("safe_phase")
	})
}

func TestPhaseTimer_Elapsed_Positive(t *testing.T) {
	ctx := context.Background()
	pt := StartPhase(ctx, "engine", "enrich", "test_elapsed")

	time.Sleep(time.Millisecond)
	elapsed := pt.Elapsed()

	assert.Greater(t, elapsed.Nanoseconds(), int64(0))
}

func TestPhaseTimer_LogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	pt := StartPhase(ctx, "cli", "overview", "data_loading")
	time.Sleep(time.Millisecond)
	pt.Done("data_loading")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2, "should have start and done log entries")

	// Verify start log entry
	var startEntry map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &startEntry))
	assert.Equal(t, "cli", startEntry["component"])
	assert.Equal(t, "overview", startEntry["operation"])
	assert.Equal(t, "data_loading", startEntry["phase"])
	assert.Equal(t, "debug", startEntry["level"])
	assert.Equal(t, "overview phase starting", startEntry["message"])

	// Verify done log entry
	var doneEntry map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &doneEntry))
	assert.Equal(t, "cli", doneEntry["component"])
	assert.Equal(t, "overview", doneEntry["operation"])
	assert.Equal(t, "data_loading", doneEntry["phase"])
	assert.Equal(t, "info", doneEntry["level"])
	assert.Equal(t, "overview phase complete", doneEntry["message"])

	// elapsed_ms should be present and non-negative
	elapsedMS, ok := doneEntry["elapsed_ms"].(float64)
	require.True(t, ok, "elapsed_ms should be a number")
	assert.GreaterOrEqual(t, elapsedMS, float64(0))
}

func TestPhaseTimer_LogFields_DifferentComponents(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	pt := StartPhase(ctx, "pulumi", "stack_export", "export")
	pt.Done("export")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)

	var doneEntry map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &doneEntry))
	assert.Equal(t, "pulumi", doneEntry["component"])
	assert.Equal(t, "stack_export", doneEntry["operation"])
	assert.Equal(t, "export", doneEntry["phase"])
	assert.Equal(t, "stack_export phase complete", doneEntry["message"])
}

func TestPhaseTimer_WithTraceID(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel).Hook(TracingHook{})
	ctx := logger.WithContext(context.Background())
	ctx = ContextWithTraceID(ctx, "test-trace-phase")

	pt := StartPhase(ctx, "cli", "overview", "traced")
	pt.Done("traced")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)

	for i, line := range lines {
		var entry map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		assert.Equal(t, "test-trace-phase", entry["trace_id"], "line %d should have trace ID", i)
	}
}
