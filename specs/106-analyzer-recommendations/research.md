# Research: Analyzer Recommendations Display

**Feature**: 106-analyzer-recommendations
**Date**: 2025-12-25
**Updated**: 2025-12-25 (Critical Architecture Finding)

## Executive Summary

This feature requires integration with the `GetRecommendations` RPC, which is
a **separate RPC call** from `GetProjectedCost`. The pluginsdk v0.4.10 provides
both the `GetRecommendations` RPC and `Recommendation` types, but the current
analyzer only calls `GetProjectedCost` - it does not fetch recommendations.

**Critical Finding**: The `GetProjectedCostResponse` proto does NOT include
recommendations. Recommendations must be fetched separately via the
`GetRecommendations` RPC and merged with cost results before diagnostic
formatting.

## Research Topics

### 0. Critical: GetRecommendations RPC Architecture (NEW)

**Finding**: The `GetProjectedCostResponse` proto message does NOT contain
recommendations. The proto definition shows:

```protobuf
message GetProjectedCostResponse {
  double unit_price = 1;
  string currency = 2;
  double cost_per_month = 3;
  string billing_detail = 4;
  repeated ImpactMetric impact_metrics = 5;
  // NO recommendations field!
}
```

**Required Integration**: The analyzer must call `GetRecommendations` RPC
separately and merge results with cost data.

**GetRecommendations RPC** (from pluginsdk v0.4.10):

```protobuf
rpc GetRecommendations(GetRecommendationsRequest) returns (GetRecommendationsResponse);

message GetRecommendationsRequest {
  RecommendationFilter filter = 1;
  int32 limit = 2;
  int32 offset = 3;
  string page_token = 4;
  repeated string excluded_recommendation_ids = 5;
}

message GetRecommendationsResponse {
  repeated Recommendation recommendations = 1;
  RecommendationSummary summary = 2;
  string next_page_token = 3;
  int32 total_count = 4;
}
```

**Integration Points Required**:

1. **Proto Adapter** (`internal/proto/adapter.go`):
   - Add `GetRecommendations` to `CostSourceClient` interface
   - Implement `GetRecommendations` in `clientAdapter`

2. **Plugin Host** (`internal/pluginhost/`):
   - Expose `GetRecommendations` through `Client.API`

3. **Engine** (`internal/engine/engine.go`):
   - Add `GetRecommendationsForResource` method
   - Merge recommendations into `CostResult.Recommendations`

4. **Analyzer Server** (`internal/analyzer/server.go`):
   - Call `GetRecommendations` after getting costs
   - Match recommendations to resources by resource ID/type

**Data Flow**:

```text
AnalyzeStack Request
    ↓
GetProjectedCost (per resource) → CostResult
    ↓
GetRecommendations (bulk or per-resource) → Recommendations
    ↓
Merge: CostResult.Recommendations = matched recommendations
    ↓
CostToDiagnostic (formats with recommendations)
    ↓
AnalyzeDiagnostic Response
```

### 1. Existing Recommendation Types in pluginsdk

**Decision**: Use simplified local `Recommendation` struct in
`engine.CostResult` that captures the essential display fields.

**Rationale**: The full protobuf `Recommendation` type from pluginsdk is
designed for the full `GetRecommendations` RPC with filtering, pagination,
and detailed action types. For diagnostic display, we need only:

- `Type` (string): e.g., "Right-sizing", "Terminate", "Purchase Commitment"
- `Description` (string): Actionable text, e.g., "Switch to t3.small"
- `EstimatedSavings` (float64): Monthly savings amount
- `Currency` (string): ISO currency code

**Alternatives Considered**:

1. **Embed full proto `Recommendation`**: Rejected - brings unnecessary
   complexity and proto dependencies into engine types
2. **Use `interface{}`**: Rejected - lacks type safety and documentation
3. **Create adapter layer**: Rejected - over-engineering for display-only use

### 2. Sustainability Metrics Pattern Analysis

**Decision**: Follow the existing sustainability metrics pattern in
`diagnostics.go:122-148`.

**Rationale**: The pattern is proven and provides:

- Conditional appending (only when data exists)
- Consistent bracket formatting `[...]`
- Deterministic ordering for testability
- Clean separation from base cost message

**Pattern to follow**:

```go
// Append recommendations if present
if len(cost.Recommendations) > 0 {
    var recParts []string
    for _, rec := range cost.Recommendations {
        part := formatRecommendation(rec)
        recParts = append(recParts, part)
    }
    message += fmt.Sprintf(" | Recommendations: %s",
        strings.Join(recParts, "; "))
}
```

### 3. Message Format Design

**Decision**: Use pipe-separated sections with semicolon-separated
recommendations.

**Rationale**: Maintains readability in terminal output while clearly
separating concerns.

**Format Examples**:

Single recommendation:

```text
Estimated Monthly Cost: $25.50 USD (source: aws-plugin) |
  Recommendations: Right-sizing: Switch to t3.small to save $15.00/mo
```

Multiple recommendations:

```text
Estimated Monthly Cost: $150.00 USD (source: aws-plugin) |
  Recommendations: Right-sizing: Switch to t3.medium to save $50.00/mo;
  Terminate: Remove idle instance to save $100.00/mo
```

No savings available:

```text
Estimated Monthly Cost: $25.50 USD (source: aws-plugin) |
  Recommendations: Review storage class configuration
```

### 4. Stack Summary Aggregation

**Decision**: Aggregate recommendation count and total savings in stack
summary.

**Rationale**: Provides quick visibility into optimization potential without
overwhelming the summary.

**Format**:

```text
Total Estimated Monthly Cost: $500.00 USD (10 resources analyzed) |
  3 recommendations, up to $125.00/mo potential savings
```

**Edge cases**:

- Mixed currencies: Show "3 recommendations (mixed currencies)"
- No savings data: Show "3 recommendations available"

### 5. Display Limit for Multiple Recommendations

**Decision**: Display up to 3 recommendations per resource; show "and N more"
for additional.

**Rationale**: Balances information density with readability. Three is enough
to show variety without overwhelming output.

**Implementation**:

```go
const maxRecommendationsToShow = 3

if len(recommendations) > maxRecommendationsToShow {
    displayed := recommendations[:maxRecommendationsToShow]
    remaining := len(recommendations) - maxRecommendationsToShow
    // ... format displayed ...
    recParts = append(recParts, fmt.Sprintf("and %d more", remaining))
}
```

### 6. E2E Testing Strategy

**Decision**: Extend existing `analyzer_e2e_test.go` with new test cases.

**Rationale**: The existing E2E infrastructure (`AnalyzerDiagnostic` pattern
matching, `runAnalyzerPreview` helper) supports this feature directly.

**New test cases**:

1. `TestAnalyzer_RecommendationDisplay`: Verify recommendations appear in
   diagnostic output
2. `TestAnalyzer_StackSummaryWithRecommendations`: Verify aggregate savings
   in summary

**Test fixture approach**: Modify the existing `fixtures/analyzer` Pulumi
project or create a mock plugin that returns recommendations.

## Dependencies Verified

| Dependency      | Version   | Purpose                                    |
| --------------- | --------- | ------------------------------------------ |
| finfocus-spec | v0.4.10   | `Recommendation` proto types (ref only)    |
| pulumi/sdk      | v3.210.0+ | Analyzer protocol                          |
| testify         | v1.11.1   | Unit/E2E test assertions                   |

## Risk Assessment

| Risk                     | Likelihood | Impact | Mitigation                |
| ------------------------ | ---------- | ------ | ------------------------- |
| Plugin no recs support   | High       | None   | Graceful, empty list      |
| GetRecommendations fails | Medium     | None   | Continue with costs only  |
| Message too long         | Low        | Low    | Truncation (3 max)        |
| Mixed currencies         | Medium     | Low    | Warning, skip totals      |
| Rec-resource match fail  | Medium     | Medium | Match by type/ID          |
| Extra RPC call perf      | Low        | Low    | Batch requests, timeout   |

## No NEEDS CLARIFICATION Items

All technical decisions resolved through codebase analysis:

1. Type structure → Use local simplified struct
2. Display format → Follow sustainability pattern
3. Aggregation → Count + total savings
4. Display limit → 3 recommendations max
5. E2E testing → Extend existing infrastructure
