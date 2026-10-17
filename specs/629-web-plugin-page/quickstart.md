# Quickstart: Web Plugin Page

Confirm the Plugins page against a local dashboard. Unit and browser tests
are the gate. This guide is the manual pass.

## Preconditions

- `make build` has produced `bin/finfocus`.
- `GITHUB_TOKEN` is set when the live catalog is used. Without it, GitHub
  may rate-limit and the page must say so and mention `GITHUB_TOKEN`.
- A Pulumi project directory is optional. The global tier is always
  available. The project tier appears when a project directory resolves.

## Catalog

```bash
./bin/finfocus --web
```

Open the printed URL, then Plugins.

- Every built-in catalog entry is visible with its security level.
- `finfocus plugin list` and `finfocus plugin list --available` agree with
  the installed and latest versions on the cards.
- A plugin installed from outside the catalog is listed, and Install and
  Update are absent for it.

## Install, update, remove

1. Choose a catalog plugin and Install. The dialog names the plugin,
   version, source repository, and security level. Dismissing it leaves the
   plugin directory unchanged.
2. Confirm. Progress moves through resolve, download, verify, extract, and
   validate. `finfocus plugin list` then shows that version.
3. Update an older install the same way. Remove names the version and
   deletes only that version's directory.
4. `finfocus plugin install` text for the same plugin matches the text from
   before this branch. The page is not a way to pass `--skip-checksum`.

## Trust

- A URL or `owner/repo` string is rejected and nothing is downloaded.
- A release whose checksum does not match fails and is not installed.
- A release with no `checksums.txt` stops until a second confirmation.
- A release with no files names the fallback version and waits.

## Settings

1. Open settings for an installed plugin that advertises them (recorder,
   jev, prometheus, or kubernetes). Required fields and choice lists follow
   the plugin. Secrets show set or unset.
2. Save a non-secret in the project tier. Only that tier's `config.hujson`
   changes. Comments elsewhere stay.
3. Run a dashboard query that launches the plugin. The plugin's environment
   contains the saved value. A variable exported in the shell that launched
   `finfocus` still wins.
4. Reload settings. The saved secret is not in the page data.

## Validate

Validate a healthy install (pass) and an install whose binary was removed
(fail, with a reason). Conformance and certify are not on the page.

## Tests

```bash
go test ./internal/registry/ ./internal/pluginhost/ ./internal/config/ ./internal/webui/ ./internal/engine/
make test-frontend
make test-e2e-web
make check-plugin-boundaries
make validate
make test
make lint
```

The browser test installs the recorder from a stand-in catalog, sets
`FINFOCUS_RECORDER_MOCK_RESPONSE` through the form, runs a query, and checks
that the recorder received it.
