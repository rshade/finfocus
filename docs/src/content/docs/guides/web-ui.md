---
title: "Use the browser dashboard"
description: "Launch FinFocus locally and explore overview, actual costs, recommendations, and what-if estimates in your browser."
layout: "docs"
---

`finfocus --web` serves the overview, actual-cost, recommendations, and what-if
estimate experiences as a local web page. It uses the same engine, plugin
routing, calculations, filtering, sorting, formatting, and recommendation
scoring as the CLI and terminal UI, so the browser shows the same numbers.

This guide covers the common tasks. For every flag, view, and status code, see
the [Web UI reference](../reference/web-ui.md). For why the server is built the
way it is, see [Web UI design](../architecture/web-ui.md).

## Before you begin

- Install the plugins your resources need (`finfocus plugin list` shows what
  is installed). The browser adds no pricing of its own.
- Have a Pulumi project, or exported Pulumi preview or state files. Terraform
  state is not supported by the browser dashboard.
- Use a browser on the same machine that runs `finfocus`. The server listens
  only on `127.0.0.1` and has no option to listen on another interface.

## Open the dashboard for a Pulumi project

Run FinFocus from the directory that contains your Pulumi program:

```bash
cd <your-pulumi-project>
finfocus --web
```

FinFocus detects the project, the current stack, and its state the same way
`finfocus overview` does. It prints the session URL on stdout and opens your
default browser on it. The page fills in progressively while plugins answer.
The URL contains a session token, so keep it private.

To use a stack other than the current one, add `--stack`:

```bash
finfocus --web --stack dev
```

`--project-dir` still controls only configuration resolution. Source detection
always starts from the working directory.

## Run without opening a browser

On a machine with no graphical browser, or when you want to choose the browser
yourself, suppress the automatic open:

```bash
finfocus --web --no-browser
```

Open the printed `http://127.0.0.1:<port>/?token=<token>` URL yourself. If
FinFocus cannot start a browser, it logs a warning and keeps serving, so the
printed URL still works.

## Pin the port

The operating system picks a free port unless you set one:

```bash
finfocus --web --port 8484
```

A port that is already in use stops the launch with an error naming the port.
Pick another port, or omit `--port`.

## Use exported Pulumi files

When you generate sources yourself, for example in a script, pass them
explicitly:

```bash
pulumi stack export > state.json
pulumi preview --json > plan.json
finfocus --web --pulumi-state state.json --pulumi-json plan.json
```

An explicit source takes precedence over auto-detection. A preview file also
loads by itself outside a Pulumi project:

```bash
finfocus --web --pulumi-json plan.json
```

Inside a detected project, a preview-only launch still exports the project's
current state for the overview. Pass `--pulumi-state` to supply that state
yourself.

## Narrow the data

The companion flags work as they do on `finfocus overview`:

```bash
# Actual costs for September (YYYY-MM-DD or RFC3339)
finfocus --web --from 2026-09-01 --to 2026-09-30

# Query one installed plugin only
finfocus --web --adapter aws-public

# Only AWS resources
finfocus --web --filter provider=aws
```

These flags are valid only together with `--web`, and only on the root
`finfocus` command. Sources and dates are fixed for the session; the page has
no file picker or upload control. Restart the session to change them.

## Unlock an encrypted stack

If the stack uses a passphrase secrets provider, the overview opens an
**Encrypted stack** dialog. Enter the passphrase and choose **Unlock**. A wrong
passphrase shows an inline error and lets you try again.

FinFocus hands the passphrase only to the `pulumi` subprocess environment. It
never puts it in a response, an event, or a log line, and never saves it.

## Preview pending changes

When the session starts from state only, choose **Run preview** in the overview
to run `pulumi preview` on demand, like the `p` key in the terminal UI. The
overview shows the elapsed time and updates pending changes when the preview
finishes. Only one preview runs at a time.

There is no general refresh. To fetch fresh actual costs, stop the session and
launch it again.

## Choose a What-If resource

The What-If picker hides Pulumi provider and stack bookkeeping by default.
It also hides resources confirmed non-billable by available, agreeing plugin
pricing metadata. **Show all resources** restores the complete safe source list,
within your launch filters. An eligible selected resource and its unsaved edits
stay selected when you toggle it, provided the source properties are unchanged.
If a preview changes those properties during the toggle, What-If reloads the
editor and baseline from the updated source. A failed resource-list refresh
shows a separate error and preserves the current estimate.

Zero baseline, zero rate, or zero actual spend does not establish non-billability.
Unsupported resources, missing region or SKU, pricing errors, and unknown
billability remain available for estimation. Metadata is used only when already
available for the effective pricing providers; listing resources never triggers
new pricing probes. A positive projected baseline helps choose the initial
resource without affecting which resources are visible.

If only known non-billable resources remain, the picker explains the empty list
and offers **Show all resources**. Planned resources and existing launch filters
still apply. Interactive plan estimation uses the same default eligibility
policy; explicit `cost estimate --provider ... --resource-type ...` remains
available for inspecting a specific type.

## Open another tab or reconnect

Open the printed URL again to start another tab or browser profile. The URL
stays valid until the server stops. Reloading a page recovers the current load
state instead of restarting the load.

If the page reports that a valid session cookie is required, the browser has no
session for this server. Open the URL printed in the terminal.

## Edit configuration

Open **Config** in the header. The page lists each known setting, the value
that is in effect, and whether that value came from the built-in default, the
global file, the project file, or the environment.

Choose **Project** or **Global**, edit the values, and choose **Review
changes**. The preview names each change and writes nothing. **Confirm**
updates only that tier's `config.hujson`. **Cancel** leaves the file as it was.
Comments and keys the page does not edit stay in the file.

A secret is shown as set or not set. The field starts empty. Leave it blank to
keep the stored value, or type a replacement. The page does not show the stored
secret and does not write the masked placeholder back to the file.

The running dashboard keeps the configuration it loaded at startup. The new
values apply to the next launch and to command-line runs. Alert destinations,
routing, and analyzer settings are visible and are not editable on this page.

## Install and configure plugins

Open **Plugins** in the header. The page lists every built-in catalog plugin
and any installed plugin that came from outside the catalog.

Choose **Install**, **Update**, or **Remove**. The confirmation names the
plugin, the version, the source repository, and the security level. Nothing
is downloaded until you confirm. Progress then shows each phase, including
bytes received when the download reports a size. A second operation on the
same plugin is refused until the first one finishes. Try it again afterward.

A plugin installed from outside the catalog can be validated, configured, and
removed. Install and update stay on the catalog entries. The page does not
accept a download URL. Custom registry addresses stay on the command line.

**Validate** checks the installed files and that the plugin starts. It does
not certify the plugin.

Plugin settings use the same **Project** or **Global** choice as Config.
Review the changes, then save. A secret field starts empty. Leave it blank to
keep the stored secret. The page never shows the stored secret. Saved
settings apply the next time that plugin is launched. The dashboard opens
plugins again after a successful install, update, remove, or settings save.
A plugin process that was already running keeps the environment it started
with until that reopen.

An environment variable that is already set for the dashboard process takes
precedence over the project file, the global file, and the plugin's default.
Set a secret that way when you do not want it written to a file.

## Stop the session

Closing the browser tab does not stop FinFocus. Press `Ctrl+C` in the terminal
that launched it to shut down the server and its plugins.

## Troubleshooting

| Message | Cause and fix |
| --- | --- |
| `--port is only valid with --web` (any companion flag) | A companion flag was given without `--web`. Add `--web`. Exit code 2. |
| `--web and --mcp are mutually exclusive` | Run the web dashboard and the MCP server as separate processes. Exit code 2. |
| `unknown flag: --web` | `--web` was put on a subcommand, such as `finfocus overview --web`. Use `finfocus --web`. |
| `no Pulumi project found in current or parent directories` | Run from the Pulumi project directory, or pass `--pulumi-json`. |
| `port <n> on 127.0.0.1 is unavailable` | Another process holds the pinned port. Choose another `--port` or omit it. |
| `valid session cookie required; open the URL printed in the terminal` | Open the printed URL, not the bare address. |
| `invalid session token; open the URL printed in the terminal` | The token belongs to an earlier session. Use the URL from the current run. |
| No cost data | Check that the needed plugins are installed: `finfocus plugin list`. |

## Limitations

- Pulumi only. `--terraform-state` is not a `--web` flag.
- Local only. There is no option to listen on a non-loopback address and no
  TLS.
- The browser cannot dismiss, snooze, or undismiss recommendations. Use
  `finfocus cost recommendations dismiss` and its siblings.
- **Run preview** is offered even when the session already has preview data,
  and state-only sessions do not show the terminal UI's `Projected*` marker
  ([#1751](https://github.com/rshade/finfocus/issues/1751)).

## Related

- [Web UI reference](../reference/web-ui.md): flags, views, controls, and
  HTTP behavior
- [Web UI design](../architecture/web-ui.md): the security model and how the
  dashboard shares the CLI engine
- [overview command](../commands/overview.md): the terminal equivalent
- [CLI Commands Reference](../reference/cli-commands.md#root-command-entry-points)
