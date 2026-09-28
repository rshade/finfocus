# Kubernetes In-Cluster Cost Allocation — Design

- **Date**: 2026-09-24
- **Status**: Draft, pending review
- **Repos affected**: finfocus (core + in-repo plugin), finfocus-spec (via issues),
  finfocus-plugin-aws-public (future issues only)

## 1. Goal

Break Kubernetes cluster costs down by namespace, controller (Deployment,
StatefulSet, DaemonSet, Job), pod, node, and label — first as a current
**run-rate** from a live cluster, then as **historical actuals** once a metrics
backend (Prometheus, Datadog) is supported, and finally as **projected** costs
for Pulumi-managed `kubernetes:*` resources.

### Decisions already made

| Topic | Decision |
| --- | --- |
| Priority | Both actual and projected; actual-side pipeline first |
| Plugin model | New plugin types: a **usage source** (`GetStats`) and an **allocator** (`Allocate`) |
| Allocation math | Lives in an **allocator plugin**, not core. Core stays Kubernetes-agnostic |
| First usage source | Kubernetes-API-only, run-rate (no metrics backend required) |
| Later usage sources | Prometheus (SP4, tested in kind), Datadog (SP5), OpenCost (SP6, pre-allocated) |
| CLI | New `finfocus cost cluster` first; Pulumi-stack integration (`overview` expansion) later |
| Plugin location | `plugins/kubernetes/` as a nested Go module, shipped via the registry from the monorepo; graduates to its own repo once it has usage and outside contributors |
| Policy | Plugin owns defaults and schema; user overrides in `allocation.hujson`, delivered opaquely by core |
| finfocus-spec changes | Delivered as **issues on finfocus-spec, one per RPC** — not edited directly from this effort |

### Non-goals (this slice)

- Historical actuals (arrives with SP4 Prometheus).
- Spot pricing (aws-public has none; spot nodes priced on-demand with a note).
- Fargate pricing (reported as `NO_COST_DATA`).
- Shared-cost redistribution (idle, kube-system) beyond "separate rows".
- A snapshot collector that builds history from the Kubernetes API (rejected:
  sampling gaps, stateful plugin, duplicates kube-state-metrics + Prometheus).
- Adopting ax-go reserved exit codes (project-wide decision, out of scope).

## 2. Decomposition

| # | Sub-project | Repo | Depends on |
| --- | --- | --- | --- |
| SP1 | `UsageSourceService.GetStats` + `AllocatorService.Allocate` contracts, capabilities, pluginsdk helpers, conformance suites | finfocus-spec (issues) | — |
| SP2 | `plugins/kubernetes/`: run-rate usage source + allocator | finfocus | SP1 |
| SP3 | `cost cluster` command, orchestration pipeline, group-by, rendering, MCP | finfocus | SP1 |
| SP3b | Registry + release-please support for monorepo plugins | finfocus | — |
| SP4 | Prometheus usage source (historical), kind-based CI | finfocus `plugins/prometheus/` (same model as SP2; confirmed in its own spec) | SP1, SP3 |
| SP5 | Datadog usage source | finfocus `plugins/datadog/` (same model as SP2; confirmed in its own spec) | SP1, SP3 |
| SP6 | OpenCost plugin returning pre-allocated rows | existing roadmap | SP1, SP3 |
| Later | `kubernetes:*` projected costs + `overview` cluster expansion | finfocus | SP2, SP3 |
| Later | Spot pricing | aws-public | — |

**Slice 1 = SP1 + SP2 + SP3 + SP3b.** It exercises every new interface with no
external dependency; Prometheus then becomes a pure plugin addition.

### finfocus-spec delivery

The contract work in §3 is filed as issues on finfocus-spec, one per RPC:

1. **`UsageSourceService.GetStats`** — service, request/response/`UsageRow`
   messages, `StatsMode`, `PLUGIN_CAPABILITY_USAGE_STATS = 14`, well-known subject
   key constants, pluginsdk server registration helper, subject-key conformance
   check.
2. **`AllocatorService.Allocate`** — service, request/response/`PricedResource`/
   `AllocationRow` messages, `PLUGIN_CAPABILITY_ALLOCATION = 15`, pluginsdk server
   registration helper, allocator conformance suite (conservation invariant).

Core and plugin work consume the released finfocus-spec version. During
development, a local `replace` directive may point at a finfocus-spec branch;
it must not be merged.

## 3. Contracts (SP1)

Both services are served by ordinary finfocus plugin binaries with the existing
lifecycle. Core detects them through `capabilities_enum` in `GetPluginInfo`. A
plugin may implement any combination of `CostSourceService`,
`UsageSourceService`, and `AllocatorService`.

```proto
enum PluginCapability {
  // ... existing 0-13 ...
  PLUGIN_CAPABILITY_USAGE_STATS = 14;
  PLUGIN_CAPABILITY_ALLOCATION  = 15;
}

service UsageSourceService {
  rpc GetStats(GetStatsRequest) returns (GetStatsResponse);
}

service AllocatorService {
  rpc Allocate(AllocateRequest) returns (AllocateResponse);
}

enum StatsMode {
  STATS_MODE_UNSPECIFIED = 0;
  STATS_MODE_RUN_RATE    = 1;  // point-in-time rates
  STATS_MODE_HISTORICAL  = 2;  // integrated over [start, end]
}

message GetStatsRequest {
  string scope = 1;                     // e.g. kubeconfig context / cluster id
  google.protobuf.Timestamp start = 2;  // unset in run-rate mode
  google.protobuf.Timestamp end = 3;
  map<string, string> selector = 4;     // namespace, label selectors
  repeated string metrics = 5;          // "cpu_request", "mem_request", ...
}

message GetStatsResponse {
  repeated UsageRow rows = 1;
  repeated ResourceDescriptor priceable = 2;  // existing message: nodes, cluster
  StatsMode mode = 3;
  repeated string warnings = 4;
}

message UsageRow {
  map<string, string> subject = 1;
  string metric = 2;
  double amount = 3;  // per-hour rate (RUN_RATE) or integrated resource-hours
  string unit = 4;    // "core", "GiB", "core-hours", "GiB-hours"
}

message PricedResource {
  ResourceDescriptor resource = 1;
  double cost = 2;          // for the normalized period
  string currency = 3;
  bool priced = 4;          // false = pricing failed
  string note = 5;
}

message AllocateRequest {
  repeated UsageRow usage = 1;
  repeated PricedResource priced = 2;
  bytes policy_json = 3;    // standardized JSON; empty = plugin defaults
  StatsMode mode = 4;
}

message AllocateResponse {
  repeated AllocationRow rows = 1;
  bytes effective_policy_json = 2;
  string policy_digest = 3;  // sha256 of canonical effective policy
  repeated string warnings = 4;
}

message AllocationRow {
  map<string, string> subject = 1;
  double cpu_cost = 2;
  double mem_cost = 3;
  double total_cost = 4;
  string currency = 5;
  string note = 6;
}
```

### Well-known subject keys

`cluster`, `namespace`, `controller_kind`, `controller`, `pod`, `node`,
`label.<key>`, and `kind` (`workload`, `node`, `__idle__`, `__cluster__`).
Keys are strings rather than typed fields so label grouping and future
non-Kubernetes usage sources need no spec change; the conformance check
enforces the documented set.

### Metrics (slice 1)

Workload rows: `cpu_request`, `mem_request`. Node rows (`kind=node`):
`cpu_allocatable`, `mem_allocatable`. Later sources add `cpu_usage`,
`mem_usage`.

### Invariant

For every `AllocateResponse`, the sum of `total_cost` over all rows equals the
sum of `cost` over all `priced` entries with `priced=true`, within a relative
epsilon of 1e-6. The conformance suite and core both enforce it.

## 4. Data flow

### CLI (SP3)

```text
finfocus cost cluster
  [--context <kubecontext>]
  [--namespace <ns>] [--selector <k=v>]
  [--group-by namespace|controller|pod|node|label:<key>]   # default: namespace
  [--policy <file>] [--show-policy]
  [--usage-source <plugin>] [--allocator <plugin>]
  [--output table|json|ndjson]
```

- Uses `resolveOutputFormat`; validates `--output` before loading plugins.
- Exposed as an MCP tool; `internal/cli/testdata/mcp/tools.golden` updated.

### Core pipeline (`internal/engine/cluster.go`, orchestration only)

1. **Resolve plugins by capability.** Usage source: `--usage-source` > config
   routing > the single installed plugin with `USAGE_STATS`. Allocator likewise
   with `ALLOCATION`. Zero candidates or ambiguous candidates are errors.
2. **`GetStats`** with scope and selector.
3. **Price `priceable`** through the existing engine path and router.
   `RUN_RATE` uses `GetProjectedCost` (monthly, `hoursPerMonth = 730`);
   `HISTORICAL` (SP4+) uses `GetActualCost` over `[start, end]`.
4. **Normalize units** to one period. Run-rate: usage rates × 730, node costs
   monthly. The allocator is period-agnostic.
5. **Resolve policy**: `--policy` > `$PROJECT/.finfocus/allocation.hujson` >
   `$FINFOCUS_HOME/allocation.hujson` > none. First found wins; files do not merge
   with each other. HuJSON is standardized to JSON and sent as bytes.
6. **`Allocate`**.
7. **Verify conservation invariant.**
8. **Group** rows by the requested subject key (generic string-map aggregation;
   `controller` aliases `controller_kind/controller`) and **render**. Footer:
   mode, policy source + digest, total, idle %, and completeness.

Core never interprets Kubernetes semantics: nodes arrive as ordinary
`ResourceDescriptor`s and are priced like any other resource.

### Kubernetes plugin (SP2, `plugins/kubernetes/`)

Nested Go module importing finfocus-spec `pluginsdk` only. A `make` check
(`go list -deps`) fails if any `github.com/rshade/finfocus/internal/...` package
appears in its dependency graph.

**Usage source (run-rate):**

- client-go with standard kubeconfig loading (`KUBECONFIG`, in-cluster);
  context from `scope`.
- Lists pods (paged via `Limit`/`Continue`), skipping `Succeeded`/`Failed`.
  Sums container requests using the Kubernetes effective-request rule
  (max of largest init container and sum of app containers).
- Resolves owners to the top-level controller (Pod → ReplicaSet → Deployment,
  Job → CronJob, StatefulSet, DaemonSet, bare Pod).
- Lists nodes; emits `kind=node` rows with allocatable CPU and memory, and a
  `ResourceDescriptor{provider: aws, resource_type: ec2, sku: <instance-type
  label>, region: <topology region label>}` with the instance ID parsed from
  `providerID` in tags.
- Emits an `eks` descriptor for the control plane when the cluster name can be
  determined.
- Detects spot capacity via `eks.amazonaws.com/capacityType` or
  `karpenter.sh/capacity-type`; detects Fargate via
  `eks.amazonaws.com/compute-type=fargate`.

**Allocator:**

1. Decode policy: defaults struct, then strict `json.Unmarshal` of the user
   document into it (`DisallowUnknownFields`). Unknown `version` is an error.
   Slices replace rather than append (documented).
2. Split each node's cost into CPU and memory portions per `node_split`.
3. Charge each workload `max(request, usage) / allocatable` of each portion.
4. Emit the remainder per node as a `kind=__idle__` row.
5. Emit the control plane as a `kind=__cluster__` row.
6. Return rows, effective policy, and digest. Empty usage returns only the
   effective policy (backs `--show-policy`).

### Allocation policy (`allocation.hujson`)

Schema owned by the plugin. Slice 1 defaults:

```hujson
{
  "version": 1,
  // Keep waste visible. "share" (future) spreads idle across workloads.
  "idle": "separate",
  // kube-system and DaemonSets appear as their own rows.
  "system_workloads": "separate",
  // Divide node price into CPU/memory using public unit-price weights.
  "node_split": { "method": "unit-price-ratio" },
  // max(request, usage); slice 1 has requests only.
  "charge": "max-request-usage",
  "control_plane": "separate",
  "spot_nodes": "on-demand-with-note"
}
```

Allocation policy is deliberately separate from `config.hujson`:

- It is organizational policy meant to be committed and reviewed; plugin
  connection settings are per-user and may be secret.
- `config.hujson` merges shallowly at top-level keys, so a project override
  under `plugins.kubernetes` would replace the entire global `plugins` key.

### Registry and release (SP3b)

- `registry.json` entries gain optional `tag_prefix` (e.g. `kubernetes-`).
- `internal/registry/github.go`: when `tag_prefix` is set, list `/releases` and
  select the highest semver tag with that prefix, skipping prereleases, instead
  of `/releases/latest`. Entries without a prefix are unchanged.
- release-please: add a `plugins/kubernetes` package with its own component and
  changelog, producing tags such as `kubernetes-v0.1.0`; the plugin release
  workflow publishes assets with the names the registry already expects.

## 5. Error handling

Guiding rule: a failure must never render as a plausible number.

| Situation | Behavior | Code |
| --- | --- | --- |
| No kubeconfig / unknown context | Fatal; names context and kubeconfig path | `VALIDATION_ERROR` |
| Unauthenticated | Fatal; surfaces underlying auth hint | `PLUGIN_ERROR` |
| RBAC forbids listing pods in some namespaces | Fatal unless `--namespace` restricts to readable ones; lists missing verbs/resources and links a minimal ClusterRole | `PLUGIN_ERROR` |
| RBAC forbids listing nodes | Fatal | `PLUGIN_ERROR` |
| `--namespace` filter | Workload rows exact; idle and cluster rows omitted with a footer notice | warning |
| One node unpriced | Its rows at $0 with a note; total marked incomplete | `NO_COST_DATA` |
| All nodes unpriced | Fatal | `PLUGIN_ERROR` |
| Spot node | On-demand price with a note | note |
| Fargate pods | Own rows at $0, "Fargate pricing not supported yet"; never idle | `NO_COST_DATA` |
| Policy parse error / unknown field / unknown version | Fatal with file path and JSON path; never falls back to defaults | `VALIDATION_ERROR` |
| Explicit `--policy` missing | Fatal | `VALIDATION_ERROR` |
| No plugin with required capability | Fatal with install hint | `VALIDATION_ERROR` |
| Ambiguous plugins | Fatal; lists candidates and the flag/config key | `VALIDATION_ERROR` |
| Slow `GetStats` | Paged listing in plugin; existing plugin timeout in core | `TIMEOUT_ERROR` |
| Mixed currencies | Existing `ErrMixedCurrencies` | existing |
| Cluster with no pods | Valid; all cost is idle | — |
| Conservation invariant violated | Fatal: "allocator violated cost conservation" | `PLUGIN_ERROR` |

All failures exit with code 1, consistent with other commands.

## 6. Testing

### SP1 (finfocus-spec issues)

- `buf lint` and `buf breaking` (changes are additive).
- Allocator conformance suite in pluginsdk: conservation across fixtures
  (single node, multi-node, empty cluster, fully packed node, unpriced node);
  non-negative idle; strict policy rejection; empty-usage `Allocate` returns
  effective policy; stable digest.
- `GetStats` subject-key conformance check.

### SP2 (plugin)

- Table-driven allocator tests with golden JSON fixtures; 95% coverage target.
- Policy decode: strict rejection, defaults + override, slice-replace pinned.
- Usage source with client-go `kubernetes/fake`: owner resolution, init
  container rule, phase filtering, `providerID` parsing, spot and Fargate
  labels, paging, RBAC `Forbidden`.
- SP1 conformance suite against the real allocator.
- Dependency-boundary `make` check.

### SP3 (core)

- Pipeline tests with in-process fake usage-source and allocator plugins:
  plugin resolution (0/1/many), partial pricing, conservation violation from a
  deliberately broken fake, mixed currencies.
- Policy resolution order and "broken discovered file is fatal", with
  `t.Setenv("FINFOCUS_HOME", t.TempDir())` in every test.
- `--output` validated before plugin load.
- Group-by aggregation including `label:<key>`, controller alias, `__idle__`.
- Table golden tests; JSON/NDJSON shape tests.
- MCP `tools.golden` regenerated; live `tools/list` integration test via the
  `ax.Execute` helper.

### SP3b (registry)

`httptest` GitHub API: prefix match, highest semver wins over newest publish
date, prereleases skipped, no-match error, unprefixed entries unchanged.

### E2E (`make test-e2e-kind`, no AWS credentials)

- kind cluster with nodes labeled `node.kubernetes.io/instance-type=m5.large`
  and `topology.kubernetes.io/region=us-east-1` via kind config.
- Fixed workloads with known requests: Deployment, StatefulSet, DaemonSet,
  CronJob.
- Real `finfocus` binary, real kubernetes plugin, real aws-public plugin with
  embedded pricing. No stubbed costs.
- Assert conservation; Deployment cost equals
  `request / allocatable × m5.large monthly price` within epsilon; idle is the
  remainder; `--namespace` omits idle; bad policy exits 1.
- CI job via `helm/kind-action`. SP4 extends the same kind setup with
  Prometheus.

## 7. Proposed changes requiring approval

- **`.golangci.yml` `depguard` rule** forbidding `plugins/kubernetes` from
  importing `finfocus/internal`. Not made without explicit approval; the
  `go list -deps` `make` check covers the boundary meanwhile.

## 8. Plan-time amendments

Codebase research while writing the implementation plans changed these details.
Where this section and earlier sections disagree, this section wins.

1. **pluginsdk serving is SDK work.** `serveGRPC`/`serveConnect` hardcode
   `CostSourceService`, so both finfocus-spec issues include registration in gRPC
   and Connect modes, the health checker, and harness support — not only protos.
2. **Plugin selection** for `cost cluster` is explicit flag > the single plugin
   declaring the capability. Config routing for these capabilities is deferred
   (router features are a fixed list in `internal/router/features.go`).
3. **Policy standardization** uses `ax.ParseConfig` (the project's HuJSON path),
   not `hujson.Standardize`.
4. **No usage scaling.** Allocation uses ratios of request to allocatable, so run-rate
   usage is not multiplied by 730; costs are monthly because nodes are priced with
   projected monthly cost.
5. **`$0` means unpriced.** aws-public answers unknown instance types with `$0`, not an
   error; core treats a priceable with `Monthly <= 0` as unpriced.
6. **Node descriptors** use `resource_type aws:ec2/instance:Instance` with `sku` and
   `region` passed as engine properties (read by `resolveSKUAndRegion`). Provider comes
   from label `finfocus.dev/provider`, else the `providerID` scheme.
7. **The kubernetes plugin opts out of pricing** by implementing `Supports` (always
   false). finfocus-spec v0.6.2 (#507, released 2026-09-28) delivers that answer to hosts
   without a custom `RegistryLookup`; earlier SDKs errored and the engine failed open, which
   would have recorded a "not supported" error per resource in `cost projected`.
8. **Explicit capabilities**: the plugin declares only `USAGE_STATS` and `ALLOCATION`
   because `inferCapabilities` always adds projected/actual/pricing/estimate.
9. **SP3b is larger**: the registry must use a *canonical version* (tag minus prefix)
   for install directories, asset names, and version comparison; prefixed lookups scan
   100 releases; CLI workflows (`goreleaser.yml`, `nightly.yml`, `git describe`) ignore
   plugin tags; plugin assets come from a generic script, not GoReleaser.
   Added during execution: plugin releases are un-marked as the repo's "Latest" (the CLI
   release is re-marked), GoReleaser gets explicit current/previous CLI tags, `git describe`
   uses `--match 'v[0-9]*'`, and monorepo plugin names must not start with `v`
   (`tag_prefix` pattern `^[a-uw-z0-9][a-z0-9-]*-$`); release-tag values reach `run:` only via `env`.
10. **Policy v1 has no list fields**, so the "lists replace" rule is documented but
    has nothing to test yet; enum fields accept only their single supported value.
11. **Fargate detection** in the allocator uses the `fargate-` node-name prefix; the
    usage source omits Fargate nodes' capacity rows and priceable entries.
12. **MCP helpers** (`resolveOutputFormat`, `tools.golden`) come from PR #1510, which
    SP3 waits for.
13. **kind E2E** labels nodes with `kubectl label` (instance type, region,
    `finfocus.dev/provider=aws`) and installs the released aws-public with
    `--metadata region=us-east-1`; expected costs derive from the node price the
    command reports, not a hardcoded table.
14. **SDK-owned allocation rules** (finfocus-spec 051/052 review, 2026-09-25): mixed currencies are
    `InvalidArgument` (not `FailedPrecondition`); an empty currency takes the others' single
    currency (all empty → `USD`); duplicate priced `(kind, id)` is rejected; unknown policy fields
    are reported with their JSON path. The plugin uses `pluginsdk.DecodePolicy`,
    `ValidateAllocateRequest`, and `ResolveCurrency`, and core's conservation check delegates to
    `pluginsdk.CheckConservation`, so none of these rules is implemented twice. Usage rows must
    carry `kind`, `kind=node` rows must name their node, and duplicate `(subject, metric)` rows are
    invalid (051).

## 9. Future work

- SP4 Prometheus (historical actuals, `cpu_usage`/`mem_usage`, kind CI).
- SP5 Datadog.
- SP6 OpenCost pre-allocated rows in the same `AllocationRow` shape.
- `GetProjectedCost` for `kubernetes:*` Pulumi resources and `overview`
  expansion of cluster resources.
- aws-public spot pricing; Fargate pricing.
- Idle and shared-cost redistribution policies.
- Wire the unused `plugins.<name>` config slot to deliver plugin connection
  settings.
- Graduate `plugins/kubernetes` to its own repository.
