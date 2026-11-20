# Implementation Plan: Cluster Allocation Web Hook

**Branch**: `2021-cluster-web-hook` | **Date**: 2026-11-19 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2021-cluster-web-hook/spec.md`

## Summary

Give `finfocus --web` the `cost cluster` report through two POST endpoints,
`/api/cluster/query` and `/api/cluster/policy`. `runCostCluster` is split
into command-free steps (parse the query, build the report, build the
policy) that the CLI and the new `SessionOptions.Cluster` and
`SessionOptions.ClusterPolicy` callbacks both call. The JSON contract moves
from `internal/cli` to `internal/engine` so `internal/webui` can return it,
and the display text the CLI table derives (mode line, incomplete
explanation, warning summary) moves to `internal/viewmodel` so both surfaces
share one definition. Errors become typed (`engine.ClusterInputError`,
`engine.ClusterPluginMissingError`) so the CLI maps them to exit 2 or 1 and
the web to 400 or an "unavailable" 200.

## Technical Context

**Language/Version**: Go 1.27.4
**Primary Dependencies**: Cobra, ax-go, gRPC, finfocus-spec v0.7.12 (`UsageSourceService`, `AllocatorService`), testify
**Storage**: N/A (no new state)
**Testing**: `go test` with testify; in-process gRPC fakes over `bufconn` for the parity test; `httptest` for handlers
**Target Platform**: Linux, macOS, Windows (CLI and local dashboard)
**Project Type**: single Go module (CLI plus embedded web server)
**Performance Goals**: no overhead beyond the CLI run; the query reuses open plugins
**Constraints**: `internal/webui` must not import `cli`, `registry`, or `pluginhost`; CLI output byte-identical; no path from the page is opened
**Scale/Scope**: about 10 Go files touched, 2 endpoints, docs reference table

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: orchestration only; usage and
  allocation stay in plugins, and core still never interprets Kubernetes.
- [x] **Test-Driven Development**: builder, display, policy parse, handler,
  and parity tests are written before each step's implementation.
- [x] **Cross-Platform Compatibility**: no OS-specific code.
- [x] **Documentation Integrity**: `reference/web-ui.md` endpoint table and
  CLAUDE.md gotchas are updated in this change.
- [x] **Protocol Stability**: no proto change; the CLI JSON shape is
  unchanged.
- [x] **Implementation Completeness**: no stubs; both endpoints are complete.
- [x] **Persistence Model**: no persistent state added.
- [x] **Web Parity**: the spec carries Web UI Fit; `contracts/http-api.md`
  names every Go method the hook calls; tasks include the render check and
  the follow-up issue.
- [x] **Quality Gates**: `make validate`, `make test`, `make lint`,
  `make test-e2e-web` run before the PR.
- [x] **Multi-Repo Coordination**: none needed; the capacity split (#1840)
  already shipped in core.

**Violations Requiring Justification**: none. Re-checked after Phase 1
design: unchanged.

## Project Structure

### Documentation (this feature)

```text
specs/2021-cluster-web-hook/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/http-api.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
internal/engine/
├── cluster_report.go         # ClusterReport, ClusterPolicy, NewClusterReport (moved from cli)
└── cluster_errors.go         # ClusterInputError, ClusterPluginMissingError
internal/config/
└── allocation_policy.go      # ParseAllocationPolicy (reader + source label)
internal/viewmodel/
└── cluster_report_display.go # ClusterReportDisplay, ClusterModeLine, warning summary, incomplete lines
internal/cli/
├── cost_cluster.go           # flags and runCostCluster over the builders
├── cost_cluster_build.go     # parseClusterQuery, buildClusterReport, buildClusterPolicy
├── cost_cluster_render.go    # renders engine.ClusterReport; text from viewmodel
├── cost_cluster_warnings.go  # stderr printing over the viewmodel summary
├── web_cluster.go            # webData.cluster, webData.clusterPolicy
└── web.go                    # SessionOptions wiring
internal/webui/
├── handlers_cluster.go       # POST /api/cluster/query, POST /api/cluster/policy
├── session.go                # SessionOptions.Cluster, ClusterPolicy
└── server.go                 # registerCluster
docs/src/content/docs/reference/web-ui.md
```

**Structure Decision**: single module. The shared contract goes to `engine`
(next to `ClusterGroup` and `CapacityGroup`), shared display text to
`viewmodel` (which already imports `engine` and backs other web displays),
and the transport stays in `internal/webui`.

## Complexity Tracking

No violations.
