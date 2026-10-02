# Feature Specification: Allocator policy v2 (idle and system share)

**Feature Branch**: `614-allocator-policy-v2`
**Created**: 2026-10-02
**Status**: Implemented (issue #1533)
**Input**: GitHub issue #1533. Supersedes the "shared-cost redistribution is
out of scope" non-goal in `specs/612-k8s-cost-allocation/` for the two policy
fields below. Schema version stays `1`.

## Overview

The kubernetes allocator keeps today's behavior unless an allocation policy
opts in. `idle` and `system_workloads` each accept `"separate"` (the default)
or `"share"`. Sharing stays on the node that produced the cost. Cluster rows
(`__cluster__`) are unchanged.

The redistribution weight is the cost the row already holds in that
dimension. CPU idle and CPU system cost move in proportion to CPU cost.
Memory idle and memory system cost move in proportion to memory cost. There
is no blended total, and there is no even split of idle onto zero-cost
workloads.

### Non-goals

- Cross-node redistribution.
- Team weights, cost-weighted blends, or any weight other than the row's
  existing CPU cost or memory cost in that dimension.
- Changing `charge`, `node_split`, `control_plane`, or `spot_nodes`.
- Dropping the idle row. `pluginsdk.ValidateAllocateResponse` (finfocus-spec
  v0.7.0) requires exactly one idle row per priced node (`priced=true` and
  tag `kind=node`). Sharing moves the dollars and leaves a `$0` idle row.

## Formula

Base rows are computed first, exactly as `"separate"` does today: the node's
price is split into a CPU portion and a memory portion by
`node_split.method=unit-price-ratio`, then each portion is split across the
node's workloads by `charge=max-request-usage`. The idle remainder of each
portion is `max(portion - sum(workload costs in that portion), 0)`.

`distribute(amount, weights, evenIfZero)` then splits one amount:

1. `amount <= 0` or no weights yields all zeros. Nothing moves.
2. Otherwise only strictly positive weights count. Their sum is `S`.
3. When `S` is 0 and `evenIfZero` is false, nothing moves.
4. When `S` is 0 and `evenIfZero` is true, every recipient gets
   `amount / n`, and the last recipient absorbs `amount - sum(shares)` so
   the shares add up to `amount`.
5. When `S > 0`, recipient `i` with weight `w` gets `amount * w / S`.
   Recipients with `w <= 0` get 0. The last positive-weight recipient
   absorbs `amount - sum(shares)`.

### Idle share

When `idle` is `"share"`, leftover idle CPU is distributed across every
current workload row with `evenIfZero=false` and weights equal to those
rows' CPU costs. Leftover idle memory uses the memory costs. A dimension
whose workload costs are all zero stays on the idle row (an empty node, or
a node whose workloads have no cost in that dimension). The idle row is
still emitted. Its costs are whatever `distribute` did not move.

### System share

A workload is system when its subject `namespace` is `kube-system` or its
subject `controller_kind` is `DaemonSet`.

When `system_workloads` is `"share"`:

- No system rows, or no non-system rows: the rows stay as they are. A node
  that is entirely system keeps its system rows.
- Otherwise each dimension's system cost is distributed onto the non-system
  rows with `evenIfZero=true`, weighted by those rows' cost in that
  dimension. The even split is only the fallback for a dimension whose
  non-system costs are all zero, so the system cost still has a home.
  System rows are then dropped. The idle row is not a recipient.

### Both fields set to share

Idle is shared first, across every workload row including system rows. System
rows, which now include the idle they absorbed, are then folded into the
non-system rows. The two fields are otherwise independent: idle can be
`"share"` while system workloads stay `"separate"`, and the reverse.

### Conservation

After sharing, `pluginsdk.CheckConservation` with
`DefaultConservationEpsilon` still holds. Default `"separate"` (no policy
document, or a document that omits these fields) does not change existing
allocation bytes.

## User scenarios

### User Story 1 - Keep waste visible (Priority: P1)

As a platform engineer who does not set a policy, I want idle and
kube-system cost to stay on their own rows.

**Acceptance**: an empty `PolicyJson`, or a document that sets both fields
to `"separate"`, matches the pre-v2 golden allocation. `policy.Defaults()`
still sets both fields to `"separate"`.

### User Story 2 - Fold idle into the workloads on that node (Priority: P1)

As a team doing chargeback, I want unused node capacity added to the
workloads already on that node, in proportion to what they already cost.

**Acceptance**:

1. One workload on a node absorbs that node's whole price. The idle row
   total is 0.
2. Two equal workloads each receive half. The idle row total is 0.
3. A fully packed node matches `"separate"` (there is no idle to move).
4. A node with no workloads keeps a single idle row of the full node price.
5. Two nodes do not exchange idle. A second node's idle stays on that node.

### User Story 3 - Fold system workloads into the other workloads (Priority: P1)

As a team doing chargeback, I want kube-system and DaemonSet cost included
in the application workloads on the same node.

**Acceptance**:

1. An application pod plus a kube-system pod plus an application DaemonSet,
   with idle left `"separate"`, leaves only the application pod among
   workload rows. Its total is three quarters of the node when the three
   pods were equal and the idle quarter stays on the idle row. No row keeps
   namespace `kube-system` or controller kind `DaemonSet`.
2. A node whose only workload is kube-system keeps that row.
3. Both fields `"share"` makes the application row equal the node price and
   the idle row 0.
4. Idle `"share"` with system `"separate"` still shows the kube-system row,
   and the idle row is 0 when the workloads absorbed the idle.

### User Story 4 - See the effective value (Priority: P2)

As an operator, I want `finfocus cost cluster --show-policy` to show
`"idle":"share"` when that is the effective policy.

**Acceptance**: `Policy.Canonical()` JSON contains the effective strings.
The digest changes when either field changes. Unsupported strings such as
`"spread"` fail validation with
`idle: unsupported value "spread" (supported: "separate", "share")`
(and the same shape for `system_workloads`). The CLI does not keep a second
list of allowed values; it prints the plugin's effective JSON.

## Edge cases

- A non-finite or negative usage amount is already treated as 0 before
  sharing. Sharing does not reintroduce it.
- Float remainder is assigned to the last positive recipient (or the last
  recipient of an even split) so the moved amount sums exactly.
- Unpriced nodes and orphan rows are unchanged. Sharing only runs inside
  `allocateNode` on the rows that node already produced.
