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

The header links to six views. Each has its own URL fragment, and a
**Skip to main content** link comes first in the tab order.

| View | Fragment | Shows |
| --- | --- | --- |
| Overview | `#/overview` | Loading checklist; resource table (Resource, Type, Status, Actual MTD, Projected, Delta, Drift%, Recs, Warn); totals; budget health; resource details |
| Cost | `#/cost` | Actual costs, summary with provider shares and carbon equivalencies when plugins supply them, trends when history is available |
| Recommendations | `#/recommendations` | Recommendation counts, savings, actions, descriptions, and scorer signals when scoring is configured |
| Estimate | `#/estimate` | What-if comparison for one resource: baseline, modified monthly cost, total change, and per-property deltas |
| Carbon | `#/carbon` | Provider-published monthly carbon emissions for complete UTC months, with source, provider, grouping, period, model version, and retrieval time per record |
| Config | `#/config` | Effective configuration, the tier each value comes from, and edits to known settings in the project or global file |
| Plugins | `#/plugins` | Built-in catalog, installed versions, install, update, remove, validate, and per-plugin settings |

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

Recommendations use the same live inventory as `finfocus cost recommendations`:
preview resources plus unchanged deployed resources, including preview-only
creates. A resource present in both state and preview uses the preview version.

| Control | Effect |
| --- | --- |
| Filter | Filters by resource, action, or description. |
| Sort by | Savings, Resource, or Action. |
| **Include dismissed** | Fetches dismissed recommendations too. It reads dismissal records and never changes them. |
| Row | Opens details, including every scorer signal and reasoning. |

Dismissed recommendations are hidden by default. The browser cannot dismiss,
snooze, or undismiss a recommendation. Results are paged at 250 rows.

### Carbon controls

The Carbon view calls the same Go method as `finfocus carbon actual` and shows
the same report as `--output json`. It sends nothing until you press **Run
report**, because each query launches the `aws-ce` plugin and calls AWS.

| Control | Effect |
| --- | --- |
| Start, End | First-of-month UTC dates in `YYYY-MM-DD`. The end date is exclusive. |
| Accounting method, Scope | Market-based or location-based; total, scope 1, 2, or 3. |
| Accounts, Regions, Services | Optional comma-separated filters. |

The total is the sum of the returned available records, calculated by the
server. A record with no value shows **Missing**, which differs from an
explicit `0`. A report with no records keeps the query you ran and states that
the total is unavailable, not zero. A failed query clears earlier results and
shows a fixed message; a disabled provider names the setting that enables it.

### Estimate controls

The resource picker uses the same live inventory as `finfocus cost estimate`,
including unchanged deployed resources and preview-only creates. Estimate and
Recommendations read the shared snapshot used by Overview.

| Control | Effect |
| --- | --- |
| Resource | Chooses the resource. Resources that the initial preview creates are included, also on a first deployment. |
| Pricing mode | **Automatic provider**, or a mode from a plugin's pricing spec. Choosing a mode recalculates through that plugin. |
| Current value | Edit a property value, then press `Enter` or leave the field to recalculate. Leaving a field you did not change does not recalculate. |

If the baseline price fails, the property rows and pricing modes still appear,
the comparison stays empty, and the error is shown inline. Editing a value or
choosing a mode tries the price again. An unknown pricing mode is rejected
without those rows.

### Config controls

| Control | Effect |
| --- | --- |
| Edit tier | Selects the project or global `config.hujson` to edit. Project is disabled when no project directory resolves. |
| Setting editor | Changes one known setting. A secret field starts empty; leaving it blank keeps the stored secret. |
| Remove | Drops that key from the selected tier only. |
| **Review changes** | Asks the server for a before/after preview. The preview writes nothing. |
| **Confirm** | Writes the previewed changes to the selected file. |
| **Cancel** | Closes the preview. The file is unchanged. |
| **Reload** | Reads both configuration files again. |

Secrets and notification destinations are shown as set or not set. The page
never fills a secret field from the stored value. Alert, routing, and analyzer
settings are listed and are not editable. A successful save tells you that the
running session keeps its startup configuration.

### Plugins controls

| Control | Effect |
| --- | --- |
| Catalog row | Shows the name, description, providers, capabilities, security level, installed versions, and the latest catalog version. An older install is marked update-available. |
| Outside the catalog | An installed plugin that is not a catalog entry. Validate, settings, and remove stay available. Install and update are not offered. |
| **Install** / **Update** / **Remove** | Opens a confirmation that names the plugin, version, repository, and security level. The download or delete starts after confirmation. |
| Progress | Shows the current phase and, while downloading, bytes received and the total when the host reports one. |
| **Validate** | Checks the installed files and that the plugin process starts. A failure stays on the page with the reason. |
| Settings tier | Selects the project or global `config.hujson`, the same files as Config. |
| Setting editor | Edits one plugin setting. A secret field starts empty; leaving it blank keeps the stored secret. When ESC supplies the effective value, the Status cell reads `from ESC (<reference>)` and the help text names that environment. The page never shows the variable's value. |
| **Review changes** / **Confirm** | Previews, then writes, through the same configuration writer as Config. |
| Save notice | Saved settings apply the next time that plugin is launched. |

The page installs catalog names only. A download URL is not accepted. When the
newest release has no files yet, the confirmation names the older version that
will be installed. A missing checksum file is named before the install
continues. A checksum that does not match stops the install.

### Web-only controls

The **Group by** selector and the **Include dismissed** checkbox expose existing
CLI options that the terminal UI fixes at launch or lacks. Both go through the
same engine code path as the CLI flags.

## Keyboard and display

| Input | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Move between controls |
| `Enter` / `Space` | Activate the focused button |
| `Enter` in an estimate value, or `Tab` out of an edited one | Recalculate |
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
| `GET /api/config` | 200 | - | Effective configuration. Secret values are omitted. |
| `POST /api/config/preview` | 200 | - | Before/after diff. Writes nothing. |
| `POST /api/config` | 200 | - | Writes the selected `config.hujson` and returns the page plus a reload notice. |
| `POST /api/config` with a stale `baseVersion` | 409 | `conflict` | `configuration changed since it was loaded; reload and try again` |
| `POST /api/config` or `/api/config/preview` with no `baseVersion` | 400 | `invalid_request` | `baseVersion is required` |
| `POST /api/config` with the project tier and no project | 400 | `project_unavailable` | `no project directory is available` |
| `POST /api/config` that fails validation | 400 | `validation` | The message names the setting. `fields` lists each failure. The file is unchanged. |
| `GET /api/plugins/catalog` | 200 | - | Every built-in catalog entry, plus installed plugins that are outside the catalog. |
| `POST /api/plugins/{name}/plan` | 200 | - | Names the version, repository, and security level. Downloads nothing. |
| `POST /api/plugins/{name}/install` or `/update` | 202 | - | Starts the confirmed install. The body matches the fresh plan. |
| `DELETE /api/plugins/{name}/{version}` | 202 | - | Starts the confirmed removal of that version. |
| `GET /api/plugins/operations/{id}/stream` | 200 | - | Server-sent events: `snapshot`, then `progress`, then `done`. |
| `POST /api/plugins/{name}/validate` | 200 | - | `{passed, reason}`. A failed check is still HTTP 200. |
| `GET /api/plugins/{name}/settings` | 200 | - | Plugin settings. Secret values are omitted. |
| `POST /api/plugins/{name}/settings/preview` | 200 | - | Before/after diff for that plugin. Writes nothing. |
| `POST /api/plugins/{name}/settings` | 200 | - | Writes the selected `config.hujson`. The message says the settings apply the next time the plugin is launched. |
| Plugin plan or install for a name outside the catalog | 404 | `not_found` | `plugin is not in the catalog` |
| Install or update whose confirmation does not match a fresh plan | 409 | `conflict` | The named version, repository, or security level changed. |
| A second install, update, or remove of the same plugin | 409 | `busy` | That plugin is already being modified. |
| Catalog host failure | 502 | `upstream` | A rate limit says to set `GITHUB_TOKEN`. Other host failures stay generic. |

The session cookie is `finfocus_session_<port>`, with `HttpOnly`,
`SameSite=Strict`, and `Path=/`, and without `Secure`. The token stays valid
until the server stops.

## Side effects and storage

- Read-only toward infrastructure. The dashboard does not run `pulumi up` or
  `pulumi destroy`. Plugin queries, the on-demand `pulumi preview`, and the
  CLI's existing optional cost cache and history reads and writes keep their
  configured defaults.
- Config can write the resolved project or global `config.hujson`. The write
  patches that file in place. Comments, unknown keys, and the other tier's file
  stay. The running session does not reload the file.
- Plugins can install, update, and remove a plugin in the local plugin
  directory, and can write that plugin's settings through the same
  configuration files. Saved settings apply the next time the plugin is
  launched. The dashboard opens its plugins again after a successful install,
  update, remove, or settings save.
- The existing dismissal store is read so earlier decisions are honored.
- No browser-specific store is added. Missing local stores do not prevent
  launch.
- Pages load only from the local server; nothing comes from a CDN.
