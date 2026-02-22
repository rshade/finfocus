# API Contract: Overview Budget Integration

**Feature**: 601-overview-budget-status
**Date**: 2026-02-22

## Overview

This feature adds no new external APIs. All changes are internal wiring of existing
budget APIs into the overview command's output layer. This contract documents the
internal function signatures and data shapes used for integration.

## CLI Interface Contract

### New Flags on `finfocus overview`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--exit-on-threshold` | `bool` | `false` | Exit with non-zero code when budget thresholds exceeded (non-TTY only) |
| `--exit-code` | `int` | `1` | Exit code to use when thresholds exceeded (0-255) |
| `--budget-scope` | `string` | `""` | Filter budget scopes: global, provider, provider=aws, tag, type |

### Exit Code Behavior

| Condition | TUI Mode | Non-TTY Mode |
|-----------|----------|--------------|
| No budgets configured | N/A (no footer) | Exit 0 |
| Budgets OK | Green footer | Exit 0 |
| Budgets WARNING | Yellow footer | Exit 0 |
| Budgets CRITICAL + `--exit-on-threshold` | Red footer (no exit) | Exit `--exit-code` |
| Budgets EXCEEDED + `--exit-on-threshold` | Red footer (no exit) | Exit `--exit-code` |
| Budget fetch error | No footer (logged) | Exit 0 (non-fatal) |

## JSON Output Contract

### `finfocus overview --output json`

Added `budgets` field to top-level output:

```json
{
  "metadata": { "...existing..." },
  "resources": [ "...existing..." ],
  "summary": { "...existing..." },
  "budgets": [
    {
      "budgetID": "monthly-infra",
      "budgetName": "Monthly Infrastructure",
      "provider": "aws",
      "health": "WARNING",
      "utilization": 85.2,
      "forecasted": 102.5,
      "currency": "USD",
      "limit": 10000.00,
      "currentSpend": 8520.00
    }
  ],
  "errors": [ "...existing..." ]
}
```

**Field presence rules**:

- `budgets` is omitted (`omitempty`) when no budgets are configured or fetch fails
- Each budget entry uses `engine.BudgetHealthResult` serialization
- Health values: `"OK"`, `"WARNING"`, `"CRITICAL"`, `"EXCEEDED"`

### `finfocus overview --output ndjson`

No changes. Budget data is NOT included in NDJSON output (stack-scoped, not
resource-scoped). Each line remains a single `OverviewRow`.

## Internal Function Contracts

### Budget Fetch (TUI Path)

```text
Caller: overviewInitAndEnrich() goroutine
Target: engine.GetBudgets(ctx, nil)
Returns: *engine.BudgetResult, error
Delivery: p.Send(tui.BudgetDataReadyMsg{Result, Error})
```

### Budget Evaluation (Non-TTY Path)

```text
Caller: executeOverview()
Target: evaluateBudgetStatus(cmd, costResults, totalCost)
Returns: error (nil or *BudgetExitError)
Pattern: Same as cost_projected.go usage
```

### TUI Message Contract

```text
Message: BudgetDataReadyMsg
  Result: *engine.BudgetResult (nil on failure)
  Error:  error (nil on success)
Handler: OverviewModel.Update() stores result, sets budgetLoaded=true
Effect:  Re-render triggers footer and detail view budget section
```
