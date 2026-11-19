# Feature Specification: Batch Actual Cost on Every Actual-Cost Path

**Feature Branch**: `feat/actual-cost-batch`
**Created**: 2026-11-18
**Status**: Implemented
**Input**: `cost actual` on a 46-resource state with a plugin that declares
`BATCH_COST` made 50 unary `GetActualCost` calls and no `BatchCost` call.
Only `GetActualCostWithOptions` batched; every user-facing path calls
`GetActualCostWithOptionsAndErrors`, which priced each resource alone.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A batch-capable plugin is asked once per batch (Priority: P1)

A user runs `finfocus cost actual` against a stack whose actual-cost plugin
declares `BATCH_COST`. Core sends the resources in `BatchCost` calls of up to
100 instead of one `GetActualCost` per resource. Plugins with a request budget
(CloudZero allows 60 cost requests a day) stop running out on one stack.

**Independent Test**: An engine with one batch-capable fake plugin prices five
resources through `GetActualCostWithOptionsAndErrors`; the fake records one
`BatchCost` call for the resources that existed all window, one for the two
created in the same hour, and no unary call.

**Acceptance Scenarios**:

1. **Given** a batch-capable plugin, **When** `cost actual`, `cost history
   collect-daily`, or the web Cost page asks for actual cost, **Then** core
   sends `BatchCost` and results keep input order.
2. **Given** resources created inside the window, **When** they are
   batched, **Then** they share one batch per creation hour, start from that
   hour, and each row keeps the requested window with no spend before its
   creation day.
3. **Given** the same query twice with the cache on, **Then** the second is
   served from the cache with no plugin call.
4. **Given** a resource with an ARN in state, **Then** its batch descriptor
   carries `arn`, as the unary request does.

### User Story 2 - Batch failures report like unary failures (Priority: P1)

A per-resource error in a batch response, or a row with no cost data, becomes
an `ErrorDetail` for that resource with the plugin's name, exactly as a failed
unary call would. Core does not ask the same plugin again for that resource.
The next fallback plugin, the state estimate, and the `--fallback-estimate`
placeholder behave as on the unary path.

**Independent Test**: The fake returns an item error for one resource and an
empty actual result for another; the result carries two `ErrorDetail`s
(`ErrNoCostData` for the empty one), a priced row for the third, and no
unary call.

### User Story 3 - Plugins without BATCH_COST are unchanged (Priority: P2)

A plugin without `BATCH_COST` is still called once per resource.

### Edge Cases

- A whole `BatchCost` call fails (transport error, count mismatch): those
  resources fall back to unary calls, as before.
- A batch still rate limited after its retries marks its resources; the
  unary path records `errBatchRateLimited` for that plugin and does not call
  it again.
- A type the plugin marks unsupported in the batch goes to the unary path.
- `--adapter` and `--tag` filters apply before the batch.
- Overview enrichment asks one resource per call with its own window start,
  so it sends one-resource batches: the same call count as before.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `GetActualCostWithOptionsAndErrors` MUST price resources whose
  primary plugin declares `batch_cost` with `BatchCost`.
- **FR-002**: Both actual-cost engine entry points MUST share one batch step.
- **FR-003**: A batch verdict for a plugin MUST replace that plugin's unary
  call for the resource; fallback, state estimate, placeholders, and
  `ErrorDetail`s MUST follow the unary rules.
- **FR-004**: Result order, the cache, `RetainDailyObservations`, the
  rate-limit rule, and unary pricing for plugins without `batch_cost` MUST
  be kept.
- **FR-005**: Resources created inside the window MUST be batched by start
  (creation time truncated to the hour, not before the window start).
- **FR-006**: An actual batch descriptor MUST carry the resource ARN.
- **FR-007**: A batch item that fails with `Unavailable`, or that the batch
  never answered (its call failed, or its descriptor exceeds the SDK
  limits), MUST get no verdict, so that plugin is asked for it alone once.
- **FR-008**: A resource MUST be batched only to the first plugin its
  per-resource chain would ask (ActualCosts route, then `--adapter`), and
  only when that plugin declares `batch_cost`.
- **FR-009**: Daily costs MUST start on the resource's creation day,
  counted from the window start, as on the unary path.

### Key Entities

- **Batch verdict**: per resource index, the plugin that was asked and
  either its result or the error it gave.

## Web UI Fit *(mandatory)*

Not applicable: no new surface. The web Cost page already calls
`GetActualCostWithOptionsAndErrors` (`internal/cli/web_data.go`), so it gets
the batch with no UI change. What users see is unchanged; only the number of
plugin calls drops.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For N resources that existed the whole window on one
  batch-capable plugin, `cost actual` makes ceil(N/100) `BatchCost` calls
  (or more if the plugin hints a smaller `max_batch_size`) and zero unary
  `GetActualCost` calls. The debug log shows `executing batch actual cost`
  and `sending batch cost request`.
- **SC-002**: Existing engine and CLI tests pass unchanged.

## Assumptions

- A plugin's per-item batch answer is the same answer its unary RPC would
  give, so asking again would only spend budget. `Unavailable` is transient,
  so it is the exception (FR-007).
- Starting a mid-window resource's batch at its creation hour rather than
  its exact creation time is acceptable. The plugins that declare
  `batch_cost` (aws-ce, opencost, cloudzero) answer from recorded billing or
  allocation data, which has nothing before a resource exists, so the extra
  hour adds nothing. Plugins without `batch_cost` (aws-public and the other
  list-price plugins) stay unary with the exact creation time, and the state
  estimate uses `pulumi:created`, not the batch start. A plugin that prices
  by elapsed time and declares `batch_cost` would gain at most an hour.
- A plugin's start groups are sent one after another, so a rate limit on one
  stops the rest. A stack deployed across many separate hours makes one
  round trip per hour; that is slower than one batch but far fewer calls
  than one per resource.

## Out of Scope

- Grouping overview rows into one batch (each row has its own window start).
- Any protocol change.
