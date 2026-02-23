# Data Model: Overview Budget Status and Health

**Feature**: 601-overview-budget-status
**Date**: 2026-02-22

## Overview

This feature adds no new persistent entities. It wires existing budget types from the
engine and proto layers into the overview command's display and output layers. The
data model changes are limited to adding budget fields to existing overview output
structures and creating TUI message types for async budget delivery.

## Existing Entities (Reused, Not Modified)

### engine.BudgetResult

Source: `internal/engine/budget.go:35-39`

| Field | Type | Description |
|-------|------|-------------|
| Budgets | `[]*pbc.Budget` | Filtered budgets with health status |
| Summary | `*ExtendedBudgetSummary` | Aggregated statistics |
| Errors | `[]error` | Errors during processing |

### engine.ExtendedBudgetSummary

Source: `internal/engine/budget.go:43-50`

| Field | Type | Description |
|-------|------|-------------|
| *pbc.BudgetSummary | (embedded) | Base proto summary |
| ByProvider | `map[string]*pbc.BudgetSummary` | Per-provider breakdown |
| ByCurrency | `map[string]*pbc.BudgetSummary` | Per-currency breakdown |
| OverallHealth | `pbc.BudgetHealthStatus` | Worst-case health across all budgets |
| CriticalBudgets | `[]string` | IDs of critical/exceeded budgets |

### engine.BudgetHealthResult

Source: `internal/engine/budget.go:53+`

| Field | Type | Description |
|-------|------|-------------|
| BudgetID | `string` | Budget identifier |
| BudgetName | `string` | Human-readable name |
| Provider | `string` | Source provider |
| Health | `pbc.BudgetHealthStatus` | Calculated health |
| Utilization | `float64` | Current % used (0-100+) |
| Forecasted | `float64` | Forecasted % at period end |
| Currency | `string` | ISO 4217 code |
| Limit | `float64` | Budget limit |
| CurrentSpend | `float64` | Current spend |

### Health Status Thresholds

| Status | Range | Color |
|--------|-------|-------|
| OK | 0-79% | Green |
| WARNING | 80-89% | Yellow |
| CRITICAL | 90-99% | Red |
| EXCEEDED | 100%+ | Red (bold) |

## Modified Entities

### engine.StackContext (Add Budget Fields)

Source: `internal/engine/overview_types.go:363-375`

**New fields**:

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| BudgetHealth | `*BudgetHealthSummary` | `budgetHealth,omitempty` | Stack-level budget health summary |

### engine.OverviewJSONOutput (Add Budgets)

Source: `internal/engine/overview_render.go:374-380`

**New field**:

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| Budgets | `[]BudgetHealthResult` | `budgets,omitempty` | Per-budget health data |

### engine.BudgetHealthSummary (New Type)

Purpose: Lightweight budget health summary for StackContext JSON serialization.

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| OverallHealth | `string` | `overallHealth` | Health status label (OK, WARNING, CRITICAL, EXCEEDED) |
| TotalBudgets | `int` | `totalBudgets` | Number of budgets evaluated |
| CriticalCount | `int` | `criticalCount` | Number in CRITICAL or EXCEEDED state |

## New TUI Message Types

### tui.BudgetDataReadyMsg

Purpose: Delivers budget data from background goroutine to TUI model.

| Field | Type | Description |
|-------|------|-------------|
| Result | `*engine.BudgetResult` | Complete budget result (nil on failure) |
| Error | `error` | Fetch error (nil on success) |

Lifecycle: Sent once by the budget fetch goroutine in `overviewInitAndEnrich()`.
Received by `OverviewModel.Update()` which stores the result and triggers a re-render
of the list view footer and detail view budget section.

## New TUI Model Fields

### OverviewModel (Budget Extensions)

| Field | Type | Description |
|-------|------|-------------|
| budgetResult | `*engine.BudgetResult` | Budget data from plugins (nil until loaded) |
| budgetErr | `error` | Budget fetch error (nil on success) |
| budgetLoaded | `bool` | True after BudgetDataReadyMsg received |

State transitions:

- `budgetLoaded == false`: Footer hidden, detail view has no BUDGET STATUS section
- `budgetLoaded == true && budgetResult != nil`: Footer visible, detail view shows budget
- `budgetLoaded == true && budgetResult == nil`: Footer hidden (fetch failed or no budgets)

## Data Flow

```text
overviewInitAndEnrich() goroutine
    ├── enrichment (existing) ─── OverviewResourceLoadedMsg → TUI
    └── budget fetch (new, concurrent)
            │
            ├── engine.GetBudgets(ctx, nil)
            │       └── plugin gRPC: GetBudgets()
            │               └── budget health calculation
            │
            └── BudgetDataReadyMsg{Result, Error} → TUI
                    │
                    └── OverviewModel.Update()
                            ├── Store budgetResult
                            ├── Set budgetLoaded = true
                            └── Re-render (footer appears)
```

## Validation Rules

- Budget footer only renders when `budgetResult != nil` and `len(budgetResult.Budgets) > 0`
- Budgets with `Limit <= 0` are treated as disabled (excluded from footer aggregation)
- Mixed-currency footer shows only health badge + status label, no dollar amounts
- JSON output omits `budgets` field entirely when `len(Budgets) == 0`
- NDJSON output never includes budget data (stack-scoped, not resource-scoped)
