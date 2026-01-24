# Feature Specification: Budget Health Suite

**Feature Branch**: `123-budget-health-suite`
**Created**: 2026-01-24
**Status**: Draft
**Input**: User description: "Implement Budget Health Suite combining #263 and #267: Core engine functionality for GetBudgets RPC with provider filtering, currency handling, summary aggregation, health status calculation, and threshold-based alerting. Uses proto definitions from finfocus-spec v0.5.4 including Budget, BudgetFilter, BudgetSummary, BudgetHealthStatus, and BudgetThreshold types."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Budget Health Overview (Priority: P1)

As a DevOps engineer managing multi-cloud infrastructure, I want to see the health status of all my budgets at a glance so I can quickly identify which budgets need attention.

**Why this priority**: This is the core value proposition - without health visibility, users cannot make informed decisions about cloud spending. It enables proactive cost management.

**Independent Test**: Can be fully tested by querying budgets from a plugin and displaying health status. Delivers immediate visibility into budget utilization.

**Acceptance Scenarios**:

1. **Given** I have budgets configured across AWS and GCP, **When** I call `engine.GetBudgets(ctx, nil)`, **Then** each budget in the result has its health status (OK, WARNING, CRITICAL, or EXCEEDED) based on current utilization percentage.
2. **Given** I have a budget at 85% utilization, **When** I retrieve the budget via the engine, **Then** the budget shows WARNING status with the current spend and limit populated.
3. **Given** I have no budgets configured, **When** I call `engine.GetBudgets(ctx, nil)`, **Then** I receive an empty result set (no error).

---

### User Story 2 - Filter Budgets by Provider (Priority: P2)

As a cloud platform administrator, I want to filter budgets by cloud provider so I can focus on spending for a specific platform (e.g., only AWS budgets).

**Why this priority**: Filtering is essential for multi-cloud environments where users need to focus on specific providers without noise from others.

**Independent Test**: Can be tested by applying provider filters and verifying only matching budgets are returned.

**Acceptance Scenarios**:

1. **Given** I have budgets from AWS, GCP, and Kubecost, **When** I filter by provider "aws", **Then** I see only AWS budgets.
2. **Given** I filter by provider "azure", **When** no Azure budgets exist, **Then** I see an empty result set with a clear message.
3. **Given** I provide multiple providers in the filter, **When** I query budgets, **Then** I see budgets matching any of the specified providers.

---

### User Story 3 - Budget Summary Statistics (Priority: P2)

As a finance team member, I want to see aggregated summary statistics across all budgets so I can report on overall budget health to leadership.

**Why this priority**: Summary statistics enable quick executive reporting and trend identification without reviewing individual budgets.

**Independent Test**: Can be tested by aggregating budget counts by health status and verifying totals match.

**Acceptance Scenarios**:

1. **Given** I have 10 budgets with various health statuses, **When** I request a summary, **Then** I see counts for each status (OK: 5, WARNING: 3, CRITICAL: 1, EXCEEDED: 1) and total count (10).
2. **Given** all budgets are healthy, **When** I view the summary, **Then** I see all budgets counted as OK with zero in other categories.
3. **Given** budgets exist across multiple currencies, **When** I view the summary, **Then** counts are aggregated regardless of currency (no currency conversion).

---

### User Story 4 - Threshold Alerting (Priority: P3)

As an operations engineer, I want to see which budget thresholds have been triggered so I can take proactive action before budgets are exceeded.

**Why this priority**: Threshold alerting enables proactive intervention, but requires the health calculation foundation from P1/P2 to be meaningful.

**Independent Test**: Can be tested by evaluating thresholds against current and forecasted spend.

**Acceptance Scenarios**:

1. **Given** a budget with thresholds at 50%, 80%, and 100%, **When** current spend is at 75%, **Then** the 50% threshold shows as triggered and 80%/100% show as not triggered.
2. **Given** a budget with a forecasted threshold at 90%, **When** forecasted spend is at 95%, **Then** the forecasted threshold shows as triggered.
3. **Given** a budget with no configured thresholds, **When** I view thresholds, **Then** default thresholds (50%, 80%, 100%) are applied and evaluated.

---

### User Story 5 - Forecasted Spending (Priority: P3)

As a budget owner, I want to see forecasted end-of-period spending so I can anticipate whether budgets will be exceeded before the period ends.

**Why this priority**: Forecasting provides predictive value, but requires the core budget infrastructure from P1/P2.

**Independent Test**: Can be tested by calculating linear extrapolation from current spend and period dates.

**Acceptance Scenarios**:

1. **Given** a monthly budget with 10 days elapsed and $100 spent, **When** I view the forecast, **Then** I see forecasted spend of approximately $300 for the full month.
2. **Given** a budget period that hasn't started yet, **When** I view the forecast, **Then** forecasted spend equals current spend (no extrapolation).
3. **Given** a weekly budget at 90% through the period, **When** I view the forecast, **Then** the forecast reflects the short remaining time appropriately.

---

### Edge Cases

- What happens when a budget has zero limit configured? System should treat as invalid and skip or warn.
- How does the system handle plugins returning no budget data? Return empty results with informative message.
- What happens when currency codes are invalid (not 3 characters)? Validate and reject with clear error.
- How are budgets with missing status information handled? Calculate health from available data or mark as UNSPECIFIED.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST calculate budget health status (OK, WARNING, CRITICAL, EXCEEDED) based on percentage utilization thresholds (0-79% = OK, 80-89% = WARNING, 90-99% = CRITICAL, 100%+ = EXCEEDED).
- **FR-002**: System MUST filter budgets by provider using case-insensitive matching, returning all budgets when no filter is specified.
- **FR-003**: System MUST validate currency codes as ISO 4217 format (exactly 3 uppercase letters).
- **FR-004**: System MUST calculate budget summary statistics with counts per health status that sum to total budget count.
- **FR-005**: System MUST evaluate budget thresholds against both actual and forecasted spend percentages.
- **FR-006**: System MUST calculate forecasted end-of-period spending using linear extrapolation from current spend and elapsed time.
- **FR-007**: System MUST apply default thresholds (50%, 80%, 100% actual) when no thresholds are configured for a budget.
- **FR-008**: System MUST aggregate health status across providers, returning the worst-case status as overall health.
- **FR-009**: System MUST support multiple providers in a single filter (OR logic - budget matches if it matches any provider).
- **FR-010**: System MUST track when each threshold was triggered (timestamp).

### Key Entities

- **Budget**: Represents a spending limit with id, name, source (provider), amount (limit + currency), period, optional filter, thresholds, and status.
- **BudgetFilter**: Scope restrictions including providers, regions, resource types, and tags.
- **BudgetAmount**: Monetary limit with value and ISO 4217 currency code.
- **BudgetStatus**: Current spending state including current spend, forecasted spend, utilization percentages, and health status.
- **BudgetThreshold**: Alert configuration with percentage, type (actual/forecasted), triggered flag, and triggered timestamp.
- **BudgetSummary**: Aggregated counts of budgets by health status (total, ok, warning, critical, exceeded).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view health status for all budgets within 2 seconds for up to 100 budgets.
- **SC-002**: Budget filtering by provider returns results within 500ms for datasets up to 1000 budgets.
- **SC-003**: Summary statistics accurately reflect actual budget counts with 100% accuracy (sum of status counts equals total).
- **SC-004**: Forecasted spend calculations are within 5% accuracy of linear extrapolation for the given time period.
- **SC-005**: 80% code coverage for all budget health calculation logic.
- **SC-006**: Users can identify critical and exceeded budgets within 5 seconds of viewing the budget list.

## Assumptions

- Proto definitions from finfocus-spec v0.5.4 are available and stable.
- Plugins return budget data in the expected proto format.
- Currency handling does not require conversion - amounts displayed in original currency.
- Linear extrapolation is sufficient for forecasting (no weighted or pattern-based methods needed for initial implementation).
- Default threshold values (50%, 80%, 100%) align with industry standards for budget alerting.

## Dependencies

- finfocus-spec v0.5.4 (Budget, BudgetFilter, BudgetSummary, BudgetHealthStatus, BudgetThreshold proto types)
- Existing engine infrastructure for plugin communication
- Related issues: #265 (health aggregation tests), #266 (namespace filtering), #219 (exit codes)

## Out of Scope

- Currency conversion between different currencies
- Historical trend analysis beyond simple forecasting
- Notification delivery (email, webhook) - covered by separate issue #220
- CLI command implementation - this spec covers engine functionality only
- Kubecost-specific namespace filtering - covered by #266
