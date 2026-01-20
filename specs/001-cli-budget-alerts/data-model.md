# Data Model: Budgeting

This document defines the data structures used for budget configuration and evaluation.

## Entities

### BudgetConfig
Represents the spending limit and associated alerts for a period.

| Field | Type | Description |
|-------|------|-------------|
| Amount | float64 | Total spend limit for the period. |
| Currency | string | ISO 4217 currency code (e.g., "USD"). |
| Period | string | Time period for the budget (default: "monthly"). |
| Alerts | []AlertConfig | List of thresholds that trigger notifications. |

### AlertConfig
Defines a specific point in the budget where the user should be notified.

| Field | Type | Description |
|-------|------|-------------|
| Threshold | float64 | Percentage of budget consumed (e.g., 80.0). |
| Type | string | Evaluation type: "actual" or "forecasted". |

### BudgetStatus (Engine Output)
The result of evaluating a budget against current spend.

| Field | Type | Description |
|-------|------|-------------|
| Budget | BudgetConfig | The original budget configuration. |
| CurrentSpend | float64 | The current actual spend. |
| Percentage | float64 | Percentage of budget consumed. |
| ForecastedSpend | float64 | Estimated total spend by end of period. |
| ForecastPercentage | float64 | Percentage of budget forecasted to be consumed. |
| Alerts | []ThresholdStatus | Status of each configured alert. |

### ThresholdStatus
Status of an individual threshold.

| Field | Type | Description |
|-------|------|-------------|
| Threshold | float64 | The threshold percentage. |
| Type | string | "actual" or "forecasted". |
| Status | string | "OK", "APPROACHING", or "EXCEEDED". |

## Validation Rules
1. **Amount**: Must be greater than 0 (or exactly 0 to disable).
2. **Threshold**: Must be between 0 and 1000 (allowing for >100% alerts).
3. **Type**: Must be one of "actual" or "forecasted".
4. **Currency**: Must be non-empty if Amount > 0.
