# Research: Shared Plugin Processes Across Web Stacks

## Decision 1: Carry the pool on the context

- **Decision**: `pluginhost.WithPool(ctx, pool)` in `runWebStacks`;
  `registry.openPlugin` reads `pluginhost.PoolFromContext`.
- **Rationale**: Every plugin open of a stack runs under a context derived
  from the one `newWebStack` receives: the pipeline open (`pipeCtx`), reloads
  (`Session.Context()`, which is `pipeCtx`), and the scorer
  (`resolveScorerClient` runs under the pipeline or reload context). The
  stack's ESC source and install name already travel this way. Explicit
  parameters would change `openPlugins`, which a dozen CLI commands share, and
  the `overviewPipelineConfig.openPlugins` test seam, for a value only the
  multi-stack path sets.
- **Alternatives**: A `Registry.WithPool` option threaded through
  `openPlugins(ctx, adapter, audit, pool)` - rejected for the churn above.
  A process-global pool - rejected: it would change single-stack and CLI runs
  (FR-010).
- **Consequence**: Plugin probes (settings read, validate) run under an HTTP
  request context, which does not carry the pool, so they keep launching a
  short-lived process of their own, as the spec requires.

## Decision 2: Key on the launch environment plus the stack's ESC offer

- **Decision**: key = binary path + the binary file's size and modification
  time + SHA-256 over the launch `Delivered.Pairs`, override names, and every
  name, value, reference, and revision in the stack's `esc.Delivery` for that
  plugin. The restart after `GetPluginInfo` resolves its environment from that
  same `esc.Delivery` (`applyAdvertisedSettings` takes the launch inputs), so
  the key and the process cannot read ESC twice and disagree.
- **Rationale**: Before the first launch, `resolveLaunchInputs` (`resolveInjectedFrom` with no advertised settings)
  only allows ESC names listed in `esc.plugins.<name>.variables`. A name the
  plugin advertises is allowed only after `GetPluginInfo`, when
  `applyAdvertisedSettings` restarts the process. Two stacks with different
  ESC credentials for an advertised name therefore resolve the same launch
  environment and diverge only after the restart. Keying on the launch
  environment alone would hand stack B a process holding stack A's
  credential. Config files and the parent environment are process-wide, so
  the launch environment plus the ESC offer determine the final environment
  for a given binary.
- **Why not `Delivered.Digest`**: the issue proposed it. The digest hashes ESC
  values by reference and revision, which is equivalent in practice, but the
  invariant is "no value crosses", so the key hashes what the process is
  actually given. Reference and revision are hashed as well, because
  `DeliveredDigest`, which feeds cache keys, hashes ESC values by them: stacks
  that receive the same value from different environments keep separate
  processes and digests (FR-011). The file identity keeps a same-version
  reinstall from reusing the old binary's process.
- **Alternatives**: Remember each binary's advertised settings and key on the
  final environment - rejected: the first concurrent launches of a binary
  would still need the conservative key, so it adds a second code path for a
  small gain.

## Decision 3: Pool-owned launch context

- **Decision**: Each launch runs under
  `context.WithCancel(context.WithoutCancel(requesterCtx))`, cancelled by
  `context.AfterFunc(pool.ctx, ...)` or when the entry closes.
- **Rationale**: `exec.CommandContext` kills the plugin (and, through
  `cmd.Cancel`, its process group) when the launch context ends. Values
  (install name, delivered environment, log writer and tap, trace id, ESC
  source) still come from the requester, while cancellation belongs to the
  pool. The launch runs in a goroutine, so a cancelled requester returns at
  once without killing a launch another stack may be waiting for.

## Decision 4: Holds and release

- **Decision**: An entry counts holds; a requester takes a hold before
  waiting for the launch, and gives it back if it stops waiting or the launch
  fails. Each holder gets a copy of the `Client` (metadata deep-copied) whose
  `Close` releases once. The last release closes the process; a launch that
  finishes with no holds is closed at once.
- **Rationale**: `registry.mergeRegistryMetadata` writes into
  `Client.Metadata`, so a shared struct would race between stacks (FR-012).
  `Conn` and `API` are safe for concurrent use.

## Decision 5: Failures and crashes

- **Decision**: A failed launch is delivered to every waiter and removed from
  the pool. A launch every requester gave up on leaves the lookup at once and
  is closed when it finishes, so a launch that never answers does not block
  the key. An entry that was already running is checked with a `Name` RPC
  (5s) before it is handed out; a failure evicts it and the request launches a
  replacement. Holders of the evicted process keep it until they release.
- **Rationale**: gRPC connectivity state does not identify a dead process: a
  lost connection returns to `IDLE`. The `Name` RPC is cheap, plugins already
  answer it first on every launch, and acquisitions are rare (activation and
  reload). A false eviction launches a duplicate, which is safe.

## Decision 6: Shutdown

- **Decision**: `runWebStacks` stops the stacks, then calls `Pool.Close`,
  which closes every running entry, cancels the pool context (killing any
  launch in progress through `cmd.Cancel`), and waits for in-flight launches
  to finish closing.
- **Rationale**: Stacks release their holds first, so closing is normally a
  no-op; `Close` is the backstop that makes FR-009 unconditional. A process
  stopped by a last release or by an abandoned launch is tracked in a
  WaitGroup that `Close` waits on, so `Close` never returns while a stop is
  still in progress.

## Confirmed: `ReplacePluginEnvDigests`

`pluginhost.PluginEnvDigest()` has no caller outside tests; cost queries use
`CombinedDigest(clients)` per engine (`internal/engine/engine.go`). The
process-wide write in `openPlugins` therefore cannot mix stacks' cache keys and
is left unchanged.

## Confirmed: settings changes reach other stacks only on their next open

`wirePlugins` is wired per stack, and `Server` calls the stack session's
`ReloadPlugins`. A settings save reloads only that stack, with or without a
pool. Other stacks see the new settings at their next plugin open.
