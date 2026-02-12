# Research: TUI Detail View Recommendations

## Decision 1: CLI-level Fetch and Merge

We will fetch recommendations in the CLI layer (e.g., in `executeCostProjected` and
`executeCostActual`) after cost results are returned from the engine. These
recommendations will then be merged into the `engine.CostResult` objects based on
`ResourceID`.

### Rationale

1. **Completeness**: Fetching in the CLI ensures that recommendations are available
   for all output formats (Interactive TUI, JSON, NDJSON, and Styled Table).
2. **Minimal Engine Impact**: No changes are required to the `Engine.GetProjectedCost`
   or `Engine.GetActualCost` methods, preserving their single-responsibility focus on
   cost calculation.
3. **Graceful Degradation**: By calling `GetRecommendationsForResources` separately,
   we can easily wrap it in error handling that logs failures but doesn't block the
   primary cost display workflow (FR-006).
4. **Reusability**: Leveraging the existing `GetRecommendationsForResources` method
   which already handles plugin orchestration, batching, caching, and dismissal
   filtering.

### Alternatives Considered

#### Fetching inside TUI `RenderDetailView`

**Rejected because**: `RenderDetailView` is a synchronous, pure rendering function
(`func(engine.CostResult, int) string`). Fetching data there would require it to be
asynchronous or use a complex message-passing pattern in Bubble Tea. It would also
bypass JSON/NDJSON output requirements (FR-007).

#### Modifying Engine to always include recommendations

**Rejected because**: This would increase the latency of every cost calculation call
even when recommendations aren't needed (e.g., in non-interactive scripts). It would
also complicate the `Engine` interface.

## Decision 2: Extend Recommendation Struct with Reasoning

The internal `engine.Recommendation` struct will be extended with a `Reasoning []string`
field, mapped from the proto `Recommendation.Reasoning` repeated field.

### Why This Approach

1. **Proto field available**: The proto `Recommendation` message already has a
   `Reasoning` repeated string field (field 14) that plugins can populate with
   warnings and caveats.
2. **Two-layer mapping required**:
   - **Adapter layer** (`internal/proto/adapter.go:862-888`): Maps proto
     `pbc.Recommendation` → internal `proto.Recommendation`. Must add `Reasoning`
     field to `proto.Recommendation` struct and populate from `rec.GetReasoning()`.
   - **Engine layer** (`internal/engine/engine.go:2633-2652`): Maps internal
     `proto.Recommendation` → `engine.Recommendation` via `convertProtoRecommendation`.
     Must copy `Reasoning` field.
3. **Minimal blast radius**: Only the conversion functions and the struct definition
   change; no protocol or plugin changes needed.

### Other Options Evaluated

#### Embedding warnings in the Description field

**Rejected because**: This conflates two distinct concerns (the recommendation itself
vs. caveats about implementing it). It would make JSON consumers unable to distinguish
between the core recommendation and its warnings.

## Decision 3: Sort by Estimated Savings Descending

Recommendations will be sorted by `EstimatedSavings` in descending order before
rendering. Recommendations with zero/missing savings maintain plugin-returned order
after the sorted ones.

### Why This Ordering

1. **Actionability**: Highest-savings recommendations appear first, directing user
   attention to the most impactful items.
2. **Stable sort**: Using `sort.SliceStable` preserves plugin-returned order for
   recommendations with equal savings values.

## Technical Details

### Conversion Pipeline (Current State)

```text
Plugin (gRPC) → pbc.Recommendation (proto)
    ↓ adapter.go:862-888 (GetRecommendations)
proto.Recommendation (internal proto types)
    ↓ engine.go:2633-2652 (convertProtoRecommendation)
engine.Recommendation (engine types)
    ↓ common_execution.go (fetchAndMergeRecommendations)
engine.CostResult.Recommendations (merged into cost results)
    ↓ cost_view.go (renderRecommendationsSection)
TUI output
```

### Key Files and Functions

| File | Function | Line | Purpose |
|------|----------|------|---------|
| `internal/proto/adapter.go` | `GetRecommendations` | ~862 | Proto → internal conversion |
| `internal/engine/engine.go` | `convertProtoRecommendation` | ~2633 | Internal → engine conversion |
| `internal/engine/engine.go` | `GetRecommendationsForResources` | ~2658 | Orchestrates plugin calls |
| `internal/cli/cost_projected.go` | `executeCostProjected` | ~96 | Projected cost CLI entry |
| `internal/cli/cost_actual.go` | `executeCostActual` | ~141 | Actual cost CLI entry |
| `internal/cli/common_execution.go` | (new) `fetchAndMergeRecommendations` | - | Shared helper |
| `internal/tui/cost_view.go` | `RenderDetailView` | ~275 | Detail view renderer |
| `internal/tui/cost_view.go` | (new) `renderRecommendationsSection` | - | Recommendations renderer |
| `internal/engine/types.go` | `Recommendation` struct | ~134 | Data model |

### Recommendation Fetching Logic

Helper function for `internal/cli/common_execution.go`:

```go
func fetchAndMergeRecommendations(ctx context.Context, eng *engine.Engine,
    resources []engine.ResourceDescriptor, results []engine.CostResult) {
    recsResult, err := eng.GetRecommendationsForResources(ctx, resources)
    if err != nil {
        logging.FromContext(ctx).Warn().Err(err).
            Msg("failed to fetch recommendations for detail view")
        return
    }

    // Map recommendations by ResourceID for fast lookup
    recMap := make(map[string][]engine.Recommendation)
    for _, rec := range recsResult.Recommendations {
        recMap[rec.ResourceID] = append(recMap[rec.ResourceID], rec)
    }

    // Merge into results
    for i := range results {
        if recs, found := recMap[results[i].ResourceID]; found {
            results[i].Recommendations = recs
        }
    }
}
```

### TUI Rendering Logic

In `internal/tui/cost_view.go`, a new `renderRecommendationsSection` function:

```go
func renderRecommendationsSection(content *strings.Builder,
    recommendations []engine.Recommendation) {
    if len(recommendations) == 0 {
        return
    }

    // Sort by estimated savings descending (FR-009)
    sorted := make([]engine.Recommendation, len(recommendations))
    copy(sorted, recommendations)
    sort.SliceStable(sorted, func(i, j int) bool {
        return sorted[i].EstimatedSavings > sorted[j].EstimatedSavings
    })

    content.WriteString(HeaderStyle.Render("RECOMMENDATIONS"))
    content.WriteString("\n")

    for _, rec := range sorted {
        savingsStr := ""
        if rec.EstimatedSavings > 0 {
            currency := rec.Currency
            if currency == "" {
                currency = "USD"
            }
            savingsStr = fmt.Sprintf(" ($%.2f %s/mo savings)",
                rec.EstimatedSavings, currency)
        }
        fmt.Fprintf(content, "- [%s] %s%s\n", rec.Type, rec.Description, savingsStr)

        // Render reasoning/caveats as indented warnings (FR-002)
        for _, reason := range rec.Reasoning {
            fmt.Fprintf(content, "    %s\n",
                WarningStyle.Render("⚠ "+reason))
        }
    }
    content.WriteString("\n")
}
```

### Dismissed Recommendation Handling

No special handling needed in this feature. The existing
`GetRecommendationsForResources` method already:

1. Loads the dismissal store via `loadExcludedRecommendationIDs` (engine.go:2697)
2. Passes `excludedIDs` to plugin requests (engine.go:2816-2818)
3. Plugins filter out dismissed/snoozed recommendations server-side

This means only active recommendations flow through to the detail view.

## Dependencies

- `engine.GetRecommendationsForResources` must be functional (verified: it is).
- `engine.CostResult` must have the `Recommendations` field (verified: it does).
- Proto `Recommendation.Reasoning` field must exist (verified: field 14 in
  costsource.pb.go).
- `internal/proto.Recommendation` struct needs `Reasoning` field added.
