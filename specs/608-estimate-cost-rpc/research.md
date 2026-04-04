# Research: EstimateCost RPC Consumer

**Feature**: 608-estimate-cost-rpc
**Date**: 2026-03-31

## Research Question 1: Proto Type Shape

**Task**: Determine the exact fields on `pbc.EstimateCostRequest` and
`pbc.EstimateCostResponse` in finfocus-spec v0.6.0.

**Decision**: Use the actual proto types as-is.

**Findings**:

### EstimateCostRequest (costsource.pb.go:3558)

| Field | Type | Description |
|-------|------|-------------|
| `ResourceType` | `string` | Pulumi resource type (e.g., `aws:ec2/instance:Instance`) |
| `Attributes` | `*structpb.Struct` | Resource properties as structured JSON |

### EstimateCostResponse (costsource.pb.go:3637)

| Field | Type | Description |
|-------|------|-------------|
| `Currency` | `string` | ISO 4217 code (e.g., `USD`) |
| `CostMonthly` | `float64` | Estimated monthly cost (730h basis) |
| `PricingCategory` | `FocusPricingCategory` | Standard/Committed/Dynamic |
| `SpotInterruptionRiskScore` | `float64` | 0.0-1.0 spot risk score |

**Rationale**: The proto is simpler than the original design document. It
provides a single estimation (not a baseline/modified pair), so the comparison
logic must live in the engine.

**Alternatives considered**: Wrapping proto types in internal types (rejected —
adds unnecessary indirection since proto is simple enough to use directly,
following the `GetBudgets`/`DryRun`/`Supports` pattern).

## Research Question 2: Attributes Field Construction

**Task**: Determine how to convert `map[string]interface{}` (engine
`ResourceDescriptor.Properties`) to `*structpb.Struct` (proto request).

**Decision**: Use `structpb.NewStruct()` from `google.golang.org/protobuf`.

**Findings**:

- `structpb.NewStruct(m map[string]interface{})` handles the conversion
- Already imported via `google.golang.org/protobuf/types/known/timestamppb`
  in the same package
- Need to add `"google.golang.org/protobuf/types/known/structpb"` import
- For `map[string]string` (property overrides), convert to
  `map[string]interface{}` before calling `structpb.NewStruct`

**Rationale**: Standard protobuf approach. No custom serialization needed.

**Alternatives considered**: Manual `structpb.Value` construction (rejected —
more code, same result).

## Research Question 3: Two-Call Strategy in tryEstimateCostRPC

**Task**: Determine the correct flow for what-if comparison using a
single-estimation proto.

**Decision**: `tryEstimateCostRPC` calls `EstimateCost` twice (baseline +
modified) and computes the delta.

**Findings**:

The flow is:

1. Build baseline proto request from `request.Resource.Properties`
2. Call `client.API.EstimateCost(ctx, baselineReq)` → baseline response
3. Deep copy resource properties, merge `request.PropertyOverrides`
4. Build modified proto request from merged properties
5. Call `client.API.EstimateCost(ctx, modifiedReq)` → modified response
6. Convert both responses to `engine.CostResult`
7. Compute `TotalChange = modified.Monthly - baseline.Monthly`
8. Build per-property deltas (single-property: attributed; multi-property:
   combined — same logic as `estimateCostFallback`)
9. Return `*EstimateResult` with `UsedFallback = false`

**Rationale**: Mirrors the existing `estimateCostFallback` strategy but uses
`EstimateCost` instead of `GetProjectedCost`. The advantage is that
`EstimateCost` is purpose-built for single-resource estimation with arbitrary
attributes, while `GetProjectedCost` requires a `ResourceDescriptor` with
SKU/region resolution.

**Alternatives considered**:
- Single RPC call returning baseline+modified (rejected — proto doesn't
  support it)
- Delegating delta computation to plugin (rejected — proto doesn't support it)

## Research Question 4: Internal Type Cleanup

**Task**: Determine which internal types in `adapter.go` become dead code.

**Decision**: Remove `EstimateCostRequest`, `EstimateCostResponse`, and
`CostDelta` from `adapter.go`. Also remove `ErrEstimateCostNotSupported`.

**Findings**:

- `EstimateCostRequest` (adapter.go:1277) — was designed for the original
  spec; `BuildEstimateCostRequest` now returns `*pbc.EstimateCostRequest`
- `EstimateCostResponse` (adapter.go:1289) — never used outside the stub;
  engine uses `EstimateResult` directly
- `CostDelta` (adapter.go:1301) — duplicates `engine.CostDelta` (types.go:591);
  engine type is authoritative
- `ErrEstimateCostNotSupported` (adapter.go:24-27) — sentinel error for the
  stub; no longer needed

**Rationale**: Dead code violates constitution Principle VI. Removing these
types eliminates confusion about which types are authoritative (engine types
are).

**Alternatives considered**: Keeping internal types as a mapping layer
(rejected — adds indirection without value since proto types are used
directly for the interface method).

## Research Question 5: Mock Updates

**Task**: Identify all mock implementations that need `EstimateCost` added.

**Decision**: Update `CostSourceClient` interface and all mock
implementations.

**Findings**:

Mocks that implement `CostSourceClient`:
- `adapter_test.go` — test mocks within the proto package
- `internal/engine/estimate_test.go` — engine test mocks
- Any other test files using the interface

The `mockPbcCostSourceServiceClient` in `adapter_test.go:3105` already has
`EstimateCost` (implements the generated `pbc.CostSourceServiceClient`).
Only the internal `CostSourceClient` interface mocks need the new method.

**Rationale**: Adding a method to an interface is a compile-breaking change.
All implementors must be updated.
