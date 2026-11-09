---
title: Web UI Design
description: Why the finfocus --web dashboard shares the CLI engine, how a session works, and how its localhost security model protects cost data.
---

`finfocus --web` is a third presentation layer next to the CLI output and the
terminal UI. This page explains the choices behind it: why the browser computes
nothing, how one session serves several tabs, and why the local server needs
more protection than "it only listens on localhost". For tasks, see
[Use the browser dashboard](../guides/web-ui.md); for exact behavior, see the
[Web UI reference](../reference/web-ui.md).

## One engine, three renderers

The feature's central rule is that every cost behavior has exactly one
implementation. Filtering, sorting, grouping, pagination, money formatting,
budget health, and estimate arithmetic live in Go, in `internal/engine` and
`internal/viewmodel`, and the terminal UI uses the same functions. The overview
load runs through the same seven-phase pipeline as `finfocus overview`.

The web layer (`internal/webui`) is transport only. Its handlers call the
shared code and return JSON that already contains display strings, and the
JavaScript renders those strings without doing arithmetic. Two consequences
follow:

- The browser, the terminal UI, and CLI JSON agree on every number, because no
  second implementation exists that could drift.
- A capability missing from the browser is added to the shared Go path, not to
  the page.

The page itself is hand-written ES modules embedded in the binary with
`go:embed`. There is no Node build step, so `make build` stays pure Go, and the
page loads nothing from a CDN.

## Sessions, streaming, and tabs

One `finfocus --web` process is one session. It loads the sources once, at
launch, with the flags you gave. Progress (load phases, rows as plugins answer,
budget health) is pushed to the browser over Server-Sent Events. SSE fits
because data flows one way, from server to page, and it needs nothing beyond
Go's standard library.

Every new event stream starts with a snapshot of everything loaded so far. That
is why a reload or a second tab picks up the current state instead of starting
the load again. Per-view choices, such as expanded clusters, stay in the page
that made them, so two tabs do not interfere with each other.

The dashboard refreshes only where the terminal UI does: an on-demand
`pulumi preview` and estimate recalculation. There is no timed refresh; you
relaunch for fresh actual costs.

## Security model

A server on `127.0.0.1` is unreachable from other machines, but it is still
reachable from other local processes and, through the browser, from any web
page you have open. A page on the internet can send requests to
`http://127.0.0.1:<port>`, and a page served from another localhost port counts
as the same site for cookies. The dashboard returns cost data and can trigger
plugin calls and `pulumi preview`, so it defends against each of those paths.

| Threat | Control | Why this control |
| --- | --- | --- |
| Access from another machine | Binds `127.0.0.1` only, with no flag to change it | A local, single-user tool has no reason to accept remote connections. Unlike `finfocus mcp-server --transport=http`, there is no opt-in. |
| Another local process or user guessing the port | 128-bit random token in the printed URL | The port is easy to find; a 128-bit token cannot be guessed. Treat the printed URL as a secret. |
| Token leaking through history, logs, or referrers | The token is exchanged once for an `HttpOnly` cookie, followed by a redirect to `/`; request logs omit query strings; `Referrer-Policy: no-referrer` | A token in every URL would end up in browser history and logs on each reconnect. `EventSource` cannot send custom headers, so the cookie carries authentication for the event stream. |
| DNS rebinding (a hostile domain resolving to `127.0.0.1`) | The `Host` header must be `127.0.0.1:<port>` or `localhost:<port>` | A rebinding page sends its own domain as `Host`, so the check rejects it. |
| A web page submitting forms or `fetch` calls | `POST` requires the server's own `Origin` and `Content-Type: application/json` | Browsers do not let a cross-origin page send JSON without a preflight, and they always attach `Origin` to such requests. |
| A page on another localhost port | `SameSite=Strict`, plus refusal of any request whose `Sec-Fetch-Site` is not `same-origin` or `none` | `SameSite` treats all localhost ports as one site, so it does not help here. Some read requests call plugins and record history, and an `<img>` tag on another port could send one with the cookie and no `Origin`. `Sec-Fetch-Site` tells the server where the request really came from. |
| Two `finfocus --web` sessions overwriting each other | The cookie name includes the port: `finfocus_session_<port>` | Cookies are not isolated by port. |
| Injected or third-party script | `Content-Security-Policy: default-src 'self'` and no CDN assets | The page runs only the scripts the binary ships. |

The cookie has no `Secure` attribute. The server speaks plain HTTP on
loopback, and a `Secure` cookie would never be sent back. TLS was rejected as
disproportionate for a tool that never leaves the machine.

### What the browser can see

The rule is that the dashboard never shows more than the equivalent CLI JSON or
terminal view already does:

- Pulumi secret values, credential-like property names, and `__`-prefixed
  Pulumi internals such as `__defaults` are removed with the engine's shared
  rules, the same ones that filter plugin attributes and nested tags
  (`engine.IsHiddenPropertyKey` and `history.IsPulumiSecret`). The web layer keeps
  no rule list of its own, so the rules cannot drift apart. Redaction covers
  resource details, property diffs, and the estimate view's editable
  properties.
- Budget health uses the CLI's JSON shape, which has no notification
  destinations, so webhook URLs, Slack channels, and headers never reach the
  page.
- A stack passphrase is passed only in the `pulumi` subprocess environment. It
  never appears in a response, an event, or a log line.
- Error messages from the Pulumi subprocess, which can contain its stderr, are
  replaced with fixed, safe labels before they reach the page.

Maps keyed by domain data (a resource URN, a breakdown line, or a service,
provider, or adapter label) keep their keys. Pulumi secret values inside
those maps are still removed. Property maps, property diffs, and estimate
overrides still drop credential-like names.

The Config page does not use that payload redactor. Its JSON is already masked:
the values of secret settings and notification destinations are omitted, while
each setting and its set or source status stay in the response, and the handler
writes that document directly. A redactor that drops credential-looking keys
would delete settings the page is supposed to list.

## Log capture

The optional Logs view (`web.logs.enabled`, `FINFOCUS_WEB_LOGS`) shows the log
lines of the core process and its plugins. Four choices shape it.

**Lines are parsed and redacted in Go.** The redaction rules live in Go
(`config.IsCredentialName` for names, `history.IsPulumiSecret` for values), and
the project keeps one copy of each. Redacting in JavaScript would need a second
list that could drift. So the `internal/weblog` package turns each raw line
into an entry, redacts its fields, and sets its display strings before the line
is stored. The browser receives entries and never a raw line. The free-text
`message` is not redacted, because it cannot be inspected reliably; plugins are
told never to log credentials. `finfocus logs` uses the same package, so the
terminal and the page show one pipeline's output.

**Plugins stop receiving `FINFOCUS_LOG_FILE` while logs are on.** When core
logs to a file, it normally passes that path to each plugin, and the plugin SDK
then opens the file and writes straight to it. Core would see nothing on the
plugin's standard error. With the view on, core omits the variable, so the SDK
writes JSON to standard error. Core copies each plugin's standard error to its
existing log destination and to the log hub, so the file receives the same
lines as before. With the view off, the plugin environment is unchanged.
Core's own lines reach the hub through a tee writer on the logger, next to the
normal output.

**The viewer is a separate package.** `packages/zerolog-viewer` is plain ES
modules with no dependencies and no build step. It imports nothing outside its
own `src` folder, calls no network API, and never writes HTML from log text. A
test enforces that. The dashboard's `views/logs.js` is the only glue: it opens
the stream and feeds entries to the viewer. That keeps the viewer usable on its
own and keeps the dashboard's page free of log-parsing code.

**The stream is dedicated.** Log entries use their own server-sent event
stream, `/api/logs/stream`, not the overview event bus. That bus redacts
payloads with the engine's data-keyed rules and drops slow subscribers, which
suits neither raw log entries nor a replayable buffer. The log stream sends a snapshot of
the buffer first, then each new entry with its sequence number as the event id,
so a reconnect resumes without duplicates. The hub keeps a bounded in-memory
ring (5000 entries) and drops the oldest entry when it is full. A `gap` event
counts entries already dropped from that ring. The page says they were dropped
from the buffer.

When the setting is off, the server still registers `GET /api/logs/status`
and answers `200` with `enabled` false, so the dashboard's startup probe is
not a failed request. It does not register `/api/logs/stream`, so that path
answers 404 like any unknown path. The header link stays hidden, and no log
hub exists to capture anything. Requests under `/api/logs/` are left out of
the request log, so watching the logs does not add lines to them.

`internal/webui` reaches the hub only through a small `LogSource` interface, so
it does not import `cli`, `registry`, or `pluginhost`.

## Layout and adding a feature

Every view is built from the same slots, in the same order, so a new feature
reads as part of the page rather than an add-on:

1. The header: the product name, the view links, and the connection status.
2. The view heading.
3. A toolbar (`.toolbar`): filter, sort, and an apply or run button.
4. A status line (`role="status"`) for loading, progress, and errors.
5. A totals bar (`.totals-bar`) that summarizes the table below it.
6. A captioned table (`.table-wrap`, `.overview-table` or `.detail-table`)
   with numeric columns aligned, a pager, and a detail panel opened from a row.
7. Panels below the table, such as budget health.

The Overview is the reference. A feature that adds columns, badges, deltas, or
a detail section extends it. A feature with its own data and controls becomes
a new view built from the same slots. Neither adds arithmetic to the
JavaScript, since display strings come from the Go side.

A CLI feature that produces user-visible output ships its web hook together
with the CLI feature. The handler in `internal/webui` calls the same Go method
as the CLI command, and the spec's HTTP contract names that method. The later
web ticket is then UX only: layout, rendering, interaction, copy, and
accessibility. If it needs engine, API, redaction, or data-shape work, that
work was part of the CLI feature.

To check a fit, render the nearest existing view in a browser against a
project with real resources, and compare it with the spec's `Web UI Fit`
section before the CLI feature is called complete.

## Boundaries

- **Read-only toward cloud infrastructure.** The dashboard cannot run
  `pulumi up` or `pulumi destroy`, and it cannot dismiss recommendations. Its
  cloud side effects are those the terminal UI already has: plugin queries,
  `pulumi preview`, and the optional cost cache and history.
- **Configuration files are user settings.** The Config page and the Plugins
  page can write the resolved project or global `config.hujson`. That file is
  not cloud infrastructure. Alert, routing, and analyzer settings stay
  read-only on the Config page.
- **Plugin files are local user state.** The Plugins page can install, update,
  and remove catalog plugins in the directory `plugin list` already reads.
  Those files are not cloud infrastructure. Install, update, and remove are
  not agent tools.
- **No other new store.** Deleting the local stores does not stop `--web`
  from launching.
- **Not an MCP surface.** `--web` and `--mcp` are mutually exclusive, and the
  dashboard adds no MCP tools.
- **Pulumi only.** The overview, recommendations, and estimate experiences
  have no Terraform input path in the CLI yet, so the browser has none either.
