# Data Model: State-Only Flag

**Feature**: 607-state-only-flag
**Date**: 2026-03-30

## Entity Changes

This feature introduces no new entities. It adds a single field to an existing
struct.

### Modified Entity: `overviewParams`

**Location**: `internal/cli/overview.go`

| Field        | Type   | Default | Description                                     |
|--------------|--------|---------|-------------------------------------------------|
| `stateOnly`  | `bool` | `false` | When true, skip pulumi preview unconditionally  |

**Relationship to existing fields**:

- **Mutually exclusive with `pulumiJSON`**: If both are set, Cobra returns an
  error before `executeOverview()` runs.
- **Compatible with `pulumiState`**: `--pulumi-state` + `--state-only` loads
  state from the explicit file and skips preview.
- **Overrides `yes` for preview decision**: When `stateOnly` is true, the
  preview is skipped regardless of `--yes`. However, `--yes` still suppresses
  other prompts (e.g., confirmation).

### Unmodified Entities

The following entities are consumed but not modified by this feature:

- **`engine.StateResource`**: Populated from stack export; unaffected.
- **`engine.PlanStep`**: Will be `nil` when `stateOnly=true`; existing nil
  handling is already correct.
- **`engine.OverviewRow`**: Built via `NewRowsFromState()` when state-only;
  all rows have `StatusActive`. Existing behavior, unchanged.
- **`tui.OverviewSetStateOnlyMsg`**: Already sent when `isStateOnly=true` in
  the TUI path. The `--state-only` flag triggers this existing message.

## State Transitions

No new state transitions. The existing `isStateOnly` boolean flow is reused:

```text
Flag parsed → resolveIsStateOnly() returns true → skip preview →
  NewRowsFromState() → all rows StatusActive → TUI sends OverviewSetStateOnlyMsg
```
