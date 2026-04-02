---
layout: default
title: Plugin Protocol Specification
description: Complete gRPC protocol specification for FinFocus cost source plugins
---

This document specifies the gRPC protocol that cost source plugins must
implement to integrate with FinFocus. The protocol is defined in protocol
buffers in the `finfocus-spec` repository.

## Protocol Version

**Current Version:** v0.1.0 (frozen and integrated)

**Protocol Definition:**
`github.com/rshade/finfocus-spec/proto/finfocus/v1/costsource.proto`

**Generated SDK:**
`github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`

## Service Definitions

### CostSourceService

The CostSourceService provides the core gRPC interface for cost source
plugins. All plugins MUST implement this service.

```protobuf
service CostSourceService {
  rpc Name(NameRequest) returns (NameResponse);
  rpc Supports(SupportsRequest) returns (SupportsResponse);
  rpc GetActualCost(GetActualCostRequest) returns (GetActualCostResponse);
  rpc GetProjectedCost(GetProjectedCostRequest)
      returns (GetProjectedCostResponse);
  rpc GetPricingSpec(GetPricingSpecRequest)
      returns (GetPricingSpecResponse);
}
```

### ObservabilityService

The ObservabilityService provides telemetry, health checks, and monitoring
capabilities. Implementation is OPTIONAL but RECOMMENDED.

```protobuf
service ObservabilityService {
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  rpc GetMetrics(GetMetricsRequest) returns (GetMetricsResponse);
  rpc GetServiceLevelIndicators(GetServiceLevelIndicatorsRequest)
      returns (GetServiceLevelIndicatorsResponse);
}
```

## Core RPC Methods

### Name

Returns the display name of the cost source plugin.

**Request:**

```protobuf
message NameRequest {}
```

**Response:**

```protobuf
message NameResponse {
  string name = 1;
}
```

**Example:**

```json
{
  "name": "kubecost"
}
```

### Supports

Checks if the cost source supports pricing for a given resource type.

**Request:**

```protobuf
message SupportsRequest {
  ResourceDescriptor resource = 1;
}
```

**Response:**

```protobuf
message SupportsResponse {
  bool supported = 1;
  string reason = 2;
}
```

**Example Success:**

```json
{
  "supported": true
}
```

**Example Failure:**

```json
{
  "supported": false,
  "reason": "Unsupported provider: azure"
}
```

### GetProjectedCost

Calculates projected cost information for a resource.

**Request:**

```protobuf
message GetProjectedCostRequest {
  ResourceDescriptor resource = 1;
}
```

**Response:**

```protobuf
message GetProjectedCostResponse {
  double unit_price = 1;
  string currency = 2;
  double cost_per_month = 3;
  string billing_detail = 4;
}
```

**Example:**

```json
{
  "unit_price": 0.0104,
  "currency": "USD",
  "cost_per_month": 7.592,
  "billing_detail": "on-demand"
}
```

### GetActualCost

Retrieves historical cost data for a specific resource.

**Request:**

```protobuf
message GetActualCostRequest {
  string resource_id = 1;
  google.protobuf.Timestamp start = 2;
  google.protobuf.Timestamp end = 3;
  map<string, string> tags = 4;
}
```

**Response:**

```protobuf
message GetActualCostResponse {
  repeated ActualCostResult results = 1;
}

message ActualCostResult {
  google.protobuf.Timestamp timestamp = 1;
  double cost = 2;
  double usage_amount = 3;
  string usage_unit = 4;
  string source = 5;
}
```

**Example:**

```json
{
  "results": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "cost": 73.42,
      "usage_amount": 730.0,
      "usage_unit": "hour",
      "source": "kubecost"
    },
    {
      "timestamp": "2024-01-02T00:00:00Z",
      "cost": 74.18,
      "usage_amount": 730.0,
      "usage_unit": "hour",
      "source": "kubecost"
    }
  ]
}
```

### GetPricingSpec

Returns detailed pricing specification for a resource type.

**Request:**

```protobuf
message GetPricingSpecRequest {
  ResourceDescriptor resource = 1;
}
```

**Response:**

```protobuf
message GetPricingSpecResponse {
  PricingSpec spec = 1;
}
```

**Example:**

```json
{
  "spec": {
    "provider": "aws",
    "resource_type": "ec2",
    "sku": "t3.micro",
    "region": "us-east-1",
    "billing_mode": "per_hour",
    "rate_per_unit": 0.0104,
    "currency": "USD",
    "description": "AWS EC2 t3.micro instance",
    "source": "vantage"
  }
}
```

## Message Types

### ResourceDescriptor

Describes a cloud resource for cost analysis.

```protobuf
message ResourceDescriptor {
  string provider = 1;
  string resource_type = 2;
  string sku = 3;
  string region = 4;
  map<string, string> tags = 5;
}
```

**Fields:**

- `provider` - Cloud provider ("aws", "azure", "gcp", "kubernetes", "custom")
- `resource_type` - Resource type (e.g., "ec2", "s3", "k8s-namespace")
- `sku` - Provider SKU or instance size (e.g., "t3.micro")
- `region` - Deployment region (e.g., "us-east-1")
- `tags` - Label/tag hints for resource identification (e.g., app=web)

### PricingSpec

Detailed pricing information for a specific resource type.

```protobuf
message PricingSpec {
  string provider = 1;
  string resource_type = 2;
  string sku = 3;
  string region = 4;
  string billing_mode = 5;
  double rate_per_unit = 6;
  string currency = 7;
  string description = 8;
  repeated UsageMetricHint metric_hints = 9;
  map<string, string> plugin_metadata = 10;
  string source = 11;
}
```

**Billing Modes:**

- `per_hour` - Hourly billing (compute instances)
- `per_gb_month` - Storage billing (GB per month)
- `per_request` - Request-based billing (API calls)
- `per_day` - Daily billing
- `per_cpu_hour` - CPU-hour billing (Kubernetes)
- `flat` - Flat-rate billing

### Error Handling

#### ErrorCode Enumeration

```protobuf
enum ErrorCode {
  ERROR_CODE_UNSPECIFIED = 0;

  // Transient errors
  ERROR_CODE_NETWORK_TIMEOUT = 1;
  ERROR_CODE_SERVICE_UNAVAILABLE = 2;
  ERROR_CODE_RATE_LIMITED = 3;
  ERROR_CODE_TEMPORARY_FAILURE = 4;
  ERROR_CODE_CIRCUIT_OPEN = 5;

  // Permanent errors
  ERROR_CODE_INVALID_RESOURCE = 6;
  ERROR_CODE_RESOURCE_NOT_FOUND = 7;
  ERROR_CODE_INVALID_TIME_RANGE = 8;
  ERROR_CODE_UNSUPPORTED_REGION = 9;
  ERROR_CODE_PERMISSION_DENIED = 10;
  ERROR_CODE_DATA_CORRUPTION = 11;

  // Configuration errors
  ERROR_CODE_INVALID_CREDENTIALS = 12;
  ERROR_CODE_MISSING_API_KEY = 13;
  ERROR_CODE_INVALID_ENDPOINT = 14;
  ERROR_CODE_INVALID_PROVIDER = 15;
  ERROR_CODE_PLUGIN_NOT_CONFIGURED = 16;
}
```

#### ErrorCategory Enumeration

```protobuf
enum ErrorCategory {
  ERROR_CATEGORY_UNSPECIFIED = 0;
  ERROR_CATEGORY_TRANSIENT = 1;
  ERROR_CATEGORY_PERMANENT = 2;
  ERROR_CATEGORY_CONFIGURATION = 3;
}
```

#### ErrorDetail Message

```protobuf
message ErrorDetail {
  ErrorCode code = 1;
  ErrorCategory category = 2;
  string message = 3;
  map<string, string> details = 4;
  optional int32 retry_after_seconds = 5;
  google.protobuf.Timestamp timestamp = 6;
}
```

## Plugin Implementation Guide

### Minimal Plugin Implementation

A minimal plugin MUST implement:

1. `Name()` - Return plugin name
2. `Supports()` - Indicate supported resource types
3. `GetProjectedCost()` - Provide cost estimates

**Example (Go):**

```go
type MyPlugin struct {
    pb.UnimplementedCostSourceServiceServer
}

func (p *MyPlugin) Name(ctx context.Context,
    req *pb.NameRequest) (*pb.NameResponse, error) {
    return &pb.NameResponse{Name: "my-plugin"}, nil
}

func (p *MyPlugin) Supports(ctx context.Context,
    req *pb.SupportsRequest) (*pb.SupportsResponse, error) {
    supported := req.Resource.Provider == "aws"
    return &pb.SupportsResponse{Supported: supported}, nil
}

func (p *MyPlugin) GetProjectedCost(ctx context.Context,
    req *pb.GetProjectedCostRequest) (
    *pb.GetProjectedCostResponse, error) {
    return &pb.GetProjectedCostResponse{
        UnitPrice: 0.0104,
        Currency: "USD",
        CostPerMonth: 7.592,
        BillingDetail: "on-demand",
    }, nil
}
```

### Server Setup

Plugins MUST start a gRPC server on a TCP port or stdio:

```go
func main() {
    lis, err := net.Listen("tcp", ":0")
    if err != nil {
        log.Fatal(err)
    }

    // Print port for plugin host to connect
    fmt.Printf("1|1|tcp|127.0.0.1:%d|grpc\n",
        lis.Addr().(*net.TCPAddr).Port)

    server := grpc.NewServer()
    pb.RegisterCostSourceServiceServer(server, &MyPlugin{})

    if err := server.Serve(lis); err != nil {
        log.Fatal(err)
    }
}
```

### Configuration

Plugins read configuration from `~/.finfocus/config.yaml`:

```yaml
integrations:
  my-plugin:
    api_key: 'secret_key'
    endpoint: 'https://api.example.com'
```

### Manifest File

Optional but RECOMMENDED: Create `plugin.manifest.json`:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "protocol_version": "1",
  "executable": "my-plugin",
  "description": "My custom cost source plugin"
}
```

## Protocol Versioning

### Current Version: v0.1.0

**Compatibility:**

- Plugins MUST implement protocol version 1
- Future protocol changes will increment version
- Plugin Host will check protocol_version in manifest
- Incompatible versions will be rejected

### Future Versions

Breaking changes will increment major version (v1.0.0 → v2.0.0):

- Service method signature changes
- Required field additions
- Message type changes

Non-breaking changes will increment minor version (v1.0.0 → v1.1.0):

- Optional field additions
- New service methods
- New error codes

## Transport Options

### TCP Transport (Default)

Plugins listen on a TCP port and print connection info to stdout:

```text
1|1|tcp|127.0.0.1:50051|grpc
```

**Format:** `version|protocol_version|transport|address|protocol`

### Stdio Transport

Alternative for simpler plugins:

- Plugin reads requests from stdin
- Plugin writes responses to stdout
- Plugin writes logs to stderr

## Health Checks

Plugins SHOULD implement HealthCheck for monitoring:

```protobuf
message HealthCheckResponse {
  enum Status {
    STATUS_UNSPECIFIED = 0;
    STATUS_SERVING = 1;
    STATUS_NOT_SERVING = 2;
    STATUS_SERVICE_UNKNOWN = 3;
  }
  Status status = 1;
  string message = 2;
  google.protobuf.Timestamp last_check_time = 3;
}
```

**Usage:**

```bash
grpcurl -plaintext localhost:50051 \
  finfocus.v1.ObservabilityService/HealthCheck
```

## Testing Plugins

### Manual Testing

```bash
# Start plugin
./my-plugin

# Test Name RPC
grpcurl -plaintext localhost:50051 \
  finfocus.v1.CostSourceService/Name

# Test Supports RPC
grpcurl -plaintext -d '{
  "resource": {
    "provider": "aws",
    "resource_type": "ec2",
    "sku": "t3.micro"
  }
}' localhost:50051 \
  finfocus.v1.CostSourceService/Supports
```

### Integration Testing

Use FinFocus CLI to test end-to-end:

```bash
# Install plugin
mkdir -p ~/.finfocus/plugins/my-plugin/1.0.0
cp my-plugin ~/.finfocus/plugins/my-plugin/1.0.0/

# Test with FinFocus
finfocus plugin list
finfocus cost projected --pulumi-json plan.json
```

---

**Related Documentation:**

- [System Overview](system-overview.md) - High-level architecture
- [Plugin Lifecycle](diagrams/plugin-lifecycle.md) - Lifecycle states
- [API Reference](../reference/api-reference.md) - Complete API docs
- [Plugin Development](../plugins/plugin-development.md) - Development guide
