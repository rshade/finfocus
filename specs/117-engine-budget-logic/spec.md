# Feature Specification: Engine Budget Logic

**Feature Branch**: `117-engine-budget-logic`
**Created**: 2026-01-18
**Status**: Draft
**Input**: User description: "Implement core engine functionality to support the GetBudgets RPC from pulumicost-spec v0.4.11."

## Clarifications

### Session 2026-01-18
- Q: Dependency version mismatch (Prompt asks for `pulumicost-spec v0.4.11`, `go.mod` has `finfocus-spec v0.5.2`) → A: Use `finfocus-spec v0.5.2` to align with project state.
- Q: Currency handling in summary (Conflict between T018 struct and T017 "Group by currency") → A: Global Counts Only. Summary aggregates health across ALL currencies into single integers.
- Q: Handling missing/nil BudgetStatus in summary → A: Exclude from health buckets (Ok/Warn/Crit/Exceeded), count in Total, and LOG a warning using `zerolog`.
- Q: Scope of `BudgetFilter` fields (Prompt mentions only Provider, but Proto has more) → A: Implement Full Filtering. Include logic for Regions, ResourceTypes, and Tags in addition to Providers.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Advanced Budget Filtering (Priority: P1)

As a platform engineer managing multi-cloud spend, I need to filter budgets by cloud provider, region, resource type, and tags so that I can precisely isolate and analyze specific spending slices (e.g., "AWS EC2 budgets in us-east-1 with tag Project:X").

**Why this priority**: Core functionality for users with large, multi-cloud environments. Without advanced filtering, the budget list remains too noisy even with provider-only filters.

**Independent Test**: Can be fully tested by ingesting mixed budgets and applying a filter with multiple criteria (e.g., Provider="aws" AND Region="us-east-1"), verifying only matching budgets are returned.

**Acceptance Scenarios**:

1. **Given** a set of budgets from AWS, GCP, and Kubecost, **When** I filter for "aws-budgets", **Then** only AWS budgets are returned.
2. **Given** a set of budgets, **When** I provide an empty filter, **Then** all budgets are returned.
3. **Given** a set of budgets, **When** I filter for a provider (e.g., "AWS-BUDGETS") using different casing, **Then** it matches correctly (case-insensitive).
4. **Given** a set of budgets, **When** I filter for multiple criteria (e.g., Provider="aws" AND Region="us-east-1"), **Then** only budgets matching ALL criteria are returned.
5. **Given** a set of budgets with tags, **When** I filter by a specific tag key/value pair, **Then** only budgets with that exact tag are returned.

---

### User Story 2 - Budget Health Summary (Priority: P1)

As a finops practitioner, I need to see a high-level summary of budget health (e.g., "3 Critical, 5 Warning") so that I can immediately identify and triage areas requiring attention.

**Why this priority**: Provides immediate "at-a-glance" value. Users shouldn't have to count manually.

**Independent Test**: Can be tested by creating budgets with known health states and verifying the summary counts match.

**Acceptance Scenarios**:

1. **Given** a list of budgets with various health states (OK, Warning, Critical, Exceeded), **When** I request the budget summary, **Then** the counts for each category exactly match the input budgets.
2. **Given** the summary counts, **When** I sum them up, **Then** the total equals the total number of budgets processed.
3. **Given** a mixed list of budgets in USD and EUR, **When** I request the summary, **Then** the counts reflect the total volume of budgets regardless of currency.
4. **Given** a budget with missing `Status` or `Health` data, **When** the summary is calculated, **Then** it is counted in `TotalBudgets` but NOT in any health bucket, and a warning is logged.

---

### User Story 3 - Multi-Currency Budget Display (Priority: P2)

As a global infrastructure manager, I need budget amounts to be displayed in their original currency (e.g., EUR for EU regions, USD for US) so that I see the exact contractual limits without confusing exchange rate fluctuations.

**Why this priority**: Essential for accuracy in multi-region/multi-currency orgs, though single-currency orgs might not need it immediately.

**Independent Test**: Can be tested by verifying that a budget defined in "EUR" is returned as "EUR" without conversion to "USD".

**Acceptance Scenarios**:

1. **Given** a budget defined with a limit in "EUR", **When** I retrieve the budget, **Then** the amount and currency code "EUR" are preserved unchanged.
2. **Given** a budget input with an invalid currency code (e.g., "US"), **When** the engine processes it, **Then** it is flagged as invalid or rejected (ISO 4217 3-char validation).

### Edge Cases

- What happens when a budget filter contains a provider that doesn't exist? (Should return empty list, no error)
- How does the system handle budgets with missing health status during summary calculation? (Count in `Total` only and log via `zerolog`)
- What happens if 10,000 budgets are requested? (Performance should remain under acceptable limits)
- How should tag filtering handle budgets with missing tags? (Budgets missing a required tag key/value from the filter MUST be excluded)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow filtering budgets by Cloud Provider (case-insensitive).
- **FR-002**: System MUST support filtering by multiple providers simultaneously (OR logic within the provider field).
- **FR-003**: System MUST return all budgets if no filter is provided.
- **FR-004**: System MUST calculate a single aggregate summary containing counts for: Total, OK, Warning, Critical, and Exceeded budgets across ALL currencies.
- **FR-005**: System MUST validate that budget currency codes adhere to ISO 4217 (3-character code).
- **FR-006**: System MUST preserve the original currency of the budget (no automatic currency conversion).
- **FR-007**: System MUST accurately categorize budget health based on the provided status (OK < 80%, Warning 80-89%, Critical 90-99%, Exceeded >= 100%).
- **FR-008**: Implementation MUST use `github.com/rshade/finfocus-spec` (v0.5.2+) for proto definitions.
- **FR-009**: System MUST log a warning (using `zerolog`) for any budget with missing or unspecified health status during summary calculation.
- **FR-010**: System MUST support advanced filtering by Regions, ResourceTypes, and Tags (AND logic across different filter fields).

### Key Entities

- **Budget**: A spending limit definition containing Amount, Period, and current Status.
- **BudgetFilter**: Criteria including Providers, Regions, ResourceTypes, and Tags used to narrow down the list of budgets.
- **BudgetSummary**: A statistical aggregation of budget health states (OK, Warning, Critical, Exceeded).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Engine accurately filters budgets matching all criteria (Provider, Region, Tag, etc.) 100% of the time in test scenarios.
- **SC-002**: Budget Summary counts exactly match the sum of individual budget health statuses in all test cases.
- **SC-003**: Filtering logic processes 1000 budgets in under 100ms (as per user performance requirement).
- **SC-004**: Invalid currency codes (non-3-char) are consistently rejected or flagged during validation.