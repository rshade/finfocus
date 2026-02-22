# Contracts: Batch Bug Fixes (599)

These fixes do not introduce new APIs, gRPC endpoints, or CLI commands.
They correct existing behavior. No contract changes are required.

## Behavioral Contracts (Corrected)

### Registry.ListPlugins()

**Before**: Silently skips `DirEntry` items where `IsDir() == false` (includes symlinks-to-dirs).
**After**: Uses `os.Stat(path).IsDir()` — follows symlinks at both directory levels.

### analyzer.Server.ConfigureStack()

**Before**: Calls `clearCostCache()` — may wipe costs from preceding `Analyze` calls.
**After**: Does NOT clear cost cache. Cache lifetime is managed by `AnalyzeStack`.

### analyzer.Server.AnalyzeStack()

**Before**: Reads cost cache (already cleared) → always returns 0 resources, $0.00 total.
**After**: Reads cost cache (populated by `Analyze` calls) → correct aggregate total.
         Clears cache at the END after emitting the summary.

### config.New() — PluginDir resolution

**Before**: `PluginDir` always = `filepath.Join(finfocusDir, "plugins")`.
**After**: Overridable via `plugin_dir:` config key, then `FINFOCUS_PLUGIN_DIR` env var.

### analyzer serve — Log output

**Before**: All logs → stderr (captured by Pulumi as Diagnostics).
**After**: When `FINFOCUS_ANALYZER_MODE=true`, all logs → `~/.finfocus/logs/analyzer.log`.
