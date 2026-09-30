# Feature Specification: Kubernetes In-Cluster Cost Allocation

**Feature Branch**: `612-k8s-cost-allocation`
**Created**: 2026-09-24
**Status**: Complete (retrospective Spec Kit conversion of the superpowers design
and plans; SP2 Task 8 — the registry entry — remains open in `tasks.md`)
**Input**: Superpowers design "Kubernetes In-Cluster Cost Allocation — Design"
(2026-09-24, committed in PR #1522 under the since-removed superpowers docs directory)
(committed in PR #1522) and its SP1/SP2/SP3b implementation plans. The design's §8
plan-time amendments are folded into the requirements below; where the original
document's earlier sections disagreed with §8, §8 won.

## Overview

Break Kubernetes cluster costs down by namespace, controller (Deployment,
StatefulSet, DaemonSet, Job), pod, node, and label — first as a current
**run-rate** from a live cluster. Historical actuals arrive once a metrics
backend (Prometheus, Datadog) is supported, and projected costs for
Pulumi-managed `kubernetes:*` resources come later. This feature covers the
delivered slice: the finfocus-spec contracts (SP1), the `plugins/kubernetes`
usage-source + allocator plugin (SP2), and the monorepo registry/release
pipeline (SP3b). The `finfocus cost cluster` command (SP3) is specified
separately in `specs/613-cost-cluster/`.

### Non-goals (this slice)

- Historical actuals (arrives with SP4 Prometheus).
- Spot pricing (aws-public has none; spot nodes are priced on-demand with a note).
- Fargate pricing (reported as `NO_COST_DATA`).
- Shared-cost redistribution (idle, kube-system) beyond "separate rows".
- A snapshot collector that builds history from the Kubernetes API (rejected:
  sampling gaps, stateful plugin, duplicates kube-state-metrics + Prometheus).
- Adopting ax-go reserved exit codes (project-wide decision, out of scope).
- The `finfocus cost cluster` CLI and core pipeline (SP3 → `specs/613-cost-cluster/`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Collect Cluster Usage (Priority: P1)

As a FinOps engineer, I want a plugin that reads my live Kubernetes cluster and
reports per-workload CPU/memory requests and per-node allocatable capacity, so
that cost allocation has accurate usage data without requiring a metrics
backend.

**Why this priority**: Usage data is the input to every allocation; without it
nothing else delivers value. The Kubernetes-API-only source works on any
cluster with no extra infrastructure.

**Independent Test**: Install the `kubernetes` plugin, point it at a kubeconfig
context, and call `GetStats`; the response passes
`plugintesting.ValidateStatsResponse` and lists workload rows (`cpu_request`,
`mem_request`) plus node capacity rows (`cpu_allocatable`, `mem_allocatable`)
and priceable node descriptors.

**Acceptance Scenarios**:

1. **Given** a cluster with Deployments, StatefulSets, DaemonSets, and CronJobs,
   **When** `GetStats` runs in run-rate mode, **Then** every running pod
   contributes workload rows keyed by `namespace`/`pod`/`node`/`controller_kind`/
   `controller`, and pods in `Succeeded`/`Failed` phase are skipped.
2. **Given** a pod with init containers, **When** requests are summed, **Then**
   the Kubernetes effective-request rule applies (max of the largest init
   container and the sum of app containers).
3. **Given** nodes labeled with instance type and region, **When** `GetStats`
   runs, **Then** each node yields a priceable `ResourceDescriptor` whose `id`
   equals the `node` subject of its capacity rows.
4. **Given** a `GetStats` request with a start time (historical mode), **When**
   the plugin receives it, **Then** it returns `InvalidArgument` (run-rate only).

---

### User Story 2 - Allocate Costs by Policy (Priority: P1)

As a FinOps engineer, I want an allocator plugin to divide priced node and
control-plane costs across workloads according to a versioned policy, so that
every dollar of cluster cost lands on exactly one row (workload, idle, or
cluster) and the total is conserved.

**Why this priority**: Allocation is the core value of the feature — turning
node bills into per-workload costs — and it must be provably correct
(conservation invariant) to be trustworthy.

**Independent Test**: Run `plugintesting.RunAllocatorConformance` against the
plugin's allocator; all fixtures (single node, multi-node, empty cluster, fully
packed node, unpriced node, strict policy rejection, empty-usage policy echo,
stable digest) pass.

**Acceptance Scenarios**:

1. **Given** priced nodes and workload usage, **When** `Allocate` runs, **Then**
   each node's cost is split into CPU and memory portions per `node_split`
   (`unit-price-ratio` weights) and each workload is charged
   `max(request, usage) / allocatable` of each portion.
2. **Given** any valid request, **When** `Allocate` returns, **Then** Σ row
   `total_cost` equals Σ `priced=true` cost within relative epsilon 1e-6, and
   the remainder per node appears as a `kind=__idle__` row (never negative,
   never dropped).
3. **Given** a policy document with an unknown field, **When** decoded, **Then**
   the allocator rejects it with `InvalidArgument` naming the JSON path; an
   unknown `version` is likewise rejected.
4. **Given** an empty `usage` and `priced`, **When** `Allocate` runs, **Then**
   the response carries no rows but includes `effective_policy_json` and a
   stable `policy_digest` (backs `--show-policy`).

---

### User Story 3 - Install Monorepo Plugins (Priority: P2)

As a finfocus user, I want `finfocus plugin install kubernetes` to work even
though the plugin is released from the finfocus monorepo under prefixed tags
(`kubernetes-v0.1.0`), so that plugin installation is uniform regardless of
where the plugin's source lives.

**Why this priority**: Without registry and release-pipeline support, the
kubernetes plugin cannot be distributed at all; this unblocks adoption but is
independent of the allocation logic itself.

**Independent Test**: Against an `httptest` GitHub API serving interleaved CLI
(`v0.3.9`) and plugin (`kubernetes-v0.1.0`) releases, install and update a
`tag_prefix` registry entry; the plugin lands in
`~/.finfocus/plugins/kubernetes/v0.1.0/` and `ListLatestPlugins` discovers it.

**Acceptance Scenarios**:

1. **Given** a registry entry with `tag_prefix: "kubernetes-"`, **When** the
   latest release is resolved, **Then** the highest semver among stable releases
   carrying the prefix wins (publish order ignored, prereleases skipped, up to
   100 releases scanned) instead of `/releases/latest`.
2. **Given** a user runs `plugin install kubernetes@v0.1.0` (or `@0.1.0` or
   `@kubernetes-v0.1.0`), **When** the release is fetched, **Then** tag
   `kubernetes-v0.1.0` is used and the installed/canonical version is `v0.1.0`.
3. **Given** a prefixed plugin tag is published, **When** CI runs, **Then** the
   CLI release workflows (GoReleaser, nightly, `git describe`) ignore the tag
   and the plugin asset workflow builds/uploads the standard asset names.
4. **Given** a registry entry without `tag_prefix`, **When** installed, **Then**
   behavior is exactly as before (all pre-existing registry tests unchanged).

---

### User Story 4 - Engine `Supports` Routing (Priority: P2)

As a plugin author, I want the engine's `Supports` checks to send provider,
resource type, SKU, and region — and to respect my plugin's answer — so that a
usage-only plugin can decline pricing queries instead of being called and
failing on every resource.

**Why this priority**: Since finfocus-spec v0.6.2 a plugin's `Supports` answer
reaches the host; sending an incomplete descriptor would make region-bound
plugins (aws-public) answer "unsupported" for everything and break pricing.

**Independent Test**: `internal/engine` tests assert the `Supports` request
carries provider/type/SKU/region and that the cache key includes them; a
region-bound fake plugin declines other regions instead of being called.

**Acceptance Scenarios**:

1. **Given** a resource in `us-east-1` and one in `us-west-2`, **When** the
   engine checks `Supports` against a region-bound plugin, **Then** each region
   gets its own answer (no cache poisoning across regions or SKUs).
2. **Given** a plugin that never implements `SupportsProvider`, **When** the
   SDK returns `Supported:false` with
   `pluginsdk.DefaultSupportsNotImplementedReason`, **Then** the engine treats
   that reason as fail-open (cached `true`), same as an RPC error.
3. **Given** the `kubernetes` plugin is installed, **When** `cost projected`
   runs, **Then** no per-resource "not supported" errors are recorded for it
   (it declares only `USAGE_STATS` and `ALLOCATION` and answers
   `Supports=false`).

---

### Edge Cases

- **Spot nodes**: priced on-demand with the note `"spot node priced on-demand"`;
  detected via `eks.amazonaws.com/capacityType` or `karpenter.sh/capacity-type`.
- **Fargate**: pods on Fargate get their own `$0` rows with the note
  `"Fargate pricing not supported yet"` and never become idle; the usage source
  omits Fargate nodes' capacity rows and priceable entries (allocator detects
  the `fargate-` node-name prefix).
- **Unpriced node**: aws-public answers unknown instance types with `$0`, not an
  error; a priceable whose monthly cost is `<= 0` (or carries a structured
  error) is treated as unpriced — its rows are `$0` with a note and the total is
  marked incomplete. All nodes unpriced is fatal.
- **Zero or missing allocatable** (cordoned/NotReady node): the whole node cost
  goes to `__idle__` — never NaN, never lost.
- **Overcommitted node** (requests above allocatable): shares are scaled down so
  idle is never negative.
- **Pending pods** (no `nodeName`) are skipped; pods on a node missing from the
  priced set get `$0` rows with a note.
- **Duplicate pod names** across namespaces stay separate rows (aggregation key
  is `namespace/pod`).
- **Empty cluster** (no pods): valid; all cost is idle.
- **Policy edge cases**: `{}`, comments-only, or missing `version` decode to
  defaults and digest identically to no policy; a discovered policy file that
  fails to parse is fatal (never silently falls back to defaults).
- **Mixed currencies**: `InvalidArgument` from the SDK request validation.

## Requirements *(mandatory)*

### Functional Requirements

**Contracts (delivered by finfocus-spec v0.6.2 via
rshade/finfocus-spec#505, #506, #507, #510, #518)**

- **FR-001**: finfocus-spec MUST define `UsageSourceService.GetStats` and
  `AllocatorService.Allocate` protos (`usage.proto`, `allocation.proto`) with
  `StatsMode`, `UsageRow`, `PricedResource`, and `AllocationRow` messages;
  changes MUST be additive (`buf breaking` passes).
- **FR-002**: `PluginCapability` MUST gain `PLUGIN_CAPABILITY_USAGE_STATS = 14`
  and `PLUGIN_CAPABILITY_ALLOCATION = 15`; `IsValidCapability` bounds MUST cover
  them.
- **FR-003**: pluginsdk MUST serve the new services in gRPC mode, Connect mode,
  and the grpchealth static checker when the plugin implements
  `UsageSourceProvider`/`AllocatorProvider` (protos alone would ship dead
  services — plan-time amendment §8.1).
- **FR-004**: pluginsdk MUST provide the well-known subject/kind/metric
  constants (`SubjectCluster`, `SubjectNamespace`, `SubjectControllerKind`,
  `SubjectController`, `SubjectPod`, `SubjectNode`, `SubjectKind`,
  `SubjectLabelPrefix`; `KindWorkload`, `KindNode`, `KindIdle`, `KindCluster`;
  `MetricCPURequest`, `MetricMemRequest`, `MetricCPUAllocatable`,
  `MetricMemAllocatable`, `MetricCPUUsage`, `MetricMemUsage`).
- **FR-005**: pluginsdk MUST own the shared allocation rules so they are not
  implemented twice (§8.14): `DecodePolicy` (strict decode, unknown fields
  reported by JSON path), `ValidateAllocateRequest` (nonzero cost on unpriced
  resources, duplicate `(kind, id)`, mixed currencies → `InvalidArgument`),
  `ValidateAllocateResponse`, `ResolveCurrency` (an empty currency takes the
  others' single currency; all empty → `USD`), and `CheckConservation` with
  `DefaultConservationEpsilon` (1e-6 relative).
- **FR-006**: plugintesting MUST provide `ValidateStatsResponse` (subject-key
  conformance) and `RunAllocatorConformance` (conservation fixtures).
- **FR-007**: A plugin's `Supports` answer MUST reach the host without a custom
  `RegistryLookup` (finfocus-spec #507/#510); plugins that do not implement
  `SupportsProvider` return `Supported:false` with
  `DefaultSupportsNotImplementedReason`.

**Kubernetes plugin (`plugins/kubernetes`, SP2)**

- **FR-008**: The plugin MUST be a nested Go module
  (`github.com/rshade/finfocus/plugins/kubernetes`) that does not import any
  `github.com/rshade/finfocus/(internal|pkg)` package, enforced by
  `make check-plugin-boundaries`.
- **FR-009**: The plugin MUST declare exactly `PLUGIN_CAPABILITY_USAGE_STATS`
  and `PLUGIN_CAPABILITY_ALLOCATION` explicitly, because SDK capability
  inference always adds projected/actual/pricing/estimate (§8.8), and MUST
  implement `Supports` returning `false` so hosts never route price queries to
  it (§8.7).
- **FR-010**: The usage source MUST use client-go with standard kubeconfig
  loading (`KUBECONFIG`, in-cluster; context from `scope`), list pods paged via
  `Limit`/`Continue`, skip `Succeeded`/`Failed` pods, sum container requests
  using the Kubernetes effective-request rule, and resolve owners to the
  top-level controller (Pod → ReplicaSet → Deployment, Job → CronJob,
  StatefulSet, DaemonSet, bare Pod).
- **FR-011**: The usage source MUST emit node rows (`kind=node`) with
  allocatable CPU/memory and priceable descriptors using `resource_type`
  `aws:ec2/instance:Instance` with `sku` and `region` (passed as engine
  properties, §8.6); the provider comes from the `finfocus.dev/provider` label,
  else the `providerID` scheme. It MUST emit an `eks` control-plane descriptor
  when the cluster name can be determined, and MUST detect spot and Fargate
  capacity from the documented labels.
- **FR-012**: The allocator MUST decode policy via `pluginsdk.DecodePolicy` onto
  its defaults, reject unknown `version`, treat enum fields as single-value
  (policy v1 has no list fields, so the documented "lists replace" rule has
  nothing to test yet, §8.10), and return a stable hex SHA-256 digest of the
  canonical effective policy.
- **FR-013**: The allocator MUST split node cost into CPU/memory portions per
  `node_split`, charge each workload `max(request, usage) / allocatable`, emit
  the remainder per node as a `kind=__idle__` row, emit the control plane as a
  `kind=__cluster__` row, and return the effective policy and digest. Run-rate
  allocation uses ratios only — usage is NOT scaled by 730 hours; costs are
  monthly because nodes are priced at projected monthly cost (§8.4).
- **FR-014**: Every request or policy rejection MUST be `InvalidArgument`; SDK
  request/currency errors are returned unchanged (they already carry
  `InvalidArgument`).
- **FR-015**: The plugin MUST log to stderr only (stdout is the pluginsdk
  handshake) and MUST ship a README documenting RBAC (with a minimal
  ClusterRole in `deploy/clusterrole.yaml`), node labels, and the policy
  reference.

**Registry and release pipeline (SP3b)**

- **FR-016**: `registry.json` entries MUST accept an optional `tag_prefix`
  matching `^[a-uw-z0-9][a-z0-9-]*-$` — prefixes MUST NOT start with `v`, since
  CLI workflows treat `v*` tags as CLI releases (§8.9) — and the capabilities
  `usage_stats` and `allocation`.
- **FR-017**: For prefixed entries, "latest" MUST mean the highest semver among
  stable releases carrying the prefix (scanning up to 100 releases, skipping
  prereleases), never `/releases/latest`.
- **FR-018**: Install directories, asset names, recorded versions, and update
  comparisons MUST use the canonical version (tag minus prefix, e.g. `v0.1.0`
  via `CanonicalVersion`); `ReleaseTag` MUST map bare `0.1.0`, `v0.1.0`, and
  `kubernetes-v0.1.0` to the prefixed tag.
- **FR-019**: The release pipeline MUST build plugin assets via
  `scripts/release-plugin-assets.sh` (not GoReleaser) with the standard asset
  names plus `checksums.txt`; plugin releases MUST be un-marked as the repo's
  "Latest" (the CLI release is re-marked); GoReleaser gets explicit
  current/previous CLI tags; `git describe` uses `--match 'v[0-9]*'`; and
  release-tag values reach `run:` only via `env:` (no shell injection) (§8.9).
- **FR-020**: Entries without `tag_prefix` MUST behave exactly as before.

**Engine `Supports` routing (SP1 Task 5)**

- **FR-021**: `checkPluginSupports` MUST send provider, resource type, SKU, and
  region, and MUST cache per
  client+provider+type+region+SKU+feature so a SKU-less or region-different
  resource never reuses another resource's answer.
- **FR-022**: The engine MUST fail open (cached `true`) only for RPC errors and
  for plugins on SDKs older than v0.6.2 (identified by
  `DefaultSupportsNotImplementedReason`).

### Key Entities *(include if feature involves data)*

- **UsageRow**: one usage measurement; `subject` string map (`cluster`,
  `namespace`, `controller_kind`, `controller`, `pod`, `node`, `label.<key>`,
  `kind`), `metric`, `amount` (per-hour rate in run-rate mode), `unit`
  (`core`, `GiB`).
- **PricedResource**: a `ResourceDescriptor` (node or control plane) plus
  `cost`, `currency`, `priced` flag, and `note` for the normalized period.
- **AllocationRow**: one allocated cost; `subject` map with `kind` ∈
  {`workload`, `__idle__`, `__cluster__`}, `cpu_cost`, `mem_cost`,
  `total_cost`, `currency`, `note`.
- **Allocation policy**: plugin-owned versioned document (`allocation.hujson`
  on the user side; strict JSON on the wire): `version`, `idle`,
  `system_workloads`, `node_split`, `charge`, `control_plane`, `spot_nodes`.
- **Registry entry `tag_prefix`**: marks a plugin released from the monorepo
  under prefixed tags; drives canonical-version install semantics.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `GetStats` against a live cluster returns a response that passes
  `plugintesting.ValidateStatsResponse` with workload and node rows for every
  schedulable node.
- **SC-002**: `RunAllocatorConformance` passes against the real allocator, and
  every `AllocateResponse` conserves cost within relative epsilon 1e-6.
- **SC-003**: Allocator package coverage ≥ 95%; plugin package coverage ≥ 80%.
- **SC-004**: `make check-plugin-boundaries` proves `plugins/kubernetes` has no
  dependency on finfocus core packages.
- **SC-005**: A `kubernetes-vX.Y.Z` release installs via
  `finfocus plugin install kubernetes` into a canonical semver directory and is
  discovered by `plugin list`.
- **SC-006**: With the kubernetes plugin installed, `cost projected` records
  zero "not supported" errors attributable to it.
- **SC-007**: Registry entries without `tag_prefix` pass all pre-existing
  registry tests unchanged.
