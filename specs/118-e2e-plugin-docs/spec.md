# Feature Specification: Update documentation for E2E testing and plugin ecosystem

**Feature Branch**: `118-e2e-plugin-docs`  
**Created**: 2026-01-19  
**Status**: Draft  
**Input**: User description: "Update documentation for E2E testing and plugin ecosystem..." (detailed input processed)

## Clarifications

### Session 2026-01-19
- Q: What Go version requirement should be documented? → A: Go 1.25.6+ (due to critical bug in 1.25.4)
- Q: How should the new documentation files be organized in the `docs/` directory? → A: Structure by category (e.g., `docs/testing/`, `docs/architecture/`, `docs/guides/`)
- Q: What format should be used for the architecture diagrams? → A: Mermaid diagrams (in-line code, versionable)
- Q: Should README links point to local files or the documentation site? → A: Docs site links preferred
- Q: What additional reference material should be included in the plugin documentation? → A: Plugin Compatibility Matrix (Feature support table)

## User Scenarios & Testing

### User Story 1 - E2E Testing Setup and Execution (Priority: P1)

Developers and CI engineers need a clear guide to set up and run end-to-end (E2E) tests to ensure the system is working correctly with real cloud resources.

**Why this priority**: E2E testing is critical for validating the entire system stack, but currently, users lack instructions on how to run these tests, leading to potential misuse or inability to verify changes.

**Independent Test**: Can be fully tested by following the new "E2E Testing Guide" from a clean environment and successfully executing `make test-e2e`.

**Acceptance Scenarios**:

1. **Given** a developer with Go 1.25.6+ and AWS credentials, **When** they follow the "E2E Testing Quick Start" guide, **Then** they can successfully install required plugins and run the E2E test suite.
2. **Given** an E2E test failure, **When** the user consults the "Troubleshooting" section, **Then** they find steps to interpret results (e.g., `test-results/e2e-summary.json`) and diagnose common issues (e.g., credential errors).

---

### User Story 2 - Understanding Plugin Ecosystem Architecture (Priority: P1)

New contributors and advanced users need to understand how the Core, Public Plugin, and CostExplorer Plugin interact to effectively use and extend the platform.

**Why this priority**: The relationship between components (Core vs. Plugins, Public vs. CostExplorer) is complex. clear documentation is required to prevent architectural confusion and ensure users choose the right plugin for their needs.

**Independent Test**: Can be tested by reviewing the new "Plugin Ecosystem" documentation and verifying it accurately describes the current system architecture (Core -> gRPC -> Plugins).

**Acceptance Scenarios**:

1. **Given** a user exploring the project, **When** they read the "Architecture Overview", **Then** they can identify the role of the Core Engine, Plugin Host, and Registry.
2. **Given** a user needing real billing data, **When** they read the "Plugin Relationships" section, **Then** they clearly understand the difference between the fallback Public Plugin (list prices) and the CostExplorer Plugin (actual billing).

---

### User Story 3 - Troubleshooting & Common Issues (Priority: P2)

Users encountering installation or runtime errors need a centralized resource to resolve common problems quickly without filing duplicate issues.

**Why this priority**: Reduces support burden and improves user experience by enabling self-service resolution of known issues.

**Independent Test**: Verify that the "Troubleshooting" guide covers specific scenarios like "Plugin Installation Failures" and "AWS Credential Problems" with actionable solutions.

**Acceptance Scenarios**:

1. **Given** a user with a failed plugin installation, **When** they search the documentation, **Then** they find specific steps to debug and resolve the issue.

## Requirements

### Functional Requirements

- **FR-001**: The documentation MUST include a comprehensive "E2E Testing Guide" covering prerequisites, plugin installation, test execution (`make test-e2e`), and result interpretation.
- **FR-002**: The documentation MUST describe the plugin ecosystem architecture, including the relationship between `pulumicost-core`, `pulumicost-plugin-aws-public` (fallback), and `pulumicost-plugin-aws-costexplorer`.
- **FR-003**: The documentation MUST explain the cost calculation workflow for both Projected Costs (pre-deployment list prices) and Actual Costs (post-deployment runtime/billing data).
- **FR-004**: The `README.md` for `pulumicost-core` MUST be updated to include a summary of E2E testing, plugin ecosystem links, and an architecture diagram using Mermaid.
- **FR-005**: The `README.md` for `pulumicost-plugin-aws-public` MUST be updated to clarify its role as a fallback/public-pricing plugin and its relationship to the core.
- **FR-006**: The documentation MUST include a "Troubleshooting" section addressing common issues: Plugin Installation Failures, AWS Credential Problems, Cost Calculation Errors, and E2E Test Timeouts.
- **FR-007**: The documentation MUST provide a "Quick Start" for E2E testing that assumes a clean environment (Go 1.25.6+, AWS creds).
- **FR-008**: Documentation files MUST be organized by category within the `docs/` directory (e.g., `docs/testing/e2e-guide.md`, `docs/architecture/plugin-ecosystem.md`, `docs/guides/troubleshooting.md`).
- **FR-009**: All architecture diagrams MUST be implemented using Mermaid formatting within the Markdown files.
- **FR-010**: Links in `README.md` files SHOULD point to the hosted documentation site where available, with relative local links as fallbacks.
- **FR-011**: The documentation MUST include a "Plugin Compatibility Matrix" summarizing feature support (E2E, Actual Cost, etc.) across available plugins.

### Key Entities

- **Documentation Artifacts**: Markdown files organized by category (`docs/**/*.md`) serving as the source of truth.
- **Architecture Diagrams**: Visual representations implemented via Mermaid in-line code.
- **Compatibility Matrix**: A structured table mapping plugins to supported capabilities.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A new user can successfully run E2E tests (`make test-e2e`) within 10 minutes by following the "E2E Testing Quick Start" guide (assuming prerequisites are met).
- **SC-002**: The `README.md` files for both Core and AWS Public Plugin are updated to reference the new ecosystem and testing documentation.
- **SC-003**: A new "Troubleshooting" guide exists and covers at least 4 common failure scenarios (Installation, Creds, Calculation, Timeouts).
- **SC-004**: Architecture documentation clearly defines the specific roles of "Core", "Public Plugin", and "CostExplorer Plugin" without ambiguity.
