package pluginhost_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// T036: Unit test for gRPC unary client interceptor.
func TestTraceInterceptor_ReturnsUnaryClientInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := pluginhost.TraceInterceptor()
	assert.NotNil(t, interceptor)
}

// T036: Test that interceptor is of correct type.
func TestTraceInterceptor_CorrectType(t *testing.T) {
	t.Parallel()

	interceptor := pluginhost.TraceInterceptor()
	assert.NotNil(t, interceptor)
}

// T037: Unit test for trace ID metadata injection.
func TestTraceInterceptor_InjectsTraceIDMetadata(t *testing.T) {
	t.Parallel()

	interceptor := pluginhost.TraceInterceptor()

	// Create context with trace ID
	ctx := context.Background()
	traceID := "test-trace-id-12345"
	ctx = logging.ContextWithTraceID(ctx, traceID)

	// Create a mock invoker that captures the context using a channel
	capturedCtxChan := make(chan context.Context, 1)
	mockInvoker := func(invokerCtx context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
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

	values := md.Get(pluginhost.TraceIDMetadataKey)
	require.Len(t, values, 1, "should have exactly one trace ID value")
	assert.Equal(t, traceID, values[0])
}

// T037: Test that interceptor handles missing trace ID gracefully.
func TestTraceInterceptor_NoTraceIDNoMetadata(t *testing.T) {
	t.Parallel()

	interceptor := pluginhost.TraceInterceptor()

	// Create context without trace ID
	ctx := context.Background()

	// Create a mock invoker that captures the context using a channel
	capturedCtxChan := make(chan context.Context, 1)
	mockInvoker := func(invokerCtx context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
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
		values := md.Get(pluginhost.TraceIDMetadataKey)
		assert.Empty(t, values, "should not have trace ID metadata when not set in context")
	}
	// If no metadata at all, that's also fine
}

// T037: Test that interceptor preserves existing metadata.
func TestTraceInterceptor_PreservesExistingMetadata(t *testing.T) {
	t.Parallel()

	interceptor := pluginhost.TraceInterceptor()

	// Create context with existing metadata and trace ID
	ctx := context.Background()
	traceID := "test-trace-id-67890"
	ctx = logging.ContextWithTraceID(ctx, traceID)
	ctx = metadata.AppendToOutgoingContext(ctx, "existing-key", "existing-value")

	// Create a mock invoker that captures the context using a channel
	capturedCtxChan := make(chan context.Context, 1)
	mockInvoker := func(invokerCtx context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
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

	traceValues := md.Get(pluginhost.TraceIDMetadataKey)
	require.Len(t, traceValues, 1)
	assert.Equal(t, traceID, traceValues[0])
}

// T037: Test that interceptor propagates invoker errors.
func TestTraceInterceptor_PropagatesInvokerError(t *testing.T) {
	t.Parallel()

	interceptor := pluginhost.TraceInterceptor()

	ctx := context.Background()
	ctx = logging.ContextWithTraceID(ctx, "test-trace-id")

	// Create a mock invoker that returns an error
	expectedErr := assert.AnError
	mockInvoker := func(_ context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return expectedErr
	}

	// Call interceptor
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, mockInvoker)
	assert.ErrorIs(t, err, expectedErr)
}

// Test the TraceIDMetadataKey constant value.
func TestTraceIDMetadataKey_Value(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "x-finfocus-trace-id", pluginhost.TraceIDMetadataKey)
}

func TestLoggedInterceptor(t *testing.T) {
	t.Parallel()

	// Capture logs
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	interceptor := pluginhost.LoggedInterceptor(logger)

	ctx := context.Background()
	method := "/test.Service/Method"

	// Success case
	mockInvokerSuccess := func(_ context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return nil
	}

	err := interceptor(ctx, method, nil, nil, nil, mockInvokerSuccess)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "making gRPC call to plugin")

	// Error case
	buf.Reset()
	mockInvokerError := func(_ context.Context, _ string, _, _ interface{}, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return assert.AnError
	}

	err = interceptor(ctx, method, nil, nil, nil, mockInvokerError)
	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, buf.String(), "gRPC call failed")
}

func TestGRPCInterceptor_HandledDeclinesLogAtDebug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "invalid argument",
			err:  status.Error(codes.InvalidArgument, "region is required"),
			want: "debug",
		},
		{
			name: "unimplemented",
			err:  status.Error(codes.Unimplemented, "not implemented"),
			want: "debug",
		},
		{
			name: "unavailable",
			err:  status.Error(codes.Unavailable, "invalid region"),
			want: "warn",
		},
		{
			name: "non status",
			err:  errors.New("boom"),
			want: "warn",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Run("trace", func(t *testing.T) {
				t.Parallel()
				assertGRPCFailureLevel(t, tt.err, tt.want, true)
			})
			t.Run("logged", func(t *testing.T) {
				t.Parallel()
				assertGRPCFailureLevel(t, tt.err, tt.want, false)
			})
		})
	}
}

func assertGRPCFailureLevel(t *testing.T, callErr error, want string, trace bool) {
	t.Helper()

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.TraceLevel)
	ctx := context.Background()
	var interceptor grpc.UnaryClientInterceptor
	if trace {
		ctx = logger.WithContext(ctx)
		interceptor = pluginhost.TraceInterceptor()
	} else {
		interceptor = pluginhost.LoggedInterceptor(logger)
	}
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return callErr
	}
	err := interceptor(ctx, "/test.Service/Method", nil, nil, nil, invoker)
	require.ErrorIs(t, err, callErr)
	assert.Equal(t, want, logLevelForMessage(t, buf.String(), "gRPC call failed"))
}

func logLevelForMessage(t *testing.T, raw, message string) string {
	t.Helper()
	for _, line := range strings.Split(raw, "\n") {
		if !strings.Contains(line, message) {
			continue
		}
		var event map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		level, _ := event["level"].(string)
		return level
	}
	require.Failf(t, "missing log line", "message %q not in %s", message, raw)
	return ""
}
