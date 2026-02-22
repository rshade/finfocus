# Data Model: Batch Bug Fixes (599)

**Date**: 2026-02-21

These fixes modify behavior, not data schemas. This file documents the entities
affected by each fix and the state/lifecycle changes involved.

---

## Entity: Plugin Discovery Entry (Registry)

**Affected by**: #750

| Field | Type | Notes |
|-------|------|-------|
| `Name` | string | Plugin name (directory name, may be a symlink target) |
| `Version` | string | Version string (directory name, may be a symlink target) |
| `Path` | string | Absolute path to the executable binary |
| `Metadata` | map[string]string | Optional metadata from `plugin.metadata.json` |

**Lifecycle change**: `ListPlugins()` must follow symlinks when checking whether a
directory entry is a directory. A symlink-to-directory must be treated identically
to a real directory. A broken symlink must be silently skipped.

**Invariant**: `Path` always points to a real executable file (not a symlink
itself); `os.Stat` is already used downstream in `findBinary()` so file-level
symlinks are already followed.

---

## Entity: Analyzer Cost Cache

**Affected by**: #746

| Field | Type | Notes |
|-------|------|-------|
| Key | string | Resource ID (URN-derived) |
| Value | `engine.CostResult` | Per-resource projected cost from an `Analyze` call |

**Lifecycle (corrected)**:

```text
ConfigureStack called
  → do NOT clear cache (old: cleared here — BUG)

Analyze called N times
  → cacheCost(resourceID, cost) for each resource

AnalyzeStack called
  → getCachedCosts() → builds summary
  → emit stack-cost-summary diagnostic
  → clearCostCache() ← moved here (correct)
```

**Invariant**: The cache is populated by `Analyze` calls and read exactly once by
`AnalyzeStack`. It is cleared after `AnalyzeStack` completes to prepare for any
subsequent analysis session.

---

## Entity: Analyzer Install Directory

**Affected by**: #749

| Attribute | Value |
|-----------|-------|
| Location | `~/.pulumi/plugins/` |
| Name format | `analyzer-finfocus-v{version}` (exactly one `v`) |
| Binary name | `pulumi-analyzer-finfocus` |

**Version normalization rule**:

- If `GetVersion()` returns `"v1.2.3"` → directory is `analyzer-finfocus-v1.2.3`
- If `GetVersion()` returns `"1.2.3-dirty"` → directory is `analyzer-finfocus-v1.2.3-dirty`
- Never: `analyzer-finfocus-vv1.2.3`

---

## Entity: Config (PluginDir override chain)

**Affected by**: #752, #753

**Precedence** (highest to lowest):

| Source | Mechanism | Scope |
|--------|-----------|-------|
| `FINFOCUS_PLUGIN_DIR` env var | `applyEnvOverrides()` | Plugin dir only |
| `FINFOCUS_HOME` env var | `ResolveConfigDir()` | Entire finfocus home |
| `plugin_dir:` config key | YAML field `PluginDirOverride` | Plugin dir only |
| Default | `filepath.Join(finfocusDir, "plugins")` | Plugin dir only |

**New Config field**:

```go
// Config struct addition
PluginDirOverride string `yaml:"plugin_dir,omitempty" json:"plugin_dir,omitempty"`
```

**Evaluation in `New()`** (after `cfg.Load()`):

```text
1. Apply PluginDirOverride from YAML if non-empty
2. Apply FINFOCUS_PLUGIN_DIR from env in applyEnvOverrides() (overrides YAML)
```

---

## Entity: AnalyzerPlugin Config

**Affected by**: #751

```go
type AnalyzerPlugin struct {
    Path    string            `yaml:"path"`
    Enabled bool              `yaml:"enabled"` // default: true (zero value = false, so absence means unset)
    Env     map[string]string `yaml:"env"`
}
```

**Effective enablement rule**:

- Plugin entry absent from `cfg.Analyzer.Plugins` → **enabled** (default, backward compatible)
- Plugin entry present with `Enabled: true` → enabled
- Plugin entry present with `Enabled: false` → **disabled** (excluded from client list)
- Plugin entry present with `Enabled` not set (zero value `false`) → **disabled**

> **Note**: Because the Go zero value for `bool` is `false`, the absence of `enabled:`
> in YAML is indistinguishable from `enabled: false`. The filter logic MUST only
> exclude plugins that are **explicitly listed** in `cfg.Analyzer.Plugins` with
> `Enabled == false`. Plugins NOT listed in the map default to enabled.

---

## Entity: Log Output Configuration (Analyzer Mode)

**Affected by**: #748

**Normal mode** (`FINFOCUS_ANALYZER_MODE` unset):

| Output | Destination |
|--------|-------------|
| Logs | Console (stderr) |
| Port | stdout (`fmt.Println(port)`) |

**Analyzer mode** (`FINFOCUS_ANALYZER_MODE=true`):

| Output | Destination |
|--------|-------------|
| Logs | `~/.finfocus/logs/analyzer.log` (file only) |
| Port | stdout (`fmt.Println(port)`) — unchanged |

The `logging.Config.Output` field is set to `"file"` and `logging.Config.File` is
set to the log file path. No stderr logging occurs.

---

## Entity: GetRecommendationsResponse (Recorder Plugin)

**Affected by**: #747

| Field | Type | Required |
|-------|------|----------|
| `Recommendations` | `[]*pbc.Recommendation` | Yes (can be empty slice) |
| `Summary` | `*pbc.RecommendationsSummary` | Yes (MUST be non-nil) |
| `NextPageToken` | string | No |

**RecommendationsSummary fields** (to be populated even when empty):

| Field | Value when empty |
|-------|-----------------|
| `TotalCount` | 0 |
| `TotalMonthlySavings` | 0 |
| `Currency` | `"USD"` |

---

## Entity: GitHub Workflow Permissions

**Affected by**: #698

| Job | Old | New |
|-----|-----|-----|
| `build-and-push` in `docker.yml` | `contents: read` | `contents: write` |
| `packages` | `write` | `write` (unchanged) |
