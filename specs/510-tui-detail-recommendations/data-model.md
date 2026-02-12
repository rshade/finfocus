# Data Model: TUI Detail View Recommendations

This feature leverages existing data structures in `internal/engine/types.go`.

## Entities

### Recommendation

Represents a single cost optimization suggestion.

```go
type Recommendation struct {
    ResourceID       string               `json:"resourceId,omitempty"`
    Type             string               `json:"type"`             // e.g., "RIGHTSIZE", "TERMINATE"
    Description      string               `json:"description"`      // Human-readable explanation
    EstimatedSavings float64              `json:"estimatedSavings,omitempty"`
    Currency         string               `json:"currency,omitempty"`
    Status           RecommendationStatus `json:"status,omitempty"` // "Active", "Dismissed", "Snoozed"
    Reasoning        []string             `json:"reasoning,omitempty"` // NEW: Warnings/caveats from proto
}
```

**New field**: `Reasoning` is mapped from the proto `Recommendation.Reasoning` repeated
field via the adapter (`internal/proto/adapter.go`). It carries plugin-provided warnings
and caveats (e.g., "Ensure application compatibility with ARM64 before migrating to
Graviton"). Empty when plugins provide no reasoning entries.

### CostResult

The core result structure for cost calculations, now enriched with recommendations.

```go
type CostResult struct {
    ResourceType    string                          `json:"resourceType"`
    ResourceID      string                          `json:"resourceId"`
    Adapter         string                          `json:"adapter"`
    Currency        string                          `json:"currency"`
    Monthly         float64                         `json:"monthly"`
    Hourly          float64                         `json:"hourly"`
    Notes           string                          `json:"notes"`
    Breakdown       map[string]float64              `json:"breakdown"`
    Sustainability  map[string]SustainabilityMetric `json:"sustainability,omitempty"`
    Recommendations []Recommendation                `json:"recommendations,omitempty"` // ENRICHED FIELD
    
    // Actual cost specific fields
    TotalCost  float64   `json:"totalCost,omitempty"`
    // ... other fields
}
```

## Relationships

- A `CostResult` contains zero or more `Recommendation` objects.
- Recommendations are matched to `CostResult` via `ResourceID`.

## Ordering

- **FR-009**: Recommendations are sorted by `EstimatedSavings` descending (highest
  savings first). Recommendations with zero or missing savings appear last in
  plugin-returned order.

## Validation Rules

- **FR-004**: If `EstimatedSavings` is 0 or less, it should not be displayed in the TUI.
- **FR-005**: If `Recommendations` is empty or nil, the section should not be rendered.
- **FR-002**: If `Reasoning` is non-empty, each entry is rendered as an indented line
  beneath the recommendation description.
