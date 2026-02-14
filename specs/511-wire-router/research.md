# Research: Wire Router into Cost Commands

**Feature**: 511-wire-router
**Date**: 2026-02-13

## R1: Type Bridge Between router.PluginMatch and engine.PluginMatch

**Decision**: Create `engine_adapter.go` in `internal/router/` that implements `engine.Router` by delegating to `router.DefaultRouter` and converting `router.PluginMatch` slices to `engine.PluginMatch` slices.

**Rationale**: The two `PluginMatch` types are structurally identical except for `MatchReason` -- `router.PluginMatch.MatchReason` is `MatchReason` (int enum with `String()` method) while `engine.PluginMatch.MatchReason` is `string`. Placing the adapter in the `router` package avoids circular imports since `router` already imports `engine` (for `engine.ResourceDescriptor` in `SelectPlugins` signature).

**Alternatives considered**:

- Placing adapter in `internal/cli/`: Rejected because it would expose router internals to the CLI layer and duplicate type knowledge.
- Placing adapter in `internal/engine/`: Rejected because engine cannot import router (circular dependency -- router already imports engine for `ResourceDescriptor`).
- Unifying the two PluginMatch types: Rejected because the engine deliberately mirrors the type to avoid importing router (documented in engine.go comment: "mirrors router.PluginMatch to avoid circular imports").

## R2: CLI Helper Function Location and Signature

**Decision**: Add `createRouterForEngine(ctx context.Context, clients []*pluginhost.Client) engine.Router` to `internal/cli/common_execution.go`, adjacent to the existing `openPlugins()` helper.

**Rationale**: `common_execution.go` already contains shared helpers for cost commands (`openPlugins()`, `loadAndMapResources()`, `auditContext`). The router helper follows the same pattern. It loads config via `config.New()`, creates a `router.DefaultRouter`, wraps it in `router.NewEngineAdapter()`, and returns `engine.Router`. On any error, it logs a warning and returns `nil` (engine handles nil router gracefully by querying all plugins).

**Alternatives considered**:

- Inline router creation in each command: Rejected because it duplicates ~10 lines of boilerplate across 9 call sites.
- Passing router from parent command via context: Rejected because it adds unnecessary complexity and deviates from the established pattern where each command creates its own engine.

## R3: Call Site Wiring Strategy

**Decision**: Chain `.WithRouter(createRouterForEngine(ctx, clients))` at each of the 9 `engine.New()` call sites that have non-nil clients.

**Rationale**: The `engine.WithRouter()` method already exists and returns `*Engine` for method chaining. This is the simplest, most readable approach. Each call site gets a single additional method call.

**Call sites to wire** (9 total):

| File | Line | Current |
|------|------|---------|
| `cost_actual.go` | 214 | `engine.New(clients, nil)` |
| `cost_projected.go` | 181 | `engine.New(clients, spec.NewLoader(specDir))` |
| `cost_estimate.go` | 361 | `engine.New(clients, spec.NewLoader(specDir))` |
| `cost_estimate.go` | 419 | `engine.New(clients, spec.NewLoader(specDir))` |
| `cost_estimate.go` | 746 | `engine.New(clients, spec.NewLoader(specDir))` |
| `cost_recommendations.go` | 243 | `engine.New(clients, nil)` |
| `cost_recommendations_dismiss.go` | 314 | `engine.New(clients, nil)` |
| `overview.go` | 143 | `engine.New(clients, nil)` |
| `analyzer_serve.go` | 122 | `engine.New(clients, specLoader)` |

**Call sites to SKIP** (3 total -- nil clients, no plugins):

| File | Line | Current |
|------|------|---------|
| `cost_recommendations_dismiss.go` | 301 | `engine.New(nil, nil)` |
| `cost_recommendations_history.go` | 55 | `engine.New(nil, nil)` |
| `cost_recommendations_undismiss.go` | 62 | `engine.New(nil, nil)` |

**Alternatives considered**:

- Creating a wrapper function `newEngineWithRouter()`: Rejected because the call sites have varying second arguments (some `nil`, some `spec.NewLoader(specDir)`, some `specLoader`) making a unified wrapper awkward.
- Modifying `engine.New()` to accept a router: Rejected because it changes the stable public API and requires updating all callers including tests.

## R4: Config Loading Strategy

**Decision**: Use `config.New()` in the helper function to load the current configuration, then access `cfg.Routing` for the routing config.

**Rationale**: `config.New()` is the standard factory function already used throughout the CLI. It loads from `~/.finfocus/config.yaml`, applies environment overrides, and returns a `*Config` with `Routing *RoutingConfig` (nil if not configured). The `config_validate.go` already uses this pattern for routing validation, ensuring consistency (FR-005).

**Alternatives considered**:

- Passing config from the command's flag parsing: Rejected because not all commands load config the same way, and adding a config parameter would require modifying each command's parameter struct.
- Using a global config singleton: The `config.New()` function is already a safe factory that reads once per invocation. No additional caching needed.

## R5: Graceful Degradation Behavior

**Decision**: When router creation fails, log at WARN level and return `nil`. The engine already handles `nil` router by querying all plugins (existing fallback behavior).

**Rationale**: This matches the existing engine behavior (engine.go lines 166-183) where a nil router causes all clients to be returned as matches. Users without routing config or with invalid config see no behavioral change. The warning log provides diagnostic information without blocking the command.

**Error conditions that trigger fallback**:

- `config.New()` returns nil Routing (no config) -- returns nil immediately, no warning
- `router.NewRouter()` fails (invalid patterns) -- logs warning, returns nil
- Router is created successfully -- wraps in adapter and returns

## R6: MatchReason String Mapping

**Decision**: Use `router.MatchReason.String()` method to convert int enum to string for `engine.PluginMatch.MatchReason`.

**Rationale**: The `String()` method already exists on `router.MatchReason` (router.go:111-124) and returns values that align with engine expectations:

| router.MatchReason | String() output | Existing engine usage |
|--------------------|-----------------|----------------------|
| `MatchReasonAutomatic` | `"automatic"` | Used in engine fallback (line 179) |
| `MatchReasonPattern` | `"pattern"` | Expected by engine |
| `MatchReasonGlobal` | `"global"` | Expected by engine |
| `MatchReasonNoMatch` | `"no_match"` | Should not appear in results |

The `"automatic"` string is already used in the engine's nil-router fallback path, confirming compatibility.
