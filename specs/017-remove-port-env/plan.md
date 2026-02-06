# Implementation Plan: Remove PORT Environment Variable

**Branch**: `017-remove-port-env` | **Date**: 2025-12-15 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/017-remove-port-env/spec.md`

## Summary

Remove the legacy `PORT` environment variable from plugin process spawning in `internal/pluginhost/process.go`. The `--port` flag (already implemented) becomes the sole authoritative port communication mechanism. `FINFOCUS_PLUGIN_PORT` is retained for debugging/backward compatibility. Add guidance logging when plugins fail to bind, and DEBUG-level logging when `PORT` is detected in user's environment.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: github.com/rshade/finfocus-spec v0.4.1 (pluginsdk), google.golang.org/grpc v1.77.0
**Storage**: N/A
**Testing**: go test (stdlib), github.com/stretchr/testify
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI project
**Performance Goals**: No change to existing performance characteristics
**Constraints**: External dependency on finfocus-spec#129 (Plugin SDK must support --port flag first)
**Scale/Scope**: Focused refactoring - ~20 lines of production code, ~100 lines of test updates

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core (ProcessLauncher) - compliant
- [x] **Test-Driven Development**: Tests exist and will be updated; coverage target maintained
- [x] **Cross-Platform Compatibility**: No platform-specific changes; env var handling is portable
- [x] **Documentation as Code**: CLAUDE.md will be updated to reflect new behavior
- [x] **Protocol Stability**: No protocol buffer changes; this is internal orchestration
- [x] **Quality Gates**: `make lint` and `make test` will be run before completion
- [x] **Multi-Repo Coordination**: Dependency on finfocus-spec#129 documented

**Violations Requiring Justification**: None - all principles satisfied

## Project Structure

### Documentation (this feature)

```text
specs/017-remove-port-env/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # N/A for this refactoring
├── quickstart.md        # N/A for this refactoring
├── contracts/           # N/A for this refactoring
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/pluginhost/
├── process.go           # PRIMARY: Remove PORT env var, add guidance logging
├── process_test.go      # Update tests to verify PORT is NOT set
└── CLAUDE.md            # Update documentation

# Affected test files
test/integration/
└── pluginhost_test.go   # Verify integration tests still pass
```

**Structure Decision**: This is a focused refactoring in the existing `internal/pluginhost/` package with no new files or directories needed.

## Complexity Tracking

> No violations - table not needed
