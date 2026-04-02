---
layout: default
title: API Reference
description: Complete gRPC API reference for FinFocus plugin protocol
---

Complete API reference for the FinFocus plugin protocol defined in
`finfocus-spec` repository.

## Protocol Information

**Package:** `finfocus.v1`

**Protocol Version:** v0.1.0

**Go Package:**
`github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1`

**Proto Definition:**
`github.com/rshade/finfocus-spec/proto/finfocus/v1/costsource.proto`

## Services

### CostSourceService

Core cost calculation service. All plugins MUST implement this service.

**Service Definition:**

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

Monitoring and health check service. Implementation is OPTIONAL but
RECOMMENDED.

**Service Definition:**

```protobuf
service ObservabilityService {
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  rpc GetMetrics(GetMetricsRequest) returns (GetMetricsResponse);
  rpc GetServiceLevelIndicators(GetServiceLevelIndicatorsRequest)
      returns (GetServiceLevelIndicatorsResponse);
}
```

## RPC Methods

### Name

Returns the display name of the cost source plugin.

**Method:** `Name(NameRequest) NameResponse`

**Request:**

```protobuf
message NameRequest {}
```

No fields required.

**Response:**

```protobuf
message NameResponse {
  string name = 1;
}
```

**Fields:**

- `name` (string) - Plugin display name (e.g., "kubecost", "vantage")

**Example:**

```bash
# Request
grpcurl -plaintext localhost:50051 \
  finfocus.v1.CostSourceService/Name

# Response
{
  "name": "kubecost"
}
```

**Client Example (Go):**

```go
ctx := context.Background()
req := &pb.NameRequest{}
resp, err := client.Name(ctx, req)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Plugin name:", resp.Name)
```

---

### Supports

Checks if the cost source supports pricing for a given resource type.

**Method:** `Supports(SupportsRequest) SupportsResponse`

**Request:**

```protobuf
message SupportsRequest {
  ResourceDescriptor resource = 1;
}
```

**Fields:**

- `resource` (ResourceDescriptor) - Resource to check support for

**Response:**

```protobuf
message SupportsResponse {
  bool supported = 1;
  string reason = 2;
}
```

**Fields:**

- `supported` (bool) - True if resource type is supported
- `reason` (string) - Optional explanation if not supported

**Example:**

```bash
# Request
grpcurl -plaintext -d '{
  "resource": {
    "provider": "aws",
    "resource_type": "ec2",
    "sku": "t3.micro",
    "region": "us-east-1"
  }
}' localhost:50051 \
  finfocus.v1.CostSourceService/Supports

# Response (Supported)
{
  "supported": true
}

# Response (Not Supported)
{
  "supported": false,
  "reason": "Unsupported provider: azure"
}
```

**Client Example (Go):**

```go
ctx := context.Background()
req := &pb.SupportsRequest{
    Resource: &pb.ResourceDescriptor{
        Provider: "aws",
        ResourceType: "ec2",
        Sku: "t3.micro",
        Region: "us-east-1",
    },
}
resp, err := client.Supports(ctx, req)
if err != nil {
    log.Fatal(err)
}
if resp.Supported {
    fmt.Println("Resource is supported")
} else {
    fmt.Printf("Not supported: %s\n", resp.Reason)
}
```

---

### GetProjectedCost

Calculates projected cost information for a resource.

**Method:** `GetProjectedCost(GetProjectedCostRequest)
GetProjectedCostResponse`

**Request:**

```protobuf
message GetProjectedCostRequest {
  ResourceDescriptor resource = 1;
}
```

**Fields:**

- `resource` (ResourceDescriptor) - Resource to calculate projected cost for

**Response:**

```protobuf
message GetProjectedCostResponse {
  double unit_price = 1;
  string currency = 2;
  double cost_per_month = 3;
  string billing_detail = 4;
}
```

**Fields:**

- `unit_price` (double) - Price per unit (hourly rate for compute)
- `currency` (string) - Currency code (e.g., "USD", "EUR")
- `cost_per_month` (double) - Estimated monthly cost
- `billing_detail` (string) - Billing context (e.g., "on-demand",
  "reserved")

**Example:**

```bash
# Request
grpcurl -plaintext -d '{
  "resource": {
    "provider": "aws",
    "resource_type": "ec2",
    "sku": "t3.micro",
    "region": "us-east-1"
  }
}' localhost:50051 \
  finfocus.v1.CostSourceService/GetProjectedCost

# Response
{
  "unit_price": 0.0104,
  "currency": "USD",
  "cost_per_month": 7.592,
  "billing_detail": "on-demand"
}
```

**Client Example (Go):**

```go
ctx := context.Background()
req := &pb.GetProjectedCostRequest{
    Resource: &pb.ResourceDescriptor{
        Provider: "aws",
        ResourceType: "ec2",
        Sku: "t3.micro",
        Region: "us-east-1",
    },
}
resp, err := client.GetProjectedCost(ctx, req)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Monthly cost: %.2f %s\n",
    resp.CostPerMonth, resp.Currency)
```

---

### GetActualCost

Retrieves historical cost data for a specific resource.

**Method:** `GetActualCost(GetActualCostRequest) GetActualCostResponse`

**Request:**

```protobuf
message GetActualCostRequest {
  string resource_id = 1;
  google.protobuf.Timestamp start = 2;
  google.protobuf.Timestamp end = 3;
  map<string, string> tags = 4;
}
```

**Fields:**

- `resource_id` (string) - Plugin-specific resource identifier
  (e.g., "i-abc123")
- `start` (Timestamp) - Start of time range
- `end` (Timestamp) - End of time range
- `tags` (map) - Optional tag filters (e.g., `{env: "prod"}`)

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

**Fields (ActualCostResult):**

- `timestamp` (Timestamp) - Point-in-time or bucket start
- `cost` (double) - Total cost for the period
- `usage_amount` (double) - Optional usage amount
- `usage_unit` (string) - Unit of usage (e.g., "hour", "GB")
- `source` (string) - Data source (e.g., "kubecost", "vantage")

**Example:**

```bash
# Request
grpcurl -plaintext -d '{
  "resource_id": "i-abc123",
  "start": "2024-01-01T00:00:00Z",
  "end": "2024-01-03T00:00:00Z",
  "tags": {
    "env": "prod"
  }
}' localhost:50051 \
  finfocus.v1.CostSourceService/GetActualCost

# Response
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
    },
    {
      "timestamp": "2024-01-03T00:00:00Z",
      "cost": 73.89,
      "usage_amount": 730.0,
      "usage_unit": "hour",
      "source": "kubecost"
    }
  ]
}
```

**Client Example (Go):**

```go
ctx := context.Background()
start := timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
end := timestamppb.New(time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC))

req := &pb.GetActualCostRequest{
    ResourceId: "i-abc123",
    Start: start,
    End: end,
    Tags: map[string]string{
        "env": "prod",
    },
}
resp, err := client.GetActualCost(ctx, req)
if err != nil {
    log.Fatal(err)
}

total := 0.0
for _, result := range resp.Results {
    fmt.Printf("%s: $%.2f\n",
        result.Timestamp.AsTime().Format("2006-01-02"),
        result.Cost)
    total += result.Cost
}
fmt.Printf("Total: $%.2f\n", total)
```

---

### GetPricingSpec

Returns detailed pricing specification for a resource type.

**Method:** `GetPricingSpec(GetPricingSpecRequest) GetPricingSpecResponse`

**Request:**

```protobuf
message GetPricingSpecRequest {
  ResourceDescriptor resource = 1;
}
```

**Fields:**

- `resource` (ResourceDescriptor) - Resource to get pricing spec for

**Response:**

```protobuf
message GetPricingSpecResponse {
  PricingSpec spec = 1;
}

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

**Fields (PricingSpec):**

- `provider` (string) - Cloud provider
- `resource_type` (string) - Resource type
- `sku` (string) - SKU or instance type
- `region` (string) - Geographic region
- `billing_mode` (string) - Billing mode (per_hour, per_gb_month, etc.)
- `rate_per_unit` (double) - Price per billing unit
- `currency` (string) - Currency code
- `description` (string) - Human-readable description
- `metric_hints` (repeated UsageMetricHint) - Usage metric guidance
- `plugin_metadata` (map) - Plugin-specific metadata
- `source` (string) - Where pricing originated (aws, kubecost, etc.)

**Example:**

```bash
# Request
grpcurl -plaintext -d '{
  "resource": {
    "provider": "aws",
    "resource_type": "ec2",
    "sku": "t3.micro",
    "region": "us-east-1"
  }
}' localhost:50051 \
  finfocus.v1.CostSourceService/GetPricingSpec

# Response
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

---

### HealthCheck

Returns the current health status of the plugin.

**Method:** `HealthCheck(HealthCheckRequest) HealthCheckResponse`

**Request:**

```protobuf
message HealthCheckRequest {
  string service_name = 1;
}
```

**Fields:**

- `service_name` (string) - Optional specific service to check

**Response:**

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

**Fields:**

- `status` (Status) - Health status
- `message` (string) - Optional status details
- `last_check_time` (Timestamp) - When status was last updated

**Example:**

```bash
# Request
grpcurl -plaintext localhost:50051 \
  finfocus.v1.ObservabilityService/HealthCheck

# Response
{
  "status": "STATUS_SERVING",
  "message": "All systems operational",
  "last_check_time": "2024-01-15T10:30:00Z"
}
```

## Common Types

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

- `provider` (string) - Cloud provider (aws, azure, gcp, kubernetes)
- `resource_type` (string) - Resource type (ec2, s3, vm, k8s-namespace)
- `sku` (string) - SKU or instance size (t3.micro, Standard_D2s_v3)
- `region` (string) - Geographic region (us-east-1, westus2)
- `tags` (map) - Label/tag hints for identification

**Example:**

```json
{
  "provider": "aws",
  "resource_type": "ec2",
  "sku": "t3.micro",
  "region": "us-east-1",
  "tags": {
    "env": "prod",
    "app": "web"
  }
}
```

### Error Codes

```protobuf
enum ErrorCode {
  ERROR_CODE_UNSPECIFIED = 0;
  ERROR_CODE_NETWORK_TIMEOUT = 1;
  ERROR_CODE_SERVICE_UNAVAILABLE = 2;
  ERROR_CODE_RATE_LIMITED = 3;
  ERROR_CODE_TEMPORARY_FAILURE = 4;
  ERROR_CODE_CIRCUIT_OPEN = 5;
  ERROR_CODE_INVALID_RESOURCE = 6;
  ERROR_CODE_RESOURCE_NOT_FOUND = 7;
  ERROR_CODE_INVALID_TIME_RANGE = 8;
  ERROR_CODE_UNSUPPORTED_REGION = 9;
  ERROR_CODE_PERMISSION_DENIED = 10;
  ERROR_CODE_DATA_CORRUPTION = 11;
  ERROR_CODE_INVALID_CREDENTIALS = 12;
  ERROR_CODE_MISSING_API_KEY = 13;
  ERROR_CODE_INVALID_ENDPOINT = 14;
  ERROR_CODE_INVALID_PROVIDER = 15;
  ERROR_CODE_PLUGIN_NOT_CONFIGURED = 16;
}
```

**Categories:**

- **Transient** (1-5) - Retry with backoff
- **Permanent** (6-11) - Do not retry
- **Configuration** (12-16) - Fix configuration

### Error Categories

```protobuf
enum ErrorCategory {
  ERROR_CATEGORY_UNSPECIFIED = 0;
  ERROR_CATEGORY_TRANSIENT = 1;
  ERROR_CATEGORY_PERMANENT = 2;
  ERROR_CATEGORY_CONFIGURATION = 3;
}
```

## Client Examples

### Complete Go Client

```go
package main

import (
    "context"
    "log"
    "time"

    pb "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
    // Connect to plugin
    conn, err := grpc.Dial("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewCostSourceServiceClient(conn)
    ctx := context.Background()

    // Get plugin name
    nameResp, err := client.Name(ctx, &pb.NameRequest{})
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Plugin: %s", nameResp.Name)

    // Check support
    resource := &pb.ResourceDescriptor{
        Provider: "aws",
        ResourceType: "ec2",
        Sku: "t3.micro",
        Region: "us-east-1",
    }

    supportsResp, err := client.Supports(ctx, &pb.SupportsRequest{
        Resource: resource,
    })
    if err != nil {
        log.Fatal(err)
    }

    if !supportsResp.Supported {
        log.Fatalf("Not supported: %s", supportsResp.Reason)
    }

    // Get projected cost
    projectedResp, err := client.GetProjectedCost(ctx,
        &pb.GetProjectedCostRequest{
            Resource: resource,
        })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Projected cost: $%.2f/month (%s)",
        projectedResp.CostPerMonth,
        projectedResp.Currency)

    // Get actual cost
    start := timestamppb.New(time.Now().AddDate(0, -1, 0))
    end := timestamppb.Now()

    actualResp, err := client.GetActualCost(ctx,
        &pb.GetActualCostRequest{
            ResourceId: "i-abc123",
            Start: start,
            End: end,
        })
    if err != nil {
        log.Fatal(err)
    }

    total := 0.0
    for _, result := range actualResp.Results {
        total += result.Cost
    }
    log.Printf("Actual cost (last month): $%.2f", total)
}
```

---

**Related Documentation:**

- [Plugin Protocol](../architecture/plugin-protocol.md) - Protocol overview
- [System Overview](../architecture/system-overview.md) - Architecture
- [CLI Commands](cli-commands.md) - CLI reference
- [Plugin Development](../plugins/plugin-development.md) - Build plugins
