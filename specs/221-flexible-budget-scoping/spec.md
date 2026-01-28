# Feature Specification: Flexible Budget Scoping

**Feature Branch**: `221-flexible-budget-scoping`
**Created**: 2026-01-24
**Status**: Draft

## Clarifications

### Session 2026-01-24

- Q: When a resource matches multiple tag-based budgets, how should the system handle cost allocation? → A: Require explicit priority weighting in config; emit visible warning if overlapping tags lack priority; requires extensive documentation.
- Q: What configuration format should be used for defining scoped budgets? → A: Extend existing `~/.finfocus/config.yaml` with a `budgets:` section.
- Q: Which capabilities should be out of scope for MVP? → A: Both multi-currency support and historical budget tracking are out of scope for MVP.
- Q: What logging/debugging output should be available for scoped budgets? → A: Debug-level logging showing which scopes each resource matched, enabled via `--debug` flag.
- Q: How should the CLI display multiple scoped budgets? → A: Hierarchical display with Global first, then grouped sections (BY PROVIDER, BY TAG, BY TYPE).

**Input**: User description: "title: feat(cli): Add flexible budget scoping (per-provider, per-resource-type) state: OPEN author: rshade labels: roadmap/next comments: 1 assignees: projects: milestone: number: 221 -- ## Summary Extend budget configuration to support flexible scoping beyond global budgets, including per-provider, per-resource-type, and per-tag budgets. ## Motivation Different teams and resources have different budget requirements: - AWS and GCP may have separate budget allocations - Compute resources may have different limits than storage - Production vs development environments need separate tracking - Teams may have their own budget allocations via tags ## User Stories - As a FinOps manager, I want per-provider budgets so that I can allocate spending by cloud - As a team lead, I want per-tag budgets so that my team has its own spending limit - As a platform engineer, I want per-resource-type budgets so that I can control specific resource categories ..."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Multi-Cloud Spend Management (Priority: P1)

As a FinOps manager, I want to set and track separate budgets for different cloud providers (AWS, GCP, Azure) to ensure each cloud's spending stays within its allocated portion of the total budget.

**Why this priority**: Essential for organizations using multiple clouds where budgets are often allocated per-provider. This is the primary driver for moving beyond a single global budget.

**Independent Test**: Can be tested by configuring budgets for two different providers and verifying that costs from resources in Provider A only count toward Provider A's budget and the Global budget, not Provider B's.

**Acceptance Scenarios**:

1. **Given** a configuration with an AWS budget of $200 and a Global budget of $500, **When** total AWS spend is $150, **Then** the CLI should show AWS budget at 75% and Global budget reflecting total spend.
2. **Given** a configuration with multiple provider budgets, **When** running the CLI, **Then** a "BY PROVIDER" table should be displayed showing each provider's budget, spend, and status.

---

### User Story 2 - Team-Based Budgeting via Tags (Priority: P2)

As a team lead, I want to set budgets based on resource tags (e.g., `team:platform`, `environment:production`) so that I can manage my team's specific financial footprint independently of other teams.

**Why this priority**: Highly valuable for decentralized cost management. Allows teams to be accountable for their own spending.

**Independent Test**: Can be tested by assigning a budget to a specific tag and verifying that resources carrying that tag are correctly aggregated against that budget.

**Acceptance Scenarios**:

1. **Given** a budget defined for tag `team:platform`, **When** resources with this tag consume $120, **Then** the CLI should show the `team:platform` budget status.
2. **Given** a resource has tags matching multiple budgets (e.g., `team:platform` with priority=100 and `env:prod` with priority=50), **When** evaluating, **Then** the system should allocate cost only to the highest-priority tag budget (`team:platform`) per FR-009.

---

### User Story 3 - Resource Category Control (Priority: P3)

As a platform engineer, I want to set budgets for specific resource types (e.g., `aws:ec2/instance`) to identify and control high-cost resource categories before they impact the overall budget.

**Why this priority**: Useful for granular control over expensive services, but typically secondary to organizational (team/provider) budgets.

**Independent Test**: Can be tested by configuring a budget for a specific resource type and ensuring only resources of that exact type contribute to that budget.

**Acceptance Scenarios**:

1. **Given** a budget for `aws:ec2/instance`, **When** EC2 costs reach the threshold, **Then** a specific alert or warning for that resource type should be visible in the CLI.

---

### Edge Cases

- **Multiple Tag Matches**: When a resource has multiple tags with associated budgets, the system requires explicit `priority` weighting in the budget config to determine which single tag budget applies. If overlapping tag budgets lack priority values, the CLI MUST emit a visible warning (e.g., "WARNING: Overlapping tag budgets without priority - resource cost allocated to first matched"). This prevents silent budget aggregation that could mask overspend. **Documentation required**: Comprehensive guide on priority configuration with examples.
- **Missing Provider/Type Mapping**: How does the system handle resource types it doesn't recognize? (Assumption: These resources only contribute to the Global budget).
- **Currency Mismatch**: Scoped budgets MUST use the same currency as the global budget. Multi-currency support is out of scope for MVP (see OOS-001). The CLI should validate currency consistency at config load time and emit a clear error if mismatched.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support configuration of budgets scoped by cloud provider (e.g., aws, gcp, azure).
- **FR-002**: System MUST support configuration of budgets scoped by specific resource types (e.g., `aws:ec2/instance`).
- **FR-003**: System MUST support configuration of budgets scoped by resource tags (e.g., `env:prod`).
- **FR-004**: System MUST apply a Global budget as a fallback for all resources regardless of other scopes.
- **FR-005**: System MUST aggregate costs into multiple applicable scopes by scope type: a resource counts toward its Global budget, its Provider budget, and its Resource Type budget. For tag budgets, cost is allocated to a single tag budget based on priority (see FR-009).
- **FR-006**: System MUST support independent alert thresholds for each budget scope.
- **FR-007**: CLI MUST display a consolidated "BUDGET STATUS" view in hierarchical format: Global budget first, followed by grouped sections (BY PROVIDER, BY TAG, BY TYPE) showing each scope's budget, spend, and status.
- **FR-008**: System MUST support a `--budget-scope` flag to filter the CLI output to specific scopes (e.g., `provider`, `tag=team:platform`).
- **FR-009**: System MUST support `priority` field on tag-based budgets; when a resource matches multiple tag budgets, only the highest-priority budget receives the cost allocation.
- **FR-010**: System MUST emit a visible CLI warning when overlapping tag budgets are detected without explicit priority configuration.

### Key Entities

- **Budget Scope**: The definition of what resources are included in a budget (Global, Provider, Resource Type, or Tag).
- **Budget Configuration**: The financial limit, currency, period, and alert thresholds for a specific scope.
- **Cost Allocation**: The process of mapping an individual resource's cost to all applicable budget scopes.

### Technical Constraints

- **TC-001**: Budget configuration MUST be stored in the existing `~/.finfocus/config.yaml` file under a `budgets:` section.
- **TC-002**: Configuration MUST use YAML format consistent with existing FinFocus config patterns.

### Non-Functional Requirements

- **NFR-001**: Debug-level logging MUST show which budget scopes each resource matched during cost allocation (enabled via `--debug` flag).
- **NFR-002**: Logging MUST use existing zerolog patterns with structured fields (`component`, `resource_type`, `matched_scopes`).

### Out of Scope (MVP)

- **OOS-001**: Multi-currency support - All scoped budgets must use the same currency as the global budget. Currency conversion between USD, EUR, etc. is deferred to a future release.
- **OOS-002**: Historical budget tracking - Persisting and querying past budget status over time is not included. Budget evaluation is point-in-time only.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view a breakdown of spending by cloud provider in a single CLI command execution.
- **SC-002**: Users can identify which specific team or environment is exceeding its allocation via the Tag-based budget status table.
- **SC-003**: Filtering the budget view using `--budget-scope` returns relevant results in under 500ms for plans with up to 10,000 resources.
- **SC-004**: 100% of configured scoped budgets are correctly evaluated and displayed if resources matching that scope exist in the input data.
