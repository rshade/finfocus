---
title: Budget Health & Forecasting Guide
description: Learn how the Budget Health Suite provides real-time status tracking, forecasting, and threshold alerting for cloud budgets.
layout: page
---

The Budget Health Suite provides real-time status tracking, forecasting, and
threshold alerting for your cloud budgets. This guide explains how the engine
processes budget data to help you stay on top of your cloud spending.

## Core Concepts

### Health Status

Each budget is assigned a health status based on its utilization percentage (`Current Spend / Limit`).

| Status       | Utilization | Meaning                                                            |
| :----------- | :---------- | :----------------------------------------------------------------- |
| **OK**       | 0% - 79%    | Budget is healthy. No immediate action required.                   |
| **WARNING**  | 80% - 89%   | Budget is approaching its limit. Monitor closely.                  |
| **CRITICAL** | 90% - 99%   | Budget is very close to its limit. Remediation action recommended. |
| **EXCEEDED** | 100%+       | Budget has exceeded its limit. Immediate action required.          |

### Aggregation Logic

When viewing multiple budgets, the overall health status is determined by the "worst-case" scenario:

`EXCEEDED` > `CRITICAL` > `WARNING` > `OK`

If _any_ budget in a group is `CRITICAL`, the group's status becomes `CRITICAL` (unless one is `EXCEEDED`).

## Forecasting

The engine predicts end-of-period spending using **Linear Extrapolation**.

### How it Works

The forecast assumes that spending will continue at the same average daily rate for the remainder of the period.

**Formula**:

```text
Forecast = CurrentSpend + (DailyRate × RemainingDays)
```

Where:

- `DailyRate = CurrentSpend / DaysElapsed`
- `RemainingDays = TotalDaysInPeriod - DaysElapsed`

### Behaviors

- **Mid-Period**: Calculates forecast based on run rate.
- **Period Not Started**: Forecast is 0.
- **Period Ended**: Forecast equals current spend.
- **Zero Spend**: Forecast is 0.

## Threshold Alerting

Thresholds allow you to define specific percentages that trigger alerts.

### Evaluation Types

1. **ACTUAL**: Triggered when _current spend_ exceeds the percentage.
2. **FORECASTED**: Triggered when _forecasted spend_ exceeds the percentage.

### Default Thresholds

If a budget comes from a plugin without specific thresholds defined, the engine applies these defaults automatically:

- **50% (ACTUAL)**: Mid-month check-in
- **80% (ACTUAL)**: Warning level (aligns with `WARNING` health status)
- **100% (ACTUAL)**: Limit reached (aligns with `EXCEEDED` health status)

## Filtering

You can filter budgets by their source provider. This is case-insensitive.

**Examples**:

- `aws-budgets`: Shows only budgets from AWS
- `kubecost`: Shows only budgets from Kubecost
- `gcp-billing`: Shows only budgets from GCP

## Integration Guide (Go SDK)

### Basic Usage

```go
// Create context
ctx := context.Background()

// 1. Get all budgets
result, err := engine.GetBudgets(ctx, nil)
if err != nil {
    log.Fatal(err)
}

// 2. Process results
for _, budget := range result.Budgets {
    fmt.Printf("Budget: %s\n", budget.Name)
    fmt.Printf("Health: %s (%.1f%%)\n",
        budget.HealthStatus,
        budget.Status.PercentageUsed,
    )
    fmt.Printf("Forecast: $%.2f\n", budget.Status.ForecastedSpend)
}

// 3. Check Summary
fmt.Printf("Total Budgets: %d\n", result.Summary.TotalBudgets)
fmt.Printf("Critical: %d\n", result.Summary.BudgetsCritical)
```

### With Filtering

```go
// Filter for specific providers
filter := &engine.BudgetFilterOptions{
    Providers: []string{"aws-budgets", "gcp-billing"},
}

result, err := engine.GetBudgets(ctx, filter)
if err != nil {
    log.Fatal(err)
}
```

## Performance

The budget engine is designed for high performance:

- **Health Calculation**: < 100ms for 1000 budgets
- **Filtering**: < 500ms for 1000 budgets
- **Stateless**: No database required; all calculations happen in-memory

## Troubleshooting

### "Unknown" Health Status

- **Cause**: Budget limit is 0 or missing.
- **Fix**: Ensure the plugin provides a valid budget limit.

### Forecast Seems Wrong

- **Cause**: Linear extrapolation assumes constant spending.
- **Note**: Spiky workloads (e.g., batch jobs) may cause inaccurate
  early-month forecasts. Accuracy improves as the period progresses.

### Currency Mismatches

- **Issue**: Summary totals cannot be calculated for mixed currencies.
- **Behavior**: The engine groups summaries by currency code (e.g., "USD",
  "EUR"). Ensure you check the `ByCurrency` map in the summary result.
