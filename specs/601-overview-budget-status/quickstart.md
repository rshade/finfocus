# Quickstart: Overview Budget Status and Health

**Feature**: 601-overview-budget-status
**Date**: 2026-02-22

## What This Feature Does

Wires the existing budget health system into the `finfocus overview` command so users
see budget status alongside resource costs in all output modes (TUI, plain text, JSON).

## Prerequisites

- Budget configuration in `~/.finfocus/config.yaml` or `$PROJECT/.finfocus/config.yaml`
- At least one cost plugin installed that implements `GetBudgets` gRPC endpoint
- Existing `finfocus overview` command working with cost data

## Implementation Sequence

### Step 1: TUI Message and Model (Foundation)

Files: `internal/tui/overview_messages.go`, `internal/tui/overview_model.go`

1. Add `BudgetDataReadyMsg` type to messages file
2. Add `budgetResult`, `budgetErr`, `budgetLoaded` fields to `OverviewModel`
3. Add message handler in `Update()` method (store result, set loaded flag)

### Step 2: TUI Budget Rendering

Files: `internal/tui/overview_budget.go` (new), `internal/tui/overview_view.go`

1. Create `renderBudgetFooter(m OverviewModel) string` helper
2. Create `renderDetailBudgetStatus(m OverviewModel) string` helper
3. Insert footer call in `renderListView()` between table and status bar
4. Insert detail section call in `renderDetailView()` after recommendations

### Step 3: Budget Fetch in Background

File: `internal/cli/overview.go`

1. In `overviewInitAndEnrich()`, after plugin clients are created, launch a
   concurrent goroutine that calls `engine.GetBudgets(ctx, nil)` and sends
   `BudgetDataReadyMsg` to the TUI via `p.Send()`
2. Pass the engine instance to the goroutine (already available in scope)

### Step 4: Non-TTY Budget Integration

File: `internal/cli/overview.go`

1. Add `--exit-on-threshold`, `--exit-code`, `--budget-scope` flags to `NewOverviewCmd()`
2. In `executeOverview()`, after `renderOverviewOutput()`, calculate `totalCost` from
   enriched rows and call `evaluateBudgetStatus(cmd, costResults, totalCost)`
3. Return the error (which may be a `BudgetExitError` with exit code)

### Step 5: JSON Output

Files: `internal/engine/overview_types.go`, `internal/engine/overview_render.go`

1. Add `BudgetHealthSummary` type to `overview_types.go`
2. Add `Budgets` field to `OverviewJSONOutput` struct
3. In `RenderOverviewAsJSON()`, populate budgets from the budget result
4. Pass budget data through the rendering pipeline

### Step 6: Tests

Files: `internal/tui/overview_model_test.go`, `internal/tui/overview_budget_test.go`,
`internal/cli/overview_test.go`, `internal/engine/overview_render_test.go`

1. Test `BudgetDataReadyMsg` handling in model Update
2. Test footer rendering for OK, WARNING, CRITICAL, EXCEEDED states
3. Test footer hidden when no budgets / fetch failed
4. Test detail view budget section
5. Test non-TTY budget evaluation wiring
6. Test JSON output includes budgets
7. Test `--exit-on-threshold` flag behavior

## Key Integration Points

| What | Where | How |
|------|-------|-----|
| Budget fetch (TUI) | `overviewInitAndEnrich()` | Concurrent goroutine + `p.Send()` |
| Budget fetch (non-TTY) | `executeOverview()` | Sequential call to `evaluateBudgetStatus()` |
| Footer rendering | `renderListView()` | Insert between table and status bar |
| Detail section | `renderDetailView()` | Append after recommendations section |
| JSON output | `RenderOverviewAsJSON()` | Add `Budgets` field to output struct |
| Exit code | `executeOverview()` return | Return `BudgetExitError` from evaluation |

## Verification

```bash
make test          # All tests pass
make lint          # Linting passes
go test -v ./internal/tui/...    # TUI-specific tests
go test -v ./internal/cli/...    # CLI-specific tests
go test -v ./internal/engine/... # Engine-specific tests
```
