# Feature Specification: v0.2.1 Developer Experience Improvements

**Feature Branch**: `115-v021-dx-improvements`
**Created**: 2026-01-17
**Status**: Draft
**Input**: User description provided via CLI

## Clarifications

### Session 2026-01-17

- Q: What is the concurrency limit for plugin metadata fetching? → A: Use the same parallelism constant used for core engine queries (e.g., matching `NumCPU` or the standard worker pool size) to ensure consistent resource usage.
- Q: When using `--clean`, which versions are removed? → A: Remove **all** installed versions of that plugin except the one just successfully installed.
- Q: How should legacy plugins (no GetPluginInfo) be indicated in the plugin list? → A: Show "Legacy" (or "N/A") inline in the Spec Version column.
- Q: Where should the shared filter application helper reside? → A: Inside the `internal/cli` package (e.g., `internal/cli/filters.go`) to stay close to its primary consumers.
- Q: How strictly should plugin compatibility be enforced? → A: Check automatically, but only **WARN** on mismatch by default (Permissive), while supporting a `--strict` flag (or config) to enforce blocking.

## User Scenarios & Testing

### User Story 1 - Fast Plugin Listing (Priority: P1)

As a user with multiple plugins installed, I want the `plugin list` command to return results quickly, regardless of how many plugins I have, so that I can check their status without excessive waiting.

**Why this priority**: Slow CLI performance degrades the user experience and makes the tool feel sluggish, especially as the ecosystem grows.

**Independent Test**: Install 5+ mock plugins with artificial delays (e.g., 1s each). Run `finfocus plugin list`. The command should complete in approximately the time of a single delay (plus overhead), not the sum of all delays.

**Acceptance Scenarios**:

1. **Given** 5 plugins installed that each take 1s to respond, **When** I run `finfocus plugin list`, **Then** the command completes in under 2 seconds (vs ~5s serially).
2. **Given** plugins installed in any order, **When** I run `finfocus plugin list`, **Then** the output is deterministically sorted (by name).

### User Story 2 - Plugin Compatibility & Diagnostics (Priority: P1)

As an administrator, I want to see the specification version and compatibility status of my installed plugins, so I can identify outdated or incompatible extensions.

**Why this priority**: Essential for ecosystem stability and troubleshooting as the plugin spec evolves.

**Independent Test**: Install a mix of "legacy" (no GetPluginInfo) and "modern" (with GetPluginInfo) plugins. Run `plugin list`.

**Acceptance Scenarios**:

1. **Given** a plugin that implements `GetPluginInfo`, **When** I run `finfocus plugin list`, **Then** the "Spec Version" column displays the reported version.
2. **Given** a legacy plugin (no `GetPluginInfo`), **When** I run `finfocus plugin list`, **Then** the "Spec Version" column displays "Legacy".
3. **Given** a plugin returning invalid metadata, **When** the system initializes it, **Then** it is marked as failed/invalid and does not load.

### User Story 3 - Automatic Cleanup of Old Plugins (Priority: P2)

As a user, I want the option to automatically remove old versions of a plugin when I successfully install a new one, so that my disk space isn't wasted by unused binaries.

**Why this priority**: Prevents "bit rot" and disk bloat on user machines.

**Independent Test**: Install v1 of a plugin, then install v2 with the cleanup flag. Check the filesystem.

**Acceptance Scenarios**:

1. **Given** multiple older versions of `aws-public` (e.g., v0.0.5, v0.0.6) are installed, **When** I run `finfocus plugin install aws-public --force --clean` (installing v0.0.7), **Then** ALL older versions (v0.0.5, v0.0.6) are removed from the filesystem after v0.0.7 is successfully installed.
2. **Given** the installation of v0.0.7 fails, **When** I specified `--clean`, **Then** all existing versions are preserved (rollback/safety).

### User Story 4 - Consistent Cost Filtering (Priority: P3)

As a user, I expect filter flags to behave exactly the same way across all cost analysis commands, so that I can rely on my query results.

**Why this priority**: Ensures data integrity and consistent user experience across the CLI suite.

**Independent Test**: Apply a specific filter to `cost actual` and `cost projected` and verify identical validation/filtering logic is applied.

**Acceptance Scenarios**:

1. **Given** a filter string, **When** I use it with `cost actual`, **Then** it filters resources using the standard validation logic.
2. **Given** the same filter string, **When** I use it with `cost projected`, **Then** it produces strictly consistent results using the same logic.

### Edge Cases

- **Slow Plugin**: A plugin hangs during metadata retrieval. The system must enforce a strict timeout (e.g., 5s) and not block the entire list command indefinitely.
- **Invalid Metadata**: A plugin returns malformed JSON/Protobuf. The system must mark it as "Invalid" or "Error" in the list, rather than crashing.
- **Concurrent Install/List**: User runs `plugin list` while `plugin install` is modifying the directory. The system should handle file locking or race conditions gracefully (e.g., by skipping the changing plugin or retrying).
- **Network Failure during Install**: If `plugin install` fails due to network, the cleanup of the *old* version MUST NOT happen (rollback safety).

## Requirements

### Functional Requirements

- **FR-001**: The system MUST fetch metadata for installed plugins concurrently, utilizing the application-standard parallelism limit (e.g., matching the engine's `NumCPU`-based worker pool) to minimize total execution time.
- **FR-002**: The `plugin list` command MUST display a "Spec Version" column for each plugin.
- **FR-003**: The system MUST attempt to query plugin metadata (`GetPluginInfo`) upon initialization with a timeout (default 5s).
- **FR-004**: The system MUST validate plugin compatibility (Spec Version). By default, mismatches MUST log a warning but allow initialization (Permissive). The system MUST support a configuration to enforce strict blocking of incompatible plugins.
- **FR-005**: The system MUST handle legacy plugins (missing `GetPluginInfo`) gracefully, indicating "Legacy" in the CLI output while allowing operation if strictly compatible.
- **FR-006**: The `plugin install` command MUST support a `--clean` flag that removes **all** other installed versions of the specific plugin upon successful installation.
- **FR-007**: The system MUST use a unified logic for validating and applying resource filters across all cost-related commands (`actual`, `projected`).

### Key Entities

- **Plugin Metadata**: Includes Name, Version, Spec Version, Supported Clouds.
- **Filter Expression**: A string defining criteria for including/excluding resources.

### Dependencies

- **finfocus-spec v0.5.1**: The Core system requires the updated specification definition that includes the `GetPluginInfo` RPC.

## Success Criteria

### Measurable Outcomes

- **SC-001**: `plugin list` execution time scales with the maximum single plugin latency (O(1) relative to count) rather than the sum of latencies (O(N)), up to a concurrency limit.
- **SC-002**: 100% of installed plugins display their Spec Version (or "Legacy") in the CLI list.
- **SC-003**: Plugin upgrades with the clean option result in 0 bytes of disk space used by **any** previous versions of that plugin.
- **SC-004**: Code duplication for filter logic is reduced to zero (single shared implementation).
