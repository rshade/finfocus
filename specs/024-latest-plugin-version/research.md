# Research: Latest Plugin Version Selection

## Current Implementation Analysis

The `internal/registry` package already contains a `ListLatestPlugins` method in `registry.go` (Lines 82-115).

### Existing Logic
- **Discovery**: `ListPlugins` scans `~/.finfocus/plugins/<name>/<version>/`.
- **Selection**: `ListLatestPlugins` iterates over all plugins, uses `semver.NewVersion` to parse versions, and keeps the highest version for each plugin name.
- **Semver**: Uses `github.com/Masterminds/semver/v3`.
- **Filtering**: `Open` method (Lines 169-257) uses `ListLatestPlugins` to get the list of plugins to load.

### Code Gap Analysis
- **Missing Tests**: `internal/registry/registry_test.go` contains `TestListPlugins` but **no tests** for `ListLatestPlugins`.
- **Test Coverage**: The existing `TestListPlugins` validates that multiple versions are *listed*, but does not validate that `ListLatestPlugins` correctly *selects* the latest one.
- **Edge Cases**: No visible tests for:
  - Invalid version strings (should be skipped/warned).
  - Pre-release versions (precedence rules).
  - Corrupted directories.

## Technology Choices

- **Semver Library**: `github.com/Masterminds/semver/v3` (Already in use).
  - **Rationale**: Industry standard for Go, robust parsing.
- **Plugin Structure**: `~/.finfocus/plugins/<name>/<version>/` (Already established).
  - **Rationale**: Supports side-by-side version installation.

## Implementation Strategy

Since the core logic exists but lacks verification:

1.  **Retroactive TDD**: Create `TestListLatestPlugins` in `internal/registry/registry_test.go` covering all Acceptance Scenarios from the Spec *before* modifying any code.
2.  **Verify & Fix**: Run tests against existing implementation. If bugs found (e.g. pre-release handling), fix them.
3.  **Refactor**: Ensure `Open` uses `ListLatestPlugins` correctly (it seems it does, but verify).

## Unknowns Resolved

- **Existing Plugin Logic**: Fully identified in `internal/registry/registry.go`.
- **Semver Lib**: Confirmed `v3`.
- **Directory Structure**: Confirmed.
- **Concurrent Access**: `Registry` methods are read-only on the filesystem. Concurrent calls to `ListLatestPlugins` are safe as they only read.