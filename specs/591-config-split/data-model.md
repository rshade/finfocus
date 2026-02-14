# Data Model: Split Project-Local and User-Global Configuration

**Feature**: 591-config-split | **Date**: 2026-02-14

## Entities

### ConfigDirs (new concept)

Represents the resolved pair of configuration directories for a single CLI invocation.

| Field | Type | Description |
|-------|------|-------------|
| GlobalDir | `string` | Global config directory (from `ResolveConfigDir()`) |
| ProjectDir | `string` | Project-local config directory, empty if none found |
| ProjectRoot | `string` | Pulumi project root directory, empty if none found |

**Resolution Precedence for ProjectDir**:

1. `--project-dir` CLI flag (explicit override)
2. `$FINFOCUS_PROJECT_DIR` environment variable
3. Walk up from CWD to find `Pulumi.yaml`, use `$PROJECT_ROOT/.finfocus/`
4. Empty string (no project context, use global only)

**Validation Rules**:

- `GlobalDir` is always non-empty (fallback guarantees this)
- `ProjectDir` may be empty (no project context)
- If `ProjectDir` is non-empty, `ProjectRoot` is also non-empty
- `ProjectDir` is always `ProjectRoot + "/.finfocus"` when set

---

### Config (existing, modified)

The existing `Config` struct is unchanged. The change is in **how** it gets populated:

**Before**: `New()` → loads from `ResolveConfigDir()/config.yaml`

**After**: `NewWithProject(projectDir)` → loads global, then shallow-merges project config

| Merge Behavior | Global Key Present | Project Key Present | Result |
|---|---|---|---|
| output | yes | no | Global value |
| output | yes | yes | Project value (replaces entire section) |
| cost.budgets | yes | yes | Project value (replaces entire section) |
| plugins | yes | yes | Project value (replaces entire section) |
| logging | yes | no | Global value |

**Shallow Merge Rule**: For each top-level YAML key (`output`, `plugins`, `logging`,
`analyzer`, `plugin_host`, `cost`, `routing`), if the project config file contains
that key, the entire section from the project config replaces the global default.
Keys not present in the project config file inherit from global.

---

### DismissalStore (existing, modified)

**Before**: `NewDismissalStore("")` → always `~/.finfocus/dismissed.json`

**After**: `NewDismissalStore("")` → resolves using project context:

1. If project dir available: `$PROJECT/.finfocus/dismissed.json`
2. Else: `ResolveConfigDir()/dismissed.json`

**No schema changes** to `dismissed.json`. Same version, same format.

---

### .gitignore (new artifact)

Generated inside project-local `.finfocus/` directory.

**Contents**:

```gitignore
# FinFocus project-local data (auto-generated)
# Config is tracked; user-specific state is not.
dismissed.json
dismissed.json.lock
dismissed.json.tmp
*.log
```

**State Transitions**:

- Created by `config init` when inside a Pulumi project
- Never overwritten if already exists (FR-007)
- No API to modify; user edits directly if needed

## Relationships

```text
                    ┌─────────────────┐
                    │   CLI Command   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   ConfigDirs    │
                    │  (resolved at   │
                    │   startup)      │
                    └──┬──────────┬───┘
                       │          │
              ┌────────▼──┐  ┌───▼────────┐
              │ GlobalDir  │  │ ProjectDir │
              │~/.finfocus │  │$PRJ/.finfoc│
              └──┬───┬─┬──┘  └──┬─────┬───┘
                 │   │ │        │     │
         ┌───────┘   │ │   ┌────┘     │
         │           │ │   │          │
    ┌────▼───┐  ┌────▼─▼───▼──┐  ┌────▼──────┐
    │plugins/│  │ config.yaml  │  │dismissed. │
    │cache/  │  │ (merged)     │  │json       │
    │logs/   │  └──────────────┘  │(project-  │
    └────────┘                    │ scoped)   │
                                  └───────────┘
```

## Directory Layout

### Global (~/.finfocus/)

```text
~/.finfocus/                  # ResolveConfigDir() result
├── config.yaml               # Global default configuration
├── dismissed.json             # Fallback dismissal store (no project context)
├── plugins/                   # Shared across all projects
│   └── <name>/<version>/
├── cache/                     # Shared query cache
├── logs/                      # Shared log files
│   └── finfocus.log
└── specs/                     # Legacy local pricing specs
```

### Project ($PROJECT/.finfocus/)

```text
$PULUMI_PROJECT/              # Directory containing Pulumi.yaml
├── Pulumi.yaml
├── .finfocus/                 # Project-local config directory
│   ├── .gitignore             # Auto-generated (protects user-specific data)
│   ├── config.yaml            # Project-specific overrides (committed to VCS)
│   └── dismissed.json         # Project-scoped dismissal state (git-ignored)
└── src/                       # Project source code
```

## New Functions

### internal/config/project.go

```go
// ResolveProjectDir returns the project-local .finfocus directory path.
// It checks (in order): --project-dir flag, FINFOCUS_PROJECT_DIR env,
// Pulumi.yaml walk-up from startDir. Returns "" if no project found.
func ResolveProjectDir(flagValue, startDir string) string

// NewWithProjectDir creates a Config by loading global config then
// shallow-merging project-local config on top. If projectDir is empty,
// behaves identically to New().
func NewWithProjectDir(projectDir string) *Config
```

### internal/config/merge.go

```go
// ShallowMergeYAML loads a YAML file and merges its top-level keys onto
// the target Config. Keys present in the overlay replace entire sections
// in the target. Keys absent in the overlay are left unchanged.
func ShallowMergeYAML(target *Config, overlayPath string) error
```

### internal/config/gitignore.go

```go
// EnsureGitignore creates a .gitignore file in the given directory if one
// does not already exist. Returns true if a new file was created.
func EnsureGitignore(dir string) (bool, error)
```

### internal/config/dismissed.go (modified)

```go
// NewDismissalStore: When filePath is "", resolves using project context
// via the package-level resolved project dir, falling back to
// ResolveConfigDir()/dismissed.json.
func NewDismissalStore(filePath string) (*DismissalStore, error)
```
