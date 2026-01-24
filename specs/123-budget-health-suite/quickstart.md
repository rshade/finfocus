# Quickstart: Budget Health Suite

**Feature**: 123-budget-health-suite
**Date**: 2026-01-24

## Overview

This guide shows how to use the Budget Health Suite engine functionality.

> **Note**: This feature covers engine functionality only. CLI commands are out of scope for this implementation.

## Prerequisites

- finfocus binary built (`make build`)
- At least one budget-aware plugin installed (e.g., aws-budgets, kubecost)
- Plugin configured with cloud credentials

## Usage Examples

### Example 1: Calculate Budget Health

```go
package main

import (
    "context"
    "fmt"

    "github.com/rshade/finfocus/internal/engine"
    pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func main() {
    ctx := context.Background()

    // Create engine with plugin clients
    eng := engine.New(clients, specLoader)

    // Get all budgets with health status
    result, err := eng.GetBudgets(ctx, nil)
    if err != nil {
        panic(err)
    }

    // Display health for each budget
    for _, budget := range result.Budgets {
        health := engine.CalculateBudgetHealth(budget)
        fmt.Printf("Budget: %s, Health: %s, Utilization: %.1f%%\n",
            budget.GetName(),
            health.String(),
            budget.GetStatus().GetPercentageUsed(),
        )
    }

    // Display summary
    fmt.Printf("\nSummary: %d total, %d OK, %d WARNING, %d CRITICAL, %d EXCEEDED\n",
        result.Summary.TotalBudgets,
        result.Summary.BudgetsOk,
        result.Summary.BudgetsWarning,
        result.Summary.BudgetsCritical,
        result.Summary.BudgetsExceeded,
    )
}
```

### Example 2: Filter by Provider

```go
// Filter to AWS budgets only
filter := &engine.BudgetFilterOptions{
    Providers: []string{"aws-budgets"},
}

result, err := eng.GetBudgets(ctx, filter)
if err != nil {
    panic(err)
}

fmt.Printf("Found %d AWS budgets\n", len(result.Budgets))
```

### Example 3: Multiple Providers (OR Logic)

```go
// Get budgets from AWS or GCP
filter := &engine.BudgetFilterOptions{
    Providers: []string{"aws-budgets", "gcp-billing"},
}

result, err := eng.GetBudgets(ctx, filter)
// Returns budgets matching either provider
```

### Example 4: Check Threshold Triggers

```go
for _, budget := range result.Budgets {
    // Evaluate thresholds (requires context for logging)
    evaluated := engine.EvaluateThresholds(
        ctx,
        budget,
        budget.GetStatus().GetCurrentSpend(),
        budget.GetStatus().GetForecastedSpend(),
    )

    for _, evalResult := range evaluated {
        if evalResult.Triggered {
            fmt.Printf("ALERT: Budget %s crossed %.0f%% threshold!\n",
                budget.GetName(),
                evalResult.Threshold.GetPercentage(),
            )
        }
    }
}
```

### Example 5: Forecasted Spending

```go
import "time"

// Calculate forecast for current month
now := time.Now()
periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

currentSpend := 1500.0 // $1500 spent so far

forecasted := engine.CalculateForecastedSpend(
    currentSpend,
    periodStart,
    periodEnd,
)

fmt.Printf("Current: $%.2f, Forecasted: $%.2f\n", currentSpend, forecasted)
```

### Example 6: Identify Critical Budgets

```go
result, _ := eng.GetBudgets(ctx, nil)

if result.Summary.OverallHealth >= pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL {
    fmt.Println("⚠️  ATTENTION REQUIRED:")
    for _, id := range result.Summary.CriticalBudgets {
        fmt.Printf("  - Budget ID: %s\n", id)
    }
}
```

## Health Status Reference

| Status   | Utilization | Meaning                                |
| -------- | ----------- | -------------------------------------- |
| OK       | 0-79%       | Budget is healthy, no action needed    |
| WARNING  | 80-89%      | Approaching limit, monitor closely     |
| CRITICAL | 90-99%      | Near limit, take action soon           |
| EXCEEDED | 100%+       | Over budget, immediate action required |

## Default Thresholds

When a budget has no configured thresholds, these defaults are applied:

| Percentage | Type   | Purpose           |
| ---------- | ------ | ----------------- |
| 50%        | ACTUAL | Midpoint check    |
| 80%        | ACTUAL | Warning alignment |
| 100%       | ACTUAL | Exceeded alert    |

## Error Handling

```go
result, err := eng.GetBudgets(ctx, filter)
if err != nil {
    // Fatal error - could not retrieve budgets
    log.Fatal(err)
}

// Check for partial errors (some budgets may have failed)
for _, e := range result.Errors {
    log.Printf("Warning: %v", e)
}

// Process successful budgets
for _, budget := range result.Budgets {
    // ...
}
```

## Performance Notes

- Health calculation for 1000 budgets: < 100ms
- Provider filtering for 1000 budgets: < 500ms
- All operations are stateless and parallelizable
