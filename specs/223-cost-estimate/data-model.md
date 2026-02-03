# Data Model: Cost Estimate Command

**Feature**: 223-cost-estimate
**Date**: 2026-02-02

## Entity Definitions

### 1. EstimateResult

Aggregates the complete result of a what-if cost estimation.

**Location**: `internal/engine/types.go`

```go
// EstimateResult represents the result of a what-if cost estimation.
// It contains baseline and modified costs along with per-property deltas.
type EstimateResult struct {
    // Resource is the resource being estimated
    Resource *ResourceDescriptor

    // Baseline is the cost with original properties
    Baseline *CostResult

    // Modified is the cost with property overrides applied
    Modified *CostResult

    // TotalChange is the difference between modified and baseline monthly costs
    // Positive = increase, negative = savings
    TotalChange float64

    // Deltas contains per-property cost impact breakdown
    Deltas []CostDelta

    // UsedFallback indicates if EstimateCost RPC was unavailable
    // and the result was computed from two GetProjectedCost calls
    UsedFallback bool
}
```

**Validation Rules**:

- Resource MUST NOT be nil
- Baseline and Modified SHOULD be populated (may be nil on error)
- TotalChange = Modified.Monthly - Baseline.Monthly

### 2. CostDelta

Represents the cost impact of a single property change.

**Location**: `internal/engine/types.go`

```go
// CostDelta represents the cost impact of changing a single property.
type CostDelta struct {
    // Property is the name of the property that was changed
    Property string

    // OriginalValue is the value before the change
    OriginalValue string

    // NewValue is the value after the change
    NewValue string

    // CostChange is the monthly cost difference
    // Positive = increase, negative = savings
    CostChange float64
}
```

**Validation Rules**:

- Property MUST NOT be empty
- CostChange may be zero if property doesn't affect pricing

### 3. ResourceModification (Plan-Based)

Represents modifications to apply to a resource in a Pulumi plan.

**Location**: `internal/cli/cost_estimate.go`

```go
// ResourceModification captures property overrides for a specific resource.
type ResourceModification struct {
    // ResourceName is the Pulumi resource name or URN
    ResourceName string

    // Overrides is the map of property changes to apply
    Overrides map[string]string
}
```

### 4. EstimateRequest (Internal)

Internal request structure for engine layer.

**Location**: `internal/engine/estimate.go`

```go
// EstimateRequest encapsulates parameters for EstimateCost.
type EstimateRequest struct {
    // Resource is the base resource descriptor
    Resource *ResourceDescriptor

    // PropertyOverrides are the changes to evaluate
    PropertyOverrides map[string]string

    // UsageProfile optionally provides context (dev, prod, etc.)
    UsageProfile string
}
```

## Relationships

```text
EstimateResult
├── Resource → ResourceDescriptor (1:1, existing type)
├── Baseline → CostResult (1:1, existing type)
├── Modified → CostResult (1:1, existing type)
└── Deltas → CostDelta[] (1:N, new type)

ResourceModification (CLI layer)
├── ResourceName → string (identifies resource in plan)
└── Overrides → map[string]string (property changes)
```

## Existing Types Used (No Changes)

### ResourceDescriptor

Existing type representing a cloud resource.

**Location**: `internal/engine/types.go`

```go
type ResourceDescriptor struct {
    ID         string
    Provider   string
    Type       string
    Region     string
    Properties map[string]interface{}
}
```

### CostResult

Existing type for cost calculation results.

**Location**: `internal/engine/types.go`

```go
type CostResult struct {
    ResourceType string
    ResourceID   string
    Adapter      string
    Currency     string
    Monthly      float64
    Hourly       float64
    TotalCost    float64
    Notes        string
    Breakdown    map[string]float64
    StartDate    time.Time
    EndDate      time.Time
}
```

## Proto Types (From finfocus-spec v0.5.5)

### EstimateCostRequest

```protobuf
message EstimateCostRequest {
  ResourceDescriptor resource = 1;
  map<string, string> property_overrides = 2;
  UsageProfile usage_profile = 3;
}
```

### EstimateCostResponse

```protobuf
message EstimateCostResponse {
  CostResult baseline = 1;
  CostResult modified = 2;
  repeated CostDelta deltas = 3;
}

message CostDelta {
  string property = 1;
  string original_value = 2;
  string new_value = 3;
  double cost_change = 4;
}
```

## State Transitions

This command is stateless. No state machine or lifecycle transitions apply.

## Serialization

### JSON Output Format

```json
{
  "resource": {
    "provider": "aws",
    "type": "ec2:Instance",
    "region": "us-east-1"
  },
  "baseline": {
    "monthly": 8.32,
    "currency": "USD"
  },
  "modified": {
    "monthly": 83.22,
    "currency": "USD"
  },
  "totalChange": 74.90,
  "deltas": [
    {
      "property": "instanceType",
      "originalValue": "t3.micro",
      "newValue": "m5.large",
      "costChange": 65.70
    },
    {
      "property": "volumeSize",
      "originalValue": "8",
      "newValue": "100",
      "costChange": 9.20
    }
  ],
  "usedFallback": false
}
```

### NDJSON Output Format

One JSON object per line for streaming:

```json
{"resource":{"provider":"aws","type":"ec2:Instance"},"baseline":{"monthly":8.32},"modified":{"monthly":83.22},"totalChange":74.90,"deltas":[...]}
```
