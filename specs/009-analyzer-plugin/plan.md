# Implementation Plan: Pulumi Analyzer Plugin Integration

**Branch**: `009-analyzer-plugin` | **Date**: 2025-12-05 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/009-analyzer-plugin/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement the Pulumi Analyzer plugin interface (`pulumirpc.Analyzer`) to enable zero-click cost estimation directly within `pulumi preview` output. The plugin will:

1. Implement the gRPC `Analyzer` service interface from Pulumi's protocol
2. Map `pulumirpc.AnalyzerResource` messages to the existing `finfocus.ResourceDescriptor` format
3. Leverage the existing `internal/engine` for cost calculations
4. Return costs as `AnalyzeDiagnostic` messages with `INFO`/`WARNING` severity (never `ERROR` in MVP)
5. Expose the analyzer via a new `finfocus analyzer serve` subcommand
6. Handle the Pulumi plugin handshake (random TCP port printed to stdout)

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**:

- `google.golang.org/grpc v1.77.0` - gRPC server implementation
- `github.com/pulumi/pulumi/sdk/v3/proto/go` - Pulumi Analyzer protocol buffers
- `github.com/rshade/finfocus-spec v0.4.3` - FinFocus cost source protocol
- `github.com/spf13/cobra v1.10.1` - CLI framework
- `github.com/rs/zerolog v1.34.0` - Structured logging

**Storage**: `~/.finfocus/config.yaml` for plugin configuration (existing infrastructure)
**Testing**: Go stdlib `testing` package + `github.com/stretchr/testify v1.11.1`
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project (Go monorepo with `internal/` packages)
**Performance Goals**: Plugin handshake completes in <100ms; cost estimation adds <2s for stacks under 50 resources (SC-003)
**Constraints**: stdout reserved for handshake (port number only); all logs to stderr; graceful degradation on pricing API failures
**Scale/Scope**: MVP targets small-to-medium stacks (<1000 resources with configurable timeouts)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: The analyzer IS the plugin host, orchestrating existing cost source plugins. No direct provider integration in core.
- [x] **Test-Driven Development**: Tests planned before implementation (see Phase 1 contracts). Target 80% coverage, 95% for critical paths (analyzer server, resource mapping).
- [x] **Cross-Platform Compatibility**: Using Go's cross-compilation. No platform-specific code required. Random port selection uses `net.Listen("tcp", ":0")`.
- [x] **Documentation as Code**: Quickstart guide, developer integration guide, and CLI reference planned.
- [x] **Protocol Stability**: Implementing Pulumi's stable `pulumirpc.Analyzer` interface. No breaking changes to finfocus-spec required.
- [x] **Quality Gates**: CI checks in place (tests, lint, security). Will add analyzer-specific integration tests.
- [x] **Multi-Repo Coordination**: No finfocus-spec changes required. Using existing `CostSourceService` protocol.

**Violations Requiring Justification**: None - all principles satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/009-analyzer-plugin/
├── plan.md              # This file
├── research.md          # Phase 0 output - protocol details, handshake patterns
├── data-model.md        # Phase 1 output - resource mapping, diagnostic structures
├── quickstart.md        # Phase 1 output - user installation and configuration
├── contracts/           # Phase 1 output - gRPC interface contracts
│   ├── analyzer-service.md
│   └── resource-mapping.md
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── analyzer/            # NEW - Pulumi Analyzer implementation
│   ├── doc.go           # Package documentation
│   ├── server.go        # gRPC Analyzer service implementation
│   ├── server_test.go   # Unit tests for analyzer service
│   ├── mapper.go        # pulumirpc.Resource -> ResourceDescriptor mapping
│   ├── mapper_test.go   # Mapper unit tests
│   ├── diagnostics.go   # Cost results -> AnalyzeDiagnostic conversion
│   └── diagnostics_test.go
├── cli/
│   ├── analyzer.go      # NEW - `analyzer` command group
│   ├── analyzer_serve.go # NEW - `analyzer serve` subcommand
│   └── analyzer_test.go  # NEW - analyzer CLI tests
├── engine/              # EXISTING - reuse for cost calculations
├── pluginhost/          # EXISTING - reuse for cost plugin communication
├── registry/            # EXISTING - reuse for plugin discovery
├── config/              # EXISTING - add analyzer section support
│   └── config.go        # Add AnalyzerConfig struct
└── logging/             # EXISTING - reuse for stderr logging

test/
├── integration/
│   └── analyzer_test.go # NEW - end-to-end analyzer integration tests
└── fixtures/
    └── analyzer/        # NEW - test fixtures for analyzer
        ├── sample-stack.json
        └── expected-diagnostics.json
```

**Structure Decision**: Single project layout following existing `internal/` package structure. New `internal/analyzer` package for all analyzer-specific logic. CLI extension follows existing pattern in `internal/cli/`.

## Complexity Tracking

> No Constitution violations - table not required.
