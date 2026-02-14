# PluginSDK API Reference

## Table of Contents

- [Imports](#imports)
- [BasePlugin](#baseplugin)
- [Validation Functions](#validation-functions)
- [Environment Constants](#environment-constants)
- [gRPC Metadata](#grpc-metadata)
- [Proto Message Types](#proto-message-types)
- [Error Handling](#error-handling)

## Imports

```go
import (
    "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)
```

## BasePlugin

Embed for common functionality:

```go
type BasePlugin struct {
    name      string
    providers []string  // Supported providers (use ["*"] for wildcard)
}

pluginsdk.NewBasePlugin("my-plugin", []string{"aws", "azure"})
pluginsdk.NewBasePlugin("recorder", []string{"*"})  // Wildcard
```

## Validation Functions

```go
// Validate projected cost request (checks Resource field)
pluginsdk.ValidateProjectedCostRequest(req *pbc.GetProjectedCostRequest) error

// Validate actual cost request (checks Resource, dates)
pluginsdk.ValidateActualCostRequest(req *pbc.GetActualCostRequest) error
```

## Environment Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `pluginsdk.EnvPort` | `FINFOCUS_PLUGIN_PORT` | gRPC listen port |
| `pluginsdk.EnvLogLevel` | `FINFOCUS_LOG_LEVEL` | Log verbosity |
| `pluginsdk.EnvLogFormat` | `FINFOCUS_LOG_FORMAT` | json or text |
| `pluginsdk.EnvLogFile` | `FINFOCUS_LOG_FILE` | Log file path |
| `pluginsdk.EnvTraceID` | `FINFOCUS_TRACE_ID` | Trace correlation ID |

Reading port: `pluginsdk.GetPort()` returns the configured port.

## gRPC Metadata

| Constant | Value | Purpose |
|----------|-------|---------|
| `pluginsdk.TraceIDMetadataKey` | `x-finfocus-trace-id` | Trace ID in gRPC metadata |

**Server interceptor** (extracts trace ID from incoming calls):

```go
server := grpc.NewServer(
    grpc.UnaryInterceptor(pluginsdk.TracingUnaryServerInterceptor()),
)
```

**Get trace ID in handler**:

```go
traceID := pluginsdk.TraceIDFromContext(ctx)
```

## Proto Message Types

### GetProjectedCostRequest

```protobuf
message GetProjectedCostRequest {
  ResourceDescriptor resource = 1;
}

message ResourceDescriptor {
  string provider = 1;
  string resource_type = 2;
  string sku = 3;
  string region = 4;
  map<string, string> tags = 5;
}
```

### GetProjectedCostResponse

```protobuf
message GetProjectedCostResponse {
  repeated CostEstimate estimates = 1;
}

message CostEstimate {
  double monthly_cost = 1;
  double hourly_cost = 2;
  string currency = 3;
  string description = 4;
  map<string, double> breakdown = 5;
}
```

### GetActualCostRequest

```protobuf
message GetActualCostRequest {
  ResourceDescriptor resource = 1;
  google.protobuf.Timestamp start_time = 2;
  google.protobuf.Timestamp end_time = 3;
  string granularity = 4;  // "daily" or "monthly"
}
```

### GetRecommendationsRequest

```protobuf
message GetRecommendationsRequest {
  string provider = 1;
  int32 page_size = 2;    // 0-1000
  string page_token = 3;  // Pagination cursor
}
```

### GetPluginInfoRequest / Response

```protobuf
message GetPluginInfoResponse {
  string name = 1;
  string version = 2;
  repeated string providers = 3;
}
```

## Error Handling

Return gRPC status errors:

```go
// Validation error
return nil, status.Errorf(codes.InvalidArgument, "missing resource: %v", err)

// Internal error
return nil, status.Errorf(codes.Internal, "failed to query costs: %v", err)

// Not found
return nil, status.Errorf(codes.NotFound, "no pricing data for %s", resourceType)
```

Core handles these errors gracefully and continues processing other resources.
