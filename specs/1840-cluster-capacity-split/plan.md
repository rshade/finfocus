# Implementation Plan: Cluster Spend by Capacity Type

**Branch**: `feat/1840-cluster-capacity-split` | **Date**: 2026-11-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/1840-cluster-capacity-split/spec.md`

## Summary

`RunClusterAllocation` already holds the allocator rows and the usage source's
priceable descriptors. After conservation is verified it joins each row's
`node` subject to the descriptor id, reads the `capacity_type` tag, and
aggregates with `GroupClusterByCapacity` (plain string-map grouping). The
result lands in `ClusterResult.Capacity`; `cost cluster` renders it as a table
section and a `capacity` JSON array. Run-rate "spot priced on-demand" is not a
core rule: the allocator already notes it on the node's rows, and the group
reports those notes.

## Technical Context

**Language/Version**: Go 1.27.3
**Primary Dependencies**: Cobra, ax-go, testify; no new modules, no
finfocus-spec bump
**Storage**: None
**Testing**: testify table tests in `internal/engine` and `internal/cli`
**Constraints**: Core never interprets Kubernetes and keeps no spot table;
`plugins/*` unchanged and not imported
**Scale/Scope**: One grouping function, one result field, one render section

## Constitution Check

- [x] **Plugin-First Architecture**: Generic tag grouping; no provider logic.
- [x] **Test-Driven Development**: Tests in `cluster_capacity_test.go` and
  `cost_cluster_render_test.go` cover mixed nodes, idle, conservation, JSON.
- [x] **Cross-Platform Compatibility**: No OS-specific code.
- [x] **Documentation Integrity**: `guides/cluster-costs.md` updated.
- [x] **Protocol Stability**: No protocol change.
- [x] **Implementation Completeness**: No stubs.
- [x] **Persistence Model**: No new state.
- [x] **Web Parity**: Web UI Fit records `Not applicable` with the reason.
- [x] **Quality Gates**: `make lint`, `make test`, `make validate`.
- [x] **Multi-Repo Coordination**: None.

**Violations Requiring Justification**: None.

## Project Structure

```text
specs/1840-cluster-capacity-split/
├── spec.md
├── plan.md
├── tasks.md
└── contracts/cli.md
```

```text
internal/engine/cluster_capacity.go       # CapacityGroup, GroupClusterByCapacity
internal/engine/cluster_capacity_test.go
internal/engine/cluster.go                # ClusterResult.Capacity
internal/cli/cost_cluster_render.go       # capacity JSON field and table section
internal/cli/cost_cluster_render_test.go
docs/src/content/docs/guides/cluster-costs.md
```

**Structure Decision**: The join lives next to the other cluster grouping code
in `internal/engine`. The issue names `internal/viewmodel/cluster_display.go`,
but that file holds Overview row pagination, not cluster allocation, so no
viewmodel change is made.

## Complexity Tracking

No violations.
