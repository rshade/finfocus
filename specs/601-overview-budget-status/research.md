# Research: Overview Budget Status and Health

**Feature**: 601-overview-budget-status
**Date**: 2026-02-22

## Research Summary

All technical unknowns have been resolved through codebase exploration. No external
research was needed — this feature wires existing, proven budget infrastructure into
the overview command.

## Decision 1: Budget Data Source for Overview

**Decision**: Use `evaluateBudgetStatus()` from `common_execution.go` for the non-TTY
path, and `engine.GetBudgets()` for the TUI path.

**Rationale**: The non-TTY path already has `CostResult` data from enrichment, matching
the exact signature `evaluateBudgetStatus(cmd, results, totalCost)` expects. The TUI
path needs budget data delivered as a Bubble Tea message, so it calls
`engine.GetBudgets()` directly in a background goroutine (parallel to enrichment).

**Alternatives considered**:

- Using `evaluateBudgetStatus()` for both paths: Rejected because the TUI needs raw
  budget data (not just rendered output) for the footer and detail views.
- Using `engine.GetBudgets()` for both paths: Rejected because the non-TTY path should
  reuse the exact `evaluateBudgetStatus()` pattern from `cost projected` for consistency.

## Decision 2: Budget Flag Placement

**Decision**: Add `--exit-on-threshold`, `--exit-code`, and `--budget-scope` flags
directly to the overview command definition in `NewOverviewCmd()`.

**Rationale**: The overview command is a top-level command (added at `root.go:126`
alongside `cost`, `plugin`, etc.), NOT a child of the `cost` command. The budget
flags are `PersistentFlags` on the `cost` command and do not propagate to `overview`.
The flags must be added independently.

**Alternatives considered**:

- Making overview a child of `cost`: Rejected because it would be a breaking CLI change
  and the overview command serves a fundamentally different purpose (interactive dashboard
  vs. single-shot cost calculation).
- Sharing flag definitions via a helper function: Acceptable but unnecessary for 3 flags.
  Direct definition in `NewOverviewCmd()` is simpler and matches the existing pattern.

## Decision 3: TUI Budget Footer Placement

**Decision**: Render the budget health footer between the table and the status bar in
the list view. The footer is a single line when one budget/currency exists, expanding
to show per-budget lines when multiple budgets exist.

**Rationale**: The status bar is the bottom-most element containing keybinding hints.
Placing the budget footer above it keeps the status bar consistently at the bottom
while making budget health visible without scrolling. This follows the pattern of the
pagination footer which also sits between table and status bar.

**Alternatives considered**:

- Embedding budget info in the status bar: Rejected because the status bar is already
  dense (sort, filter, preview status, keybindings) and budget info needs more space.
- Placing budget info above the table: Rejected because it would push the table down,
  and budget data loads async (would cause layout shift).

## Decision 4: TUI Budget Footer Content for Multiple Budgets

**Decision**: When multiple budgets exist with the same currency, show aggregated
spend/limit/percentage on one line with worst-case health badge. When mixed currencies
exist, show only the overall health badge and status label (no aggregated dollar amounts).
When a single budget exists, show its spend/limit/percentage directly.

**Rationale**: Dollar aggregation across currencies is meaningless ($5,000 + 3,000 EUR
= ???). The health badge (green/yellow/red + status label) is always meaningful since
it reflects worst-case health from `ExtendedBudgetSummary.OverallHealth`. The detail
view provides per-budget breakdown for users who need specifics.

**Alternatives considered**:

- Always showing per-budget lines in the footer: Rejected because it could consume
  too many vertical lines with many budgets, and the detail view already serves this.
- Showing only the worst budget in the footer: Acceptable but less informative than
  the aggregated approach for same-currency budgets.

## Decision 5: Budget Fetch Timing in TUI

**Decision**: Launch budget fetch as a concurrent goroutine alongside enrichment in
`overviewInitAndEnrich()`. The budget fetch goroutine sends `BudgetDataReadyMsg`
to the TUI when complete, independently of the enrichment progress.

**Rationale**: Budget data comes from plugins via `engine.GetBudgets()` which makes
gRPC calls. This is independent of per-resource enrichment and can run concurrently.
Using the existing Bubble Tea `p.Send()` pattern (same as `OverviewDataReadyMsg`)
ensures thread-safe delivery. The footer appears as soon as budget data arrives,
regardless of enrichment progress.

**Alternatives considered**:

- Sequential fetch after enrichment: Rejected because it delays budget display
  unnecessarily. Budget data is independent of per-resource enrichment.
- Fetching budget data before enrichment: Rejected because it would add latency
  before the resource table appears.

## Decision 6: JSON Output Budget Placement

**Decision**: Add a `Budgets` field to the `OverviewJSONOutput` struct at the
top level (alongside `Metadata`, `Resources`, `Summary`, `Errors`). The field
contains an array of budget health objects.

**Rationale**: Budgets are stack-scoped, not resource-scoped. Placing them at the
top level of the JSON output (not inside `Metadata` or `Summary`) makes them
independently parseable by downstream tools. This mirrors the separate concern of
budget health vs. cost data.

**Alternatives considered**:

- Adding to `StackContext` (inside Metadata): Rejected because `StackContext` has a
  `Validate()` method and adding budget data would complicate validation. Budget data
  is also optional (nil when not configured), which doesn't fit StackContext's
  always-present semantics.
- Adding to `OverviewSummary`: Rejected because summary is about cost aggregates,
  not budget health. Mixing concerns would complicate consumers.

## Decision 7: NDJSON Budget Handling

**Decision**: Do not include budget data in NDJSON output lines. NDJSON is per-resource
streaming output; budgets are stack-scoped and do not belong on resource lines.

**Rationale**: NDJSON consumers expect one resource per line. Adding stack-level budget
data to each line is redundant and bloats output. Consumers needing budget data should
use `--output json` instead.

**Alternatives considered**:

- Emitting a special budget-only NDJSON line at the end: Rejected because it would
  break consumers expecting uniform resource lines. Type-discriminated NDJSON adds
  parsing complexity for marginal benefit.
