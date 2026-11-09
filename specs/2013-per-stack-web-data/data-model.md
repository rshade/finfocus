# Data Model: Per-Stack-Safe Web Data Path

No persisted data changes. In-memory types (package `cli` unless noted):

## sharedStores

| Field | Meaning |
| --- | --- |
| `cache` | `cache.Cache`, nil when caching is off or failed to open |
| `history` | `history.Store`, nil when history is off or failed to open |
| `closeOnce` | guards the single close |

- Created once by `openSharedStores(ctx, cmd, cfg)`; failures log one warning
  (same text as today) and leave the field nil.
- `Close()` closes history then cache, once, idempotent.
- Invariant: no engine or pipeline built with a non-nil `sharedStores` opens a
  Bolt file.

## webStack

One stack's runtime: `data *webData`, `pipe *OverviewPipeline`, `session *webui.Session`,
`pipeCtx`/`cancel`, `pipeDone`. The passphrase channel is created in
`newWebStack` and held only by the pipeline config and the session's submit
callback.

- Owns: plugin set, query gate, snapshot, ESC source, passphrase channel.
- Shares (by reference, never closes): `*sharedStores`, config editor, log hub,
  audit context, dismissals, plugin catalog.

## stack ESC source

`*esc.Source` built from the root `Options()` with `Stack` set from the stack's
own settings file; held on that stack's `webData` and attached to its contexts.

## StackInfo (existing, `internal/pulumi`)

`Name`, `Current`, `URL`. `ListStacks` returns `[]StackInfo`; `ValidateStacks`
checks names against it.
