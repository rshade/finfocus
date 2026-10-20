---
name: plugin-manage
description: >
  Manage the FinFocus plugin lifecycle: discover, install, update, remove,
  validate, inspect, and troubleshoot installed plugins. Use when installing or
  updating a pricing plugin, installing a plugin from a GitHub URL, choosing a
  region-specific binary, fixing a plugin that will not start, a stale lock, a
  checksum or "no asset found" failure, or checking why a plugin is not queried.
  Triggers on: "install plugin", "update plugin", "remove plugin", "plugin list",
  "plugin validate", "plugin inspect", "plugin not found", "plugin timeout",
  "plugin lock", "region plugin", "finfocus plugin", "which plugins are installed",
  or any task involving managing installed finfocus plugins. For first-time
  environment setup use finfocus-install; for routing rules use finfocus-routing.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus Plugin Management

Drive `finfocus plugin *` for the user. Read state first, preview with
`--dry-run` before changing it, and verify after.

## Workflow

1. **Inspect state**: `finfocus plugin list --output json` (installed) and
   `finfocus plugin list --available` (embedded registry, no plugin launch).
2. **Change state**: install, update, or remove (below). Preview with the global
   `--dry-run` first. It prints what would happen and changes nothing.
3. **Verify**: `finfocus plugin validate`, then `finfocus plugin list --verbose`.
4. **Check routing** if a plugin installs but is never queried:
   `finfocus config routes test <resource-type> [region]`.

Never pass `--skip-checksum` or `--force` on a first attempt. Use them only after
the diagnosis in [references/troubleshooting.md](references/troubleshooting.md)
points there, and tell the user why.

## Discover

```bash
finfocus plugin list                  # table
finfocus plugin list --verbose        # capabilities, spec version, providers
finfocus plugin list --output json    # array of name, version, path, specVersion,
                                      # runtimeVersion, supportedProviders,
                                      # capabilities, notes
finfocus plugin list --available      # registry catalog
```

`list` launches each installed plugin (5 s timeout each, concurrently) to read
runtime metadata. A plugin that cannot start stays in the output with
`notes: "Failed: <error>"`, so read `notes` rather than assuming it is absent.

Official registry plugins: `aws-public`, `aws-ce`, `azure-public`, `opencost`, `flexera`,
`vantage`, `kubernetes`, `scorer`, `carbon`, and deprecated `jev`. Details and provider mapping:
[references/registry-and-layout.md](references/registry-and-layout.md).

## Install

```bash
finfocus plugin install aws-public                         # registry, latest
finfocus plugin install aws-public@v0.4.0                  # pinned
finfocus plugin install github.com/owner/finfocus-plugin-x # not in the registry
finfocus plugin install github.com/owner/repo@v1.2.3
finfocus plugin install aws-public --metadata="region=us-west-2"
```

| Flag | Effect |
| --- | --- |
| `-f, --force` | Reinstall an existing version |
| `--no-save` | Do not record the plugin in the config file |
| `--clean` | Remove other installed versions afterwards |
| `--plugin-dir` | Override the plugin directory |
| `--metadata k=v` | Repeatable; written to `plugin.metadata.json` |
| `--skip-checksum` | Skip SHA256 verification |
| `--fallback-to-latest` | If the version lacks a binary for this platform, install the latest stable that has one |
| `--no-fallback` | Fail instead of falling back |

`--fallback-to-latest` and `--no-fallback` are mutually exclusive (Cobra rejects
both). Without either, an interactive run asks (default No) and a
non-interactive run fails, so use `--fallback-to-latest` in CI.

A registry name that is not in the catalog fails with `plugin "x" not found in
registry`; install by GitHub URL instead. Checksums: only a confirmed hash
mismatch is fatal. A missing `checksums.txt` only warns.

Region-specific binaries and the multi-region router binary:
[references/registry-and-layout.md](references/registry-and-layout.md#regions).

## Update

```bash
finfocus plugin update aws-public --dry-run   # preview
finfocus plugin update aws-public             # latest
finfocus plugin update aws-public --version v0.5.0
```

The plugin must already be installed (`plugin "x" is not installed` otherwise).
It resolves the source from the config entry, installs the new version, removes
the old version directory, and updates the config. To update a plugin's *source
code* to a newer finfocus-spec, that is `finfocus plugin upgrade`, not `update`.

## Remove

```bash
finfocus plugin remove aws-public               # aliases: uninstall, rm
finfocus plugin remove aws-public --keep-config # keep the config entry
```

Deletes the version directories (and the plugin directory once empty) and the
config entry. If the plugin is not in the config it falls back to what is on
disk.

## Validate and inspect

```bash
finfocus plugin validate [--plugin <name>]
finfocus plugin inspect aws-public aws:ec2/instance:Instance [--version v0.1.0] [--json]
```

`validate` checks the binary exists, is not a directory, is executable (a `.exe`
on Windows), and, when `plugin.manifest.json` exists, that its name and version
match the install directory. It does not start the plugin; `list` does.

`inspect` launches the plugin, calls `DryRun`, and prints field mappings per
resource type (SUPPORTED, UNSUPPORTED, CONDITIONAL, DYNAMIC). A plugin without
`DryRun` fails with `does not support inspection`.

## Author-side commands

For plugin developers, not for managing installed plugins:

```bash
finfocus plugin conformance ./path/to/plugin [--mode tcp|stdio] \
  [--category protocol,error,performance,context] [--output table|json|junit] \
  [--output-file report.xml] [--filter <regex>] [--timeout 5m]
finfocus plugin certify ./path/to/plugin [--mode tcp|stdio] [-o report.md]
finfocus plugin init my-plugin --author "Name" --providers aws
finfocus plugin upgrade --dry-run
```

`certify` needs 100% of conformance tests to pass. `init` takes the name as a
positional argument and `--author` is required. Implementation help lives in the
`finfocus-plugin-dev` and `finfocus-plugin-upgrade` skills.

## Routing relationship

With no `routing:` section, every installed plugin is queried. Installing a
plugin does not create a route. With routing configured, a plugin that is not
listed is not used for the features and patterns the rules cover. `$0.00` is a
valid result and does not fall back; only an empty result or an error does.
Higher priority number wins.

```bash
finfocus config routes list --output json
finfocus config routes test aws:ec2:Instance us-east-1
```

Rule syntax and scenarios: the `finfocus-routing` skill.

## Environment variables

| Variable | Purpose |
| --- | --- |
| `FINFOCUS_HOME` | Base directory (default `~/.finfocus`) |
| `FINFOCUS_PLUGIN_DIR` | Plugin directory; beats the `plugin_dir` config key |
| `FINFOCUS_PROJECT_DIR` | Project config directory |
| `GITHUB_TOKEN` | GitHub API auth; avoids the unauthenticated rate limit |
| `FINFOCUS_STRICT_COMPATIBILITY` | `true` makes a spec-version mismatch fail the load (default: warn) |
| `FINFOCUS_LOG_LEVEL`, `FINFOCUS_LOG_FORMAT` | Logging (`--debug` wins) |
| `FINFOCUS_TRACE_ID` | Inject a trace id |

Core never sets `PORT`. Plugins read `--port` or `FINFOCUS_PLUGIN_PORT`. Never
print or log a plugin's credentials; they live in the user's environment.

## Troubleshooting

Map the exact error text to a recovery in
[references/troubleshooting.md](references/troubleshooting.md): stale lock,
`no asset found`, `integrity check failed`, rate limit, `timeout connecting to
plugin`, spec-version incompatibility, `does not support inspection`.

## References

- [references/registry-and-layout.md](references/registry-and-layout.md) —
  directory layout, file formats, registry entries, regions, URL installs
- [references/troubleshooting.md](references/troubleshooting.md) — error text to
  cause and recovery
