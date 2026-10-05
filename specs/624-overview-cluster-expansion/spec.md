# Feature Specification: Overview Cluster Expansion

**Feature Branch**: `fix/issue-1526-overview-cluster-expansion`
**Created**: 2026-10-04
**Status**: Implemented
**Input**: GitHub issue #1526

## Context

`finfocus overview` renders a flat table of the resources in a Pulumi stack,
one row per resource, priced individually. When a stack provisions an EKS,
GKE, or AKS cluster, that row today shows only the control-plane cost; the
workloads actually running inside the cluster, and the node cost they
consume, are invisible. The overview command is the primary place users look
at whole-stack cost, so cluster resources should expand into workload rows.

Two data sources already exist and this feature consumes both:

- **Live allocation** (spec 613, merged): `engine.RunClusterAllocation`
  prices a live cluster's nodes and allocates their cost to workloads via a
  usage-source plugin and an allocator plugin, grouped by namespace by
  default.
- **Projected workload cost** (spec 621, merged): the kubernetes plugin
  prices Pulumi-declared `Deployment` / `StatefulSet` / `DaemonSet` / `Job` /
  `CronJob` resources from configured rates, so declared workloads already
  appear as individually priced overview rows.

### Resolved design questions

The issue asked for these decisions to be resolved in this spec:

1. **Cluster resource → kubeconfig context mapping.** Resolution order:
   (a) an explicit mapping in `config.hujson`
   (`overview.cluster_contexts`, keyed by cluster name or full URN →
   kubeconfig context); (b) a kubeconfig context whose name equals the
   cluster resource's `name` property, its full ARN, or the name segment of
   its ARN; (c) when the stack declares exactly one cluster and neither
   mapping matches, the kubeconfig current-context is used and the expansion
   is footnoted as assumed. Core does not read kubeconfig: it asks the usage
   source for each candidate in order and stops at the first one that
   answers. An explicit mapping is the only candidate when present. When no
   context resolves, live expansion is skipped for that cluster (debug log
   only; a failed explicit mapping is a warning) and projected expansion may
   still apply.
2. **Live vs. projected precedence.** When live expansion succeeds for a
   cluster, the projected workload rows grouped under that cluster are
   suppressed (not shown as children, not double-counted in totals) and a
   footnote states the suppression, matching the existing state-only `*`
   footnote convention. When live expansion is unavailable or fails, the
   projected rows are the children. Live allocation children are excluded
   from cost totals: they re-allocate node cost already represented by other
   rows; projected children keep their costs in totals because they were
   ordinary overview rows before expansion.
3. **TUI expand/collapse.** A cluster row with children shows an expander
   marker (`▸` collapsed, `▾` expanded). Clusters start collapsed; pressing
   `e`, `→` (expand), or `←` (collapse) on a selected cluster row toggles
   it. Children render indented beneath their parent. In the plain table
   renderer children are always shown, indented with a `↳` prefix.
4. **JSON/NDJSON parent/child shape.** Rows stay a flat list consistent with
   `OverviewRow` conventions in `internal/engine/overview_types.go`. A child
   row carries `parentUrn` and `expansionSource` (`"live"` or
   `"projected"`); a cluster row with children carries `childUrns` in
   display order. Live children are synthetic rows whose URN is
   `<clusterURN>#ns/<namespace>` and whose type is
   `finfocus:k8s/namespace:Allocation`.

### Dependency note

Both dependencies are merged: `RunClusterAllocation` exists in
`internal/engine/cluster.go` (spec 613), and the kubernetes plugin prices
the five workload kinds from Pulumi plans (spec 621). Both the live and the
projected expansion paths are therefore implemented in this feature. The
code is structured so the context-resolution rules above are the only
coupling between the two paths.

## User Scenarios & Testing

### User Story 1 - Declared workloads nest under their cluster (Priority: P1)

A platform engineer has one Pulumi stack that declares an EKS cluster and
several `kubernetes:apps/v1:Deployment` resources. They run
`finfocus overview`. The cluster row is expandable; the declared workloads
appear as its children instead of disconnected flat rows, and each child
keeps the projected cost the kubernetes plugin already computed.

**Why this priority**: This is the case the issue was filed for — the same
Pulumi program manages both the cluster and its workloads — and it requires
no live cluster, no extra plugins, and no configuration.

**Independent Test**: Run overview on a fixture stack containing one cluster
resource and two declared workloads with no usage-source or allocator plugin
installed. Assert the cluster row lists two child URNs, each child row
carries the cluster URN as its parent reference and `projected` as its
expansion source, and the summary totals are unchanged from a run without
expansion.

**Acceptance Scenarios**:

1. **Given** a stack with one EKS cluster and two Deployments, **When**
   overview runs in JSON mode, **Then** the cluster row contains
   `childUrns` referencing both workloads and each workload row contains
   `parentUrn` pointing at the cluster and `expansionSource: "projected"`.
2. **Given** the same stack, **When** overview runs in the TUI, **Then** the
   cluster row shows a collapsed expander and pressing the expand key
   reveals the two workload rows indented beneath it.
3. **Given** a stack with a cluster but no declared workloads and no
   usage/allocator plugins, **When** overview runs, **Then** the cluster row
   renders exactly as it does today — no expander, no error, unchanged
   totals.
4. **Given** a stack with declared workloads but no cluster resource,
   **When** overview runs, **Then** the workloads render as ordinary flat
   rows exactly as today.

---

### User Story 2 - Live allocation expands a cluster (Priority: P2)

An engineer has a stack declaring a GKE cluster, a usage-source plugin and
an allocator plugin installed, and a kubeconfig context matching the
cluster's name. They run `finfocus overview` and expand the cluster row to
see per-namespace allocation rows — including idle capacity — sourced from
the live cluster.

**Why this priority**: Live data is the most accurate view of what runs in
the cluster, but it requires plugins and a reachable cluster, so it builds
on the P1 projected path.

**Independent Test**: With a fake usage source and allocator, run overview
expansion for a cluster row and assert namespace-grouped synthetic child
rows appear with `expansionSource: "live"`, and that the summary totals do
not add live child costs.

**Acceptance Scenarios**:

1. **Given** a cluster whose name matches a kubeconfig context and both
   plugin kinds installed, **When** overview enriches rows, **Then** the
   cluster gains live allocation children grouped by namespace.
2. **Given** the same setup, **When** the cluster is unreachable or the
   allocation fails, **Then** the cluster row falls back to projected
   children (or no children) and the overview otherwise succeeds — expansion
   failure is never fatal.
3. **Given** live children exist, **When** totals are computed, **Then**
   live child costs are excluded from the summary and a footnote explains
   that live rows re-allocate cost already represented elsewhere.

---

### User Story 3 - Live data wins when both sources exist (Priority: P2)

An engineer's stack declares both an AKS cluster and its workloads, and the
live path is available. The cluster expands into live namespace rows; the
declared workload rows are suppressed as separate children so cost is not
double-counted, and a footnote says live data was preferred.

**Why this priority**: Double-counting cluster cost would be worse than
showing only one source; the precedence rule is an explicit acceptance
criterion of the issue.

**Independent Test**: Run expansion with both a live result and grouped
projected workloads for the same cluster; assert children come from the live
result, projected rows are suppressed, and the footnote reports how many
projected rows were suppressed.

**Acceptance Scenarios**:

1. **Given** both live and projected data for one cluster, **When**
   expansion runs, **Then** the cluster's children are the live rows and the
   previously projected workload rows are omitted from the flat list.
2. **Given** that suppression happened, **When** output renders in table or
   JSON, **Then** a footnote / metadata note states that live cluster data
   was preferred and how many declared workload rows were suppressed.

---

### User Story 4 - Machine-readable parent/child references (Priority: P1)

A CI pipeline consumes `finfocus overview --output json` and `--output
ndjson`. Cluster expansion must be visible to machines: a consumer can find
a cluster's children, and can find a child's parent, without heuristics.

**Why this priority**: JSON/NDJSON is the contract other tools build on; it
is golden-file tested like the rest of overview output.

**Independent Test**: Golden-file test the JSON and NDJSON output of a
fixture stack with an expanded cluster; assert `childUrns`, `parentUrn`, and
`expansionSource` appear exactly as specified.

**Acceptance Scenarios**:

1. **Given** an expanded cluster, **When** `--output json` runs, **Then**
   the cluster resource entry contains `childUrns` in display order and each
   child entry contains `parentUrn` and `expansionSource`.
2. **Given** the same run, **When** `--output ndjson` runs, **Then** parent
   and child lines carry the same fields, one row per line, in flat order.

---

### Edge Cases

- A stack with multiple clusters: projected workloads group under a cluster
  only when exactly one cluster exists, or when the workload's Pulumi
  provider/provider chain unambiguously references one cluster; otherwise
  workloads stay flat. Live expansion still applies per cluster when its
  context resolves. This limitation is documented.
- Live expansion succeeds for one cluster but fails for another: each
  cluster independently uses live or projected children.
- A live allocation returns only idle cost (no workloads): the cluster still
  expands, showing the idle row.
- `--state-only` runs and runs without a preview: expansion works from state
  resources alone for the projected path.
- Cluster resource is pending deletion or errored: expansion is skipped for
  that cluster.
- Filters (`--filter type=...`) apply to the flat row list before expansion
  grouping; children whose parent was filtered out render flat with their
  parent reference intact.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST detect Kubernetes cluster resources by type
  token, covering EKS (`aws:eks/cluster:Cluster`, `eks:index:Cluster`), GKE
  (`gcp:container/cluster:Cluster`, `google-native:container/v1:Cluster`),
  and AKS (`azure:containerservice/kubernetesCluster:KubernetesCluster`,
  `azure-native:containerservice:ManagedCluster`).
- **FR-002**: The system MUST detect Pulumi-declared workload resources of
  the five kinds priced by spec 621 (`kubernetes:apps/v1:Deployment`,
  `kubernetes:apps/v1:StatefulSet`, `kubernetes:apps/v1:DaemonSet`,
  `kubernetes:batch/v1:Job`, `kubernetes:batch/v1:CronJob`).
- **FR-003**: When a stack contains exactly one cluster resource, the system
  MUST group all declared workload rows in the stack as that cluster's
  projected children.
- **FR-004**: When both a live allocation and projected workload rows are
  available for the same cluster, the system MUST use the live rows as
  children, MUST suppress the projected rows from the flat list, and MUST
  emit a footnote/metadata note stating live data was preferred and how many
  rows were suppressed.
- **FR-005**: Live child rows MUST be grouped at namespace granularity by
  default, matching `cost cluster`'s default `--group-by namespace`.
- **FR-006**: Live child rows MUST be excluded from summary cost totals;
  projected child rows MUST retain the same totals contribution they had as
  flat rows.
- **FR-007**: Expansion MUST never fail the overview: any live-expansion
  error (unreachable cluster, missing plugins, unresolved context) falls
  back to the projected path or to no expansion, with at most a warning.
- **FR-008**: With no usage-source/allocator plugins and no declared
  workloads, a cluster row MUST render byte-identically to today (no
  expander, no new fields, unchanged totals).
- **FR-009**: The TUI MUST render cluster rows that have children with an
  expander marker, start them collapsed, and toggle expansion with `e`, `→`,
  and `←` keys, rendering children indented beneath the parent.
- **FR-010**: JSON and NDJSON output MUST represent expansion as a flat row
  list where children carry `parentUrn` and `expansionSource`
  (`"live"` | `"projected"`) and cluster rows with children carry
  `childUrns`; this shape MUST be golden-file tested.
- **FR-011**: The plain (non-TUI) table renderer MUST show children as
  indented rows (`↳` prefix) beneath their parent.
- **FR-012**: Cluster→context resolution MUST follow the precedence:
  explicit config mapping, then kubeconfig context name matching the
  cluster's name/ARN, then (single-cluster stacks only) the current context
  footnoted as assumed.

### Key Entities

- **Cluster row**: an existing overview row whose resource type is a
  detected cluster type; gains an optional ordered list of child references.
- **Workload child row (projected)**: an existing overview row for a
  declared workload, re-parented to a cluster row; identity, cost, and
  totals contribution unchanged.
- **Allocation child row (live)**: a synthetic overview row representing one
  namespace's allocated cost from a live cluster; carries the allocated
  cost, its parent reference, and the `live` expansion source; excluded from
  totals.
- **Cluster context mapping**: user configuration associating a cluster name
  or URN with a kubeconfig context.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user running overview on a stack with a cluster and declared
  workloads sees every declared workload nested under its cluster in all
  three output modes (TUI, table, JSON/NDJSON).
- **SC-002**: A stack with a cluster but no workloads and no allocation
  plugins produces overview output identical to the previous release (zero
  changed lines in golden output).
- **SC-003**: When live and projected data are both present, workload cost
  appears exactly once in the rendered expansion (no double counting), with
  a visible note saying live data was preferred.
- **SC-004**: Expansion adds no more than one extra plugin round trip per
  cluster beyond today's enrichment flow, and overview completes
  successfully even when every live expansion attempt fails.
- **SC-005**: 100% of the new detection and merge logic is covered by unit
  tests; the JSON/NDJSON expansion shape is pinned by golden tests; the TUI
  expanded view is verified by rendering and reading the full output.
