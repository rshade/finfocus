# Implementation Plan: Web UI Log Viewer

**Branch**: `1925-web-logs-viewer` | **Date**: 2026-11-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1925-web-logs-viewer/spec.md`

## Summary

Add an opt-in Logs view to `finfocus --web` that shows core and per-plugin
zerolog output, colour coded and separable by source. Go captures lines
(a tee on core's logger, a tagged writer on each plugin's stderr), parses and
redacts them once with the existing credential rules, holds 5000 in a ring
buffer, and streams them over a dedicated SSE route. The browser renders with
a self-contained viewer kept in `packages/zerolog-viewer/` so it can be
published separately later. A snapshot command, `finfocus logs`, renders the
log file through the same Go entry builder, filter, and display strings, so the
terminal, the dashboard endpoint, and JSON output agree. The feature is off
unless `FINFOCUS_WEB_LOGS` or
`web.logs.enabled` turns it on. Research settled one risk that changes the
wiring: SDK plugins write straight to the shared log file when core passes
`FINFOCUS_LOG_FILE`, so core stops passing it while logs are enabled
([research.md](research.md) R4).

## Technical Context

**Language/Version**: Go 1.27.3 (`go.mod`); browser JavaScript as ES modules,
no build step; Node 26 (`mise.toml`) for tests only
**Primary Dependencies**: zerolog, standard `net/http` SSE, existing
`internal/config`, `internal/history`, `internal/logging`, `internal/pluginhost`.
No new Go or npm dependency
**Storage**: N/A. In-memory ring buffer only; nothing written to disk
**Testing**: testify (`require`/`assert`), table-driven, `-race`; `node:test`
for the viewer package and the page glue; Playwright render check; one
`test/e2e` web scenario
**Target Platform**: Linux, macOS, Windows (plugin names strip `.exe`; no
signal or `syscall` use added)
**Project Type**: Go CLI with an embedded web UI, plus one self-contained
JavaScript package folder
**Performance Goals**: Page stays responsive through a 10,000-line burst
(filter change within 1 s); hub append is O(1); a slow tab never delays others
**Constraints**: Buffer 5000 entries; page shows at most 2000 rows; line cap
64 KiB; subscriber channel 256; CSP `default-src 'self'`; `internal/webui` must
not import `cli`, `registry`, or `pluginhost`; `internal/weblog` must not
import `cli`, `webui`, `registry`, or `pluginhost`
**Scale/Scope**: One process, a handful of plugins, a few tabs; about 15 new
Go files, 6 JavaScript source files, 1 glue view

## Constitution Check

*GATE: passes before research and again after design.*

- [x] **Plugin-First Architecture**: Core reads only the generic `plugin_name`
  field and tags by launch path. No provider or plugin knowledge enters core.
- [x] **Test-Driven Development**: Tests are listed first in `tasks.md` phases;
  coverage target 80%, 95% for `internal/weblog` (it is on the redaction path).
  No TUI change, so golden files do not apply; the page is checked with a
  Playwright render.
- [x] **Cross-Platform Compatibility**: No platform-specific code. Windows `.exe`
  suffix handled in the plugin-name derivation and covered by a table test.
- [x] **Documentation Integrity**: `packages/zerolog-viewer/README.md` (the
  package has more than three exported symbols), godoc on every exported Go
  symbol in `weblog` and `zerologviewer`, updates to the web UI guide, reference,
  and architecture pages, and a CLAUDE.md note. Code samples in the README are
  exercised by a test.
- [x] **Protocol Stability**: No proto or SDK change. R4 uses an existing
  environment variable; the SDK is untouched.
- [x] **Implementation Completeness**: No stubs. Every task in `tasks.md` ships
  working behaviour with tests.
- [x] **Persistence Model**: No new store. The buffer is memory only, so
  nothing needs documenting in `CONTEXT.md` or the constitution.
- [x] **Web Parity**: The spec's Web UI Fit section is present (disposition: new
  view with a CLI counterpart, `finfocus logs`).
  `contracts/http-api.md` names the Go methods (`weblog.(*Hub).Status`,
  `weblog.(*Hub).Subscribe`, and the shared `weblog.NewEntry`, `Decode`,
  `Filter.Match`, `Display` that `finfocus logs` also calls). The handler adds
  no logic beyond transport. A
  UX-only `web-ui` follow-up issue is filed at completion.
- [x] **Quality Gates**: `make lint`, `make test`, `make test-race`,
  `make docs-lint`, `go vet ./...`, `govulncheck`, markdownlint on the new
  Markdown.
- [x] **Multi-Repo Coordination**: None. `finfocus-spec` and plugins are not
  changed.

**Post-design re-check**: unchanged. The only item to watch is R4's removal of
`FINFOCUS_LOG_FILE` from the plugin environment while logs are on; it is gated
and tested for an identical file result, so it is a behaviour of the opt-in
feature, not a protocol change.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/1925-web-logs-viewer/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── http-api.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit.tasks
```

### Source Code (repository root)

```text
internal/
├── weblog/                       # NEW: capture, parse, redact, buffer, fan-out
│   ├── doc.go
│   ├── entry.go                  # Entry, Source, Status, Snapshot
│   ├── redact.go                 # config.IsCredentialName + history.IsPulumiSecret
│   ├── assembler.go              # line assembler (chunks, cap, flush)
│   ├── build.go                  # NewEntry: one entry builder for Decode and the live writers
│   ├── decode.go                 # Decode(io.Reader): stream a log file into entries
│   ├── filter.go                 # Filter.Match (source, min level, text)
│   ├── display.go                # Display: time, level label, source strings
│   ├── testdata/                 # shared NDJSON + filter_cases.json (Go and JS tests)
│   ├── hub.go                    # Hub: ring buffer, sources, Subscribe, Status
│   ├── writer.go                 # Hub.Writer(name) io.WriteCloser; Hub.Tee()
│   ├── context.go                # ContextWithHub, HubFromContext
│   └── *_test.go
├── logging/zerolog.go            # EDIT: Config.Tee; MultiLevelWriter in newLoggerWithWriter
│                                 #       plugin log tap in context (name -> io.Writer)
├── pluginhost/process.go         # EDIT: tag from path, fan stderr to tap,
│                                 #       omit FINFOCUS_LOG_FILE when a tap is set
├── config/
│   ├── config.go                 # EDIT: Web.Logs.Enabled
│   ├── descriptor.go             # EDIT: KnownSettings web.logs.enabled
│   ├── view.go                   # EDIT: defaultValue case
│   └── weblogs.go                # NEW: ResolveWebLogs(getenv)
├── cli/
│   ├── logs.go                   # NEW: `finfocus logs` (snapshot of the log file)
│   ├── logs_test.go
│   ├── mcp.go / testdata/mcp/tools.golden   # EDIT: expose `logs` (non-blocking)
│   ├── output_validation_test.go # EDIT: add `logs` to outputFormatCases
│   ├── logging_setup.go          # EDIT: create Hub when --web and enabled; tee; tap
│   └── web.go                    # EDIT: pass weblog.HubFromContext(ctx) as Options.Logs
└── webui/
    ├── server.go                 # EDIT: Options.Logs; registerLogs; skip requestLog
    ├── handlers_logs.go          # NEW: status + stream handlers, LogSource interface
    ├── static.go                 # EDIT: mount viewer FS at /static/zerolog-viewer/
    └── static/
        ├── index.html            # EDIT: nav entry (hidden until status is 200)
        ├── styles.css            # EDIT: map --zlv-* to dashboard tokens
        └── views/logs.js         # NEW: thin glue (route, status probe, EventSource)

packages/zerolog-viewer/          # NEW: self-contained, publishable later
├── package.json                  # private: true, type: module, exports, no deps
├── README.md
├── LICENSE                       # Apache-2.0
├── embed.go                      # package zerologviewer; //go:embed src; var FS
├── src/
│   ├── index.js                  # createLogViewer, parseNDJSON, normalizeEntry
│   ├── ndjson.js
│   ├── normalize.js
│   ├── viewer.js
│   ├── element.js                # optional <zerolog-viewer>
│   └── viewer.css                # --zlv-* variables
└── test/*.test.mjs               # ndjson, normalize, viewer, boundary

test/e2e/web_browser_test.go      # EDIT: FINFOCUS_WEB_LOGS=1 via fixtureEnv
mise.toml                         # EDIT: [env] FINFOCUS_WEB_LOGS = "1"
Makefile                          # EDIT: test-zerolog-viewer; make test runs it
docs/src/content/docs/{architecture,guides,reference}/web-ui.md   # EDIT
docs/src/content/docs/commands/logs.md                            # NEW
CLAUDE.md                         # EDIT: short note (config, plugin env rule)
```

**Structure Decision**: Capture and redaction live in a new leaf package
`internal/weblog` that both `cli` (writes) and `webui` (reads) may import, which
keeps `webui` free of `cli`, `registry`, and `pluginhost`. The viewer is a
separate folder with its own Go embed file because `go:embed` cannot cross
package directories. `pluginhost` gains only a generic "tap" seam in
`internal/logging`, so it never learns about the web UI.

## Design Notes

- **Wiring order**: `setupLogging` (root `PersistentPreRunE`) resolves
  enablement, and only for the `--web` command builds the Hub, sets
  `logging.Config.Tee = hub.Tee()`, and stores the hub and a plugin tap
  (`func(name string) io.WriteCloser`) in the context. `pluginhost.startPlugin`
  reads the tap, derives the plugin name from the path, and replaces
  `cmd.Stderr` with `io.MultiWriter(existing, tap(name))`. It also omits
  `FINFOCUS_LOG_FILE` from `cmd.Env` while a tap is present (R4). Plugin
  `cmd.Wait` is followed by `tap.Close()` to flush a last partial line.
- **`finfocus logs`** (spec User Story 6): `--file`, `--tail` (200), `--source`
  (repeatable), `--level`, `--grep`, and `--output table|json|ndjson` resolved
  through `resolveOutputFormat`. It never redeclares `--format`. The default
  file is the resolved `logging.file`; a missing default exits 1 with the path
  it tried, a missing `--file` or bad flag value goes through
  `toValidationError` (exit 2). A positive `--tail` uses `weblog.TailDecode`,
  which seeks backward and then calls `weblog.Decode` on that suffix.
  `--tail 0` streams the whole file with `weblog.Decode`. Both apply
  `weblog.Filter` and render the table from `weblog.Display`
  with the badge colours in `internal/tui` styles (colour off under
  `NO_COLOR`, resolved with `tui.DetectResolvedOutputMode`). It is a snapshot,
  so it is exposed as an MCP tool and listed in `tools.golden`; no
  `mcpExcludedCommands` entry. `--follow` is deliberately absent.
- **Bad `FINFOCUS_WEB_LOGS` value**: `ResolveWebLogs` returns an error and
  `setupLogging`'s caller returns it through `toValidationError` (exit 2).
  The error names the variable and the accepted values.
- **Self-noise**: `requestLog` skips `/api/logs/` (R9).
- **Level**: the tee sees what the logger emits; lowering the level is the
  user's `--debug` (R5). `mise.toml` sets only `FINFOCUS_WEB_LOGS`.
- **Frontend glue**: `logs.js` probes `GET /api/logs/status` and adds the nav
  entry only when `enabled` is true. A disabled server answers 200 with
  `enabled` false, so the probe is not a failed request. When the route is
  opened and logs are on, it calls
  `createLogViewer(container, {...})`, opens an `EventSource` on
  `/api/logs/stream`, and forwards `snapshot`/`log`/`gap` events to
  `viewer.push`. Reconnect reuses the capped backoff pattern from
  `OverviewStream`. The glue owns no rendering.

## Complexity Tracking

No constitution violations. Two deliberate additions worth recording:

| Addition | Why Needed | Simpler Alternative Rejected Because |
| --- | --- | --- |
| New package `internal/weblog` | `webui` cannot import `cli`/`pluginhost`, and both sides need the buffer | Putting the buffer in `webui` would force `pluginhost` or `cli` to import it back |
| Separate Go embed file in the viewer folder | Keeps the folder self-contained and publishable; `go:embed` cannot reach outside its directory | Copying files into `internal/webui/static` creates two sources of truth |
