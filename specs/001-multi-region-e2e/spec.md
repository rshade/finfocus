# Feature Specification: Add multi-region E2E testing support

**Feature Branch**: `001-multi-region-e2e`  
**Created**: 2026-01-21  
**Status**: Draft  
**Input**: User description: "title: Add multi-region E2E testing support
state: OPEN
author: rshade
labels: enhancement, roadmap/current
comments: 0
assignees:
projects:
milestone:
number: 185
--

## Summary

Add support for multi-region E2E testing to validate both projected and actual cost calculations across different AWS regions with varying pricing.

## Priority

**Post-MVP** - This is a future enhancement, not required for the initial MVP release.

## Context

From the E2E Test Plan, multi-region testing ensures:

- Cost calculations work correctly across regions with different pricing
- Region-specific plugin binaries are properly loaded
- Pricing data accuracy is validated per-region

## Requirements

### Test Scenarios

- [ ] Test projected costs in us-east-1 (baseline)
- [ ] Test projected costs in eu-west-1 (different pricing)
- [ ] Test projected costs in ap-northeast-1 (different pricing)
- [ ] Test actual costs in us-east-1 (baseline)
- [ ] Test actual costs in eu-west-1 (different pricing)
- [ ] Test actual costs in ap-northeast-1 (different pricing)
- [ ] Validate region-specific pricing differences are reflected for both cost types (eu-west-1: +7-15% vs us-east-1 baseline, ap-northeast-1: +10-20% vs us-east-1 baseline)

### Infrastructure

- [ ] Parameterized test fixtures for different regions containing 5-10 resources across 3-4 resource types (e.g., compute, storage, network, database)
- [ ] Region-specific Pulumi programs in test harness
- [ ] Cost assertion thresholds per region with ±5% variance tolerance

### Validation

- [ ] Verify region is correctly passed to plugin
- [ ] Confirm pricing matches expected regional rates
- [ ] Test fallback behavior when region-specific plugin unavailable

## Dependencies

- #177 - E2E test with Pulumi Automation API (must be complete first)
- #24 (pulumicost-plugin-aws-public) - Fallback GetActualCost implementation (required for actual cost testing across regions)

## Related

- E2E_TEST_PLAN.md - Section 4.2 Multi-region testing support"

## Clarifications

### Session 2026-01-21

- Q: Should the multi-region E2E tests validate both projected AND actual cost calculations, or only projected costs? → A: Both projected and actual costs (comprehensive validation, covers both pipelines)
- Q: What acceptable variance threshold should be used when validating cost calculations against expected regional rates? → A: ±5% variance tolerance (accounts for minor fluctuations, good for dynamic pricing)
- Q: How many resources of different types should each region's test fixture include to provide adequate validation coverage? → A: 5-10 resources across 3-4 types (balanced coverage, reasonable execution time)
- Q: What should the expected behavior be when pricing data is completely unavailable for a region? → A: Fail the test immediately (strict validation, blocks execution)
- Q: How does system handle network failures during plugin loading? → A: Retry 3 times with exponential backoff, then fail (handles transient issues)

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Run E2E Tests Across Multiple Regions (Priority: P1)

As a developer, I want to run end-to-end tests that validate both projected and actual cost calculations across different AWS regions so that I can ensure pricing accuracy and region-specific plugin loading work correctly for both cost pipelines.

**Why this priority**: This is the core functionality needed to validate multi-region cost calculations for both projected and actual costs, which is essential for comprehensive feature validation.

**Independent Test**: Can be fully tested by running the E2E test suite with region parameters and verifying both projected and actual cost outputs match expected regional rates.

**Acceptance Scenarios**:

1. **Given** a Pulumi program configured for us-east-1, **When** running E2E tests, **Then** projected costs are calculated using baseline pricing
2. **Given** a Pulumi program configured for eu-west-1, **When** running E2E tests, **Then** projected costs reflect European pricing differences
3. **Given** a Pulumi program configured for ap-northeast-1, **When** running E2E tests, **Then** projected costs reflect Asia Pacific pricing differences
4. **Given** a Pulumi state for us-east-1, **When** running actual cost tests, **Then** actual costs are retrieved using baseline regional pricing
5. **Given** a Pulumi state for eu-west-1, **When** running actual cost tests, **Then** actual costs reflect European pricing differences
6. **Given** a Pulumi state for ap-northeast-1, **When** running actual cost tests, **Then** actual costs reflect Asia Pacific pricing differences
7. **Given** a Pulumi program configured for us-east-1, **When** running projected cost tests, **Then** costs reflect baseline US East pricing (e.g., t3.micro EC2 at $7.01/month, gp3 EBS at $8.00/100GB)
8. **Given** a Pulumi program configured for eu-west-1, **When** running projected cost tests, **Then** costs reflect European pricing with 7-15% increase over us-east-1 baseline
9. **Given** a Pulumi program configured for ap-northeast-1, **When** running projected cost tests, **Then** costs reflect Asia Pacific pricing with 10-20% increase over us-east-1 baseline
10. **Given** a region with no pricing data available, **When** running E2E tests, **Then** the test fails immediately with a clear error message
11. **Given** a transient network failure during plugin loading, **When** running E2E tests, **Then** the system retries 3 times with exponential backoff before failing

---

### User Story 2 - Validate Region-Specific Plugin Loading (Priority: P2)

As a developer, I want to ensure that region-specific plugin binaries are loaded correctly during E2E tests so that accurate pricing data is used for each region.

**Why this priority**: Plugin loading is critical for cost accuracy, but secondary to the basic multi-region testing capability.

**Independent Test**: Can be tested by verifying which plugin version is loaded and used for cost calculations in each region.

**Acceptance Scenarios**:

1. **Given** a test run in us-east-1, **When** the plugin is invoked, **Then** the correct region-specific plugin binary is loaded
2. **Given** a test run in eu-west-1, **When** the plugin is invoked, **Then** the correct region-specific plugin binary is loaded

---

### User Story 3 - Test Fallback Behavior (Priority: P3)

As a developer, I want to test fallback behavior when region-specific plugins are unavailable so that the system gracefully handles missing regional data.

**Why this priority**: Fallback testing ensures robustness, but is the least critical aspect of the multi-region testing feature.

**Independent Test**: Can be tested by simulating missing region-specific plugins and verifying fallback to public pricing data.

**Acceptance Scenarios**:

1. **Given** a region without a specific plugin, **When** running E2E tests, **Then** the system falls back to public pricing data

---

### User Story 4 - Unified Multi-Region Fixture Testing (Priority: P3)

As a developer, I want to test cost calculations for a single Pulumi program that deploys resources across multiple AWS regions so that I can validate FinFocus correctly handles cross-region infrastructure deployments.

**Why this priority**: Many production Pulumi programs span multiple regions. Testing this scenario ensures comprehensive cost calculation coverage beyond per-region fixtures.

**Independent Test**: Can be tested by creating a unified Pulumi program with explicit provider configurations for multiple regions and verifying aggregated costs are calculated correctly.

**Acceptance Scenarios**:

1. **Given** a single Pulumi program with resources in us-east-1, eu-west-1, and ap-northeast-1, **When** running projected cost tests, **Then** each resource's cost reflects its region-specific pricing
2. **Given** a unified plan JSON with cross-region resources, **When** FinFocus processes the plan, **Then** costs are correctly aggregated per region
3. **Given** a Pulumi program using explicit AWS provider aliases for each region, **When** generating a preview, **Then** the plan JSON correctly identifies each resource's target region
4. **Given** the unified fixture, **When** running the full E2E test suite, **Then** total monthly cost equals the sum of individual region costs within ±5% tolerance

---

### Edge Cases

- When a region has no pricing data available, the test MUST fail immediately with a clear error message indicating missing pricing data
- When network failures occur during plugin loading, the system MUST retry 3 times with exponential backoff, then fail the test if the issue persists (distinguishes transient from persistent network issues)
- What occurs when region configuration is invalid?

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST support parameterized test fixtures for different AWS regions, each containing 5-10 resources across 3-4 resource types (compute, storage, network, database)
- **FR-002**: System MUST provide region-specific Pulumi programs and state files in the test harness
- **FR-003**: System MUST validate that projected costs vary correctly across regions (us-east-1 baseline, eu-west-1, ap-northeast-1)
- **FR-004**: System MUST validate that actual costs vary correctly across regions (us-east-1 baseline, eu-west-1, ap-northeast-1)
- **FR-005**: System MUST verify that regions are correctly passed to plugins for both projected and actual cost calculations
- **FR-006**: System MUST confirm pricing matches expected regional rates for both cost types within ±5% variance tolerance
- **FR-007**: System MUST test fallback behavior when region-specific plugins are unavailable
- **FR-008**: System MUST fail tests immediately when pricing data is completely unavailable for a region (no graceful degradation in E2E tests)
- **FR-009**: System MUST retry plugin loading 3 times with exponential backoff on network failures, then fail the test if the issue persists
- **FR-010**: System MUST source expected cost values from official AWS pricing APIs or documentation, with manual override capability for test fixtures

### Key Entities _(include if feature involves data)_

- **Region**: Represents an AWS region with associated pricing data and plugin versions
- **Test Fixture**: Parameterized configuration for running tests in specific regions, containing 5-10 resources across 3-4 resource types (compute, storage, network, database)
- **Pulumi Program**: Infrastructure-as-code definition used in E2E tests
- **Cost Assertion**: Expected cost thresholds and validation rules per region (includes ±5% variance tolerance for cost comparisons)

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: E2E tests pass successfully in us-east-1, eu-west-1, and ap-northeast-1 with regional pricing within ±5% of expected values for both projected and actual costs
- **SC-002**: Region-specific plugin binaries are loaded and used for both projected and actual cost calculations in each test region
- **SC-003**: Fallback to public pricing works when region-specific plugins are unavailable for both cost types
- **SC-004**: Test execution time remains under 5 minutes per region (covering both cost types)
