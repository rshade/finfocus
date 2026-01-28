# Data Model: Flexible Budget Scoping

**Feature Branch**: `221-flexible-budget-scoping`
**Date**: 2026-01-24
**Status**: Design Complete

## Entity Definitions

### 1. BudgetsConfig (Root Container)

**Location**: `internal/config/budget_scoped.go`

**Purpose**: Root configuration container for all budget scopes. Replaces the flat
`BudgetConfig` for new hierarchical structure while maintaining backward compatibility.

```go
// BudgetsConfig holds all budget scope configurations.
// It supports a global fallback budget and optional provider, tag, and type scopes.
type BudgetsConfig struct {
    // Global is the fallback budget that applies to all resources.
    // Required if any scoped budgets are defined.
    Global *ScopedBudget `yaml:"global,omitempty"`

    // Providers maps cloud provider names (aws, gcp, azure) to their budgets.
    // Provider names are case-insensitive during matching.
    Providers map[string]*ScopedBudget `yaml:"providers,omitempty"`

    // Tags defines budgets scoped by resource tags with priority ordering.
    // Higher priority values take precedence when a resource matches multiple tags.
    Tags []TagBudget `yaml:"tags,omitempty"`

    // Types maps resource type patterns to their budgets.
    // Patterns use exact matching (e.g., "aws:ec2/instance").
    Types map[string]*ScopedBudget `yaml:"types,omitempty"`

    // --- Legacy fields for backward compatibility ---

    // Amount is deprecated. Use Global.Amount instead.
    // If set and Global is nil, auto-migration converts to Global scope.
    Amount float64 `yaml:"amount,omitempty"`

    // Currency is deprecated. Use Global.Currency instead.
    Currency string `yaml:"currency,omitempty"`

    // Period is deprecated. Use Global.Period instead.
    Period string `yaml:"period,omitempty"`

    // Alerts is deprecated. Use Global.Alerts instead.
    Alerts []AlertConfig `yaml:"alerts,omitempty"`

    // ExitOnThreshold applies to all scopes unless overridden.
    ExitOnThreshold bool `yaml:"exit_on_threshold,omitempty"`

    // ExitCode is the default exit code when thresholds are exceeded.
    ExitCode int `yaml:"exit_code,omitempty"`
}
```

**Validation Rules**:

- If `Providers`, `Tags`, or `Types` are non-empty, `Global` must be defined
- All currency values must match `Global.Currency` (or inherit if empty)
- `ExitCode` must be 0-255

**State Transitions**: N/A (configuration entity, immutable after load)

---

### 2. ScopedBudget (Budget Definition)

**Location**: `internal/config/budget_scoped.go`

**Purpose**: Represents a single budget with its amount, currency, period, and
alert thresholds. Used for global, provider, and type scopes.

```go
// ScopedBudget defines a budget limit with alert thresholds.
type ScopedBudget struct {
    // Amount is the budget limit in the specified currency.
    // Must be positive (zero disables the budget).
    Amount float64 `yaml:"amount"`

    // Currency is the ISO 4217 currency code (e.g., "USD", "EUR").
    // If empty, inherits from global budget.
    Currency string `yaml:"currency,omitempty"`

    // Period defines the budget time window. Only "monthly" is supported.
    // If empty, defaults to "monthly".
    Period string `yaml:"period,omitempty"`

    // Alerts defines threshold percentages and their types.
    // If empty, uses default thresholds (50%, 80%, 100% actual).
    Alerts []AlertConfig `yaml:"alerts,omitempty"`

    // ExitOnThreshold overrides the global setting for this scope.
    // If nil, inherits from BudgetsConfig.ExitOnThreshold.
    ExitOnThreshold *bool `yaml:"exit_on_threshold,omitempty"`

    // ExitCode overrides the global exit code for this scope.
    // If nil, inherits from BudgetsConfig.ExitCode.
    ExitCode *int `yaml:"exit_code,omitempty"`
}
```

**Validation Rules**:

- `Amount` must be >= 0 (0 means disabled)
- `Currency` must be valid ISO 4217 code if specified
- `Period` must be "monthly" (only supported value)
- `ExitCode` must be 0-255 if specified

---

### 3. TagBudget (Tag-Scoped Budget)

**Location**: `internal/config/budget_scoped.go`

**Purpose**: Represents a budget scoped by resource tags with priority ordering
for conflict resolution.

```go
// TagBudget defines a budget scoped by a tag selector with priority ordering.
type TagBudget struct {
    // Selector is the tag pattern in "key:value" or "key:*" format.
    // "key:value" matches exact tag values.
    // "key:*" matches any resource with the specified tag key.
    Selector string `yaml:"selector"`

    // Priority determines which tag budget receives cost when a resource
    // matches multiple tag selectors. Higher values take precedence.
    // If multiple budgets have the same priority, a warning is emitted
    // and the first alphabetically wins.
    Priority int `yaml:"priority,omitempty"`

    // ScopedBudget embeds the budget configuration.
    ScopedBudget `yaml:",inline"`
}
```

**Validation Rules**:

- `Selector` must match pattern `^[a-zA-Z0-9_-]+:(\\*|[a-zA-Z0-9_-]+)$`
- `Priority` should be unique across tag budgets (warning if duplicated)

**Selector Examples**:

| Selector         | Matches                                       |
| ---------------- | --------------------------------------------- |
| `team:platform`  | Resources with tag `team=platform`            |
| `env:prod`       | Resources with tag `env=prod`                 |
| `team:*`         | Any resource with a `team` tag (any value)    |
| `cost-center:*`  | Any resource with a `cost-center` tag         |

---

### 4. AlertConfig (Existing, Unchanged)

**Location**: `internal/config/budget.go`

**Purpose**: Defines alert threshold configuration. Already exists, no changes needed.

```go
// AlertConfig defines a threshold alert configuration.
type AlertConfig struct {
    // Threshold is the percentage value (0-1000).
    Threshold float64 `yaml:"threshold"`

    // Type is the alert type: "actual" or "forecasted".
    Type AlertType `yaml:"type"`
}
```

---

### 5. ScopedBudgetStatus (Runtime State)

**Location**: `internal/engine/budget_scope.go`

**Purpose**: Runtime state for a scoped budget after cost allocation and
health evaluation.

```go
// ScopedBudgetStatus represents the evaluated state of a scoped budget.
type ScopedBudgetStatus struct {
    // ScopeType identifies the budget scope category.
    ScopeType ScopeType

    // ScopeKey is the identifier within the scope type.
    // For provider: "aws", "gcp", etc.
    // For tag: "team:platform", "env:prod", etc.
    // For type: "aws:ec2/instance", etc.
    // For global: empty string.
    ScopeKey string

    // Budget is the configured budget for this scope.
    Budget ScopedBudget

    // CurrentSpend is the total cost allocated to this scope.
    CurrentSpend float64

    // Percentage is CurrentSpend / Budget.Amount * 100.
    Percentage float64

    // ForecastedSpend is the projected end-of-period spend.
    ForecastedSpend float64

    // ForecastPercentage is ForecastedSpend / Budget.Amount * 100.
    ForecastPercentage float64

    // Health is the overall health status (OK, WARNING, CRITICAL, EXCEEDED).
    Health BudgetHealthStatus

    // Alerts is the list of evaluated threshold statuses.
    Alerts []ThresholdStatus

    // MatchedResources is the count of resources allocated to this scope.
    MatchedResources int

    // Currency is the budget currency for display.
    Currency string
}

// ScopeType identifies the category of a budget scope.
type ScopeType string

const (
    ScopeTypeGlobal   ScopeType = "global"
    ScopeTypeProvider ScopeType = "provider"
    ScopeTypeTag      ScopeType = "tag"
    ScopeTypeType     ScopeType = "type"
)
```

---

### 6. BudgetAllocation (Allocation Tracking)

**Location**: `internal/engine/budget_scope.go`

**Purpose**: Tracks which scopes a resource's cost was allocated to for
debugging and audit purposes.

```go
// BudgetAllocation tracks cost allocation for a single resource.
type BudgetAllocation struct {
    // ResourceID is the unique identifier of the resource.
    ResourceID string

    // ResourceType is the full type string (e.g., "aws:ec2/instance").
    ResourceType string

    // Provider is the extracted provider from the resource type.
    Provider string

    // Cost is the resource's cost that was allocated.
    Cost float64

    // AllocatedScopes lists all scopes that received this resource's cost.
    // Format: "global", "provider:aws", "tag:team:platform", "type:aws:ec2/instance"
    AllocatedScopes []string

    // MatchedTags lists all tags that matched tag budgets for this resource.
    // If multiple matched, only the highest priority receives cost.
    MatchedTags []string

    // SelectedTagBudget is the tag budget that received the cost allocation.
    // Empty if no tag budget matched or no tag budgets configured.
    SelectedTagBudget string

    // Warnings contains any warnings generated during allocation.
    // e.g., "overlapping tag budgets without priority"
    Warnings []string
}
```

---

### 7. ScopedBudgetResult (Aggregated Output)

**Location**: `internal/engine/budget_scope.go`

**Purpose**: Complete result of scoped budget evaluation for CLI display.

```go
// ScopedBudgetResult contains all evaluated scoped budgets and summaries.
type ScopedBudgetResult struct {
    // Global is the global budget status (always present if configured).
    Global *ScopedBudgetStatus

    // ByProvider maps provider names to their budget statuses.
    ByProvider map[string]*ScopedBudgetStatus

    // ByTag contains tag budget statuses in priority order.
    ByTag []*ScopedBudgetStatus

    // ByType maps resource types to their budget statuses.
    ByType map[string]*ScopedBudgetStatus

    // OverallHealth is the worst health status across all scopes.
    OverallHealth BudgetHealthStatus

    // CriticalScopes lists scope identifiers with CRITICAL or EXCEEDED status.
    CriticalScopes []string

    // Allocations contains per-resource allocation details (debug mode only).
    Allocations []BudgetAllocation

    // Warnings contains all warnings generated during evaluation.
    Warnings []string
}
```

---

## Relationships

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                          BudgetsConfig                                  │
│  (root container in ~/.finfocus/config.yaml)                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────────┐                                                   │
│   │  Global          │ ────────────────────────────────────────────────┐ │
│   │  (ScopedBudget)  │                                                 │ │
│   └──────────────────┘                                                 │ │
│                                                                        │ │
│   ┌──────────────────────────────────────────────────┐                 │ │
│   │  Providers: map[string]*ScopedBudget             │                 │ │
│   │    aws  ─────────────────────────────────────────┼─────────────────│ │
│   │    gcp  ─────────────────────────────────────────┼─────────────────│ │
│   │    azure ────────────────────────────────────────┼─────────────────│ │
│   └──────────────────────────────────────────────────┘                 │ │
│                                                                        │ │
│   ┌──────────────────────────────────────────────────┐                 │ │
│   │  Tags: []TagBudget                               │                 │ │
│   │    ┌─────────────────────────────────────────┐   │                 │ │
│   │    │ Selector: "team:platform"               │   │                 │ │
│   │    │ Priority: 100                           │───┼─────────────────│ │
│   │    │ ScopedBudget (embedded)                 │   │   inherits      │ │
│   │    └─────────────────────────────────────────┘   │   currency      │ │
│   │    ┌─────────────────────────────────────────┐   │   from global   │ │
│   │    │ Selector: "env:prod"                    │   │                 │ │
│   │    │ Priority: 50                            │───┼─────────────────┤ │
│   │    │ ScopedBudget (embedded)                 │   │                 │ │
│   │    └─────────────────────────────────────────┘   │                 │ │
│   └──────────────────────────────────────────────────┘                 │ │
│                                                                        │ │
│   ┌──────────────────────────────────────────────────┐                 │ │
│   │  Types: map[string]*ScopedBudget                 │                 │ │
│   │    "aws:ec2/instance" ───────────────────────────┼─────────────────┤ │
│   │    "aws:rds/instance" ───────────────────────────┼─────────────────┘ │
│   └──────────────────────────────────────────────────┘                   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘

                              ▼ evaluation

┌─────────────────────────────────────────────────────────────────────────┐
│                       ScopedBudgetResult                                │
│  (runtime evaluation output)                                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   Global: *ScopedBudgetStatus ──────────────────────────────────────┐    │
│                                                                      │   │
│   ByProvider: map[string]*ScopedBudgetStatus                         │   │
│     "aws" ──────────────────────────────────────────────────────────┐│   │
│     "gcp" ──────────────────────────────────────────────────────────││   │
│                                                                      ││  │
│   ByTag: []*ScopedBudgetStatus (sorted by priority desc)             ││  │
│     "team:platform" ────────────────────────────────────────────────┐││  │
│     "env:prod" ─────────────────────────────────────────────────────│││  │
│                                                                      │││ │
│   ByType: map[string]*ScopedBudgetStatus                             │││ │
│     "aws:ec2/instance" ─────────────────────────────────────────────┐│││ │
│                                                                      │││││
│   OverallHealth: "worst of all scopes" ◄────────────────────────────┼┼┼┼┤│
│   CriticalScopes: []string ◄────────────────────────────────────────┼┼┼┼┘│
│   Warnings: []string                                                 ││││ │
│   Allocations: []BudgetAllocation (debug only)                       ││││ │
│                                                                      ││││ │
└──────────────────────────────────────────────────────────────────────┼┼┼┼─┘
                                                                       ││││
                              ▼ each status contains                   ││││
                                                                       ││││
┌──────────────────────────────────────────────────────────────────────┼┼┼┼──┐
│                      ScopedBudgetStatus                              ◄┴┴┴┘ │
├─────────────────────────────────────────────────────────────────────────────┤
│  ScopeType:  "global" | "provider" | "tag" | "type"                         │
│  ScopeKey:   "" | "aws" | "team:platform" | "aws:ec2/instance"              │
│  Budget:     ScopedBudget (configuration)                                   │
│  CurrentSpend, Percentage, ForecastedSpend, ForecastPercentage              │
│  Health:     OK | WARNING | CRITICAL | EXCEEDED                             │
│  Alerts:     []ThresholdStatus                                              │
│  MatchedResources: int                                                      │
│  Currency:   string                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Configuration Example

```yaml
# ~/.finfocus/config.yaml

cost:
  budgets:
    # Global budget - fallback for all resources
    global:
      amount: 5000.00
      currency: USD
      period: monthly
      alerts:
        - threshold: 50.0
          type: actual
        - threshold: 80.0
          type: actual
        - threshold: 100.0
          type: forecasted

    # Per-provider budgets
    providers:
      aws:
        amount: 3000.00
        # currency inherited from global
        alerts:
          - threshold: 80.0
            type: actual
      gcp:
        amount: 2000.00

    # Tag-based budgets with priority
    tags:
      - selector: "team:platform"
        priority: 100
        amount: 1500.00
        exit_on_threshold: true
        exit_code: 2
      - selector: "env:prod"
        priority: 50
        amount: 3000.00
      - selector: "cost-center:*"
        priority: 10
        amount: 500.00

    # Resource type budgets
    types:
      "aws:ec2/instance":
        amount: 500.00
        alerts:
          - threshold: 90.0
            type: actual
      "aws:rds/instance":
        amount: 800.00

    # Default exit code settings
    exit_on_threshold: true
    exit_code: 1
```

---

## Migration from Legacy Config

The system auto-detects and migrates legacy single-budget configurations:

**Legacy Format**:

```yaml
cost:
  budgets:
    amount: 5000.0
    currency: USD
    period: monthly
    alerts:
      - threshold: 80.0
        type: actual
```

**Auto-Migrated To**:

```yaml
cost:
  budgets:
    global:
      amount: 5000.0
      currency: USD
      period: monthly
      alerts:
        - threshold: 80.0
          type: actual
```

Migration happens transparently at config load time. A log message is emitted
at INFO level: `"migrated legacy budget config to global scope"`.
