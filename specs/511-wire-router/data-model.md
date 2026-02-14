# Data Model: Wire Router into Cost Commands

**Feature**: 511-wire-router
**Date**: 2026-02-13

## Entities

This feature introduces no new data entities. It bridges two existing representations of the same concept (plugin match results) and wires existing routing logic into the runtime path.

### Existing Entity: engine.PluginMatch (unchanged)

**Location**: `internal/engine/engine.go:79-87`

| Field | Type | Description |
|-------|------|-------------|
| Client | `*pluginhost.Client` | The matched plugin client |
| Priority | `int` | Configured priority (0 = default) |
| Fallback | `bool` | Whether fallback is enabled |
| MatchReason | `string` | Why this plugin matched ("automatic", "pattern", "global") |
| Source | `string` | Where the routing decision came from ("automatic", "config") |

### Existing Entity: router.PluginMatch (unchanged)

**Location**: `internal/router/router.go:67-87`

| Field | Type | Description |
|-------|------|-------------|
| Client | `*pluginhost.Client` | The matched plugin client |
| Priority | `int` | Configured priority (0 = default) |
| Fallback | `bool` | Whether fallback is enabled |
| MatchReason | `MatchReason` (int enum) | Why this plugin matched |
| Source | `string` | Where the routing decision came from ("automatic", "config") |

### Existing Entity: engine.Router (interface, unchanged)

**Location**: `internal/engine/engine.go:89-98`

| Method | Signature | Description |
|--------|-----------|-------------|
| SelectPlugins | `(ctx, resource, feature) []PluginMatch` | Returns plugins matching a resource |
| ShouldFallback | `(pluginName) bool` | Returns fallback status for a plugin |

### Existing Entity: config.RoutingConfig (unchanged)

**Location**: `internal/config/routing.go:19-24`

| Field | Type | Description |
|-------|------|-------------|
| Plugins | `[]PluginRouting` | Ordered list of plugin routing rules |

### New Component: EngineAdapter

**Location**: `internal/router/engine_adapter.go` (to be created)

Implements `engine.Router` by wrapping `router.DefaultRouter` and converting `router.PluginMatch` to `engine.PluginMatch`.

| Field | Type | Description |
|-------|------|-------------|
| router | `Router` | The underlying router implementation |

**Methods**:

| Method | Returns | Description |
|--------|---------|-------------|
| SelectPlugins | `[]engine.PluginMatch` | Delegates to router, converts PluginMatch slice |
| ShouldFallback | `bool` | Delegates directly to router |

## Type Conversion

The adapter performs a single-field transformation:

```text
router.PluginMatch.MatchReason (MatchReason int enum)
    → .String()
    → engine.PluginMatch.MatchReason (string)
```

All other fields (Client, Priority, Fallback, Source) are copied directly with no transformation.

## Data Flow

```text
CLI command
  → config.New().Routing (*RoutingConfig or nil)
  → router.NewRouter(WithClients, WithConfig)
  → router.NewEngineAdapter(router) → engine.Router
  → engine.New(...).WithRouter(adapter)
  → engine.selectPluginMatchesForResource()
      → adapter.SelectPlugins()
          → router.SelectPlugins()
          → convert []router.PluginMatch → []engine.PluginMatch
      → engine uses PluginMatch results for cost queries
```
