# Implementation Plan: Add UnaryInterceptors Support to ServeConfig

**Branch**: `005-pluginsdk-interceptors` | **Date**: 2025-11-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-pluginsdk-interceptors/spec.md`
**Target Repository**: finfocus-spec (sdk/go/pluginsdk)

## Summary

Enable plugin developers to register custom gRPC unary interceptors via
`ServeConfig.UnaryInterceptors`. The existing `TracingUnaryServerInterceptor()`
is already hardcoded in `Serve()` - this feature adds the ability to chain
additional interceptors while preserving the built-in tracing interceptor
as the first in the chain.

## Technical Context

**Language/Version**: Go 1.25.7
**Primary Dependencies**: google.golang.org/grpc v1.74.2, github.com/rs/zerolog
**Storage**: N/A (stateless configuration)
**Testing**: go test with race detection
**Target Platform**: Linux, macOS, Windows (amd64, arm64)
**Project Type**: Single project (SDK library)
**Performance Goals**: Plugin startup time impact within 5% of baseline
**Constraints**: Backward compatible - existing plugins must work unchanged
**Scale/Scope**: Minimal change - 1 struct field, ~10 lines of Serve() modification

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution:

- [x] **Plugin-First Architecture**: Feature enhances plugin SDK, not core
- [x] **Test-Driven Development**: Tests planned for interceptor chaining and nil handling
- [x] **Cross-Platform Compatibility**: Pure Go, no platform-specific code
- [x] **Documentation as Code**: Quickstart example and godoc updates planned
- [x] **Protocol Stability**: No protocol buffer changes (struct field only)
- [x] **Quality Gates**: make lint + make test required
- [x] **Multi-Repo Coordination**: Change in finfocus-spec only; core consumes via go.mod

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/005-pluginsdk-interceptors/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A - no API contracts for SDK)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (finfocus-spec repository)

```text
sdk/go/pluginsdk/
├── sdk.go               # ServeConfig struct + Serve() function (MODIFY)
├── sdk_test.go          # Unit tests for Serve() (ADD/MODIFY)
├── logging.go           # TracingUnaryServerInterceptor (NO CHANGE)
└── traceid.go           # Trace ID generation (NO CHANGE)
```

**Structure Decision**: Minimal modification to existing sdk.go file. No new
files required. Tests added to existing sdk_test.go.

## Complexity Tracking

No violations - no entries needed.

## Current Implementation Analysis

### Existing ServeConfig (sdk.go lines 196-202)

```go
type ServeConfig struct {
    Plugin   Plugin
    Port     int             // If 0, will use PORT env var or random port
    Registry RegistryLookup  // Optional; if nil, DefaultRegistryLookup is used
    Logger   *zerolog.Logger // Optional; if nil, a default logger is used
}
```

### Existing Serve() (sdk.go lines 273-309)

```go
grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(TracingUnaryServerInterceptor()),
)
```

**Issue**: TracingUnaryServerInterceptor() is hardcoded. Plugin developers
cannot add custom interceptors.

## Proposed Changes

### ServeConfig Extension

```go
type ServeConfig struct {
    Plugin            Plugin
    Port              int
    Registry          RegistryLookup
    Logger            *zerolog.Logger
    UnaryInterceptors []grpc.UnaryServerInterceptor // NEW
}
```

### Serve() Modification

```go
// Build interceptor chain: tracing first, then user interceptors
interceptors := make([]grpc.UnaryServerInterceptor, 0, 1+len(config.UnaryInterceptors))
interceptors = append(interceptors, TracingUnaryServerInterceptor())
interceptors = append(interceptors, config.UnaryInterceptors...)

grpcServer := grpc.NewServer(
    grpc.ChainUnaryInterceptor(interceptors...),
)
```

**Design Decision**: TracingUnaryServerInterceptor() remains first in chain
to ensure trace IDs are always available to subsequent interceptors.
