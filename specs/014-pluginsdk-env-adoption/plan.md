# Implementation Plan: Adopt pluginsdk/env.go for Environment Variable Handling

**Branch**: `014-pluginsdk-env-adoption` | **Date**: 2025-12-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/014-pluginsdk-env-adoption/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Migrate finfocus-core to use standardized pluginsdk/env.go constants for environment variable handling, ensuring consistency between core and plugins and centralizing variable definitions.

## Technical Context

**Language/Version**: Go 1.25.6  
**Primary Dependencies**: pluginsdk from finfocus-spec v0.4.5+ (github.com/rshade/finfocus-spec/sdk/go/pluginsdk)  
**Storage**: N/A (environment variable configuration)  
**Testing**: Go unit tests, integration tests  
**Target Platform**: Cross-platform (Linux amd64/arm64, macOS amd64/arm64, Windows amd64)  
**Project Type**: CLI tool refactor  
**Performance Goals**: N/A (refactor maintains existing performance)  
**Constraints**: Must maintain backward compatibility, no breaking changes to plugin communication  
**Scale/Scope**: Core codebase refactor affecting plugin host and code generator

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic (plugin host) - core remains provider-agnostic
- [x] **Test-Driven Development**: Tests will be updated/maintained, integration test added (80% minimum coverage maintained)
- [x] **Cross-Platform Compatibility**: Go code runs on Linux, macOS, Windows
- [x] **Documentation as Code**: CLAUDE.md will be updated with patterns
- [x] **Protocol Stability**: No protocol changes - only environment variable naming standardization
- [x] **Quality Gates**: All CI checks (tests, lint, security) will pass
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec#127, documented in spec

**Violations Requiring Justification**: None - all principles followed

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Core Go CLI application
cmd/finfocus/          # CLI entry point
internal/                # Internal packages
├── pluginhost/         # Plugin process management (target for env var changes)
├── config/             # Configuration handling
├── logging/            # Logging utilities
└── ...

pkg/                     # Shared packages
test/                    # Test fixtures and helpers
├── unit/               # Unit tests
├── integration/        # Integration tests
└── e2e/               # End-to-end tests

# Dependencies
go.mod                   # Go module dependencies (will add finfocus-spec)
```

**Structure Decision**: Single Go project following existing repository structure. Changes primarily in internal/pluginhost/ for environment variable handling and potential updates to cmd/gen/ for code generator.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation                  | Why Needed         | Simpler Alternative Rejected Because |
| -------------------------- | ------------------ | ------------------------------------ |
| [e.g., 4th project]        | [current need]     | [why 3 projects insufficient]        |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient]  |
