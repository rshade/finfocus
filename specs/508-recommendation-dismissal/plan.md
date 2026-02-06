# Implementation Plan: Recommendation Dismissal and Lifecycle Management

**Branch**: `508-recommendation-dismissal` | **Date**: 2026-02-05 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/508-recommendation-dismissal/spec.md`

## Summary

Implement recommendation lifecycle management (dismiss, snooze, undismiss, history) using a plugin-primary architecture with local fallback. When a plugin advertises `PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS`, the CLI calls the `DismissRecommendation` RPC and the plugin owns the dismissal state. The CLI always persists dismissals locally for client-side filtering via `ExcludedRecommendationIds` and audit trail purposes.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Cobra v1.10.2 (CLI), gRPC v1.78.0 (plugins), finfocus-spec v0.5.5 (protocol), zerolog v1.34.0 (logging), testify v1.11.1 (testing)
**Storage**: Local JSON file (`~/.finfocus/dismissed.json`) for dismissal state; plugin-side storage delegated to plugins
**Testing**: `go test` with testify assertions, table-driven tests, mock plugin patterns
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module CLI application
**Performance Goals**: 1,000+ stored dismissals without degradation; dismiss operation completes in <1s
**Constraints**: No new dependencies; cross-platform file paths; thread-safe state file access
**Scale/Scope**: Single-user CLI workflows; cross-environment consistency via plugin-side persistence

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Dismissal delegates to plugins via DismissRecommendation RPC when the plugin advertises the capability. Core remains orchestration-only. Local state is a fallback/filter layer, not a provider integration.
- [x] **Test-Driven Development**: Tests planned before implementation. Unit tests for state management, engine methods, CLI commands, and adapter. Integration tests for plugin communication flow. Target 80%+ coverage.
- [x] **Cross-Platform Compatibility**: Local state file uses `os.UserHomeDir()` for path resolution. JSON format is platform-agnostic. No platform-specific code required.
- [x] **Documentation Integrity**: CLI reference docs, README updates, and godoc for all new exported symbols planned.
- [x] **Protocol Stability**: Uses existing DismissRecommendation RPC and DismissalReason enum from finfocus-spec v0.5.5. No protocol changes required.
- [x] **Implementation Completeness**: No stubs or TODOs. All subcommands (dismiss, snooze, undismiss, history) fully implemented.
- [x] **Quality Gates**: `make lint` and `make test` required before completion.
- [x] **Multi-Repo Coordination**: Follow-up tickets documented for finfocus-spec enhancements (include_dismissed field, GetRecommendationHistory RPC). No spec changes required for this feature.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/508-recommendation-dismissal/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── adapter.md       # CostSourceClient interface contract
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── cost_recommendations.go              # MODIFY: add subcommand registration, --include-dismissed flag
│   ├── cost_recommendations_dismiss.go      # NEW: dismiss + snooze subcommands
│   ├── cost_recommendations_dismiss_test.go # NEW: unit tests
│   ├── cost_recommendations_undismiss.go    # NEW: undismiss subcommand
│   ├── cost_recommendations_undismiss_test.go # NEW: unit tests
│   ├── cost_recommendations_history.go      # NEW: history subcommand
│   └── cost_recommendations_history_test.go # NEW: unit tests
├── config/
│   ├── dismissed.go                         # NEW: dismissal state management
│   └── dismissed_test.go                    # NEW: unit tests
├── engine/
│   ├── engine.go                            # MODIFY: add DismissRecommendation, filter dismissed in GetRecommendations
│   ├── engine_dismiss_test.go               # NEW: unit tests for dismiss methods
│   └── types.go                             # MODIFY: add DismissRequest/Response types
├── proto/
│   ├── adapter.go                           # MODIFY: add DismissRecommendation to CostSourceClient interface
│   ├── adapter_test.go                      # MODIFY: add tests for dismiss adapter
│   └── dismissal_reasons.go                 # NEW: dismissal reason parsing utilities (mirrors action_types.go pattern)
│   └── dismissal_reasons_test.go            # NEW: unit tests
└── pluginhost/
    └── host.go                              # (no changes needed - ConvertCapabilities already handles dismiss_recommendations)

test/
├── unit/
│   └── config/
│       └── dismissed_test.go                # NEW: state file unit tests
└── integration/
    └── cli/
        └── recommendations_dismiss_test.go  # NEW: integration tests
```

**Structure Decision**: Follows existing FinFocus patterns: one CLI file per subcommand group, state management in `internal/config/`, adapter extension in `internal/proto/`, engine orchestration in `internal/engine/`. The dismissal reason utilities follow the `action_types.go` pattern for consistency.
