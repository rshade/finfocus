# Feature Specification: Unified Engine Caching System

**Feature Branch**: `592-engine-caching`
**Created**: 2026-02-14
**Status**: Draft
**Input**: Issues #541, #542, #600 (with #543 closed as superseded by #600)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Shared Cache Initialization (Priority: P1)

As a CLI user, I want cache setup to work consistently across all cost commands
(`cost projected`, `cost actual`, `cost recommendations`) so that I only need to
learn one flag (`--cache-ttl`) and one environment variable to control caching
everywhere.

**Why this priority**: This is the foundation. Without a shared initialization
mechanism and a proper abstraction layer, caching cannot be added to new commands
without duplicating 40+ lines of boilerplate. It also enables testability by
allowing mock cache implementations.

**Independent Test**: Can be fully tested by running `cost recommendations` with
`--cache-ttl 3600` and verifying it still works identically to today, then
confirming the same flag is recognized by `cost projected` and `cost actual`.

**Acceptance Scenarios**:

1. **Given** a user runs `finfocus cost recommendations --cache-ttl 3600`,
   **When** results are returned,
   **Then** cache files appear in `~/.finfocus/cache/` and a second identical run
   returns results from cache.
2. **Given** `FINFOCUS_CACHE_TTL=1800` is set in the environment,
   **When** a user runs any cost command without `--cache-ttl`,
   **Then** caching is enabled with an 1800-second TTL.
3. **Given** `--cache-ttl 0` is passed (or no flag and no env var),
   **When** a cost command runs,
   **Then** no cache files are created and all results come from live plugin calls.
4. **Given** the config file sets `cost.cache.ttl_seconds: 7200`,
   **When** `--cache-ttl 900` is also passed,
   **Then** the CLI flag wins (900 seconds used).

---

### User Story 2 - Projected Cost Caching (Priority: P2)

As a user running `finfocus cost projected` on a large stack (500+ resources), I
want repeated runs with the same plan to return results near-instantly so that I
can iterate on output format or grouping without waiting for plugin calls each time.

**Why this priority**: This is the primary demo value target. A 1000-resource
stack returning results in under 1 second on the second run is the key metric for
scale demonstrations.

**Independent Test**: Run `cost projected --pulumi-json plan.json --cache-ttl 3600`
twice. The second run should show cached results (visible via `--debug` logging)
and complete significantly faster.

**Acceptance Scenarios**:

1. **Given** a user runs projected cost with caching enabled,
   **When** the same plan is queried a second time within the TTL window,
   **Then** cached results are returned without making any plugin calls.
2. **Given** a user modifies one resource in a Pulumi plan,
   **When** they re-run projected cost,
   **Then** only the changed resource triggers a plugin call; unchanged resources
   return cached results (per-resource caching).
3. **Given** a cached result exists but the TTL has expired,
   **When** the user re-runs the command,
   **Then** fresh results are fetched from plugins and the cache is updated.
4. **Given** caching is enabled but the cache store encounters a read/write error,
   **When** a cost command runs,
   **Then** the command completes normally using live plugin calls, and a warning
   is logged (cache errors never block cost calculations).

---

### User Story 3 - Actual Cost Caching (Priority: P3)

As a user running `finfocus cost actual` with time range queries, I want repeated
runs with the same parameters to return cached results so that I can iterate on
grouping or output format without hitting rate-limited cloud provider APIs
(e.g., AWS Cost Explorer) each time.

**Why this priority**: Actual cost queries are the most expensive (rate-limited
external APIs). Caching these has the highest impact on user experience and API
cost reduction. Ranked P3 because it depends on the same shared infrastructure
as P1 and P2.

**Independent Test**: Run `cost actual --from 2025-01-01 --to 2025-01-31
--cache-ttl 3600` twice. The second run should return cached results.

**Acceptance Scenarios**:

1. **Given** a user runs actual cost with caching enabled,
   **When** the same query (same resources, time range, tags, adapter, grouping)
   is executed again within the TTL window,
   **Then** cached results are returned without making plugin calls.
2. **Given** a user changes the `--from` or `--to` date range,
   **When** they re-run actual cost,
   **Then** a fresh query is made (different cache key).
3. **Given** a user adds `--filter "tag:env=prod"` to an otherwise identical query,
   **When** they re-run actual cost,
   **Then** a fresh query is made (tags affect the cache key).
4. **Given** a user changes only the `--output` format flag,
   **When** they re-run actual cost,
   **Then** cached results are used (output format is not part of the cache key).

---

### Edge Cases

- What happens when the cache directory doesn't exist? It is created automatically.
- What happens when disk is full? Cache write fails, warning is logged, command
  continues with live results.
- What happens when cache files are corrupted (invalid JSON)? Treated as cache
  miss, warning logged, fresh results fetched and cache overwritten.
- What happens when two concurrent processes write to the same cache key? Atomic
  file writes (temp + rename) prevent corruption.
- What happens when `--cache-ttl` is set to a value below the minimum (60s) or
  above the maximum (7 days)? The existing TTL validation rejects it.
- What happens when a cached cost result is returned? The Adapter field is
  appended with `"(cached)"` so users can distinguish cached from live results.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a cache abstraction that can be substituted with
  alternative implementations (e.g., for testing).
- **FR-002**: System MUST provide a single shared cache initialization mechanism
  used by all cost commands (`cost projected`, `cost actual`,
  `cost recommendations`).
- **FR-003**: The `--cache-ttl` flag MUST be available on all cost commands with
  consistent behavior: 0 = disabled, positive value = enabled with that TTL.
- **FR-004**: Cache enablement precedence MUST be: CLI flag > environment variable
  (`FINFOCUS_CACHE_TTL`) > config file (`cost.cache.ttl_seconds`) > default (0,
  disabled).
- **FR-005**: Projected cost caching MUST use per-resource cache keys incorporating
  provider, resource type, and all resource properties so that partial plan changes
  only invalidate affected resources.
- **FR-006**: Actual cost caching MUST use cache keys incorporating resource types,
  time range, tags, adapter, and grouping parameters so that different queries
  produce different cache entries.
- **FR-007**: Cache errors (read failures, write failures, corrupt data) MUST
  never prevent cost calculations from completing. Errors MUST be logged at
  warning level and the system MUST fall through to live plugin calls.
- **FR-008**: The default TTL constant MUST be defined in exactly one place
  (the cache package) and reused by all consumers. Duplicate definitions in
  other packages MUST be removed.
- **FR-009**: The existing `cost recommendations` caching MUST continue to work
  identically after the refactoring.
- **FR-010**: Cached cost results MUST append `"(cached)"` to the Adapter field
  so users can visually distinguish cached results from live plugin results.

### Key Entities

- **Cache Entry**: A stored cost result with a unique key, JSON data payload,
  creation timestamp, and expiration timestamp.
- **Cache Key**: A deterministic identifier derived from query parameters. Two
  identical queries MUST produce the same key. Different queries MUST produce
  different keys.
- **Cache Store**: The backing storage for cache entries, supporting get, set,
  and enabled-check operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Second run of a projected cost query with the same plan completes
  in under 1 second for stacks with 500+ resources (vs. 10+ seconds for first
  run with live plugin calls).
- **SC-002**: All cost commands (`projected`, `actual`, `recommendations`) accept
  and honor the `--cache-ttl` flag with identical behavior.
- **SC-003**: Cache initialization code exists in exactly one location (no
  duplication across commands).
- **SC-004**: All existing tests pass with no regressions after the refactoring.
- **SC-005**: New cache-related test coverage achieves 80%+ for all new code
  paths.
- **SC-006**: `make lint` and `make test` pass cleanly.

## Clarifications

### Session 2026-02-14

- Q: Should cached results be visually distinguishable from live results? → A: Yes, append "(cached)" to the Adapter field on cache hits.
- Q: Which environment variable name for cache TTL? → A: `FINFOCUS_CACHE_TTL` (matches `--cache-ttl` flag; rename existing `FINFOCUS_CACHE_TTL_SECONDS` constant).

## Assumptions

- The 1-hour default TTL (3600 seconds) from `cache.DefaultTTLSeconds` is
  appropriate for all cost command types.
- Per-resource caching for projected costs is preferred over whole-result-set
  caching because it provides partial cache hits when plans change slightly.
- The `EstimateConfidence` field in `ActualCostRequest` does not affect query
  results and is excluded from cache keys (same as specified in issue #542).
- Output format (`--output table|json|ndjson`) does not affect cached data and
  is excluded from cache keys.
- The existing `FINFOCUS_CACHE_TTL_SECONDS` constant in the cache package will
  be renamed to `FINFOCUS_CACHE_TTL` for consistency with the `--cache-ttl` flag.

## Traceability

| Issue | Title | Role in Feature |
| ----- | ----- | --------------- |
| #541  | Extract Cache interface and refactor FileStore | Foundation: interface + shared init |
| #542  | Add caching to GetActualCost with 1-hour TTL | Actual cost caching |
| #600  | Projected cost caching | Projected cost per-resource caching |
| #543  | ~~GetProjectedCost SHA-based caching~~ | Closed, superseded by #600 |
