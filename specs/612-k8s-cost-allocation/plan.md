# Implementation Plan: Kubernetes In-Cluster Cost Allocation

**Branch**: `612-k8s-cost-allocation` | **Date**: 2026-09-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/612-k8s-cost-allocation/spec.md`

**Note**: Retrospective plan converted from the superpowers SP1/SP2/SP3b plans
(committed in PR #1522 under the since-removed superpowers docs directory). All work
described here is delivered except SP2 Task 8 (the registry
entry), which is the single open task in `tasks.md`. The `cost cluster` command
(SP3) is planned separately in `specs/613-cost-cluster/`.

## Summary

Deliver Kubernetes in-cluster cost allocation as three independently testable
sub-projects:

- **SP1 (finfocus-spec, issues only)**: file one fully specified issue per RPC
  (`UsageSourceService.GetStats`, `AllocatorService.Allocate`) plus the
  `Supports` default-registry fix; consume the released finfocus-spec v0.6.2;
  make the engine's `Supports` checks send provider/type/SKU/region.
- **SP2 (`plugins/kubernetes`)**: a nested Go module serving run-rate `GetStats`
  (client-go, Kubernetes-API only) and policy-driven `Allocate` (idle and
  control-plane rows, conservation guaranteed), built from three pure packages
  (`policy`, `allocate`, `usage`) behind thin gRPC glue.
- **SP3b (registry + release)**: registry `tag_prefix` support with canonical
  versions, and a monorepo plugin release pipeline that never collides with
  the CLI's tags.

## Technical Context

**Language/Version**: Go 1.27.1 (see `go.mod`)
**Primary Dependencies**: finfocus-spec v0.6.2+ (usage/allocation protos and
pluginsdk helpers), gRPC, Connect, `k8s.io/client-go` + `k8s.io/api` +
`k8s.io/apimachinery` (plugin only), `github.com/Masterminds/semver/v3`,
zerolog, testify
**Storage**: N/A (no new persistent state)
**Testing**: `go test` with testify; client-go `kubernetes/fake` for the usage
source; `httptest` GitHub API for the registry; SP1 conformance suites from
plugintesting (`ValidateStatsResponse`, `RunAllocatorConformance`)
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI binary with gRPC plugin architecture; plugin is a
nested Go module shipped via the registry from the monorepo
**Performance Goals**: Paged pod listing (`Limit`/`Continue`, page size 500);
existing per-plugin timeout in core covers slow `GetStats`
**Constraints**: Plugin module must not import finfocus core packages
(`make check-plugin-boundaries`); units are CPU cores and GiB (2^30 bytes);
run-rate mode only (historical requests → `InvalidArgument`); allocation is
ratio-based (no 730-hour usage scaling); logging to stderr only
**Scale/Scope**: 3 sub-projects; ~15 delivered tasks across finfocus-spec
(issues), `plugins/kubernetes/`, `internal/registry/`, `internal/engine/`, and
CI/release workflows

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: The feature IS two new plugin types
  (usage source, allocator); core stays Kubernetes-agnostic — nodes arrive as
  ordinary `ResourceDescriptor`s and are priced through the existing engine
  path. The kubernetes plugin is a nested module forbidden from importing core
  (enforced by `make check-plugin-boundaries`).
- [x] **Test-Driven Development**: Every plan task is test-first (failing test,
  verify failure, implement). Allocator coverage target 95%, packages ≥ 80%.
  No TUI changes in this slice.
- [x] **Cross-Platform Compatibility**: Plugin assets build for linux/darwin
  amd64+arm64 and windows amd64 (`scripts/release-plugin-assets.sh`,
  `CGO_ENABLED=0`); client-go and the SDK are cross-platform.
- [x] **Documentation Integrity**: `plugins/kubernetes/README.md` (RBAC, node
  labels, policy reference) shipped with the plugin; CONTRIBUTING.md documents
  monorepo plugin releases; CLAUDE.md gotchas updated for `checkPluginSupports`
  and registry `tag_prefix`.
- [x] **Protocol Stability**: All proto changes additive (`buf breaking`
  passes); new capabilities are enum values 14/15; delivered via a semver
  finfocus-spec release (v0.6.2) consumed through `go.mod`, never a merged
  `replace` directive.
- [x] **Implementation Completeness**: No stubs; the plugin serves real
  `GetStats`/`Allocate`; the one deferred item (SP2 Task 8, registry entry) is
  an open task in `tasks.md` gated on a published `kubernetes-v0.1.0` release,
  tracked as issue #1534.
- [x] **Persistence Model**: No new persistent stores. Plugin install
  directories under `~/.finfocus/plugins/<name>/<version>/` follow the existing
  registry model.
- [x] **Quality Gates**: `make test` and `make lint` required per task;
  `make test-kubernetes`, `make lint-kubernetes`, and
  `make check-plugin-boundaries` wired into `test`/`lint`; actionlint and
  shellcheck cover the new workflow and script.
- [x] **Multi-Repo Coordination**: Contract work filed as finfocus-spec issues
  (#505, #506, #507), released as v0.6.2, then consumed by core and the
  plugin; aws-public spot pricing deferred to its own repo.

**Violations Requiring Justification**: None. (A depguard rule for
`plugins/kubernetes` was proposed but not added — `.golangci.yml` changes
require explicit approval; the `go list -deps` make check covers the boundary.)

## Project Structure

### Documentation (this feature)

```text
specs/612-k8s-cost-allocation/
├── plan.md              # This file
├── research.md          # Decisions with rationale (incl. §8 amendments)
├── data-model.md        # UsageRow/PricedResource/AllocationRow, policy, tag_prefix
├── contracts/           # References to the finfocus-spec v0.6.2 protos
└── tasks.md             # Delivered tasks checked; SP2 Task 8 open
```

### Source Code (repository root)

```text
plugins/kubernetes/          # nested Go module (SP2)
├── policy/                  # defaults, strict decode, canonical digest
├── allocate/                # usage + priced + policy → rows (conserving)
├── usage/                   # client-go listing → usage rows + priceable descriptors
├── deploy/clusterrole.yaml  # minimal RBAC
├── cmd/main.go              # entry point (explicit capabilities)
└── plugin.go, kubeconfig.go # pluginsdk glue

internal/registry/           # SP3b: tag_prefix, canonical versions, prefix latest
internal/engine/engine.go    # SP1 Task 5: Supports sends provider/type/SKU/region
scripts/release-plugin-assets.sh
.github/workflows/release-monorepo-plugin.yml
```

**Structure Decision**: Single CLI repository with a nested plugin module under
`plugins/kubernetes/` (same model as `plugins/recorder`); registry changes live
in `internal/registry/`; contract work happens in finfocus-spec via issues and
is consumed as a released module version.

## Sub-Project Execution Summary

### SP1 — finfocus-spec issues and adoption (Tasks T001–T005 in tasks.md)

Filed rshade/finfocus-spec#505 (`GetStats`), #506 (`Allocate`), and #507
(`Supports` default-registry fix) as fully specified issues; finfocus-spec
v0.6.2 (released 2026-09-28) delivers both services with pluginsdk serving in
gRPC and Connect modes, the health checker, capability inference, subject/kind
/metric constants, `DecodePolicy`, `ValidateAllocateRequest`,
`ValidateAllocateResponse`, `ResolveCurrency`, `CheckConservation`, and the
plugintesting harness. Core bumped both `go.mod` and `test/e2e/go.mod` to
v0.6.2, and `checkPluginSupports` now sends provider/type/SKU/region with a
cache key that includes all of them plus the feature.

### SP2 — `plugins/kubernetes` (Tasks T100–T700 in tasks.md)

- `policy`: defaults per spec, strict decode via `pluginsdk.DecodePolicy`,
  canonical JSON + hex SHA-256 digest.
- `allocate`: node cost split by `unit-price-ratio` weights
  (`cpu_core_hour` 0.031611, `mem_gib_hour` 0.004237), workload charge
  `max(request, usage) / allocatable`, per-node `__idle__` rows, `__cluster__`
  rows, SDK-owned request/currency validation, deterministic output order,
  conservation within 1e-6.
- `usage`: kubeconfig/in-cluster clients, paged pod listing, phase filtering,
  effective-request rule, owner resolution to top-level controllers, node and
  EKS control-plane descriptors, spot/Fargate detection.
- Glue: `pluginsdk.BasePlugin` with explicit capabilities
  `[USAGE_STATS, ALLOCATION]`, `Supports` always false, `cmd/main.go` serving
  via `pluginsdk.Serve`; conformance suites run against the real plugin.
- README + minimal ClusterRole; Makefile targets (`build/test/lint/install-
  kubernetes`, `check-plugin-boundaries`), CI wiring, release-please component
  `kubernetes` producing `kubernetes-vX.Y.Z` tags.

### SP3b — registry and release pipeline (Tasks T200–T204 in tasks.md)

`RegistryEntry.TagPrefix` with pattern validation; `HintsForEntry`,
`CanonicalVersion`, `ReleaseTag`; `GetLatestReleaseWithPrefix` scanning 100
stable releases; install/fallback/update paths on canonical versions;
`usage_stats`/`allocation` registry capabilities;
`scripts/release-plugin-assets.sh` + `release-monorepo-plugin.yml`; CLI
workflows (`goreleaser.yml`, `nightly.yml`, Makefile `git describe`) ignore
non-`v*` tags.

## Complexity Tracking

No constitution violations to justify.
