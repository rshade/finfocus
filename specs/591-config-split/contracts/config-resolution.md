# Contract: Config Resolution

**Feature**: 591-config-split | **Date**: 2026-02-14

## Overview

This contract defines the behavior of configuration resolution when project-local
and user-global configuration coexist.

## Functions

### ResolveProjectDir

```go
func ResolveProjectDir(flagValue, startDir string) string
```

**Inputs**:

- `flagValue`: Value from `--project-dir` CLI flag (may be empty)
- `startDir`: Directory to start walk-up search from (typically CWD)

**Outputs**:

- Returns the path to the project-local `.finfocus/` directory
- Returns `""` if no project context is found

**Behavior**:

1. If `flagValue` is non-empty, return `filepath.Join(flagValue, ".finfocus")`
2. If `FINFOCUS_PROJECT_DIR` env var is set, return `filepath.Join($FINFOCUS_PROJECT_DIR, ".finfocus")`
3. Call `pulumi.FindProject(startDir)`:
   - If found, return `filepath.Join(projectRoot, ".finfocus")`
   - If `ErrNoProject`, return `""`
   - If other error, log warning and return `""`

**Postconditions**:

- Does NOT create the directory (read-only operation)
- Does NOT check if `.finfocus/` exists at the resolved path
- Returned path is always absolute (or empty)

---

### NewWithProjectDir

```go
func NewWithProjectDir(projectDir string) *Config
```

**Inputs**:

- `projectDir`: Path to project-local `.finfocus/` directory (may be empty)

**Outputs**:

- Returns a fully populated `*Config`

**Behavior**:

1. Call `New()` to load global config with defaults
2. If `projectDir` is empty, return the global config unchanged
3. Check if `projectDir/config.yaml` exists:
   - If not, return the global config unchanged
   - If yes, call `ShallowMergeYAML(cfg, projectDir + "/config.yaml")`
4. Return the merged config

**Error Handling**:

- Missing project config file: silent (use global defaults)
- Corrupted project config file: log warning, use global defaults
- Permission denied on project config: log warning, use global defaults

---

### ShallowMergeYAML

```go
func ShallowMergeYAML(target *Config, overlayPath string) error
```

**Inputs**:

- `target`: Base config to merge onto (modified in place)
- `overlayPath`: Path to the overlay YAML file

**Behavior**:

1. Read `overlayPath` into `map[string]interface{}`
2. For each key present in the overlay map:
   - Marshal the overlay value back to YAML bytes
   - Unmarshal onto the corresponding section of `target`
3. Keys not present in overlay are left unchanged in `target`

**Top-Level Keys**:

| Key | Type | Merge Semantics |
|-----|------|-----------------|
| `output` | `OutputConfig` | Entire section replaced |
| `plugins` | `map[string]PluginConfig` | Entire map replaced |
| `logging` | `LoggingConfig` | Entire section replaced |
| `analyzer` | `AnalyzerConfig` | Entire section replaced |
| `plugin_host` | `PluginHostConfig` | Entire section replaced |
| `cost` | `CostConfig` | Entire section replaced |
| `routing` | `*RoutingConfig` | Entire section replaced |

---

### EnsureGitignore

```go
func EnsureGitignore(dir string) (bool, error)
```

**Inputs**:

- `dir`: Directory to create `.gitignore` in

**Outputs**:

- `bool`: `true` if a new file was created, `false` if one already existed
- `error`: File system errors

**Behavior**:

1. Check if `dir/.gitignore` exists
2. If exists, return `(false, nil)` — never overwrite (FR-007)
3. If not, write the standard `.gitignore` content
4. Return `(true, nil)`

---

### NewDismissalStore (modified)

```go
func NewDismissalStore(filePath string) (*DismissalStore, error)
```

**Modified Behavior** (when `filePath` is empty):

1. Check package-level resolved project dir:
   - If set AND `$PROJECT_DIR/dismissed.json` path is valid: use it
2. Fall back to `ResolveConfigDir() + "/dismissed.json"`

**Migration**: No migration needed. If project-local `dismissed.json` doesn't exist,
the store starts empty (existing behavior for missing files).

## Invariants

1. **Global resources never project-scoped**: `PluginDir`, `SpecDir`, cache, and logs
   always resolve from `ResolveConfigDir()`, never from project dir.
2. **Merge is idempotent**: `NewWithProjectDir("")` produces identical results to `New()`.
3. **No auto-creation on read**: Config resolution never creates `.finfocus/` directories.
   Only `config init` creates them.
4. **Backward compatible**: Users with no Pulumi project context see zero behavior change.
5. **Nearest project wins**: When multiple `Pulumi.yaml` files exist in the hierarchy,
   the deepest (nearest to CWD) is used, matching Pulumi CLI behavior.

## Error Behavior

| Scenario | Behavior |
|----------|----------|
| No `Pulumi.yaml` in hierarchy | Use global config only |
| `Pulumi.yaml` found but no `.finfocus/` | Use global config only |
| `.finfocus/config.yaml` exists but corrupted | Log warning, use global config |
| `.finfocus/config.yaml` permission denied | Log warning, use global config |
| `--project-dir` points to nonexistent dir | Use global config only, log warning |
| `FINFOCUS_PROJECT_DIR` set to invalid path | Use global config only, log warning |
