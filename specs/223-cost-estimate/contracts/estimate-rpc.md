# gRPC Contract: EstimateCost RPC

**Service**: CostSourceService
**RPC**: EstimateCost
**Source**: finfocus-spec v0.5.5

## Overview

The `EstimateCost` RPC enables what-if cost analysis by calculating costs for hypothetical resource configurations. Unlike `GetProjectedCost`, this RPC accepts arbitrary property overrides and returns both baseline and modified costs.

## Request

### EstimateCostRequest

```protobuf
message EstimateCostRequest {
  // The resource to estimate costs for
  ResourceDescriptor resource = 1;

  // Property overrides to apply for the modified calculation
  // Keys are property names, values are the new values to use
  map<string, string> property_overrides = 2;

  // Optional usage profile for context (e.g., development, production)
  UsageProfile usage_profile = 3;
}
```

### ResourceDescriptor

```protobuf
message ResourceDescriptor {
  string id = 1;
  string provider = 2;         // e.g., "aws", "gcp", "azure"
  string resource_type = 3;    // e.g., "ec2:Instance"
  string sku = 4;              // e.g., "t3.micro"
  string region = 5;           // e.g., "us-east-1"
  map<string, string> tags = 6;
}
```

### UsageProfile

```protobuf
message UsageProfile {
  string name = 1;             // e.g., "development", "production"
  double utilization = 2;      // 0.0-1.0, for usage-based adjustments
}
```

## Response

### EstimateCostResponse

```protobuf
message EstimateCostResponse {
  // Cost with original resource properties
  CostResult baseline = 1;

  // Cost with property_overrides applied
  CostResult modified = 2;

  // Per-property cost impact breakdown
  repeated CostDelta deltas = 3;
}
```

### CostResult

```protobuf
message CostResult {
  string currency = 1;         // e.g., "USD"
  double monthly_cost = 2;     // Monthly cost in currency units
  double hourly_cost = 3;      // Hourly cost in currency units
  string notes = 4;            // Optional notes about the calculation
  map<string, double> breakdown = 5; // Cost breakdown by component
}
```

### CostDelta

```protobuf
message CostDelta {
  string property = 1;         // Property name that was changed
  string original_value = 2;   // Value before override
  string new_value = 3;        // Value after override
  double cost_change = 4;      // Monthly cost difference (positive = increase)
}
```

## Error Handling

### gRPC Status Codes

| Code | Condition | Client Action |
|------|-----------|---------------|
| `OK` | Success | Process response |
| `INVALID_ARGUMENT` | Invalid request (missing fields, bad format) | Fix request and retry |
| `NOT_FOUND` | Unknown resource type or region | Check resource type and region |
| `UNIMPLEMENTED` | Plugin doesn't support EstimateCost | Fall back to GetProjectedCost |
| `UNAVAILABLE` | Plugin temporarily unavailable | Retry with backoff |
| `INTERNAL` | Plugin internal error | Log and report error |

### Fallback Strategy

When a plugin returns `UNIMPLEMENTED`:

1. Call `GetProjectedCost` with original resource properties → baseline
2. Merge property_overrides into resource properties
3. Call `GetProjectedCost` with modified properties → modified
4. Calculate deltas: for each override, delta = (modified - baseline)

Note: Fallback cannot provide per-property delta breakdown accurately when multiple properties are overridden simultaneously. In this case, report a single delta with property="combined".

## Validation

### Request Validation

Before calling the RPC:

1. `resource.provider` MUST NOT be empty
2. `resource.resource_type` MUST NOT be empty
3. `property_overrides` MUST have at least one entry for meaningful comparison
4. Each override key MUST NOT be empty

### Response Validation

After receiving the response:

1. `baseline` and `modified` SHOULD both be present
2. `baseline.currency` SHOULD equal `modified.currency`
3. `deltas` MAY be empty if no per-property breakdown available

## Example Usage

### Example Request

```json
{
  "resource": {
    "provider": "aws",
    "resource_type": "ec2:Instance",
    "sku": "t3.micro",
    "region": "us-east-1"
  },
  "property_overrides": {
    "instanceType": "m5.large",
    "volumeSize": "100"
  },
  "usage_profile": {
    "name": "production",
    "utilization": 0.8
  }
}
```

### Example Response

```json
{
  "baseline": {
    "currency": "USD",
    "monthly_cost": 8.32,
    "hourly_cost": 0.0114,
    "notes": "On-demand pricing, t3.micro"
  },
  "modified": {
    "currency": "USD",
    "monthly_cost": 83.22,
    "hourly_cost": 0.114,
    "notes": "On-demand pricing, m5.large + 100GB EBS"
  },
  "deltas": [
    {
      "property": "instanceType",
      "original_value": "t3.micro",
      "new_value": "m5.large",
      "cost_change": 65.70
    },
    {
      "property": "volumeSize",
      "original_value": "8",
      "new_value": "100",
      "cost_change": 9.20
    }
  ]
}
```

## Integration Notes

### Core Implementation

The core (`finfocus`) calls this RPC via the engine layer:

```go
// internal/engine/estimate.go
func (e *Engine) EstimateCost(ctx context.Context, req *EstimateRequest) (*EstimateResult, error) {
    protoReq := buildEstimateCostRequest(req)

    for _, client := range e.clients {
        resp, err := client.API.EstimateCost(ctx, protoReq)
        if err != nil {
            if isUnimplemented(err) {
                return e.estimateCostFallback(ctx, req)
            }
            continue
        }
        return convertEstimateResponse(resp), nil
    }
    return nil, ErrNoPluginAvailable
}
```

### Plugin Implementation

Plugins implement this RPC in their gRPC server:

```go
// plugin implementation
func (s *Server) EstimateCost(ctx context.Context, req *pb.EstimateCostRequest) (*pb.EstimateCostResponse, error) {
    // Calculate baseline with original properties
    baseline := s.calculateCost(req.Resource)

    // Apply overrides and calculate modified
    modifiedResource := applyOverrides(req.Resource, req.PropertyOverrides)
    modified := s.calculateCost(modifiedResource)

    // Calculate per-property deltas
    deltas := s.calculateDeltas(req.Resource, req.PropertyOverrides)

    return &pb.EstimateCostResponse{
        Baseline: baseline,
        Modified: modified,
        Deltas:   deltas,
    }, nil
}
```
