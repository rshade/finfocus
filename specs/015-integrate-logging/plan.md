# Implementation Plan: Complete Logging Integration

**Branch**: `007-integrate-logging` | **Date**: 2025-12-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/007-integrate-logging/spec.md`

## Summary

Complete the logging integration by connecting the existing `internal/logging` package to the
`internal/config` configuration system, adding CLI output for log file paths, and implementing
audit logging for cost queries. The logging package already provides structured logging with
zerolog, trace ID propagation, and sensitive data redaction - this feature wires it to config
and adds audit capabilities.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: zerolog v1.34.0 (already integrated), cobra v1.10.1, yaml.v3
**Storage**: File system (`~/.finfocus/config.yaml`, log files)
**Testing**: go test with testify, race detection required
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: Logging overhead < 1ms per operation
**Constraints**: No external log rotation (operator responsibility)
**Scale/Scope**: Single-user CLI tool, log files up to 100MB before external rotation

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A - This is orchestration/CLI infrastructure, not a cost data source
- [x] **Test-Driven Development**: Tests planned before implementation (80% minimum coverage)
- [x] **Cross-Platform Compatibility**: Using standard Go I/O, no platform-specific code
- [x] **Documentation as Code**: User guide updates planned for logging configuration
- [x] **Protocol Stability**: N/A - No protocol changes, internal feature only
- [x] **Quality Gates**: All CI checks required (tests, lint, security)
- [x] **Multi-Repo Coordination**: N/A - Changes confined to finfocus-core

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/007-integrate-logging/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no external API)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── logging/
│   ├── zerolog.go       # Existing: Logger creation, trace IDs, redaction
│   ├── errors.go        # Existing: Categorized errors
│   ├── audit.go         # NEW: Audit logging for cost queries
│   └── audit_test.go    # NEW: Audit logging tests
├── config/
│   ├── config.go        # Existing: Config struct with LoggingConfig
│   ├── integration.go   # Existing: GetLogLevel(), GetLogFile()
│   └── logging.go       # NEW: Bridge config to logging package
├── cli/
│   ├── root.go          # MODIFY: Wire config to logging, display log path
│   ├── cost_projected.go # MODIFY: Add audit logging
│   └── cost_actual.go   # MODIFY: Add audit logging
└── engine/
    └── engine.go        # Existing: Already uses logging.FromContext()

test/
├── unit/
│   └── logging/         # NEW: Unit tests for audit logging
└── integration/
    └── logging_test.go  # NEW: Integration tests for config → logging flow
```

**Structure Decision**: Using existing Go package structure. New code goes into
`internal/logging/` for audit features and `internal/config/` for config-to-logging bridge.

## Complexity Tracking

No violations - standard feature implementation within existing architecture.
