# Feature Specification: Analyzer Install/Uninstall

**Feature Branch**: `590-analyzer-install`
**Created**: 2026-02-13
**Status**: Draft
**Input**: User description: "Replace the current 4 manual steps for Pulumi Analyzer setup with a single `finfocus analyzer install` command. Add corresponding `finfocus analyzer uninstall` for clean removal."
**GitHub Issue**: #597

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-Time Analyzer Installation (Priority: P1)

A developer wants to enable zero-click cost estimation during `pulumi preview`. Today this requires 4 manual steps: locating the finfocus binary, creating the Pulumi plugins directory, symlinking/copying the binary with the correct name, and verifying the installation. With the new command, they run a single `finfocus analyzer install` and the analyzer is ready to use immediately.

**Why this priority**: This is the core value proposition. Without this, the feature has no purpose. First-time setup is the highest-friction point for new users.

**Independent Test**: Can be fully tested by running `finfocus analyzer install` and then verifying that the analyzer binary exists in the Pulumi plugin directory with the correct name and permissions.

**Acceptance Scenarios**:

1. **Given** finfocus is installed and Pulumi CLI is available, **When** user runs `finfocus analyzer install`, **Then** the analyzer binary is placed in the Pulumi plugin directory with the correct name and the command reports success with the installed version.
2. **Given** the Pulumi plugin directory does not exist, **When** user runs `finfocus analyzer install`, **Then** the directory structure is created automatically and the installation succeeds.
3. **Given** finfocus is installed but Pulumi CLI is not found, **When** user runs `finfocus analyzer install`, **Then** the command warns about missing Pulumi but still allows installation (the plugin directory can exist without the CLI).

---

### User Story 2 - Analyzer Uninstall (Priority: P1)

A developer wants to remove the analyzer from their Pulumi plugin directory, either to troubleshoot issues or because they no longer need cost estimation during preview. They run `finfocus analyzer uninstall` and the analyzer is cleanly removed.

**Why this priority**: Install without uninstall is incomplete. Users must be able to reverse the action. Equal priority with install as they form a pair.

**Independent Test**: Can be fully tested by installing the analyzer, then running `finfocus analyzer uninstall` and verifying the plugin directory entry is removed.

**Acceptance Scenarios**:

1. **Given** the analyzer is currently installed, **When** user runs `finfocus analyzer uninstall`, **Then** the analyzer binary and its plugin directory are removed and the command reports success.
2. **Given** the analyzer is not installed, **When** user runs `finfocus analyzer uninstall`, **Then** the command reports that no installation was found (not an error, just informational).

---

### User Story 3 - Force Reinstall / Upgrade (Priority: P2)

A developer has updated finfocus to a newer version and wants the Pulumi analyzer to match. They run `finfocus analyzer install --force` to replace the existing installation with the current binary version.

**Why this priority**: Important for upgrade workflows but not needed for initial adoption. Users who have already installed once will need this when they update finfocus.

**Independent Test**: Can be fully tested by installing version A, then running `finfocus analyzer install --force` and verifying the installed version matches the current binary.

**Acceptance Scenarios**:

1. **Given** an older version of the analyzer is installed, **When** user runs `finfocus analyzer install --force`, **Then** the old installation is replaced with the current binary version.
2. **Given** the same version is already installed, **When** user runs `finfocus analyzer install` (without `--force`), **Then** the command reports the analyzer is already installed at the current version and no action is needed.
3. **Given** an older version is installed, **When** user runs `finfocus analyzer install` (without `--force`), **Then** the command reports the installed version and suggests using `--force` to upgrade.

---

### User Story 4 - Custom Target Directory (Priority: P3)

A developer has a non-standard Pulumi configuration or wants to install the analyzer to a specific location. They use `finfocus analyzer install --target-dir /custom/path` to override the default Pulumi plugin directory.

**Why this priority**: Niche use case for advanced users with custom Pulumi setups. Default behavior covers the majority of users.

**Independent Test**: Can be fully tested by running `finfocus analyzer install --target-dir /tmp/test-plugins` and verifying the binary appears at the specified location.

**Acceptance Scenarios**:

1. **Given** the user specifies a custom target directory, **When** they run `finfocus analyzer install --target-dir /custom/path`, **Then** the analyzer is installed in the specified directory instead of the default Pulumi plugin directory.
2. **Given** the specified target directory does not exist, **When** user runs `finfocus analyzer install --target-dir /new/path`, **Then** the directory is created and the installation proceeds.

---

### Edge Cases

- What happens when the user lacks write permissions to the Pulumi plugin directory? The command provides an actionable error message explaining the permission issue and suggesting remediation.
- What happens on Windows where symlinks require elevated privileges? The command falls back to copying the binary instead of symlinking.
- What happens if the finfocus binary path cannot be resolved (e.g., running from a pipe)? The command reports a clear error explaining that it cannot determine the binary location.
- What happens if the Pulumi plugin directory contains a manually installed analyzer? The `--force` flag handles this case; without it, the existing installation is reported to the user.
- What happens if disk space is insufficient during copy? The command reports the filesystem error with context about what was being attempted.
- What happens if a previous version directory exists alongside the new one? The install command places files in a version-specific directory, so old version directories are left untouched (uninstall removes the current version only).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a command to install the finfocus analyzer into the Pulumi plugin directory with a single invocation.
- **FR-002**: System MUST provide a command to remove the analyzer from the Pulumi plugin directory with a single invocation.
- **FR-003**: System MUST detect whether the analyzer is already installed and report its version.
- **FR-004**: System MUST support a `--force` flag to overwrite an existing installation without prompting.
- **FR-005**: System MUST support a `--target-dir` flag to override the default installation directory.
- **FR-006**: System MUST create the target directory structure if it does not exist.
- **FR-007**: System MUST use symlinks on Unix-like systems and file copies on Windows for the binary placement.
- **FR-008**: System MUST compare the installed version against the current binary version and report whether an update is available.
- **FR-009**: System MUST provide clear, actionable error messages for all failure modes (permission denied, missing binary path, disk full).
- **FR-010**: System MUST report the installation path and version upon successful install.
- **FR-011**: System MUST name the installed binary correctly so that Pulumi recognizes it as an analyzer plugin (following the Pulumi analyzer naming convention).
- **FR-012**: System MUST cleanly remove the entire plugin directory entry during uninstall, not just the binary file.

### Key Entities

- **Analyzer Installation**: Represents the installed analyzer binary in the Pulumi plugin directory. Key attributes: installed version, installation path, binary name, installation method (symlink or copy).
- **Installation Options**: User-configurable parameters for installation. Key attributes: force overwrite flag, custom target directory.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can set up the Pulumi analyzer in a single command invocation, reducing the setup from 4 manual steps to 1.
- **SC-002**: Users can remove the analyzer in a single command invocation with complete cleanup.
- **SC-003**: The install command works across Linux, macOS, and Windows without platform-specific user instructions.
- **SC-004**: All failure modes produce error messages that tell the user what went wrong and how to fix it.
- **SC-005**: Unit test coverage for the install/uninstall functionality is at least 80%.
- **SC-006**: Users upgrading finfocus can update the analyzer with a single `--force` reinstall.

## Assumptions

- The Pulumi plugin directory follows the standard `~/.pulumi/plugins/` convention. Users with `PULUMI_HOME` set will have their plugin directory at `$PULUMI_HOME/plugins/`.
- The analyzer binary naming convention is already established in the existing analyzer serve implementation.
- Version information is available at runtime via the build-time injected version package.
- Symlink support is available on all Unix-like systems. Windows may require copy fallback due to symlink privilege requirements.
- The uninstall command only removes files that finfocus installed (the specific plugin directory entry), not the entire Pulumi plugins directory.

## Dependencies

- Existing analyzer serve implementation for binary naming conventions.
- Version package for binary version detection.
- Existing CLI patterns for command structure and output formatting.

## Out of Scope

- Automatic detection of when the analyzer needs updating (e.g., post-upgrade hooks).
- Managing multiple installed analyzer versions simultaneously.
- Installing the analyzer system-wide (e.g., to `/usr/local/bin`).
- Integration with package managers (brew, apt, chocolatey).
- Pulumi tool plugin mode installation (separate from analyzer plugin).
