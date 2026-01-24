package pluginhost

import (
	"bytes"
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/rshade/finfocus/internal/logging"
)

// T036: Unit test for gRPC unary client interceptor.
func TestTraceInterceptor_ReturnsUnaryClientInterceptor(t *testing.T) {
	interceptor := TraceInterceptor()
	assert.NotNil(t, interceptor)
}

// T036: Test that interceptor is of correct type.
func TestTraceInterceptor_CorrectType(t *testing.T) {
	interceptor := TraceInterceptor()
	// Type assertion should succeed
	var _ grpc.UnaryClientInterceptor = interceptor
}

// T037: Unit test for trace ID metadata injection.
func TestTraceInterceptor_InjectsTraceIDMetadata(t *testing.T) {
	interceptor := TraceInterceptor()

	// Create context with trace ID
	ctx := context.Background()
	traceID := "test-trace-id-12345"
	ctx = logging.ContextWithTraceID(ctx, traceID)

	// Create a mock invoker that captures the context using a channel
	capturedCtxChan := make(chan context.Context, 1)
	mockInvoker := func(invokerCtx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtxChan <- invokerCtx
		return nil
	}

	// Call interceptor
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	require.NoError(t, err)

	// Retrieve captured context
	capturedCtx := <-capturedCtxChan

	// Verify metadata was injected
	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok, "outgoing metadata should exist")

	values := md.Get(TraceIDMetadataKey)
	require.Len(t, values, 1, "should have exactly one trace ID value")
	assert.Equal(t, traceID, values[0])
}

// T037: Test that interceptor handles missing trace ID gracefully.
func TestTraceInterceptor_NoTraceIDNoMetadata(t *testing.T) {
	interceptor := TraceInterceptor()

	// Create context without trace ID
	ctx := context.Background()

	// Create a mock invoker that captures the context using a channel
	capturedCtxChan := make(chan context.Context, 1)
	mockInvoker := func(invokerCtx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtxChan <- invokerCtx
		return nil
	}

	// Call interceptor
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	require.NoError(t, err)

	// Retrieve captured context
	capturedCtx := <-capturedCtxChan

	// Verify no trace ID metadata was added (but outgoing metadata might exist)
	md, ok := metadata.FromOutgoingContext(capturedCtx)
	if ok {
		values := md.Get(TraceIDMetadataKey)
		assert.Empty(t, values, "should not have trace ID metadata when not set in context")
	}
	// If no metadata at all, that's also fine
}

// T037: Test that interceptor preserves existing metadata.
func TestTraceInterceptor_PreservesExistingMetadata(t *testing.T) {
	interceptor := TraceInterceptor()

	// Create context with existing metadata and trace ID
	ctx := context.Background()
	traceID := "test-trace-id-67890"
	ctx = logging.ContextWithTraceID(ctx, traceID)
	ctx = metadata.AppendToOutgoingContext(ctx, "existing-key", "existing-value")

	// Create a mock invoker that captures the context using a channel
	capturedCtxChan := make(chan context.Context, 1)
	mockInvoker := func(invokerCtx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtxChan <- invokerCtx
		return nil
	}

	// Call interceptor
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	require.NoError(t, err)

	// Retrieve captured context
	capturedCtx := <-capturedCtxChan

	// Verify both existing and trace ID metadata are present
	md, ok := metadata.FromOutgoingContext(capturedCtx)
	require.True(t, ok, "outgoing metadata should exist")

	existingValues := md.Get("existing-key")
	require.Len(t, existingValues, 1)
	assert.Equal(t, "existing-value", existingValues[0])

	traceValues := md.Get(TraceIDMetadataKey)
	require.Len(t, traceValues, 1)
	assert.Equal(t, traceID, traceValues[0])
}

// T037: Test that interceptor propagates invoker errors.
func TestTraceInterceptor_PropagatesInvokerError(t *testing.T) {
	interceptor := TraceInterceptor()

	ctx := context.Background()
	ctx = logging.ContextWithTraceID(ctx, "test-trace-id")

	// Create a mock invoker that returns an error
	expectedErr := assert.AnError
	mockInvoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return expectedErr
	}

	// Call interceptor
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	assert.ErrorIs(t, err, expectedErr)
}

// Test the TraceIDMetadataKey constant value.
func TestTraceIDMetadataKey_Value(t *testing.T) {
	assert.Equal(t, "x-finfocus-trace-id", TraceIDMetadataKey)
}

func TestLoggedInterceptor(t *testing.T) {
	// Capture logs
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	interceptor := LoggedInterceptor(logger)

	ctx := context.Background()
	method := "/test.Service/Method"

	// Success case
	mockInvokerSuccess := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	err := interceptor(ctx, method, nil, nil, nil, mockInvokerSuccess)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "making gRPC call to plugin")

	// Error case
	buf.Reset()
	mockInvokerError := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return assert.AnError
	}

	err = interceptor(ctx, method, nil, nil, nil, mockInvokerError)
	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, buf.String(), "gRPC call failed")
}
