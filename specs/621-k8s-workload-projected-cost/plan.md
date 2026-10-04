# Implementation Plan: Kubernetes Workload Projected Cost

**Branch**: `621-k8s-workload-projected-cost` | **Date**: 2026-10-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/621-k8s-workload-projected-cost/spec.md`

## Summary

Core sends every projected-cost descriptor's declared properties as the
generic `ResourceDescriptor.attributes` (finfocus-spec v0.7.3), redacted, size
bounded, part of the cache key, and split across `BatchCost` requests by size.
The kubernetes plugin reads Deployment, StatefulSet, DaemonSet, Job, and
CronJob pod specs from those attributes and prices declared requests at
rates from its own environment. It explains every refusal through a
`Supports` decline, the only plugin text core keeps in a `NO_COST_DATA` note
(research R1).

## Technical Context

**Language/Version**: Go 1.27.1 (core, `plugins/kubernetes`, `plugins/jev`)
**Primary Dependencies**: finfocus-spec v0.7.3 (`pluginsdk.AttributeValue`,
`MaxAttributesBytes`), `google.golang.org/protobuf/types/known/structpb`,
`k8s.io/apimachinery/pkg/api/resource` (already in the plugin module)
**Storage**: Projected cost cache keys gain `/attrs-<digest>`; no new store
**Testing**: `go test` with testify; integration tests in `test/integration`
that build the real plugin
**Target Platform**: Linux, macOS, Windows (amd64, arm64)
**Project Type**: CLI plus nested plugin module
**Performance Goals**: attributes are built for nearly every resource, so
(a) every projected cache key gains `/attrs-…` and the cache misses once after
upgrade (the cache is optional and rebuilds), and (b) `Supports` is called
about once per distinct resource per plugin instead of once per type, SKU,
and region. Both are measured by benchmarks on `aws-simple-plan.json`
**Constraints**: core has no Kubernetes logic (FR-008); a batch request stays
under 3 MiB; attributes at most 65536 bytes
**Scale/Scope**: plans of up to thousands of resources; five workload kinds

## Constitution Check

- [x] **Plugin-First Architecture**: Kubernetes logic is only in
  `plugins/kubernetes`; core gains a generic data channel.
- [x] **Test-Driven Development**: Tests are written first for each unit
  (attributes builder, cache keys, chunking, shape, estimate, config).
  Critical engine paths keep 95% coverage.
- [x] **Cross-Platform Compatibility**: Pure Go; the integration test builds
  the plugin with the platform's `.exe` suffix, as today.
- [x] **Documentation Integrity**: Plugin README, the cluster-costs guide, the
  upgrade guide, and CLAUDE.md gotchas are updated in the same change.
- [x] **Protocol Stability**: Consumes an additive v0.7.3 field; no protocol
  change here. The `plugin upgrade` hop is added.
- [x] **Implementation Completeness**: No stubs; every decline path is real
  and tested.
- [x] **Persistence Model**: The cache stays optional; the key change only
  avoids wrong hits.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`,
  `make lint`, `make test-kubernetes`, and the integration tests.
- [x] **Multi-Repo Coordination**: rshade/finfocus-spec#617 (released in
  v0.7.3) is the dependency.

No violations.

## Project Structure

### Documentation (this feature)

```text
specs/621-k8s-workload-projected-cost/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── host-attributes.md
│   └── plugin-config.md
└── tasks.md
```

### Source Code

```text
go.mod, plugins/kubernetes/go.mod, plugins/jev/go.mod   # finfocus-spec v0.7.3
internal/history/cost_plan.go        # export IsPulumiSecret
internal/engine/attributes.go        # BuildAttributes, attrsCacheSuffix (new)
internal/engine/engine.go            # Supports descriptor + key; projected descriptor; cache key
internal/engine/engine_batch.go      # attributes on projected batch; size chunking
internal/engine/pricing_spec.go      # attributes on GetPricingSpec
internal/engine/pricing_discovery.go # attributes on discovery
internal/proto/adapter.go            # proto.ResourceDescriptor.Attributes
internal/proto/refs.go               # PrepareProjectedDescriptor copies it
internal/pluginupgrade/hops.go       # v0.7.3 hop
agent-skills/finfocus-plugin-upgrade/references/to-v0.7.3.md
plugins/kubernetes/config.go         # Config from environment (new)
plugins/kubernetes/workload/         # shape extraction, quantities, estimate (new)
plugins/kubernetes/plugin.go         # Info, Supports, GetProjectedCost
plugins/kubernetes/cmd/main.go       # load Config
test/integration/cost_cluster_test.go  # extend: workloads priced and declined
examples/plans/k8s-workloads-plan.json # fixture (new)
```

**Structure Decision**: Core changes stay in the existing engine and proto
packages. The plugin gets a `workload` package so `plugin.go` stays a thin
gRPC surface, matching `allocate/` and `usage/`.

## Complexity Tracking

None.
