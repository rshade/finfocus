# Implementation Plan: Cost Recommendations Command Enhancement

**Branch**: `109-cost-recommendations` | **Date**: 2025-12-30 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/109-cost-recommendations/spec.md`
**GitHub Issue**: #216

## Summary

Enhance the existing `finfocus cost recommendations` command to add summary/verbose display modes, interactive Bubble Tea TUI with table navigation and detail views, and loading spinner feedback. The command currently exists with basic table/JSON/NDJSON output and action type filtering. This plan adds:

1. Summary mode (default): Top 5 recommendations by savings with aggregate statistics
2. Verbose mode (`--verbose`): Full details for all recommendations
3. Interactive TUI: Bubble Tea table with Enter for details, "/" for filtering, "s" for sorting
4. Loading spinner during plugin queries
5. TTY detection for graceful fallback to plain text

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: Cobra v1.10.1, Bubble Tea, Lip Gloss, zerolog v1.34.0
**Storage**: N/A (stateless command, data from plugins via gRPC)
**Testing**: go test with testify, table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: CLI tool
**Performance Goals**: Summary renders in <3 seconds, keyboard response <100ms
**Constraints**: 80% test coverage minimum, existing command backward compatible
**Scale/Scope**: Up to 1000 recommendations without degradation

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Feature is orchestration logic only; cost data comes from gRPC plugins
- [x] **Test-Driven Development**: Tests planned before implementation (80% minimum coverage target)
- [x] **Cross-Platform Compatibility**: All code uses standard Go libraries and existing TUI package
- [x] **Documentation as Code**: CLI help text will be updated; examples already in command
- [x] **Protocol Stability**: No protocol changes required; uses existing GetRecommendations RPC
- [x] **Implementation Completeness**: All features fully implemented, no stubs or TODOs
- [x] **Quality Gates**: `make lint` and `make test` will pass before completion
- [x] **Multi-Repo Coordination**: No cross-repo changes needed; spec already has GetRecommendations

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/109-cost-recommendations/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # N/A (no API contracts for CLI)
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── cost_recommendations.go       # MODIFY: Add summary/verbose/TUI modes
│   └── cost_recommendations_test.go  # MODIFY: Add tests for new modes
├── tui/
│   ├── recommendations_model.go      # NEW: Bubble Tea model for recommendations
│   ├── recommendations_model_test.go # NEW: Tests for recommendations model
│   ├── recommendations_view.go       # NEW: Render functions for recommendations
│   └── recommendations_view_test.go  # NEW: Tests for recommendations views
└── engine/
    └── types.go                      # READ ONLY: Use existing Recommendation struct

test/
├── unit/
│   └── cli/
│       └── cost_recommendations_test.go  # Already exists, extend
└── integration/
    └── recommendations_tui_test.go       # NEW: Integration tests for TUI
```

**Structure Decision**: Single-project CLI pattern. New TUI files follow existing `cost_model.go` and `cost_view.go` patterns in `internal/tui/`. CLI modifications extend existing `cost_recommendations.go`.

## Complexity Tracking

No violations requiring justification.

## Design Decisions

### D1: TUI Model Pattern

Follow existing `CostViewModel` pattern from `internal/tui/cost_model.go`:

- Create `RecommendationsViewModel` with same state machine (Loading, List, Detail, Quitting, Error)
- Reuse existing `LoadingState` and spinner components
- Reuse existing keyboard handling patterns

### D2: Output Mode Detection

Use existing `tui.DetectOutputMode()` function to determine:

- `OutputModeInteractive`: Full Bubble Tea TUI
- `OutputModeStyled`: Lip Gloss colored output (no interactivity)
- `OutputModePlain`: Plain text for CI/pipes

Explicit `--output json|ndjson` bypasses this and always outputs machine-readable format.

### D3: Summary vs Verbose Mode

- Default (summary): Show aggregate stats + top 5 by savings
- `--verbose`: Show all recommendations with full descriptions
- Both modes available in all output formats (table/styled/interactive)

### D4: Data Flow

```text
CLI → Load Resources → Open Plugins → Engine.GetRecommendationsForResources()
                                                   ↓
                             Apply Filters (existing) → Sort by Savings
                                                   ↓
                             Detect Output Mode → Route to Renderer
                                   ↓
       ┌───────────────────────┼───────────────────────┐
  Interactive            Styled/Plain              JSON/NDJSON
       ↓                       ↓                       ↓
 RecommendationsViewModel   renderRecommendations   renderRecommendations
 (Bubble Tea)               Summary/Verbose         JSON/NDJSON
                            (tabwriter+lipgloss)
```

## Implementation Phases

### Phase 1: Add `--verbose` Flag and Summary Mode (P1, P2 Stories)

1. Add `--verbose` flag to command
2. Implement `renderRecommendationsSummary()` for default mode
3. Update `renderRecommendationsTable()` to show all when verbose
4. Add sorting by savings (descending)
5. Add tests for both modes

### Phase 2: Create Recommendations TUI Model (P3 Story)

1. Create `RecommendationsViewModel` in `internal/tui/`
2. Implement state machine: Loading → List → Detail
3. Implement keyboard handlers (Enter, Esc, /, s, q)
4. Implement filtering by resource ID/type/description
5. Add tests for model state transitions

### Phase 3: Create Recommendations TUI Views (P3 Story)

1. Create `RenderRecommendationsSummary()` for styled summary
2. Create `RenderRecommendationDetail()` for detail view
3. Create `NewRecommendationsTable()` for table display
4. Add tests for view rendering

### Phase 4: Integrate TUI into CLI Command (P3, P4 Stories)

1. Add TTY detection to `executeCostRecommendations()`
2. Route to interactive mode when appropriate
3. Add loading spinner during plugin queries
4. Add `--plain` and `--no-color` flags for override
5. Integration tests for CLI → TUI flow

### Phase 5: JSON Output Enhancement (P2 Story - US5)

1. Add `summary` object to JSON output structure
2. Include `count_by_action_type` breakdown
3. Add `savings_by_action_type` breakdown
4. Update NDJSON to include summary as first line
5. Add tests for JSON schema compliance

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| TUI complexity | Reuse existing patterns from `cost_model.go` |
| Backward compatibility | All new features are additive; existing flags work unchanged |
| Performance with 1000+ recommendations | Lazy loading in TUI, pagination if needed |
| Cross-platform terminal issues | Use `tui.DetectOutputMode()` with graceful fallback |

## Dependencies Verified

- [x] `internal/tui/` package exists with all needed components
- [x] `engine.Recommendation` struct has required fields
- [x] `engine.GetRecommendationsForResources()` exists
- [x] `proto.ActionTypeLabelFromString()` exists for formatting
- [x] `tui.DetectOutputMode()` exists for TTY detection
- [x] `tui.LoadingState` and spinner exist

## Success Metrics

- [ ] `make test` passes with 80%+ coverage for recommendations code
- [ ] `make lint` passes
- [ ] All acceptance scenarios from spec verified
- [ ] Interactive TUI responds to keyboard within 100ms
- [ ] Summary mode works in non-TTY environments
