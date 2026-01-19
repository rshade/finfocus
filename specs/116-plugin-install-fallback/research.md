# Research: Plugin Install Version Fallback

**Date**: 2026-01-18
**Feature**: 116-plugin-install-fallback

## Research Tasks

### 1. Existing Fallback Infrastructure

**Question**: Does the codebase already have fallback search logic?

**Finding**: YES - `internal/registry/github.go:FindReleaseWithAsset()` already implements the core fallback search logic:

- Tries the requested version first
- Falls back to up to 10 stable releases
- Returns the first release with a compatible platform asset
- Silent operation (no user notification)

**Decision**: Reuse existing `FindReleaseWithAsset()` but refactor to expose:

1. Whether fallback was triggered (original version lacked assets)
2. The original requested version vs. actual installed version

**Rationale**: Avoids code duplication and leverages battle-tested search logic.

**Alternatives Considered**:

- New `FindReleaseWithFallbackInfo()` method - Rejected (too much duplication)
- Modify return type to include metadata - Selected approach

### 2. TTY Detection Patterns

**Question**: How should we detect interactive vs. non-interactive mode?

**Finding**: Existing infrastructure in `internal/tui/detect.go`:

- `IsTTY()` - Returns true if stdout is connected to a terminal
- `DetectOutputMode()` - Full detection with CI environment handling
- Uses `golang.org/x/term.IsTerminal()` under the hood

**Decision**: Use `tui.IsTTY()` for simple interactive check.

**Rationale**: Consistent with existing codebase patterns and cross-platform tested.

**Alternatives Considered**:

- Direct `term.IsTerminal()` call - Rejected (breaks abstraction)
- Full `DetectOutputMode()` - Rejected (overkill for yes/no prompt)

### 3. User Prompt Implementation

**Question**: How should the interactive prompt be implemented?

**Finding**: The project uses Bubble Tea for complex TUI but simple prompts should be lightweight.

**Decision**: Create a minimal prompt utility in `internal/cli/prompt.go`:

- Use `bufio.Scanner` for stdin reading
- Support "Y/n" format with "n" as default (per clarification)
- Return immediately on non-TTY without prompting

**Rationale**: Simple prompts don't warrant full Bubble Tea complexity.

**Alternatives Considered**:

- Bubble Tea prompt component - Rejected (heavyweight for Y/n)
- Third-party prompt library - Rejected (unnecessary dependency)
- Direct fmt.Scanln - Rejected (doesn't handle edge cases well)

### 4. Flag Mutual Exclusivity

**Question**: How to enforce `--fallback-to-latest` and `--no-fallback` mutual exclusivity?

**Finding**: Cobra doesn't have built-in mutual exclusivity for flags. Two approaches:

1. `cmd.MarkFlagsMutuallyExclusive()` - Available in Cobra v1.6+
2. Manual validation in `RunE` function

**Decision**: Use `cmd.MarkFlagsMutuallyExclusive("fallback-to-latest", "no-fallback")` - Available in project's Cobra v1.10.2.

**Rationale**: Native Cobra support provides better error messages and help text.

**Alternatives Considered**:

- Manual validation in RunE - Rejected (Cobra provides better UX)
- Single `--fallback` flag with values - Rejected (three states awkward)

### 5. Error Message Design

**Question**: What information should the fallback warning/error messages contain?

**Finding**: Current error message is: `no asset found for linux/amd64. Available: []`

**Decision**: New message format for fallback scenarios:

```text
Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64).
? Would you like to install the latest stable version (v0.1.2) instead? [y/N]
```

On acceptance:

```text
Installing aws-public@v0.1.2 (fallback from v0.1.3)...
✓ Plugin installed successfully
  Name:    aws-public
  Version: v0.1.2 (requested: v0.1.3)
  Path:    ~/.finfocus/plugins/aws-public/v0.1.2
```

**Rationale**: Clear communication of what happened, what's being offered, and what was installed.

### 6. Integration with Existing Install Flow

**Question**: Where in the install flow should fallback logic be injected?

**Finding**: Current flow in `internal/registry/installer.go`:

1. `Install()` → parses specifier, acquires lock
2. `installFromRegistry()` or `installFromURL()` → fetches release
3. `installRelease()` → downloads and installs

The release fetching in step 2 is where version-specific assets are checked.

**Decision**: Modify `Installer` to:

1. Add `InstallOptions.FallbackToLatest` and `InstallOptions.NoFallback` fields
2. Add `InstallResult.WasFallback` and `InstallResult.RequestedVersion` fields
3. Handle fallback logic in `installFromRegistry()` and `installFromURL()`
4. CLI layer handles prompting based on `InstallOptions` and TTY state

**Rationale**: Keeps CLI-level concerns (prompting) separate from installation logic.

## Summary

Research Item | Decision | Confidence
--------------|----------|------------
Fallback search | Reuse `FindReleaseWithAsset()` with metadata | High
TTY detection | Use `tui.IsTTY()` | High
Prompt implementation | Custom `bufio.Scanner` in `cli/prompt.go` | High
Flag exclusivity | Cobra `MarkFlagsMutuallyExclusive()` | High
Error messages | Structured format with version info | High
Integration point | Modify `InstallOptions`/`InstallResult` | High

## Open Questions

None - all clarifications resolved.
