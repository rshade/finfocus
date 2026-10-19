# Plugin Troubleshooting

Start with `finfocus plugin validate`, then `finfocus plugin list --output json`
and read each `notes`. Re-run the failing command with `--debug`.

## Install and update

| Error text | Cause | Recovery |
| --- | --- | --- |
| `plugin "X" is currently being modified by another process (lock file ... exists)` | Another install/update/remove is running, or a lock is left behind | Wait and retry. On Linux and macOS a lock whose PID is dead is removed automatically; if it persists, read the PID in `<plugin dir>/X.lock`, confirm `ps -p <pid>` shows nothing, then delete the file. On Windows the OS releases the lock when the process exits, so a leftover file is harmless |
| `plugin "X" not found in registry` | Not an official plugin, or a typo | `finfocus plugin list --available`; otherwise install by `github.com/owner/repo` |
| `plugin X@vN already installed. Use --force to reinstall` | Version present | `--force`, or `plugin update` for a newer one |
| `plugin "X" is not installed` (update/remove) | Nothing under that name | `plugin list`; use `install` |
| `no asset found for <os>/<arch>` / `no compatible asset found` | The release has no archive for this platform, or its upload has not finished | Run with `--fallback-to-latest`. A latest release with no assets yet is replaced by the newest earlier one that has assets; an explicit version is never swapped. As a last resort build from source |
| `integrity check failed` | Downloaded archive does not match `checksums.txt` | Retry once (corrupt download). If it repeats, do not bypass silently: tell the user the artifact may be tampered with, and use `--skip-checksum` only on their explicit say-so |
| `GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits` | Unauthenticated GitHub API | `export GITHUB_TOKEN=...` (for example `$(gh auth token)`) and retry |
| `failed to get release` | Network failure, wrong repo or tag | Check connectivity, the repository path, and that the tag exists |
| `if any flags in the group [fallback-to-latest no-fallback] are set none of the others can be` | Both fallback flags passed | Choose one |

## Runtime

| Symptom | Cause | Recovery |
| --- | --- | --- |
| `timeout connecting to plugin` | Plugin did not bind its port in time, or crashed at start | Run the binary directly to see stderr; confirm it is executable (`plugin validate`); retry with `--debug` |
| `connection refused` / `EOF` in `list` notes | Plugin exited during startup (missing credentials or config) | Check the plugin's required environment variables in its docs |
| `context deadline exceeded` | Plugin is slow or hung | `--debug`; if it is a network-backed plugin, check the endpoint. `list` waits 5 s per plugin |
| `plugin spec version incompatible with core` | Major spec version mismatch, with strict mode on | Update the plugin (`plugin update`) or core. Strict mode is `FINFOCUS_STRICT_COMPATIBILITY=true` or the plugin host config; unset it to downgrade to a warning. `--skip-version-check` skips the check for one command |
| `does not support inspection` | Plugin has no `DryRun` | Update the plugin |
| Plugin installed but never queried | Routing config excludes it, or no provider match | `finfocus config routes list`, then `config routes test <type> [region]` |
| Plugin shows `$0` for everything | Plugin does not price that type, or a missing SKU/region | Not a management fault; use the `finfocus-diagnose` skill |
| `validate`: `manifest name mismatch` / `manifest version mismatch` | Manifest disagrees with the directory | Reinstall with `--force`, or fix the directory names |
| `validate`: `plugin binary is not executable` | Lost executable bit | `chmod +x` the binary, or reinstall |

## Safe recovery order

1. `finfocus plugin validate --plugin <name>`
2. `finfocus plugin update <name> --dry-run`, then without `--dry-run`
3. `finfocus plugin install <name> --force`
4. `finfocus plugin remove <name>` then a clean `install`

Stop and ask before deleting anything outside `<plugin dir>/<name>/` and its
lock file.
