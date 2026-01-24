# Data Model: Budget Health Suite

**Feature**: 123-budget-health-suite
**Date**: 2026-01-24

## Overview

This feature adds budget health functionality to the engine package. All types leverage existing proto definitions from finfocus-spec v0.5.4 - no new proto types are introduced.

## Proto Type Mappings

The engine works with proto types directly. No wrapper types needed.

### Core Types (from finfocus-spec)

| Proto Type               | Package                                          | Purpose                                   |
| ------------------------ | ------------------------------------------------ | ----------------------------------------- |
| `pbc.Budget`             | `github.com/rshade/finfocus-spec/gen/go/cost/v1` | Budget definition with status             |
| `pbc.BudgetAmount`       | same                                             | Monetary limit + currency                 |
| `pbc.BudgetStatus`       | same                                             | Current spending state                    |
| `pbc.BudgetFilter`       | same                                             | Scope restrictions                        |
| `pbc.BudgetThreshold`    | same                                             | Alert configuration                       |
| `pbc.BudgetSummary`      | same                                             | Aggregated health counts                  |
| `pbc.BudgetHealthStatus` | same                                             | Health status enum                        |
| `pbc.BudgetPeriod`       | same                                             | Budget period enum                        |
| `pbc.ThresholdType`      | same                                             | Threshold type enum (ACTUAL/FORECASTED)   |

## Engine Types (New)

### BudgetHealthResult

Returned by health calculation functions.

```go
// BudgetHealthResult contains health assessment for a single budget.
type BudgetHealthResult struct {
    BudgetID    string                  // Budget identifier
    BudgetName  string                  // Human-readable name
    Provider    string                  // Source provider (aws-budgets, kubecost, etc.)
    Health      pbc.BudgetHealthStatus  // Calculated health status
    Utilization float64                 // Current percentage used (0-100+)
    Forecasted  float64                 // Forecasted percentage at period end
    Currency    string                  // ISO 4217 currency code
    Limit       float64                 // Budget limit amount
    CurrentSpend float64                // Current spend amount
}
```

### BudgetFilterOptions

Options for filtering budgets.

```go
// BudgetFilterOptions contains criteria for filtering budgets.
type BudgetFilterOptions struct {
    Providers []string // Filter by provider names (case-insensitive, OR logic)
}
```

### ExtendedBudgetSummary

Extended summary with additional breakdowns.

```go
// ExtendedBudgetSummary provides detailed budget health breakdown.
type ExtendedBudgetSummary struct {
    *pbc.BudgetSummary           // Embedded proto summary
    ByProvider    map[string]*pbc.BudgetSummary // Per-provider breakdown
    ByCurrency    map[string]*pbc.BudgetSummary // Per-currency breakdown
    OverallHealth pbc.BudgetHealthStatus        // Worst-case health
    CriticalBudgets []string                     // IDs of critical/exceeded budgets
}
```

### ThresholdEvaluationResult

Result of threshold evaluation.

```go
// ThresholdEvaluationResult contains evaluated threshold state.
type ThresholdEvaluationResult struct {
    Threshold   *pbc.BudgetThreshold // Original threshold
    Triggered   bool                  // Whether threshold was crossed
    TriggeredAt time.Time            // When triggered (zero if not)
    SpendType   string               // "actual" or "forecasted"
}
```

## Validation Rules

### Budget Validation

| Field           | Rule                     | Error                                                |
| --------------- | ------------------------ | ---------------------------------------------------- |
| Amount.Limit    | Must be > 0              | "budget limit must be positive"                      |
| Amount.Currency | Must match `^[A-Z]{3}$`  | "invalid currency code: must be 3 uppercase letters" |
| Id              | Must be non-empty        | "budget id is required"                              |

### Currency Validation

```go
// ValidateCurrency checks if a currency code is valid ISO 4217 format.
// Returns error if invalid, nil if valid.
func ValidateCurrency(code string) error
```

- Must be exactly 3 characters
- Must be uppercase letters A-Z
- Empty string is invalid

## State Transitions

### Budget Health Status

```text
┌─────────┐
│   OK    │  (0-79% utilization)
└────┬────┘
     │ utilization >= 80%
     ▼
┌─────────┐
│ WARNING │  (80-89% utilization)
└────┬────┘
     │ utilization >= 90%
     ▼
┌─────────┐
│CRITICAL │  (90-99% utilization)
└────┬────┘
     │ utilization >= 100%
     ▼
┌─────────┐
│EXCEEDED │  (100%+ utilization)
└─────────┘
```

Transitions are one-way within a budget period. Status can only worsen as spend increases.

## Relationships

```text
Engine
  │
  └── uses ──► []*pbc.Budget (from plugin via GetBudgets RPC)
                   │
                   ├── has ──► *pbc.BudgetAmount (limit + currency)
                   │
                   ├── has ──► *pbc.BudgetStatus (current state)
                   │               │
                   │               └── contains ──► pbc.BudgetHealthStatus
                   │
                   ├── has ──► *pbc.BudgetFilter (scope)
                   │
                   └── has ──► []*pbc.BudgetThreshold (alerts)
```

## Data Flow

```text
Plugin (GetBudgets RPC)
         │
         ▼
    []*pbc.Budget
         │
         ├─── FilterBudgetsByProvider() ───► Filtered budgets
         │
         ├─── CalculateBudgetHealth() ───► []BudgetHealthResult
         │
         ├─── CalculateForecastedSpend() ───► Updated forecasts
         │
         ├─── EvaluateThresholds() ───► []ThresholdEvaluationResult
         │
         └─── CalculateBudgetSummary() ───► *ExtendedBudgetSummary
```

## Constants

```go
const (
    // Health threshold boundaries
    HealthThresholdWarning  = 80.0  // >= 80% = WARNING
    HealthThresholdCritical = 90.0  // >= 90% = CRITICAL
    HealthThresholdExceeded = 100.0 // >= 100% = EXCEEDED

    // Default threshold percentages
    DefaultThreshold50  = 50.0
    DefaultThreshold80  = 80.0
    DefaultThreshold100 = 100.0

    // Currency validation regex
    CurrencyCodePattern = `^[A-Z]{3}$`
)
```
