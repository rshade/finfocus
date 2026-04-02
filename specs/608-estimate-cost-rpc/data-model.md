# Data Model: EstimateCost RPC Consumer

**Feature**: 608-estimate-cost-rpc
**Date**: 2026-03-31

## Entity Overview

This feature connects existing entities — no new data models are introduced.
The key mapping is between engine-level types and proto-level types.

## Proto Types (finfocus-spec v0.6.0, read-only)

### pbc.EstimateCostRequest

```text
EstimateCostRequest
├── ResourceType    string           # Pulumi type token
└── Attributes      *structpb.Struct # Resource properties
```

### pbc.EstimateCostResponse

```text
EstimateCostResponse
├── Currency                  string                # ISO 4217
├── CostMonthly               float64               # Monthly cost (730h)
├── PricingCategory            FocusPricingCategory  # Standard/Committed/Dynamic
└── SpotInterruptionRiskScore  float64               # 0.0-1.0
```

## Engine Types (existing, unchanged)

### engine.EstimateRequest (types.go:606)

```text
EstimateRequest
├── Resource           *ResourceDescriptor  # Base resource
└── PropertyOverrides  map[string]string    # Changes to evaluate
```

### engine.EstimateResult (types.go:559)

```text
EstimateResult
├── Resource      *ResourceDescriptor
├── Baseline      *CostResult          # Cost with original properties
├── Modified      *CostResult          # Cost with overrides applied
├── TotalChange   float64              # Modified.Monthly - Baseline.Monthly
├── Deltas        []CostDelta          # Per-property cost impact
└── UsedFallback  bool                 # false when RPC succeeded
```

### engine.CostResult (types.go — relevant fields)

```text
CostResult
├── Currency       string
├── MonthlyCost    float64    # Mapped from pbc.CostMonthly
├── HourlyCost     float64    # Derived: CostMonthly / 730
├── Notes          string
├── CostBreakdown  map[string]float64
├── Sustainability map[string]SustainabilityMetric
└── ExpiresAt      *time.Time  # Not on EstimateCostResponse proto
```

## Type Mapping: Proto → Engine

### EstimateCostResponse → CostResult

| Proto Field | Engine Field | Transformation |
|-------------|-------------|----------------|
| `Currency` | `Currency` | Direct copy |
| `CostMonthly` | `MonthlyCost` | Direct copy |
| `CostMonthly` | `HourlyCost` | `CostMonthly / 730` |
| `PricingCategory` | `Notes` | Append category name |
| `SpotInterruptionRiskScore` | `Notes` | Append if > 0 |
| (not present) | `ExpiresAt` | nil (EstimateCost proto lacks expires_at) |

### ResourceDescriptor → EstimateCostRequest

| Engine Field | Proto Field | Transformation |
|-------------|-------------|----------------|
| `Type` | `ResourceType` | Direct copy |
| `Properties` | `Attributes` | `structpb.NewStruct(properties)` |

## Types Removed (dead code)

These internal adapter types become unused after stub replacement:

- `proto.EstimateCostRequest` (adapter.go:1277) — replaced by `pbc.EstimateCostRequest`
- `proto.EstimateCostResponse` (adapter.go:1289) — engine uses `EstimateResult` directly
- `proto.CostDelta` (adapter.go:1301) — duplicates `engine.CostDelta`
- `proto.ErrEstimateCostNotSupported` (adapter.go:24) — sentinel error for stub

## Validation Rules

### BuildEstimateCostRequest Input Validation

- `ResourceDescriptor` MUST NOT be nil
- `ResourceDescriptor.Type` MUST NOT be empty
- `Properties` MAY be nil (treated as empty struct)

### EstimateCostResponse Output Validation

- Response MUST NOT be nil
- `CostMonthly` MUST be >= 0 (negative costs are invalid)
- `Currency` SHOULD NOT be empty (default to "USD" if missing)
