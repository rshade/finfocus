# FinFocus for VS Code

Shows what each Pulumi resource costs on the line that declares it. Every
figure is text produced by the `finfocus` CLI; the extension formats no money.

## Install

The extension ships as one `.vsix` file attached to each `vscode-finfocus-vX.Y.Z`
GitHub release, with a `SHA256SUMS` file. It is not on a marketplace yet. Check
the hash, then install the same file in any editor that supports VS Code
extensions:

```bash
sha256sum --check --ignore-missing SHA256SUMS
code --install-extension vscode-finfocus-X.Y.Z.vsix
cursor --install-extension vscode-finfocus-X.Y.Z.vsix
windsurf --install-extension vscode-finfocus-X.Y.Z.vsix
codium --install-extension vscode-finfocus-X.Y.Z.vsix
```

In an editor's window, **Extensions: Install from VSIX...** does the same.

- **Minimum editor**: the editor must report VS Code API 1.90 or newer (Help >
  About shows the VS Code version it is based on). Older editors refuse the
  install. Cursor, Windsurf, and VSCodium are expected to qualify; the
  per-editor results are recorded in `specs/2017-vscode-cost-extension/validation.md`
  as they are verified.
- **Remote hosts** (WSL, SSH, dev containers): the extension runs on the
  workspace host, so install it into that host (the Extensions view offers
  **Install in WSL**, **Install in SSH**, or the container) and put `finfocus`
  and `pulumi` on that host, not only on your local machine.
- **Release policy**: a release publishes the file and its hash only. The
  extension does not update itself; install the next release's file over the
  old one.

## Requirements

- `finfocus` 0.5.8 or newer, on `PATH` or set in `finfocus.path`.
- `pulumi` on `PATH` or set in `finfocus.pulumiPath`. It is used only to list
  stacks (`pulumi stack ls --json`); the extension never selects or creates one.
- A trusted workspace. In an untrusted workspace no executable is started.

## Setup

1. Open a folder that contains a `Pulumi.yaml` or `Pulumi.yml`. A workspace with
   no Pulumi project stays silent and starts nothing.
2. The extension picks the project nearest the active file, the one you chose,
   or the only one; with several candidates it asks. **FinFocus: Select Pulumi
   project** changes the choice.
3. It uses the stack Pulumi marks `current`, or asks. **FinFocus: Select Pulumi
   stack** picks another for this session; it does not run `pulumi stack select`.
4. It loads the deployed state with `finfocus overview --state-only`, which
   prices nothing new, and annotates the declarations. **FinFocus: Refresh
   costs** reloads it.

## What is annotated

Each priced declaration shows the CLI's projected figure at the end of its line.
Hover for the resource name, projected, actual, delta, and status.

- **Recorded positions.** The line is the position Pulumi recorded when the
  resource was created, taken from the innermost stack frame inside the project.
  Dependencies under `node_modules` or `site-packages` are skipped. Node.js,
  Python, and .NET record a column; Go records none. YAML programs record no
  project frames, so they get no annotations.
- **Shared lines.** Resources created by one helper or loop share a line. It
  shows `N resources`, and the hover lists each resource's own figures. They are
  never added together.
- **Unpriced resources.** A resource finfocus cannot price shows the CLI's `-`;
  it never gets an invented figure.
- **Missing locations.** A resource with no recorded position, or one whose path
  is absolute or leaves the project, gets no annotation.
- **Edits.** Annotations follow lines as you insert and delete text. They show
  the figures from the last load until you refresh.

## Reloads and the result cache

Opening a window reuses the last deployed-state result while the stack's last
update, its `Pulumi.<stack>.yaml`, the project's `.finfocus/config.hujson`, and
the finfocus version are unchanged and the result is younger than
`finfocus.cache.ttlMinutes` (default 60; `0` turns the cache off). A reused
result paints within about two seconds and starts no finfocus process.
**Refresh costs** always runs finfocus; a preview is never cached; an untrusted
workspace never reads the cache.

When a load has unpriced resources the extension runs finfocus once more to see
whether the same ones stay unpriced. If they differ (a slow plugin), the card,
status bar, and tree read `Incomplete: N of M resources unpriced` and the result
is not stored. The same ones both times read `N resources unpriced`. The guide
explains the limits.

## Stale figures

Annotations describe your deployed state. When a figure may no longer match the
code, the line ends with `· stale`, or `· freshness unknown` when the extension
cannot tell, and the hover lists the reasons.

- **File granularity.** Any edit to a file marks every resource declared in it.
  The extension does not locate the edited declaration.
- **Reasons.** `unsaved` (changes not saved), `edited` (the file changed after
  the figure was produced), `config changed` (the stack's `Pulumi.<stack>.yaml`
  or `.yml` changed; every row is marked), and `freshness unknown`.
- **Timestamps.** On load, a file newer than the resource's `modifiedAt` is
  `edited`; an equal or older file is fresh. A missing or invalid `modifiedAt`,
  an unreadable file, or a resource with no location is `freshness unknown`,
  never fresh.
- **Refresh.** Refreshing reloads deployed state and keeps edit and config
  marks, since it prices nothing you changed. Marks clear only when a preview
  covers the resources and nothing edited them while it ran.
- **No pricing.** Marking never starts `finfocus` or `pulumi preview`.

## Preview your changes

**FinFocus: Run preview** prices local code with a Pulumi preview, which reads
saved files only.

- **Unsaved files.** The command offers **Save all and run**, **Run saved
  files**, or **Cancel**. A failed save aborts the run. After **Run saved
  files**, resources in unsaved files stay `unsaved`.
- **On save.** `finfocus.previewOnSave` (off by default) previews after each
  save. It prices the saved file and saves nothing else.
- **Marks.** A preview clears a file's mark only if the file was saved before
  the run and not edited during it. A failed or cancelled run keeps every figure
  and mark.
- **Passphrase.** If needed, a masked box asks for it. An empty value is valid.
  It is kept in memory for that stack only and forgotten on removal or close.
  Escape cancels the run; a rejected value asks again, never automatically.
- **Cancel.** The progress notification or **FinFocus: Cancel preview** stops
  the run. finfocus gets SIGTERM and 5 s; then its whole session is killed and
  confirmed within 2 s (Windows: `taskkill /T`). Selection change, folder
  removal, and window close do the same. If cleanup cannot be confirmed, the
  extension names the process IDs and starts nothing for that stack until you
  retry or reload.

## Side bar

The FinFocus activity-bar icon holds a **Summary** card and a **Resources**
tree, and the status bar shows the monthly total with the budget state.

- **Card.** Stack, projected monthly, month to date, delta, budget, stale or
  unknown count, the five most expensive resources (by the numeric cost
  finfocus reports), and **Run preview**. No budget reads `No budget
  configured`; a budget reads `Budget <HEALTH>: <amounts>` in words, not color.
- **Tree.** Every resource under its type, with an **Unpriced** group last and
  row errors in the details. Enter opens the declaration; **Alt+Enter** or
  **FinFocus: Show resource details** lists every hover field, the location,
  the stale reason, and the error. A resource with no recorded location says so.
- **Status bar.** Click it to open the card. It adds `incomplete` when the
  result may understate the total.
- **Forks.** If the card's webview cannot render, the tree and status bar still
  show the stack; the card shows a notice and the reason goes to the FinFocus
  output channel.

Figures are the CLI's display strings; the extension sums and reformats nothing.

## Troubleshooting

A missing or too old `finfocus`, a missing `pulumi`, a project with no stacks,
and plugin problems each produce one message with the fix, and a missing
binary is not retried until `finfocus.path` or `finfocus.pulumiPath` changes.
The guide lists every message at
`docs/src/content/docs/guides/editor-extension.md`.
