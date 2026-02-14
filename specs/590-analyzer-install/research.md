# Research: Analyzer Install/Uninstall

**Feature**: 590-analyzer-install
**Date**: 2026-02-13

## Research Topics

### 1. Pulumi Plugin Discovery Convention

**Decision**: Use `analyzer-finfocus-vX.Y.Z/pulumi-analyzer-finfocus` naming.

**Rationale**: The `main.go` binary name detection (lines 30-31) checks for `pulumi-analyzer-finfocus` and `pulumi-analyzer-policy-finfocus`. The installed binary must match one of these patterns for the analyzer to auto-detect its mode. The Pulumi CLI discovers plugins in `~/.pulumi/plugins/<kind>-<name>-v<version>/` directories.

**Alternatives considered**:

- `pulumi-analyzer-cost` (used in `specs/009-analyzer-plugin/quickstart.md`): Would require updating `main.go` to detect this name. Not chosen because it would change existing behavior.
- `pulumi-analyzer-policy-finfocus` (legacy): Still supported via `strings.Contains()` in `main.go` but is the older naming convention. Not chosen as primary.

### 2. Pulumi Home Directory Resolution

**Decision**: Follow existing `ResolveConfigDir()` pattern from `internal/config/config.go:151-179`.

**Rationale**: The project already has a well-tested precedence system for directory resolution. The Pulumi plugin directory should follow the same pattern: explicit flag > environment variable > default.

**Resolution order**:

1. `--target-dir` flag (explicit user override)
2. `$PULUMI_HOME/plugins/` (if `PULUMI_HOME` environment variable is set)
3. `$HOME/.pulumi/plugins/` (default Pulumi location)

**Alternatives considered**:

- Only supporting `~/.pulumi/plugins/`: Rejected because `PULUMI_HOME` is a standard Pulumi environment variable and should be respected.
- Adding `FINFOCUS_PULUMI_PLUGIN_DIR` env var: Over-engineering. The `--target-dir` flag serves this purpose.

### 3. Binary Path Resolution

**Decision**: Use `os.Executable()` with `filepath.EvalSymlinks()` to resolve the current binary path.

**Rationale**: `os.Executable()` is the standard Go mechanism for finding the running binary's path. `EvalSymlinks()` ensures we get the real path even if finfocus was itself invoked via a symlink.

**Alternatives considered**:

- `os.Args[0]`: Not reliable (may be relative, may not resolve to the actual binary).
- Hardcoded path from build: Not portable.

### 4. Symlink vs Copy Strategy

**Decision**: Symlink on Unix with copy fallback; always copy on Windows.

**Rationale**: Symlinks are preferred because they don't duplicate disk space and automatically reflect binary updates. However, Windows symlinks require `SeCreateSymbolicLinkPrivilege` (admin rights), so copy is the safe default. On Unix, cross-filesystem symlinks aren't supported, so a copy fallback handles edge cases like the binary on a different mount.

**Alternatives considered**:

- Always copy: Simpler but wastes disk space and doesn't auto-update.
- Always symlink: Not portable (Windows privilege issues).
- Hard links: Not portable across filesystems.

### 5. Version Detection for Installed Analyzer

**Decision**: Parse version from directory name pattern `analyzer-finfocus-v<version>`.

**Rationale**: The Pulumi plugin directory naming convention embeds the version in the directory name. This is the canonical source of version information for installed plugins. No need for a separate metadata file.

**Alternatives considered**:

- Running the installed binary with `--version`: Requires executing the binary, which is slower and may fail.
- Writing a metadata file alongside the binary: Extra complexity; the directory name is sufficient.
- Checking symlink target's build info: Not reliable when the binary is copied rather than symlinked.

### 6. Existing Code Patterns to Reuse

**Decision**: Follow existing CLI and error handling patterns; do not reuse registry `Installer`.

**Rationale**: The `registry.Installer` is designed for downloading, extracting, and managing finfocus plugins from GitHub releases with features like locking, archive extraction, and metadata. The analyzer install is much simpler: resolve binary, create directory, create symlink/copy. Reusing `Installer` would couple the analyzer install to unnecessary complexity.

**Patterns to follow**:

- CLI command structure: `analyzer_serve.go` pattern (NewAnalyzerXxxCmd + RunE)
- Error messages: `cmd.Printf()` for output (not `fmt.Printf()`)
- Flags: `--force` and `--target-dir` follow `plugin_install.go` conventions
- Testing: `t.TempDir()` for filesystem tests, testify assertions

### 7. Naming Discrepancy Resolution

**Decision**: Document the `cost` vs `finfocus` naming discrepancy; install command uses `finfocus`.

**Rationale**: The existing `main.go` binary detection uses `pulumi-analyzer-finfocus`. The quickstart documentation uses `pulumi-analyzer-cost`. These are inconsistent. The install command must match the runtime detection code to work correctly. A separate issue should be filed to either:

- Add `pulumi-analyzer-cost` to `main.go` detection, OR
- Update all documentation to use `finfocus` consistently

This is out of scope for issue #597.
