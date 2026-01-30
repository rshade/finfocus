# Quickstart: Using UnaryInterceptors with ServeConfig

**Branch**: `005-pluginsdk-interceptors`
**Date**: 2025-11-28

## Overview

This guide shows plugin developers how to register custom gRPC interceptors
when serving a FinFocus plugin.

## Prerequisites

- Go 1.25.6+
- `github.com/rshade/finfocus-spec/sdk/go/pluginsdk` v0.2.0+

## Basic Usage

### Using the Built-in Tracing Interceptor (Default)

Existing plugins work unchanged. The built-in `TracingUnaryServerInterceptor()`
runs automatically:

```go
package main

import (
    "context"
    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func main() {
    ctx := context.Background()
    plugin := NewMyPlugin()

    // Tracing interceptor runs automatically - no config needed
    config := pluginsdk.ServeConfig{
        Plugin: plugin,
    }

    if err := pluginsdk.Serve(ctx, config); err != nil {
        log.Fatal(err)
    }
}
```

### Adding Custom Interceptors

Register additional interceptors via `UnaryInterceptors`:

```go
package main

import (
    "context"
    "google.golang.org/grpc"
    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

func main() {
    ctx := context.Background()
    plugin := NewMyPlugin()

    config := pluginsdk.ServeConfig{
        Plugin: plugin,
        UnaryInterceptors: []grpc.UnaryServerInterceptor{
            loggingInterceptor(),
            metricsInterceptor(),
        },
    }

    if err := pluginsdk.Serve(ctx, config); err != nil {
        log.Fatal(err)
    }
}

func loggingInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        // Access trace ID from context (set by built-in tracing interceptor)
        traceID := pluginsdk.TraceIDFromContext(ctx)
        log.Printf("[%s] Request: %s", traceID, info.FullMethod)

        resp, err := handler(ctx, req)

        log.Printf("[%s] Response: err=%v", traceID, err)
        return resp, err
    }
}

func metricsInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        start := time.Now()
        resp, err := handler(ctx, req)
        duration := time.Since(start)

        // Record metrics
        metrics.RecordRPCDuration(info.FullMethod, duration, err)
        return resp, err
    }
}
```

## Interceptor Execution Order

Interceptors execute in this order:

1. `TracingUnaryServerInterceptor()` (always first, built-in)
2. First element of `UnaryInterceptors`
3. Second element of `UnaryInterceptors`
4. ... and so on
5. Actual gRPC handler

**Important**: Because tracing runs first, all custom interceptors have access
to the trace ID via `pluginsdk.TraceIDFromContext(ctx)`.

## Common Patterns

### Authentication Interceptor

```go
func authInterceptor(validator TokenValidator) grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        token, err := extractToken(ctx)
        if err != nil {
            return nil, status.Errorf(codes.Unauthenticated, "missing token")
        }

        if !validator.Validate(token) {
            return nil, status.Errorf(codes.PermissionDenied, "invalid token")
        }

        return handler(ctx, req)
    }
}
```

### Rate Limiting Interceptor

```go
func rateLimitInterceptor(limiter *rate.Limiter) grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        if !limiter.Allow() {
            return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
        }
        return handler(ctx, req)
    }
}
```

## Troubleshooting

### Panic: nil interceptor

**Cause**: One of the interceptors in `UnaryInterceptors` is nil.

**Solution**: Ensure all interceptor functions are non-nil before adding to slice.

```go
// Wrong
config.UnaryInterceptors = []grpc.UnaryServerInterceptor{nil}

// Correct
interceptor := createInterceptor()
if interceptor != nil {
    config.UnaryInterceptors = append(config.UnaryInterceptors, interceptor)
}
```

### Trace ID is empty in custom interceptor

**Cause**: Custom interceptor is somehow running before tracing interceptor.

**Solution**: This should not happen with this implementation. The tracing
interceptor is always prepended. File an issue if you encounter this.

## Migration from Manual Trace Extraction

If you were manually extracting trace IDs in handlers, you can now remove
that code:

```go
// Before: Manual extraction in each handler
func (s *server) GetProjectedCost(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    traceID := extractTraceIDFromMetadata(ctx)  // Remove this
    ctx = context.WithValue(ctx, "trace_id", traceID)  // Remove this
    // ...
}

// After: Use context directly
func (s *server) GetProjectedCost(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    traceID := pluginsdk.TraceIDFromContext(ctx)  // Already available!
    // ...
}
```
