# Implementation Plan: Zerolog Distributed Tracing

**Branch**: `004-zerolog-tracing` | **Date**: 2025-11-25 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-zerolog-tracing/spec.md`

## Summary

Implement comprehensive structured logging using zerolog v1.34.0 throughout finfocus-core,
replacing the existing unused slog-based implementation. The feature enables full request tracing
from CLI entry through plugin responses via trace ID propagation in gRPC metadata, with
configurable log levels, JSON/console output formats, and a `--debug` CLI flag.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: github.com/rs/zerolog v1.34.0, github.com/oklog/ulid/v2 (for trace IDs)
**Storage**: N/A (logs to stderr/file, no persistence)
**Testing**: go test with -race flag, testify assertions
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application with gRPC plugin architecture
**Performance Goals**: <5% overhead when DEBUG logging enabled (SC-005)
**Constraints**: Logs to stderr by default, stdout reserved for command output
**Scale/Scope**: Integrates with all CLI commands, engine, registry, pluginhost packages

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic (logging infrastructure), not a
      cost data source. Plugins receive trace IDs via gRPC metadata but implement their own logging.
- [x] **Test-Driven Development**: Tests planned before implementation. Target 90% coverage for
      logging package (SC-007), 80% minimum overall.
- [x] **Cross-Platform Compatibility**: zerolog is pure Go, cross-platform. File output uses
      standard os package. No platform-specific code required.
- [x] **Documentation as Code**: Audience-specific docs planned:
  - User Guide: --debug flag, environment variables, config file options
  - Developer Guide: How to add logging to new code, trace context propagation
  - Architect Guide: Logging architecture, gRPC interceptor design
- [x] **Protocol Stability**: No protocol buffer changes required. Trace ID propagation uses
      standard gRPC metadata (not proto definitions).
- [x] **Quality Gates**: All CI checks will pass (tests, lint, security, formatting, docs).
- [x] **Multi-Repo Coordination**: finfocus-spec provides SDK logging utilities (issue #75).
      Core implementation is independent; plugins adopt separately.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/004-zerolog-tracing/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no new APIs)
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── logging/             # Primary changes - replace slog with zerolog
│   ├── zerolog.go       # New: Logger factory, context helpers
│   ├── zerolog_test.go  # New: Comprehensive tests
│   ├── logger.go        # Delete: Old slog implementation
│   ├── logger_test.go   # Delete: Old tests
│   ├── errors.go        # Keep: Error types (may add logging integration)
│   └── errors_test.go   # Keep: Error tests
├── cli/
│   ├── root.go          # Modify: Add --debug flag, initialize logger
│   ├── cost_projected.go # Modify: Add logging calls
│   └── cost_actual.go   # Modify: Add logging calls
├── engine/
│   └── engine.go        # Modify: Add cost calculation logging
├── registry/
│   └── registry.go      # Modify: Add plugin discovery logging
├── pluginhost/
│   ├── process.go       # Modify: Add connection lifecycle logging
│   └── grpc.go          # New: gRPC interceptor for trace propagation
├── ingest/
│   └── pulumi_plan.go   # Modify: Add plan parsing logging
├── spec/
│   └── loader.go        # Modify: Add spec loading logging
└── config/
    └── config.go        # Modify: Extend LoggingConfig for zerolog

cmd/
└── finfocus/
    └── main.go          # Modify: Initialize logger at startup

test/
├── unit/
│   └── logging/         # New: Logging unit tests
└── integration/
    └── trace_propagation_test.go  # New: gRPC trace propagation tests
```

**Structure Decision**: Single project structure. Changes integrate into existing `internal/`
package organization. No new top-level directories required.

## Complexity Tracking

No violations to justify. All principles satisfied.
