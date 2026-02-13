# Data Model: Unified Cost Overview Dashboard

**Feature**: Overview Command  
**Date**: 2026-02-11  
**Based On**: research.md decisions

---

## Overview

The overview command merges data from three sources (Pulumi state, Pulumi preview, and plugin cost APIs) into a unified resource-centric view. This document defines the data structures, relationships, and validation rules.

---

## Entity Relationship Diagram

```
┌─────────────────────┐
│   StackContext      │
│──────────────────────│
│ StackName  string    │
│ Region     string    │
│ TimeWindow DateRange │
│ HasChanges bool      │
└──────────┬───────────┘
           │ 1
           │
           │ *
┌──────────▼───────────┐
│   OverviewRow        │◄────────┐
│──────────────────────│         │
│ URN         string   │         │
│ Type        string   │         │
│ ResourceID  string   │         │
│ Status      enum     │         │
│ ActualCost  *Data    │───┐     │
│ ProjectedCost *Data  │───┼─┐   │
│ Recommendations []   │───┼─┼─┐ │
│ CostDrift   *Data    │   │ │ │ │
│ Error       *Error   │───┼─┼─┼─┘
└──────────────────────┘   │ │ │
                           │ │ │
        ┌──────────────────┘ │ │
        │                    │ │
┌───────▼────────┐  ┌────────▼──────────┐  ┌─────────▼────────┐
│ ActualCostData │  │ ProjectedCostData │  │ Recommendation   │
│────────────────│  │───────────────────│  │──────────────────│
│ MTDCost   float│  │ MonthlyCost float │  │ ResourceID string│
│ Currency string│  │ Currency   string │  │ Type      string │
│ Period DateRange│  │ Breakdown  map    │  │ Description text │
│ Breakdown  map │  └───────────────────┘  │ Savings    float │
└────────────────┘                         │ Currency  string │
                                           └──────────────────┘
```

---

## Core Entities

### 1. OverviewRow

**Purpose**: Unified representation of a single infrastructure resource with all cost facets.

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| URN | string | Yes | Pulumi URN format | Canonical resource identifier |
| Type | string | Yes | Non-empty, max 256 chars | Pulumi resource type (e.g., "aws:ec2/instance:Instance") |
| ResourceID | string | No | Max 1024 chars | Cloud provider resource ID |
| Status | ResourceStatus | Yes | Enum value | Current lifecycle status |
| ActualCost | *ActualCostData | No | Nil if no historical data | Month-to-date actual costs |
| ProjectedCost | *ProjectedCostData | No | Nil if no changes pending | Projected monthly costs after changes |
| Recommendations | []Recommendation | No | Empty slice if none | Cost optimization suggestions |
| CostDrift | *CostDriftData | No | Nil if drift <10% or data missing | Warning for cost prediction accuracy |
| Error | *OverviewRowError | No | Nil if no error | Error state for this resource |

**Relationships**:
- 1 OverviewRow : 0..1 ActualCostData
- 1 OverviewRow : 0..1 ProjectedCostData
- 1 OverviewRow : 0..* Recommendation
- 1 OverviewRow : 0..1 CostDriftData
- 1 OverviewRow : 0..1 OverviewRowError

**Validation Rules**:
```go
func (o *OverviewRow) Validate() error {
    if o.URN == "" {
        return errors.New("URN is required")
    }
    if o.Type == "" {
        return errors.New("Type is required")
    }
    if len(o.Type) > 256 {
        return errors.New("Type exceeds 256 characters")
    }
    if len(o.ResourceID) > 1024 {
        return errors.New("ResourceID exceeds 1024 characters")
    }
    if o.Status < StatusActive || o.Status > StatusReplacing {
        return errors.New("invalid Status value")
    }
    return nil
}
```

**State Transitions**:
```
[Non-existent] ──create──> [StatusCreating] ──success──> [StatusActive]
                                           └──fail────> [Error]

[StatusActive] ──update──> [StatusUpdating] ──success──> [StatusActive]
                                           └──fail────> [Error]

[StatusActive] ──delete──> [StatusDeleting] ──success──> [Non-existent]

[StatusActive] ──replace─> [StatusReplacing] ──success──> [StatusActive]
                                            └──fail────> [Error]
```

---

### 2. ResourceStatus

**Purpose**: Enumerate possible lifecycle states of a resource.

**Type**: Enum (Go `int` with constants)

**Values**:

| Value | Constant | Description | Preview Op |
|-------|----------|-------------|------------|
| 0 | StatusActive | Resource exists, no pending changes | "same" or absent from preview |
| 1 | StatusCreating | Resource being created | "create" |
| 2 | StatusUpdating | Resource being modified | "update" |
| 3 | StatusDeleting | Resource being deleted | "delete" |
| 4 | StatusReplacing | Resource being replaced (delete + create) | "replace" |

**Mapping Logic**:
```go
func MapOperationToStatus(op string) ResourceStatus {
    switch op {
    case "create":
        return StatusCreating
    case "update":
        return StatusUpdating
    case "delete":
        return StatusDeleting
    case "replace":
        return StatusReplacing
    default:
        return StatusActive
    }
}
```

---

### 3. ActualCostData

**Purpose**: Historical actual costs for a resource (retrieved from cloud billing APIs or estimated from state).

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| MTDCost | float64 | Yes | >= 0.0 | Month-to-date total cost |
| Currency | string | Yes | ISO 4217 code (3 chars) | Currency code (USD, EUR, etc.) |
| Period | DateRange | Yes | Valid date range | Time window for cost data |
| Breakdown | map[string]float64 | No | All values >= 0.0 | Cost by category (compute, storage, network) |

**Validation Rules**:
```go
func (a *ActualCostData) Validate() error {
    if a.MTDCost < 0 {
        return errors.New("MTDCost cannot be negative")
    }
    if len(a.Currency) != 3 {
        return errors.New("Currency must be 3-character ISO 4217 code")
    }
    if err := a.Period.Validate(); err != nil {
        return fmt.Errorf("invalid Period: %w", err)
    }
    for category, cost := range a.Breakdown {
        if cost < 0 {
            return fmt.Errorf("Breakdown[%s] cannot be negative", category)
        }
    }
    return nil
}
```

**Example**:
```json
{
  "MTDCost": 42.50,
  "Currency": "USD",
  "Period": {
    "Start": "2026-02-01T00:00:00Z",
    "End": "2026-02-11T23:59:59Z"
  },
  "Breakdown": {
    "compute": 30.00,
    "storage": 10.00,
    "network": 2.50
  }
}
```

---

### 4. ProjectedCostData

**Purpose**: Estimated monthly costs for a resource after pending infrastructure changes.

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| MonthlyCost | float64 | Yes | >= 0.0 | Projected full-month cost |
| Currency | string | Yes | ISO 4217 code (3 chars) | Currency code |
| Breakdown | map[string]float64 | No | All values >= 0.0 | Cost by category |

**Validation Rules**:
```go
func (p *ProjectedCostData) Validate() error {
    if p.MonthlyCost < 0 {
        return errors.New("MonthlyCost cannot be negative")
    }
    if len(p.Currency) != 3 {
        return errors.New("Currency must be 3-character ISO 4217 code")
    }
    for category, cost := range p.Breakdown {
        if cost < 0 {
            return fmt.Errorf("Breakdown[%s] cannot be negative", category)
        }
    }
    return nil
}
```

**Example**:
```json
{
  "MonthlyCost": 100.00,
  "Currency": "USD",
  "Breakdown": {
    "compute": 70.00,
    "storage": 25.00,
    "network": 5.00
  }
}
```

---

### 5. CostDriftData

**Purpose**: Warning indicator when actual spending significantly differs from projected estimates.

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| ExtrapolatedMonthly | float64 | Yes | >= 0.0 | Actual MTD extrapolated to full month |
| Projected | float64 | Yes | >= 0.0 | Original projected monthly cost |
| Delta | float64 | Yes | No constraint | Difference (extrapolated - projected) |
| PercentDrift | float64 | Yes | Abs value > 10.0 | Percent difference |
| IsWarning | bool | Yes | Must be true | Always true if drift exists |

**Validation Rules**:
```go
func (c *CostDriftData) Validate() error {
    if c.ExtrapolatedMonthly < 0 {
        return errors.New("ExtrapolatedMonthly cannot be negative")
    }
    if c.Projected < 0 {
        return errors.New("Projected cannot be negative")
    }
    if math.Abs(c.PercentDrift) <= 10.0 {
        return errors.New("PercentDrift must exceed 10% threshold")
    }
    if !c.IsWarning {
        return errors.New("IsWarning must be true")
    }
    return nil
}
```

**Calculation Example**:
```go
// Day 11 of month, actual MTD cost = $42.50, projected = $100.00
daysInMonth := 30
dayOfMonth := 11

extrapolated := 42.50 * (30.0 / 11.0) = $115.91
delta := 115.91 - 100.00 = $15.91
percentDrift := (15.91 / 100.00) * 100 = 15.91%

// Drift exceeds 10% threshold → CostDriftData created
```

**Edge Case Handling**:
```go
func CalculateCostDrift(actual, projected float64, dayOfMonth, daysInMonth int) (*CostDriftData, error) {
    // Early in month: unreliable extrapolation
    if dayOfMonth <= 2 {
        return nil, errors.New("insufficient data (day 1-2 of month)")
    }
    
    // Deleted resources: don't calculate drift
    if projected == 0.0 && actual > 0.0 {
        return nil, nil // no drift
    }
    
    // New resources: don't calculate drift
    if actual == 0.0 && projected > 0.0 {
        return nil, nil // no drift
    }
    
    extrapolated := actual * (float64(daysInMonth) / float64(dayOfMonth))
    delta := extrapolated - projected
    percentDrift := (delta / projected) * 100
    
    if math.Abs(percentDrift) > 10.0 {
        return &CostDriftData{
            ExtrapolatedMonthly: extrapolated,
            Projected:          projected,
            Delta:              delta,
            PercentDrift:       percentDrift,
            IsWarning:          true,
        }, nil
    }
    
    return nil, nil // no drift
}
```

---

### 6. Recommendation

**Purpose**: Cost optimization suggestion from a plugin (reuses existing `engine.Recommendation`).

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| ResourceID | string | No | Max 1024 chars | Target resource (may be empty for stack-level) |
| Type | string | Yes | Non-empty | Action type (RIGHTSIZE, TERMINATE, etc.) |
| Description | string | Yes | Non-empty, max 2048 chars | Human-readable explanation |
| EstimatedSavings | float64 | Yes | >= 0.0 | Potential monthly savings |
| Currency | string | Yes | ISO 4217 code (3 chars) | Currency code |

**Validation Rules**: Reuse existing `engine.Recommendation.Validate()`.

**Example**:
```json
{
  "ResourceID": "i-0123456789abcdef0",
  "Type": "RIGHTSIZE",
  "Description": "Instance is underutilized (5% CPU avg). Consider downsizing to t3.medium.",
  "EstimatedSavings": 50.00,
  "Currency": "USD"
}
```

---

### 7. OverviewRowError

**Purpose**: Error state when cost data cannot be fetched for a resource.

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| URN | string | Yes | Pulumi URN format | Resource identifier |
| ErrorType | ErrorType | Yes | Enum value | Error category |
| Message | string | Yes | Non-empty, max 2048 chars | Human-readable error |
| Retryable | bool | Yes | No constraint | Whether retry is possible |

**ErrorType Enum**:

| Value | Constant | Description | Retryable |
|-------|----------|-------------|-----------|
| 0 | ErrorTypeAuth | Authentication failure | No |
| 1 | ErrorTypeNetwork | Network/connectivity issue | Yes |
| 2 | ErrorTypeRateLimit | API rate limit exceeded | Yes |
| 3 | ErrorTypeUnknown | Unclassified error | Maybe |

**Validation Rules**:
```go
func (e *OverviewRowError) Validate() error {
    if e.URN == "" {
        return errors.New("URN is required")
    }
    if e.Message == "" {
        return errors.New("Message is required")
    }
    if len(e.Message) > 2048 {
        return errors.New("Message exceeds 2048 characters")
    }
    if e.ErrorType < ErrorTypeAuth || e.ErrorType > ErrorTypeUnknown {
        return errors.New("invalid ErrorType value")
    }
    return nil
}
```

---

### 8. DateRange

**Purpose**: Time window for cost data queries.

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| Start | time.Time | Yes | Before End | Start of period (inclusive) |
| End | time.Time | Yes | After Start | End of period (inclusive) |

**Validation Rules**:
```go
func (d *DateRange) Validate() error {
    if d.Start.IsZero() {
        return errors.New("Start is required")
    }
    if d.End.IsZero() {
        return errors.New("End is required")
    }
    if d.End.Before(d.Start) {
        return errors.New("End must be after Start")
    }
    return nil
}
```

---

### 9. StackContext

**Purpose**: Metadata about the Pulumi stack being analyzed.

**Fields**:

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| StackName | string | Yes | Non-empty | Pulumi stack identifier |
| Region | string | No | Max 128 chars | Primary cloud region |
| TimeWindow | DateRange | Yes | Valid range | Analysis period |
| HasChanges | bool | Yes | No constraint | Whether pending changes exist |
| TotalResources | int | Yes | >= 0 | Total resource count |
| PendingChanges | int | No | >= 0 if HasChanges | Count of resources with changes |

**Validation Rules**:
```go
func (s *StackContext) Validate() error {
    if s.StackName == "" {
        return errors.New("StackName is required")
    }
    if len(s.Region) > 128 {
        return errors.New("Region exceeds 128 characters")
    }
    if err := s.TimeWindow.Validate(); err != nil {
        return fmt.Errorf("invalid TimeWindow: %w", err)
    }
    if s.TotalResources < 0 {
        return errors.New("TotalResources cannot be negative")
    }
    if s.HasChanges && s.PendingChanges <= 0 {
        return errors.New("PendingChanges must be positive if HasChanges is true")
    }
    return nil
}
```

---

## Derived Data

### Projected Delta

**Purpose**: Net change in monthly spend if pending changes are applied.

**Calculation**:
```go
func CalculateProjectedDelta(rows []OverviewRow) float64 {
    var delta float64
    for _, row := range rows {
        if row.ProjectedCost != nil && row.ActualCost != nil {
            // Extrapolate actual to full month for comparison
            daysInMonth := 30 // simplified
            dayOfMonth := time.Now().Day()
            extrapolatedActual := row.ActualCost.MTDCost * (float64(daysInMonth) / float64(dayOfMonth))
            
            delta += row.ProjectedCost.MonthlyCost - extrapolatedActual
        } else if row.ProjectedCost != nil {
            // New resource (no actual cost)
            delta += row.ProjectedCost.MonthlyCost
        } else if row.ActualCost != nil && row.Status == StatusDeleting {
            // Deleted resource
            extrapolatedActual := row.ActualCost.MTDCost * (float64(daysInMonth) / float64(dayOfMonth))
            delta -= extrapolatedActual
        }
    }
    return delta
}
```

---

## Data Flow

### 1. Load Phase

```
[Pulumi State File]     [Pulumi Preview File]
       |                        |
       v                        v
  LoadStackExport()      LoadPulumiPlan()
       |                        |
       v                        v
  StackExport            PulumiPlan
       |                        |
       └────────┬───────────────┘
                v
        MergeResourcesForOverview()
                |
                v
          []OverviewRow (Status populated, costs nil)
```

### 2. Enrichment Phase

```
[]OverviewRow (skeleton)
       |
       ├──> Goroutine 1: FetchActualCost(URN) ──> ActualCostData
       ├──> Goroutine 2: FetchProjectedCost(URN) ──> ProjectedCostData
       ├──> Goroutine 3: FetchRecommendations(URN) ──> []Recommendation
       └──> Main: CalculateCostDrift() ──> CostDriftData
       |
       v
[]OverviewRow (fully populated)
```

### 3. Display Phase

```
[]OverviewRow
       |
       ├──> Interactive: TUI Model (Bubble Tea)
       └──> Non-Interactive: ASCII Table Renderer
```

---

## Persistence

**Storage**: None. All data is ephemeral and computed on-demand.

**Caching**: Reuse existing `internal/engine/cache` for actual cost queries (respects `--cache-ttl` flag).

**Audit Trail**: All queries logged to `~/.finfocus/logs/` per existing audit system.

---

## Performance Considerations

### Memory Estimates

| Stack Size | OverviewRow Count | Estimated Memory | Notes |
|------------|------------------|------------------|-------|
| Small | 10 resources | ~5 KB | Minimal |
| Medium | 100 resources | ~50 KB | Typical |
| Large | 250 resources | ~125 KB | Pagination threshold |
| Very Large | 500 resources | ~250 KB | 2 pages |
| Extreme | 1000 resources | ~500 KB | 4 pages, consider filtering |

**Assumptions**:
- ~500 bytes per OverviewRow (with all fields populated)
- Breakdown maps: ~10 entries avg
- Recommendations: ~2 per resource avg

### Query Parallelism

- **Concurrent queries**: 10 resources at a time (existing engine limit)
- **Batch size**: 100 resources per progress update
- **Timeout**: 30 seconds per resource query (existing timeout)

---

## Validation Summary

All entities MUST implement `Validate() error` method for use in:
- Unit tests (table-driven validation tests)
- Integration tests (end-to-end flow)
- Runtime checks (before rendering UI)

**Test Coverage Target**: 95% for validation logic (critical path per constitution).

---

## References

- **Research**: `specs/509-overview-command/research.md`
- **Existing Types**: `internal/engine/types.go`
- **Ingest Layer**: `internal/ingest/state.go`, `internal/ingest/pulumi_plan.go`
- **Constitution**: `.specify/memory/constitution.md` (Principle VI: No Stubs)

---

**Data Model Completed**: 2026-02-11  
**Next Step**: Generate API contracts in `contracts/` directory
