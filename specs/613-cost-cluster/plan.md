# Implementation Plan: `finfocus cost cluster`

**Branch**: `613-cost-cluster` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/613-cost-cluster/spec.md`

**Note**: Converted from the superpowers SP3 plan (PR #1522) and re-verified
against `main` on 2026-09-29:

- PR #1510 is **merged** — `resolveOutputFormat`
  (`internal/cli/output_mode.go:102`), `machineOutputRequested`, `writeJSON`,
  `internal/cli/testdata/mcp/tools.golden`, `TestMCPSchemaToolAllowList`, and
  `TestMCPServer_ToolListMatchesGoldenForBothEntryPoints` all exist on `main`.
- `go.mod` pins finfocus-spec **v0.7.0** (superset of v0.6.2): `usage.proto`
  and `allocation.proto` generated types, `pluginsdk.DecodePolicy`,
  `ValidateAllocateRequest`, `ValidateAllocateResponse`, `ResolveCurrency`,
  `CheckConservation`, and `DefaultConservationEpsilon` are available.
- `plugins/kubernetes` (SP2) is merged; `kubernetes-v0.1.0`/`v0.1.1` releases
  exist, so the kind E2E task is unblocked.
- Per the SP3 plan, `VerifyConservation` MUST call
  `pluginsdk.ValidateAllocateResponse` + `pluginsdk.CheckConservation`.

## Summary

Add `finfocus cost cluster`: core orchestrates three small interfaces (usage
source, allocator, pricer) in `internal/engine/cluster.go` so tests inject
fakes without `openPlugins`; nodes are priced by the existing projected-cost
path; grouping is generic aggregation over a string map; the allocation policy
file is resolved by `internal/config` and passed through as opaque JSON; the
Cobra command renders table/JSON/NDJSON and is exposed as an MCP tool; a kind
E2E proves the whole path with real plugins and no AWS credentials.

## Technical Context

**Language/Version**: Go 1.27.1 (see `go.mod`)
**Primary Dependencies**: finfocus-spec v0.7.0 (`pbc` usage/allocation types,
pluginsdk rules), Cobra, ax-go (`ax.ParseConfig`, `axtest.Run`, MCP), gRPC,
testify, kind + kubectl (E2E), `k8s.io/apimachinery` (E2E module only)
**Storage**: N/A (no new persistent state)
**Testing**: `go test` with testify; in-process fake usage-source/allocator
plugins for the pipeline; golden table tests; MCP golden + live `tools/list`
integration test; kind E2E (`test/e2e`, tag `e2e_kind`)
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI binary with gRPC plugin architecture
**Performance Goals**: Paged pod listing in the plugin; existing per-plugin
timeout in core covers slow `GetStats`
**Constraints**: Core never interprets Kubernetes semantics; `$0` price =
unpriced; conservation enforced in core; errors exit 1; stderr for warnings;
every config-touching test isolates `FINFOCUS_HOME`
**Scale/Scope**: ~9 tasks across `internal/pluginhost`, `internal/engine`,
`internal/config`, `internal/cli`, `test/integration`, `test/e2e`, docs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Core adds orchestration only; usage and
  allocation are plugin RPCs. Nodes are priced through the existing
  provider-agnostic engine path.
- [x] **Test-Driven Development**: Every task is test-first (failing test →
  verify failure → implement). Pipeline, grouping, policy resolution,
  rendering, and CLI tests planned before code; 80%/95% coverage targets
  apply. No TUI changes (table output via tabwriter, not Bubble Tea).
- [x] **Cross-Platform Compatibility**: Pure Go orchestration; the kind E2E is
  Linux CI; no platform-specific code.
- [x] **Documentation Integrity**: New guide
  `docs/src/content/docs/guides/cluster-costs.md`, CLAUDE.md gotchas
  subsection, README feature line — all in the same change set as the code;
  `make docs-lint` in verification.
- [x] **Protocol Stability**: Consumes released finfocus-spec (v0.7.0); no
  proto changes in this feature.
- [x] **Implementation Completeness**: Full command including MCP exposure,
  integration tests, and E2E; no stubs or TODOs. Deferred items (historical
  mode, config routing) are explicit non-goals with roadmap tracking, not
  stubs.
- [x] **Persistence Model**: No new stores; policy files are read-only inputs
  resolved per run.
- [x] **Quality Gates**: `make test`, `make lint`, `make docs-lint`, and the
  MCP golden/integration tests before completion.
- [x] **Multi-Repo Coordination**: Consumes the released pluginsdk helpers;
  aws-public provides node pricing unchanged; no cross-repo edits required.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/613-cost-cluster/
├── plan.md              # This file
├── research.md          # Decisions (plugin selection, $0 = unpriced, policy order)
├── data-model.md        # ClusterRequest/Result/Group, clusterOutput JSON
├── contracts/           # CLI output contract + MCP tool
└── tasks.md             # /speckit.tasks output
```

### Source Code (repository root)

```text
internal/pluginhost/host.go          # capability names usage_stats / allocation
internal/engine/cluster.go           # pipeline: UsageSource/Allocator/ResourcePricer
internal/engine/cluster_group.go     # grouping
internal/config/allocation_policy.go # policy resolution (ax.ParseConfig)
internal/cli/cost_cluster.go         # command
internal/cli/cost_cluster_render.go  # table/JSON/NDJSON rendering
internal/cli/root.go                 # register in newCostCmd
internal/cli/testdata/mcp/tools.golden
test/integration/cost_cluster_test.go
test/e2e/kind/                       # setup.sh, workloads.yaml
test/e2e/cluster_kind_test.go        # tag e2e_kind
docs/src/content/docs/guides/cluster-costs.md
```

**Structure Decision**: Single CLI repository; the command and pipeline follow
existing `cost` subcommand patterns and reuse `openPlugins`,
`newEngineWithCache`, `resolveOutputFormat`, and `writeJSON`.

## Design Notes

### Pipeline (`internal/engine/cluster.go`, orchestration only)

```go
type UsageSource interface {
    GetStats(ctx context.Context, in *pbc.GetStatsRequest, opts ...grpc.CallOption) (*pbc.GetStatsResponse, error)
}
type Allocator interface {
    Allocate(ctx context.Context, in *pbc.AllocateRequest, opts ...grpc.CallOption) (*pbc.AllocateResponse, error)
}
type ResourcePricer interface {
    GetProjectedCostWithErrors(ctx context.Context, resources []ResourceDescriptor) (*CostResultWithErrors, error)
}
```

The generated `pbc.UsageSourceServiceClient` / `pbc.AllocatorServiceClient`
satisfy the first two; `*Engine` satisfies the pricer. Entry points:
`RunClusterAllocation`, `ShowAllocationPolicy`, `VerifyConservation` (wraps
`pluginsdk.ValidateAllocateResponse` + `pluginsdk.CheckConservation`),
`PriceableToResource`. Sentinel errors: `ErrConservation`,
`ErrHistoricalUnsupported`.

### Rendering

Table: tabwriter columns `GROUP CPU MEMORY TOTAL NOTES` (`%.2f` money), then a
footer with mode/period, total, idle % (or "omitted (--namespace scoped)"),
policy source + digest, and incomplete resources. JSON/NDJSON shapes are a
stable contract — see `contracts/cost-cluster.md`. Warnings go to stderr.

### E2E (kind, no AWS credentials)

`test/e2e/kind/setup.sh` creates a kind cluster whose nodes are labeled
`node.kubernetes.io/instance-type=m5.large`,
`topology.kubernetes.io/region=us-east-1`, `finfocus.dev/provider=aws` via
`kubectl label`; fixed workloads (Deployment, StatefulSet, DaemonSet, CronJob)
with known requests; real `finfocus` binary + real kubernetes plugin + real
released aws-public (`--metadata region=us-east-1`). Expected costs derive
from the node price the command reports (`priced[]`), not a hardcoded table.
CI job `e2e-kind` via `helm/kind-action` (pin the current major at
implementation time). `make test-e2e-kind` ties it together.

## Complexity Tracking

No constitution violations to justify.
