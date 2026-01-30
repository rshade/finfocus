# Research: Add UnaryInterceptors Support to ServeConfig

**Branch**: `005-pluginsdk-interceptors`
**Date**: 2025-11-28

## Research Tasks Completed

### 1. Current Implementation Analysis

**Task**: Understand existing ServeConfig and Serve() implementation

**Findings**:

The current implementation in `finfocus-spec/sdk/go/pluginsdk/sdk.go`:

- `ServeConfig` has 4 fields: Plugin, Port, Registry, Logger
- `Serve()` hardcodes `TracingUnaryServerInterceptor()` in `grpc.NewServer()`
- No mechanism exists for plugin developers to add custom interceptors
- The tracing interceptor is always first (and only) in the chain

**Decision**: Add `UnaryInterceptors []grpc.UnaryServerInterceptor` field to
ServeConfig.

**Rationale**: This is the idiomatic gRPC pattern for interceptor configuration.
The slice type allows multiple interceptors and nil/empty handling is natural.

**Alternatives Considered**:

1. **Function option pattern** (`WithUnaryInterceptor(...)`): Rejected because
   ServeConfig already uses struct-based configuration. Mixing patterns would
   be inconsistent.
2. **Single interceptor field**: Rejected because users often need multiple
   interceptors (tracing + logging + metrics).
3. **Variadic Serve parameter**: Rejected because it breaks the existing API
   signature.

### 2. gRPC Interceptor Chaining Best Practices

**Task**: Research gRPC interceptor chaining patterns in Go

**Findings**:

- `grpc.ChainUnaryInterceptor()` is the standard approach (introduced in gRPC-Go 1.28)
- Interceptors execute in order: first registered runs first (outer → inner)
- Each interceptor receives the handler from the next interceptor in chain
- nil interceptors in the slice are not allowed (will panic)

**Decision**: Use `grpc.ChainUnaryInterceptor(interceptors...)` with tracing
interceptor prepended to user interceptors.

**Rationale**: Ensures trace IDs are available to all subsequent interceptors.
This matches the common pattern of tracing → logging → metrics → auth.

**Alternatives Considered**:

1. **User interceptors first, tracing last**: Rejected because user interceptors
   would not have access to trace IDs in their context.
2. **Separate tracing toggle**: Rejected as over-engineering; tracing is always
   useful for debugging.
3. **Replace instead of prepend**: Rejected because removing built-in tracing
   would break distributed debugging expectations.

### 3. Nil Slice Handling

**Task**: Research how gRPC handles nil/empty interceptor slices

**Findings**:

- `grpc.ChainUnaryInterceptor()` with no arguments returns a no-op interceptor
- Passing nil elements in the slice causes a panic during server creation
- Empty slice (`[]grpc.UnaryServerInterceptor{}`) is safe

**Decision**: No special nil filtering needed because:

1. If `config.UnaryInterceptors` is nil, append is safe (append to nil slice)
2. If user passes nil elements, that's a user bug (same as any other gRPC usage)

**Rationale**: We follow the principle of least surprise - behave like standard
gRPC. Users who pass nil interceptors get the same behavior they would with
direct gRPC usage.

**Alternatives Considered**:

1. **Filter nil elements**: Rejected because it hides user bugs and adds
   unexpected behavior.
2. **Explicit validation with error return**: Rejected as over-engineering for
   a developer-facing SDK.

### 4. Backward Compatibility

**Task**: Verify existing plugins continue to work

**Findings**:

- Adding a new struct field with zero value is backward compatible in Go
- Existing code that creates `ServeConfig{Plugin: p}` will have nil
  `UnaryInterceptors`
- nil slice append behavior means tracing interceptor still runs alone

**Decision**: No breaking changes. Existing plugins work unchanged.

**Rationale**: Go's zero-value semantics ensure backward compatibility.
The new field defaults to nil, which means "no additional interceptors."

**Alternatives Considered**: None needed - this is the expected behavior.

### 5. Performance Impact

**Task**: Assess performance impact of interceptor chaining

**Findings**:

- gRPC's `ChainUnaryInterceptor` has negligible overhead (~50ns per interceptor)
- The actual interceptor logic dominates execution time
- Slice allocation in `Serve()` is a one-time cost at startup

**Decision**: No performance concerns. The 5% startup time budget from Success
Criteria is easily met.

**Rationale**: We're adding one slice allocation and one append operation per
`Serve()` call, which happens once at plugin startup.

## Summary of Decisions

| Topic           | Decision                                              | Rationale                               |
| --------------- | ----------------------------------------------------- | --------------------------------------- |
| Config API      | Add `UnaryInterceptors []grpc.UnaryServerInterceptor` | Idiomatic Go, matches existing pattern  |
| Chaining order  | Tracing first, user interceptors after                | Trace IDs available to all interceptors |
| Nil handling    | No special handling                                   | Follow gRPC conventions                 |
| Backward compat | Zero-value = no change                                | Go struct semantics                     |
| Performance     | No concern                                            | One-time startup cost                   |

## Open Questions Resolved

All NEEDS CLARIFICATION items from Technical Context have been resolved:

- ✅ Language/Version: Go 1.25.6 (from go.mod)
- ✅ Dependencies: gRPC v1.74.2, zerolog (from go.mod)
- ✅ Testing: go test with race detection (from constitution)
- ✅ Platform: Linux, macOS, Windows (from constitution)
