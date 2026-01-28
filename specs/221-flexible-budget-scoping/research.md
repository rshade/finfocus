# Research: Flexible Budget Scoping

**Feature Branch**: `221-flexible-budget-scoping`
**Date**: 2026-01-24
**Status**: Complete

## Phase 0 Research Findings

### 1. Current Budget System Architecture

**Decision**: Extend existing budget infrastructure rather than replace it.

**Rationale**: The current system already has solid foundations:

- `internal/config/budget.go` - Single global `BudgetConfig` with amount, currency,
  period, alerts, and exit code settings
- `internal/engine/budget_health.go` - Health status calculation (OK/WARNING/CRITICAL/
  EXCEEDED)
- `internal/engine/budget_forecast.go` - Linear extrapolation forecasting
- `internal/engine/budget_threshold.go` - Threshold evaluation (actual vs forecasted)
- Proto-level budgets from plugins already support metadata for provider, region,
  resource type, and tags

**Alternatives Considered**:

- Complete rewrite of budget system → Rejected: Too invasive, loses proven stability
- Plugin-only scoped budgets → Rejected: User-defined scopes need local config
  management, not plugin dependency

### 2. Configuration Schema Design

**Decision**: Hierarchical `budgets:` section with scope-specific subsections.

**Rationale**:

- Maintains backward compatibility with existing single-budget configs
- YAML structure naturally expresses hierarchy (global → provider → tag → type)
- Consistent with existing FinFocus config patterns in `~/.finfocus/config.yaml`
- Allows explicit priority weighting for tag budgets to resolve conflicts

**Schema Structure**:

```yaml
cost:
  budgets:
    global:
      amount: 5000.0
      currency: USD
      period: monthly
      alerts: [...]
    providers:
      aws:
        amount: 3000.0
        alerts: [...]
      gcp:
        amount: 2000.0
    tags:
      - selector: "team:platform"
        priority: 100
        amount: 1500.0
      - selector: "env:prod"
        priority: 50
        amount: 3000.0
    types:
      "aws:ec2/instance":
        amount: 500.0
```

**Alternatives Considered**:

- Flat list of budgets with scope field → Rejected: Less readable, harder to
  validate hierarchy
- Separate config files per scope → Rejected: Fragments user configuration,
  complicates loading

### 3. Cost Allocation Strategy

**Decision**: Multi-scope allocation with single-tag priority resolution.

**Rationale** (from spec clarifications):

- Resources ALWAYS count toward Global budget
- Resources ALWAYS count toward their Provider budget (if configured)
- Resources ALWAYS count toward their Resource Type budget (if configured)
- Resources count toward ONE tag budget only (highest priority wins)
- This prevents double-counting in tag budgets while maintaining hierarchical
  visibility

**Allocation Algorithm**:

```text
For each resource:
  1. Add cost to Global budget
  2. Extract provider from resource type (e.g., "aws" from "aws:ec2/instance")
  3. If provider budget exists → add cost
  4. If resource type budget exists → add cost
  5. For tag budgets:
     a. Find all matching tags
     b. If multiple matches exist:
        - If any lacks priority → emit warning
        - Select highest priority budget
     c. Add cost to selected tag budget only
```

**Alternatives Considered**:

- Proportional allocation across tag budgets → Rejected: Complex to explain,
  surprising behavior
- All matching tag budgets receive cost → Rejected: Spec clarification explicitly
  requires priority-based single allocation

### 4. CLI Display Strategy

**Decision**: Hierarchical grouped display with scope sections.

**Rationale** (from spec requirement FR-007):

- Global budget first (always visible)
- BY PROVIDER section with each provider's status
- BY TAG section with tag-based budgets
- BY TYPE section with resource-type budgets
- Clear visual hierarchy aids quick comprehension

**Display Order**:

```text
BUDGET STATUS
─────────────
GLOBAL
  Total: $5,000.00 | Spend: $3,250.00 (65%) | Status: OK

BY PROVIDER
  AWS: $3,000.00 | Spend: $2,100.00 (70%) | Status: OK
  GCP: $2,000.00 | Spend: $1,150.00 (58%) | Status: OK

BY TAG
  team:platform: $1,500.00 | Spend: $1,200.00 (80%) | Status: WARNING
  env:prod: $3,000.00 | Spend: $2,500.00 (83%) | Status: WARNING

BY TYPE
  aws:ec2/instance: $500.00 | Spend: $450.00 (90%) | Status: CRITICAL
```

**Alternatives Considered**:

- Single flat table → Rejected: Loses scope grouping, harder to scan
- Tree view with indentation → Rejected: TTY width constraints, table format
  preferred

### 5. Tag Matching Implementation

**Decision**: Exact tag key:value matching with wildcard support.

**Rationale**:

- Resources have tags as `map[string]string`
- Budget config specifies `selector: "key:value"` format
- Exact matching is deterministic and debuggable
- Future wildcard support (e.g., `team:*`) can be added without breaking changes

**Matching Rules**:

```text
Selector format: "key:value" or "key:*"

Match conditions:
- "team:platform" matches resource.tags["team"] == "platform"
- "env:*" matches resource.tags["env"] exists (any value)
```

**Edge Cases**:

- Missing tag → No match for that budget
- Multiple tags with budgets → Priority resolution
- Same priority collision → Warning emitted, first match wins (alphabetical)

**Alternatives Considered**:

- Regex-based matching → Rejected: Over-complex, error-prone, hard to document
- Label selector syntax (Kubernetes-style) → Rejected: Overkill for initial scope

### 6. Warning and Debug Output Strategy

**Decision**: Structured zerolog with budget-specific fields.

**Rationale** (from NFR-001, NFR-002):

- Use existing zerolog patterns in the codebase
- Add structured fields: `component`, `resource_type`, `matched_scopes`
- Debug-level output shows scope matching decisions
- Warning-level output for overlapping tag budgets without priority

**Log Examples**:

```text
DEBUG | component=budget | resource_type=aws:ec2/instance |
        matched_scopes=["global","provider:aws","type:aws:ec2/instance"]

WARN  | component=budget | message="overlapping tag budgets without priority" |
        tags=["team:platform","env:prod"] | resource=i-1234567890abcdef0
```

**Alternatives Considered**:

- Custom logger → Rejected: Constitution requires existing patterns
- JSON-only debug → Rejected: Console format needed for interactive use

### 7. Performance Considerations

**Decision**: Single-pass resource iteration with index-based scope lookup.

**Rationale** (from SC-003: <500ms for 10,000 resources):

- Pre-build scope indexes at config load time:
  - `providerBudgets: map[string]*ScopedBudget`
  - `tagBudgets: []TagBudget` (sorted by priority desc)
  - `typeBudgets: map[string]*ScopedBudget`
- Single iteration over resources, O(1) lookups per scope type
- Tag matching is O(t) where t = number of tag budgets (typically <20)

**Complexity Analysis**:

```text
For n resources, p provider budgets, t tag budgets, r type budgets:
- Time: O(n × (1 + 1 + t + 1)) = O(n × t)
- With t ≈ 10, effectively O(n)
- Space: O(p + t + r) for indexes
```

**Alternatives Considered**:

- Multi-pass with grouping → Rejected: Multiple iterations over large datasets
- Lazy evaluation → Rejected: Complicates progress reporting and streaming

### 8. Backward Compatibility Strategy

**Decision**: Detect and migrate legacy single-budget config automatically.

**Rationale**:

- Existing users have `cost.budgets.amount` as a single float
- New schema has `cost.budgets.global.amount` structure
- Auto-migration preserves user experience

**Migration Logic**:

```go
func migrateConfig(cfg *Config) {
    if cfg.Cost.Budgets.Amount > 0 && cfg.Cost.Budgets.Global == nil {
        // Legacy format detected
        cfg.Cost.Budgets.Global = &ScopedBudget{
            Amount:   cfg.Cost.Budgets.Amount,
            Currency: cfg.Cost.Budgets.Currency,
            Period:   cfg.Cost.Budgets.Period,
            Alerts:   cfg.Cost.Budgets.Alerts,
        }
        cfg.Cost.Budgets.Amount = 0 // Clear legacy field
        log.Info().Msg("migrated legacy budget config to global scope")
    }
}
```

**Alternatives Considered**:

- Require manual migration → Rejected: Poor UX, friction for existing users
- Parallel legacy support forever → Rejected: Maintenance burden, config ambiguity

### 9. Exit Code Integration

**Decision**: Maintain existing exit code behavior with per-scope override.

**Rationale**:

- Global `exit_on_threshold` and `exit_code` remain the default
- Scoped budgets inherit global settings unless explicitly overridden
- CI/CD pipelines rely on deterministic exit codes

**Exit Code Priority**:

```text
1. Check if any EXCEEDED scope has exit_on_threshold: true
2. Use the highest exit_code from triggered scopes
3. If multiple scopes trigger, worst exit code wins
```

**Alternatives Considered**:

- Scope-specific exit codes only → Rejected: Breaking change for existing users
- Exit on first exceeded → Rejected: May hide other critical budget states

### 10. Currency Validation

**Decision**: Enforce single currency across all scoped budgets.

**Rationale** (from Edge Cases in spec):

- MVP explicitly excludes multi-currency support (OOS-001)
- All budgets must use same currency as global budget
- Validate at config load time with clear error message

**Validation Logic**:

```go
func validateCurrency(budgets *BudgetsConfig) error {
    baseCurrency := budgets.Global.Currency
    for provider, budget := range budgets.Providers {
        if budget.Currency != "" && budget.Currency != baseCurrency {
            return fmt.Errorf("provider %q uses %s, but global uses %s: " +
                "multi-currency not supported in MVP",
                provider, budget.Currency, baseCurrency)
        }
    }
    // Similar checks for tags and types
    return nil
}
```

**Alternatives Considered**:

- Silent currency conversion → Rejected: Inaccurate, surprising behavior
- Currency per scope allowed → Rejected: Explicitly out of scope for MVP

## Dependencies

### Internal Dependencies

| Package           | Purpose                              | Impact                          |
| ----------------- | ------------------------------------ | ------------------------------- |
| `internal/config` | Budget config parsing                | New struct fields, migration    |
| `internal/engine` | Cost allocation, health calculation  | New scope-aware aggregation     |
| `internal/cli`    | Budget display, `--budget-scope`     | New rendering sections          |

### External Dependencies

No new external dependencies required. Uses existing:

- `github.com/spf13/cobra` - CLI flag handling
- `github.com/rs/zerolog` - Structured logging
- `gopkg.in/yaml.v3` - Config parsing (existing)

## Risk Assessment

| Risk                                     | Impact | Mitigation                              |
| ---------------------------------------- | ------ | --------------------------------------- |
| Config migration breaks existing setups  | High   | Extensive testing with legacy configs   |
| Tag priority conflicts confuse users     | Medium | Clear warnings, comprehensive docs      |
| Performance degradation at scale         | Medium | Benchmarks in CI, index optimization    |
| Scope overlap logic is complex           | Medium | Table-driven tests, debug logging       |

## Open Questions Resolved

All NEEDS CLARIFICATION items from spec have been resolved:

1. **Multiple tag matches** → Priority-based single allocation (spec clarification)
2. **Config format** → Extend `config.yaml` with hierarchical `budgets:` section
3. **CLI display** → Hierarchical grouped format (FR-007)
4. **Debug output** → Zerolog with structured fields (NFR-001, NFR-002)
5. **Currency handling** → Single currency enforced at load time (OOS-001)
