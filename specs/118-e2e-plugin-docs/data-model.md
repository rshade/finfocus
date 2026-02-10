# Phase 1: Document Model & Structure

**Feature**: Update documentation for E2E testing and plugin ecosystem
**Branch**: `118-e2e-plugin-docs`

## Document Entities (Markdown Files)

### 1. E2E Testing Guide (`docs/testing/e2e-guide.md`)
- **Title**: E2E Testing Guide
- **Audience**: Developers, CI Engineers
- **Key Sections**:
  - Prerequisites (Go 1.25.7+, AWS Creds)
  - Quick Start
  - Running Tests (`make test-e2e`)
  - Test Scenarios (Projected, Actual, Validation, Cleanup)
  - Troubleshooting specific to tests

### 2. Plugin Ecosystem Architecture (`docs/architecture/plugin-ecosystem.md`)
- **Title**: Plugin Ecosystem & Architecture
- **Audience**: Architects, Contributors
- **Key Sections**:
  - System Overview (Core -> Plugin Host -> Plugins)
  - Mermaid Diagram (Component Interaction)
  - Plugin Types (Public/Fallback vs. CostExplorer/Production)
  - Data Flow

### 3. Troubleshooting Guide (`docs/guides/troubleshooting.md`)
- **Title**: Troubleshooting Common Issues
- **Audience**: All Users
- **Key Sections**:
  - Installation Failures
  - AWS Credential Issues
  - Cost Calculation Errors
  - E2E Test Timeouts

### 4. Plugin Compatibility Matrix (`docs/reference/plugin-compatibility.md`)
- **Title**: Plugin Compatibility Matrix
- **Audience**: Users choosing a plugin
- **Key Sections**:
  - Feature Support Table (Columns: Plugin, E2E, Projected, Actual, Auth)

## Updates to Existing Files

### `README.md` (Core)
- Update "Key Features" to mention E2E testing.
- Add "Documentation" section linking to the new guides.
- Add "Architecture" subsection with a high-level Mermaid diagram.

### `docs/index.md`
- Add links to new guides in the relevant sections.
