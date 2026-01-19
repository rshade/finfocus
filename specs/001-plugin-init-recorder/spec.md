# Feature Specification: Plugin Init Recorder Fixtures

**Feature Branch**: `001-plugin-init-recorder`  
**Created**: 2026-01-19  
**Status**: Draft  
**Input**: User description: "feat(cli): Enhance 'plugin init' with Remote Plan Sourcing and Recorder RPC Updates"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Generate Recorded Fixtures on Init (Priority: P1)

As a plugin author, I want `plugin init` to produce recorded request fixtures from real plans so that my new plugin starts with accurate, current data shapes.

**Why this priority**: Without real recorded fixtures, new plugins cannot validate against the core system's current request payloads.

**Independent Test**: Can be fully tested by running `plugin init` and verifying that recorded request files are produced in the new plugin's testdata directory.

**Acceptance Scenarios**:

1. **Given** a valid plugin name and network access, **When** the user runs plugin initialization, **Then** recorded requests for projected cost, actual cost, and recommendations exist in the generated project.
2. **Given** a successful initialization, **When** the user inspects the generated testdata directory, **Then** each recorded request is labeled by request type and includes the captured payload.

---

### User Story 2 - Offline Initialization Fallback (Priority: P2)

As a plugin author working offline, I want `plugin init` to complete using local fixtures so that I can still generate starter data without network access.

**Why this priority**: Offline creation is a common scenario for developers and should not block initial project setup.

**Independent Test**: Can be fully tested by running initialization with offline mode enabled and confirming fixture generation uses local sources.

**Acceptance Scenarios**:

1. **Given** offline mode is enabled, **When** the user runs plugin initialization, **Then** the process completes without remote fetch attempts and uses locally available fixtures.

---

### User Story 3 - Recorder Supports Current Request Types (Priority: P3)

As a core maintainer, I want the recorder plugin to handle all current request types so that contract validation captures the same requests core sends to real plugins.

**Why this priority**: Missing RPCs create gaps in recorded data and reduce confidence in plugin compatibility.

**Independent Test**: Can be fully tested by invoking each request type and confirming that a recorded request is created for every supported method.

**Acceptance Scenarios**:

1. **Given** the recorder plugin is installed, **When** core invokes projected cost, actual cost, recommendations, and plugin info requests, **Then** each request is recorded without error.

---

### Edge Cases

- Remote plan sources are unavailable or return invalid data.
- Recorder output directory is not writable during initialization.
- Offline mode is enabled but local fixture files are missing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST fetch canonical plan and state fixtures from the official finfocus repository during plugin initialization when online mode is active.
- **FR-002**: The system MUST support pinning fetched fixtures to a specified release version.
- **FR-003**: The system MUST provide an offline initialization mode that uses locally available fixtures without remote network access.
- **FR-004**: The system MUST run contract validation workflows during initialization and record the resulting request payloads in the new plugin's testdata directory.
- **FR-005**: The recorder plugin MUST record requests for projected cost, actual cost, recommendations, and plugin info request types.
- **FR-006**: The recorder plugin MUST provide deterministic mock responses for recommendations when mock mode is enabled.
- **FR-007**: The system MUST present clear progress updates and a completion summary during the recording workflow.
- **FR-008**: The system MUST surface actionable error messages when fixture retrieval or recording fails.

### Requirement Acceptance

- **FR-001** is satisfied when online initialization retrieves canonical fixtures and reports successful acquisition.
- **FR-002** is satisfied when initialization can use a specified release version for fixture sources.
- **FR-003** is satisfied when offline mode completes without attempting remote retrieval and uses local fixtures.
- **FR-004** is satisfied when initialization finishes with recorded request files written to the generated testdata directory.
- **FR-005** is satisfied when each required request type produces a recorded request file.
- **FR-006** is satisfied when recommendation responses remain consistent between repeated runs in mock mode.
- **FR-007** is satisfied when users see progress indicators for fetching, recording, and completion.
- **FR-008** is satisfied when failures include a clear reason and suggested remediation.

### Key Entities

- **Fixture Source**: The canonical plan or state data used to drive recording, including source location and version.
- **Recording Session**: A single initialization run that captures requests, including timestamps and status.
- **Recorded Request**: The captured request payload for a specific request type, stored for plugin testing.
- **Plugin Init Output**: The generated project directory including recorded request artifacts.

### Assumptions

- The official finfocus repository maintains canonical fixtures for supported plan and state formats.
- Plugin authors want recorded request data created during initialization rather than manual capture.

### Dependencies

- finfocus-spec v0.5.x provides the authoritative request definitions used during recording.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of successful plugin initializations produce recorded request files for projected cost, actual cost, and recommendations.
- **SC-002**: Offline initialization completes without network access and produces a minimum of three recorded request files.
- **SC-003**: 90% of plugin authors can complete initialization and locate recorded fixtures on the first attempt.
- **SC-004**: Fixture retrieval and recording completes within 5 minutes for 95% of runs under normal network conditions.
