# Feature Specification: Cost Estimate Command for What-If Scenario Modeling

**Feature Branch**: `223-cost-estimate`
**Created**: 2026-02-02
**Status**: Draft
**Input**: User description: "Add cost estimate command for what-if scenario modeling - GitHub Issue #463"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Quick Property Changes (Priority: P1)

A developer wants to quickly evaluate the cost impact of changing a single resource configuration, such as changing an EC2 instance type from `t3.micro` to `m5.large`, without modifying any Pulumi code.

**Why this priority**: This is the most common use case - rapid "what-if" cost exploration during decision-making. It delivers immediate value with minimal friction and represents the core feature purpose.

**Independent Test**: Can be fully tested by running `finfocus cost estimate --provider aws --resource-type ec2:Instance --property instanceType=m5.large` and verifying baseline/modified cost output is displayed.

**Acceptance Scenarios**:

1. **Given** a developer with the finfocus CLI installed, **When** they run `finfocus cost estimate --provider aws --resource-type ec2:Instance --property instanceType=m5.large`, **Then** the system displays a table showing baseline cost, modified cost, and the cost delta.

2. **Given** a developer specifying multiple properties, **When** they run `finfocus cost estimate --provider aws --resource-type ec2:Instance --property instanceType=m5.large --property volumeSize=100`, **Then** the system displays per-property cost deltas showing the impact of each change.

3. **Given** a developer requesting JSON output, **When** they add `--output json` to the command, **Then** the system outputs structured JSON containing baseline, modified, and deltas objects.

---

### User Story 2 - Batch Modifications on Existing Plans (Priority: P2)

A developer has an existing Pulumi preview JSON file and wants to estimate the cost impact of upgrading specific resources in that plan without re-running `pulumi preview`.

**Why this priority**: Builds on P1 by enabling plan-based estimation. Many users already have plan files from their workflow and want to explore modifications to existing infrastructure.

**Independent Test**: Can be fully tested by running `finfocus cost estimate --pulumi-json plan.json --modify "web-server:instanceType=m5.large"` against a valid plan file and verifying the modified resource costs are displayed.

**Acceptance Scenarios**:

1. **Given** a developer with a valid Pulumi preview JSON file containing a resource named "web-server", **When** they run `finfocus cost estimate --pulumi-json plan.json --modify "web-server:instanceType=m5.large"`, **Then** the system displays the baseline cost from the plan and the modified cost with the new instance type.

2. **Given** a developer modifying multiple resources in a plan, **When** they specify multiple `--modify` flags, **Then** the system displays cost deltas for each modified resource.

3. **Given** a developer specifying a resource name that doesn't exist in the plan, **When** they run the command with `--modify "nonexistent:instanceType=m5.large"`, **Then** the system displays a clear error message indicating the resource was not found.

---

### User Story 3 - Interactive TUI Exploration (Priority: P3)

A developer wants to interactively explore different property configurations and see live cost updates as they modify values, enabling rapid iteration without running multiple CLI commands.

**Why this priority**: This is an enhanced user experience feature that builds on P1 and P2. It requires TUI infrastructure and represents a more advanced workflow that benefits power users.

**Independent Test**: Can be fully tested by running `finfocus cost estimate --interactive`, modifying a property in the TUI, and verifying the cost display updates in real-time.

**Acceptance Scenarios**:

1. **Given** a developer launching interactive mode, **When** they run `finfocus cost estimate --interactive`, **Then** the system displays a TUI with a property editor allowing key-value input and a cost display panel.

2. **Given** a developer in interactive mode, **When** they modify a property value in the editor, **Then** the cost display updates within 2 seconds to show the new estimate.

3. **Given** a developer in interactive mode, **When** they press 'q' or Ctrl+C, **Then** the TUI exits cleanly and returns to the shell.

---

### Edge Cases

- What happens when the plugin doesn't implement the EstimateCost RPC? The system falls back to calling GetProjectedCost twice (once with original properties, once with overrides) and calculates the delta.
- What happens when an invalid resource type is specified? The system displays a validation error with a helpful message indicating the resource type is not recognized.
- What happens when no properties are specified for single-resource mode? The system displays the baseline cost only (no delta calculation needed).
- What happens when the property key doesn't affect pricing? The plugin returns identical baseline/modified costs, and the delta shows $0.00 change.
- What happens when --pulumi-json and single-resource flags are both provided? The system displays an error indicating mutually exclusive options.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `cost estimate` subcommand under the `cost` command group
- **FR-002**: System MUST accept `--provider` flag to specify the cloud provider (e.g., aws, gcp, azure)
- **FR-003**: System MUST accept `--resource-type` flag to specify the resource type (e.g., ec2:Instance)
- **FR-004**: System MUST accept repeatable `--property` flags in `key=value` format for property overrides
- **FR-005**: System MUST accept `--pulumi-json` flag to load resources from an existing Pulumi preview JSON file
- **FR-006**: System MUST accept repeatable `--modify` flags in `resource-name:key=value` format for plan-based modifications
- **FR-007**: System MUST accept `--interactive` flag to launch the TUI exploration mode
- **FR-008**: System MUST display output in table format by default showing Property, Current, Proposed, and Monthly Delta columns
- **FR-009**: System MUST accept `--output` flag supporting `table`, `json`, and `ndjson` formats
- **FR-010**: System MUST display baseline cost (original configuration) and modified cost (with overrides applied)
- **FR-011**: System MUST display per-property cost deltas showing the cost impact of each individual property change
- **FR-012**: System MUST pass property overrides directly to the plugin without interpreting which properties affect pricing (anti-guess boundary)
- **FR-013**: System MUST provide graceful fallback when plugin doesn't implement EstimateCost by using GetProjectedCost twice
- **FR-014**: System MUST validate that --pulumi-json and single-resource flags (--provider, --resource-type) are mutually exclusive
- **FR-015**: System MUST display color-coded deltas (green for savings, red for increases) in table output when terminal supports colors

### Key Entities

- **ResourceDescriptor**: Represents a cloud resource with provider, type, region, and properties map
- **PropertyOverride**: A key-value pair representing a property change (e.g., instanceType=m5.large)
- **CostResult**: Contains monthly cost amount, currency, and optional notes
- **CostDelta**: Represents the cost impact of a single property change, including original value, new value, and cost difference
- **EstimateResult**: Aggregates baseline cost, modified cost, and list of per-property deltas

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can obtain cost estimates for hypothetical resource configurations in a single command without modifying infrastructure code
- **SC-002**: Cost comparison clearly displays the difference between current and proposed configurations with exact dollar amounts
- **SC-003**: Users can estimate costs for at least 3 different resource configurations within 5 minutes (vs. modifying IaC code which would take 15+ minutes each)
- **SC-004**: 90% of cost estimate requests return results within 5 seconds for single-resource estimation
- **SC-005**: Per-property cost breakdown enables users to identify which specific change has the highest cost impact
- **SC-006**: Interactive mode allows users to explore at least 10 different property combinations without restarting the tool

## Assumptions

- The finfocus-spec v0.5.5 EstimateCost RPC is available and its message definitions are stable
- Plugins that implement EstimateCost will return accurate baseline and modified costs
- For plugins that don't implement EstimateCost, the fallback using two GetProjectedCost calls provides acceptable accuracy
- The existing TUI components from the internal/tui package can be reused for interactive mode
- Property keys provided by users match the property names expected by plugins (no transformation needed)
- Users have sufficient plugin coverage for the resource types they want to estimate

## Dependencies

- finfocus-spec v0.5.5+ with EstimateCost RPC definition
- Plugin implementations (aws-public, kubecost) must implement EstimateCost RPC for full functionality
- Existing TUI infrastructure (Bubble Tea, Lip Gloss) for interactive mode
- Existing ingestion layer for parsing Pulumi preview JSON files
