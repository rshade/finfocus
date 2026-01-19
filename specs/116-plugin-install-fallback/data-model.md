# Data Model: Plugin Install Version Fallback

**Date**: 2026-01-18
**Feature**: 116-plugin-install-fallback

## Overview

This feature primarily modifies existing data structures rather than introducing new entities. The changes extend `InstallOptions` and `InstallResult` to support fallback behavior tracking.

## Modified Entities

### InstallOptions (Extended)

**Location**: `internal/registry/installer.go`

Existing fields preserved, new fields added:

Field | Type | Description
------|------|------------
Force | bool | Reinstall even if version exists (existing)
NoSave | bool | Don't add to config file (existing)
PluginDir | string | Custom plugin directory (existing)
**FallbackToLatest** | bool | Enable automatic fallback without prompting
**NoFallback** | bool | Disable fallback entirely (fail on missing assets)

**Validation Rules**:

- `FallbackToLatest` and `NoFallback` are mutually exclusive
- If both are false, behavior depends on caller (CLI prompts if interactive)

### InstallResult (Extended)

**Location**: `internal/registry/installer.go`

Existing fields preserved, new fields added:

Field | Type | Description
------|------|------------
Name | string | Plugin name (existing)
Version | string | Installed version (existing)
Path | string | Installation path (existing)
FromURL | bool | Installed from URL (existing)
Repository | string | Source repository (existing)
**WasFallback** | bool | True if installed version differs from requested
**RequestedVersion** | string | Original version requested (empty if @latest)

**State Transitions**:

- `WasFallback=false`: Direct installation of requested version
- `WasFallback=true`: Fallback occurred, `RequestedVersion` shows original request

## New Entities

### FallbackInfo

**Location**: `internal/registry/github.go`

Returned from modified release search to communicate fallback state:

Field | Type | Description
------|------|------------
Release | *GitHubRelease | The release that was selected
Asset | *ReleaseAsset | The platform-compatible asset
WasFallback | bool | True if different from requested version
RequestedVersion | string | Original version that was requested
FallbackReason | string | Why fallback occurred (e.g., "no compatible assets")

### PromptResult

**Location**: `internal/cli/prompt.go`

Result of user prompt interaction:

Field | Type | Description
------|------|------------
Accepted | bool | User accepted the fallback
TimedOut | bool | No response within timeout (if applicable)
Cancelled | bool | User explicitly cancelled (Ctrl+C)

## Entity Relationships

```text
CLI Command
    │
    ├── InstallOptions (input)
    │       ├── FallbackToLatest
    │       └── NoFallback
    │
    └── Installer.Install()
            │
            ├── GitHubClient.FindReleaseWithAsset()
            │       └── Returns FallbackInfo
            │
            └── InstallResult (output)
                    ├── WasFallback
                    └── RequestedVersion
```

## Validation Rules

1. **Flag Exclusivity**: `FallbackToLatest` and `NoFallback` cannot both be true
2. **Version Format**: `RequestedVersion` follows semver with optional "v" prefix
3. **Fallback Integrity**: If `WasFallback=true`, then `Version != RequestedVersion`
