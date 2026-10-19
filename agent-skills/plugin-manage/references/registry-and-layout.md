# Registry, Layout, and Regions

## Directory layout

The base is `FINFOCUS_HOME`, else `PULUMI_HOME/finfocus`, else `~/.finfocus`.
The plugin directory is `FINFOCUS_PLUGIN_DIR`, else the `plugin_dir` config key,
else `<base>/plugins`.

```text
<base>/
├── config.hujson            # global config, installed_plugins list
└── plugins/
    ├── <name>.lock          # present only while install/update/remove runs
    └── <name>/<version>/
        ├── finfocus-plugin-<name>[-<region>]   # binary (.exe on Windows)
        ├── plugin.metadata.json                # user metadata, e.g. {"region": "us-west-2"}
        └── plugin.manifest.json                # optional, written by the plugin
```

A project can override the global config with
`$PROJECT/.finfocus/config.hujson`. A legacy `config.yaml` is still read and is
migrated to Hujson by the global loader on first read.

## Config entry

`installed_plugins` records what `install` saved (skipped with `--no-save`):

```json
{
  "installed_plugins": [
    { "name": "aws-public", "url": "github.com/rshade/finfocus-plugin-aws-public", "version": "v0.4.0" }
  ]
}
```

`update` reads this entry to find the source. `remove` deletes it unless
`--keep-config`.

## Manifest

`plugin.manifest.json` is flat: `name`, `version`, and optional fields.
`plugin validate` requires `name` and `version` to equal the install directory
names, so a plugin copied into the wrong directory fails validation.

## Embedded registry

`internal/registry/registry.json` (schema `1.0.0`) maps a short name to a
GitHub repository. Fields: `name`, `description`, `repository`,
`supported_providers`, `capabilities`, `security_level`, `asset_hints`
(`asset_prefix`, `version_prefix`), optional `tag_prefix`, `min_spec_version`.

| Name | Repository | Providers |
| --- | --- | --- |
| `aws-public` | `rshade/finfocus-plugin-aws-public` | aws |
| `azure-public` | `rshade/finfocus-plugin-azure-public` | azure |
| `opencost` | `rshade/finfocus-plugin-opencost` | kubernetes |
| `flexera` | `rshade/finfocus-plugin-flexera` | aws, azure, gcp |
| `vantage` | `rshade/finfocus-plugin-vantage` | aws, azure, gcp, kubernetes |
| `kubernetes` | `rshade/finfocus` (tag prefix `kubernetes-`) | kubernetes |
| `scorer` | `rshade/finfocus` (tag prefix `scorer-`) | any |
| `jev` | `rshade/finfocus` (tag prefix `jev-`, deprecated; use `scorer`) | any |
| `carbon` | `rshade/finfocus` (tag prefix `carbon-`) | aws |

The `scorer` plugin replaces legacy `jev` for new installations and supports
Jev or Clef backends. The `jev` catalog entry is deprecated: installing it
warns and still selects its separate legacy release. It does not migrate
configuration. Until scorer release assets are published, build scorer with
`make install-scorer`. See the scorer plugin README for the `SCORER_*`
settings and migration table.

Run `finfocus plugin list --available` for the live catalog; the table can lag a
release. Provider-to-plugin mapping and per-plugin credentials are in the
`finfocus-install` skill. Monorepo plugins (`tag_prefix`) resolve "latest" to the
highest prefixed stable version, not the repository's latest release.

## Installing a plugin that is not in the registry

```bash
finfocus plugin install github.com/owner/finfocus-plugin-x
finfocus plugin install github.com/owner/finfocus-plugin-x@v1.2.3
```

The release needs an archive for the host OS and architecture, and the binary
inside must be named `finfocus-plugin-<name>`. Publishing a `checksums.txt` with
the release enables verification.

## Regions

`aws-public` ships a multi-region router binary by default. To pin one region:

```bash
finfocus plugin install aws-public --metadata="region=us-west-2"
```

That prefers the release archive named
`<project>_<version>_<Os>_<arch>_us-west-2.<ext>`, falls back to the
unsuffixed archive when none exists, and writes `{"region": "us-west-2"}` to
`plugin.metadata.json`. On discovery, a plugin with
a `region` in metadata is looked up under the region-suffixed binary name first.
If metadata is absent, the region is parsed from the binary name (patterns such
as `us-east-1`, `eu-west-3`). For several regions, install each into its own
version directory or route by region (see `finfocus-routing`).

The router binary starts a child process per region. After a hard kill, check
for orphans: `ps aux | grep finfocus-plugin-aws-public`.
