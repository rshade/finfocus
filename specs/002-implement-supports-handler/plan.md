# Implementation Plan: Implement Supports() gRPC Handler

**Branch**: `002-implement-supports-handler` | **Date**: 2025-11-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-implement-supports-handler/spec.md`

## Summary

This plan outlines the implementation of a `Supports` gRPC handler in the `pluginsdk`. The handler will determine if a plugin supports a given resource by checking for a new, optional `SupportsProvider` interface on the plugin. If the interface exists, the call is delegated; otherwise, a default "not supported" response is returned. This change will be implemented in `pkg/pluginsdk/sdk.go`, following the established pattern for other gRPC handlers in that file.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: `github.com/rshade/finfocus-spec v0.1.0`, `google.golang.org/grpc v1.77.0`
**Storage**: N/A
**Testing**: Go `testing` package with `stretchr/testify`
**Target Platform**: Cross-platform (Linux, macOS, Windows)
**Project Type**: CLI / Backend Service
**Performance Goals**: 99% of `Supports` queries complete within 50ms.
**Constraints**: Must not break compatibility with plugins that do not implement the `Supports` capability.
**Scale/Scope**: The change is confined to the `pluginsdk` and involves adding one new RPC handler and one new interface.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: Yes, this feature improves the core orchestration logic that serves plugins.
- [x] **Test-Driven Development**: Yes, tests will be created to validate both the supported and unsupported scenarios before implementation.
- [x] **Cross-Platform Compatibility**: Yes, the implementation will use standard Go libraries that are platform-agnostic.
- [x] **Documentation as Code**: Yes, this plan and the associated design documents serve as documentation. Code comments will also be updated.
- [x] **Protocol Stability**: Yes, this change implements an existing, stable part of the v0.1.0 protocol. No protocol changes are being made.
- [x] **Quality Gates**: N/A during planning, but the implementation will adhere to all CI checks.
- [x] **Multi-Repo Coordination**: Yes, the plan acknowledges the dependency on the `finfocus-spec` repository.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/002-implement-supports-handler/
├── plan.md              # This file
├── research.md          # Research on the gRPC server implementation
├── data-model.md        # Description of the Resource entity
└── contracts/
    └── grpc.md          # Description of the Supports() gRPC contract
```

### Source Code (repository root)

```text
pkg/pluginsdk/
├── sdk.go               # Add SupportsProvider interface and Supports() handler
└── sdk_test.go          # Add unit tests for the Supports() handler

```

**Structure Decision**: The changes are localized to the `pluginsdk` package, which is the central place for plugin hosting and SDK logic. This aligns with the existing project structure.

## Scope Notes

### Registry Infrastructure (Deferred)

The two-step validation approach references `registry.json` for plugin lookup by provider/region. For this feature:

- **In Scope**: Define the registry lookup interface/function signature in `pkg/pluginsdk/sdk.go`
- **Out of Scope**: Actual registry.json file creation and population (tracked as separate feature)
- **Implementation Note**: The lookup function should return a clear error when no registry exists or no matching plugin is found, satisfying the InvalidArgument edge case

This allows the Supports handler to be fully functional while registry infrastructure is developed separately.
