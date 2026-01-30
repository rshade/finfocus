# Implementation Plan: Reference Recorder Plugin for DevTools

**Branch**: `018-recorder-plugin` | **Date**: 2025-12-11 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/018-recorder-plugin/spec.md`

## Summary

Implement a reference "Recorder" plugin within finfocus-core that captures all gRPC requests to JSON files and optionally returns mock cost responses. This plugin serves as both a developer tool for inspecting Core-to-plugin data shapes and a canonical reference implementation demonstrating pluginsdk v0.4.6 patterns.

**Technical Approach**: Build as a standalone plugin in `plugins/recorder/` using the pluginsdk from finfocus-spec v0.4.6+. The plugin will implement CostSourceService, serialize requests to JSON, and generate randomized mock responses when configured.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**:

- `github.com/rshade/finfocus-spec v0.4.6+` (pluginsdk, protobuf definitions)
- `github.com/oklog/ulid/v2` (ULID generation for filenames)
- `github.com/rs/zerolog` (structured logging)
- `google.golang.org/grpc` (gRPC server/transport)

**Storage**: Local filesystem (`./recorded_data` default, configurable via env var)
**Testing**: `go test` with testify, mock plugin infrastructure from `test/mocks/plugin/`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single plugin binary within monorepo
**Performance Goals**: Recording should add <10ms overhead per request (dev tool, not production-critical)
**Constraints**: Must work with existing plugin discovery in `~/.finfocus/plugins/`
**Scale/Scope**: Dev tool; typical usage: dozens to hundreds of requests per session

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This IS a plugin implementing CostSourceService via gRPC
- [x] **Test-Driven Development**: Tests planned with 80%+ coverage target, critical paths at 95%
- [x] **Cross-Platform Compatibility**: Will build on Linux, macOS, Windows (amd64, arm64)
- [x] **Documentation as Code**: README.md, quickstart.md, and inline code comments planned
- [x] **Protocol Stability**: Uses existing finfocus-spec v0.4.6 protocol, no changes required
- [x] **Quality Gates**: CI workflow will run tests, lint, security scan, cross-platform builds
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec v0.4.6 (already released)

**Violations Requiring Justification**: None - full compliance

## Project Structure

### Documentation (this feature)

```text
specs/018-recorder-plugin/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (gRPC service definitions)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
plugins/recorder/
├── main.go              # Plugin entry point and gRPC server setup
├── plugin.go            # RecorderPlugin struct implementing CostSourceService
├── recorder.go          # Request serialization to JSON files
├── mocker.go            # Mock response generation (randomized costs)
├── config.go            # Configuration from environment variables
├── plugin.manifest.json # Plugin metadata for registry
├── README.md            # Plugin documentation
├── recorder_test.go     # Unit tests for recorder
├── plugin_test.go       # Unit tests for plugin struct
├── config_test.go       # Unit tests for configuration
└── mocker_test.go       # Unit tests for mock generation

# Build output
bin/
└── finfocus-plugin-recorder  # Built binary (cross-platform)

# Test integration
test/
├── integration/
│   └── recorder_test.go        # Integration tests with recorder plugin
└── fixtures/
    └── recorder/               # Test fixtures for recorder tests
```

**Structure Decision**: Plugin lives in `plugins/recorder/` within the monorepo. This keeps it close to Core for CI/CD integration and contract testing while maintaining clear separation as a standalone plugin binary. The `plugins/` directory is new and establishes the pattern for in-repo reference plugins.

## Complexity Tracking

No violations - complexity tracking not required.

## Post-Design Constitution Re-Check

*Performed after Phase 1 design completion.*

- [x] **Plugin-First Architecture**: ✅ Confirmed - CostSourceService implementation via gRPC
- [x] **Test-Driven Development**: ✅ Confirmed - Unit tests in recorder_test.go, integration tests planned
- [x] **Cross-Platform Compatibility**: ✅ Confirmed - Standard Go, no platform-specific code
- [x] **Documentation as Code**: ✅ Confirmed - quickstart.md, contracts/, data-model.md created
- [x] **Protocol Stability**: ✅ Confirmed - Uses existing v0.4.6 protocol, no changes
- [x] **Quality Gates**: ✅ Confirmed - Makefile target integrates with CI
- [x] **Multi-Repo Coordination**: ✅ Confirmed - finfocus-spec v0.4.6 dependency documented

**Post-Design Status**: All constitution principles satisfied. Ready for task generation.

## Generated Artifacts

| Artifact | Path | Description |
|----------|------|-------------|
| Research | [research.md](research.md) | SDK patterns, serialization, configuration decisions |
| Data Model | [data-model.md](data-model.md) | Entity definitions, validation rules, state transitions |
| Contract | [contracts/cost-source-service.md](contracts/cost-source-service.md) | gRPC interface specification |
| Quickstart | [quickstart.md](quickstart.md) | Developer guide (~10 min) |

## Next Steps

Run `/speckit.tasks` to generate actionable task list from this plan.
