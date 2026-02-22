# Research: Batch Bug Fixes (599)

**Date**: 2026-02-21
**Branch**: `599-batch-bug-fixes`

## Finding 1 — Registry Symlink Detection (#750)

**Decision**: Replace `DirEntry.IsDir()` with `os.Stat(path).IsDir()` at both the
plugin-name level (line 52) and the version level (line 63) of `ListPlugins()` in
`internal/registry/registry.go`.

**Rationale**: `DirEntry.IsDir()` reads the symlink's own type (`ModeSymlink`), not
the target's type. `os.Stat()` follows symlinks and returns the target's `FileInfo`,
so `IsDir()` returns `true` for symlinks-to-directories.

**Confirmed by**: `finfocus-demo/scripts/run-analyzer-preview.sh` contains a 9-line
comment block explaining exactly this limitation and showing the complex per-file
symlink workaround currently required.

**Broken symlink guard**: Wrap the `os.Stat` call and `continue` on any error (covers
broken symlinks and permission errors without crashing).

**Alternatives considered**:

- `os.Lstat` — reads symlink itself, not suitable.
- `filepath.EvalSymlinks` — overkill; `os.Stat` sufficient.

---

## Finding 2 — Analyzer Install Double-v (#749)

**Decision**: Change `analyzerDirPrefix` from `"analyzer-finfocus-v"` to
`"analyzer-finfocus-"` and normalize the version in `Install()`:

```go
ver := version.GetVersion()
if !strings.HasPrefix(ver, "v") {
    ver = "v" + ver
}
// Directory: "analyzer-finfocus-" + "v0.3.1" = "analyzer-finfocus-v0.3.1" ✓
```

**Rationale**: Production builds embed a `v`-prefixed version string (e.g.,
`v0.3.1`). The old prefix `"analyzer-finfocus-v"` + `"v0.3.1"` = double-v. The
new approach always guarantees exactly one `v` for both `"v0.3.1"` and `"0.3.1-dirty"`.

**Affected locations in `internal/analyzer/install.go`**:

- Line 18: constant definition
- Lines 99, 121–122: prefix-based directory scan (`HasPrefix`/`TrimPrefix`) — no
  change needed here since the prefix itself is the match key
- Lines 177, 211: version concatenation — apply normalization here

**Alternatives considered**:

- `strings.TrimPrefix(version.GetVersion(), "v")` at each call site — more invasive,
  easier to miss a call site.

---

## Finding 3 — AnalyzeStack Zero Cost (#746)

**Decision**: Move `s.clearCostCache()` from `ConfigureStack` to the end of
`AnalyzeStack` (after building and emitting the summary). Also verify
`BuildCostSummary` filter does not exclude valid results.

**Root cause confirmed**: `ConfigureStack` (line 435 of `server.go`) calls
`clearCostCache()`. Pulumi's `ConfigureStack` call may arrive after `Analyze` calls
complete (the order is not guaranteed by the protocol). Clearing there wipes all
cached costs before `AnalyzeStack` reads them.

**Secondary cause**: `BuildCostSummary` (`summary.go` lines 80–95) skips resources
where `c.Error != nil || isErrorNote(c.Notes)`. If cached results carry error Notes
(e.g., `"VALIDATION: ..."` prefix), they are excluded and `ResourceCount` stays 0.
This must also be investigated and tested.

**Fix sequence**:

1. Remove `s.clearCostCache()` from `ConfigureStack`.
2. Add `s.clearCostCache()` at the END of `AnalyzeStack`, after emitting diagnostics.
3. Add a unit test that calls `Analyze` N times then `AnalyzeStack` and asserts
   `ResourceCount == N` and `TotalMonthlyCost > 0`.

**Alternatives considered**:

- Clear cache only if called before any `Analyze` — requires state tracking, more
  complex and fragile.
- Clear at the START of `AnalyzeStack` — still wrong; clears what we need to read.

---

## Finding 4 — Analyzer JSON Logs in Diagnostics (#748)

**Decision**: In `setupLogging()` (called from `root.go` `PersistentPreRunE`), check
`os.Getenv(constants.EnvAnalyzerMode) == "true"` and when true, configure logging
output to `"file"` pointed at `cfg.Logging.File` (default: `~/.finfocus/logs/analyzer.log`).

**Detection mechanism**: `constants.EnvAnalyzerMode = "FINFOCUS_ANALYZER_MODE"` is
already set to `"true"` in `RunAnalyzerServe()` (line 160 of `analyzer_serve.go`).
No new flag or env var needed.

**Logging infrastructure**: `internal/logging/zerolog.go` already supports `Output:
"file"` in its `Config` struct. `NewLoggerWithPath()` handles file creation and parent
directory creation automatically.

**Fix**: Locate `setupLogging()` in the `internal/cli/` package. When analyzer mode
is detected, replace or supplement the console log output with a file output. The
stdout port handshake is unaffected since it uses `fmt.Println(port)`.

**Non-regression**: When `FINFOCUS_ANALYZER_MODE` is not set (direct CLI usage),
logging behavior is unchanged.

**Alternatives considered**:

- `--log-file` flag — requires caller (Pulumi) to pass it; Pulumi passes no custom
  flags to policy pack binaries.
- Always log to file for `analyzer serve` — would break direct user invocations where
  users expect to see logs on screen.

---

## Finding 5 — Recorder Nil Summary (#747)

**Decision**: In `plugins/recorder/plugin.go` `GetRecommendations`, add a non-nil
`Summary` to all response return paths.

**Proto type**: `pbc.RecommendationsSummary` from `finfocus-spec`. Fields include
`TotalCount`, `TotalMonthlySavings`, `Currency`.

**All return paths to fix** (lines 140–208 in `plugins/recorder/plugin.go`):

1. Mock mode with `mocker.CreateRecommendationsResponse()` — wrap result to ensure
   `Summary` is set if not already set by mocker.
2. Mock-enabled pagination path (lines 201–203) — add Summary.
3. Non-mock path (lines 206–208) — add empty Summary.

**Alternatives considered**: Removing the nil check in `pluginhost/host.go` — wrong;
the check protects against plugin bugs; fix the plugin, not the host.

---

## Finding 6 — FINFOCUS_PLUGIN_DIR Not Implemented (#752)

**Decision**: Add `FINFOCUS_PLUGIN_DIR` handling to `applyEnvOverrides()` in
`internal/config/config.go`. Applied AFTER `FINFOCUS_HOME` sets the base directory,
so it overrides only `PluginDir` without affecting cache, logs, or specs.

```go
// At end of applyEnvOverrides():
if pluginDir := os.Getenv("FINFOCUS_PLUGIN_DIR"); pluginDir != "" {
    c.PluginDir = pluginDir
}
```

**Precedence** (highest to lowest):

1. `FINFOCUS_PLUGIN_DIR` env var
2. `FINFOCUS_HOME` → `$FINFOCUS_HOME/plugins`
3. `plugin_dir:` config key (see Finding 7)
4. Computed default `~/.finfocus/plugins`

**Alternatives considered**: Remove from docs — not preferred; the env var is a
legitimate use case (CI override without changing home).

---

## Finding 7 — plugins.dir Config Key Not Parsed (#753)

**Decision**: Add a top-level `plugin_dir` key to the YAML config (NOT nested under
`plugins:`). This avoids conflicting with the existing `plugins: <name>: ...` per-plugin
map schema. Update `docs/reference/config-reference.md` to reflect the actual key name.

```go
// In Config struct, change:
PluginDir string `yaml:"-" json:"-"`
// To a new separate field:
PluginDirOverride string `yaml:"plugin_dir,omitempty" json:"plugin_dir,omitempty"`
```

In `New()`, after `cfg.Load()`:

```go
if cfg.PluginDirOverride != "" {
    cfg.PluginDir = cfg.PluginDirOverride
}
```

**Why not `plugins.dir`**: The `plugins:` YAML key maps to
`map[string]PluginConfig` — a map keyed by plugin name. Adding a `dir:` key inside it
would require custom `UnmarshalYAML` logic to distinguish structural keys from plugin
names. `plugin_dir:` at the top level avoids this complexity entirely.

**Docs update**: Change `plugins:\n  dir:` examples to `plugin_dir:` in
`config-reference.md`.

**Alternatives considered**:

- Custom YAML unmarshaling for `plugins:` section — technically correct but high
  complexity for low value.
- Remove from docs — not preferred; the feature is useful for multi-project setups.

---

## Finding 8 — AnalyzerPlugin.Enabled Dead Code (#751)

**Decision**: In `setupAnalyzerInfra()` (`internal/cli/analyzer_serve.go`), after
`reg.Open()` returns `clients`, filter out clients whose plugin name appears in
`cfg.Analyzer.Plugins` with `Enabled == false`.

```go
// Filter clients based on AnalyzerPlugin.Enabled
if len(cfg.Analyzer.Plugins) > 0 {
    filtered := clients[:0]
    for _, client := range clients {
        pluginCfg, configured := cfg.Analyzer.Plugins[client.Name()]
        if configured && !pluginCfg.Enabled {
            logger.Info().Str("plugin", client.Name()).Msg("plugin disabled in analyzer config")
            continue
        }
        filtered = append(filtered, client)
    }
    clients = filtered
}
```

**Default behavior**: A plugin with no entry in `cfg.Analyzer.Plugins` is treated as
enabled (backward compatible — existing configs unaffected).

**Requires knowing client name**: Need to verify the method to get the plugin name
from a `*pluginhost.Client` — likely `client.Name()` or accessing a `Name` field.
Check `internal/pluginhost/` for the `Client` struct definition.

**Alternatives considered**:

- Remove the `Enabled` field — not preferred; per-plugin control is genuinely useful
  without requiring a separate `FINFOCUS_HOME` directory.

---

## Finding 9 — SBOM CI Permission (#698)

**Decision**: In `.github/workflows/docker.yml`, change `contents: read` to
`contents: write` in the `permissions:` block of the job that runs `anchore/sbom-action`.

**Impact**: Minimal. `contents: write` is required for `anchore/sbom-action`'s
`upload-release-assets: true` behavior (the default). The `release.yml` workflow
already uses `contents: write` for the same reason.

**Non-tag runs**: The action's release asset upload is gated on tag detection
internally; non-tag runs produce no release asset upload regardless of permission.

---

## Finding 10 — Intermittent $0.00 TUI Projected Costs (#723)

**Decision**: Root cause is likely in `resolveSKUAndRegion` returning empty strings
for some resource types, causing the plugin to receive a request with no SKU/region
and return $0.

**Evidence from research**:

- `overview_enrich.go` passes `row.Properties` as `resource.Properties`
- `adapter.go` calls `resolveSKUAndRegion(provider, resourceType, properties)` which
  runs provider-specific extraction chains with multiple fallbacks
- If all fallbacks miss (e.g., an unusual AWS property name), both `sku` and `region`
  are empty strings
- `pluginsdk.ValidateProjectedCostRequest` may pass an empty-SKU request through
  (not all plugins require SKU for pricing)
- The plugin then returns $0 for an unrecognized empty-SKU resource

**Investigation plan**:

1. Add `DEBUG`-level logs in `resolveSKUAndRegion` when returning empty SKU or region
2. Run `finfocus cost overview --debug` 10× and grep for empty SKU/region log lines
3. If confirmed, either: (a) add missing property name mappings for the affected resource
   types, or (b) return a placeholder result with a diagnostic note instead of $0

**Intermittency explanation**: Plugin process initialization on first run may return
$0 before the process is ready; subsequent runs use a warmed plugin. Adding a startup
health check or retry is the secondary fix if SKU extraction is not the primary cause.

---

## Finding 11 — Force Reinstall Policy Pack Sync (#754)

**Decision**: Deferred. This bug depends on the policy pack setup feature (not yet
implemented). Once `--setup-policy-pack` is added to `finfocus analyzer install`,
the `--force` path should also re-sync the policy pack binary.

**No research needed now**. Track in the spec as P4 and implement after the
dependency lands.
