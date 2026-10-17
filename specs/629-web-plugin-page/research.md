# Research: Web Plugin Page

## Decision: Reuse the config writer for plugin settings

**Decision**: Settings preview and save call the existing `ConfigPreview` and
`ConfigSave` session callbacks with `plugins.<name>.<key>` changes. The
Plugins page does not grow a second file writer.

**Rationale**: Specification 628 already preserves comments, checks
`baseVersion`, locks, masks credential keys, and rejects a stale view.
`internal/config/patch.go` is that writer. A parallel writer would drift.

**Alternatives considered**: A plugin-settings file next to the install.
Rejected because the command line and the Config page already store
`plugins.*` in `config.hujson`, and the two would disagree.

## Decision: Deliver settings as environment variables, resolved before the cost call

**Decision**: `pluginhost.NewClient` (`internal/pluginhost/host.go`) already
calls `GetPluginInfo` after `Name`. Extend that handshake:

1. Derive the install name from the binary path (`<plugin dir>/<name>/<version>/…`).
2. Inject resolved variables through a context value that `ProcessLauncher.startPlugin`
   appends in `process.go` after `FINFOCUS_PLUGIN_PORT`. Existing keys in the
   parent environment are not replaced.
3. After `GetPluginInfo`, if the advertised `env` names need values that were
   not injected, close the process and start it once more with those values,
   then call `GetPluginInfo` again. Do not loop.
4. Keep the resolved pairs in memory on the client. Do not write a new
   settings database.

**Rationale**: Plugins read their own variable names (`TYPESAFE_API_KEY`,
`FINFOCUS_PROMETHEUS_URL`, `FINFOCUS_RECORDER_MOCK_RESPONSE`). Those names
are not knowable until `GetPluginInfo`, and that RPC already runs on every
client. One restart makes the same launch the one that receives the saved
settings, which is what the dashboard query and a cold `finfocus cost` both
are. Memory-only state stays inside the constitution's "no new store" rule:
deleting nothing is required, and a process with no prior state still runs.

**Alternatives considered**:

- Persist descriptors under the FinFocus home. That is a new store and would
  need a constitution amendment. The handshake makes it unnecessary.
- Inject only `FINFOCUS_PLUGIN_<NAME>_<KEY>`. The shipped plugins do not read
  that name, so the form would save values they never see.
- Require `expires_at = now` instead of a cache digest. The constitution's
  operational rule is that anything newly sent to a plugin is part of the
  projected cache key. A digest busts stale prices even when a plugin forgets
  to expire. Plugins that depend on state core cannot see still set
  `expires_at` themselves. That duty is unchanged.

## Decision: Cache digest of delivered values

**Decision**: Hash the effective delivered pairs (plugin, variable name,
value), not the raw secret, and append `/penv-<digest>` to projected and
actual plugin cache keys, and to the `checkPluginSupports` key, only when
the digest is non-empty. An empty digest keeps today's keys so existing
entries still hit.

**Rationale**: A parent override, a project value, a global value, and a
descriptor default can all change the answer. Hashing keeps secrets out of
the key string and out of logs.

**Alternatives considered**: Invalidate the whole Bolt database on any
settings save. Heavier, and it misses a parent-environment change that never
touches the file.

## Decision: Typed progress beside the string callback

**Decision**: Add `ProgressEvent` and an optional `InstallOptions.OnEvent`.
The existing `progress func(string)` stays. Every current string is still
emitted unchanged. `OnEvent` also carries `Phase` (`resolve`, `download`,
`verify`, `extract`, `validate`, `done`), byte counts from `DownloadAsset`,
and a warning flag.

**Rationale**: CLI tests and user-visible install text stay stable. The page
needs structure the free-text lines do not have.

**Alternatives considered**: Replace `func(string)` with the struct. That
touches every installer call and makes "output unchanged" a rewrite instead
of a regression test.

## Decision: Web installs are a policy on the existing installer

**Decision**: `InstallOptions` gains:

- `CatalogOnly`: reject URL and `owner/repo` specifiers before any download.
- `SkipChecksum` remains available to the CLI and is ignored when
  `CatalogOnly` is set.
- `RequireChecksumFile`: a missing `checksums.txt` returns
  `ErrChecksumUnavailable` instead of the current warn-and-continue, unless
  `AcceptMissingChecksum` is set. A confirmed mismatch stays fatal for
  everyone (`ErrChecksumMismatch`).
- `AcceptFallbackVersion`: when the requested release has no assets, return
  `*NoAssetsError` (via `errors.As`) with the fallback version filled in and
  do not install, unless this field equals that fallback version.

The CLI leaves the new fields zero. Its warn-and-continue checksum behavior
and its interactive fallback prompt stay.

**Rationale**: `verifyChecksumForRelease` already treats only a hash mismatch
as fatal. The page needs the missing-file case to be a choice. Policy flags
avoid a second installer.

**Alternatives considered**: A web-only downloader. It would fork checksum,
monorepo tag, and asset-fallback behavior that already has tests.

## Decision: Confirmation is a plan, then an operation

**Decision**: `POST /api/plugins/{name}/plan` resolves the target and returns
the summary (plugin, version, repository, security level, fallback,
checksum-file gap). It does not download. `POST` install/update and `DELETE`
remove require the summary fields echoed back. The server resolves again and
rejects a mismatch. The browser dialog is not the only gate.

**Rationale**: Specification FR-002 says a confirmation that does not match
the resolved target is rejected. A UI-only dialog would not be testable from
the handler.

## Decision: One operation per plugin, streamed as SSE

**Decision**: The mutating call returns `{ "operationId": "…" }` immediately
and runs the installer on the session context. `GET /api/plugins/operations/{id}/stream`
uses the existing `text/event-stream` writer in `sse.go`. A second mutating
call for that plugin while the lock is held returns 409 with a busy message.
The in-process lock in `lock.go` already produces that error. The HTTP layer
maps it.

**Rationale**: Install can take longer than a request the browser will wait
on. SSE is already how overview progress reaches the page.

## Decision: webui stays behind session callbacks

**Decision**: New `SessionOptions` callbacks are implemented in
`internal/cli/web.go`. `internal/webui` does not import `internal/cli`,
`internal/registry`, or `internal/pluginhost`. Catalog, plan, operate,
validate, and the settings view are callbacks. Settings writes reuse
`ConfigPreview` and `ConfigSave`.

**Rationale**: That is the boundary the config page already uses.

## Decision: Descriptor form, with a generic fallback

**Decision**: When `GetPluginInfo` returns one or more valid settings, the
form is those settings (`string`, `int`, `bool`, `enum`, `url`, `duration`).
When it returns none, or the plugin is not installed, the form is the keys
already stored under `plugins.<name>` plus rows the person adds. Keys for
which `config.IsCredentialName` is true are write-only. The page does not
invent a float type. `JEV_DUPLICATE_THRESHOLD` and the Kubernetes rates are
strings because the spec proto has no float; the plugin keeps parsing them.

**Rationale**: finfocus-spec v0.7.6 already shipped `WithSettings` and
`PluginSetting`. Core must not hard-code jev, Prometheus, or Kubernetes
names. Those plugins call `WithSettings` themselves.

**Alternatives considered**: The manifest `configuration` section. No plugin
ships a manifest, and `registry.Manifest` ignores that section. Launching
for `GetPluginInfo` is the same path `plugin list` already uses.

## Decision: Built-in plugins advertise what they already read

**Decision**: `plugins/jev`, `plugins/prometheus`, `plugins/kubernetes`, and
`plugins/recorder` pass `pluginsdk.WithSettings` from the env constants they
already have. Recorder is included because the browser test sets
`FINFOCUS_RECORDER_MOCK_RESPONSE`, which the generic `FINFOCUS_PLUGIN_*`
convention would not deliver.

**Rationale**: The issue names the three product plugins. The testing
strategy names the recorder. Advertising is a plugin-local change and does
not teach core those names.

## Decision: Catalog list includes installed plugins outside the catalog

**Decision**: Merge `registry.GetAllPluginEntries` with directories in the
discovery plugin dir. An installed name with no catalog entry is marked
`inCatalog: false` and has no install or update action.

**Rationale**: The CLI can already install `owner/repo`. Hiding that plugin
would make remove and settings impossible for it. The page still refuses to
download it.

## Decision: Rate-limit text is the existing GitHub error

**Decision**: Surface `GitHub API rate limit exceeded. Set GITHUB_TOKEN for
higher limits` on the operation error. Do not read a `gh` credential. Correct
the comment in `github.go` that still mentions a `gh` fallback. Token values
never enter the error.

**Rationale**: `getGitHubToken` already reads only `GITHUB_TOKEN`.

## Decision: Out of scope stays out

Custom registry URLs, signature checks, `plugin init`, `plugin upgrade`,
conformance, certify, `npx`, routing edits, per-request gRPC credentials, and
MCP exposure are not implemented and have no stubs.
