# Research: Per-Stack-Safe Web Data Path

## R1 — Why a second store fails today

- **Decision**: Open the cache and history once and inject them.
- **Finding**: `newEngine` → `newEngineWithCacheAndHistory` calls
  `initCacheFromConfig` and `initHistoryFromConfig`, each `bolt.Open` with a
  lock timeout (cache 500 ms, history 1 s). bbolt takes a file lock per open
  handle, so a second open in the same process times out and the code logs
  "proceeding without cache/history". The overview pipeline additionally opens
  history in three places (`overview_pipeline.go` state snapshot, plan lineage
  after load, plan lineage after on-demand preview), each open-and-close.
- **Rationale**: `reloadPlugins` already reuses `d.cache` for this reason; the
  rule becomes uniform.
- **Alternatives**: retry the open with a longer timeout (hides the collision,
  serializes stacks); a process-global singleton (lint forbids globals and
  breaks parallel tests); per-stack files (loses shared dedupe, changes layout).

## R2 — Are the stores safe to share?

- **Decision**: Share the existing `*cache.BoltStore` and `*history.BoltStore`
  without extra locking.
- **Finding**: Both wrap a bbolt DB, whose transactions are goroutine-safe, and
  both guard `Close` with `sync.Once`. A multi-engine race test (`-race`) is
  part of the tasks to confirm.
- **Alternatives**: wrap in a mutex (needless; would serialize reads).

## R3 — History directory is per project, not per stack

- **Decision**: One shared history store per process.
- **Finding**: `resolveHistoryDir` depends on the config and the project dir,
  not the stack. The per-stack cost history is a different file
  (`<project>@<stack>.history.db`) opened elsewhere and out of scope.
- **Alternatives**: one store per stack (same file, collides again).

## R4 — ESC per stack

- **Decision**: Build a stack's source from the root source's `Options()` with
  `Stack` replaced by `LoadStackContext(projectDir, stack)`, and attach it to
  that stack's context. An empty stack name keeps the root source.
- **Finding**: `attachESCSource` runs once in root `PersistentPreRun` and calls
  `loadESCStack` → `LoadStackContext(projectDir, stackFlagValue(cmd))`. For
  `--stack dev,tailscale` no settings file matches; `absentWhenMissing` turns
  the not-found into an absent environment, so ESC is silently dropped.
  `LoadStackContext` with an empty stack reads the workspace-selected stack, so
  the empty case must keep the root source to stay identical.
  `esc.Source` is documented as not reusable across commands or reloads, so each
  stack needs its own instance; `Fresh()` already exists for reloads.
- **Rationale**: global and per-plugin configuration, `Warn`, `Getenv`, and
  budget stay identical (FR-007); only the stack environments differ.
- **Alternatives**: parse the list in `stackFlagValue` and pick the first
  (wrong for the others); make `esc.Source` multi-stack (large, leaks the web
  concern into `esc`).

## R5 — Passphrase wiring

- **Decision**: Each `webStack` creates its own unbuffered channel; its read
  side goes to that stack's pipeline and its send side to that stack's
  `webSessionOptions`, which already scopes `SubmitPassphrase` to the channel
  and the stack's `pipeCtx`. No relay type is needed.
- **Finding**: `Session` already owns `passphraseRequired` and one
  `SubmitPassphrase`; `/api/passphrase` is served per session (and per stack
  prefix by #2037). The only shared piece was the single channel and the single
  `OnPassphraseRequired` closure in `runWebSession`.
- **Alternatives**: a `passphraseRelay` wrapper (adds a type with no behavior);
  one channel keyed by stack (shared blocking, wrong cancel scope).

## R6 — Stack listing

- **Decision**: `ListStacks(ctx, projectDir) ([]StackInfo, error)` runs
  `pulumi stack ls --json` once; `GetCurrentStack` is rewritten on top of it,
  keeping its error text and `NoCurrentStackError`. `ValidateStacks(requested,
  available)` returns an error naming unknown stacks and the available ones.
- **Rationale**: one parser, one runner seam (`getRunner()`), cancellation
  handling already correct in `GetCurrentStack`.
- **Alternatives**: read `Pulumi.*.yaml` files (misses stacks with no settings
  file and ignores the backend).

## R7 — Where the single-stack path stands

- **Decision**: Refactor `runWebSession` to construct one `webStack` through
  the new constructors; keep the order of startup events.
- **Rationale**: proves the constructors on the shipping path, so #2039 does
  not first-use them.
