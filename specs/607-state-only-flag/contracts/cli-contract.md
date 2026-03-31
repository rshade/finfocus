# CLI Contract: --state-only Flag

**Feature**: 607-state-only-flag
**Date**: 2026-03-30

## Flag Definition

```text
--state-only    Skip pulumi preview (faster, but won't detect pending changes) [default: false]
```

## Flag Interactions

| Combination                         | Behavior                             | Error? |
|-------------------------------------|--------------------------------------|--------|
| `--state-only`                      | Auto-detect state, skip preview      | No     |
| `--state-only --stack prod`         | Auto-detect state for prod, skip     | No     |
| `--state-only --pulumi-state f.json`| Load state from file, skip preview   | No     |
| `--state-only --pulumi-json f.json` | N/A                                  | Yes: mutually exclusive |
| `--state-only --yes`                | Skip preview (--yes irrelevant)      | No     |
| `--state-only --plain`              | Plain text, state-only               | No     |
| `--state-only --output json`        | JSON output, state-only              | No     |
| `--state-only --output ndjson`      | NDJSON output, state-only            | No     |

## Error Messages

### Mutually exclusive flags

```text
if any flags in the group [state-only pulumi-json] are set none of the others can be; but [state-only pulumi-json] were all set
```

(Standard Cobra `MarkFlagsMutuallyExclusive` message format.)

## Output Contract

When `--state-only` is active:

- `StackContext.IsStateOnly` = `true`
- All `OverviewRow.Status` = `"active"`
- `StackContext.PendingChanges` = `0`
- Cost data (actual, projected, drift, recommendations) = identical to default
- JSON output includes `"is_state_only": true` in the stack context

When `--state-only` is not active:

- No behavioral change. All existing functionality preserved.
