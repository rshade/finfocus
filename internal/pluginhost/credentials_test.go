package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func credentialContext(t *testing.T, values map[string]string) context.Context {
	t.Helper()
	set, err := pluginsdk.NewCredentials(values)
	require.NoError(t, err)
	return pluginsdk.WithCredentials(t.Context(), set)
}

func TestMaskCredentials_ErrorsWithoutAValueAreUnchanged(t *testing.T) {
	t.Parallel()
	ctx := credentialContext(t, map[string]string{"token": "test-secret-value"})
	plain := errors.New("connection lost")
	withStatus := status.Error(codes.NotFound, "no such resource")

	require.NoError(t, maskCredentials(ctx, nil))
	assert.Same(t, plain, maskCredentials(ctx, plain))
	assert.Equal(t, withStatus, maskCredentials(ctx, withStatus))
	assert.Equal(t, withStatus, maskCredentials(t.Context(), withStatus))
}

func TestMaskCredentials_MasksTheMessageAndKeepsTheCode(t *testing.T) {
	t.Parallel()
	ctx := credentialContext(t, map[string]string{"short": "test-secret", "long": "test-secret-value"})

	err := maskCredentials(ctx, status.Error(codes.Unauthenticated, "token test-secret-value and test-secret rejected"))

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Equal(t, "token "+redactedText+" and "+redactedText+" rejected", status.Convert(err).Message())

	plain := maskCredentials(ctx, errors.New("dial with test-secret"))
	assert.NotContains(t, plain.Error(), "test-secret")
}

func TestMaskCredentials_DropsOnlyDetailsThatHoldAValue(t *testing.T) {
	t.Parallel()
	ctx := credentialContext(t, map[string]string{"token": "test-secret-value"})
	retryAfter := int32(7)
	st, err := status.New(codes.ResourceExhausted, "slow down").WithDetails(
		&pbc.ErrorDetail{Message: "token test-secret-value is throttled"},
		&pbc.ErrorDetail{Message: "throttled", RetryAfterSeconds: &retryAfter},
	)
	require.NoError(t, err)

	details := status.Convert(maskCredentials(ctx, st.Err())).Details()

	require.Len(t, details, 1)
	detail, ok := details[0].(*pbc.ErrorDetail)
	require.True(t, ok)
	assert.Equal(t, retryAfter, detail.GetRetryAfterSeconds())
}

type echoServer struct {
	pbc.UnimplementedCostSourceServiceServer
}

func (echoServer) Supports(context.Context, *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "backend refused test-secret-value")
}

// Telemetry records the plugin's error in the span before any interceptor
// sees it, and AX_OTEL_DEBUG prints that span.
//
//nolint:paralleltest // replaces the process-wide tracer provider
func TestDialPlugin_SpansDoNotHoldAnEchoedCredential(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	pbc.RegisterCostSourceServiceServer(server, echoServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := dialPlugin(t.Context(), listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	set, err := pluginsdk.NewCredentials(map[string]string{"token": "test-secret-value"})
	require.NoError(t, err)

	_, err = pbc.NewCostSourceServiceClient(newCredentialConn(conn, set)).
		Supports(t.Context(), &pbc.SupportsRequest{})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "test-secret-value")
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	assert.Contains(t, spans[0].Status().Description, "backend refused")
	assert.NotContains(t, fmt.Sprintf("%+v", tracetest.SpanStubFromReadOnlySpan(spans[0])), "test-secret-value")
}

// fmt ignores a Stringer in an unexported field, so the set must not sit in one.
func TestCredentialConn_FormattingShowsNoValue(t *testing.T) {
	t.Parallel()
	set, err := pluginsdk.NewCredentials(map[string]string{"token": "test-secret-value"})
	require.NoError(t, err)
	client := &Client{Conn: newCredentialConn(&grpc.ClientConn{}, set)}

	for _, format := range []string{"%v", "%+v", "%#v"} {
		assert.NotContains(t, fmt.Sprintf(format, client), "test-secret-value")
		assert.NotContains(t, fmt.Sprintf(format, client.Conn), "test-secret-value")
	}
}
