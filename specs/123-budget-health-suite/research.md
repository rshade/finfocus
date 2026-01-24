# Research: Budget Health Suite

**Feature**: 123-budget-health-suite
**Date**: 2026-01-24
**Status**: Complete

## Proto Type Analysis

### Current Dependency

- **finfocus-spec**: v0.5.4 (already in go.mod)
- All required proto types are available

### Available Proto Types (from finfocus-spec)

#### Budget

```go
type Budget struct {
    Id         string              // Unique identifier (required)
    Name       string              // Human-readable name (required)
    Source     string              // Provider: "aws-budgets", "gcp-billing", "kubecost"
    Amount     *BudgetAmount       // Monetary limit and currency (required)
    Period     BudgetPeriod        // DAILY, WEEKLY, MONTHLY, QUARTERLY, YEARLY
    Filter     *BudgetFilter       // Scope restrictions (optional)
    Thresholds []*BudgetThreshold  // Alert points (optional)
    Status     *BudgetStatus       // Current spending state (optional)
    Metadata   map[string]string   // Provider-specific data (optional)
}
```

#### BudgetHealthStatus

```go
const (
    BUDGET_HEALTH_STATUS_UNSPECIFIED = 0
    BUDGET_HEALTH_STATUS_OK          = 1  // < 80% used
    BUDGET_HEALTH_STATUS_WARNING     = 2  // 80-89% used
    BUDGET_HEALTH_STATUS_CRITICAL    = 3  // 90-99% used
    BUDGET_HEALTH_STATUS_EXCEEDED    = 4  // >= 100% used
)
```

#### BudgetAmount

```go
type BudgetAmount struct {
    Limit    float64  // Maximum spending (required, > 0)
    Currency string   // ISO 4217 code (required, e.g., "USD")
}
```

#### BudgetStatus

```go
type BudgetStatus struct {
    CurrentSpend         float64            // Actual spending to date
    ForecastedSpend      float64            // Predicted end-of-period
    PercentageUsed       float64            // Current utilization %
    PercentageForecasted float64            // Forecasted utilization %
    Currency             string             // ISO 4217 currency code
    Health               BudgetHealthStatus // Overall health assessment
}
```

#### BudgetFilter

```go
type BudgetFilter struct {
    Providers     []string          // Cloud provider restrictions
    Regions       []string          // Geographic region restrictions
    ResourceTypes []string          // Resource type restrictions
    Tags          map[string]string // Tag-based filtering
}
```

#### BudgetThreshold

```go
type BudgetThreshold struct {
    Percentage  float64       // Threshold percentage (0-100+)
    Type        ThresholdType // ACTUAL or FORECASTED
    Triggered   bool          // Whether threshold has been crossed
    TriggeredAt *timestamp    // When threshold was triggered
}
```

#### BudgetSummary

```go
type BudgetSummary struct {
    TotalBudgets    int32  // Total count
    BudgetsOk       int32  // Healthy budgets (< 80%)
    BudgetsWarning  int32  // Approaching limits (80-89%)
    BudgetsCritical int32  // Near limits (90-99%)
    BudgetsExceeded int32  // Over budget (>= 100%)
}
```

## Design Decisions

### Decision 1: Health Status Thresholds

**Decision**: Use fixed thresholds matching proto enum semantics.

| Status | Utilization Range | Rationale |
| ------ | ----------------- | --------- |
| OK | 0-79% | Safe zone, no action needed |
| WARNING | 80-89% | Approaching limit, monitor |
| CRITICAL | 90-99% | Near limit, take action |
| EXCEEDED | 100%+ | Over budget, immediate action |

**Alternatives Considered**:

- Configurable thresholds: Rejected - adds complexity without clear use case
- Three-tier (OK/WARNING/EXCEEDED): Rejected - proto defines 4 levels

### Decision 2: Provider Filtering

**Decision**: Case-insensitive matching with OR logic for multiple providers.

**Rationale**:

- Users may type "AWS", "aws", or "Aws" - all should match
- Multiple providers = union (OR), not intersection (AND)
- Empty filter returns all budgets (no filtering)

**Alternatives Considered**:

- Case-sensitive matching: Rejected - poor UX
- AND logic for multiple providers: Rejected - rarely useful

### Decision 3: Currency Validation

**Decision**: Validate ISO 4217 format (exactly 3 uppercase letters).

**Rationale**:

- Standard format ensures consistency
- Empty currency treated as validation error
- No currency conversion - display in original currency

**Alternatives Considered**:

- Accept any string: Rejected - leads to inconsistent data
- Currency conversion: Rejected - out of scope, complex

### Decision 4: Forecasting Method

**Decision**: Linear extrapolation based on elapsed time and current spend.

```text
dailyRate = currentSpend / daysElapsed
forecastedSpend = dailyRate * totalDays
```

**Rationale**:

- Simple and predictable
- Works for any budget period (daily, weekly, monthly, etc.)
- No historical data required

**Alternatives Considered**:

- Weighted average (recent spend weighted higher): Deferred - requires more data
- Pattern-based (historical trends): Deferred - requires historical data storage

### Decision 5: Default Thresholds

**Decision**: Apply 50%, 80%, 100% actual thresholds when none configured.

**Rationale**:

- Industry-standard budget alert levels
- 50% = midpoint check
- 80% = warning (aligns with health WARNING)
- 100% = exceeded (aligns with health EXCEEDED)

**Alternatives Considered**:

- No defaults: Rejected - budgets without thresholds would have no alerts
- Single 100% threshold: Rejected - no early warning

### Decision 6: Aggregated Health Logic

**Decision**: Overall health = worst-case across all budgets.

**Rationale**:

- One EXCEEDED budget means overall health is EXCEEDED
- Ensures critical issues are surfaced
- Simple and intuitive for dashboards

**Alternatives Considered**:

- Average health: Rejected - hides critical outliers
- Weighted by budget size: Rejected - complex, unclear benefit

## Existing Engine Patterns

### Pattern: Error Handling

From existing engine code, errors are:

- Logged with zerolog structured logging
- Returned as error values (no panics)
- Aggregated in `CostResultWithErrors` pattern

**Apply to Budget Health**: Use same pattern for budget validation errors.

### Pattern: Filtering

From existing `FilterResources` function:

- Case-insensitive matching
- Multiple criteria support
- Empty filter = no filtering

**Apply to Budget Filtering**: Follow same pattern for provider filtering.

### Pattern: Test Structure

From existing engine tests:

- Table-driven tests with testify
- Subtests for each scenario
- require for setup, assert for verification

**Apply to Budget Tests**: Use same structure.

## Dependencies Verified

| Dependency | Version | Status |
| ---------- | ------- | ------ |
| finfocus-spec | v0.5.4 | Available in go.mod |
| testify | v1.11.1 | Available in go.mod |
| zerolog | v1.34.0 | Available in go.mod |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
| ---- | ---------- | ------ | ---------- |
| Proto type changes | Low | High | Pin to v0.5.4, monitor spec updates |
| Performance at scale | Low | Medium | Benchmark with 1000 budgets |
| Currency edge cases | Medium | Low | Strict validation, clear error messages |

## Conclusion

All technical unknowns resolved. Ready for Phase 1 design.
