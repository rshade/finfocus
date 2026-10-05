# Implementation Plan: Overview Cluster Expansion

**Branch**: `fix/issue-1526-overview-cluster-expansion` | **Date**: 2026-10-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/623-overview-cluster-expansion/spec.md`

## Summary

Expand Kubernetes cluster rows (EKS/GKE/AKS) in `finfocus overview` into
workload rows. Two expansion sources feed one row shape: the **projected**
path re-parents Pulumi-declared workload rows (priced per spec 621) under
the cluster row, and the **live** path runs `engine.RunClusterAllocation`
(spec 613) per cluster and attaches synthetic namespace-allocation rows.
Live wins when both exist; suppressed projected rows are footnoted. The
engine gains pure grouping/merge functions; the CLI orchestrates live calls;
the TUI gains per-cluster expand/collapse; JSON/NDJSON gain `parentUrn`,
`expansionSource`, and `childUrns` fields pinned by golden tests.

## Technical Context

**Language/Version**: Go 1.27.1
**Primary Dependencies**: Cobra, zerolog, bubbletea v2 (TUI), finfocus-spec SDK v0.7.5 (UsageSource/Allocator protos)
**Storage**: N/A (optional `~/.finfocus/config.yaml` gains an `overview.cluster_contexts` mapping)
**Testing**: `go test ./...`, testify assert/require, golden files under `testdata/overview/golden` and `internal/tui/testdata`
**Target Platform**: Linux/macOS/Windows CLI
**Project Type**: single Go CLI (core in `internal/`, plugins in `plugins/`)
**Performance Goals**: at most one extra plugin round trip per cluster row (SC-004)
**Constraints**: expansion is never fatal; no new dependency in core (no client-go — context resolution passes a scope string to the usage-source plugin, which owns kubeconfig parsing)
**Scale/Scope**: single-cluster-per-stack grouping; multi-cluster stacks documented as a limitation

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Orchestration logic in core; all Kubernetes
  knowledge stays in plugins (usage source parses kubeconfig, allocator splits
  cost, kubernetes plugin prices declared workloads). Core only matches type
  tokens, matching the precedent of `internal/proto/adapter.go`'s EKS SKU fallback.
- [x] **Test-Driven Development**: Unit tests planned for detection, grouping,
  live/projected merge, and scope resolution before implementation; TUI changes
  get golden-file snapshots plus a render-and-read verification (Principle II).
- [x] **Cross-Platform Compatibility**: Pure Go; golden comparisons normalize CRLF
  per the existing `assertGoldenFile` pattern.
- [x] **Documentation Integrity**: Godoc on new exported identifiers; spec status
  tracking updated; plugin READMEs untouched (no plugin changes).
- [x] **Protocol Stability**: No finfocus-spec changes; consumes existing protos.
- [x] **Implementation Completeness**: Both paths fully implemented; the live path
  is not stubbed because spec 613 is merged.
- [x] **Persistence Model**: No new persistent state; the config mapping is
  optional and commands work without it.
- [x] **Quality Gates**: `make lint`, `make test` must pass.
- [x] **Multi-Repo Coordination**: None — consumes finfocus-spec v0.7.5 as-is.

**Violations Requiring Justification**: none.

## Project Structure

### Documentation (this feature)

```text
specs/623-overview-cluster-expansion/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (JSON/NDJSON expansion contract)
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/engine/
├── overview_types.go        # OverviewRow gains ParentURN, ExpansionSource, ChildURNs
├── overview_cluster.go      # NEW: cluster/workload detection, projected grouping, live merge
├── overview_cluster_test.go # NEW: unit tests (detection, grouping, merge)
├── overview_render.go       # table: child indent + footnotes; aggregation skips live rows
├── overview_enrich.go       # unchanged (expansion runs post-enrichment)
internal/config/
├── config.go                # OverviewConfig{ClusterContexts map[string]string}
internal/cli/
├── overview.go              # plain path: expansion step after enrichment
├── overview_cluster.go      # NEW: live-expansion orchestration + scope resolution
├── overview_cluster_test.go # NEW: scope resolution + orchestration tests
├── overview_integration_test.go  # golden tests for expanded JSON/NDJSON/table
internal/tui/
├── overview_model.go        # expansion state, e/→/← keys, display-order mapping
├── overview_view.go         # expander marker + indented children + footnote line
├── overview_golden_test.go  # expanded/collapsed golden views
testdata/overview/
├── state-cluster-expansion.json      # NEW fixture: cluster + declared workloads
└── golden/                           # NEW goldens: table/json/ndjson expanded
```

**Structure Decision**: Single Go project; engine holds pure functions
(detection, grouping, merging) so both the plain and TUI CLI paths share one
implementation, and the live path slots in behind the same row shape.

## Complexity Tracking

No constitution violations.
