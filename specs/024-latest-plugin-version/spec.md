# Feature Specification: Latest Plugin Version Selection

**Feature Branch**: `001-latest-plugin-version`
**Created**: 2025-01-05
**Status**: Draft
**Input**: Issue #140: "Registry should pick latest version when multiple plugin versions installed"

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Automated Cost Analysis Uses Latest Plugin Version (Priority: P1)

When a user runs a cost analysis command, the system automatically selects and uses the latest version of each installed plugin, regardless of how many versions are installed.

**Why this priority**: This is the core issue preventing duplicate cost calculations and ensures accurate results. Without this fix, users get incorrect cost data when they have multiple plugin versions installed.

**Independent Test**: Can be fully tested by installing multiple versions of the same plugin and running a cost analysis, then verifying that only the latest version is used in calculations.

**Acceptance Scenarios**:

1. **Given** multiple versions of the same plugin are installed (e.g., v1.0.0 and v2.0.0), **When** user runs cost analysis, **Then** only the latest version (v2.0.0) is used for calculations
2. **Given** only one version of a plugin is installed, **When** user runs cost analysis, **Then** that single version is used
3. **Given** three versions of a plugin are installed (v1.0.0, v1.1.0, v2.0.0), **When** user runs cost analysis, **Then** v2.0.0 is used (highest semver)
4. **Given** multiple plugins with different names are installed, **When** user runs cost analysis, **Then** the latest version of each plugin is selected independently

---

### User Story 2 - View All Installed Plugin Versions (Priority: P2)

When a user wants to see all installed plugins, they can list all installed versions of each plugin for visibility and management purposes.

**Why this priority**: While not as critical as preventing duplicate calculations, this visibility is important for users to understand their plugin inventory and manage versions effectively.

**Independent Test**: Can be fully tested by installing multiple versions of the same plugin and running the plugin list command, then verifying all versions are displayed.

**Acceptance Scenarios**:

1. **Given** multiple versions of the same plugin are installed (e.g., v1.0.0 and v2.0.0), **When** user runs plugin list command, **Then** both versions are displayed with their version numbers
2. **Given** multiple plugins with different names are installed, **When** user runs plugin list command, **Then** each plugin and all its versions are displayed
3. **Given** no plugins are installed, **When** user runs plugin list command, **Then** a clear message indicates no plugins are installed

---

### Edge Cases

- What happens when version strings are invalid or malformed? (should be handled gracefully with clear error messages)
- What happens when two versions have identical version strings? (should be treated as the same plugin)
- What happens when pre-release versions (e.g., v2.0.0-alpha) are present? (semver standard determines precedence: stable > pre-release)
- When a plugin directory structure is corrupted, it is skipped with a warning message showing the directory path and error description, while other plugins continue processing normally
- When multiple plugins have the same name but are in different locations, all are treated as duplicates and the single latest version across all locations is selected without warning
- File system failure modes: permission denied (continue with warning), disk full (fail with error), network path errors (continue with warning)

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST automatically select the latest version when multiple versions of the same plugin are installed for cost analysis operations
- **FR-002**: System MUST compare plugin versions using semantic versioning (semver) standards to determine the latest version
- **FR-003**: System MUST display all installed versions when users request a list of installed plugins
- **FR-004**: System MUST prevent duplicate cost calculations by ensuring only one instance of each plugin is used
- **FR-005**: System MUST handle invalid or malformed version strings with clear error messages, and skip corrupted plugin directories while continuing to process other plugins with warnings
- **FR-008**: System MUST validate plugin binary existence and basic metadata file presence as validity criteria beyond version format
- **FR-009**: System MUST support concurrent read-only operations for multiple cost analysis processes
- **FR-006**: System MUST treat pre-release versions according to semver precedence rules (stable versions preferred over pre-release)
- **FR-007**: System MUST provide a single source of truth for the "latest" version across all plugin operations that require version selection, treating plugins with same name in different locations as duplicates and selecting the single latest version

### Key Entities

- **Plugin**: Represents a cost analysis plugin with attributes for name (identifies plugin type), version (semantic version string), and installation location (file system path)
- **PluginRegistry**: Manages the collection of available plugins and provides operations for listing and selecting plugins based on version

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: Cost analysis results are accurate with no duplicate calculations when multiple plugin versions are installed
- **SC-002**: Plugin list command displays 100% of installed versions for each plugin
- **SC-003**: Version selection follows semver standards with 100% accuracy (verified by test cases)
- **SC-004**: System handles edge cases (invalid versions, pre-releases, malformed directories) without crashes
- **SC-005**: Plugin discovery performance: <50ms for <100 plugins, <200ms for 100-500 plugins, <500ms for 500+ plugins

## Clarifications

### Session 2025-01-05

- Q: How should plugins with same name in different locations be handled? → A: Treat as duplicates across all locations and select single latest version, with no warning to user
- Q: When a corrupted plugin directory is encountered, what should the system do and what should the user see? → A: Skip corrupted directory, continue with warning message showing path and error, process other plugins normally
- Q: What performance targets should be defined for plugin discovery and version selection operations? → A: Define specific latency targets for small (<100 plugins), medium (100-500), and large (500+) plugin inventories
- Q: What file system failure modes should be explicitly handled beyond corrupted plugin directories? → A: Handle permission denied, disk full, and network path errors specifically
- Q: What validation criteria should be used beyond version format to determine if a plugin is valid? → A: Check for plugin binary existence and basic metadata file presence
- Q: Should plugin operations support concurrent access from multiple cost analysis processes? → A: Yes, support concurrent read-only operations

## Assumptions

- The project already uses semver v3 library for version comparisons
- Users may have multiple plugin versions installed due to testing or migration purposes
- The plugin directory structure follows the established pattern: `~/.finfocus/plugins/<plugin-name>/<version>/`
- Plugin names are case-sensitive identifiers
- Users primarily care about cost analysis accuracy, not the technical details of version selection

## Out of Scope

- Manual version selection via command-line flags (deferred to future enhancement)
- Plugin update notifications or automatic updates
- Plugin dependency management (one plugin requiring another)
- Plugin conflict resolution beyond version selection
