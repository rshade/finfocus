---
title: Web UI Reference
description: Launch rules, output, exit codes, views, controls, and HTTP behavior of the finfocus --web browser dashboard.
---

Reference for the browser dashboard that `finfocus --web` serves. For
step-by-step tasks, see [Use the browser dashboard](../guides/web-ui.md). For
the reasoning behind the security model, see
[Web UI design](../architecture/web-ui.md).

## Launch

```bash
finfocus --web [--port N] [--no-browser] [--pulumi-json FILE] [--pulumi-state FILE]
               [--stack NAME] [--from DATE] [--to DATE] [--adapter NAME] [--filter EXPR]...
```

- `--web` and its companion flags are root-local: they exist only on the root
  `finfocus` command and no subcommand inherits them.
- Every companion flag requires `--web`. `--web` and `--mcp` cannot be combined.
- With no source flag, the Pulumi project, stack, and state are detected from
  the working directory, as for `finfocus overview`.
- `--terraform-state` is not accepted.

The flag table, with defaults, is in the
[CLI Commands Reference](cli-commands.md#options-web).

## Output

| Stream | Content |
| --- | --- |
| stdout | One line: the session URL, `http://127.0.0.1:<port>/?token=<token>`. The token is 128 random bits as 32 hex characters. |
| stderr | Diagnostics and one request log line per request that passes the `Host` check: method, path without the query string, and status. Tokens and request bodies are never logged. |

Unless `--no-browser` is set, FinFocus opens the URL with the platform opener
(`open` on macOS, `rundll32` on Windows, `xdg-open` elsewhere). A failure to
start the opener is logged as a warning; the server keeps running.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | The server shut down cleanly after `Ctrl+C`. |
| 1 | Launch failure: no Pulumi project found, port unavailable or outside 0-65535, unknown flag (including `--web` on a subcommand). |
| 2 | Invalid input: a companion flag without `--web`, `--web` with `--mcp`, or an unparsable `--from`/`--to`. |

## Views

The header links to four views. Each has its own URL fragment, and a
**Skip to main content** link comes first in the tab order.

| View | Fragment | Shows |
| --- | --- | --- |
| Overview | `#/overview` | Loading checklist; resource table (Resource, Type, Status, Actual MTD, Projected, Delta, Drift%, Recs, Warn); totals; budget health; resource details |
| Cost | `#/cost` | Actual costs, summary with provider shares and carbon equivalencies when plugins supply them, trends when history is available |
| Recommendations | `#/recommendations` | Recommendation counts, savings, actions, descriptions, and scorer signals when scoring is configured |
| Estimate | `#/estimate` | What-if comparison for one resource: baseline, modified monthly cost, total change, and per-property deltas |

### Overview controls

| Control | Effect |
| --- | --- |
| Filter | Filters resources by name or type. `Escape` in the box clears it. |
| **Sort** | Cycles the sort field: Cost, Name, Type, Delta. |
| **Run preview** | Runs `pulumi preview` on demand and applies pending changes. One preview runs at a time; elapsed time is shown while it runs. |
| Cluster expand/collapse | Expands or collapses a Kubernetes cluster row. |
| Resource row | Opens details: actual and projected breakdowns, current and after-change cost impact, cost drift, property changes, recommendations, budget status. |
| **Previous** / **Next** | Pages through large tables. |

When a stack mixes currencies, resource rows stay visible and the totals show
an explicit totals-unavailable message instead of a sum.

### Cost controls

| Control | Effect |
| --- | --- |
| Filter | Filters by resource or type. |
| Group by | None, Resource, Type, Provider, Daily, or Monthly. The CLI's deprecated `date` alias is not offered. |
| Tag filter | A `key=value` tag filter, parsed as the CLI parses it. |
| Sort by | Cost, Name, Type, or Delta. For Daily and Monthly, Name becomes Period and Type and Delta are disabled. |
| Row | Opens actual-cost details, breakdown, sustainability metrics, and linked recommendations. Time-period rows have no details. |

Results are paged at 250 rows.

### Recommendations controls

| Control | Effect |
| --- | --- |
| Filter | Filters by resource, action, or description. |
| Sort by | Savings, Resource, or Action. |
| **Include dismissed** | Fetches dismissed recommendations too. It reads dismissal records and never changes them. |
| Row | Opens details, including every scorer signal and reasoning. |

Dismissed recommendations are hidden by default. The browser cannot dismiss,
snooze, or undismiss a recommendation. Results are paged at 250 rows.

### Estimate controls

| Control | Effect |
| --- | --- |
| Resource | Chooses the resource. Resources that the initial preview creates are included, also on a first deployment. |
| Pricing mode | **Automatic provider**, or a mode from a plugin's pricing spec. Choosing a mode recalculates through that plugin. |
| Current value | Edit a property value and press `Enter` to recalculate. |

### Web-only controls

The **Group by** selector and the **Include dismissed** checkbox expose existing
CLI options that the terminal UI fixes at launch or lacks. Both go through the
same engine code path as the CLI flags.

## Keyboard and display

| Input | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Move between controls |
| `Enter` / `Space` | Activate the focused button |
| `Enter` in an estimate value | Recalculate |
| `Escape` | Close the open detail panel or dialog; in the overview filter box, clear the filter |

Closing a detail view returns focus to the control that opened it. Light and
dark palettes follow the browser's `prefers-color-scheme` setting.

## HTTP behavior

The server binds `127.0.0.1` only. All responses carry
`Cache-Control: no-store`, `Referrer-Policy: no-referrer`, and
`Content-Security-Policy: default-src 'self'`. Errors are JSON:
`{"error": "<message>", "code": "<code>"}`, except that a missing file under
`/static/` gets Go's standard plain-text 404. An unknown path, or an API path
called with the wrong method, gets `404` / `not_found` (`unknown path`).

| Request | Status | Code | Message |
| --- | --- | --- | --- |
| `GET /?token=<valid>` | 303 to `/` | - | Sets the session cookie |
| `GET /?token=<wrong>` | 401 | `unauthorized` | `invalid session token; open the URL printed in the terminal` |
| Any other request without the session cookie | 401 | `unauthorized` | `valid session cookie required; open the URL printed in the terminal` |
| `Host` other than `127.0.0.1:<port>` or `localhost:<port>` | 403 | `forbidden` | `host header is not this server` |
| `Sec-Fetch-Site` present and other than `same-origin` or `none` | 403 | `forbidden` | `request is not from this server` |
| `Origin` other than the server's own | 403 | `forbidden` | `origin is not this server` |
| `POST` without `Origin` | 403 | `forbidden` | `origin header required` |
| `POST` whose `Content-Type` is not `application/json` | 415 | `unsupported_media_type` | `Content-Type must be application/json` |

The session cookie is `finfocus_session_<port>`, with `HttpOnly`,
`SameSite=Strict`, and `Path=/`, and without `Secure`. The token stays valid
until the server stops.

## Side effects and storage

- Read-only toward infrastructure. The only actions are plugin queries, the
  on-demand `pulumi preview`, and the CLI's existing optional cost cache and
  history reads and writes, which keep their configured defaults.
- The existing dismissal store is read so earlier decisions are honored.
- No browser-specific store is added. Missing local stores do not prevent
  launch.
- Pages load only from the local server; nothing comes from a CDN.
