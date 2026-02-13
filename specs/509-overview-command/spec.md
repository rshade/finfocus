# Feature Specification: Unified Cost Overview Dashboard

**Feature Branch**: `509-overview-command`  
**Created**: 2026-02-11  
**Status**: Draft  
**Input**: User description: "Add a top-level `finfocus overview` command that provides a unified, interactive dashboard combining actual costs, projected costs, and recommendations for every resource in a Pulumi stack."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Unified Stack Dashboard (Priority: P1)

As a Cloud Engineer, I want a single command that shows me everything I need to know about my stack's costs—what I'm currently paying, what my upcoming changes will cost, and how I can save money—so I don't have to run multiple separate commands and manually correlate the data.

**Why this priority**: This is the "flagship" experience and the primary value proposition of the feature. It solves the fragmentation problem.

**Independent Test**: Can be tested by running `finfocus overview` on a stack with both existing resources and pending changes, verifying that both actual and projected costs appear in a single view.

**Acceptance Scenarios**:

1. **Given** a Pulumi stack with 10 existing resources and 2 pending updates, **When** I run `finfocus overview`, **Then** I see a list of 10 resources where 2 show both actual (old) and projected (new) costs.
2. **Given** a stack with no pending changes, **When** I run `finfocus overview`, **Then** the system detects zero changes and optimizes by querying only actual costs and recommendations.

---

### User Story 2 - Progressive Data Loading (Priority: P2)

As a user with a large stack (100+ resources), I want the dashboard to open immediately and show me data as it's fetched, rather than waiting for all API calls to finish, so I can start analyzing my infrastructure without delay.

**Why this priority**: Essential for usability on medium-to-large environments where full data retrieval can take 30-60 seconds.

**Independent Test**: Run the command on a stack with many resources and verify that the interface appears instantly with "loading" indicators that are replaced by data row-by-row or cell-by-cell.

**Acceptance Scenarios**:

1. **Given** a stack requiring many API calls, **When** the command starts, **Then** an interactive interface appears immediately with partial results displayed as they arrive and a persistent progress banner (e.g., "Loading: 45/100 resources") shown until all data is fetched.
2. **Given** data is still being fetched, **When** a resource's cost is retrieved, **Then** its row appears in the table with actual data values (not placeholders) without flickering the rest of the UI.
3. **Given** the progress banner is visible, **When** all resource data has been loaded, **Then** the banner is removed and the table shows the complete dataset.
4. **Given** a stack with more than 250 resources, **When** the overview loads, **Then** pagination mode is automatically enabled to prevent UI clutter, showing 250 resources per page with navigation controls.

---

### User Story 3 - Resource Cost Drill-down (Priority: P2)

As a FinOps analyst, I want to select a specific resource from the overview and see a detailed breakdown of its costs and specific optimization recommendations, so I can understand *why* a resource is expensive and *how* to fix it.

**Why this priority**: Provides the "actionable" part of the "what should I fix?" motivation.

**Independent Test**: Navigate to a resource in the table, press Enter, and verify that a detail view appears with breakdowns for actual cost, projected cost, and a list of specific recommendations.

**Acceptance Scenarios**:

1. **Given** I am viewing the overview table, **When** I select an EC2 instance and press Enter, **Then** I see a breakdown of its compute, storage, and network costs.
2. **Given** I am in the detail view, **When** I press Escape, **Then** I am returned to the main overview table with my previous position and filter preserved.

---

### User Story 4 - Non-Interactive Overview (Priority: P3)

As a DevOps Engineer, I want to see the same unified cost overview in my terminal or CI pipeline without interactivity, so I can get a quick snapshot of stack health in text format.

**Why this priority**: Ensures the tool is useful in automated environments and for users who prefer static output.

**Independent Test**: Run `finfocus overview --plain` or pipe the output to a file and verify a formatted text table is produced.

**Acceptance Scenarios**:

1. **Given** a non-interactive terminal (TTY-less), **When** I run `finfocus overview`, **Then** the system automatically defaults to a static table output.
2. **Given** the `--plain` flag is used, **When** the command finishes, **Then** a summary of MTD actual costs and projected deltas is printed at the bottom of the table.

### Edge Cases

- **Partial Failures**: If the actual cost API fails for one resource, the system MUST prompt the user interactively with 'API call failed for resource X. Retry? [y/n/skip]'. If user selects 'n' (no), the row displays an error indicator. If 'skip', the resource is excluded from the view. If 'y' (yes), the API call is retried once.
- **New Resources**: Resources that exist only in the preview (not yet created) must be clearly marked as "creating" and show no actual costs.
- **Deleted Resources**: Resources being deleted should show their historical actual cost but $0.00 for projected cost.
- **Empty Stacks**: How does the system handle a stack with zero resources? (Should show a friendly "No resources found" message).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a top-level `finfocus overview` command.
- **FR-002**: The system MUST merge Pulumi state (ground truth) and Pulumi preview (pending changes) into a single resource list.
- **FR-003**: The system MUST calculate actual costs for the current Month-to-Date (MTD) period by default.
- **FR-004**: The system MUST highlight "cost drift" when extrapolated actual monthly costs differ from projected costs by more than 10%.
- **FR-005**: The system MUST require a pre-flight confirmation prompt showing the number of resources and plugins before making expensive API calls, unless bypassed with a flag.
- **FR-006**: The system MUST support filtering the resource list by name or type.
- **FR-007**: The system MUST display a "projected delta" representing the net change in monthly spend if the pending infrastructure changes are applied.
- **FR-008**: The system MUST detect when no changes are pending in a preview and optimize the execution by skipping the projected cost pipeline.
- **FR-009**: The system MUST automatically enable pagination mode when displaying more than 250 resources to maintain UI responsiveness and readability.
- **FR-010**: The system MUST authenticate to cloud provider APIs using standard SDK credential chains (AWS profiles via AWS_PROFILE/~/.aws/credentials, Azure CLI via `az account`, GCP Application Default Credentials) without storing credentials internally.
- **FR-011**: The system MUST display resources in the order they appear in the Pulumi state file, preserving the natural ordering from the infrastructure definition.

### Key Entities *(include if feature involves data)*

- **Overview Row**: A unified representation of a single infrastructure resource, containing its current status (active, changing, deleting), its actual historical cost, its projected future cost, and any associated recommendations.
- **Cost Drift**: A derived state indicating a significant discrepancy between predicted costs and observed actual spending.
- **Stack Context**: The metadata identifying the Pulumi stack, region, and the time window being analyzed.

## Clarifications

### Session 2026-02-11

- Q: For large stacks with slow-loading cost data, should the UI display a loading indicator and wait for all data before showing the table, display partial results as they arrive with a progress banner, or show an empty table skeleton that fills in cell-by-cell? → A: Display partial results as they arrive, with a persistent banner showing 'Loading: 45/100 resources' until complete.
- Q: When an API call fails for a specific resource during data loading, what should the system do? → A: Prompt user interactively: 'API call failed for resource X. Retry? [y/n/skip]'
- Q: For very large stacks, should the system render unlimited resources in the interactive table, limit to 100 and show 'use --filter', limit to 250 and auto-enable pagination beyond, or limit to 500 before requiring pagination? → A: Up to 250 resources; beyond this, auto-enable pagination mode.
- Q: How should the tool authenticate to cloud provider cost APIs? → A: Use standard cloud SDK credential chains (AWS profiles, Azure CLI, GCP ADC) with no credential storage by the tool.
- Q: In what order should resources be displayed in the overview table? → A: Resources should be displayed in the order they appear in the Pulumi state file (preserving the natural ordering from the infrastructure definition).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view a complete cost profile (actual + projected + recs) in a single screen without executing multiple commands.
- **SC-002**: Initial UI render occurs in under 500ms, regardless of stack size, using progressive loading.
- **SC-003**: 100% of resources in a stack are correctly categorized into one of the unified statuses (active, creating, updating, deleting, replacing).
- **SC-004**: Cost drift is accurately identified for resources where actual spending exceeds projected estimates by the 10% threshold.
- **SC-005**: The tool correctly calculates the net "Projected Delta" for a stack update, accounting for both additions and removals.
