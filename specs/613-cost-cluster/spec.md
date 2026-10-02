# Feature Specification: `finfocus cost cluster`

**Feature Branch**: `613-cost-cluster`
**Created**: 2026-09-24
**Status**: Implemented (issue #1528, branch `issue-1528`; converted from the
superpowers SP3 plan and re-verified against `main` on 2026-09-29)
**Input**: Superpowers plan "SP3 — `finfocus cost cluster` Implementation Plan"
(2026-09-24, committed in PR #1522 under the since-removed superpowers docs directory), seeded by the design spec now living at
`specs/612-k8s-cost-allocation/spec.md`. Implements SP3 of the Kubernetes
cost-allocation decomposition; tracked as issue #1528.

## Overview

Add `finfocus cost cluster`, which gathers usage from a usage-source plugin,
prices the reported nodes through the existing projected-cost engine, has an
allocator plugin divide the cost, verifies conservation, and renders grouped
rows as table, JSON, or NDJSON. Core stays Kubernetes-agnostic: nodes arrive
as ordinary `ResourceDescriptor`s, allocation math lives in the allocator
plugin (`plugins/kubernetes`, delivered in `specs/612-k8s-cost-allocation/`).

### Non-goals (this feature)

- Historical actuals (`STATS_MODE_HISTORICAL` arrives with SP4 Prometheus;
  requests with a start time get a clear "not supported yet" error).
- Config routing for the new capabilities (router features are a fixed list in
  `internal/router/features.go`; plugin selection is explicit flag or the
  single installed capable plugin).
- Idle/shared-cost redistribution policies; spot and Fargate pricing (handled
  as notes / `NO_COST_DATA` by the plugin).
- `kubernetes:*` projected costs and `overview` cluster expansion (later
  roadmap items).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Cluster Cost Breakdown (Priority: P1)

As a FinOps engineer, I want to run `finfocus cost cluster` against my
kubeconfig context and see the cluster's monthly run-rate cost broken down by
namespace (or controller, pod, node, or label), including idle capacity, so
that I know which workloads drive the bill.

**Why this priority**: This is the entire user-facing value of the feature —
one command from a live cluster to a trustworthy per-workload breakdown.

**Independent Test**: With the `kubernetes` and `aws-public` plugins installed
and a kind cluster with labeled nodes, `finfocus cost cluster --output json`
reports groups whose costs sum to the priced total within 1e-6, with a
positive `idle` amount.

**Acceptance Scenarios**:

1. **Given** installed usage-source and allocator plugins, **When** the user
   runs `finfocus cost cluster`, **Then** usage is collected for the current
   kubeconfig context, nodes are priced at projected monthly cost, the
   allocator divides the cost, conservation is verified, and a table grouped
   by namespace is rendered with a footer (mode, total, idle, policy
   source + digest, completeness).
2. **Given** `--group-by label:team`, **When** pods lack the label, **Then**
   their cost groups under `<none>` and is never dropped.
3. **Given** `--namespace e2e`, **When** the command runs, **Then** workload
   rows are exact for that namespace and `__idle__`/`__cluster__` rows are
   omitted with a footer notice.
4. **Given** no plugin with the required capability, **When** the command
   runs, **Then** it exits 1 naming the capability with an install hint
   (`finfocus plugin install kubernetes`); ambiguous candidates are listed.

---

### User Story 2 - Inspect and Override the Allocation Policy (Priority: P2)

As a platform engineer, I want to see the effective allocation policy and
override it per project, so that allocation matches my organization's
conventions and I can audit exactly which policy produced a report.

**Why this priority**: Policy transparency (source + digest in every footer)
is what makes the numbers reviewable; overrides make the feature adoptable.

**Independent Test**: `finfocus cost cluster --show-policy` with only an
allocator plugin installed prints the effective policy JSON and digest without
contacting any cluster; `--policy <file>` with an unknown field exits 1 naming
the JSON path.

**Acceptance Scenarios**:

1. **Given** no flags and no files, **When** the command runs, **Then** the
   plugin's built-in defaults apply and the footer shows
   `built-in defaults · <digest>`.
2. **Given** `$PROJECT/.finfocus/allocation.hujson` and a global
   `allocation.hujson`, **When** the command runs in the project, **Then** the
   project file wins (files never merge with each other).
3. **Given** a discovered policy file that fails to parse, **When** the
   command runs, **Then** it exits 1 with the file path — it never falls back
   to defaults.
4. **Given** `--policy` pointing at a missing file, **When** the command runs,
   **Then** it exits 1 (`VALIDATION_ERROR`).

---

### User Story 3 - Automate via JSON/NDJSON and MCP (Priority: P2)

As a platform engineer, I want stable machine-readable output and an MCP tool,
so that agents and CI can consume cluster allocation without parsing tables.

**Why this priority**: Machine consumers need a stable shape; MCP exposure
makes the command available to agent workflows like every other read-only
command.

**Independent Test**: `--output json` parses to the documented shape (summary,
groups, priced, and policy); `--output ndjson` emits a `summary` line followed
by one `group` line per group; the MCP `tools.golden` lists `cost cluster`.

**Acceptance Scenarios**:

1. **Given** `--output json`, **When** allocation succeeds, **Then** stdout is
   a single document with `mode`, `period`, `currency`, `group_by`, `total`,
   `idle`, `namespace_scoped`, `incomplete`, `groups`, `priced`, `policy`, and
   `warnings`; warnings also go to stderr for table output.
2. **Given** an invalid `--output` or `--group-by`, **When** the command runs,
   **Then** validation fails BEFORE any plugin is loaded (exit 1).
3. **Given** the MCP server, **When** `tools/list` runs, **Then** `cost
   cluster` appears (read-only, bounded) and matches
   `internal/cli/testdata/mcp/tools.golden`.

---

### Edge Cases

- **Node priced `$0`** (aws-public answers unknown instance types with `$0`,
  not an error): treated as unpriced — rows at `$0` with a note, total marked
  incomplete; all nodes unpriced is fatal.
- **Conservation violation** from a misbehaving allocator: fatal
  ("allocator violated cost conservation", `PLUGIN_ERROR`); core delegates the
  check to `pluginsdk.ValidateAllocateResponse` + `pluginsdk.CheckConservation`.
- **Mixed currencies**: existing `ErrMixedCurrencies`.
- **Slow `GetStats`**: plugin pages pod listing; the existing plugin timeout
  applies (`TIMEOUT_ERROR`).
- **No kubeconfig / unknown context**: fatal, naming context and kubeconfig
  path (`VALIDATION_ERROR`).
- **RBAC forbids listing pods in some namespaces**: fatal unless `--namespace`
  restricts to readable ones; the message lists missing verbs/resources and
  links the minimal ClusterRole (`plugins/kubernetes/deploy/clusterrole.yaml`).
- **Cluster with no pods**: valid; all cost is idle.
- **Historical request** (start time set): `ErrHistoricalUnsupported` —
  run-rate only in this feature.
- **Priced results joined to descriptors**: by `(type, id)`, never by slice
  index.
- **Installed kubernetes plugin must not pollute other commands**: its
  `Supports=false` keeps `cost projected` free of per-resource "not supported"
  errors (integration test).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The command MUST be
  `finfocus cost cluster [--context <ctx>] [--namespace <ns>]
  [--selector k=v]... [--group-by namespace|controller|pod|node|label:<key>]
  [--policy <file>] [--show-policy] [--usage-source <plugin>]
  [--allocator <plugin>] [--output table|json|ndjson]`, with default
  `--group-by namespace`.
- **FR-002**: Plugin selection MUST be explicit flag > the single installed
  plugin declaring the capability (`usage_stats` / `allocation`); zero
  candidates or ambiguous candidates are fatal errors listing candidates and
  the flag/config key.
- **FR-003**: The pipeline MUST be: resolve plugins by capability → `GetStats`
  with scope/selector → price `priceable` through
  `Engine.GetProjectedCostWithErrors` → `Allocate` with the resolved policy →
  verify conservation → group → render. Run-rate costs are monthly
  (`hoursPerMonth = 730`) because nodes are priced at projected monthly cost;
  allocation uses ratios only (no usage scaling).
- **FR-004**: `internal/pluginhost` MUST map capability enums 14/15 to the
  names `usage_stats` and `allocation` (`ConvertCapabilities`,
  `HasCapability`).
- **FR-005**: A priceable with monthly cost `<= 0` or a structured error MUST
  be treated as unpriced (`priced=false`), never as free.
- **FR-006**: Conservation MUST be enforced in core via
  `pluginsdk.ValidateAllocateResponse` + `pluginsdk.CheckConservation` at
  `pluginsdk.DefaultConservationEpsilon` (1e-6 relative), even though the
  conformance suite also checks it — third-party allocators exist.
- **FR-007**: Policy resolution MUST be `--policy` >
  `$PROJECT/.finfocus/allocation.hujson` > `<ResolveConfigDir()>/allocation.hujson`
  > none; first found wins; HuJSON is standardized to JSON via
  `ax.ParseConfig`; a discovered file that fails to parse is fatal; the policy
  bytes are opaque to core.
- **FR-008**: Grouping MUST be generic string-map aggregation: `namespace` →
  `namespace`; `controller` → `namespace/controller_kind/controller`; `pod` →
  `namespace/pod`; `node` → `node`; `label:<k>` → the `label.<k>` value;
  missing values group under `<none>`; `__idle__` rows group under the node
  name for `node` grouping, else `__idle__`; `__cluster__` rows group under
  `__cluster__` for every dimension; groups sort by total descending, then key
  ascending; notes de-duplicated and sorted.
- **FR-009**: `--namespace` MUST omit `__idle__` and `__cluster__` rows and say
  so in the footer (`namespace_scoped: true`, `idle` omitted in JSON).
- **FR-010**: `--output` and `--group-by` MUST be validated before plugins or
  state are loaded; output format resolution MUST use `resolveOutputFormat`
  (PR #1510); errors exit 1.
- **FR-011**: Table output MUST show columns `GROUP CPU MEMORY TOTAL NOTES`
  with a footer (mode/period, total, idle %, policy source + digest,
  incomplete resources); JSON/NDJSON MUST match the documented shapes; the
  command MUST be exposed as an MCP tool with `tools.golden` updated.
- **FR-012**: `--show-policy` MUST work with only an allocator installed and
  MUST NOT contact the cluster (empty-usage `Allocate` returns the effective
  policy and digest).
- **FR-013**: Every test that touches config MUST isolate `FINFOCUS_HOME` to a
  temp dir.

### Key Entities *(include if feature involves data)*

- **ClusterRequest**: scope (kube context), namespace, selector, policy JSON.
- **ClusterResult / ClusterRow**: allocator rows plus mode, currency, priced
  summaries, total, idle, namespace-scoped/incomplete flags, policy digest and
  effective policy, warnings (see `data-model.md`).
- **ClusterGroup**: aggregated key, cpu/mem/total costs, row count, notes.
- **clusterOutput**: the stable JSON contract (see `contracts/`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `finfocus cost cluster` against a kind cluster reports groups
  whose sum equals the priced node total within 1e-6, with idle as the
  remainder.
- **SC-002**: A Deployment's reported cost equals
  `request / allocatable × node monthly price` within epsilon, computed from
  the node price the command itself reports (no hardcoded price table).
- **SC-003**: `--namespace` output contains no `__idle__` group and sets
  `namespace_scoped: true`; a bad policy file exits 1 naming the JSON path.
- **SC-004**: `--show-policy` succeeds with no cluster reachable and no usage
  plugin installed.
- **SC-005**: The kind E2E (`make test-e2e-kind`) runs the real `finfocus`
  binary with the real kubernetes and aws-public plugins — no stubbed costs —
  and passes in CI via `helm/kind-action` without AWS credentials.
- **SC-006**: MCP `tools.golden` and the live `tools/list` integration test
  include `cost cluster`; pipeline unit tests cover plugin resolution
  (0/1/many), partial pricing, conservation violation, and mixed currencies.
