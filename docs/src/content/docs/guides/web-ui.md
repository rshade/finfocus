---
title: "Use the browser dashboard"
description: "Launch FinFocus locally and explore overview, actual costs, recommendations, and what-if estimates in your browser."
layout: "docs"
---

Run FinFocus from the directory containing your Pulumi program. The browser
dashboard uses the same engine, plugin routing, calculations, filtering,
sorting, formatting, and recommendation scoring as the CLI and terminal UI.
Install the plugins your resources require before launching.

## Launch a session

```bash
cd /path/to/your/pulumi-project
finfocus --web
```

FinFocus auto-detects the project, current stack, and exported state as
`finfocus overview` does. It prints a URL and opens your default browser.
The operating system chooses an available port unless you specify one:

```bash
finfocus --web --no-browser --port 8484 --stack dev
```

`--no-browser` suppresses automatic opening; follow the printed URL yourself.
It is useful over SSH and in environments with no graphical browser. A port
already in use produces a launch error. Sources are chosen at launch; there
is no file picker or upload control.

## Choose sources and dates

Use exported Pulumi files when you manage source generation yourself:

```bash
pulumi stack export > state.json
pulumi preview --json > plan.json
finfocus --web --pulumi-state state.json --pulumi-json plan.json --no-browser
```

An explicit preview outside a detected Pulumi project can also load by itself:

```bash
finfocus --web --pulumi-json /path/to/plan.json --no-browser
```

An explicit source takes precedence over auto-detection. In a detected project,
a preview-only launch also exports the project's current state for the overview.
Use `--pulumi-state` when you need to supply that state explicitly.

| Root-local companion | Purpose |
| --- | --- |
| `--pulumi-json` | Pulumi preview JSON source |
| `--pulumi-state` | Pulumi stack export source |
| `--stack` | Override the detected Pulumi stack |
| `--from`, `--to` | Date range, using overview's date parsing and defaults |
| `--adapter` | Select one installed cost plugin |
| `--filter` | Apply an overview-compatible launch filter |
| `--port` | Bind a particular loopback port |
| `--no-browser` | Print the URL without opening a browser |

For example, pin a historical range using dates or RFC3339 timestamps:

```bash
finfocus --web --pulumi-state state.json --from 2026-09-01 --to 2026-09-30
```

These flags are defined on the root command and require `--web`. They are not
inherited by subcommands: `finfocus cost actual --web` is invalid.
`--web` and `--mcp` cannot be used together. `--project-dir` still controls
configuration resolution; it does not move Pulumi source detection away from
the working directory. Terraform state is not supported by the browser UI,
and `--terraform-state` is not a web launch flag.

## Explore the four views

- **Overview:** follow loading phases and resource enrichment, filter and sort
  resources, open cost and property details, inspect budget health, and expand
  or collapse clusters. Rows and totals come from the shared overview pipeline.
  Mixed-currency stacks retain resource rows and show an explicit totals-unavailable
  message. Details include current and after-change cost impact, cost drift,
  budget forecasts and triggered thresholds.
- **Cost:** inspect actual costs, breakdowns, trends when history is available,
  and sustainability metrics supplied by plugins. The summary includes resource
  and recommendation counts, provider shares and carbon equivalencies when
  available. Group by None, Resource,
  Type, Provider, Daily, or Monthly, or apply a `key=value` tag filter.
  The deprecated CLI `date` grouping alias is not offered. Time groups support
  Cost and Period sorting; Type and Delta do not apply to time periods.
- **Recommendations:** inspect counts, savings, actions, descriptions, and
  scorer signals when scoring is configured. Dismissed recommendations are
  hidden by default; **Include dismissed** fetches them without changing any
  dismissal record. The browser cannot dismiss or undismiss recommendations.
- **Estimate:** select a resource, edit its current property values, and press
  Enter to recalculate. Resources created by the initial preview are selectable,
  including on a first deployment. The comparison shows baseline, modified monthly cost,
  total change, and property deltas. Pricing modes identify real plugin pricing
  providers; selecting a mode recalculates through that provider.

Group selection and Include dismissed are web-only controls for existing CLI
options. The TUI keeps the grouping selected at launch and has no dismissal
visibility toggle. Tables paginate large results. Use Tab/Shift+Tab to reach
controls, Enter or Space to activate buttons, and Escape to close modal details.
Detail views restore focus to their launching control. Light and dark palettes
follow your browser's preferred color scheme.

## Preview and encrypted stacks

**Run preview** starts an on-demand `pulumi preview`, corresponding to the TUI's
preview key in state-only mode. The overview shows elapsed time while it runs.
Only one preview runs at a time. Preview updates the overview's pending changes;
there is no general refresh control. Restart the session to fetch fresh actuals.

An encrypted project stack opens a masked passphrase prompt. A wrong passphrase
shows an inline error and allows another attempt, including during preview.
FinFocus passes the value only in the Pulumi subprocess environment. It does not
put passphrases in application responses, events, or logs, or save them to a store.

## Local security and storage

The server listens on `127.0.0.1` and offers no non-loopback binding option.
The printed URL carries a random token valid for the lifetime of the process.
Opening it sets an HTTP-only, SameSite Strict cookie named for the server port
and redirects to a URL without the token. Use the printed URL again for another
tab or browser profile; reloads recover the current session state.

Requests need the session cookie, an accepted loopback Host, and the server's
own Origin when an Origin header is present. POST requests also require that
Origin and JSON content type. The cookie uses local HTTP; it has no Secure flag.
Keep the printed URL private. Assets load from the local server, with no CDN.
Responses use `no-store` caching and the shared secret-redaction rules.

The dashboard is read-only toward infrastructure. Its side effects are plugin
queries, Pulumi preview, and the CLI's existing optional cost cache and history
reads and writes. Cache and history retain their normal configured defaults;
no browser-specific store is added. Missing local stores do not prevent launch.
The existing dismissal store is read to honor prior decisions.

Closing a browser tab does not stop FinFocus. Press Ctrl+C in the launching
terminal to shut down the server and its plugins.

## Run the browser acceptance tests

From the repository root:

```bash
make test-frontend
make test-e2e-web
```

The frontend tests use Node pinned in `mise.toml`. Browser acceptance builds the
real Go binary, installs Chromium with the exact Playwright version resolved by
the nested `test/e2e` module, and runs the `e2e_web` tests. The fixture plugin and
Pulumi process are local and deterministic; no cloud credentials are required.
The Makefile and CI require the browser tests to run. Direct `go test` invocation
can skip with a clear message if the binary, driver, or browser is missing.

On a machine missing Chromium's system libraries, use:

```bash
make test-e2e-web PLAYWRIGHT_INSTALL_ARGS='--with-deps chromium'
```

The root Go dependency graph and the pure Go `make build` command do not need
Playwright or Node.
