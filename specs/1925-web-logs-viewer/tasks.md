---
description: "Task list for the Web UI Log Viewer (1925-web-logs-viewer)"
---

# Tasks: Web UI Log Viewer

**Input**: Design documents from `specs/1925-web-logs-viewer/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http-api.md, quickstart.md

**Tests**: Per Constitution Principle II, tests are written before the code
they cover and must fail first. Coverage: 80% overall, 95% for
`internal/weblog` (it is on the redaction path).

**Completeness**: Per Principle VI, no stubs, placeholders, or TODO comments.

**Documentation**: Per Principle IV, README and `docs/` change in the same PR.

**Organization**: Grouped by user story. US1 to US3 are all P1 and ship
together; US4 to US6 add on.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an
  unfinished task)
- **[Story]**: US1 to US6 map to the spec's user stories
- Go unit tests are colocated (`internal/<pkg>/<name>_test.go`); JavaScript
  tests for the viewer live in `packages/zerolog-viewer/test/`; page glue tests
  live in `internal/webui/frontendtest/`
- JavaScript tests run under `ulimit -v` and `timeout`, and assert text or
  identity, never `assert.equal` on DOM-double nodes (they exhaust memory)
- Tests that call `t.Setenv`, `t.Chdir`, build a root command, or write then
  exec a script are not parallel (see CLAUDE.md "Parallel Tests")

---

## Phase 1: Setup

**Purpose**: Skeletons and shared fixtures that later tasks read.

- [x] T001 [P] Create `packages/zerolog-viewer/package.json` (`name` `@rshade/zerolog-viewer`, `version` `0.0.0`, `private: true`, `type: module`, `exports` for `./src/index.js` and `./src/element.js` and `./src/viewer.css`, `files`, no dependencies, `license` Apache-2.0)
- [x] T002 [P] Create `packages/zerolog-viewer/LICENSE` with the repo's Apache-2.0 text copied from `LICENSE`
- [x] T003 [P] Create `internal/weblog/doc.go` with the package comment: purpose, the rule that it must not import `cli`, `webui`, `registry`, or `pluginhost`, and that all state is in memory
- [x] T004 [P] Create `internal/weblog/testdata/sample.ndjson`: core lines with `component`, SDK plugin lines with `plugin_name`, a plain-text line, a line with an `api_token` field, a line with a Pulumi secret envelope value, a `__defaults` key, a line with an unknown level, and a line whose message is over 64 KiB
- [x] T005 [P] Create `internal/weblog/testdata/filter_cases.json`: array of `{name, entries, filter, expectSeq}` covering source sets, every level threshold, text on message, text on a field value, case-insensitivity, and combined filters; read by both the Go and the JavaScript tests

---

## Phase 2: Foundational (blocks every story)

**Purpose**: The shared entry pipeline, the hub, the settings, and the logging
seams. Nothing in later phases works without these.

### Tests first (must fail before implementation)

- [x] T006 [P] `internal/weblog/build_test.go`: `NewEntry` normalises level (unknown and missing become `info`), parses `time` or falls back to receive time, resolves source in the order `plugin_name`, writer tag, `component`, `core`, keeps non-JSON as a `raw` entry, cuts messages over 64 KiB with `truncated`, and builds `display`
- [x] T007 [P] `internal/weblog/redact_test.go`: credential-named keys keep the key with `"[redacted]"`, nested objects and arrays are walked, Pulumi secret envelopes are replaced, `__` keys are removed, and the result matches `config.IsCredentialName` and `history.IsPulumiSecret` (no second rule list)
- [x] T008 [P] `internal/weblog/assembler_test.go`: line split across writes, several lines in one write, `\r\n`, empty lines, tail over 64 KiB flushed as truncated raw, `Close` flushes the tail once
- [x] T009 [P] `internal/weblog/display_test.go`: local time `15:04:05.000`, fixed-width upper-case level label, source string
- [x] T010 [P] `internal/weblog/filter_test.go`: runs every case in `testdata/filter_cases.json` against `Filter.Match`
- [x] T011 [P] `internal/weblog/decode_test.go`: `Decode` over `testdata/sample.ndjson` yields entries in file order, raw lines get source `unknown`, and a lazily generated 200,000-line reader completes while a tail ring of 200 keeps only the last 200 (memory bounded by N); add `BenchmarkDecodeTail` and record MB/s in the test comment for SC-010
- [x] T012 [P] `internal/weblog/hub_test.go`: sequence numbers start at 1 and increase, ring eviction at capacity with `dropped`, `Subscribe(afterSeq)` returns retained entries after that seq, a lagging subscriber's channel is closed without blocking `Append`, sources are registered once, `Status` fields; run with `-race -shuffle=on -count=3`
- [x] T013 [P] `internal/weblog/writer_test.go` and `context_test.go`: `Hub.Writer(name)` tags raw lines and JSON lines without `plugin_name`, `Hub.Tee()` receives one JSON event per `Write`, `ContextWithHub` and `HubFromContext` round-trip, a nil hub is safe
- [x] T014 [P] `internal/config/weblogs_test.go`: `ResolveWebLogs` table over env `1`, `true`, `0`, `false`, empty, `maybe` (error naming the variable and accepted values) and config true, false, absent; env wins over config, config over default false; no parallelism (`t.Setenv`)
- [x] T015 [P] `internal/logging/zerolog_tee_test.go`: with `Format: "console"` the destination receives console text while `Config.Tee` receives raw JSON events; level filtering applies to the tee; `ContextWithPluginLogTap` and `PluginLogTapFromContext` round-trip

### Implementation

- [x] T016 Create `internal/weblog/entry.go` and `internal/weblog/build.go`: `Entry`, `Source`, `Display`, `Status`, `Snapshot`, and `NewEntry(line []byte, tag string, now time.Time) Entry` per data-model.md, with godoc on every exported symbol
- [x] T017 [P] Create `internal/weblog/redact.go`: recursive redaction using `config.IsCredentialName` and `history.IsPulumiSecret`, called from `NewEntry`
- [x] T018 [P] Create `internal/weblog/assembler.go`: the line assembler (64 KiB cap, `Close` flush) used by writers and `Decode`
- [x] T019 [P] Create `internal/weblog/display.go` and `internal/weblog/filter.go`: `Display(Entry)` and `Filter{Sources, MinLevel, Text}.Match` with the level order from data-model.md
- [x] T020 Create `internal/weblog/decode.go`: `Decode(r io.Reader, tag string) iter.Seq2[Entry, error]` streaming through the assembler and `NewEntry`
- [x] T021 Create `internal/weblog/hub.go`: `Hub` with a 5000-entry ring, sequence numbers, source registry, `Append`, `Subscribe(afterSeq)` (snapshot, live channel of 256, cancel), and `Status`
- [x] T022 Create `internal/weblog/writer.go` and `internal/weblog/context.go`: `(*Hub).Writer(name) io.WriteCloser`, `(*Hub).Tee() io.Writer`, `ContextWithHub`, `HubFromContext`
- [x] T023 Add the `web` section to `internal/config/config.go` (`Web.Logs.Enabled *bool`, yaml and json tags `web` / `logs` / `enabled`; add a `keyWeb` constant to the top-level key list near `config.go:455` so `web` is not also kept in `extraKeys` and written twice; add a `case keyWeb` to `Config.Set` near `config.go:605`; load and merge like other top-level sections) with a test that a config file holding `web` round-trips through load and save without duplicating the key and that `config validate` accepts it, and create `internal/config/weblogs.go` with `ResolveWebLogs(getenv func(string) string) (bool, error)`
- [x] T024 Edit `internal/logging/zerolog.go`: add `Config.Tee io.Writer`, apply it with `zerolog.MultiLevelWriter` in `newLoggerWithWriter` so the destination keeps its format, and add `ContextWithPluginLogTap` / `PluginLogTapFromContext` for a `func(name string) io.WriteCloser`

**Checkpoint**: `go test -race ./internal/weblog/... ./internal/config/... ./internal/logging/...` passes.

---

## Phase 3: User Story 1 - See what core and the plugins are doing (P1)

**Goal**: With logs enabled, a Logs view lists core and per-plugin lines live,
with earlier lines present on open.

**Independent Test**: `FINFOCUS_WEB_LOGS=1 finfocus --web` against a stack that
uses a plugin; the Logs view shows core and that plugin's lines, each with
time, level badge, source, and message.

### US1 tests first

- [x] T025 [P] [US1] `internal/pluginhost/plugin_name_test.go`: `pluginNameFromPath` for `<home>/plugins/<name>/<version>/finfocus-plugin-<name>`, the `.exe` form, a bare binary `finfocus-plugin-x`, and an odd name
- [x] T026 [P] [US1] `internal/pluginhost/process_logtap_test.go`: a fixture script that prints one JSON line and one plain line to stderr; with a tap set, both reach `tap(name)` and the existing destination, and `FINFOCUS_LOG_FILE` is absent from the plugin environment; with no tap the environment is unchanged; with a log file configured, run the same SDK-style fixture with and without the tap and assert the plugin lines in the file are identical (SC-004, FR-013). Not parallel (writes then execs a script)
- [x] T027 [P] [US1] `internal/cli/logging_setup_web_test.go`: for the `--web` command with logs enabled, `setupLogging` puts a hub in the context, core log events reach it as JSON, and the configured log file still receives the same lines; a non-web command gets no hub; a dashboard started by the overview's `w` handoff (`startWebSession`) also gets the hub; and plugin clients the dashboard reopens later (Plugins-page reload) launch with the tap in their context. Not parallel (`t.Setenv`, root command)
- [x] T028 [P] [US1] `internal/webui/handlers_logs_test.go`: `/api/logs/status` returns capacity and sources; `/api/logs/stream` sends `snapshot` then `log` with `id` set to `seq`; `Last-Event-ID` resumes; `gap` is sent when entries were evicted; non-GET is 405; a request without the session cookie is 401 and one with a foreign Host header is 403; a slow subscriber is closed without delaying another; `requestLog` writes nothing for `/api/logs/` paths
- [x] T029 [P] [US1] `packages/zerolog-viewer/test/ndjson.test.mjs` and `normalize.test.mjs`: chunk splitting (line across chunks, CRLF, blank lines, bad JSON kept as `raw`), `normalizeEntry` with the default and a custom `fieldMap`, `sourceFields` order
- [x] T030 [P] [US1] `packages/zerolog-viewer/test/viewer.test.mjs` and `internal/webui/frontendtest/logs.test.mjs`: rows render time, level badge text, source, and message; follow-tail scrolls; the 2000-row cap drops the oldest and says so, and 10,000 pushes leave at most 2000 row nodes (SC-005); the glue hides the nav entry unless `enabled` is true, adds it when status says logs are on, and forwards `snapshot`, `log`, and `gap` to `viewer.push`; reconnect backoff

### US1 implementation

- [x] T031 [US1] Edit `internal/pluginhost/process.go`: add `pluginNameFromPath`; in `startPlugin`, when `logging.PluginLogTapFromContext(ctx)` is set, fan `cmd.Stderr` into the tap for that name and omit `FINFOCUS_LOG_FILE` from `cmd.Env`; after `cmd.Wait` in the close path, close the tap writer to flush a last partial line
- [x] T032 [US1] Edit `internal/cli/logging_setup.go`: when a dashboard will run (the `--web` command, or the `overview` command whose TUI can hand off to `runWebSession` through `startWebSession` at `internal/cli/overview.go:954`), resolve `config.ResolveWebLogs`, and when enabled create the hub, set `Config.Tee = hub.Tee()`, and add `weblog.ContextWithHub` and the plugin tap (`hub.Writer`) to the context in `attachWithTrace`; return a bad `FINFOCUS_WEB_LOGS` value through `toValidationError` (exit 2)
- [x] T033 [US1] Edit `internal/webui/server.go` and create `internal/webui/handlers_logs.go`: `LogSource` interface, `Options.Logs`, `registerLogs` (status always, stream only when non-nil), the status and stream handlers following the contract and the overview stream's write deadline and keepalive, and skip `requestLog` for `/api/logs/`
- [x] T034 [US1] Edit `internal/cli/web.go`: pass `weblog.HubFromContext(ctx)` as `webui.Options.Logs` in `listenAndPrintWeb`
- [x] T035 [US1] Create `packages/zerolog-viewer/src/ndjson.js`, `normalize.js`, and `index.js`: `parseNDJSON` / `pushChunk` (per-line `JSON.parse` in try/catch, tail kept), `normalizeEntry` with an overridable `fieldMap`, exports `createLogViewer`, `parseNDJSON`, `normalizeEntry`
- [x] T036 [US1] Create `packages/zerolog-viewer/src/viewer.js` and `viewer.css`: `createLogViewer(container, options)` returning `push`, `pushLine`, `pushChunk`, `clear`, `setFilters`, `destroy`; rows built with `textContent` only; level colour plus text badge; source badge; expandable field list; 2000-row cap; `--zlv-*` variables with light and dark defaults; uses `entry.display` when present
- [x] T037 [US1] Create `packages/zerolog-viewer/embed.go` (`package zerologviewer`, `//go:embed src`, exported `FS`) and edit `internal/webui/static.go` to mount `fs.Sub(zerologviewer.FS, "src")` at `/static/zerolog-viewer/`; add a Go test that `GET /static/zerolog-viewer/index.js` returns JavaScript from the embed
- [x] T038 [US1] Create `internal/webui/static/views/logs.js` (route, status probe, `EventSource` to `/api/logs/stream`, capped backoff like `OverviewStream`, forwards events to the viewer), and edit `internal/webui/static/index.html` (hidden nav entry), `internal/webui/static/app.js` (register the route), and `internal/webui/static/styles.css` (map `--zlv-*` to dashboard tokens)
- [x] T039 [US1] Edit `Makefile`: add `test-zerolog-viewer` running `node --test packages/zerolog-viewer/test/*.test.mjs` (modelled on the frontend line near `internal/webui/frontendtest`), and run it from `make test`

**Checkpoint**: US1 works end to end with `FINFOCUS_WEB_LOGS=1`.

---

## Phase 4: User Story 2 - Narrow the noise (P1)

**Goal**: Filter by source, minimum level, and text; pause, clear, download.

**Independent Test**: With core and two plugins present, hide one plugin, raise
the level to warn, search a word, download, and confirm the file matches what
is visible.

- [x] T040 [P] [US2] `packages/zerolog-viewer/test/filters.test.mjs`: runs `internal/weblog/testdata/filter_cases.json` through the viewer's filter; source chips list sources from the snapshot before any line matches; pause keeps scroll and counts new lines; resume jumps to newest; clear empties and keeps only later lines; download builds NDJSON of the visible entries
- [x] T041 [US2] Create `packages/zerolog-viewer/src/filters.js` and wire it into `viewer.js`: source set, minimum level, case-insensitive text over message and field values, combined; `setFilters` API
- [x] T042 [US2] Edit `packages/zerolog-viewer/src/viewer.js` and `viewer.css`: controls row (source chips, level select, search, follow toggle, clear, download), status line ("N shown of M, K new while paused", dropped notice), and the NDJSON download through a `Blob`
- [x] T043 [US2] Edit `internal/webui/static/views/logs.js` to render the viewer's controls in the dashboard's controls-row and status-line slots, and extend `internal/webui/frontendtest/logs.test.mjs` for the slots

**Checkpoint**: US1 and US2 deliver the full view.

---

## Phase 5: User Story 3 - Off unless asked, safe when on (P1)

**Goal**: No Logs entry or stream unless enabled; no secret reaches the page.

**Independent Test**: Start without the setting: no entry, status reports logs off, stream 404. Start
with it and a plugin that logs a credential field: the value is `[redacted]`
everywhere.

- [x] T044 [P] [US3] `internal/webui/handlers_logs_disabled_test.go`: with `Options.Logs` nil, `GET /api/logs/status` returns 200 with `enabled` false and `/api/logs/stream` returns the standard 404 `not_found` body (#2013)
- [x] T045 [P] [US3] `internal/cli/web_logs_enable_test.go`: matrix of env (`1`, `0`, `maybe`, unset) and config (true, false, absent) giving the spec's results; `maybe` exits 2; a project-tier `web.logs.enabled` is honoured; every case sets or clears `FINFOCUS_WEB_LOGS` explicitly so a dev shell or CI with the `mise.toml` value cannot change the result. Not parallel
- [x] T046 [P] [US3] `internal/webui/handlers_logs_redaction_test.go`: a plugin fixture writing `api_token` and a Pulumi secret value; the stream body, the status body, and the entries a download would hold contain no secret and no `__` key
- [x] T047 [P] [US3] `packages/zerolog-viewer/test/xss.test.mjs`: an entry whose message and field values contain `<img src=x onerror=...>` and `</script>` is rendered as literal text and creates no element beyond the row
- [x] T048 [US3] If T045's project-tier case fails because `setupLogging` reads config through `config.GetLoggingConfig()` without the project overlay, edit `internal/cli/logging_setup.go` to read `web.logs.enabled` from the merged config (`config.InitializedGlobalConfig`) at run time, never while building commands; keep `TestNewRootCmdDoesNotInitializeConfig` green

**Checkpoint**: P1 stories complete; the feature is safe to review.

---

## Phase 6: User Story 4 - On for testing, reusable later (P2)

**Goal**: Dev shells and test fixtures enable logs; the viewer folder stands
alone.

**Independent Test**: Run the viewer's tests with no finfocus source on the
path; add an outside import and watch the boundary test fail.

- [x] T049 [P] [US4] `packages/zerolog-viewer/test/boundary.test.mjs`: scans every file in `src/` and fails on any import that leaves the folder, names `app.js`, or reads a finfocus global; includes a case proving a planted bad import is caught
- [x] T050 [P] [US4] `packages/zerolog-viewer/test/readme.test.mjs`: extracts the README's JavaScript fenced samples and runs them against the DOM double
- [x] T051 [P] [US4] `packages/zerolog-viewer/test/element.test.mjs`: `<zerolog-viewer>` forwards `push`, observes attributes for filters, and cleans up on disconnect
- [x] T052 [US4] Create `packages/zerolog-viewer/src/element.js` (optional custom element wrapper over `createLogViewer`)
- [x] T053 [US4] Create `packages/zerolog-viewer/README.md`: install-free usage, `pushChunk` and `push`, the entry schema, `fieldMap` and `sourceFields`, `--zlv-*` theming table, security notes (`textContent`, no `eval`), and the publish checklist (npm scope, flip `private`, release-please entry, `npm publish --provenance`)
- [x] T054 [US4] Edit `mise.toml` to add `[env] FINFOCUS_WEB_LOGS = "1"` with a comment that it is for development and tests only, and edit `test/e2e/web_browser_test.go` so the web fixture passes `FINFOCUS_WEB_LOGS=1` through `fixtureEnv` (`webEnvironment` strips every `FINFOCUS_*` value); add an e2e check that the Logs entry appears and shows a core line

---

## Phase 7: User Story 5 - Settable from the Config page (P3)

**Goal**: `web.logs.enabled` is a known boolean setting.

**Independent Test**: `finfocus config set web.logs.enabled true`, then
`config get` and the Config page agree; a non-boolean is rejected.

- [x] T055 [P] [US5] `internal/config/descriptor_weblogs_test.go` and a case in `internal/webui/handlers_config_test.go`: the setting is listed as a bool with help text naming resource names and paths, default false, `config set` accepts `true`/`false` and rejects other values as a validation error
- [x] T056 [US5] Edit `internal/config/descriptor.go` (add `{Key: "web.logs.enabled", Kind: KindBool, Help: ...}` to `KnownSettings`) and `internal/config/view.go` (`defaultValue` case returning `false`), and confirm `Config.Set` and `PatchFile` accept the key through the `keyWeb` case added in T023

---

## Phase 8: User Story 6 - Read the same logs from the terminal (P2)

**Goal**: `finfocus logs` prints the log file through the shared pipeline.

**Independent Test**: With core and plugin lines in the log file,
`finfocus logs --source <plugin> --level warn` prints only those lines with
badges, and `--output ndjson` matches the dashboard entries for the same
fixture.

### US6 tests first

- [x] T057 [P] [US6] `internal/cli/logs_test.go`: default file used; missing default file exits 1 with the path tried; missing `--file` exits 2 through `toValidationError`; `--tail` default 200 and `0` for all; `--source` repeatable, `--level`, `--grep` combine as `weblog.Filter`; `--output table|json|ndjson`; an unknown `--output` exits 2 before reading; credential values print as `[redacted]`; the file is byte-identical afterwards; no plugin is started. Not parallel (root command, `t.Setenv`)
- [x] T058 [P] [US6] `internal/cli/logs_parity_test.go`: for `internal/weblog/testdata/sample.ndjson`, `finfocus logs --output ndjson --tail 0` and entries streamed from a `Hub` fed the same lines are field-for-field identical (SC-009)
- [x] T059 [P] [US6] `internal/cli/logs_render_test.go` with golden files `internal/cli/testdata/logs_table.golden` and `logs_table_nocolor.golden`: render the table for the sample, read it, then lock it; colour off under `NO_COLOR` and `--color`, resolved with `tui.DetectResolvedOutputMode`; level readable from the text badge alone (SC-006). Write the assertions first; generate the goldens after T062 (`UPDATE_GOLDEN=1`), read them, then lock them
- [x] T060 [P] [US6] Add `logs` to `outputFormatCases` in `internal/cli/output_validation_test.go` and regenerate `internal/cli/testdata/mcp/tools.golden` (`UPDATE_GOLDEN=1`) once T062 registers the command; the command is non-blocking, so it is exposed and gets no `mcpExcludedCommands` entry

### US6 implementation

- [x] T061 [US6] Create `internal/cli/logs.go`: `newLogsCmd` with `--file`, `--tail`, `--source`, `--level`, `--grep`, `--output`, using `RunE`, `cmd.Printf`, `resolveOutputFormat`, `weblog.Decode` into a ring of N, `weblog.Filter`, and `weblog.Display`; never declares `--format`; errors per FR-023
- [x] T062 [US6] Register the command in the root command in `internal/cli/root.go` and style the badges and source label with the existing `internal/tui` styles
- [x] T063 [US6] Create `docs/src/content/docs/commands/logs.md`: usage, flags, output formats, exit codes, examples against the default log file, and the note that it is a snapshot

**Checkpoint**: terminal, JSON, and dashboard agree for the fixture.

---

## Phase 9: Web Parity (Constitution Principle VII)

**Purpose**: The hook, contract, render check, docs, and follow-up issue.

- [x] T064 `internal/webui/parity_logs_test.go` (modelled on `parity_overview_test.go`): the stream's `snapshot` entries for the sample fixture equal `weblog.Decode` output run through the same `Display`, and the handler imports nothing from `cli`, `registry`, or `pluginhost` (an import check like the existing boundary tests)
- [x] T065 [P] Review `specs/1925-web-logs-viewer/contracts/http-api.md` against the finished handlers; every Go method it names exists with that name and signature
- [x] T066 Render the Logs view with Playwright against `../finfocus-demo` (`FINFOCUS_WEB_LOGS=1 PULUMI_CONFIG_PASSPHRASE='' finfocus --web --no-browser --port <port>`), compare with the Plugins view for typography, spacing, controls row, and status line, take screenshots under `.playwright-mcp/`, and record stack and date in the spec's `Render evidence` line
- [x] T067 [P] Edit `docs/src/content/docs/reference/web-ui.md` (view, endpoints, setting, env var, default) and `docs/src/content/docs/guides/web-ui.md` (how to turn it on, filter, download, redaction, privacy note)
- [x] T068 Final task, after the work is merged-ready and with the maintainer's go-ahead: file one `web-ui` follow-up issue, UX only (layout, rendering, interaction, copy, accessibility); no engine, API, redaction, or data-shape work

---

## Phase 10: Polish & Cross-Cutting

- [x] T069 [P] Edit `docs/src/content/docs/architecture/web-ui.md`: record why logs are parsed and redacted in Go, why plugins stop receiving `FINFOCUS_LOG_FILE` while logs are on, why the viewer is a separate folder, and why `finfocus logs` and the endpoint share one pipeline
- [x] T070 [P] Edit `CLAUDE.md` (Key Patterns / Plugin Host / CLI): the `weblog` import boundary, the `FINFOCUS_LOG_FILE` rule while logs are enabled, `web.logs.enabled`, and that `pluginhost` tags by path; keep it short and point to this spec
- [ ] T071 Run coverage for `internal/weblog` and confirm at least 95%, and 80% for the other changed packages; add table cases for any uncovered branch (not met: `internal/cli` is 78.7%; accepted by the maintainer, new files 85-100%)
- [x] T072 Run `go test -race -shuffle=on -count=3 ./internal/weblog/... ./internal/pluginhost/... ./internal/webui/...` and `go vet ./...` (also after a rebase, per CLAUDE.md)
- [x] T073 Run `make lint` (extended timeout), `make test`, `make test-zerolog-viewer`, `make docs-lint`, `make validate`, and `markdownlint` on every new Markdown file; fix findings without editing `.golangci.yml`
- [x] T074 Run the quickstart end to end against `../finfocus-demo` (off by default, env on, config on, precedence, redaction, log file unchanged, `finfocus logs`), `pulumi destroy` afterwards if a stack was deployed, stop with an anchored `pkill -f '^<path>/finfocus --web'`, and check `ps` for orphaned `finfocus-plugin-*` children
- [x] T075 Run `govulncheck` and confirm no new Go or npm dependency was added (`git diff go.mod go.sum package.json`)
- [x] T076 Update `specs/1925-web-logs-viewer/spec.md` status to Implemented and tick `checklists/requirements.md`

---

## Dependencies & Execution Order

- **Phase 1** has no dependencies. **Phase 2** needs Phase 1 and blocks every
  story.
- **US1** needs Phase 2. **US2** needs US1 (it extends the viewer and glue).
  **US3** needs US1 (its tests exercise the finished wiring). Ship US1, US2,
  US3 together.
- **US4** needs US1 (the viewer exists) and can run beside US2 and US3.
- **US5** needs only Phase 2's config section and can run any time after it.
- **US6** needs Phase 2 only (`Decode`, `Filter`, `Display`) and can run beside
  US1; its parity test (T058) is easiest after T021 and T022.
- **Web Parity** needs US1, US2, and US6. **Polish** comes last.
- Within a phase: tests first and failing, then implementation. Models before
  services, services before handlers.

### Parallel opportunities

- Phase 1: T001 to T005 together.
- Phase 2 tests: T006 to T015 together (separate files); then T017, T018, T019
  together after T016.
- US1 tests: T025 to T030 together; implementation T031 (pluginhost), T033
  (webui), T035 and T036 (viewer) touch different packages and can run in
  parallel once their tests exist.
- US6 (T057 to T063) can be a second stream beside US1 and US2.

## Parallel Example: Phase 2 tests

```text
Task: "internal/weblog/build_test.go ..."      (T006)
Task: "internal/weblog/redact_test.go ..."     (T007)
Task: "internal/weblog/assembler_test.go ..."  (T008)
Task: "internal/config/weblogs_test.go ..."    (T014)
Task: "internal/logging/zerolog_tee_test.go ..." (T015)
```

## Implementation Strategy

### MVP (the P1 trio)

1. Phase 1 and Phase 2, with the checkpoint green.
2. US1, then US3's tests (they are quick and catch safety gaps early), then
   US2.
3. Stop and validate with the quickstart, steps 1 to 4. This is the smallest
   release that meets the request: off by default, on by env var or config,
   coloured, separable, redacted.

### Incremental delivery

1. Add US4 (test environment and the standalone-package proof).
2. Add US6 (`finfocus logs`) and US5 (Config page) in either order.
3. Web Parity and Polish close the work.

### Notes

- Commit only when the maintainer asks; branch is `1925-web-logs-viewer`.
- Do not edit `.golangci.yml` or `CHANGELOG.md` (release-please manages it).
- Every task above ships working behaviour; none is a stub.
