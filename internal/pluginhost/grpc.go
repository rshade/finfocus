package pluginhost

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/rshade/finfocus/internal/logging"
)

// TraceIDMetadataKey is the gRPC metadata key for trace ID propagation.
const TraceIDMetadataKey = "x-finfocus-trace-id"

// TraceInterceptor returns a gRPC unary client interceptor that propagates trace IDs.
// The interceptor extracts the trace ID from the context and injects it into gRPC metadata,
// allowing end-to-end request tracing across process boundaries to plugins.
func TraceInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Extract trace ID from context
		traceID := logging.TraceIDFromContext(ctx)

		// Inject trace ID into metadata if present
		if traceID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, TraceIDMetadataKey, traceID)
		}

		// Log the gRPC call
		log := logging.FromContext(ctx)
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("operation", "grpc_call").
			Str("method", method).
			Str("trace_id", traceID).
			Msg("making gRPC call to plugin")

		// Invoke the actual RPC
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Log result
		if err != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "pluginhost").
				Str("operation", "grpc_call").
				Str("method", method).
				Err(err).
				Msg("gRPC call failed")
		} else {
			log.Debug().
				Ctx(ctx).
				Str("component", "pluginhost").
				Str("operation", "grpc_call").
				Str("method", method).
				Msg("gRPC call completed")
		}

		return err
	}
}

// LoggedInterceptor returns a gRPC unary client interceptor that logs calls without trace propagation.
// Use this when trace propagation is not needed but logging is desired.
func LoggedInterceptor(logger zerolog.Logger) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		logger.Debug().
			Str("component", "pluginhost").
			Str("operation", "grpc_call").
			Str("method", method).
			Msg("making gRPC call to plugin")

		err := invoker(ctx, method, req, reply, cc, opts...)

		if err != nil {
			logger.Warn().
				Str("component", "pluginhost").
				Str("operation", "grpc_call").
				Str("method", method).
				Err(err).
				Msg("gRPC call failed")
		}

		return err
	}
}
