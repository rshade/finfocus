# Quickstart: Implementing the --state-only Flag

**Feature**: 607-state-only-flag
**Date**: 2026-03-30

## Overview

This feature adds a single boolean flag (`--state-only`) to the `overview`
command. The implementation reuses the existing `isStateOnly` code path —
no new downstream logic is needed.

## Implementation Steps

### 1. Add field to `overviewParams` struct

In `internal/cli/overview.go`, add `stateOnly bool` to the struct at line ~50.

### 2. Register flag in `NewOverviewCmd()`

Add `cmd.Flags().BoolVar(&params.stateOnly, "state-only", false, ...)` after
the existing flag registrations (around line 110).

Add `cmd.MarkFlagsMutuallyExclusive("state-only", "pulumi-json")` for
conflict validation.

### 3. Short-circuit in `resolveIsStateOnly()` (TUI path)

At the top of `resolveIsStateOnly()` (line ~1082), add:

```go
if params.stateOnly {
    return true
}
```

This ensures the TUI path (via `overviewInitAndEnrich`) respects the flag.

### 4. Short-circuit in `loadPlainOverviewData()` (plain text path)

In `loadPlainOverviewData()` (line ~341), after the explicit-files check and
before the auto-detect path, add an early return when `stateOnly` is true
that skips change detection and preview.

### 5. Update command help text and examples

Add `--state-only` example to the `Example` string in `NewOverviewCmd()`.

### 6. Update documentation

Add `--state-only` to the options table in `docs/commands/overview.md` and
add a usage example.

## Testing Checklist

- [ ] Flag is registered and appears in `--help` output
- [ ] `--state-only --pulumi-json plan.json` returns mutual exclusion error
- [ ] `resolveIsStateOnly()` returns `true` when `stateOnly=true` regardless of other inputs
- [ ] `loadPlainOverviewData()` returns `isStateOnly=true` and nil planSteps when flag set
- [ ] State-only overview with `--pulumi-state` + `--state-only` works
- [ ] `make test` passes
- [ ] `make lint` passes

## Key Insight

The critical realization is that `isStateOnly=true` is already a fully
supported mode in both TUI and plain text paths. This feature is purely
about giving users a flag to force that mode, rather than relying on
automatic change detection heuristics.
