# Implementation Plan: Multi-Plugin Routing

**Branch**: `126-multi-plugin-routing` | **Date**: 2026-01-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/126-multi-plugin-routing/spec.md`

## Summary

Implement intelligent plugin routing that enables the engine to select appropriate plugins based on declared capabilities, resource patterns, and priority rules. This is a **two-layer routing system**:

1. **Automatic Routing (Zero Config)**: Uses existing `Metadata.SupportedProviders` to route resources to matching plugins automatically
2. **Declarative Routing (User Config)**: Optional YAML configuration for patterns, feature assignments, priorities, and fallback rules

The engine currently queries ALL plugins for ALL resources. This feature transforms it into a smart routing layer that filters plugins by supported providers before invoking them.

## Technical Context

**Language/Version**: Go 1.25.7 (per go.mod)
**Primary Dependencies**: github.com/spf13/cobra (CLI), google.golang.org/grpc (plugin communication), github.com/rshade/finfocus-spec (protocol definitions)
**Storage**: ~/.finfocus/config.yaml (YAML configuration)
**Testing**: go test with testify/require, testify/assert (80% minimum coverage)
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single project - CLI tool with plugin host system
**Performance Goals**: <10ms routing decision per resource (SC-002), <100ms failover (SC-005)
**Constraints**: Backward compatible with existing configs, no breaking changes to plugin protocol

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core - plugins remain unchanged. Routing uses existing plugin metadata (SupportedProviders).
- [x] **Test-Driven Development**: Tests planned before implementation with 80% minimum coverage (SC-008).
- [x] **Cross-Platform Compatibility**: Pure Go implementation, no platform-specific code required.
- [x] **Documentation Synchronization**: README and docs/ updates planned as part of deliverables.
- [x] **Protocol Stability**: No changes to gRPC protocol. Uses existing GetPluginInfo metadata.
- [x] **Implementation Completeness**: Feature fully scoped with 7 user stories and 23 functional requirements.
- [x] **Quality Gates**: All CI checks (tests, lint, security) planned as validation criteria.
- [x] **Multi-Repo Coordination**: Spec dependency on finfocus-spec#287 (PluginCapability enum) ✅ RESOLVED in v0.5.5+ (currently using v0.5.5).

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/126-multi-plugin-routing/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (internal API contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── router/              # NEW: Plugin routing logic
│   ├── router.go        # PluginRouter interface and implementations
│   ├── automatic.go     # Provider-based automatic routing
│   ├── declarative.go   # Config-based declarative routing
│   ├── priority.go      # Priority-based selection and fallback
│   ├── pattern.go       # Glob/regex pattern matching
│   └── cache.go         # Compiled pattern cache
├── engine/
│   └── engine.go        # Modified to use router for plugin selection
├── config/
│   └── routing.go       # NEW: Routing configuration structures
└── cli/
    ├── config_validate.go  # NEW: Config validation command
    └── plugin_list.go      # Modified to show capabilities/providers

test/
├── unit/
│   └── router/          # NEW: Router unit tests
├── integration/
│   └── routing_test.go  # NEW: Multi-plugin routing integration tests
└── fixtures/
    └── routing/         # NEW: Test routing configurations
```

**Structure Decision**: Single project structure. New `internal/router/` package encapsulates all routing logic, keeping engine/config changes minimal.

## Complexity Tracking

No constitution violations identified.

## Design Decisions

### Layer 1: Automatic Routing

**Decision**: Use existing `Metadata.SupportedProviders` from plugin's `GetPluginInfo()` response.

**Rationale**:

- Zero configuration required
- Data already captured but unused
- Provider extracted via existing `extractProviderFromType()` in analyzer/mapper.go

**Implementation**:

1. Extract provider from resource type (e.g., "aws:ec2/instance:Instance" → "aws")
2. Filter plugins where `SupportedProviders` contains the provider or `["*"]`
3. Query only matching plugins

### Layer 2: Declarative Routing

**Decision**: YAML configuration in `~/.finfocus/config.yaml` under `routing:` key.

**Configuration Schema**:

```yaml
routing:
  plugins:
    - name: aws-ce
      features: [Recommendations]
      priority: 20
      fallback: true
    - name: aws-public
      features: [ProjectedCosts, ActualCosts]
      patterns:
        - type: regex
          pattern: "aws:.*"
      priority: 10
    - name: eks-plugin
      patterns:
        - type: glob
          pattern: "aws:eks:*"
      priority: 30  # Higher = preferred
```

### Priority and Fallback

**Decision**: Higher priority values = preferred. Equal priority (0) = query all matching.

**Fallback Chain**:

1. Try highest-priority matching plugin
2. If fails AND `fallback: true`, try next priority
3. If all plugins fail, fall back to local specs
4. If no specs, return "no cost data available"

### Pattern Matching

**Decision**: Support both glob (filepath.Match) and regex (regexp.Compile).

**Pattern Types**:

- `glob`: Uses Go's `filepath.Match` semantics (e.g., `aws:ec2:*`)
- `regex`: Uses Go's `regexp` package with RE2 syntax (e.g., `aws:eks:.*`)

### Caching Strategy

**Decision**: Cache compiled regex patterns at config load time.

**Rationale**: Avoids recompilation on each request (FR-022).

## Key Implementation Files

| File | Purpose | Changes |
| ---- | ------- | ------- |
| `internal/router/router.go` | Router interface and factory | NEW |
| `internal/router/automatic.go` | Provider-based routing | NEW |
| `internal/router/declarative.go` | Config-based routing | NEW |
| `internal/router/priority.go` | Priority selection, fallback | NEW |
| `internal/router/pattern.go` | Glob/regex matching | NEW |
| `internal/engine/engine.go` | Use router for plugin selection | MODIFY (lines 183-216) |
| `internal/config/routing.go` | RoutingConfig struct | NEW |
| `internal/config/config.go` | Add routing field | MODIFY |
| `internal/cli/config_validate.go` | `finfocus config validate` | NEW |
| `internal/cli/plugin_list.go` | Show capabilities/providers | MODIFY |

## Risk Mitigation

| Risk | Mitigation |
| ---- | ---------- |
| Breaking existing configs | Routing is additive; no routing config = automatic routing (backward compatible) |
| Performance regression | <10ms routing overhead target, caching for patterns |
| Plugin protocol changes | No protocol changes required; uses existing metadata |
| Circular fallback | Priority-ordered linear chain; each plugin tried once |
| Missing PluginCapability enum | ✅ RESOLVED in v0.5.5+: PluginCapability enum now available in finfocus-spec |

## Dependencies

- **Internal**: `internal/pluginhost/host.go` (Client.Metadata.SupportedProviders)
- **Internal**: `internal/analyzer/mapper.go` (extractProviderFromType)
- **Internal**: `internal/config/config.go` (Config struct)
- **External**: rshade/finfocus-spec#287 (PluginCapability enum) - ✅ RESOLVED in v0.5.5+ (currently using v0.5.5)
