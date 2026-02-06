# Implementation Plan: Analyzer Recommendations Display

**Branch**: `106-analyzer-recommendations` | **Date**: 2025-12-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/106-analyzer-recommendations/spec.md`

## Summary

Add cost optimization recommendations to Pulumi Analyzer diagnostics. This
requires calling the `GetRecommendations` RPC (separate from `GetProjectedCost`)
to fetch recommendations from plugins, then merging them with cost results
before formatting for display in `pulumi preview` output.

**Critical Architecture Note**: The `GetProjectedCostResponse` proto does NOT
include recommendations. The analyzer must:

1. Call `GetProjectedCost` for cost estimates (existing)
2. Call `GetRecommendations` for optimization suggestions (NEW)
3. Merge recommendations with cost results by resource ID/type
4. Format combined data in diagnostics

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: github.com/rshade/finfocus-spec v0.4.10 (pluginsdk,
protobuf types), github.com/pulumi/pulumi/sdk/v3 (Analyzer protocol)
**Storage**: N/A (display-only feature)
**Testing**: go test with testify, E2E tests in test/e2e/
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project (Go CLI tool)
**Performance Goals**: No measurable overhead (string formatting only)
**Constraints**: ADVISORY enforcement level only (never block deployments)
**Scale/Scope**: Affects analyzer diagnostics output; minimal code changes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution:

- [x] **Plugin-First Architecture**: Feature displays data from plugins; no
  direct provider integration
- [x] **Test-Driven Development**: Unit tests for diagnostics + E2E analyzer
  tests required (80% minimum coverage)
- [x] **Cross-Platform Compatibility**: Pure Go, no platform-specific code
- [x] **Documentation as Code**: Inline code documentation for new types/funcs
- [x] **Protocol Stability**: Uses existing pluginsdk types; no protocol changes
- [x] **Implementation Completeness**: Full implementation required, no stubs
- [x] **Quality Gates**: make lint + make test before completion
- [x] **Multi-Repo Coordination**: No cross-repo changes needed (pluginsdk
  already has Recommendation types)

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/106-analyzer-recommendations/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no API contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── proto/
│   ├── adapter.go           # Add GetRecommendations to CostSourceClient
│   └── adapter_test.go      # Add tests for GetRecommendations adapter
├── pluginhost/
│   └── client.go            # Expose GetRecommendations via Client.API
├── engine/
│   ├── types.go             # Add Recommendation struct and CostResult field
│   ├── engine.go            # Add GetRecommendationsForResources method
│   └── engine_test.go       # Add tests for recommendation fetching
├── analyzer/
│   ├── server.go            # Call GetRecommendations, merge with costs
│   ├── server_test.go       # Add tests for recommendation integration
│   ├── diagnostics.go       # Update formatCostMessage, StackSummaryDiagnostic
│   └── diagnostics_test.go  # Add recommendation formatting tests

test/
├── e2e/
│   └── analyzer_e2e_test.go  # Add recommendation E2E tests
```

**Structure Decision**: This feature requires changes across multiple layers:

1. **Proto Adapter Layer**: Add `GetRecommendations` RPC support
2. **Engine Layer**: Fetch and merge recommendations with costs
3. **Analyzer Layer**: Orchestrate recommendation fetching in AnalyzeStack
4. **Diagnostics Layer**: Format recommendations (already implemented)

The diagnostic formatting follows the existing sustainability pattern in
`diagnostics.go:122-148`.

## Complexity Tracking

> Medium complexity - requires integration across proto, engine, and analyzer
> layers with a new RPC call.

## Architecture: GetRecommendations Integration

### Data Flow

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                         AnalyzeStack Request                            │
│                    (resources from pulumi preview)                      │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Engine.GetProjectedCost (existing)                   │
│                    Returns: []CostResult (no recommendations)           │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Engine.GetRecommendations (NEW)                      │
│    For each plugin: call GetRecommendations RPC                         │
│    Returns: map[resourceID][]Recommendation                             │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    MergeRecommendations (NEW)                           │
│    Match recommendations to CostResults by resource ID/type             │
│    Populate CostResult.Recommendations field                            │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    CostToDiagnostic (existing, enhanced)                │
│    Formats cost + recommendations into AnalyzeDiagnostic                │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         AnalyzeStack Response                           │
│                    (diagnostics with recommendations)                   │
└─────────────────────────────────────────────────────────────────────────┘
```

### Proto Adapter Changes

**File**: `internal/proto/adapter.go`

```go
// Add to CostSourceClient interface
type CostSourceClient interface {
    // ... existing methods ...
    GetRecommendations(
        ctx context.Context,
        in *GetRecommendationsRequest,
        opts ...grpc.CallOption,
    ) (*GetRecommendationsResponse, error)
}

// New request/response types
type GetRecommendationsRequest struct {
    ResourceTypes []string           // Filter by resource types
    ResourceIDs   []string           // Filter by resource IDs
    Limit         int32              // Max recommendations per resource
}

type GetRecommendationsResponse struct {
    Recommendations []ProtoRecommendation
}

type ProtoRecommendation struct {
    ResourceID       string
    ResourceType     string
    Type             string   // e.g., "Right-sizing", "Terminate"
    Description      string
    EstimatedSavings float64
    Currency         string
}
```

### Engine Changes

**File**: `internal/engine/engine.go`

```go
// GetRecommendationsForResources fetches recommendations for a list of resources
// from all connected plugins.
func (e *Engine) GetRecommendationsForResources(
    ctx context.Context,
    resources []ResourceDescriptor,
) (map[string][]Recommendation, error) {
    // 1. Extract resource IDs and types
    // 2. Call GetRecommendations on each plugin
    // 3. Aggregate and dedupe recommendations
    // 4. Return map[resourceID][]Recommendation
}

// MergeRecommendations populates CostResult.Recommendations from the map
func MergeRecommendations(
    costs []CostResult,
    recommendations map[string][]Recommendation,
) []CostResult {
    // Match by ResourceID, fallback to ResourceType
}
```

### Analyzer Server Changes

**File**: `internal/analyzer/server.go`

Update `AnalyzeStack` to:

1. Call `GetProjectedCost` (existing)
2. Call `GetRecommendationsForResources` (NEW)
3. Call `MergeRecommendations` (NEW)
4. Pass merged results to `CostToDiagnostic`

### Graceful Degradation

- If `GetRecommendations` RPC fails: log warning, continue with costs only
- If plugin doesn't support recommendations: returns empty list
- If no recommendations match resources: diagnostics show costs only
- ADVISORY enforcement: never block deployments

### Performance Considerations

- Batch recommendation requests where possible
- Set reasonable timeout (5s) for recommendation calls
- Cache recommendations per resource type (future optimization)
