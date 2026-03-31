# Research: State-Only Flag for Overview Command

**Feature**: 607-state-only-flag
**Date**: 2026-03-30

## Research Questions

### R1: How does the existing `isStateOnly` code path work?

**Decision**: The `--state-only` flag will short-circuit the existing `isStateOnly`
decision logic by returning `true` immediately in `resolveIsStateOnly()` when
`params.stateOnly` is set.

**Rationale**: The codebase already has complete downstream handling for
`isStateOnly=true`:

- `loadAndProcessPlainOverview()` (plain text path, line 240): skips
  `DetectPendingChanges` when `isStateOnly` is true
- `loadAndProcessPlainOverview()` (line 248): uses `engine.NewRowsFromState()`
  instead of `MergeResourcesForOverview()`
- `overviewInitAndEnrich()` (TUI path, line 1141): calls `resolveIsStateOnly()`
  and branches on result
- `buildOverviewRows()` (line 1207): returns state-only rows when `isStateOnly`
  is true
- `OverviewSetStateOnlyMsg` is sent to TUI (line 1181-1183) with a preview
  command for on-demand `p` key preview

No new downstream code is needed — only the decision function and the plain-text
data loading path need changes.

**Alternatives considered**:

- *New code path*: Creating a separate `loadStateOnlyOverview()` function.
  Rejected because the existing state-only infrastructure is complete and
  battle-tested.
- *Environment variable*: Using `FINFOCUS_STATE_ONLY=1`. Rejected because
  CLI flags are more discoverable and consistent with the existing flag pattern.

### R2: Where exactly should the `--state-only` flag short-circuit?

**Decision**: Two insertion points:

1. **`resolveIsStateOnly()`** (TUI/interactive path): Add `if params.stateOnly { return true }` at the top. This handles the interactive TUI path where `overviewInitAndEnrich()` calls `resolveIsStateOnly()`.

2. **`loadPlainOverviewData()`** (plain text path): Add an early return after
   state loading when `params.stateOnly` is true. This skips change detection
   and the preview prompt entirely.

**Rationale**: These are the two entry points where the preview/state-only
decision is made — one for TUI mode, one for plain text mode. Both must respect
the flag.

**Alternatives considered**:

- *Single insertion at `executeOverview()`*: Would require restructuring both
  paths. More invasive than necessary.
- *Flag validation only in `NewOverviewCmd()`*: Cobra `MarkFlagsMutuallyExclusive`
  handles the `--state-only` + `--pulumi-json` conflict at the framework level,
  but doesn't handle the skip logic.

### R3: How should the `--state-only` + `--pulumi-json` conflict be handled?

**Decision**: Use Cobra's `cmd.MarkFlagsMutuallyExclusive("state-only", "pulumi-json")`
for the conflict check. This provides a standard Cobra error message and is
consistent with how other CLI tools handle flag conflicts.

**Rationale**: Cobra's built-in mutual exclusion provides:

- Consistent error formatting with the rest of the CLI
- Automatic `--help` annotation
- No custom validation code needed

**Alternatives considered**:

- *Manual validation in `executeOverview()`*: Would work but duplicates what
  Cobra provides natively. Adds maintenance burden.
- *Silently ignoring `--pulumi-json` when `--state-only` is set*: Confusing UX
  — user explicitly provided a plan file that would be silently ignored.

### R4: Should `--state-only` skip change detection in auto-detect mode?

**Decision**: Yes. When `--state-only` is set, the `pulumidetect.DetectChanges()`
call should also be skipped in both the TUI and plain text paths, since its only
purpose is to decide whether to run preview.

**Rationale**: Change detection examines the Pulumi manifest timestamp to
determine if preview is worthwhile. When the user explicitly says "state only",
this heuristic is irrelevant — the user has already made the decision. Skipping
it saves a small amount of time and avoids unnecessary filesystem access.

**Alternatives considered**:

- *Run change detection anyway*: No harm functionally, but wasteful when the
  result is always ignored. Misleading in debug logs.

### R5: Should `--state-only` + `--pulumi-state` be allowed?

**Decision**: Yes. `--state-only` with `--pulumi-state` is a valid combination:
load state from the explicit file and skip preview. This is consistent with
the flag's semantics (skip preview) and the file flag's semantics (use this
state file).

**Rationale**: A user providing `--pulumi-state state.json --state-only` is
saying "use this state file and don't bother with preview." This is a
reasonable and non-contradictory intent.

**Alternatives considered**:

- *Treat as conflict*: Overly restrictive — the combination is logical and useful.
