# Implementation Plan: Batch Actual Cost on Every Actual-Cost Path

**Branch**: `feat/actual-cost-batch` | **Date**: 2026-11-18 | **Spec**: [spec.md](spec.md)

## Summary

Move the batch step out of `GetActualCostWithOptions` into
`Engine.batchActualCosts` (`internal/engine/actual_batch.go`). It groups
resources by primary plugin, keeps batch-capable groups that pass the adapter
and tag filters, splits them by start (window start, or creation time
truncated to the hour), calls `executeBatchForPlugin` per start, widens each
row to the requested window (`keepRequestedWindow`, shared with the unary
path), and returns an `actualBatchVerdict` per resource index. Actual batch
descriptors carry `arn` (`proto.ActualCostARN`). `GetActualCostWithOptions`
reads results and rate limits from it as before.
`GetActualCostWithOptionsAndErrors` passes each resource's verdict to
`getActualCostForResource`, which uses it in place of that plugin's unary
call. Everything after the call (fallback chain, `ErrorDetail`, state
estimate, placeholder) is the existing code, so there is no third copy.

## Technical Context

**Language/Version**: Go 1.27.4
**Primary Dependencies**: finfocus-spec `pluginsdk` v0.7.12, gRPC, zerolog,
testify
**Storage**: Existing actual-cost cache, unchanged
**Testing**: `go test` with the `mockBatchCostSourceClient` fake
**Project Type**: CLI and plugin host
**Performance Goals**: One `BatchCost` call per 100 resources per plugin
instead of one unary call per resource
**Constraints**: No protocol change; non-batch plugins unchanged

## Constitution Check

- [x] **Plugin-First Architecture**: Orchestration only; no plugin knowledge.
- [x] **Test-Driven Development**: Tests in
  `internal/engine/actual_cost_batch_test.go` were written first and failed.
- [x] **Cross-Platform Compatibility**: No OS calls.
- [x] **Documentation Integrity**: `CLAUDE.md` engine notes and
  `architecture/plugin-protocol.md` describe the rule.
- [x] **Protocol Stability**: No protocol change.
- [x] **Implementation Completeness**: No stubs; every batch outcome is
  handled.
- [x] **Persistence Model**: No new state.
- [x] **Web Parity**: `Not applicable`; the web Cost page calls the same
  engine method.
- [x] **Quality Gates**: `make lint`, `make test`, `go test -race` on
  `internal/engine` and `internal/cli`.
- [x] **Multi-Repo Coordination**: None required; the CloudZero plugin
  already implements `BatchCost`.

## Project Structure

### Documentation (this feature)

```text
specs/2017-actual-cost-batch/
├── spec.md
├── plan.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/engine/
├── actual_batch.go            # batchActualCosts, actualBatchVerdict (new)
├── actual_cost_batch_test.go  # engine tests (new)
├── engine.go                  # both entry points call batchActualCosts
└── window_cost_test.go        # new getActualCostForResource argument
```

## Complexity Tracking

None.
