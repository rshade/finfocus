# Tasks: Kubernetes In-Cluster Cost Allocation

**Input**: Design documents from `/specs/612-k8s-cost-allocation/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are
MANDATORY and must be written BEFORE implementation. All code changes must
maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness),
all tasks MUST be fully implemented. Stub functions, placeholders, and TODO
comments are strictly forbidden.

**Status note**: This is a retrospective task list converted from the
superpowers SP1/SP2/SP3b plans (PR #1522). Every task is delivered and checked
except **T303 (SP2 Task 8, registry entry)**, which is gated on a published
`kubernetes-v0.1.0` release and tracked as issue #1534.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions
- Original superpowers plan task numbers are cited as `(SPx Task N)`

---

## Phase 1: Setup (Contract Definition via finfocus-spec Issues)

**Purpose**: Define the `GetStats`/`Allocate` contracts as fully specified
finfocus-spec issues — one per RPC — so the released protocol, not this repo,
owns them. No code is written in finfocus-spec from this effort.

- [X] T001 [US1] Draft the `GetStats` issue body (proto `usage.proto`,
  `PLUGIN_CAPABILITY_USAGE_STATS = 14`, subject/kind/metric constants,
  `UsageSourceProvider`, gRPC + Connect + health-check serving,
  `plugintesting.ValidateStatsResponse`, explicit-capabilities rule for
  usage-only plugins) — filed as rshade/finfocus-spec#505 (SP1 Task 1)
- [X] T002 [US2] Draft the `Allocate` issue body (proto `allocation.proto`,
  `PLUGIN_CAPABILITY_ALLOCATION = 15`, contract rules incl. conservation and
  strict policy, `AllocatorProvider`,
  `plugintesting.RunAllocatorConformance`/`CheckConservation` fixtures) —
  filed as rshade/finfocus-spec#506, linked to #505 (SP1 Task 2)
- [X] T003 File both issues on rshade/finfocus-spec after user confirmation
  (plus #507, the `Supports` default-registry fix); all three closed by
  finfocus-spec v0.6.2 (SP1 Task 3)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Consume the released contracts and fix engine `Supports` routing
before any pricing plugin upgrades to finfocus-spec v0.6.2.

- [X] T004 Bump `github.com/rshade/finfocus-spec` to v0.6.2 in `go.mod`/`go.sum`
  and `test/e2e/go.mod`/`go.sum`; verify `make build && make test && make lint`
  and compare `make test-integration` failure set against a clean-main baseline
  (SP1 Task 4)
- [X] T005 [US4] Send provider, type, SKU, and region in
  `checkPluginSupports`/`filterUnsupportedPlugins` (`internal/engine/engine.go`),
  cache per client+provider+type+region+SKU+feature, fail open only for RPC
  errors and `pluginsdk.DefaultSupportsNotImplementedReason`; tests in
  `internal/engine/engine_supports_test.go`; CLAUDE.md Engine gotcha updated
  (SP1 Task 5)

**Checkpoint**: finfocus-spec v0.6.2 provides every name SP2/SP3 use;
region-bound plugins answer `Supports` correctly.

---

## Phase 3: User Story 1 - Collect Cluster Usage (Priority: P1) 🎯 MVP

**Goal**: The `usage` package turns a live cluster into `UsageRow`s and
priceable node/control-plane descriptors.

**Independent Test**: `go test ./usage/` in `plugins/kubernetes` with
client-go `kubernetes/fake`; `GetStats` responses pass
`plugintesting.ValidateStatsResponse`.

### Tests for User Story 1 (MANDATORY - TDD Required) ⚠️

- [X] T101 [P] [US1] Failing tests for effective requests and owner resolution
  (`plugins/kubernetes/usage/requests_test.go`, `owners_test.go`): init
  container rule, phase filtering, Pod → ReplicaSet → Deployment, Job →
  CronJob, StatefulSet, DaemonSet, bare Pod (SP2 Task 3)
- [X] T102 [P] [US1] Failing tests for the collector and node descriptors
  (`plugins/kubernetes/usage/collector_test.go`, `nodes_test.go`): paging,
  `providerID` parsing, `finfocus.dev/provider` override, instance-type/region
  labels with beta fallbacks, spot and Fargate labels, EKS control-plane
  descriptor, RBAC `Forbidden` (SP2 Task 4)

### Implementation for User Story 1

- [X] T103 [P] [US1] Implement `EffectiveRequests` and `OwnerIndex.Resolve` in
  `plugins/kubernetes/usage/requests.go` and `owners.go`; pin matched
  `k8s.io/api`/`apimachinery`/`client-go` minors (SP2 Task 3)
- [X] T104 [US1] Implement `Collect`, `NodeDescriptor`, and
  `ControlPlaneDescriptor` in `plugins/kubernetes/usage/collector.go` and
  `nodes.go` (paged listing via `Limit`/`Continue`, `listPageSize = 500`;
  Fargate nodes emit neither capacity rows nor priceable entries) (SP2 Task 4)

**Checkpoint**: `GetStats` returns valid run-rate usage from a fake clientset.

---

## Phase 4: User Story 2 - Allocate Costs by Policy (Priority: P1)

**Goal**: The `policy` and `allocate` packages divide priced resources across
workloads with conservation guaranteed, plus the pluginsdk glue that serves
both services.

**Independent Test**: `go test ./policy/ ./allocate/` (allocator coverage
≥ 95%) and `plugintesting.RunAllocatorConformance` against the real plugin.

### Tests for User Story 2 (MANDATORY - TDD Required) ⚠️

- [X] T111 [P] [US2] Failing tests for policy decode/validate/canonical digest
  (`plugins/kubernetes/policy/policy_test.go`): empty/`{}`/comment-only inputs
  digest like defaults, overrides merge, strict rejection with JSON paths,
  unknown version, single-value enums, stable digest (SP2 Task 1)
- [X] T112 [P] [US2] Failing tests for allocation
  (`plugins/kubernetes/allocate/allocate_test.go`): zero allocatable → all
  idle, overcommitted node → idle ≥ 0, unpriced/spot/Fargate notes, duplicate
  pod names across namespaces, deterministic order, conservation (SP2 Task 2)
- [X] T113 [P] [US2] Failing plugin glue tests
  (`plugins/kubernetes/plugin_test.go`): historical mode rejected
  (`InvalidArgument`), unknown context, `ValidateStatsResponse` on a real
  response, explicit capabilities only, conformance suites pass (SP2 Task 5)

### Implementation for User Story 2

- [X] T114 [US2] Scaffold the nested module `plugins/kubernetes/go.mod`
  (`github.com/rshade/finfocus/plugins/kubernetes`, go 1.27.1, finfocus-spec
  and testify versions matching root) and implement
  `plugins/kubernetes/policy/policy.go` (`Defaults`, `Decode` via
  `pluginsdk.DecodePolicy`, `Validate`, `Canonical`) (SP2 Task 1)
- [X] T115 [US2] Implement `Allocate` in
  `plugins/kubernetes/allocate/allocate.go`: `pluginsdk.ValidateAllocateRequest`
  first, `ResolveCurrency`, `unit-price-ratio` node split (weights 0.031611 /
  0.004237), `max(request, usage) / allocatable` charge, per-node `__idle__`
  rows, `__cluster__` rows, effective policy + digest; all rejections
  `InvalidArgument` (SP2 Task 2)
- [X] T116 [US2] Implement plugin glue: `plugins/kubernetes/plugin.go`,
  `kubeconfig.go`, `cmd/main.go`, `plugin.manifest.json` — `BasePlugin` with
  explicit capabilities `[USAGE_STATS, ALLOCATION]`, `Supports` always false,
  `ClusterFactory` injection, stderr-only logging (SP2 Task 5)

**Checkpoint**: The plugin binary serves `GetStats` and `Allocate` over
pluginsdk with conformance suites green.

---

## Phase 5: User Story 3 - Install Monorepo Plugins (Priority: P2)

**Goal**: Registry and release pipeline support for plugins released from the
finfocus monorepo under prefixed tags.

**Independent Test**: `httptest` GitHub API tests in `internal/registry/`;
`scripts/release-plugin-assets.sh` verified against the recorder plugin.

### Tests for User Story 3 (MANDATORY - TDD Required) ⚠️

- [X] T201 [P] [US3] Failing tests for `CanonicalVersion`, `ReleaseTag`,
  `HintsForEntry`, and `tag_prefix` validation (`internal/registry/version_test.go`,
  `entry_test.go`) (SP3b Task 1)
- [X] T202 [P] [US3] Failing tests for prefix latest-release selection
  (`internal/registry/github_prefix_test.go`): highest semver beats publish
  order, prereleases skipped, 100-release scan past core releases, no-match
  error (SP3b Task 2)
- [X] T203 [P] [US3] Failing install/fallback/update tests on canonical
  versions (`internal/registry/installer_prefix_test.go`): canonical install
  dir discoverable by `ListLatestPlugins`, bare/prefixed version resolution,
  prefix-filtered fallback, update compares canonical versions (SP3b Task 3)
- [X] T204 [P] [US3] Failing test allowing `usage_stats` and `allocation`
  registry capabilities (`internal/registry/registry_json_test.go`) (SP3b
  Task 4)

### Implementation for User Story 3

- [X] T205 [US3] Add `RegistryEntry.TagPrefix` + pattern
  `^[a-uw-z0-9][a-z0-9-]*-$`, `AssetNamingHints.TagPrefix`, `HintsForEntry`,
  `CanonicalVersion`, `ReleaseTag` (`internal/registry/entry.go`, `github.go`,
  `installer.go`, `version.go`) (SP3b Task 1)
- [X] T206 [US3] Implement `selectLatestByPrefix` and
  `GetLatestReleaseWithPrefix` with `prefixedReleaseScan = 100`
  (`internal/registry/github.go`) (SP3b Task 2)
- [X] T207 [US3] Route install/fallback/update through `fetchRelease` and
  canonical versions (`internal/registry/installer.go`, `github.go`,
  `internal/cli/plugin_install.go`); unprefixed entries unchanged (SP3b Task 3)
- [X] T208 [US3] Allow `usage_stats`/`allocation` capabilities and validate
  `tag_prefix` for every embedded `registry.json` entry
  (`internal/registry/registry_json_test.go`) (SP3b Task 4)
- [X] T209 [US3] Release pipeline: `scripts/release-plugin-assets.sh`
  (verified against the recorder, shellcheck clean),
  `.github/workflows/release-monorepo-plugin.yml` (tags via `env:` only,
  plugin releases un-marked as "Latest"), CLI workflows ignore non-`v*` tags
  (`goreleaser.yml`, `nightly.yml`, Makefile `git describe --match 'v[0-9]*'`,
  `ci.yml`), CONTRIBUTING.md monorepo-release section (SP3b Task 5)

**Checkpoint**: A `kubernetes-vX.Y.Z` release installs via
`finfocus plugin install kubernetes` into a canonical semver directory.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, build/CI wiring, and distribution of the plugin.

- [X] T301 [P] Write `plugins/kubernetes/README.md` (install, kubeconfig,
  RBAC, node labels, policy reference, row kinds, limitations) and
  `plugins/kubernetes/deploy/clusterrole.yaml` (minimal ClusterRole);
  markdownlint passes (SP2 Task 6)
- [X] T302 Makefile targets (`build/test/lint/install-kubernetes`,
  `check-plugin-boundaries` — boundary probe verified to fail when violated),
  CI wiring (`test`/`lint`/`validate`/`build` jobs, go.mod sync extended to
  `plugins/kubernetes`), release-please component `kubernetes` with
  `include-component-in-tag` producing `kubernetes-vX.Y.Z` (SP2 Task 7)
- [ ] T303 Add the `kubernetes` registry entry to
  `internal/registry/registry.json` (`tag_prefix: "kubernetes-"`,
  `asset_hints.asset_prefix: "finfocus-plugin-kubernetes"`, capabilities
  `["usage_stats", "allocation"]`, real `min_spec_version`) and verify
  `finfocus plugin install kubernetes` against the published release —
  **OPEN**: gated on SP3b merge, the SP3 "no pollution" integration test, and
  a published `kubernetes-v0.1.0` release with assets; tracked as issue #1534
  (SP2 Task 8)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — finfocus-spec issue drafts.
- **Foundational (Phase 2)**: Depends on the finfocus-spec v0.6.2 release
  (closes the Phase 1 issues). A local `replace` directive was allowed during
  development and never merged.
- **US1/US2 (Phases 3–4)**: Depend on Phase 2 (generated types + SDK helpers).
- **US3 (Phase 5)**: No dependency on other stories — ran in parallel.
- **Polish (Phase 6)**: T301/T302 depend on Phases 3–4; T303 additionally
  depends on Phase 5 merge and the first `kubernetes-v0.1.0` release.

### Gates (from the plan index)

- **finfocus-spec 051 + 052 → SP2/SP3**: the bump in T004 must be a release
  containing both the generated types and the SDK helpers (`DecodePolicy`,
  `ValidateAllocateRequest`, `ResolveCurrency`, `CheckConservation`).
- **SP3b → T303**: the registry entry lands only after the `tag_prefix`
  support is merged and a prefixed release exists.

---

## Notes

- The `cost cluster` command (SP3) is a separate feature: `specs/613-cost-cluster/`.
- Future sub-projects (SP4 Prometheus, SP5 Datadog, SP6 OpenCost, projected
  costs for `kubernetes:*`, spot/Fargate pricing) are roadmap items, not tasks
  here.
- Repo rule (CLAUDE.md): never `git commit`; each task's final step is
  stage-and-hand-off with a proposed conventional-commit message.
