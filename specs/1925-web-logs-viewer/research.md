# Research: Web UI Log Viewer

Every unknown in the plan's Technical Context is resolved here. Findings that
changed the design are marked **Finding**.

## R1. Is there a zerolog renderer for JavaScript?

- **Decision**: Build a small purpose-made viewer. Do not adopt a package.
- **Rationale**: Searches found no zerolog-aware browser renderer. The npm
  package named `zerolog` is an unrelated ZeroMQ logger. The nearest tools are
  standalone NDJSON apps: [allproxy/json-log-viewer](https://github.com/allproxy/json-log-viewer),
  [leno](https://www.npmjs.com/package/leno) (pino), and
  [web-log-viewer](https://www.npmjs.com/package/web-log-viewer). None embeds
  as a component, none knows zerolog field names, and none separates plugins.
  The dashboard also forbids a build step and a CDN
  (`docs/src/content/docs/architecture/web-ui.md`), and its CSP is
  `default-src 'self'`.
- **Alternatives considered**: Embedding a viewer app in an iframe (breaks the
  CSP and the shared layout); vendoring one of the apps (their field mapping,
  layout, and licence would need forking anyway).

## R2. Where are log lines parsed and redacted?

- **Decision**: In Go, before a line is stored. The browser renders entries
  and never sees raw lines from finfocus.
- **Rationale**: The redaction rules live in Go: `config.IsCredentialName`
  (`internal/config/credential.go`) for names and `history.IsPulumiSecret`
  (`internal/history/cost_plan.go`) for values. CLAUDE.md forbids a second
  rule list. Redacting in JavaScript would need a copy of both. The web UI
  rule that the browser computes nothing points the same way.
- **Alternatives considered**: Stream raw lines and parse in the browser
  (simpler Go, but secrets would reach the page unless the rules were
  duplicated).

## R3. JavaScript parsing practice

- **Decision**: Native `JSON.parse`, one call per complete line, inside
  `try/catch`. Never `eval`, never `new Function`, never `innerHTML`; text goes
  through `textContent`. Keep the unterminated tail of a chunk until the next
  chunk. A line that fails to parse becomes a `raw` entry.
- **Rationale**: This is the standard NDJSON practice and needs no
  dependency. Streaming JSON parsers solve documents larger than memory, which
  log lines are not. The package's `pushChunk` exists so standalone users get
  the same behaviour; finfocus itself sends entries.
- **Alternatives considered**: A streaming parser library (a dependency for no
  gain); regex extraction of level and message (breaks on escaped quotes).

## R4. Plugins can bypass stderr (**Finding**)

- **Decision**: When web logs are enabled, core does not pass `FINFOCUS_LOG_FILE`
  to plugins. Their output then goes to stderr, and core fans each plugin's
  stderr into the existing destination and into the log hub.
- **Rationale**: `pluginhost.startPlugin` passes `FINFOCUS_LOG_FILE` to plugins
  whenever core logs to a file (`internal/pluginhost/process.go`, around the
  `logPath` block). The SDK then opens that file itself and writes there
  (`pluginsdk.NewLogWriter` in `finfocus-spec/sdk/go/pluginsdk/logging.go`),
  so a stderr tap would see nothing from SDK plugins. With the variable
  omitted the SDK writes JSON to stderr. Core already copies plugin stderr into
  the same file handle (`session.pluginLogWriter`, opened `O_APPEND`), so the
  file receives the same lines, from the same plugins, in the same place.
  Spec FR-013 holds: same destination, same content.
- **Condition**: Only when logs are enabled. With logs off, plugin environment
  and logging are byte-for-byte unchanged.
- **Alternatives considered**: Tailing the log file (needs polling or
  inotify, differs on Windows, mixes in other finfocus processes, and races
  partial lines); changing the SDK (cross-repo change for a debugging
  feature, against the constitution's protocol rules).

## R5. Capturing core's own logs

- **Decision**: Add an optional tee writer to `logging.Config` and apply it in
  `newLoggerWithWriter` with `zerolog.MultiLevelWriter`, next to the existing
  output.
- **Rationale**: The console format wraps the destination in
  `zerolog.ConsoleWriter` (`internal/logging/zerolog.go`, `newLoggerWithWriter`).
  A tee placed after that would receive coloured text. `MultiLevelWriter`
  hands the tee zerolog's raw JSON event, one Write per event, so no line
  buffering is needed for core.
- **Limit**: The tee sees only events the logger emits, so the configured
  level applies (`--debug` or `FINFOCUS_LOG_LEVEL=debug` for debug lines). A
  separate lower level for the view is out of scope.
- **Alternatives considered**: A `zerolog.Hook` (receives the event, not the
  encoded line, so field order and types differ); a second logger (two sources
  of truth).

## R6. Naming the plugin that wrote a line

- **Decision**: A plugin line's source is `plugin_name` when the line has it,
  else the tag from the per-plugin writer, else `component`, else `core`.
  `pluginhost` derives the tag from the plugin path:
  `<home>/plugins/<name>/<version>/finfocus-plugin-<name>[.exe]` gives
  `<name>`; otherwise the binary name without the `finfocus-plugin-` prefix
  and `.exe`.
- **Rationale**: `startPlugin` receives only a path. The SDK stamps
  `plugin_name` (`pluginsdk.FieldPluginName`) but a plugin that does not use the
  SDK logger stays anonymous. The tag covers that case.
- **Known limit**: A router plugin's per-region child
  (`finfocus-plugin-aws-public-<region>`) writes through its parent's stderr,
  so its lines appear under the parent unless they carry their own
  `plugin_name`.
- **Alternatives considered**: Passing the name from `registry` (a new
  parameter through several launch paths for the same result).

## R7. Transport

- **Decision**: A dedicated per-connection SSE stream, `GET /api/logs/stream`,
  with `id:` set to the entry sequence number, `Last-Event-ID` resume, and a
  bounded per-subscriber channel that is closed when the subscriber lags.
- **Rationale**: CLAUDE.md says not to publish install-style streams on the
  overview bus: that bus runs `RedactJSON` and drops slow subscribers
  (`internal/webui/sse.go`). The log stream already redacts once, in `weblog`,
  and must survive a slow tab without affecting others. On reconnect the
  browser's `EventSource` sends `Last-Event-ID`, so the server resumes and
  emits a `gap` event only if entries were evicted in between.
- **Alternatives considered**: WebSocket (needs a new dependency and origin
  handling; one-way data suits SSE); polling `GET /api/logs?after=seq`
  (latency and request-log noise).

## R8. Enablement and the `web` config section

- **Decision**: Resolve in `internal/config`: `FINFOCUS_WEB_LOGS` (`1`, `true`
  on; `0`, `false` off; anything else is a validation error, exit 2), then
  `web.logs.enabled` on the merged config, then false. Add a `web` section with
  `logs.enabled`, a `KnownSettings` entry of kind bool, and a `defaultValue`
  case of false.
- **Rationale**: A known setting also appears on the Config page, and
  CLAUDE.md warns the project tier is lost if config is read while commands
  are built. Resolution therefore happens at run time in `setupLogging`, which
  already reads `config.GetLoggingConfig()` for the same reason. The project
  `config.hujson` overrides global at the top-level key, so `web` is a new key
  that does not disturb any existing section.
- **To verify in implementation**: that `setupLogging`'s config accessor sees
  the project overlay for `web.logs.enabled`; a test with a project config
  covers it.
- **Alternatives considered**: Detecting a dev build (rejected by the
  maintainer: env var and config key only).

## R9. Self-noise

- **Decision**: Skip `requestLog` for paths under `/api/logs/`.
- **Rationale**: `requestLog` writes an info line per request
  (`internal/webui/server.go`). Each open or closed log stream would add a line
  to the stream it serves. The routes are read-only and carry no cost data.

## R10. Viewer package placement and serving

- **Decision**: `packages/zerolog-viewer/` holds `src/`, tests, README, LICENSE,
  and a small `embed.go` (`package zerologviewer`, `//go:embed src`, exported
  `FS`). `internal/webui/static.go` mounts `fs.Sub(FS, "src")` at
  `/static/zerolog-viewer/`. The page imports
  `/static/zerolog-viewer/index.js`.
- **Rationale**: A `go:embed` pattern cannot reach outside its package
  directory, so the Go file must live in the folder. The existing static
  handler is `http.FileServerFS`, which sets JavaScript and CSS content types
  from the extension, and the CSP allows same-origin modules. No copy step,
  one source.
- **Not a release unit**: `private: true`; not added to
  `release-please-config.json` (CLAUDE.md reserves that for nested plugins).
  Publishing is a later decision; the README carries the checklist.
- **Alternatives considered**: Keeping the folder pure JS and copying into
  `internal/webui/static` at build time (two copies and a drift risk).

## R11. Limits

| Limit | Value | Reason |
| --- | --- | --- |
| Ring buffer | 5000 entries | Spans a full overview load with debug on |
| Row cap in the page | 2000 | Keeps the DOM small and filtering instant |
| Per-line cap | 64 KiB | One runaway line cannot hold memory or freeze the page |
| Subscriber channel | 256 | Same pattern as `Session.subscribe` |

A line over the cap is stored with `truncated: true` and its message cut. The
page offers the full stored text for the entry only up to the cap.

## R12. Redaction detail

- **Decision**: Keep the key, replace the value with `"[redacted]"`. Apply
  recursively to nested objects and arrays. Drop `__`-prefixed keys. Redact a
  value when its key is credential-like or when the value is a Pulumi secret
  envelope.
- **Rationale**: Showing that a field existed helps debugging; the existing
  response redaction drops hidden keys, but a log reader needs to know the
  plugin did log it. Free-text inside `message` cannot be inspected reliably,
  so it is documented as a limit: plugins are already told never to log
  credentials.

## R13. Tests

- Go: table-driven, testify only, `t.Parallel()` except where process-wide
  state is touched (`t.Setenv`, `config.ResetGlobalConfigForTest`). Race run
  with `-race -shuffle=on -count=3` for `internal/weblog`.
- Plugin launch: a fixture script that prints a JSON line and a plain line to
  stderr, run through `ProcessLauncher` with a tap; `//nolint:paralleltest` and
  no parallelism because it writes then execs a script (text file busy,
  golang/go#22315).
- JavaScript: `node:test`, run under `ulimit -v` and `timeout`; assert text or
  identity, never `assert.equal` on DOM-double nodes (OOM, see memory note on
  frontend test runs).
- End to end: `test/e2e` web fixture passes `FINFOCUS_WEB_LOGS=1` through
  `fixtureEnv` because `webEnvironment` strips every `FINFOCUS_*` value.

## R14. Constitution fit (summary)

No new persistent store (the buffer is memory only, so the Persistence Model
section does not apply and no command depends on it). No protocol change.
Core stays plugin-agnostic: it tags by path and reads only `plugin_name`.
Details in `plan.md`.
