# Feature Specification: Tag-Based Budget Filtering

**Feature Branch**: `222-budget-tag-filter`
**Created**: 2026-02-02
**Status**: Draft
**Input**: GitHub Issue #532 - feat(engine): Add tag-based filtering to BudgetFilterOptions

## Clarifications

### Session 2026-02-02

- Q: What should happen when a user provides a malformed filter syntax (e.g., missing `=value` or empty key)? → A: CLI exits with a descriptive error message before querying budgets.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Filter Budgets by Namespace (Priority: P1)

As a platform engineer managing Kubernetes workloads, I want to filter budget results by namespace so I can see budget health for specific workloads without noise from other teams.

**Why this priority**: Core use case that enables targeted budget visibility for multi-tenant environments. Without this, users see all budgets regardless of their area of responsibility.

**Independent Test**: Can be fully tested by filtering budgets with `--filter "tag:namespace=production"` and verifying only budgets with matching metadata are returned.

**Acceptance Scenarios**:

1. **Given** budgets exist with `metadata["namespace"]="production"` and `metadata["namespace"]="staging"`, **When** I filter with `tag:namespace=production`, **Then** only production budgets are returned
2. **Given** a budget has no namespace metadata, **When** I filter with `tag:namespace=production`, **Then** that budget is excluded from results
3. **Given** no budgets match the tag filter, **When** I apply the filter, **Then** an empty result set is returned (not an error)

---

### User Story 2 - Filter Budgets with Glob Patterns (Priority: P2)

As a platform engineer managing multiple environments with naming conventions (prod-us, prod-eu, prod-asia), I want to use wildcard patterns to filter budgets matching a pattern so I can view all production budgets across regions.

**Why this priority**: Enhances usability for environments with systematic naming. Less critical than exact matching but significantly improves productivity.

**Independent Test**: Can be fully tested by creating budgets with patterned namespaces and filtering with `--filter "tag:namespace=prod-*"`.

**Acceptance Scenarios**:

1. **Given** budgets with namespaces "prod-us", "prod-eu", "staging", **When** I filter with `tag:namespace=prod-*`, **Then** "prod-us" and "prod-eu" are returned, "staging" is excluded
2. **Given** budget with namespace "team-a-production", **When** I filter with `tag:namespace=*-production`, **Then** the budget is returned
3. **Given** budget with namespace "production", **When** I filter with `tag:namespace=prod*`, **Then** the budget is returned (since "production" starts with "prod")

---

### User Story 3 - Combine Multiple Tag Filters (Priority: P2)

As a finance team member, I want to filter budgets by multiple tags simultaneously (e.g., namespace AND cluster) so I can generate precise cost reports for specific workloads in specific environments.

**Why this priority**: Power user feature enabling precise filtering. Important for compliance and detailed reporting.

**Independent Test**: Can be tested by filtering with `--filter "tag:namespace=prod" --filter "tag:cluster=us-east-1"` and verifying AND logic.

**Acceptance Scenarios**:

1. **Given** budgets with various namespace/cluster combinations, **When** I filter with both `tag:namespace=prod` and `tag:cluster=us-east-1`, **Then** only budgets matching BOTH tags are returned
2. **Given** a budget matches namespace but not cluster, **When** I apply both filters, **Then** the budget is excluded

---

### User Story 4 - Combine Provider and Tag Filters (Priority: P3)

As a multi-cloud operator, I want to combine provider filtering with tag filtering so I can view budgets from a specific provider for a specific workload.

**Why this priority**: Extends existing functionality. Nice-to-have for multi-cloud scenarios.

**Independent Test**: Can be tested by filtering with `--filter "provider=kubecost" --filter "tag:namespace=staging"`.

**Acceptance Scenarios**:

1. **Given** budgets from kubecost and aws-budgets with various namespaces, **When** I filter with `provider=kubecost` and `tag:namespace=staging`, **Then** only kubecost budgets in staging namespace are returned

---

### Edge Cases

- What happens when tag key contains special characters (e.g., `tag:kubernetes.io/name=value`)? System handles keys with dots, slashes, and hyphens.
- What happens when tag value is empty string (`tag:namespace=`)? Matches budgets where metadata has the key with empty value.
- What happens when the same tag key is specified multiple times? Later value overwrites earlier (deterministic behavior).
- What happens with case sensitivity in tag keys and values? Tag matching is case-sensitive (consistent with metadata storage).
- What happens when filter syntax is malformed (e.g., `tag:namespace` missing `=`, or `tag:` with no key)? CLI exits with descriptive error message before querying budgets.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extend `BudgetFilterOptions` struct to include a `Tags` field of type `map[string]string` for metadata tag filtering
- **FR-002**: System MUST apply tag filters using AND logic (all specified tags must match for a budget to be included)
- **FR-003**: System MUST support glob patterns in tag values using `*` as wildcard character
- **FR-004**: System MUST exclude budgets that lack a required tag key from filtered results
- **FR-005**: System MUST return all budgets when no tag filters are specified (empty `Tags` map)
- **FR-006**: CLI MUST parse `--filter "tag:key=value"` syntax and populate the `Tags` field in `BudgetFilterOptions`
- **FR-007**: System MUST support combining tag filters with existing provider filters (provider filters use OR logic, tag filters use AND logic)
- **FR-008**: System MUST use `path.Match()` or equivalent glob matching for pattern evaluation
- **FR-009**: Tag filtering MUST be case-sensitive for both keys and values (matching metadata storage semantics)
- **FR-010**: CLI MUST validate filter syntax before querying and exit with descriptive error for malformed filters (missing `=`, empty key after `tag:`)

### Key Entities *(include if feature involves data)*

- **BudgetFilterOptions**: Extended struct containing `Providers []string` (existing) and `Tags map[string]string` (new). Tags are key-value pairs where values may include glob patterns.
- **Budget.Metadata**: Existing protobuf map field containing plugin-provided metadata (e.g., `namespace`, `cluster`, `environment`). Source of truth for tag matching.
- **Filter Expression**: CLI input in format `tag:key=value` parsed into key-value pairs for the Tags map.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can filter budgets by any metadata tag key within 3 seconds for datasets up to 10,000 budgets
- **SC-002**: Glob pattern matching correctly filters budgets with 100% accuracy across all supported patterns (`*` prefix, suffix, and both)
- **SC-003**: Unit test coverage for tag filtering logic achieves 90% or higher
- **SC-004**: Integration tests validate end-to-end CLI-to-engine tag filtering works correctly
- **SC-005**: 100% backward compatibility - existing provider-only filtering continues to work unchanged
- **SC-006**: Users filtering by namespace can reduce visible budget count by 80%+ in typical multi-tenant scenarios

## Assumptions

- Plugins already populate budget metadata correctly (e.g., Kubecost sets `namespace` in metadata)
- Glob pattern support is limited to `*` wildcard (not full regex)
- The `path.Match()` function from Go's standard library provides sufficient pattern matching capability
- Tag keys and values in metadata do not require normalization (used as-is from plugin)
- Empty tag value filter (`tag:key=`) is a valid use case matching budgets with that key set to empty string
