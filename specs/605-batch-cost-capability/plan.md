# Implementation Plan: Recognize Batch Cost Capability

**Branch**: `605-batch-cost-capability` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/605-batch-cost-capability/spec.md`

## Summary

Add `FeatureBatchCost` to the router's feature/capability mapping system so the
router can detect and match plugins that support batch cost queries. The
capability string conversion (`ConvertCapabilities()` in pluginhost) and plugin
list display already work — only the router feature constants and mapping
functions need updating.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: finfocus-spec v0.5.7+ (proto definitions with
`PLUGIN_CAPABILITY_BATCH_COST = 12`)
**Storage**: N/A
**Testing**: `go test` with testify (assert/require)
**Target Platform**: Linux, macOS, Windows (amd64, arm64)
**Project Type**: Single Go project (CLI tool)
**Performance Goals**: N/A (additive constant mapping, no runtime impact)
**Constraints**: None
**Scale/Scope**: ~4 files modified, ~30 lines of production code, ~40 lines of tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: This is orchestration logic in Core that
  recognizes a plugin-reported capability. No direct provider integration.
- [x] **Test-Driven Development**: Tests planned for all modified functions.
  No TUI changes (no golden files needed). 80% coverage maintained.
- [x] **Cross-Platform Compatibility**: Pure Go code with no platform-specific
  logic. Compiles on all targets.
- [x] **Documentation Integrity**: Godoc comments on new constant. No
  README/docs changes needed (capability list is dynamic from code).
- [x] **Protocol Stability**: Additive only — new capability enum consumed
  from upstream spec. No breaking changes.
- [x] **Implementation Completeness**: Full implementation, no stubs or TODOs.
- [x] **Quality Gates**: `make test && make lint` required before completion.
- [x] **Multi-Repo Coordination**: Blocked by #844 (finfocus-spec v0.5.7
  upgrade). Related to #846 (BatchCost RPC consumer).

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/605-batch-cost-capability/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (minimal — no data model changes)
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── router/
│   ├── features.go          # Add FeatureBatchCost constant, update ValidFeatures(),
│   │                        #   add methodToFeature mapping
│   ├── features_test.go     # Update tests for new feature constant
│   ├── router.go            # Add cases in capabilityEnumFromFeature(),
│   │                        #   capabilityEnumFromString()
│   └── router_test.go       # Add tests for new capability mapping
└── pluginhost/
    └── host.go              # Already handles BATCH_COST (no changes needed)
```

**Structure Decision**: Existing Go project layout. Changes are confined to the
`internal/router/` package (feature constants and capability mapping). The
`internal/pluginhost/host.go` `ConvertCapabilities()` function already maps
`PLUGIN_CAPABILITY_BATCH_COST` → `"batch_cost"` and requires no changes.
