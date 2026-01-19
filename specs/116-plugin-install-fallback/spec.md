# Feature Specification: Plugin Install Version Fallback

**Feature Branch**: `116-plugin-install-fallback`
**Created**: 2026-01-18
**Status**: Draft
**Input**: GitHub Issue #430 - Feature Request: Fallback to latest stable version when plugin asset is missing

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Interactive Fallback on Missing Assets (Priority: P1)

A user attempts to install a specific plugin version (e.g., `aws-public@v0.1.3`) during a release "build window" where the tag exists but binaries are not yet uploaded. Instead of failing immediately, the system offers to install the latest available stable version.

**Why this priority**: This is the core use case from the feature request. Users are frustrated when they cannot install a plugin just because the newest version's assets aren't ready yet, especially when a working previous version exists.

**Independent Test**: Can be fully tested by attempting to install a version without assets and verifying the user receives a prompt with fallback options, and upon acceptance, the latest stable version is installed.

**Acceptance Scenarios**:

1. **Given** a user runs `finfocus plugin install aws-public@v0.1.3` in interactive mode, **When** v0.1.3 exists but has no compatible assets, **Then** the system displays a warning message showing the missing version and prompts the user to install the latest stable version (e.g., v0.1.2) with a Y/n confirmation.

2. **Given** the user accepts the fallback prompt, **When** the system proceeds with installation, **Then** the latest stable version with compatible assets is installed and the user is informed of the actual installed version.

3. **Given** the user declines the fallback prompt, **When** the system receives the "n" response, **Then** installation is aborted with a clear message indicating no installation occurred.

---

### User Story 2 - Automated Fallback with CLI Flag (Priority: P2)

A CI/CD pipeline or automated script needs to install a plugin without interactive prompts. When the requested version lacks assets, the system automatically falls back to the latest stable version if the appropriate flag is provided.

**Why this priority**: Automation support is critical for production deployments and CI pipelines where interactive prompts are not possible.

**Independent Test**: Can be fully tested by running `finfocus plugin install aws-public@v0.1.3 --fallback-to-latest` and verifying that when v0.1.3 lacks assets, the latest stable version is automatically installed without prompting.

**Acceptance Scenarios**:

1. **Given** a user runs `finfocus plugin install aws-public@v0.1.3 --fallback-to-latest`, **When** v0.1.3 exists but has no compatible assets, **Then** the system automatically installs the latest stable version without prompting, with output indicating the fallback occurred.

2. **Given** a user runs `finfocus plugin install aws-public@v0.1.3` without the flag in a non-TTY environment (e.g., CI), **When** v0.1.3 lacks assets, **Then** the installation fails with the existing error message (preserving current behavior when fallback is not explicitly enabled).

---

### User Story 3 - Explicit Fallback Disable (Priority: P3)

A user wants strict version control and wants to ensure that only the exact requested version is installed, with no automatic fallback behavior even if prompted.

**Why this priority**: Some deployment scenarios require exact version pinning for compliance or reproducibility.

**Independent Test**: Can be fully tested by running `finfocus plugin install aws-public@v0.1.3 --no-fallback` and verifying the installation fails immediately when assets are missing, without any fallback attempt.

**Acceptance Scenarios**:

1. **Given** a user runs `finfocus plugin install aws-public@v0.1.3 --no-fallback`, **When** v0.1.3 lacks assets, **Then** the installation fails immediately with a clear error, without offering or attempting fallback.

---

### Edge Cases

- What happens when no stable versions exist at all (brand new plugin)?
  - The system fails with a message indicating no stable versions are available.

- What happens when the latest stable version also lacks compatible assets for the user's platform?
  - The system iterates through multiple stable versions (up to 10) before failing.

- What happens when the user specifies `@latest` and the latest version lacks assets?
  - Fallback applies, trying the next most recent stable version.

- What happens in a non-TTY environment without the `--fallback-to-latest` flag?
  - Current behavior is preserved: fail with the existing error message.

- How does fallback interact with the existing `--force` flag?
  - Fallback works independently of force; force applies to overwriting existing installations of whatever version is ultimately installed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST detect when a requested plugin version exists but lacks compatible platform assets.

- **FR-002**: System MUST prompt interactive users to accept fallback to the latest stable version when the requested version lacks assets (unless `--no-fallback` is specified). The prompt MUST default to "No" (abort) when the user presses Enter without explicit input.

- **FR-003**: System MUST provide a `--fallback-to-latest` flag that enables automatic fallback without prompting in non-interactive environments.

- **FR-004**: System MUST provide a `--no-fallback` flag that disables fallback behavior entirely.

- **FR-005**: System MUST clearly communicate the fallback situation to users, including:
  - The original requested version and why it failed
  - The version being offered as fallback
  - Clear indication after installation of what version was actually installed

- **FR-006**: System MUST iterate through multiple stable releases (up to 10) when searching for a version with compatible assets.

- **FR-007**: System MUST preserve existing error behavior when fallback is not enabled and assets are missing.

- **FR-008**: System MUST ensure the `--fallback-to-latest` and `--no-fallback` flags are mutually exclusive.

### Key Entities

- **Plugin Version**: A tagged release of a plugin that may or may not have platform-specific binary assets available.

- **Stable Release**: A non-draft, non-prerelease GitHub release that is a candidate for fallback.

- **Platform Asset**: A downloadable binary artifact matching the user's operating system and architecture.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can successfully install plugins even when the specifically requested version lacks assets, by accepting a fallback to the latest stable version, within the same time frame as a normal installation plus confirmation time.

- **SC-002**: CI/CD pipelines can continue operating without interruption during release build windows by using the `--fallback-to-latest` flag.

- **SC-003**: 100% of interactive fallback scenarios display the warning message, fallback version, and confirmation prompt before proceeding.

- **SC-004**: Automated installations with `--fallback-to-latest` complete without human intervention while still providing informational output about the fallback.

- **SC-005**: Users who require strict version control can disable fallback with `--no-fallback` and receive immediate failure feedback.

## Clarifications

### Session 2026-01-18

- Q: What is the default behavior when user presses Enter without typing Y or n at the fallback prompt? → A: Default to "No" (abort installation)

## Assumptions

- The GitHub API continues to provide release and asset metadata in its current format.
- Interactive mode detection relies on standard TTY detection mechanisms.
- The existing `GitHubClient.ListStableReleases()` and `FindReleaseWithAsset()` functions provide the foundation for finding fallback candidates.
- Users understand that accepting a fallback means they may receive a different version than originally requested.
- The 10-release limit for fallback searches is sufficient for typical release cadences.
