# Feature Specification: Cache Expires-At Hints

**Feature Branch**: `606-cache-expires-at`
**Created**: 2026-03-09
**Status**: Draft
**Input**: GitHub Issue #845 - feat(cache): consume expires_at caching hints from plugin cost responses

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Plugin-Directed Cache Lifetime (Priority: P1)

As a FinFocus user running cost queries against rate-limited cloud pricing APIs,
I want the cache to respect expiration hints provided by plugins,
so that cache entries live exactly as long as the plugin recommends — reducing
unnecessary API calls while ensuring data freshness.

**Why this priority**: This is the core value proposition. Without it, all cache
entries use a single fixed TTL regardless of how frequently the underlying data
changes. Plugins have domain knowledge about data freshness (e.g., AWS pricing
changes rarely vs. spot pricing changes every few minutes).

**Independent Test**: Run a projected cost query with a plugin that returns an
`expires_at` 24 hours in the future. Verify the cached entry survives for 24
hours instead of the default 1-hour TTL. Run the same query again within 24
hours and confirm a cache hit.

**Acceptance Scenarios**:

1. **Given** a plugin returns a projected cost response with `expires_at` set 24 hours from now, **When** the engine caches the result, **Then** the cache entry has a TTL of approximately 24 hours (not the default 1 hour).
2. **Given** a plugin returns an actual cost response with `expires_at` set 30 minutes from now, **When** the engine caches the result, **Then** the cache entry expires after approximately 30 minutes.
3. **Given** a plugin returns a cost response with no `expires_at` (nil/unset), **When** the engine caches the result, **Then** the cache entry uses the system default TTL (1 hour or user-configured value).

---

### User Story 2 - Stale Data Prevention (Priority: P2)

As a FinFocus user querying volatile pricing data (e.g., spot instances),
I want the cache to skip caching when a plugin signals that data is already stale,
so that I always see fresh cost data for rapidly-changing prices.

**Why this priority**: Prevents serving stale data. A plugin returning a past
`expires_at` timestamp is explicitly saying "this data is already outdated —
do not cache it." This is a safety mechanism that protects data integrity.

**Independent Test**: Simulate a plugin response where `expires_at` is set to a
timestamp in the past. Verify that the result is NOT stored in the cache. Run
the query again and confirm a fresh plugin call is made (no cache hit).

**Acceptance Scenarios**:

1. **Given** a plugin returns a cost response with `expires_at` set to a past timestamp, **When** the engine attempts to cache the result, **Then** the result is not stored in the cache.
2. **Given** a plugin returns a cost response with `expires_at` set to the current time (now), **When** the engine attempts to cache the result, **Then** the result is not stored in the cache.

---

### User Story 3 - Transparent Behavior (Priority: P3)

As a FinFocus operator debugging cache behavior,
I want cache TTL decisions to be logged when they differ from defaults,
so that I can understand why certain entries expire sooner or later than expected.

**Why this priority**: Observability is important but secondary to correctness.
Debug-level logging of TTL overrides helps operators troubleshoot cache behavior
without impacting normal usage.

**Independent Test**: Enable debug logging and run a cost query where the plugin
provides an `expires_at` hint. Verify that a debug log message indicates the
plugin-provided TTL was used instead of the default.

**Acceptance Scenarios**:

1. **Given** debug logging is enabled and a plugin returns `expires_at` with a future timestamp, **When** the result is cached, **Then** a debug log entry records the plugin-provided TTL and how it differs from the default.
2. **Given** a plugin returns `expires_at` with a past timestamp, **When** caching is skipped, **Then** a debug log entry explains that caching was skipped due to a past expiration hint.

---

### Edge Cases

- What happens when `expires_at` is only a few seconds in the future (less than the minimum TTL of 60 seconds)?
  - The system honors the plugin's hint and uses the short TTL. The minimum TTL constraint applies to user-configured defaults, not plugin-provided hints — plugins have domain authority over their data freshness.
- What happens when `expires_at` exceeds the maximum TTL of 7 days?
  - The system caps the TTL at the maximum (7 days) to prevent unbounded cache entries, and logs a warning.
- What happens when one resource in a batch has `expires_at` and another does not?
  - For projected costs (cached per-resource), each resource's cache entry uses its own hint independently. For actual costs (cached per-request), the shortest `expires_at` across all results in the response is used.
- What happens when the system clock is significantly skewed?
  - TTL calculation uses local time. Clock skew could cause unexpected behavior, but this is an operational concern outside the scope of this feature.
- What happens when cache is disabled?
  - `expires_at` hints are ignored entirely — no caching occurs regardless of hints. Existing behavior is preserved.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST read the `expires_at` field from plugin projected cost responses and map it to the internal cost result type.
- **FR-002**: The system MUST read the `expires_at` field from plugin actual cost responses and map it to the internal actual cost result type.
- **FR-003**: When a plugin provides a future `expires_at` timestamp, the system MUST use the remaining duration as the cache entry TTL instead of the default TTL.
- **FR-004**: When a plugin provides no `expires_at` (nil/unset), the system MUST fall back to the configured default TTL.
- **FR-005**: When a plugin provides a past `expires_at` timestamp, the system MUST NOT cache the result.
- **FR-006**: When a plugin provides an `expires_at` exceeding the maximum TTL (7 days), the system MUST cap the TTL at the maximum and log a warning.
- **FR-007**: For actual cost responses containing multiple results, the system MUST use the earliest (shortest) `expires_at` across all results when determining the cache TTL for the aggregated response.
- **FR-008**: The system MUST log TTL override decisions at debug level when the plugin-provided TTL differs from the default.

### Key Entities

- **CostResult**: Internal representation of a projected cost response from a plugin. Gains an optional expiration timestamp field.
- **ActualCostResult**: Internal representation of an actual cost response from a plugin. Gains an optional expiration timestamp field.
- **CacheEntry**: Existing cache storage unit with key, data, creation time, expiration time, and TTL. No structural changes needed — the TTL is already per-entry.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Cache entries respect plugin-provided expiration hints with accuracy within 1 second of the specified `expires_at` timestamp.
- **SC-002**: When no expiration hint is provided, cache behavior is identical to the current default TTL behavior (no regression).
- **SC-003**: Stale data (past `expires_at`) is never served from cache — queries for stale-marked data always result in fresh plugin calls.
- **SC-004**: All changed code maintains 80% or higher test coverage.
- **SC-005**: The feature introduces no observable performance overhead for cost queries (cache store/retrieve operations remain under 10ms).
- **SC-006**: Users running cost queries against rate-limited APIs experience fewer redundant API calls when plugins provide long-lived expiration hints.

## Assumptions

- finfocus-spec v0.5.7 is available and provides `expires_at` fields on `GetProjectedCostResponse` (field 13) and `ActualCostResult` (field 8) as `google.protobuf.Timestamp` types.
- The existing `CacheEntry` struct's `ExpiresAt` and `TTLSeconds` fields are sufficient to represent plugin-provided TTLs — no structural changes to the cache entry format are needed.
- Plugins that do not support `expires_at` will return nil/unset for the field, which is the default protobuf behavior for message-type fields.
- The maximum TTL cap of 7 days and minimum TTL behavior for plugin hints are reasonable operational constraints.

## Dependencies

- **Blocked by**: Issue #844 (upgrade finfocus-spec to v0.5.7) — the `expires_at` proto fields must be available before this feature can be implemented.

## Scope Boundaries

**In scope**:

- Reading `expires_at` from proto responses
- Mapping to internal types
- Using plugin hints to set cache TTLs
- Fallback to defaults when hints are absent
- Debug logging of TTL decisions

**Out of scope**:

- Changes to cache storage format or database schema
- User-facing configuration to override plugin hints
- Plugin-side implementation of `expires_at` (plugins set this independently)
- Cache eviction strategy changes
- Recommendations cache TTL (only projected and actual cost caches are affected)
