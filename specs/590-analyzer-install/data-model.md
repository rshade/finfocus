# Data Model: Analyzer Install/Uninstall

**Feature**: 590-analyzer-install
**Date**: 2026-02-13

## Entities

### InstallOptions

User-configurable parameters for the install operation.

| Field | Type | Required | Default | Description |
| ----- | ---- | -------- | ------- | ----------- |
| Force | bool | No | false | Overwrite existing installation |
| TargetDir | string | No | "" (auto-resolve) | Override Pulumi plugin directory |

### InstallResult

Result of an install or status check operation.

| Field | Type | Description |
| ----- | ---- | ----------- |
| Installed | bool | Whether the analyzer is currently installed |
| Version | string | Installed version (empty if not installed) |
| Path | string | Full path to the installed binary |
| Method | string | "symlink" or "copy" |
| NeedsUpdate | bool | True if installed version differs from current |
| CurrentVersion | string | Version of the running finfocus binary |

## Filesystem Layout

### Pulumi Plugin Directory Structure

```text
~/.pulumi/plugins/
└── analyzer-finfocus-v0.2.0/
    └── pulumi-analyzer-finfocus    # Symlink → /usr/local/bin/finfocus (Unix)
                                     # OR copy of finfocus binary (Windows)
```

### Directory Resolution Precedence

1. `--target-dir` flag value (if provided)
2. `$PULUMI_HOME/plugins/` (if `PULUMI_HOME` env var is set)
3. `$HOME/.pulumi/plugins/` (default)

## State Transitions

```text
NOT_INSTALLED ──install──→ INSTALLED
INSTALLED ──uninstall──→ NOT_INSTALLED
INSTALLED ──install (same version, no --force)──→ INSTALLED (no-op, report status)
INSTALLED ──install (different version, no --force)──→ INSTALLED (no-op, suggest --force)
INSTALLED ──install --force──→ INSTALLED (replaced)
NOT_INSTALLED ──uninstall──→ NOT_INSTALLED (no-op, report status)
```

## Functions

### Core Functions (internal/analyzer/install.go)

| Function | Signature | Description |
| -------- | --------- | ----------- |
| Install | `Install(ctx context.Context, opts InstallOptions) (*InstallResult, error)` | Create symlink/copy to Pulumi plugin dir |
| Uninstall | `Uninstall(ctx context.Context, targetDir string) error` | Remove installed analyzer directories |
| IsInstalled | `IsInstalled(targetDir string) (bool, error)` | Check if any analyzer-finfocus directory exists |
| InstalledVersion | `InstalledVersion(targetDir string) (string, error)` | Parse version from directory name |
| NeedsUpdate | `NeedsUpdate(targetDir string) (bool, error)` | Compare installed vs current binary version |
| ResolvePulumiPluginDir | `ResolvePulumiPluginDir(override string) (string, error)` | Resolve plugin dir with precedence |

### Constants

| Constant | Value | Description |
| -------- | ----- | ----------- |
| analyzerDirPrefix | `"analyzer-finfocus-v"` | Directory name prefix for version scanning |
| analyzerBinaryName | `"pulumi-analyzer-finfocus"` | Binary name inside the plugin directory |
