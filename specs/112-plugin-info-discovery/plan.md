# Implementation Plan: Plugin Info and DryRun Discovery

**Branch**: `112-plugin-info-discovery` | **Date**: 2026-01-10 | **Spec**: [specs/112-plugin-info-discovery/spec.md]
**Input**: Feature specification from `/specs/112-plugin-info-discovery/spec.md`

## Summary

This feature implements the consumer-side requirements for `GetPluginInfo` and `DryRun` RPCs in `finfocus-core`. It includes updating the plugin host to verify version compatibility during initialization, enhancing the `plugin list` command to display metadata, and adding a new `plugin inspect` command for capability discovery.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: github.com/rshade/finfocus-spec v0.4.14, github.com/Masterminds/semver/v3
**Storage**: N/A (Stateless CLI)
**Testing**: go test, testify
**Target Platform**: Linux, macOS, Windows
**Project Type**: single
**Performance Goals**: Discovery under 200ms (SC-002), 5s/10s RPC timeouts
**Constraints**: Handle "Unimplemented" errors for legacy plugins; provide --skip-version-check global flag
**Scale/Scope**: Impacts plugin initialization and CLI management commands

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Feature adds orchestration logic for gRPC plugins.
- [x] **Test-Driven Development**: Tests planned before implementation.
- [x] **Cross-Platform Compatibility**: Pure Go implementation, cross-platform compatible.
- [x] **Documentation Synchronization**: README and docs/reference/cli-commands.md planned for update.
- [x] **Protocol Stability**: Uses Spec v0.4.14; maintains backward compatibility for legacy plugins.
- [x] **Implementation Completeness**: Full implementation of RPC calls and validation planned.
- [x] **Quality Gates**: CI checks mandatory.
- [x] **Multi-Repo Coordination**: Dependencies on finfocus-spec documented.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/112-plugin-info-discovery/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cmd/finfocus/
└── root.go              # Global flags

internal/cli/
├── plugin_list.go       # Updated to call GetPluginInfo
└── plugin_inspect.go    # New command for DryRun

internal/pluginhost/
├── host.go              # Updated initialization logic
└── grpc.go              # Interceptors

internal/proto/
└── adapter.go           # Interface and implementation updates

pkg/version/
└── version.go           # Core spec version constant
```

**Structure Decision**: Single project (DEFAULT). We are extending existing internal packages and CLI commands.


## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
