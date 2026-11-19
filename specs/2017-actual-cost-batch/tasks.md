# Tasks: Batch Actual Cost on Every Actual-Cost Path

**Input**: [spec.md](spec.md), [plan.md](plan.md)

## Phase 1: Tests first

- [x] T001 [US1] `TestActualCostAndErrorsUsesBatchCost` in
  `internal/engine/actual_cost_batch_test.go`: one `BatchCost` per start,
  no unary call, ARN on the descriptor, input order and window kept.
- [x] T002 [US2] `TestActualCostAndErrorsBatchItemErrorsBecomeErrorDetails`:
  item error and empty row become `ErrorDetail`s, no unary call.
- [x] T003 [US3] `TestActualCostAndErrorsNonBatchPluginStaysUnary`.
- [x] T004 [US1] `TestActualCostAndErrorsBatchResultsAreCached`.

## Phase 2: Implementation

- [x] T005 Add `batchActualCosts` and `actualBatchVerdict` in
  `internal/engine/actual_batch.go`, moved from `GetActualCostWithOptions`.
- [x] T006 `GetActualCostWithOptions` reads results and rate limits from the
  verdicts.
- [x] T007 `GetActualCostWithOptionsAndErrors` passes each verdict to
  `getActualCostForResource`, which uses it in place of the unary call.

- [x] T010 Batch by start (`actualBatchStart`, `keepRequestedWindow`);
  `TestBatchActualCostGroupsResourcesByStart` replaces the unary test.
- [x] T011 Set `arn` on actual batch descriptors (`proto.ActualCostARN`).
- [x] T012 A batch rate limit keeps the plugin's message
  (`batchRateLimitError`); `TestActualCostAndErrorsBatchRateLimitKeepsPluginMessage`.
- [x] T013 An `Unavailable` item gets no verdict and is retried alone
  (FR-007); `resourceBatchError` keeps item gRPC codes;
  `TestActualCostAndErrorsBatchUnavailableItemIsRetriedAlone`.
- [x] T014 A plugin without `batch_cost` gets the exact creation time;
  `TestActualCostAndErrorsNonBatchPluginStaysUnary`.
- [x] T015 Review fixes: unanswered rows go unary
  (`TestActualCostAndErrorsFailedBatchRetryPricesResourcesAlone`,
  `TestActualCostAndErrorsOversizedDescriptorIsPricedAlone`); batch plugin
  from the ActualCosts route and adapter
  (`TestActualCostAndErrorsAdapterBatchesTheNamedPlugin`,
  `TestActualCostAndErrorsBatchFollowsActualCostRoute`); daily costs from
  the resource's own start
  (`TestActualCostAndErrorsBatchKeepsCreationDayOffHourWindow`); item errors
  render like unary (`pluginStatusError`).

## Phase 3: Polish

- [x] T008 `CLAUDE.md` engine notes and
  `docs/src/content/docs/architecture/plugin-protocol.md`.
- [x] T009 `make lint`, `make test`, `go test -race ./internal/engine
  ./internal/cli`.

## Web Parity

Not applicable: the web Cost page calls the same engine method. No `web-ui`
issue.
