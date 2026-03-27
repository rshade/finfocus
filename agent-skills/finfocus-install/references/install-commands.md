# Install Commands Reference

## finfocus setup

Full environment bootstrap (idempotent).

| Flag | Description |
|------|-------------|
| `--non-interactive` | Disable TTY output (status symbols, color) |
| `--skip-analyzer` | Skip Pulumi analyzer installation |
| `--skip-plugins` | Skip default plugin installation |

Steps: version display → Pulumi detection → directory creation (base 0700,
plugins 0750) → config init → analyzer install → plugin install (`aws-public`).

## finfocus plugin install

```bash
finfocus plugin install <specifier> [flags]
```

**Specifier formats:**

- `kubecost` — registry name (latest)
- `kubecost@v1.0.0` — registry with version
- `github.com/owner/repo` — GitHub URL
- `github.com/owner/repo@v1.0.0` — GitHub URL with version

**Flags:**

| Flag | Description |
|------|-------------|
| `-f, --force` | Reinstall even if version exists |
| `--no-save` | Don't add to config.yaml |
| `--clean` | Remove other versions after install |
| `--plugin-dir` | Custom plugin directory (default: `~/.finfocus/plugins`) |
| `--fallback-to-latest` | Auto-fallback if requested version lacks assets |
| `--no-fallback` | Fail if version lacks assets (mutually exclusive with above) |
| `--metadata` | Repeatable `key=value` pairs (e.g., `--metadata="region=us-west-2"`) |
| `--skip-checksum` | Skip SHA256 verification |

**Checksum behavior:** Only a confirmed hash mismatch is fatal. Missing
`checksums.txt` or download failures produce warnings and continue.

## finfocus plugin update

```bash
finfocus plugin update <name> [flags]
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would change |
| `--version` | Target specific version (default: latest) |
| `--plugin-dir` | Custom plugin directory |
| `--skip-checksum` | Skip SHA256 verification |

## finfocus plugin remove

```bash
finfocus plugin remove <name> [flags]
```

Aliases: `uninstall`, `rm`.

| Flag | Description |
|------|-------------|
| `--keep-config` | Retain entry in config.yaml |
| `--plugin-dir` | Custom plugin directory |

## finfocus plugin list

```bash
finfocus plugin list [flags]
```

| Flag | Description |
|------|-------------|
| `--verbose` | Show capabilities, spec version, providers |
| `--output json` | JSON array for machine consumption |
| `--available` | List registry plugins (no plugin launch) |

## finfocus plugin validate

```bash
finfocus plugin validate [--plugin <name>]
```

Checks: binary exists, not a directory, executable bit set, manifest matches.

## finfocus config init

```bash
finfocus config init [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing config |
| `--global` | Force global config even inside Pulumi project |

**Behavior:** Inside Pulumi project → creates `$PROJECT/.finfocus/config.yaml`
with `.gitignore`. Outside or with `--global` → creates `~/.finfocus/config.yaml`.

## finfocus analyzer install

```bash
finfocus analyzer install [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing installation |
| `--target-dir` | Override Pulumi plugin directory |

**Plugin dir precedence:** `--target-dir` > `$PULUMI_HOME/plugins/` >
`~/.pulumi/plugins/`.

Creates: `analyzer-finfocus-v{VERSION}/pulumi-analyzer-finfocus` (symlink on
Unix, copy on Windows).

## finfocus analyzer check

```bash
finfocus analyzer check [--output json]
```

Runs 4 sequential checks: policy_pack_dir → pulumi_policy_yaml →
binary_in_path → grpc_smoke_test.

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `FINFOCUS_HOME` | Override base directory (default: `~/.finfocus/`) |
| `FINFOCUS_VERSION` | Pin version for install script |
| `FINFOCUS_INSTALL_DIR` | Custom install dir for install script |
| `FINFOCUS_NO_VERIFY` | Skip checksum in install script |
| `FINFOCUS_PLUGIN_DIR` | Override plugin directory |
| `FINFOCUS_PROJECT_DIR` | Override project config directory |
| `PULUMI_HOME` | Override Pulumi home (affects analyzer plugin dir) |
| `GITHUB_TOKEN` | GitHub API auth for plugin downloads (rate limits) |
