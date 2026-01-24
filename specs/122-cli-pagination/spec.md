# Feature Specification: Add performance optimizations and pagination

**Feature Branch**: `122-cli-pagination`
**Created**: 2026-01-20
**Status**: Draft
**Input**: User description provided via CLI.

## User Scenarios & Testing

### User Story 1 - Enterprise Scale Performance (Priority: P1)

As an enterprise user with hundreds or thousands of resources, I want the CLI to respond quickly and efficiently so that I can view cost data without timeouts or excessive memory usage.

**Why this priority**: Core stability and usability requirement for large deployments.
**Independent Test**: Generate a dataset with 1000+ resources and verify CLI responds within performance targets.

**Acceptance Scenarios**:

1. **Given** a dataset with 1000+ resources, **When** I run `pulumicost cost recommendations`, **Then** the initial results should appear within 2 seconds.
2. **Given** a large dataset, **When** the query runs, **Then** memory usage should remain under 100MB.
3. **Given** a long-running query, **When** it executes, **Then** a progress indicator should show current status with batch-aligned counts (e.g., "Processing resources... [300/1000]" reflecting 100-item batch processing).

### User Story 2 - Output Control & Pagination (Priority: P1)

As a user, I want to limit and paginate output so that I can consume results in manageable chunks and integrate with other tools.

**Why this priority**: Essential for usability when dealing with large lists and for scripting/automation.
**Independent Test**: Run commands with limit/page flags and verify output counts.

**Acceptance Scenarios**:

1. **Given** 100 available recommendations, **When** I run `pulumicost cost recommendations --limit 10`, **Then** only 10 items are displayed.
2. **Given** multiple pages of results, **When** I run `pulumicost cost recommendations --page 2 --page-size 20`, **Then** the second set of 20 items is displayed.
3. **Given** I need specific ordering, **When** I run `pulumicost cost recommendations --sort savings:desc`, **Then** results are sorted by savings in descending order.

### User Story 3 - TUI Virtual Scrolling (Priority: P2)

As a TUI user, I want to scroll through large lists seamlessly without waiting for all rows to render, so that the interface remains responsive.

**Why this priority**: Critical for the interactive experience in the terminal UI.
**Independent Test**: Open TUI with 10,000 items and verify scrolling smoothness.

**Acceptance Scenarios**:

1. **Given** a list of 10,000 items in TUI mode, **When** I open the view, **Then** the application should start immediately without pre-rendering all rows.
2. **Given** the list is loaded, **When** I scroll down rapidly, **Then** new items should appear instantly (virtual scrolling) with no UI lag.

### User Story 4 - CI/CD & Streaming Integration (Priority: P2)

As a CI/CD operator, I want streaming JSON output so that I can process results line-by-line without buffering the entire dataset.

**Why this priority**: Enables efficient pipeline integrations for large datasets.
**Independent Test**: Pipe `ndjson` output to `head` or `jq` and verify stream behavior.

**Acceptance Scenarios**:

1. **Given** a command requesting NDJSON output, **When** I run `pulumicost cost recommendations --output ndjson`, **Then** each item should be printed as a separate JSON line as soon as it is available.
2. **Given** a large result set, **When** I pipe the output to `head -n 5`, **Then** the process should terminate early after producing 5 lines (if supported by pipeline) or at least produce the first 5 lines immediately.

### Edge Cases

- **Pagination Bounds**: When a user requests a page beyond available pages (e.g., page 10 when only 5 pages exist), return an empty result set with pagination metadata showing the requested page number and actual total pages (e.g., "page 10 of 5, 0 items").
- **Invalid Sort Fields**: When sorting is requested on a field that doesn't exist, return an error message listing all valid sort fields (e.g., "Invalid sort field 'xyz'. Valid fields: savings, cost, name, resourceType").
- **Network Failures during Lazy Load**: When the TUI detail view fails to fetch cost history, display an inline error message with a keyboard-navigable retry action (e.g., "[Press 'r' to retry]") that re-attempts loading only that subsection without crashing the app or requiring full view refresh.
- **Zero Results**: Handling of empty datasets with pagination flags enabled.

## Requirements

### Functional Requirements

- **FR-001**: The CLI MUST support a `--limit` flag to restrict the number of returned items.
- **FR-002**: The CLI MUST support `--page` and `--page-size` flags for pagination of result sets.
- **FR-002a**: When a requested page exceeds available pages, the CLI MUST return an empty result set with pagination metadata indicating the requested page number and actual total pages.
- **FR-003**: The CLI MUST support an `--offset` flag to skip a specified number of initial items.
- **FR-004**: The CLI MUST support a `--sort` flag to order results by specific fields (e.g., `savings:desc`, `cost:desc`, `name:asc`).
- **FR-004a**: The CLI MUST validate sort field names and return an error with a list of valid fields when an invalid field is specified.
- **FR-005**: The TUI MUST implement virtual scrolling for list views to only render visible rows.
- **FR-006**: The CLI MUST provide a streaming output format (NDJSON) for `cost recommendations` and related commands.
- **FR-007**: The CLI MUST display a progress indicator (spinner/counts) for queries expected to take longer than 500ms.
- **FR-008**: The System MUST process large datasets in manageable segments using a batch size of 100 items to prevent memory exhaustion while minimizing API call overhead.
- **FR-009**: The System MUST implement caching for recommendation and cost queries with a configurable TTL (default: 3600 seconds / 1 hour) specified in config file (~/.finfocus/config.yaml: cache.ttl_seconds), overridable by environment variable (FINFOCUS_CACHE_TTL_SECONDS) or CLI flag (--cache-ttl).
- **FR-010**: The TUI detail view MUST load resource-intensive data (like historical costs) only upon user request or view activation (lazy loading).
- **FR-010a**: When lazy-loaded data fails to load in the TUI, the system MUST display an inline error message with a keyboard-navigable retry action that attempts to reload only that subsection.
- **FR-011**: JSON output responses MUST include pagination metadata (page number, total items, total pages, etc.) when not in streaming mode.

### Key Entities

- **Recommendation**: Represents a single optimization suggestion (subject to pagination/sorting).
- **Visible Item Subset**: The portion of the list currently rendered to the user in TUI mode.
- **Query Cache**: Temporary storage for query results with configurable TTL (default: 1 hour). Configuration sources: config file (~/.finfocus/config.yaml), environment variable (FINFOCUS_CACHE_TTL_SECONDS), or CLI flag (--cache-ttl), in order of precedence (CLI flag > env var > config file > default).

## Success Criteria

### Measurable Outcomes

- **SC-001**: Initial load of recommendation lists (up to 1000 items) completes in under 2 seconds.
- **SC-002**: Memory usage for processing 1000 items remains below 100MB.
- **SC-003**: TUI page navigation and scrolling latency is under 100ms.
- **SC-004**: Detail view loads secondary data in under 500ms (or shows loading state immediately).
- **SC-005**: 100% of CLI filtering/sorting options (limit, page, sort) produce correct subsets of data in integration tests.

## Assumptions

- The underlying plugin system supports (or will be updated to support) the requested query options (limit/sort) pushed down where possible.
- "NDJSON" refers to Newline Delimited JSON.
- Virtual scrolling is primarily for the interactive terminal UI.

## Clarifications

### Session 2026-01-20

- Q: What happens if a user requests Page 10 when only 5 pages exist? → A: Return empty result set with pagination metadata (page 10 of 5, 0 items)
- Q: What happens if sorting is requested on a field that doesn't exist? → A: Return error with message listing valid sort fields (e.g., "Invalid sort field 'xyz'. Valid fields: savings, cost, name, resourceType")
- Q: What should the error state look like when TUI detail view fails to fetch cost history? → A: Display inline error message with keyboard-navigable retry action (e.g., "[Press 'r' to retry]") for just that subsection
- Q: How should cache TTL be configured? → A: Hybrid approach - config file default (3600 seconds / 1 hour at ~/.finfocus/config.yaml: cache.ttl_seconds), overridable by environment variable or CLI flag
- Q: What batch size should be used for processing large datasets? → A: 100 items per batch (balanced approach for memory efficiency and API call overhead)
