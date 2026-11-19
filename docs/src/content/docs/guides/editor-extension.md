---
title: "See costs in your editor"
description: "Install the FinFocus VS Code extension to see each Pulumi resource's cost on the line that declares it, and know when those figures are out of date."
layout: "docs"
---

The FinFocus extension for VS Code and its forks shows what each Pulumi resource
costs on the line that declares it. Every figure comes from the `finfocus`
command line; the extension formats no money of its own. For setup and the list
of commands, see the package `README.md` in `packages/vscode-finfocus`.

## Install the extension

Download `vscode-finfocus-X.Y.Z.vsix` and `SHA256SUMS` from the
`vscode-finfocus-vX.Y.Z` release on GitHub, verify the hash, and install the
file in your editor:

| Editor | Command |
| --- | --- |
| VS Code | `code --install-extension vscode-finfocus-X.Y.Z.vsix` |
| Cursor | `cursor --install-extension vscode-finfocus-X.Y.Z.vsix` |
| Windsurf | `windsurf --install-extension vscode-finfocus-X.Y.Z.vsix` |
| VSCodium | `codium --install-extension vscode-finfocus-X.Y.Z.vsix` |

**Extensions: Install from VSIX...** in the Command Palette does the same
without a shell command. The extension needs an editor based on VS Code API
1.90 or newer, and `finfocus` 0.5.8 or newer on the machine that holds the
workspace.

### Remote workspaces

The extension runs where the workspace lives. In WSL, over SSH, or in a dev
container, install it into that remote host and make sure `finfocus` and
`pulumi` are on the remote `PATH`, or set `finfocus.path` and
`finfocus.pulumiPath` there. A tool that exists only on your local machine is
not used.

### Releases

A release attaches the `.vsix` and its `SHA256SUMS` to the GitHub release.
Nothing is published to a marketplace, and the extension does not update
itself: install the next release's file over the old one.

## Reloads and the result cache

Loading a stack's deployed state starts `finfocus`, which starts its plugins; on
a 65-resource stack that takes a few seconds. To avoid paying it on every window
reopen, the extension keeps the last deployed-state result and shows it again
while nothing it depends on has changed. A reused result paints annotations and
the side bar within about two seconds, with no `finfocus` process.

The stored result is `finfocus`'s own JSON, unchanged. It is reused only when
all of these still match:

- the stack's last update time and resource count, as `pulumi stack ls --json`
  reports them (a stack that was never updated, or is being updated, is never
  cached);
- the contents of the stack's `Pulumi.<stack>.yaml` (or `.yml`) and of the
  project's `.finfocus/config.hujson`;
- the `finfocus` version;
- the time limit, set by `finfocus.cache.ttlMinutes` (default 60; `0` turns the
  cache off). Month-to-date figures move without a stack update, so a stored
  result expires.

**FinFocus: Refresh costs** and a change to `finfocus.path` always run
`finfocus` and store the new result. A preview is never cached, and an
untrusted workspace neither reads nor writes the cache. Entries sit in the
extension's global storage on the machine where the workspace runs, shared by every window on that machine and readable
only by you on Linux and macOS; on Windows they rely on your user profile's
storage folder. A file that cannot be read is deleted and treated as a miss.

### Unpriced resources and incomplete totals

Many resources have no price (providers, route tables, command resources). The
JSON does not say whether a missing price is expected or a plugin failed or was
slow, so after a load that has unpriced resources the extension runs `finfocus`
once more when it has not seen those same resources unpriced before:

- **Same unpriced resources on both runs.** The result is shown and stored, and
  the card notes `N resources unpriced`.
- **Different unpriced resources.** A plugin was slow or failing. The card,
  status bar, and tree say `Incomplete: N of M resources unpriced`, the total is
  not presented as complete, and nothing is stored. **Refresh costs** tries
  again.

The first load of a stack with unpriced resources therefore costs one extra run.
Later loads compare against what the extension remembers. A plugin that fails
the same resources on both runs looks like one with no prices; only a completeness
field from `finfocus` itself could tell them apart. A resource whose pricing
failed with an error shows `ERR` and its message in the tree instead.

## Know when a figure is stale

Costs are loaded from your deployed state, so they describe what was last
deployed, not what you are typing. The extension marks a line when its figure
may no longer match the code. A marked line ends with `· stale` or
`· freshness unknown`, and the hover lists the reasons. The text carries the
state, so it does not depend on color.

Marking is cheap and never prices anything. Pricing a change is a separate,
explicit step.

| Reason | When it appears |
| --- | --- |
| `unsaved` | The declaring file has changes that are not saved. |
| `edited` | The declaring file changed after the figure was produced. |
| `config changed` | The stack settings file changed. Every row is marked. |
| `freshness unknown` | The extension cannot tell whether the file changed. |

### What the extension compares

- **Initial load.** The extension compares the declaring file's modification
  time with the time finfocus reports as the resource's last change
  (`modifiedAt`). A file that is not newer than that time is fresh. A newer file
  is `edited`.
- **Equal times are fresh.** Only a file strictly newer than `modifiedAt` is
  marked.
- **Unknown freshness.** A resource with no recorded location, no valid
  `modifiedAt`, or a declaring file that cannot be read has no timestamp to
  compare. It is shown as `freshness unknown`, never as fresh.
- **Unsaved changes always count.** A document that was already modified when
  the costs loaded is `unsaved` from the start.
- **Edits mark within a second.** Typing, saving, an external change, a rename,
  and a deletion mark every resource declared in that file. In testing the mark
  appeared in under 150 ms.

### File granularity

A mark applies to a whole file. If three resources are declared in one file and
you edit a different part of it, all three are marked. The extension does not
work out which declaration an edit touched, and a helper that creates several
resources from one line marks all of them together.

### Stack settings

Changes to `Pulumi.<stack>.yaml` or `Pulumi.<stack>.yml` for the selected stack
mark every row as `config changed`, including rows with no location, because
settings can change any price. The extension matches the last segment of a
qualified stack name (`org/project/stack` uses `Pulumi.stack.yaml`) and never
builds a path from the rest. Another stack's settings file does nothing.

### Refreshing

**FinFocus: Refresh costs** reloads the deployed state without using the stored
result (see [Reloads and the result cache](#reloads-and-the-result-cache)). A refresh does not
price your local changes, so it keeps every `edited`, `unsaved`, and
`config changed` mark. It does recompute the timestamp comparison, because the
deployed state may have changed. Only a preview that covers the edited
resources, with no further edits to them while it ran, clears their marks.

An edit made while a refresh is running stays marked when the result arrives.

## Price your changes with a preview

A figure marked stale describes deployed state. **FinFocus: Run preview** prices
your local code by running `finfocus overview` with a Pulumi preview, which
reads the files on disk.

### Unsaved files

Pulumi previews saved content only. When files in the project are unsaved, the
command asks:

- **Save all and run** saves them, then runs. If any save fails, nothing runs
  and the message names the file.
- **Run saved files** runs without saving. The resources in unsaved files stay
  marked `unsaved`, because the preview could not have priced them.
- **Cancel** runs nothing.

The extension captures what counts as "changed" only after the saves finish.

### Preview on save

Setting `finfocus.previewOnSave` runs a preview after each save in the project.
It is off by default because a preview takes time and may contact your cloud
provider. A save-triggered run prices only the file you just saved. It never
saves other files and never asks. A newer save replaces an older run that is
still going.

### What a preview clears

A successful preview clears the `edited` mark of a file when the file was saved
before the run started, was not edited again while it ran, and its resources
are in the result. An edit made during the run stays marked, even if you saved
it before the result arrived. Stack settings marks clear the same way. A failed
or cancelled preview changes no figure and no mark, and the CLI's message is
shown.

### Passphrases

If the stack's secrets need a passphrase, the extension asks for it in a masked
box. An empty passphrase is a valid answer. The passphrase is kept in memory for
that project and stack only, never written to disk or logs, and is forgotten
when the project is removed or the window closes. If finfocus rejects it, the
box opens again with a note; pressing Escape cancels the run and keeps your
previous figures. There is no automatic retry.

### Cancelling and cleanup

Cancel a running preview from the progress notification or with **FinFocus:
Cancel preview**. Cancelling, switching stack or project, removing the folder,
and closing the window all stop the run the same way:

1. finfocus gets SIGTERM and five seconds to close its plugins and Pulumi.
2. Anything still running in finfocus's session is then killed, and the
   extension waits up to two more seconds to confirm none is left. This also
   runs after a normal exit, so a process left behind is swept.
3. On Windows the extension ends finfocus's whole process tree with `taskkill`
   and finfocus holds a job object that ends its plugins.

Only processes the extension started are touched, never one chosen by name. If
cleanup cannot be confirmed, the extension reports which process IDs are still
running and starts no new run for that stack until you retry or reload the
window.

## See the stack in the side bar

The FinFocus icon in the activity bar opens two views and adds one status-bar
item. All figures are the text finfocus printed for the same stack; the
extension adds no money arithmetic.

- **Summary card.** Stack name, projected monthly total, month-to-date actual,
  delta, budget, how many resources are stale or of unknown freshness, the five
  most expensive resources, and **Run preview**. The five are ordered by the
  numeric cost finfocus reports, and unpriced resources are left out of them.
  A result that may understate the total shows `Incomplete: N of M resources
  unpriced` beside the figures, with an explanation you can reach by keyboard.
- **Resources tree.** Every resource, grouped by type, with an **Unpriced**
  group last. A resource that finfocus could not price shows its error in its
  details.
- **Status bar.** The monthly total and budget state, plus `incomplete` when
  the result is incomplete. Clicking it opens the card.

### Budget

With a configured budget the card reads, for example, `Budget WARNING: $80.00
of $100.00 (80%) · Monthly budget`. The state is always written out, never
shown by color alone. Without one it says `No budget configured`.

### Keyboard and screen reader use

Every resource row has a full accessible label with its figures, declaration
and stale reason. In the tree, **Enter** opens the declaration and **Alt+Enter**
(or **FinFocus: Show resource details**) lists every detail line: name, type,
projected/actual/delta/status, `Declared at file:line` or `No recorded
location`, the stale reason, and the error. In the card, a top-five button
opens the declaration, and **F2** or **Shift+Enter** on it shows the details.

### Forks and a card that will not render

The card is a webview. If an editor fork cannot show it, the extension logs the
reason and the card shows a short notice. The Resources tree and the status-bar
item read the same result without the webview, so they keep working.

## Troubleshooting

The extension stays silent in a folder with no `Pulumi.yaml` or `Pulumi.yml`
(a file under `node_modules`, `.git`, `vendor`, `dist`, `build`, `out`, or
`target` does not count), and it starts nothing until the workspace is trusted.
Every message below also appears in the **FinFocus** output channel, with
credentials and full command lines left out.

| Message | Cause and fix |
| --- | --- |
| `finfocus was not found` | The executable is not on `PATH`. Install it and reload, or set `finfocus.path`. The extension does not retry until a setting changes. |
| `FinFocus X or newer is required; found Y` | Upgrade finfocus, or point `finfocus.path` at a newer one. |
| `Could not read the finfocus version ...; continuing` | A development or unrecognized build. It is allowed and only logged. |
| `Could not run pulumi` | Install Pulumi or set `finfocus.pulumiPath`. It is used only to list stacks. |
| `This Pulumi project has no stacks` | Run `pulumi stack init` in the project. |
| A message that mentions plugins | The CLI could not use a plugin. Run `finfocus plugin list`, then `finfocus plugin install <name>`. |
| A passphrase prompt that keeps returning | The passphrase was rejected. Enter the stack's passphrase, or an empty one if the stack has none. |
| A backend or credential error | Log in with `pulumi login` and check the backend credentials. The extension passes the CLI's reason through unchanged. |

Changing `finfocus.path` or `finfocus.pulumiPath` allows one new attempt and
reloads costs.
