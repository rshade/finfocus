//go:build integration
// +build integration

package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// Test that external trace ID takes precedence over context.
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
