# Implementation Plan: Cache Expires-At Hints

**Branch**: `606-cache-expires-at` | **Date**: 2026-03-12 | **Spec**: `specs/606-cache-expires-at/spec.md`
**Input**: Feature specification from `/specs/606-cache-expires-at/spec.md`

## Summary

Plumb the `expires_at` caching hint from plugin gRPC responses through the
adapter and engine layers into the cache store, allowing plugins to control
per-entry TTLs. When a plugin sets `expires_at` on a projected or actual
cost response, the engine calculates the remaining duration and uses it as
the cache TTL instead of the system default. Past timestamps skip caching
entirely; excessively long TTLs are capped at 7 days.

## Technical Context

**Language/Version**: Go 1.25.8 (see `go.mod`)
**Primary Dependencies**: finfocus-spec v0.5.7 (provides `expires_at` proto fields), BoltDB (cache storage), zerolog (logging)
**Storage**: BoltDB (`cache.db`) — no structural changes needed; `CacheEntry` already has per-entry `ExpiresAt`/`TTLSeconds`
**Testing**: `go test` with testify `require`/`assert`; table-driven tests; 80% coverage minimum
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go module CLI tool
**Performance Goals**: Cache store/retrieve operations remain under 10ms (SC-005)
**Constraints**: Min TTL 60s (user config only — plugin hints bypass this), Max TTL 604800s (7 days, applies to all)
**Scale/Scope**: 3 types modified, 1 interface extended, ~150 lines of new code + tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic — reads hints from plugin responses, no direct provider integration.
- [x] **Test-Driven Development**: Tests planned before implementation (80% min coverage). No TUI changes — no golden files needed.
- [x] **Cross-Platform Compatibility**: Uses only standard Go time operations and existing BoltDB storage. No platform-specific code.
- [x] **Documentation Integrity**: CLAUDE.md engine cache section will be updated. No new exported API surfaces requiring package READMEs.
- [x] **Protocol Stability**: Consuming existing proto fields (field 13 on GetProjectedCostResponse, field 8 on ActualCostResult). No protocol changes.
- [x] **Implementation Completeness**: Full implementation — no stubs or TODOs. All edge cases (past timestamps, max cap, nil hints) handled.
- [x] **Quality Gates**: `make lint` and `make test` will pass. No `.golangci.yml` changes.
- [x] **Multi-Repo Coordination**: Depends on finfocus-spec v0.5.7 (already available in go.mod). No changes to spec or plugin repos.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/606-cache-expires-at/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── proto/
│   └── adapter.go           # Add ExpiresAt to CostResult and ActualCostResult;
│                             # extract from proto responses in clientAdapter methods
├── engine/
│   ├── types.go             # Add ExpiresAt to engine.CostResult
│   ├── engine.go            # Update storeProjectedCostCache and storeActualCostCache
│   │                        # to calculate TTL from ExpiresAt; update getProjectedCostFromPlugin
│   │                        # and getActualCostFromPlugin to map ExpiresAt
│   └── cache/
│       ├── store.go         # Add SetWithTTL method to Cache interface and BoltStore
│       ├── cache_test.go    # Tests for SetWithTTL (existing test file)
│       ├── ttl.go           # Add CalculatePluginTTL helper function
│       └── ttl_test.go      # Tests for CalculatePluginTTL
└── (no other packages affected)
```

**Structure Decision**: This feature touches existing packages only — no new
packages or files beyond test files. Changes follow the existing layered
architecture: proto adapter → engine types → cache store.

## Complexity Tracking

No violations. No new complexity beyond what the feature requires.

## Design Decisions

### D1: Cache Interface Extension Strategy

**Decision**: Add `SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error`
method to the `Cache` interface rather than changing `Set()` signature.

**Rationale**: Preserves backward compatibility. Existing `Set()` callers
(recommendations cache) continue working unchanged. Only projected and actual
cost storage paths use the new method.

**Alternative rejected**: Adding an optional TTL parameter to `Set()` would
require updating all callers and mock implementations even when they don't
use custom TTLs.

### D2: TTL Calculation Location

**Decision**: Add a `CalculatePluginTTL(expiresAt *time.Time, defaultTTL int) (ttl int, skip bool)`
function in `internal/engine/cache/ttl.go`.

**Rationale**: Centralizes TTL logic (past detection, max cap, default fallback)
in the cache package where TTL constants already live. Engine callers pass
the `ExpiresAt` hint and get back either a TTL to use or a "skip" signal.

### D3: ExpiresAt Field Type

**Decision**: Use `*time.Time` (pointer) for `ExpiresAt` on both `proto.CostResult`
and `engine.CostResult`.

**Rationale**: Nil means "no hint provided" → use default TTL. Non-nil means
"plugin explicitly set an expiration." Matches the proto semantics where
`google.protobuf.Timestamp` is a message type (nil when unset).

### D4: Minimum TTL for Plugin Hints

**Decision**: Plugin hints bypass the `MinTTLSeconds` (60s) constraint. A plugin
can request a TTL as short as 1 second.

**Rationale**: Per spec edge case: "The minimum TTL constraint applies to
user-configured defaults, not plugin-provided hints — plugins have domain
authority over their data freshness." Only `MaxTTLSeconds` (7 days) applies
to plugin hints.

### D5: Batch Actual Cost TTL Strategy

**Decision**: For actual cost responses with multiple `ActualCostResult` entries,
use the **earliest** (shortest remaining) `expires_at` across all results.

**Rationale**: Per FR-007. The actual cost cache stores the aggregated response
as a single entry. Using the shortest TTL ensures no individual result's data
is served past its intended expiration.

## Data Flow

```text
Plugin gRPC Response
    │
    ▼
pbc.GetProjectedCostResponse.ExpiresAt (field 13, *timestamppb.Timestamp)
pbc.ActualCostResult.ExpiresAt          (field 8,  *timestamppb.Timestamp)
    │
    ▼  [clientAdapter.GetProjectedCost / GetActualCost]
    │  Convert timestamppb → *time.Time
    │
proto.CostResult.ExpiresAt       (*time.Time)  ─── NEW FIELD
proto.ActualCostResult.ExpiresAt (*time.Time)  ─── NEW FIELD
    │
    ▼  [Engine: getProjectedCostFromPlugin / getActualCostFromPlugin]
    │  Map proto type → engine type
    │
engine.CostResult.ExpiresAt (*time.Time)  ─── NEW FIELD
    │
    ▼  [Engine: storeProjectedCostCache / storeActualCostCache]
    │  Call cache.CalculatePluginTTL(expiresAt, defaultTTL)
    │  Returns (ttlSeconds, skipCache)
    │
    ├── skipCache=true  → Do not cache (past expiration)
    │
    └── skipCache=false → cache.SetWithTTL(key, data, ttlSeconds)
                          │
                          ▼
                     CacheEntry with plugin-controlled TTL
```

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Plugin sends absurd future timestamp | Cache entry lives 7 days max | MaxTTLSeconds cap (FR-006) |
| Plugin sends past timestamp | Stale data served | Skip caching entirely (FR-005) |
| Clock skew between plugin and core | TTL calculation off | Documented as operational concern (spec edge case) |
| Existing cache entries unaffected | None | No format changes; existing entries use stored TTL |
| Test mock implementations need update | Build break | Update mock Cache interface implementations |
