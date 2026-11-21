# Plugin naming

The install name is the plugin's identity. `finfocus plugin list` shows it,
users type it in routing rules and `--adapter`, and plugin settings and ESC
values are delivered under it. A plugin that reports a different name still
works, but core warns, and the Adapter column shows a name the user never
typed.

## One constant

```go
// PluginName is the plugin's install name.
const PluginName = "acme-cost"
```

Use it in every place the plugin states its name:

| Place | Use |
| --- | --- |
| Name RPC | `pluginsdk.NewBasePlugin(PluginName)`, or `Name()` returning `PluginName` |
| Plugin info | `Name: PluginName` in `GetPluginInfo` or `pluginsdk.NewPluginInfo(PluginName, ...)` |
| Logger | `pluginsdk.NewPluginLogger(PluginName, version, level, w)` |
| Results | `Source: PluginName` on recommendations, budgets, and cost rows |
| Cache paths | `filepath.Join(cacheRoot, PluginName)` |

A plugin with several binaries (one per region) reports `PluginName` from
each of them, not the binary's file name.

## Find stray copies

```bash
# Every string literal that spells the name, outside tests
grep -rn '"acme-cost"' --include=*.go . | grep -v _test.go

# Names that carry the binary prefix
grep -rn '"finfocus-plugin-' --include=*.go . | grep -v _test.go

# Name() implementations that return a literal
grep -rn -A2 'Name() string' --include=*.go . | grep 'return "'
```

The first command should print one line: the constant.

## The conformance checks

Run them against a built binary:

```bash
finfocus plugin conformance --category protocol ./bin/finfocus-plugin-acme-cost \
  --expected-name acme-cost
```

Without `--expected-name`, the expected name comes from the
`<plugins>/<name>/<version>/` directory the binary sits in, or else from its
`finfocus-plugin-<name>` file name (minus a region suffix that
`plugin.metadata.json` declares).

| Check | Warns when | Fix |
| --- | --- | --- |
| `Name_MatchesInstallName` | `Name()` differs from the install name | Return `PluginName` from `Name()` |
| `Name_MatchesPluginInfo` | `GetPluginInfo` reports another name, or is not implemented | Set the plugin info `Name` from `PluginName` |
| `Name_IsCanonicalForm` | The name has uppercase letters, doubled hyphens, or the `finfocus-plugin-` prefix | Use the bare install name in lowercase |

A warning never fails the run, changes the exit code, or blocks
`finfocus plugin certify`. A check that cannot run (no expected name, or no
`GetPluginInfo`) also warns and says what is missing.

## How core treats a mismatch

Core matches a user-supplied plugin name (a routing rule, `--adapter`,
`--usage-source`, `scoring.plugin`) against the install name first, then any
former names the finfocus plugin catalog lists for the plugin, then the name
the plugin reports. When the reported name differs from the install name,
core logs one warning per process.

## Renaming a published plugin

Changing what `Name()` returns breaks every routing rule and `--adapter`
value written with the old name. Before releasing the rename:

1. Open a finfocus PR that adds the old name to the plugin's catalog entry
   in `internal/registry/registry.json`:
   `"aliases": ["finfocus-plugin-acme-cost"]`.
2. Release the plugin after a finfocus release carries that alias.
3. Say in the plugin's release notes that the old name keeps working and
   that `finfocus config validate` now suggests the new one.
