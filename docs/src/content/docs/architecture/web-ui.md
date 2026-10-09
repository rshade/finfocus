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
load runs through the same six-phase pipeline as `finfocus overview`.

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

Because the redaction runs over every JSON payload, a data map whose keys look
like credential names can lose entries. Issue
[#1752](https://github.com/rshade/finfocus/issues/1752) tracks this.

## Boundaries

- **Read-only.** The dashboard cannot change infrastructure or dismiss
  recommendations. Its side effects are those the terminal UI already has:
  plugin queries, `pulumi preview`, and the optional cost cache and history.
- **No new state.** It adds no store of its own and works with every local
  store deleted.
- **Not an MCP surface.** `--web` and `--mcp` are mutually exclusive, and the
  dashboard adds no MCP tools.
- **Pulumi only.** The overview, recommendations, and estimate experiences
  have no Terraform input path in the CLI yet, so the browser has none either.
