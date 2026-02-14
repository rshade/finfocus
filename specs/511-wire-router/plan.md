# Implementation Plan: Wire Router into Cost Commands

**Branch**: `511-wire-router` | **Date**: 2026-02-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/511-wire-router/spec.md`

## Summary

Wire the existing `internal/router/` package into all CLI cost commands so that the engine performs region-aware, priority-based plugin selection instead of querying every installed plugin for every resource. This requires a type adapter (router.PluginMatch to engine.PluginMatch), a CLI helper function, and chaining `.WithRouter()` at all 9 engine instantiation sites that use plugin clients.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: Cobra v1.10.2 (CLI), gRPC v1.78.0 (plugins), finfocus-spec v0.5.6 (protocol), zerolog v1.34.0 (logging)
**Storage**: N/A (stateless per-invocation; reads `~/.finfocus/config.yaml`)
**Testing**: testify v1.11.1, `go test`, `make test`, `make lint`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module CLI application
**Performance Goals**: Router initialization <10ms per command invocation
**Constraints**: Zero behavioral regression for users without routing config; no new dependencies
**Scale/Scope**: 9 call sites to modify, 1 new adapter file, 1 new CLI helper, ~200 lines of production code + tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core (routing plugins, not implementing a provider). The router selects which existing gRPC plugins to query. No direct provider integration added.
- [x] **Test-Driven Development**: Tests planned for adapter (unit), CLI helper (unit), and integration (region-aware selection). Target 80%+ coverage for new code.
- [x] **Cross-Platform Compatibility**: No platform-specific code. Uses standard Go interfaces, config loading, and string conversion. All existing cross-platform patterns preserved.
- [x] **Documentation Integrity**: CLAUDE.md will be updated to document the router wiring pattern. No new public APIs requiring docs/ changes.
- [x] **Protocol Stability**: No protocol buffer changes. Uses existing `engine.Router` interface and `router.DefaultRouter` implementation unchanged.
- [x] **Implementation Completeness**: Full implementation of adapter, helper, and all 9 wiring sites. No stubs or TODOs.
- [x] **Quality Gates**: `make test` and `make lint` required before completion.
- [x] **Multi-Repo Coordination**: No cross-repo changes. All changes within finfocus-core.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/511-wire-router/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
internal/
├── router/
│   ├── engine_adapter.go       # NEW: Type bridge router.PluginMatch → engine.PluginMatch
│   └── engine_adapter_test.go  # NEW: Unit tests for adapter
├── cli/
│   ├── common_execution.go     # MODIFIED: Add createRouterForEngine() helper
│   ├── cost_actual.go          # MODIFIED: Chain .WithRouter()
│   ├── cost_projected.go       # MODIFIED: Chain .WithRouter()
│   ├── cost_estimate.go        # MODIFIED: Chain .WithRouter() at 3 sites
│   ├── cost_recommendations.go # MODIFIED: Chain .WithRouter()
│   ├── cost_recommendations_dismiss.go  # MODIFIED: Chain .WithRouter() at plugin site only
│   ├── overview.go             # MODIFIED: Chain .WithRouter()
│   └── analyzer_serve.go       # MODIFIED: Chain .WithRouter()
└── engine/
    └── engine.go               # UNCHANGED: Router interface already defined
```

**Structure Decision**: All changes are within the existing `internal/` directory structure. One new file (`engine_adapter.go`) in the router package and one new file (`engine_adapter_test.go`) for its tests. The CLI helper is added to the existing `common_execution.go` which already houses similar helpers (`openPlugins()`).

## Complexity Tracking

No violations to justify. The implementation is straightforward adapter + wiring pattern.
