# Quickstart: Wire Router into Cost Commands

**Feature**: 511-wire-router
**Date**: 2026-02-13

## Overview

This feature wires the existing router package into all CLI cost commands so that plugin selection is region-aware and priority-based. Three implementation components are needed.

## Component 1: Engine Adapter (`internal/router/engine_adapter.go`)

Create a new file that:

1. Defines an `EngineAdapter` struct wrapping `router.Router`
2. Implements `engine.Router` interface (two methods)
3. Converts `[]router.PluginMatch` to `[]engine.PluginMatch` using `MatchReason.String()`
4. Exports a constructor `NewEngineAdapter(r Router) engine.Router`

Test file: `internal/router/engine_adapter_test.go` covering:

- All MatchReason enum values convert correctly
- Empty match slice handled
- ShouldFallback delegation works
- Nil client in match handled (defensive)

## Component 2: CLI Helper (`internal/cli/common_execution.go`)

Add a function to the existing file:

1. `createRouterForEngine(ctx context.Context, clients []*pluginhost.Client) engine.Router`
2. Loads config via `config.New()`
3. If `cfg.Routing` is nil, returns nil (no routing configured)
4. Creates `router.NewRouter(WithClients, WithConfig)`, wraps in `NewEngineAdapter`
5. On error, logs warning and returns nil

New imports needed: `config`, `router`.

## Component 3: Call Site Wiring (9 files)

Chain `.WithRouter(createRouterForEngine(ctx, clients))` at each site:

- `cost_actual.go:214`
- `cost_projected.go:181`
- `cost_estimate.go:361, 419, 746`
- `cost_recommendations.go:243`
- `cost_recommendations_dismiss.go:314`
- `overview.go:143`
- `analyzer_serve.go:122`

Skip sites with nil clients (no plugins to route):

- `cost_recommendations_dismiss.go:301`
- `cost_recommendations_history.go:55`
- `cost_recommendations_undismiss.go:62`

## Verification

```bash
# Run unit tests
go test ./internal/router/... -run TestEngineAdapter -v

# Run all tests
make test

# Run linter
make lint
```

## Example: Before and After

**Before** (cost_actual.go):

```go
eng := engine.New(clients, nil)
```

**After** (cost_actual.go):

```go
eng := engine.New(clients, nil).WithRouter(createRouterForEngine(ctx, clients))
```

The router returns nil when no routing config exists, and `WithRouter(nil)` leaves the engine in its default "query all plugins" mode.
