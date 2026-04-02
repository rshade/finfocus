# Feature Specification: BatchCost RPC Consumer for Multi-Resource Queries

**Feature Branch**: `608-batch-cost-consumer`
**Created**: 2026-03-31
**Status**: Draft
**Input**: GitHub Issue #846 — Implement BatchCost RPC consumer to send multiple resource descriptors in a single gRPC call instead of N individual calls, enabling major performance optimization for large Pulumi stacks.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Batch Cost Estimation for Large Stacks (Priority: P1)

As a FinFocus user with a large Pulumi stack (50+ resources), I want cost queries to be batched into a single plugin call so that cost estimation completes in seconds rather than minutes.

Currently, a stack with 100 resources requires 100 individual gRPC round-trips per plugin. BatchCost reduces this to 1-2 calls by sending all resource descriptors together, enabling plugin-side query optimization, reduced gRPC overhead, and better rate limit management.

**Why this priority**: This is the core value proposition — without batch processing, the feature has no reason to exist. Every other story depends on this working correctly.

**Independent Test**: Can be fully tested by running `finfocus cost projected` against a multi-resource Pulumi plan with a batch-capable plugin, and verifying that all resources receive cost results identical to the per-resource path.

**Acceptance Scenarios**:

1. **Given** a Pulumi stack with 50 resources and a plugin that supports batch cost, **When** the user runs `finfocus cost projected`, **Then** the system sends a single BatchCost request containing all 50 resource descriptors and returns individual cost results for each resource.
2. **Given** a Pulumi stack with 200 resources and a plugin with max batch size of 100, **When** the user runs cost estimation, **Then** the system automatically chunks resources into batches of 100 and sends 2 BatchCost requests.
3. **Given** a Pulumi stack with resources and a plugin that supports batch cost, **When** the user runs `finfocus cost actual --start 2026-03-01 --end 2026-03-31`, **Then** the system sends a BatchCost request with actual cost query type and the specified date range, returning historical cost data for each resource.

---

### User Story 2 - Graceful Fallback for Non-Batch Plugins (Priority: P1)

As a FinFocus user with plugins that do not support batch cost, I want the system to automatically fall back to per-resource cost queries so that existing workflows continue to work without any configuration changes.

**Why this priority**: Equal to P1 because backward compatibility is non-negotiable. Users should not need to change anything when upgrading — batch is a transparent optimization.

**Independent Test**: Can be fully tested by running cost estimation with a plugin that does not advertise batch cost capability and verifying that behavior is identical to the current per-resource path.

**Acceptance Scenarios**:

1. **Given** a plugin that does not support batch cost capability, **When** the user runs cost estimation, **Then** the system uses the existing per-resource query path with no behavior change.
2. **Given** a batch-capable plugin that returns a batch-level gRPC error (e.g., Unimplemented, Unavailable), **When** the system receives the error, **Then** it falls back to per-resource queries for all resources targeted at that plugin and logs a warning.

---

### User Story 3 - Partial Failure Handling (Priority: P2)

As a FinFocus user, when some resources in a batch fail while others succeed, I want to see results for the successful resources and clear error information for the failed ones, rather than losing all results due to a single failure.

**Why this priority**: Partial failures are common in real-world scenarios (unsupported resource types, missing pricing data). Handling them gracefully is critical for user trust but depends on the batch path (P1) working first.

**Independent Test**: Can be tested by submitting a batch containing a mix of supported and unsupported resource types and verifying that supported resources return cost data while unsupported ones show appropriate error messages.

**Acceptance Scenarios**:

1. **Given** a batch of 10 resources where 3 are unsupported types, **When** the batch response returns errors for those 3 resources, **Then** the system shows cost results for the 7 successful resources and logs errors with resource context for the 3 failures.
2. **Given** a batch response where a resource error indicates `resource_type_unsupported`, **When** the system processes results, **Then** it skips that resource with the same behavior as current per-resource validation skip.
3. **Given** a batch of resources where one resource causes a transient error, **When** the system processes partial results, **Then** successful results are preserved and the failed resource is reported with its error details.

---

### User Story 4 - Batch Capability Visibility (Priority: P3)

As a FinFocus user managing multiple plugins, I want to see which plugins support batch cost queries so that I can understand the performance characteristics of my setup.

**Why this priority**: Informational only — useful for debugging and planning but does not affect core functionality.

**Independent Test**: Can be tested by running `finfocus plugin list` with a mix of batch-capable and non-batch plugins, verifying the output shows batch cost capability status.

**Acceptance Scenarios**:

1. **Given** an installed plugin that reports `PLUGIN_CAPABILITY_BATCH_COST`, **When** the user runs `finfocus plugin list`, **Then** the output includes batch cost in the plugin's capability list.
2. **Given** an installed plugin without batch cost capability, **When** the user runs `finfocus plugin list`, **Then** batch cost does not appear in that plugin's capabilities.

---

### Edge Cases

- What happens when a plugin reports batch capability but returns `Unimplemented` at call time? System must fall back to per-resource queries.
- What happens when a batch response returns `max_batch_size` of 0? System should treat it as "no limit" and use the default batch size.
- What happens when all resources in a batch fail? System should report all individual errors, same as if each had failed individually in per-resource mode.
- What happens when the batch request exceeds the context deadline? System should treat it as a batch-level error and fall back to per-resource queries if time remains, or propagate the timeout if no time remains.
- What happens with router-based multi-plugin setups? Resources must be grouped by their target plugin before batching — each plugin receives only its matched resources.
- What happens when a resource matches multiple plugins with fallback? The primary plugin should receive the resource in its batch first; if the batch result for that resource is nil/empty, it should fall back to the next plugin (per existing fallback semantics).

## Clarifications

### Session 2026-03-31

- Q: How should cache interact with batch requests — pre-check to exclude cached resources, or send all resources regardless? → A: Pre-check cache. Exclude already-cached resources from batch requests; only uncached resources are sent to the plugin. Individual results from the batch response are cached independently after receipt.
- Q: Should dry-run be supported in BatchCost requests, or is it out of scope? → A: Exclude dry-run. Batch supports only projected and actual query types. Dry-run continues using the existing separate DryRun RPC.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose BatchCost as a callable method through the internal plugin client interface, delegating to the underlying protocol service.
- **FR-002**: System MUST detect whether a plugin supports batch cost by checking for the `PLUGIN_CAPABILITY_BATCH_COST` capability before attempting batch queries.
- **FR-003**: System MUST group resources by their target plugin (using the router when configured) before constructing batch requests, so each plugin receives only its matched resources.
- **FR-004**: System MUST support projected and actual cost query types in batch requests, passing appropriate date ranges for actual cost queries. Dry-run is out of scope and continues using the existing separate DryRun RPC.
- **FR-005**: System MUST respect the `max_batch_size` value from batch responses and chunk subsequent requests accordingly, using a default batch size of 100 when no prior response has been received.
- **FR-006**: System MUST handle partial failures by processing successful results and reporting per-resource errors individually, preserving results from resources that succeeded.
- **FR-007**: System MUST fall back to per-resource queries when a plugin does not advertise batch cost capability.
- **FR-008**: System MUST fall back to per-resource queries when a batch-level gRPC error occurs (e.g., `Unimplemented`, `Unavailable`, `Internal`).
- **FR-009**: System MUST map batch response results back to the correct source resources using the resource ordering in the request.
- **FR-010**: System MUST display batch cost capability in the plugin list output when a plugin reports it.
- **FR-011**: System MUST treat `resource_type_unsupported` errors in batch responses identically to current per-resource validation skip behavior.
- **FR-012**: System MUST pass the full context deadline to batch calls, allowing the plugin to manage its own internal concurrency and timeouts.
- **FR-013**: System MUST check the cache for each resource before including it in a batch request, excluding already-cached resources from the batch. Individual results from the batch response MUST be cached independently after receipt.

### Key Entities

- **BatchCostRequest**: A collection of resource descriptors with a query type (projected/actual) and optional date range, sent to a single plugin in one call.
- **BatchCostResponse**: A collection of per-resource results (either cost data or error), plus a `max_batch_size` hint for future chunking.
- **ResourceCostResult**: A single resource's outcome from a batch — either successful cost data (with variants for projected, actual, estimate, dry-run) or an error with resource context.
- **CostQueryType**: Enumeration distinguishing between estimate, actual, and projected cost queries within a batch.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Cost estimation for a stack of 100 resources completes with fewer network round-trips to each plugin when that plugin supports batch cost (target: 1-2 calls instead of 100).
- **SC-002**: All resources in a batch receive the same cost results as they would through individual per-resource queries (result parity).
- **SC-003**: When a plugin does not support batch cost, the user experience and performance are identical to the current behavior (zero regression).
- **SC-004**: When some resources in a batch fail, at least 95% of the batch's successful results are preserved and displayed to the user (partial failure resilience).
- **SC-005**: Users can identify which installed plugins support batch cost by inspecting plugin list output.
- **SC-006**: New code achieves 80% or greater test coverage, with critical paths (capability detection, chunking, fallback, partial failure) at 95% or greater.

## Assumptions

- The finfocus-spec dependency has been upgraded to a version that includes the `BatchCost` RPC definition (currently v0.6.0 in go.mod, which includes it).
- The `PLUGIN_CAPABILITY_BATCH_COST` capability enum value and routing feature are already wired in the codebase (confirmed: pluginhost, router, and features all support it).
- Plugins implementing `BatchCost` maintain result ordering consistent with request ordering (i.e., result at index N corresponds to resource at index N in the request).
- The default batch size of 100 is a reasonable starting point and will be adjusted based on real-world plugin feedback.
- Cache pre-check applies to batch: already-cached resources are excluded from batch requests, and individual results from the batch response are cached independently after receipt. This aligns with existing per-resource cache behavior.
