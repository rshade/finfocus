# Tasks: Cluster Spend by Capacity Type

**Input**: `specs/1840-cluster-capacity-split/`
**Tests**: Required. Testify only.

## Phase 1: Engine

- [X] T001 Write table-driven tests in `internal/engine/cluster_capacity_test.go`: mixed spot, on-demand, untagged, control-plane row, no tags, empty tag value, no rows; every case checks group totals sum to the row total
- [X] T002 Write `RunClusterAllocation` tests: tagged nodes through the fake allocator give capacity totals equal to `Total` and idle counted once; namespace-scoped run has `idle_cost` 0
- [X] T011 Write a same-node-name-in-two-clusters test and assert CPU and memory per bucket
- [X] T003 Implement `CapacityGroup`, `capacityByNode`, `GroupClusterByCapacity` in `internal/engine/cluster_capacity.go`
- [X] T004 Add `ClusterResult.Capacity` and fill it in `newClusterResult` in `internal/engine/cluster.go`

## Phase 2: CLI

- [X] T005 Write render tests in `internal/cli/cost_cluster_render_test.go`: table section lines, section omitted when all unlabeled, JSON field names and `[]` when empty, NDJSON summary carries `capacity`, `newClusterOutput` copies the field
- [X] T006 Add `capacity` to `clusterOutput` and the table section in `internal/cli/cost_cluster_render.go`

## Phase 3: Docs and verification

- [X] T007 Update `docs/src/content/docs/guides/cluster-costs.md` (section and JSON field list)
- [X] T008 Web parity: confirm no `internal/webui` handler uses cluster allocation; Web UI Fit records `Not applicable`
- [X] T009 Run `cost cluster` against a single-node kind cluster, unlabeled and then labeled `karpenter.sh/capacity-type=spot`, with an isolated `FINFOCUS_HOME`. A mixed multi-node cluster was not run (kind multi-node creation failed); the mixed and `unlabeled` cases are covered by unit tests only
- [X] T010 `make lint`, `make test`, `go vet ./...`, `go test -race -shuffle=on -count=3` on changed packages, markdownlint
