# Quickstart: Batch Bug Fixes (599)

**Date**: 2026-02-21
**Audience**: Developer implementing these fixes

## Prerequisites

- Go 1.25.8+
- `make build` producing `bin/finfocus`
- `finfocus plugin install aws-public` for integration testing
- `make install-recorder` for recorder plugin tests
- Pulumi CLI for analyzer tests (optional)

## Working Order

Fixes are ordered by size and independence. Start with the smallest, most isolated
fixes to build confidence before tackling the analyzer correctness bugs.

### Step 1 — CI Permission Fix (#698, 5 minutes)

```bash
# Edit .github/workflows/docker.yml
# Change: contents: read
# To:     contents: write
make lint
```

No Go changes. No tests needed. Verify in the next release run.

---

### Step 2 — Analyzer Install Double-v Fix (#749, 30 minutes)

**File**: `internal/analyzer/install.go`

1. Change `analyzerDirPrefix` from `"analyzer-finfocus-v"` to `"analyzer-finfocus-"`.
2. In `Install()` before constructing `versionedDir`, normalize version:

   ```go
   ver := version.GetVersion()
   if !strings.HasPrefix(ver, "v") {
       ver = "v" + ver
   }
   ```

3. Apply `ver` (not `currentVersion`) everywhere a versioned directory is constructed.
4. Write unit test in `internal/analyzer/install_test.go`:
   - Table-driven test with inputs `"v0.3.1"`, `"0.3.1-dirty"`, `"v1.0.0-rc1"`
   - Assert resulting directory name has exactly one `v`.

```bash
go test ./internal/analyzer/... -run TestInstall
make lint
```

---

### Step 3 — Registry Symlink Fix (#750, 45 minutes)

**File**: `internal/registry/registry.go`

Replace `entry.IsDir()` at two locations:

```go
// Line ~52 — plugin name level
info, err := os.Stat(filepath.Join(r.root, entry.Name()))
if err != nil || !info.IsDir() {
    continue
}

// Line ~63 — version level
info, err := os.Stat(filepath.Join(pluginPath, version.Name()))
if err != nil || !info.IsDir() {
    continue
}
```

Write test in `internal/registry/registry_test.go`:

- Create a real plugin dir structure in `t.TempDir()`
- Create a symlink pointing to it
- Call `ListPlugins()` and assert the plugin is discovered

```bash
go test ./internal/registry/... -run TestListPlugins
make lint
```

---

### Step 4 — SBOM Workflow Fix (#698 — already done in Step 1)

Already completed in Step 1.

---

### Step 5 — Recorder Nil Summary Fix (#747, 30 minutes)

**File**: `plugins/recorder/plugin.go`

In all return paths of `GetRecommendations`, ensure `Summary` is set:

```go
summary := &pbc.RecommendationsSummary{
    TotalCount:          0,
    TotalMonthlySavings: 0,
    Currency:            "USD",
}
// ... build actual summary from recs if available ...
return &pbc.GetRecommendationsResponse{
    Recommendations: recs,
    Summary:         summary,
}, nil
```

Write test in `plugins/recorder/plugin_test.go`:

- Call `GetRecommendations` with both mock mode enabled and disabled
- Assert `resp.Summary != nil` in both cases

```bash
go test ./plugins/recorder/... -run TestGetRecommendations
make lint
```

---

### Step 6 — Config/Env Fixes (#751, #752, #753, 1–2 hours)

These three fixes touch `internal/config/config.go` and `internal/cli/analyzer_serve.go`.

**#752 — FINFOCUS_PLUGIN_DIR** (add to `applyEnvOverrides()`):

```go
if pluginDir := os.Getenv("FINFOCUS_PLUGIN_DIR"); pluginDir != "" {
    c.PluginDir = pluginDir
}
```

**#753 — plugin_dir config key** (add field to `Config` struct):

```go
PluginDirOverride string `yaml:"plugin_dir,omitempty" json:"plugin_dir,omitempty"`
```

And after `cfg.Load()` in `New()`:

```go
if cfg.PluginDirOverride != "" {
    cfg.PluginDir = cfg.PluginDirOverride
}
```

Update `docs/reference/config-reference.md` to use `plugin_dir:` (not `plugins.dir:`).

**#751 — AnalyzerPlugin.Enabled** (filter in `setupAnalyzerInfra()`):

```go
if len(cfg.Analyzer.Plugins) > 0 {
    filtered := clients[:0]
    for _, client := range clients {
        pluginCfg, configured := cfg.Analyzer.Plugins[client.Name()]
        if configured && !pluginCfg.Enabled {
            logger.Info().Str("plugin", client.Name()).Msg("plugin disabled")
            continue
        }
        filtered = append(filtered, client)
    }
    clients = filtered
}
```

Write tests in `internal/config/config_test.go` and
`internal/cli/analyzer_serve_test.go` covering each new behavior.

```bash
go test ./internal/config/... ./internal/cli/...
make lint
```

---

### Step 7 — Analyzer Log Redirect (#748, 1 hour)

**File**: Find `setupLogging()` in `internal/cli/` (likely `common_execution.go` or
`root.go`). Add analyzer mode detection:

```go
if os.Getenv(constants.EnvAnalyzerMode) == "true" {
    // Replace console output with file output
    cfg.Logging.Outputs = []config.LogOutput{
        {
            Type:  "file",
            Level: cfg.Logging.Level,
            Path:  cfg.Logging.File, // default: ~/.finfocus/logs/analyzer.log
        },
    }
}
```

Verify the fix by running `pulumi preview --policy-pack ~/.finfocus/analyzer` and
confirming no JSON lines appear in `Diagnostics:`.

```bash
go test ./internal/cli/... -run TestAnalyzer
make lint
```

---

### Step 8 — AnalyzeStack Zero Cost Fix (#746, 1.5 hours)

**File**: `internal/analyzer/server.go`

1. Remove `s.clearCostCache()` from `ConfigureStack` (line ~435).
2. Add `s.clearCostCache()` at the END of `AnalyzeStack` (after emitting the
   summary diagnostic).
3. Verify `BuildCostSummary` in `summary.go` is not filtering out valid $0-cost
   resources as errors.

Write unit test in `internal/analyzer/server_test.go`:

```go
// Call ConfigureStack, then Analyze N times with valid costs, then AnalyzeStack
// Assert: stack summary shows sum of all per-resource costs, ResourceCount == N
```

```bash
go test ./internal/analyzer/... -run TestAnalyzeStack
make lint
```

---

### Step 9 — Intermittent TUI Zero Costs (#723, investigation)

1. Add `DEBUG` log in `internal/proto/adapter.go` `resolveSKUAndRegion` when
   returning empty SKU or region.
2. Add `DEBUG` log in `internal/engine/overview_enrich.go` `enrichProjectedCost`
   when `costResult.Monthly == 0`.
3. Run `finfocus cost overview --debug 2>&1 | grep -E 'sku|region|zero|0\.00'`
   multiple times.
4. Based on findings, implement the appropriate fix (property mapping or startup
   retry).

```bash
go test ./internal/proto/... ./internal/engine/...
make lint
```

---

### Final Validation

```bash
make test      # All tests pass
make lint      # Zero lint errors
```
