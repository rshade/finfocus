# Feature Specification: Cluster Spend by Capacity Type

**Feature Branch**: `feat/1840-cluster-capacity-split`
**Created**: 2026-11-02
**Status**: Draft
**Input**: GitHub issue #1840 (part of epic #367): "Show spot share of
Kubernetes cluster spend: split `cost cluster` output by `capacity_type`
(spot vs on-demand)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See how much of a cluster runs on each capacity type (Priority: P1)

An operator runs `finfocus cost cluster` and wants to know what share of the
cluster bill sits on spot nodes, on-demand nodes, and nodes whose capacity
type is unknown.

**Why this priority**: The kubernetes and prometheus plugins already tag every
node with `capacity_type`, but nothing aggregates it.

**Independent Test**: Allocate a cluster of spot, on-demand, and untagged nodes
with a fake usage source and allocator. The capacity groups sum to the cluster
total, each node's workload and idle cost sits in its own group, and the
untagged node is in `unlabeled`.

**Acceptance Scenarios**:

1. **Given** nodes tagged `capacity_type=spot` and `capacity_type=on-demand`,
   **When** `cost cluster` runs, **Then** the table prints a `CAPACITY TYPE`
   section with one line per value: node count, total, share of the cluster
   total, idle cost, and notes.
2. **Given** a node without the tag, **When** the report is built, **Then** its
   cost is in an `unlabeled` bucket and is not added to any named bucket.
3. **Given** any run, **When** the capacity totals are summed, **Then** they
   equal the cluster total, and the idle amounts sum to the reported idle cost.
4. **Given** no node carries the tag, **When** the table prints, **Then** the
   section is omitted (the JSON still lists one `unlabeled` group).

---

### User Story 2 - Run-rate output does not imply spot pricing (Priority: P1)

In run-rate mode a pricing plugin has no spot rate, so the allocator prices a
spot node at its on-demand rate and notes it. The split must carry that note.

**Independent Test**: Allocator rows for a spot node carry the note
`spot node priced on-demand`; the spot group lists it in `notes`. A group
whose rows carry no note has none. A historical window carries no such note.

**Acceptance Scenarios**:

1. **Given** a run-rate run with spot nodes, **When** the table prints,
   **Then** the spot line shows the allocator's note.
2. **Given** a historical window, **When** the report is built, **Then** the
   group shows no on-demand note, because the spend is actual.

---

### User Story 3 - Agents read the split in JSON (Priority: P1)

`--output json` carries a `capacity` array; NDJSON carries it on the `summary`
line. MCP callers get it because `cost cluster` is already an MCP tool.

**Acceptance Scenarios**:

1. **Given** `--output json`, **Then** `capacity` is an array of objects
   (never `null`) with the fields in `contracts/cli.md`.

### Edge Cases

- `--namespace` scoped runs omit idle and cluster rows. The split then covers
  workload rows only and its `idle_cost` is 0.
- The control-plane row (`__cluster__`) names no node and lands in `unlabeled`.
- A tag with an empty value is unlabeled.
- A group named `unlabeled` by a plugin's own tag value would merge with the
  bucket; capacity values are opaque and not escaped.
- An unpriced node has $0 rows and notes; it still counts as a node in its
  bucket.
- `--group-by` does not change the capacity split.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `engine.GroupClusterByCapacity` MUST group `ClusterRow` values by
  the `capacity_type` tag of the row's node, found by joining the row's
  `cluster` and `node` subjects to a node descriptor (`kind=node`) by cluster
  and Kubernetes name (workload rows) or cluster and priced id (idle rows).
  An exact match wins, even with an empty tag. A row that names a cluster
  falls back to a descriptor with no cluster tag, as the allocator does; a
  descriptor tagged for another cluster never matches. It MUST keep no table of capacity
  values and MUST NOT interpret Kubernetes.
- **FR-002**: A row whose node has no tag, an empty tag, or no node subject MUST
  go to the `unlabeled` group.
- **FR-003**: Every row MUST land in exactly one group. Idle rows count under
  their node's group, and `idle_cost` reports them separately, so nothing is
  counted twice.
- **FR-004**: Group totals MUST sum to `ClusterResult.Total`, which
  `VerifyConservation` has already tied to the priced total.
- **FR-005**: Each group MUST carry the distinct row notes, sorted. Core adds
  no note of its own.
- **FR-006**: The JSON output MUST carry `capacity`; the table MUST print the
  section only when at least one group is not `unlabeled`.
- **FR-007**: `plugins/kubernetes` and `plugins/prometheus` MUST NOT change,
  and core MUST NOT import them.

### Key Entities

- **CapacityGroup**: `capacity_type`, `cpu_cost`, `mem_cost`, `total_cost`,
  `idle_cost`, `nodes`, `rows`, `notes`.

## Web UI Fit *(mandatory)*

- **Disposition**: Not applicable. `finfocus --web` has no cluster-allocation
  surface: no handler in `internal/webui` calls `engine.RunClusterAllocation`,
  and the dashboard's "cluster" is Pulumi resource expansion in Overview, not
  `cost cluster`. This command has no web page to extend.
- **Web data**: None.
- **Layout fit**: None.
- **Additions**: None.
- **Render evidence**: Not applicable. There is no page to render.
- **Follow-up issue scope**: No `web-ui` issue. A cluster allocation page is a
  separate feature; this change adds no web gap beyond the one that already
  exists for `cost cluster`.

## Assumptions

- The usage source tags priceable nodes with `capacity_type` and the allocator
  puts a node's note on every row of that node (both hold today).
- The control plane is not a capacity type and is reported as `unlabeled`.
- With the current kubernetes and prometheus plugins, both default
  `capacity_type` to `on-demand` for a node with no spot label. The
  `unlabeled` bucket therefore holds no-node rows (control-plane fee, Fargate
  pods, orphan rows), not untagged nodes. It holds untagged nodes only for a
  usage source that omits the tag.
- Nodes are matched by cluster and node name, because node names repeat
  across clusters. Same-named nodes in different clusters use a composite
  priced id (`cluster`, NUL, `node`) with `cluster` and `node` tags, which is
  the only shape the SDK accepts. The in-tree plugins do not emit it yet.
- Nodes are counted once per priced node, by its canonical cluster and id.

## Out of Scope

- Pricing spot nodes at spot rates; that needs a pricing plugin.
- Changing the `--group-by` dimensions.
- A cluster dashboard page.
- Plugin changes; the tag and note already exist.

## Success Criteria *(mandatory)*

- **SC-001**: With mixed spot, on-demand, and untagged nodes, capacity totals
  equal the cluster total within 1e-9.
- **SC-002**: Idle cost across groups equals the reported idle cost.
- **SC-003**: `make lint` and `make test` pass.
