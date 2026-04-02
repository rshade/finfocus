# Contract: EstimateCost RPC Mapping

**Feature**: 608-estimate-cost-rpc
**Date**: 2026-03-31

## Overview

This document maps the gRPC contract between finfocus-core (consumer) and
plugins (implementor) for the `EstimateCost` RPC.

## gRPC Method

```text
Service: finfocus.v1.CostSourceService
Method:  EstimateCost
Full:    /finfocus.v1.CostSourceService/EstimateCost
```

## Request/Response Contract

### Request (core → plugin)

```protobuf
message EstimateCostRequest {
  string resource_type = 1;        // e.g., "aws:ec2/instance:Instance"
  google.protobuf.Struct attributes = 2;  // Resource properties
}
```

### Response (plugin → core)

```protobuf
message EstimateCostResponse {
  string currency = 1;                              // ISO 4217
  double cost_monthly = 2;                           // Monthly cost (730h)
  FocusPricingCategory pricing_category = 3;         // Standard/Committed/Dynamic
  double spot_interruption_risk_score = 4;           // 0.0-1.0
}
```

## Core Consumer Contract

### Interface Addition

```go
// Added to CostSourceClient interface in internal/proto/adapter.go
EstimateCost(ctx context.Context, in *pbc.EstimateCostRequest,
    opts ...grpc.CallOption) (*pbc.EstimateCostResponse, error)
```

### Adapter Implementation

```go
// clientAdapter delegates directly to generated client
func (c *clientAdapter) EstimateCost(
    ctx context.Context,
    in *pbc.EstimateCostRequest,
    opts ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
    return c.client.EstimateCost(ctx, in, opts...)
}
```

### Request Builder

```go
// BuildEstimateCostRequest constructs a proto request from engine types.
// Called twice per estimation: once for baseline, once for modified.
func BuildEstimateCostRequest(
    resource *ResourceDescriptor,
    properties map[string]interface{},
) (*pbc.EstimateCostRequest, error)
```

## Error Handling Contract

### gRPC Status Codes

| Code | Condition | Core Action |
|------|-----------|-------------|
| `OK` | Success | Map response to CostResult |
| `UNIMPLEMENTED` | Plugin lacks EstimateCost | Fall through to next plugin; if all unimplemented, use fallback |
| `INVALID_ARGUMENT` | Bad resource type or attributes | Log, try next plugin |
| `NOT_FOUND` | Unknown resource type | Log, try next plugin |
| `DEADLINE_EXCEEDED` | Timeout | Log, try next plugin |
| `UNAVAILABLE` | Plugin down | Log, try next plugin |

### Fallback Chain

```text
For each plugin:
  1. Try EstimateCost RPC (baseline + modified)
  2. If Unimplemented → continue to next plugin
  3. If other error → log, continue to next plugin
  4. If success → return EstimateResult(UsedFallback=false)

If all plugins exhausted:
  5. Use estimateCostFallback (double GetProjectedCost)
  6. Return EstimateResult(UsedFallback=true)
```

## Two-Call Strategy

Since the proto provides single-resource estimation (not baseline/modified
pairs), the engine calls the RPC twice per what-if analysis:

```text
Call 1: EstimateCost(resource_type, original_properties) → baseline
Call 2: EstimateCost(resource_type, merged_properties)   → modified
Delta:  modified.CostMonthly - baseline.CostMonthly
```

Property merging: deep copy original properties, overlay overrides (same
logic as `estimateCostFallback` at `estimate.go:217-225`).
