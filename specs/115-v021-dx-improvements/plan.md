# Implementation Plan: v0.2.1 Developer Experience Improvements

**Branch**: `115-v021-dx-improvements` | **Date**: 2026-01-17 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/115-v021-dx-improvements/spec.md`

## Summary

This feature implements a set of developer experience (DX) and performance improvements for the v0.2.1 release. Key enhancements include:
1.  **Parallel Plugin Fetching**: Optimizing `plugin list` to fetch metadata concurrently using `errgroup`, scaling O(1) instead of O(N).
2.  **Plugin Ecosystem Maturity**: integrating `GetPluginInfo` RPC to validate plugin compatibility and display spec versions, with "Legacy" fallback.
3.  **Cleaner Upgrades**: Adding a `--clean` flag to `plugin install` to automatically remove all older versions of a plugin.
4.  **Code Quality**: Refactoring cost filter logic into a shared helper `internal/cli/filters.go` to eliminate duplication.

## Technical Context

**Language/Version**: Go 1.25.5
**Primary Dependencies**: 
- `golang.org/x/sync/errgroup` (Concurrency)
- `github.com/spf13/cobra` (CLI)
- `github.com/rshade/finfocus-spec` (Protobufs/RPC)
- `google.golang.org/grpc` (RPC Client)
**Storage**: Filesystem (Plugin directories)
**Testing**: `testing` (Standard Lib), `github.com/stretchr/testify` (Assertions)
**Target Platform**: Linux, macOS, Windows (Cross-platform Go)
**Project Type**: CLI Tool / Core Engine
**Performance Goals**: `plugin list` completion time dominated by max single plugin latency, not sum.
**Constraints**: Must handle legacy plugins gracefully (no crash). Must safely handle filesystem operations during cleanup.
**Scale/Scope**: Core logic changes in CLI and PluginHost.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Feature enhances plugin management and orchestration without bypassing plugin architecture.
- [x] **Test-Driven Development**: Tests planned for concurrency, cleanup logic, and filter helpers (80% coverage required).
- [x] **Cross-Platform Compatibility**: Go standard library used for all OS interactions; no platform-specific syscalls.
- [x] **Documentation Synchronization**: README and CLI docs will be updated with new flags and behavior.
- [x] **Protocol Stability**: Uses existing `GetPluginInfo` RPC from spec; no protocol changes required in Core (only consumption).
- [x] **Implementation Completeness**: Plan covers full implementation of all requirements.
- [x] **Quality Gates**: Standard CI pipeline applies.
- [x] **Multi-Repo Coordination**: Depends on `finfocus-spec` (already available).

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/115-v021-dx-improvements/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no new APIs)
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cmd/finfocus/
└── ... (CLI entry points)

internal/
├── cli/
│   ├── cost_actual.go       # Updated (use shared filter)
│   ├── cost_projected.go    # Updated (use shared filter)
│   ├── filters.go           # NEW: Shared filter helper
│   ├── plugin_install.go    # Updated (cleanup logic)
│   └── plugin_list.go       # Updated (parallel fetch, GetPluginInfo)
├── engine/
│   └── ...
├── pluginhost/
│   ├── client.go            # Updated (GetPluginInfo method)
│   └── ...
└── registry/
    └── installer.go         # Updated (cleanup implementation)
```

**Structure Decision**: Enhancing existing `internal/cli` and `internal/pluginhost` packages. Adding one new file `internal/cli/filters.go` for shared logic.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |