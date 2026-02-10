# Implementation Plan: Extended RecommendationActionType Enum Support

**Branch**: `108-action-type-enum` | **Date**: 2025-12-29 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/108-action-type-enum/spec.md`

## Summary

Update finfocus-core to fully support the 5 new `RecommendationActionType`
enum values (MIGRATE, CONSOLIDATE, SCHEDULE, REFACTOR, OTHER) from
finfocus-spec v0.4.9+. The dependency is already in place (v0.4.11); this
feature adds Core-side handling for filter parsing, TUI display labels, CLI
help text, and JSON serialization.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: finfocus-spec v0.4.11 (pluginsdk), cobra v1.10.1,
  bubbletea v1.3.10, lipgloss v1.1.0
**Storage**: N/A (stateless enum mapping)
**Testing**: go test (stdlib), testify v1.11.1
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI project with internal packages
**Performance Goals**: N/A (simple enum mapping operations)
**Constraints**: Backward compatibility required for existing 6 action types
**Scale/Scope**: 5 new enum values, ~3-4 files modified

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution:

- [x] **Plugin-First Architecture**: This is orchestration logic (enum parsing/
      display) not a cost data source. Plugins already return these types via
      proto - Core adds user-facing support.
- [x] **Test-Driven Development**: Tests planned before implementation;
      table-driven tests for all 11 action type mappings.
- [x] **Cross-Platform Compatibility**: Pure Go enum handling - no platform-
      specific code.
- [x] **Documentation as Code**: CLI help text updated with all action types.
- [x] **Protocol Stability**: No proto changes - using existing enum values from
      finfocus-spec v0.4.9+.
- [x] **Implementation Completeness**: Full support for all 11 action types, no
      stubs or TODOs.
- [x] **Quality Gates**: Tests, lint, coverage checks apply.
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec v0.4.11 which is
      already integrated.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/108-action-type-enum/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── proto/
│   ├── adapter.go           # Recommendation struct - existing
│   └── action_types.go      # NEW: Action type utilities
├── cli/
│   └── cost_recommendations.go  # NEW: CLI command for recommendations
└── tui/
    └── components.go        # TUI label rendering (when recommendations TUI added)

```

**Test Files** (Go convention: tests alongside source):

```text
internal/proto/action_types_test.go   # Table-driven tests for action type utilities
internal/cli/cost_recommendations_test.go  # CLI command tests
```

**Structure Decision**: This feature adds the `cost recommendations` CLI command
by leveraging existing engine infrastructure (`GetRecommendationsForResources()`,
`RecommendationsResult`, `Recommendation` types). The new CLI command follows
patterns established by `cost actual` and `cost projected` commands.

## Complexity Tracking

No violations requiring justification.
