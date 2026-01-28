# Research: Multi-Plugin Routing

**Branch**: `126-multi-plugin-routing` | **Date**: 2026-01-24

## Research Tasks

This document captures research findings for all technical unknowns identified during planning.

---

## 1. Provider Extraction from Resource Types

**Question**: How should the router extract the provider from Pulumi resource types?

**Findings**: `internal/analyzer/mapper.go` already has `extractProviderFromType()`:

```go
func extractProviderFromType(resourceType string) string {
    if resourceType == "" {
        return "unknown"
    }
    parts := strings.Split(resourceType, ":")
    if len(parts) >= 1 && parts[0] != "" {
        return parts[0]
    }
    return "unknown"
}
```

**Resource Type Formats**:

| Format | Example | Provider |
|--------|---------|----------|
| Standard Pulumi | `aws:ec2/instance:Instance` | `aws` |
| Module-less | `gcp:compute:Instance` | `gcp` |
| Native providers | `aws-native:ec2:Instance` | `aws-native` |
| Kubernetes | `kubernetes:core/v1:Pod` | `kubernetes` |
| Custom | `pulumi:providers:aws` | `pulumi` |

**Decision**: Reuse existing function. Move `extractProviderFromType()` from `internal/analyzer/mapper.go` to `internal/router/` or a shared `internal/util/` package for reuse by the router.

**Rationale**: Avoids duplication, already tested, handles edge cases.

---

## 2. Plugin Metadata Availability

**Question**: Is `SupportedProviders` reliably populated for all plugins?

**Findings** (from codebase exploration):

1. Plugin returns providers in `GetPluginInfo()` gRPC response
2. `internal/pluginhost/host.go:78-84` stores in `Client.Metadata.SupportedProviders`
3. Data is available immediately after `NewClient()` completes

**Edge Cases**:

| Scenario | Current Behavior | Desired Behavior |
|----------|------------------|------------------|
| Empty `SupportedProviders` | Plugin loaded normally | Treat as global (matches all) per FR-004 |
| `["*"]` wildcard | Plugin loaded normally | Treat as global (matches all) per FR-004 |
| Plugin crash before GetPluginInfo | Client creation fails | No routing needed |
| Plugin returns nil metadata | Empty metadata stored | Treat as global |

**Existing Plugins** (from finfocus-plugin repo):

- `aws-public`: Reports `["aws"]`
- `gcp-public`: Reports `["gcp"]`
- `recorder`: Reports `["*"]` (wildcard for debugging)

**Decision**: Use existing metadata. `SupportedProviders` is reliably populated. Treat empty/nil as global for backward compatibility.

**Rationale**: Infrastructure already exists; no protocol changes needed.

---

## 3. Pattern Matching Strategy

**Question**: How should glob and regex patterns be matched against resource types?

**Findings** (Go Standard Library Options):

| Library | Pattern | Performance | Features |
|---------|---------|-------------|----------|
| `filepath.Match` | Glob | ~100ns/match | `*`, `?`, `[...]` |
| `regexp.MatchString` | Regex | ~500ns/match | Full RE2 |
| `regexp.Regexp.MatchString` | Compiled | ~200ns/match | Full RE2 |

**Pattern Caching** (per FR-022):

```go
type PatternCache struct {
    mu      sync.RWMutex
    regexes map[string]*regexp.Regexp
}

func (c *PatternCache) Match(pattern, patternType, input string) (bool, error) {
    if patternType == "glob" {
        return filepath.Match(pattern, input)
    }
    // Get or compile regex
    c.mu.RLock()
    re, ok := c.regexes[pattern]
    c.mu.RUnlock()
    if !ok {
        c.mu.Lock()
        re, err := regexp.Compile(pattern)
        if err != nil {
            c.mu.Unlock()
            return false, err
        }
        c.regexes[pattern] = re
        c.mu.Unlock()
    }
    return re.MatchString(input), nil
}
```

**Benchmark Results** (projected):

- Glob match: ~100ns per resource
- Cached regex match: ~200ns per resource
- 10 patterns × 100 resources = 100-200µs total (well under 10ms target)

**Decision**: Support both glob and regex with caching:

- `type: glob` → `filepath.Match` (simpler patterns)
- `type: regex` → `regexp.Compile` (complex patterns, cached)
- Compile regexes at config load time, not per-request

**Rationale**: Meets <10ms performance target (SC-002), gives users flexibility.

---

## 4. Priority-Based Selection Algorithm

**Question**: How should the router select plugins when multiple match?

**Findings** (Algorithm Options):

| Option | Behavior | Complexity |
|--------|----------|------------|
| First match | Use first plugin in list | O(n) |
| Highest priority | Sort by priority, use highest | O(n log n) |
| All matching | Query all, merge results | O(n) |
| Priority with fallback | Try highest, fallback on failure | O(n) worst case |

**Spec Requirements**:

- FR-012: Route to highest-priority matching plugin first
- FR-014: Query ALL if equal priority (priority 0)
- FR-013: Support fallback to lower-priority on failure

**Proposed Algorithm**:

```go
func (r *Router) SelectPlugins(resource ResourceDescriptor, feature string) []*pluginhost.Client {
    // 1. Get all matching plugins (by provider + pattern + feature)
    matching := r.findMatchingPlugins(resource, feature)

    // 2. Group by priority
    byPriority := groupByPriority(matching)

    // 3. If all priority 0, return all (FR-014)
    if allZeroPriority(matching) {
        return matching
    }

    // 4. Sort by priority descending
    priorities := sortedKeys(byPriority, descending)

    // 5. Return ordered list for fallback chain
    var result []*pluginhost.Client
    for _, priority := range priorities {
        result = append(result, byPriority[priority]...)
    }
    return result
}
```

**Decision**: Priority-ordered list with fallback support:

1. Match plugins by provider → pattern → feature
2. Group by priority, highest first
3. If all priority=0, query all in parallel
4. Otherwise, return priority-ordered list for sequential fallback

**Rationale**: Satisfies FR-012, FR-013, FR-014 requirements.

---

## 5. Fallback Chain Implementation

**Question**: How should fallback work when a plugin fails?

**Findings** (Failure Types):

| Type | Detection | Fallback? |
|------|-----------|-----------|
| Connection timeout | `context.DeadlineExceeded` | Yes |
| Plugin crash | `EOF`, `connection reset` | Yes |
| Empty result | `len(results) == 0` | Yes (if configured) |
| Partial success | Some resources succeed | Fallback only for failed |
| Invalid response | Proto unmarshal error | Yes |

**Current Engine Behavior** (engine.go:183-216):

```go
for _, client := range e.clients {
    result, err := e.getProjectedCostFromPlugin(ctx, client, resource)
    if err != nil {
        continue  // Try next plugin
    }
    if result != nil {
        resourceResults = append(resourceResults, *result)
    }
}
```

**Proposed Fallback Logic**:

```go
func (e *Engine) getCostWithFallback(ctx context.Context, plugins []*pluginhost.Client, resource ResourceDescriptor) (*CostResult, error) {
    for _, plugin := range plugins {
        result, err := e.getProjectedCostFromPlugin(ctx, plugin, resource)

        if err == nil && result != nil && len(result.Results) > 0 {
            return result, nil  // Success
        }

        // Check if fallback enabled for this plugin
        if !e.router.ShouldFallback(plugin.Name) {
            return nil, err  // No fallback, return error
        }

        // Log and continue to next plugin
        log.Debug().
            Str("plugin", plugin.Name).
            Err(err).
            Msg("plugin failed, trying fallback")
    }

    // All plugins failed, try spec loader
    return e.getProjectedCostFromSpec(ctx, resource)
}
```

**Decision**: Per-plugin fallback configuration:

```yaml
routing:
  plugins:
    - name: aws-ce
      fallback: true   # Enable fallback to next plugin
    - name: aws-public
      fallback: false  # Do not fallback (last resort)
```

**Fallback Triggers**:

- Connection failure (timeout, reset)
- Empty result (no cost data)
- gRPC error (unavailable, internal)

**NOT Fallback Triggers**:

- Validation error (InvalidArgument) - plugin explicitly rejected
- $0 cost result - valid result, not a failure

**Rationale**: Meets SC-005 (<100ms failover), provides user control per FR-013.

---

## 6. Feature Capability Matching

**Question**: How should features be matched when PluginCapability enum doesn't exist yet?

**Findings**: rshade/finfocus-spec#287 is pending.

**Fallback Strategy**: Infer capabilities from gRPC method availability:

| gRPC Method | Inferred Capability |
|-------------|---------------------|
| `GetProjectedCost` | ProjectedCosts |
| `GetActualCost` | ActualCosts |
| `GetRecommendations` | Recommendations |
| `GetCarbonFootprint` | Carbon |
| `PerformDryRun` | DryRun |
| `GetBudgetStatus` | Budgets |

**Implementation**:

```go
type InferredCapabilities struct {
    ProjectedCosts  bool
    ActualCosts     bool
    Recommendations bool
    Carbon          bool
    DryRun          bool
    Budgets         bool
}

func InferCapabilities(client *pluginhost.Client) InferredCapabilities {
    // For now, assume all plugins support ProjectedCosts and ActualCosts
    // until we can query method availability
    return InferredCapabilities{
        ProjectedCosts: true,
        ActualCosts:    true,
        // Others default to false until explicitly configured
    }
}
```

**Decision**: Config-driven capabilities with defaults:

1. If `features` specified in config → use those
2. If not specified → assume all features plugin reports
3. Until spec#287, assume ProjectedCosts + ActualCosts as defaults

**Rationale**: Maintains forward compatibility with spec#287, works today.

---

## 7. Configuration Validation

**Question**: How should configuration validation work (eager vs lazy)?

**Findings** (Requirements):

- FR-018: Provide `finfocus config validate` for eager validation
- FR-019: Use lazy validation at first feature request by default

**Validation Checks**:

| Check | When | Severity |
|-------|------|----------|
| YAML syntax | Config load | Error |
| Unknown plugins | Eager only | Error |
| Invalid regex | Eager + Lazy | Error |
| Unsupported features | Eager only | Warning |
| Duplicate patterns | Eager only | Warning |

**Implementation**:

```go
type ValidationResult struct {
    Valid    bool
    Errors   []ValidationError
    Warnings []ValidationWarning
}

func (c *RoutingConfig) Validate(registry *Registry) ValidationResult {
    var result ValidationResult
    result.Valid = true

    for _, plugin := range c.Plugins {
        // Check plugin exists
        if !registry.Exists(plugin.Name) {
            result.Errors = append(result.Errors,
                ValidationError{Plugin: plugin.Name, Msg: "plugin not found"})
            result.Valid = false
        }

        // Check patterns compile
        for _, pattern := range plugin.Patterns {
            if pattern.Type == "regex" {
                if _, err := regexp.Compile(pattern.Pattern); err != nil {
                    result.Errors = append(result.Errors,
                        ValidationError{Plugin: plugin.Name, Msg: err.Error()})
                    result.Valid = false
                }
            }
        }

        // Check features supported (warning only)
        // ...
    }

    return result
}
```

**Decision**: Lazy validation by default, eager on command:

- Config load: Validate YAML syntax + compile patterns only
- First use: Validate patterns match resources (no-op if patterns valid)
- `finfocus config validate`: Full validation including plugin existence

**Rationale**: Fast startup for common case, full validation available.

---

## 8. Logging and Observability

**Question**: What should be logged for routing decisions?

**Findings** (per FR-020): Log routing decisions at debug level.

**Proposed Log Events**:

| Event | Level | Fields |
|-------|-------|--------|
| Route decision | DEBUG | resource_type, provider, matched_plugins, selected_plugin |
| Pattern match | TRACE | pattern, resource_type, matched |
| Priority selection | DEBUG | plugin, priority, position |
| Fallback triggered | INFO | failed_plugin, next_plugin, reason |
| All plugins failed | WARN | resource_type, plugins_tried |

**Log Format**:

```go
log.Debug().
    Ctx(ctx).
    Str("resource_type", resource.Type).
    Str("provider", provider).
    Strs("matched_plugins", pluginNames(matched)).
    Str("selected_plugin", selected.Name).
    Int("priority", priority).
    Msg("plugin routing decision")
```

**Decision**: Structured logging with trace correlation:

- Use zerolog as established in codebase
- Include trace_id from context
- DEBUG for routing decisions
- INFO for fallback events
- WARN for all-plugins-failed

**Rationale**: Consistent with existing logging patterns.

---

## Summary

All research questions resolved. No NEEDS CLARIFICATION items remaining.

| Question | Decision | Rationale |
|----------|----------|-----------|
| Provider extraction | Reuse `extractProviderFromType()` | Already implemented, tested |
| Metadata availability | Use existing, empty = global | Backward compatible |
| Pattern matching | Glob + regex with caching | Performance, flexibility |
| Priority selection | Priority-ordered list | Satisfies FR-012/13/14 |
| Fallback | Per-plugin config | User control |
| Capabilities | Config-driven with defaults | Forward compatible |
| Validation | Lazy default, eager on command | Fast startup |
| Logging | Zerolog with trace IDs | Consistent with codebase |
