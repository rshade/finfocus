# feat(proto): add UsageSourceService.GetStats for workload usage plugins

## Context

finfocus is adding in-cluster Kubernetes cost allocation (design:
rshade/finfocus `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md`).
Allocation joins **usage** (per-workload CPU/memory) with **prices** (node costs
from existing cost-source plugins). Usage comes from a new plugin type — a
*usage source* — backed by the Kubernetes API, Prometheus, Datadog, or others.
Usage sources have no prices, so they get their own service instead of stubbing
`CostSourceService`.

## Proto (new file `proto/finfocus/v1/usage.proto`, package `finfocus.v1`)

```proto
syntax = "proto3";
package finfocus.v1;
import "google/protobuf/timestamp.proto";
import "finfocus/v1/costsource.proto";  // ResourceDescriptor
option go_package = "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1;pbc";

service UsageSourceService {
  // GetStats returns pre-integrated usage for workloads and the priceable
  // resources (nodes, control plane) that usage runs on.
  rpc GetStats(GetStatsRequest) returns (GetStatsResponse);
}

enum StatsMode {
  STATS_MODE_UNSPECIFIED = 0;
  STATS_MODE_RUN_RATE    = 1;  // point-in-time per-hour rates
  STATS_MODE_HISTORICAL  = 2;  // integrated over [start, end]
}

message GetStatsRequest {
  string scope = 1;                     // e.g. kubeconfig context or cluster id
  google.protobuf.Timestamp start = 2;  // both unset => run-rate
  google.protobuf.Timestamp end = 3;
  map<string, string> selector = 4;     // "namespace" plus label selectors
  repeated string metrics = 5;          // empty => source defaults
}

message GetStatsResponse {
  repeated UsageRow rows = 1;
  repeated ResourceDescriptor priceable = 2;
  StatsMode mode = 3;
  repeated string warnings = 4;
}

message UsageRow {
  map<string, string> subject = 1;
  string metric = 2;
  double amount = 3;  // per-hour rate (RUN_RATE) or integrated resource-hours
  string unit = 4;    // "core", "GiB", "core-hours", "GiB-hours"
}
```

Add to `enums.proto` `PluginCapability` (after value 13, line ~189):

```proto
  // Plugin implements UsageSourceService.GetStats.
  PLUGIN_CAPABILITY_USAGE_STATS = 14;
```

## Semantics (document in proto comments and `docs/`)

- **Subject keys** (string map so label grouping and non-Kubernetes sources need
  no schema change): `cluster`, `namespace`, `controller_kind`, `controller`,
  `pod`, `node`, `label.<key>`, `kind` ∈ {`workload`, `node`}.
- **Metrics**: workload rows `cpu_request` (core), `mem_request` (GiB), later
  `cpu_usage`, `mem_usage`; node rows (`kind=node`) `cpu_allocatable`,
  `mem_allocatable`.
- **Priceable**: each node is an ordinary `ResourceDescriptor` (e.g.
  `provider=aws, resource_type=aws:ec2/instance:Instance, sku=m5.large,
  region=us-east-1, id=<node name>`, `tags.kind=node`,
  `tags.provider_id=<providerID>`, `tags.capacity_type=spot|on-demand`).
  A control plane is `tags.kind=cluster` (e.g. `resource_type=aws:eks/cluster:Cluster, sku=cluster`).
  `id` MUST equal the `node` subject value of that node's usage rows.
- A source that cannot serve the requested mode returns `InvalidArgument`.
- RBAC/permission failures return `PermissionDenied` naming the missing
  verb/resource; unauthenticated returns `Unauthenticated`.

## pluginsdk work (required — protos alone ship a dead service)

1. **Optional interface** (`sdk.go`, next to `ResolveResourceTypesProvider`):

   ```go
   // UsageSourceProvider is implemented by plugins that serve UsageSourceService.
   type UsageSourceProvider interface {
       GetStats(ctx context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error)
   }
   ```

2. **Serving**: when `ServeConfig.Plugin` implements `UsageSourceProvider`,
   register `pbc.RegisterUsageSourceServiceServer` in `serveGRPC` (next to
   `sdk.go:1172`), a Connect handler via `pbcconnect.NewUsageSourceServiceHandler`
   in `serveConnect` (next to `sdk.go:1222`), and add
   `pbcconnect.UsageSourceServiceName` to the grpchealth static checker
   (`sdk.go:1233-1238`). Apply the same tracing/unary interceptors.
3. **Capabilities**: `inferCapabilities` (`plugin_info.go:235`) appends
   `PLUGIN_CAPABILITY_USAGE_STATS` for `UsageSourceProvider`; add
   `legacyCapabilityNames` entry `"supports_usage_stats"`
   (`capability_compat.go`); bump `maxValidCapability` once both issues land.
4. **Usage-only plugins**: document (README capability table ~line 695 and
   `PLUGIN_DEVELOPER_GUIDE.md`) that a plugin which prices nothing MUST set
   `ServeConfig.PluginInfo.Capabilities` explicitly (e.g. `[USAGE_STATS]`),
   because inference always adds PROJECTED/ACTUAL/PRICING_SPEC/ESTIMATE and
   hosts would route price queries to it.
5. **Constants** (`pluginsdk/subjects.go`): `SubjectCluster`, `SubjectNamespace`,
   `SubjectControllerKind`, `SubjectController`, `SubjectPod`, `SubjectNode`,
   `SubjectKind`, `SubjectLabelPrefix = "label."`, `KindWorkload`, `KindNode`,
   `KindIdle = "__idle__"`, `KindCluster = "__cluster__"`, `MetricCPURequest`,
   `MetricMemRequest`, `MetricCPUAllocatable`, `MetricMemAllocatable`,
   `MetricCPUUsage`, `MetricMemUsage`.
6. **Harness** (`sdk/go/testing`): `func ValidateStatsResponse(resp *pbc.GetStatsResponse) error`
   rejecting unknown non-`label.` subject keys, negative amounts, unknown
   `kind`, priceable entries without `id`, and priceable `kind=node` entries
   whose `id` matches no `node` subject. (The reverse is allowed: a node that
   cannot be priced — unknown provider, Fargate — reports capacity without a
   priceable entry.) Extend the bufconn harness (or add
   `NewUsageSourceHarness(impl pbc.UsageSourceServiceServer)`) so plugins can
   call `GetStats` in tests.
7. TypeScript client: regenerate; add `UsageSourceService` client wrapper.

## Acceptance criteria

- [ ] `make generate` produces `usage.pb.go`, `usage_grpc.pb.go`, `pbcconnect/usage.connect.go`, TS `usage_pb.ts`
- [ ] `buf lint` passes; CI `buf breaking` passes (additive only)
- [ ] A plugin implementing `UsageSourceProvider` served via `pluginsdk.Serve` answers `GetStats` over gRPC **and** Connect (test for each)
- [ ] `GetPluginInfo` reports capability 14 when inferred and when set explicitly
- [ ] `IsValidCapability(14)` is true; bounds test updated
- [ ] `ValidateStatsResponse` has table-driven tests for every rejection above
- [ ] README capability table and developer guide updated, including the explicit-capabilities rule for usage-only plugins
- [ ] speckit folder `specs/051-usage-source-getstats/`

## Out of scope

Allocation (separate issue: AllocatorService.Allocate), any concrete usage source.
