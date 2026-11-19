# Contract: How the Extension Runs finfocus

## Executable

`finfocus.path` setting, else `finfocus` on `PATH` of the extension host
(which runs where the workspace lives: `extensionKind: ["workspace"]`).

## Version check (once per session)

`finfocus --version` → semver compared with `minFinfocusVersion` in the
extension manifest. Older: message naming the required version, no runs.
`dev` or unparsable: allowed, warning in the output channel.

## Project and stack discovery

Find `Pulumi.yaml` and `Pulumi.yml` under all workspace folders; exclude
`node_modules`, `.git`, and build/vendor directories. Do not follow directory
symlinks outside workspace folders. Choose the nearest ancestor project of
the active file (nested projects win). With no containing active file, retain
the explicit selection, select the sole project, or show a project picker;
never guess between projects. Rescan when project files or workspace folders
change. Non-Pulumi workspaces remain silent.

After trust is granted, use `pulumi stack ls --json` in the chosen project to
list stack references and their `current` flags. Discovery is the only direct
Pulumi invocation; it supplies identities, never costs. Pin its response shape
with a recorded fixture and verify it with the installed Pulumi version in
integration tests. Missing Pulumi, failed discovery, no stacks, or malformed
JSON produce actionable guidance without starting overview. No stack creation,
login, or selection command is run automatically.

Discovery also reads `lastUpdate`, `resourceCount`, and `updateInProgress` for
the selected stack. They are the cache's change signal (plan.md, Result reuse
and completeness); a stack with no `lastUpdate`, or an update in progress, is
never served from or written to the cache. Pin them in the discovery fixture.

Default to the entry marked `current`; otherwise offer a stack picker. Retain
an explicit choice in session memory for that project and pass `--stack S` on
every overview call; never invoke `pulumi stack select`. Key all state by
canonical project URI (including remote authority) plus the discovered stack
reference, not by a short name alone. Invalidate results, pending prompts,
and credentials on project removal or backend/configuration change. Refresh
reruns discovery so external stack-selection changes are visible; an explicit
extension choice remains until changed or no longer available.

Every discovery/version/overview process uses argument arrays without a shell,
runs in the workspace host, and obeys trust and cancellation rules. Watch the
stack settings candidates used by core (`pulumi.StackSettingsNameCandidates`,
`.yaml` and `.yml`); do not interpolate qualified stack names into arbitrary
paths. Selection changes increment an epoch, invalidate the old request,
cancel it, clear visible old figures, then load the new context. Late output
or prompts from the previous epoch are discarded.

## Runs

Working directory: the selected Pulumi project root, resolved as above.

| Trigger | Arguments |
| --- | --- |
| Activation, Refresh | `overview --output json --format json --yes --state-only --stack S` |
| Run preview, on save | `overview --output json --format json --yes --stack S` |

Environment: inherited, plus `PULUMI_CONFIG_PASSPHRASE=<value>` only when the
user entered one this session for this project/stack context (an empty value is passed as empty).

## Outcomes

| Exit | stdout | Extension action |
| --- | --- | --- |
| 0 | overview JSON | publish only for the current request/selection; clear stale only under the data-model generation rules |
| non-zero | ax-go envelope on stderr, `message` ends `(passphrase_required)` | ask for this context; retry only if still current; dismissal cancels; wrong values allow another prompt, never an automatic retry loop |
| non-zero | other envelope on stderr | keep previous result and stale marks, show `message` |
| non-zero | stderr holds no envelope | keep previous result, show "finfocus failed (exit N); see output channel"; stderr goes to the output channel only |

## Dirty documents and freshness

Before manual preview, offer Save all and run, Run saved files, or Cancel for
dirty documents under the selected project. Failed saves abort. No automatic
save occurs for on-save preview. Capture request ID, selection epoch, file
edit generations/dirty flags, and stack-config generation after requested
saves finish. Pulumi reads saved content only. Apply the freshness rules in
[data-model.md](../data-model.md), including preservation of edits during a
run and unknown freshness when deployed timestamps are missing.

Core preview adds `--show-sames` to `pulumi preview --json` so unchanged
resources carry current locations. Missing current locations do not fall
back to old deployed annotation positions.

## Cancellation and disposal

One overview process per context. Superseding a request first invalidates its
ID, then cancels it. All owned subprocesses (discovery, version checks,
finfocus, Pulumi, plugins, and descendants) must be contained for cleanup.

- Unix: start finfocus as a session leader (`spawn` with `detached: true`).
  Plugins move to their own process groups but stay in that session, so the
  session is the owned unit. Send SIGTERM to finfocus, allow 5 seconds for
  core context cancellation and plugin closure, then SIGKILL every process in
  the session. Read membership from `/proc/<pid>/stat` on Linux; use
  `/usr/bin/pkill -KILL -s <sid>` and `/usr/bin/pgrep -s <sid>` on macOS (see
  research R4).
- Windows: core holds a kill-on-close Job Object that every plugin and the
  Pulumi child join, so they die when finfocus exits for any reason. Core has
  no Job Object today (`process_group_windows.go` is a no-op), so core adds
  one; documentation alone is not an exemption. The extension terminates its
  owned CLI tree with `taskkill /PID <owned pid> /T /F` (argument array, never
  an image-name kill).
- Verify cleanup after both graceful cancellation and forced termination.
  Confirmation also runs after a graceful exit, so a descendant left behind
  by a clean exit is swept.
- Allow at most 2 additional seconds to confirm termination after escalation.
  Do not start a replacement until owned process cleanup is confirmed. On
  failure, show a cleanup error and block replacement for that context.

The same bounded shutdown runs on selection changes, workspace removal, and
extension deactivation; `deactivate()` returns the cleanup promise. Dispose
watchers, timers, output subscriptions, and prompts; clear credentials on
removal/deactivation. A passphrase answer after invalidation is discarded.
The non-interactive core overview uses `cancelOnShutdownSignal` on Unix and
closes plugins and Pulumi children before exiting. OS integration tests must
prove no descendants survive; cancellation is not accepted based only on a
mocked `kill()` call.

## Never

- No run in an untrusted workspace.
- No passphrase, environment value, or full command line in any message or
  log the extension writes.

## Cache and retry

Only the activation and Refresh row above is cached. Activation reads a valid
entry instead of starting finfocus; the explicit Refresh command skips it.
When a state-only run returns unpriced rows and there is no confirmed
baseline for the context (or the rows differ from it), the scheduler starts
the same command once more and applies the stability rule. Cache files live
under the extension's global storage on the extension host, are `0600`, and never contain the
passphrase or environment. The TTL is `finfocus.cache.ttlMinutes` (default 60,
`0` disables).
