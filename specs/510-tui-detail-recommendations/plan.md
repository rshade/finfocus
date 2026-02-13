# Implementation Plan: TUI Detail View Recommendations

**Branch**: `510-tui-detail-recommendations` | **Date**: 2026-02-12
**Spec**: `specs/510-tui-detail-recommendations/spec.md`
**GitHub Issue**: #575

## Summary

Display cost optimization recommendations directly in the resource detail view of the
interactive TUI for both projected and actual cost commands. This eliminates the need
for users to switch to a separate recommendations command. The implementation involves:

1. Extending the `Recommendation` struct with a `Reasoning` field
2. Updating the proto adapter to carry reasoning from plugins
3. Adding a shared `fetchAndMergeRecommendations` helper in the CLI layer
4. Adding a `renderRecommendationsSection` function to the TUI detail view
5. Wiring the fetch-and-merge into both `executeCostProjected` and `executeCostActual`

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: `bubbletea` v1.3.10, `lipgloss` v1.1.0, `finfocus-spec`
v0.5.5, `zerolog` v1.34.0, `testify` v1.11.1
**Storage**: N/A (no new persistence)
**Testing**: Go testing with `testify/require` and `testify/assert`
**Target Platform**: Linux, macOS, Windows
**Project Type**: CLI (TUI enhancement)
**Performance Goals**: Best-effort recommendation fetching; must not block or delay
cost display significantly (FR-006).
**Constraints**: Failures in recommendation fetching are handled silently from user
perspective.
**Scale/Scope**: Enhancing existing TUI detail view and CLI cost execution flows.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **I. Plugin-First Architecture**: Recommendations are fetched via established
  plugin-first engine architecture. No direct provider integrations added.
- [x] **II. Test-Driven Development**: Unit tests planned for: TUI rendering (sort,
  reasoning display, empty state), CLI merge logic, adapter Reasoning mapping.
  Target 80%+ coverage on new code.
- [x] **III. Cross-Platform Compatibility**: Uses standard Go and cross-platform TUI
  libraries (bubbletea, lipgloss). No platform-specific code.
- [x] **IV. Documentation Integrity**: Quickstart guide, data model docs, and CLAUDE.md
  updated. Godoc comments on all new exported symbols.
- [x] **V. Protocol Stability**: No changes to gRPC protocol. Uses existing
  `Recommendation.Reasoning` field (proto field 14) already defined in finfocus-spec.
- [x] **VI. Implementation Completeness**: Full implementation of all 10 functional
  requirements. No TODOs or stubs.
- [x] **Quality Gates**: `make lint` and `make test` will be run before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. Proto already supports
  `Reasoning` field.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/510-tui-detail-recommendations/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output (decisions & rationale)
├── data-model.md        # Phase 1 output (entity definitions)
├── quickstart.md        # Phase 1 output (usage guide)
└── checklists/
    └── requirements.md  # Requirements checklist
```

### Source Code Changes

```text
internal/
├── engine/
│   └── types.go              # Add Reasoning field to Recommendation struct
├── proto/
│   ├── adapter.go            # Map proto Reasoning → internal Recommendation
│   └── types.go              # Add Reasoning field to proto Recommendation struct
├── cli/
│   ├── common_execution.go   # Add fetchAndMergeRecommendations helper
│   ├── cost_projected.go     # Call fetchAndMergeRecommendations after cost calc
│   └── cost_actual.go        # Call fetchAndMergeRecommendations after cost calc
└── tui/
    ├── cost_view.go          # Add renderRecommendationsSection, update RenderDetailView
    └── cost_view_test.go     # Tests for recommendations rendering
```

## Implementation Phases

### Phase 1: Data Model Extension (Foundation)

**Goal**: Carry `Reasoning` field from proto through to engine types.

#### 1a. Extend `engine.Recommendation` struct

**File**: `internal/engine/types.go` (~line 157)
**Change**: Add `Reasoning []string` field with `json:"reasoning,omitempty"` tag.

```go
type Recommendation struct {
    ResourceID       string               `json:"resourceId,omitempty"`
    Type             string               `json:"type"`
    Description      string               `json:"description"`
    EstimatedSavings float64              `json:"estimatedSavings,omitempty"`
    Currency         string               `json:"currency,omitempty"`
    Status           RecommendationStatus `json:"status,omitempty"`
    Reasoning        []string             `json:"reasoning,omitempty"` // NEW
}
```

#### 1b. Extend `proto.Recommendation` struct

**File**: `internal/proto/adapter.go` (line ~381-405, `Recommendation` struct)
**Change**: Add `Reasoning []string` field.

#### 1c. Update adapter conversion

**File**: `internal/proto/adapter.go` (~line 862-888)
**Change**: In the proto → internal conversion loop, add:

```go
protoRec.Reasoning = rec.GetReasoning()
```

#### 1d. Update engine conversion

**File**: `internal/engine/engine.go` (~line 2633-2652, `convertProtoRecommendation`)
**Change**: Copy `Reasoning` field:

```go
engineRec.Reasoning = rec.Reasoning
```

**Tests**: Unit test that `Reasoning` survives the full conversion pipeline.

### Phase 2: CLI Fetch-and-Merge Helper

**Goal**: Shared function to fetch recommendations and merge into cost results.

#### 2a. Add `fetchAndMergeRecommendations` to `common_execution.go`

**File**: `internal/cli/common_execution.go`
**Change**: Add new function:

```go
// fetchAndMergeRecommendations fetches recommendations for the given resources
// and merges them into the corresponding cost results by ResourceID.
// Errors are logged at WARN level but never propagated (FR-006).
func fetchAndMergeRecommendations(ctx context.Context, eng *engine.Engine,
    resources []engine.ResourceDescriptor, results []engine.CostResult) {
    log := logging.FromContext(ctx)
    recsResult, err := eng.GetRecommendationsForResources(ctx, resources)
    if err != nil {
        log.Warn().Ctx(ctx).Err(err).
            Msg("failed to fetch recommendations for detail view")
        return
    }
    if len(recsResult.Recommendations) == 0 {
        return
    }

    recMap := make(map[string][]engine.Recommendation)
    for _, rec := range recsResult.Recommendations {
        recMap[rec.ResourceID] = append(recMap[rec.ResourceID], rec)
    }

    for i := range results {
        if recs, found := recMap[results[i].ResourceID]; found {
            results[i].Recommendations = recs
        }
    }

    log.Debug().Ctx(ctx).
        Int("recommendations_count", len(recsResult.Recommendations)).
        Msg("merged recommendations into cost results")
}
```

#### 2b. Wire into `executeCostProjected`

**File**: `internal/cli/cost_projected.go` (~line 96-186)
**Change**: After `eng.GetProjectedCostWithErrors(ctx, resources)` returns and before
`RenderCostOutput()`, call:

```go
fetchAndMergeRecommendations(ctx, eng, resources, resultWithErrors.Results)
```

#### 2c. Wire into `executeCostActual`

**File**: `internal/cli/cost_actual.go` (~line 141-269)
**Change**: After `eng.GetActualCostWithOptionsAndErrors(ctx, request)` returns and
before `RenderActualCostOutput()`, call:

```go
fetchAndMergeRecommendations(ctx, eng, resources, resultWithErrors.Results)
```

**Tests**: Unit test for `fetchAndMergeRecommendations` with mock engine.

### Phase 3: TUI Rendering

**Goal**: Display recommendations in the detail view with sorting and reasoning.

#### 3a. Add `renderRecommendationsSection`

**File**: `internal/tui/cost_view.go`
**Change**: Add new function following `renderSustainabilitySection` pattern:

```go
// renderRecommendationsSection writes a "RECOMMENDATIONS" section to content
// when recommendations are present. Recommendations are sorted by estimated
// savings in descending order (FR-009). Each recommendation shows its action
// type, description, and optional savings. Reasoning entries are rendered as
// indented warning lines beneath the description (FR-002).
func renderRecommendationsSection(content *strings.Builder,
    recommendations []engine.Recommendation) {
    if len(recommendations) == 0 {
        return
    }

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
        fmt.Fprintf(content, "- [%s] %s%s\n",
            rec.Type, rec.Description, savingsStr)

        for _, reason := range rec.Reasoning {
            fmt.Fprintf(content, "    %s\n",
                WarningStyle.Render(reason))
        }
    }
    content.WriteString("\n")
}
```

#### 3b. Update `RenderDetailView` to call `renderRecommendationsSection`

**File**: `internal/tui/cost_view.go` (~line 275-359)
**Change**: Insert call between sustainability and notes sections:

```go
// Sustainability metrics.
renderSustainabilitySection(&content, resource.Sustainability)

// Recommendations (FR-008: after sustainability, before notes).
renderRecommendationsSection(&content, resource.Recommendations)

// Notes/Errors.
if resource.Notes != "" {
```

**Tests**: Comprehensive unit tests:

- Recommendations with savings (sorted correctly)
- Recommendations without savings (appear last)
- Recommendations with reasoning (indented warnings)
- Empty recommendations (no section rendered)
- Mixed savings values (stable sort order)
- Default currency when empty

### Phase 4: Integration Testing and Validation

**Goal**: End-to-end verification and quality gates.

#### 4a. Add integration-style test for full rendering pipeline

Test `RenderDetailView` with a `CostResult` that has populated `Recommendations`
including reasoning entries.

#### 4b. Run quality gates

```bash
make test      # All tests pass
make lint      # Linting clean
```

#### 4c. Update CLAUDE.md if needed

Add any new patterns discovered during implementation.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations. Implementation is straightforward extension of existing patterns:

- Struct field addition (engine, proto types)
- Adapter field mapping (one line each layer)
- CLI helper function (follows existing `extractCurrencyFromResults` pattern)
- TUI section renderer (follows existing `renderSustainabilitySection` pattern)

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Plugin returns no Reasoning data | High (most plugins don't populate it yet) | Graceful: section renders without warnings when Reasoning is empty |
| Recommendation fetch adds latency | Medium | FR-006: fetch is best-effort; failures logged silently |
| Large number of recommendations | Low | Detail view is scrollable; no truncation needed |
| Proto field mismatch | Very Low | Verified: `Reasoning` is field 14 in costsource.pb.go |
