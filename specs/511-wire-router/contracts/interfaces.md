# Contracts: Wire Router into Cost Commands

**Feature**: 511-wire-router
**Date**: 2026-02-13

## Interface Contracts

This feature does not introduce new APIs or protocols. It wires existing interfaces together. The contracts below document the interfaces being consumed and the new adapter that bridges them.

### Contract 1: engine.Router (consumed, unchanged)

```go
// Engine expects this interface for plugin selection.
// Defined in internal/engine/engine.go.
type Router interface {
    SelectPlugins(ctx context.Context, resource ResourceDescriptor, feature string) []PluginMatch
    ShouldFallback(pluginName string) bool
}
```

**Invariants**:

- `SelectPlugins` may return an empty slice (engine falls back to all clients)
- `ShouldFallback` returns `true` for unknown plugin names (safe default)
- The `feature` parameter uses values like `"ProjectedCosts"`, `"ActualCosts"`, `"Recommendations"`

### Contract 2: router.Router (consumed, unchanged)

```go
// Router package's own interface with additional Validate method.
// Defined in internal/router/router.go.
type Router interface {
    SelectPlugins(ctx context.Context, resource engine.ResourceDescriptor, feature string) []PluginMatch
    ShouldFallback(pluginName string) bool
    Validate(ctx context.Context) ValidationResult
}
```

**Key difference from engine.Router**: Returns `router.PluginMatch` (with int enum `MatchReason`) instead of `engine.PluginMatch` (with string `MatchReason`).

### Contract 3: EngineAdapter (new, implements engine.Router)

```go
// Bridges router.Router → engine.Router.
// Located in internal/router/engine_adapter.go.
type EngineAdapter struct {
    router Router
}

func NewEngineAdapter(r Router) engine.Router
```

**Conversion rules**:

| router.PluginMatch field | engine.PluginMatch field | Transformation |
|--------------------------|--------------------------|----------------|
| Client | Client | Direct copy |
| Priority | Priority | Direct copy |
| Fallback | Fallback | Direct copy |
| MatchReason | MatchReason | `.String()` (int enum to string) |
| Source | Source | Direct copy |

### Contract 4: createRouterForEngine (new CLI helper)

```go
// Creates a router for the engine, returns nil if no routing config exists
// or if router creation fails.
// Located in internal/cli/common_execution.go.
func createRouterForEngine(ctx context.Context, clients []*pluginhost.Client) engine.Router
```

**Behavior**:

| Condition | Return value | Side effect |
|-----------|--------------|-------------|
| No routing config (`cfg.Routing == nil`) | `nil` | None |
| Valid routing config | `*EngineAdapter` wrapping `*DefaultRouter` | None |
| Invalid routing config (bad patterns) | `nil` | Log warning |
| Empty clients slice | `*EngineAdapter` (router handles empty clients) | None |
