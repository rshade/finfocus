# Data Model: Kubernetes In-Cluster Cost Allocation

**Date**: 2026-09-24 (converted 2026-09-29)
**Source**: design doc §3–§4 with §8 amendments folded in. Wire types are owned
by finfocus-spec v0.6.2 (`proto/finfocus/v1/usage.proto`,
`proto/finfocus/v1/allocation.proto`); this document records the entities and
their semantics as consumed by finfocus.

## Wire Entities (finfocus-spec v0.6.2)

### UsageRow

One usage measurement for a subject.

| Field | Type | Notes |
| --- | --- | --- |
| `subject` | `map<string, string>` | Well-known keys below; `label.<key>` for labels |
| `metric` | `string` | `cpu_request`, `mem_request` (workload); `cpu_allocatable`, `mem_allocatable` (node); later `cpu_usage`, `mem_usage` |
| `amount` | `double` | Per-hour rate in `RUN_RATE`; integrated resource-hours in `HISTORICAL` |
| `unit` | `string` | `core`, `GiB`, `core-hours`, `GiB-hours` |

Rules (SDK-validated): every row carries `kind`; `kind=node` rows name their
`node`; duplicate `(subject, metric)` rows are invalid.

### Subject keys and kinds

- Keys: `cluster`, `namespace`, `controller_kind`, `controller`, `pod`,
  `node`, `label.<key>`, `kind`.
- `kind` values: `workload`, `node`, `__idle__`, `__cluster__`.
- Keys are strings rather than typed fields so label grouping and future
  non-Kubernetes usage sources need no spec change; the conformance check
  (`plugintesting.ValidateStatsResponse`) enforces the documented set.

### PricedResource

A priceable resource (node or control plane) with its cost for the normalized
period.

| Field | Type | Notes |
| --- | --- | --- |
| `resource` | `ResourceDescriptor` | Node: `provider=aws`, `resource_type=aws:ec2/instance:Instance`, `sku`/`region` from node labels, `id` = node name, `tags.kind=node`, `tags.provider_id`, `tags.capacity_type=spot\|on-demand`. Control plane: `tags.kind=cluster` (e.g. `resource_type=aws:eks/cluster:Cluster`, `sku=cluster`) |
| `cost` | `double` | Cost for the normalized period (monthly for run-rate) |
| `currency` | `string` | Empty takes the others' single currency; all empty → `USD` |
| `priced` | `bool` | `false` = pricing failed; `cost` MUST be 0. Core sets this when monthly `<= 0` or a structured error is present |
| `note` | `string` | e.g. `"spot node priced on-demand"` |

`resource.id` MUST equal the `node` subject value of that node's usage rows.
The reverse is not required: a node that cannot be priced (unknown provider,
Fargate) reports capacity without a priceable entry.

### AllocationRow

One allocated cost.

| Field | Type | Notes |
| --- | --- | --- |
| `subject` | `map<string, string>` | `kind` ∈ {`workload`, `__idle__`, `__cluster__`}; idle rows carry `node`; cluster rows carry `cluster` |
| `cpu_cost` | `double` | CPU portion |
| `mem_cost` | `double` | Memory portion |
| `total_cost` | `double` | `cpu_cost + mem_cost`; waived only for `kind=__cluster__` (may put everything in `total_cost`) |
| `currency` | `string` | Resolved currency, same on every row |
| `note` | `string` | e.g. `"Fargate pricing not supported yet"` |

Invariants: no negative costs; idle is a row, never silently dropped;
Σ `total_cost` = Σ `cost` over `priced=true` entries within relative epsilon
1e-6 (`pluginsdk.CheckConservation`).

### StatsMode

`STATS_MODE_RUN_RATE` (point-in-time per-hour rates) and
`STATS_MODE_HISTORICAL` (integrated over `[start, end]`). The kubernetes
plugin serves run-rate only; historical requests get `InvalidArgument`.

## Allocation Policy (plugin-owned, version 1)

User overrides live in `allocation.hujson`; core standardizes HuJSON to JSON
and passes it as opaque `policy_json` bytes. Decoding is strict
(`pluginsdk.DecodePolicy` onto defaults): unknown fields are rejected with
their JSON path; unknown `version` is rejected; enum fields accept only their
single supported value; policy v1 has no list fields (the documented "lists
replace, not append" rule applies to future versions).

| Field | Default | Meaning |
| --- | --- | --- |
| `version` | `1` | Schema version; only `1` is supported |
| `idle` | `"separate"` | Keep waste visible as `__idle__` rows (`"share"` is future) |
| `system_workloads` | `"separate"` | kube-system and DaemonSets appear as their own rows |
| `node_split.method` | `"unit-price-ratio"` | Divide node price into CPU/memory by public unit-price weights |
| `node_split.cpu_core_hour` | `0.031611` | Weight per core-hour |
| `node_split.mem_gib_hour` | `0.004237` | Weight per GiB-hour; weights MUST be `>= 0` |
| `charge` | `"max-request-usage"` | `max(request, usage)`; slice 1 has requests only |
| `control_plane` | `"separate"` | Control plane emitted as `__cluster__` row |
| `spot_nodes` | `"on-demand-with-note"` | Spot priced on-demand with a note |

The effective policy (defaults + overrides) is returned as
`effective_policy_json` with a stable hex SHA-256 `policy_digest`; empty
`usage` + empty `priced` returns only the policy and digest (backs
`--show-policy`). `{}`, comments-only, and missing-`version` documents decode
to defaults and digest identically to no policy.

## Registry `tag_prefix`

`registry.json` entries gain an optional `tag_prefix` (e.g. `"kubernetes-"`)
marking a plugin released from the finfocus monorepo under prefixed tags.

| Concept | Rule |
| --- | --- |
| Tag format | `<prefix><semver-with-v>`, e.g. `kubernetes-v0.1.0` |
| Prefix pattern | `^[a-uw-z0-9][a-z0-9-]*-$` (never `v`-leading) |
| Canonical version | Tag minus prefix (`CanonicalVersion`), e.g. `v0.1.0` — used for install dir, asset-name version, recorded version, and update comparison |
| `ReleaseTag` | `0.1.0`, `v0.1.0`, `kubernetes-v0.1.0` all resolve to `kubernetes-v0.1.0` |
| Latest | Highest semver among stable prefixed releases, scanning up to 100 (`prefixedReleaseScan`); prereleases skipped; never `/releases/latest` |
| Asset names | `finfocus-plugin-<name>_<canonical>_<os>_<arch>.tar.gz` (`.zip` on Windows) + `checksums.txt` |
| Unprefixed entries | Unchanged in every respect |
