# Quickstart: Verify Cost in the Editor

## 1. Core output

```bash
make build
cd testdata/sourcepos/nodejs/program   # the Pulumi project the recording came from
../../../../bin/finfocus overview --output json --yes \
  --pulumi-json ../preview.json > ../overview.json
jq '(.resources[] | {urn, location, display}),
    .summary.display, .budgetDisplay' ../overview.json
jq '[.resources[].location.file // empty
     | select(startswith("/") or contains(".."))] | length' \
  ../overview.json   # 0
```

Run it from the project directory: the location resolver needs the Pulumi
project root, and an explicit `--pulumi-state` file supplies none, so that form
prints no locations. The saved preview has no state beside it, so the output
holds the preview's resources only (a warning says so).

Expect `location.file` values such as `lib/box.js`, no
absolute or escaping path, and `display` strings equal to the table output of
the same command with `--output table`.

## 2. Extension unit tests (no install)

```bash
node --test "packages/vscode-finfocus/test/unit/*.test.js"
```

## 3. Extension in an editor

```bash
cd packages/vscode-finfocus
pnpm install
pnpm run package              # builds vscode-finfocus-<version>.vsix
# if the pnpm shim fails: node_modules/.bin/vsce package --no-dependencies
code --install-extension vscode-finfocus-*.vsix   # or cursor / windsurf / codium
```

Open the deployed Node.js source-location demo, then verify Go, Python, and
.NET projects with the runtime fixtures in validation.md (a throwaway
local-backend project with `command:local:Command` resources needs no cloud
credentials; see research R1):

1. Annotations appear at each resource line; the FinFocus side bar shows the
   card, the tree, and the status-bar total.
2. Edit a declaration and save: its annotation shows `⟳ stale` within one
   second; no finfocus process starts (`ps`).
3. Run **FinFocus: Run preview**: progress shows, then new figures and deltas;
   stale marks clear only for clean files covered without later edits.
4. Mark the folder untrusted: no finfocus process runs.
5. Reload the window with the stack unchanged: annotations and the card return
   within about two seconds and no `finfocus overview` process starts (`ps`);
   **FinFocus: Refresh costs** starts one. Setting `finfocus.cache.ttlMinutes`
   to `0` makes every reload run finfocus.

## 4. Repository gates

```bash
make lint && make test
```

## 5. Acceptance evidence

Follow [validation.md](validation.md) for the full editor/OS, runtime,
accessibility, performance, cancellation, and freshness checks. The commands
above are a quick smoke test and do not substitute for the release matrix.
The source-location fixtures (`testdata/sourcepos/`) and the extension package
(`packages/vscode-finfocus`) exist; the release matrix remains in validation.md.
