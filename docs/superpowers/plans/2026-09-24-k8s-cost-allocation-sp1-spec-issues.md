# SP1 — finfocus-spec Issues (GetStats, Allocate) Implementation Plan

<!-- markdownlint-configure-file { "MD010": { "code_blocks": false } } -->

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** File two fully specified issues on `rshade/finfocus-spec` — one per RPC — so the contracts core and the kubernetes plugin depend on get built and released there.

**Architecture:** No code is written in finfocus-spec from this effort. This plan produces the issue bodies as reviewable Markdown files in this repo, files them after user confirmation, then tracks the resulting release and bumps finfocus's dependency.

**Tech Stack:** `gh` CLI, protobuf/buf (described, not run), Go pluginsdk (described).

**Spec:** `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md` (§2 "finfocus-spec delivery", §3 "Contracts")

## Global Constraints

- One issue per RPC: `UsageSourceService.GetStats` and `AllocatorService.Allocate`.
- Capability values: `PLUGIN_CAPABILITY_USAGE_STATS = 14`, `PLUGIN_CAPABILITY_ALLOCATION = 15`.
- Changes are additive; `buf breaking --against main` must pass.
- finfocus-spec conventions: next speckit folder is `specs/051-…`; CHANGELOG is release-please generated (never hand-edited); conventional commits.
- Issue bodies follow the global rule: "fully detailed prompt style issue", researched first.
- Filing issues is outward-facing: **confirm with the user before `gh issue create`.**
- Never `git commit` in finfocus; stage and hand off.

## Review Focus

- **pluginsdk cannot serve a second gRPC service today.** `serveGRPC` (`sdk.go:1172`) and `serveConnect` (`sdk.go:1222`) hardcode `CostSourceService`; `ObservabilityService` is defined but never registered. An issue that adds only protos would ship dead services. Both issues must require registration in gRPC mode, Connect mode, and the grpchealth static checker list.
- **`inferCapabilities` claims PROJECTED/ACTUAL/PRICING_SPEC/ESTIMATE for every plugin.** A usage-only plugin would be queried for EC2 prices and could return a "valid" `$0`. The issue must state that plugins set `PluginInfo.Capabilities` explicitly and that the conformance docs show a usage/allocator-only plugin doing so.
- **Connect-path drift.** `ConnectHandler` lacks `ResolveResourceTypes` and `DryRun` today; the issues must require Connect adapters for the new services and a test that calls each RPC over Connect.
- **`IsValidCapability` upper bound.** `maxValidCapability` (`plugin_info.go:217`) and the "just above max (14)" test bound (`conformance_test.go:832/838`) must move to 15/16, or valid plugins fail validation.
- **Conservation invariant must be tested in the harness, not only documented.** Third-party allocators rely on `RunAllocatorConformance` to catch it.

---

### Task 1: Draft the `GetStats` issue body

**Files:**

- Create: `docs/superpowers/plans/issues/finfocus-spec-getstats.md`

**Interfaces:**

- Produces (names every later plan relies on — keep exact):
  - Proto: `service UsageSourceService { rpc GetStats(GetStatsRequest) returns (GetStatsResponse); }`
  - Go: `pbc.GetStatsRequest`, `pbc.GetStatsResponse`, `pbc.UsageRow`, `pbc.StatsMode_STATS_MODE_RUN_RATE`, `pbc.StatsMode_STATS_MODE_HISTORICAL`, `pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS`, `pbc.NewUsageSourceServiceClient(conn)`, `pbc.UsageSourceServiceServer`
  - pluginsdk: `type UsageSourceProvider interface { GetStats(ctx context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error) }`
  - pluginsdk constants: `SubjectCluster="cluster"`, `SubjectNamespace="namespace"`, `SubjectControllerKind="controller_kind"`, `SubjectController="controller"`, `SubjectPod="pod"`, `SubjectNode="node"`, `SubjectKind="kind"`, `SubjectLabelPrefix="label."`; kinds `KindWorkload="workload"`, `KindNode="node"`, `KindIdle="__idle__"`, `KindCluster="__cluster__"`; metrics `MetricCPURequest="cpu_request"`, `MetricMemRequest="mem_request"`, `MetricCPUAllocatable="cpu_allocatable"`, `MetricMemAllocatable="mem_allocatable"`, `MetricCPUUsage="cpu_usage"`, `MetricMemUsage="mem_usage"`
  - plugintesting: `func ValidateStatsResponse(resp *pbc.GetStatsResponse) error`

- [x] **Step 1: Write the issue body file**

````markdown
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
````

- [x] **Step 2: Lint the draft**

Run: `markdownlint -c .markdownlint.json docs/superpowers/plans/issues/finfocus-spec-getstats.md`
Expected: no output (pass).

- [ ] **Step 3: Stage and hand off**

```bash
git add docs/superpowers/plans/issues/finfocus-spec-getstats.md
```

Proposed message: `docs(plans): draft finfocus-spec GetStats issue`

---

### Task 2: Draft the `Allocate` issue body

**Files:**

- Create: `docs/superpowers/plans/issues/finfocus-spec-allocate.md`

**Interfaces:**

- Consumes: `pbc.UsageRow`, `pbc.StatsMode`, subject/kind constants from Task 1.
- Produces: `service AllocatorService { rpc Allocate(AllocateRequest) returns (AllocateResponse); }`; Go `pbc.AllocateRequest`, `pbc.AllocateResponse`, `pbc.PricedResource`, `pbc.AllocationRow`, `pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION`, `pbc.NewAllocatorServiceClient(conn)`, `pbc.AllocatorServiceServer`; pluginsdk `type AllocatorProvider interface { Allocate(ctx context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) }`; plugintesting `func RunAllocatorConformance(t *testing.T, impl pbc.AllocatorServiceServer)` and `func CheckConservation(req *pbc.AllocateRequest, resp *pbc.AllocateResponse, relEpsilon float64) error`.

- [x] **Step 1: Write the issue body file**

````markdown
# feat(proto): add AllocatorService.Allocate for cost allocation plugins

## Context

Companion to the UsageSourceService.GetStats issue. An *allocator* plugin
divides priced resources (nodes, control plane) across workloads according to a
policy. finfocus core stays domain-agnostic: it gathers usage (GetStats), prices
`priceable` through existing cost-source plugins, then calls `Allocate`.
Design: rshade/finfocus `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md`.

## Proto (new file `proto/finfocus/v1/allocation.proto`)

```proto
syntax = "proto3";
package finfocus.v1;
import "finfocus/v1/costsource.proto";  // ResourceDescriptor
import "finfocus/v1/usage.proto";       // UsageRow, StatsMode
option go_package = "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1;pbc";

service AllocatorService {
  rpc Allocate(AllocateRequest) returns (AllocateResponse);
}

message PricedResource {
  ResourceDescriptor resource = 1;
  double cost = 2;       // cost for the normalized period
  string currency = 3;
  bool priced = 4;       // false => pricing failed; cost MUST be 0
  string note = 5;
}

message AllocateRequest {
  repeated UsageRow usage = 1;
  repeated PricedResource priced = 2;
  bytes policy_json = 3;   // standard JSON; empty => plugin defaults
  StatsMode mode = 4;
}

message AllocateResponse {
  repeated AllocationRow rows = 1;
  bytes effective_policy_json = 2;   // defaults + overrides as applied
  string policy_digest = 3;          // hex sha256 of canonical effective policy
  repeated string warnings = 4;
}

message AllocationRow {
  map<string, string> subject = 1;   // kind: workload | __idle__ | __cluster__
  double cpu_cost = 2;
  double mem_cost = 3;
  double total_cost = 4;
  string currency = 5;
  string note = 6;
}
```

Add to `enums.proto` `PluginCapability`:

```proto
  // Plugin implements AllocatorService.Allocate.
  PLUGIN_CAPABILITY_ALLOCATION = 15;
```

## Contract rules (proto comments + docs)

1. **Conservation**: Σ `rows.total_cost` = Σ `priced.cost` where `priced=true`,
   within relative epsilon 1e-6. Hosts verify this and reject violations.
2. `total_cost = cpu_cost + mem_cost` for every row (control-plane rows may put
   everything in `total_cost` with cpu/mem 0 — then the equality is waived for
   `kind=__cluster__` only).
3. No negative costs. Idle is a row (`kind=__idle__`, with `node` subject), never
   silently dropped.
4. **Policy**: `policy_json` is opaque to the host. The allocator owns the
   schema, decodes strictly (unknown fields → `InvalidArgument` naming the JSON
   path), rejects unknown `version`, and merges onto its defaults.
5. Empty `usage` and empty `priced` → a response with no rows but with
   `effective_policy_json` and `policy_digest` (hosts use this to show policy).
6. The digest is stable: the same effective policy always yields the same digest.

## pluginsdk work

1. `type AllocatorProvider interface { Allocate(ctx context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) }`
2. Serving: register `AllocatorService` in `serveGRPC`, `serveConnect`, and the
   grpchealth checker when the plugin implements `AllocatorProvider` (same
   mechanism as UsageSourceProvider — implement them together if both issues are
   picked up at once).
3. `inferCapabilities` appends `PLUGIN_CAPABILITY_ALLOCATION`; legacy name
   `"supports_allocation"`; `maxValidCapability = PLUGIN_CAPABILITY_ALLOCATION`;
   update the bounds test at `conformance_test.go:832/838` ("just above max" → 16).
4. **Conformance** (`sdk/go/testing/allocator_conformance.go`):
   ```go
   // CheckConservation returns an error when rows do not sum to priced cost.
   func CheckConservation(req *pbc.AllocateRequest, resp *pbc.AllocateResponse, relEpsilon float64) error
   // RunAllocatorConformance runs the standard allocator fixtures as subtests.
   func RunAllocatorConformance(t *testing.T, impl pbc.AllocatorServiceServer)
   ```
   Fixtures (as subtests): single node; three nodes; empty cluster (all idle);
   fully packed node (idle 0); node with `priced=false`; control plane present;
   requests exceeding allocatable (idle must stay ≥ 0); `policy_json` with an
   unknown field (expect `InvalidArgument`); unknown `version` (expect
   `InvalidArgument`); empty usage+priced returns effective policy and digest;
   digest stable across two identical calls; `{}` policy digest equals empty
   policy digest.

## Acceptance criteria

- [ ] Generated Go/Connect/TS code for `allocation.proto`
- [ ] `buf lint` and CI `buf breaking` pass
- [ ] Allocator served via `pluginsdk.Serve` answers over gRPC and Connect (tests)
- [ ] Capability 15 inferred and valid; bounds test updated
- [ ] `RunAllocatorConformance` passes against an in-repo reference allocator used by the SDK's own tests
- [ ] `CheckConservation` table-driven tests (pass, over, under, priced=false ignored)
- [ ] README capability table + developer guide section "Writing an allocator"
- [ ] speckit folder `specs/052-allocator-allocate/`

## Depends on

UsageSourceService.GetStats issue (imports `UsageRow`, `StatsMode`).
````

- [x] **Step 2: Lint the draft**

Run: `markdownlint -c .markdownlint.json docs/superpowers/plans/issues/finfocus-spec-allocate.md`
Expected: pass.

- [ ] **Step 3: Stage and hand off**

```bash
git add docs/superpowers/plans/issues/finfocus-spec-allocate.md
```

Proposed message: `docs(plans): draft finfocus-spec Allocate issue`

---

### Task 3: File both issues (user-confirmed)

**Files:** none modified.

**Interfaces:**

- Consumes: the two draft files.
- Produces: two issue URLs, recorded in Task 4.

- [x] **Step 1: Ask the user to confirm filing**

Show both draft paths and ask: "File these two issues on rshade/finfocus-spec now?" Do not proceed without a yes.

- [x] **Step 2: File GetStats first (Allocate references it)**

```bash
GETSTATS_URL=$(gh issue create --repo rshade/finfocus-spec \
  --title "feat(proto): add UsageSourceService.GetStats for workload usage plugins" \
  --body-file docs/superpowers/plans/issues/finfocus-spec-getstats.md \
  --label enhancement)
echo "$GETSTATS_URL"
```

Expected: an issue URL.

- [x] **Step 3: File Allocate, linking GetStats**

```bash
BODY=$(sed "s|UsageSourceService.GetStats issue (imports|$GETSTATS_URL (imports|" \
  docs/superpowers/plans/issues/finfocus-spec-allocate.md)
gh issue create --repo rshade/finfocus-spec \
  --title "feat(proto): add AllocatorService.Allocate for cost allocation plugins" \
  --body "$BODY" --label enhancement
```

Expected: an issue URL.

---

### Task 4: Bump finfocus to finfocus-spec v0.6.2

**Status:** v0.6.2 was released 2026-09-28. It contains #505 (GetStats), #506 (Allocate), #518
(`pluginsdk.ValidateAllocateResponse`), and #510, which fixes #507 (`Supports` now reaches the
plugin) and the Connect error-code bug. Issues #505, #506, and #507 are closed.

**Files:**

- Modify: `go.mod`, `go.sum` (`github.com/rshade/finfocus-spec v0.6.1` → `v0.6.2`; also raises
  `connectrpc.com/connect` to v1.21.0 and `connectrpc.com/grpchealth` to v1.5.0 as indirect deps)
- Modify: `test/e2e/go.mod`, `test/e2e/go.sum` (same version; CI `validate` enforces the match)

**Interfaces:**

- Produces: finfocus builds against v0.6.2, which provides every name SP2 and SP3 use (verified
  2026-09-27 against the module source): `pbc.GetStatsRequest/Response`, `pbc.UsageRow`,
  `pbc.StatsMode_STATS_MODE_RUN_RATE`, `pbc.AllocateRequest/Response`, `pbc.PricedResource`,
  `pbc.AllocationRow`, `pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS` (14) and `_ALLOCATION`
  (15), `pbc.NewUsageSourceServiceClient`, `pbc.NewAllocatorServiceClient`,
  `pluginsdk.UsageSourceProvider`, `pluginsdk.AllocatorProvider`, `pluginsdk.DecodePolicy`,
  `pluginsdk.CheckConservation`, `pluginsdk.ValidateAllocateRequest`,
  `pluginsdk.ValidateAllocateResponse`, `pluginsdk.ResolveCurrency`,
  `pluginsdk.DefaultConservationEpsilon`, the `Subject*`/`Kind*`/`Metric*`/`Unit*` constants,
  `plugintesting.ValidateStatsResponse`, and `plugintesting.RunAllocatorConformance`.

**Behavior change to expect:** plugins built against v0.6.2 now have their own `Supports` answer
reach the host (before, every `Supports` call errored and the engine failed open). The recorder
plugin is built from this module and always answers `false`, which is its documented intent. Its
integration test calls `client.API` directly, so it is unaffected. Task 5 makes the engine send the
fields plugins need to answer correctly.

- [ ] **Step 1: Bump both modules**

```bash
go get github.com/rshade/finfocus-spec@v0.6.2 && go mod tidy
(cd test/e2e && go get github.com/rshade/finfocus-spec@v0.6.2 && go mod tidy)
```

- [ ] **Step 2: Verify**

Run: `make build && make test && make lint` (allow >5 minutes for lint).
Expected: PASS. Then run `make test-integration` and compare the failure set with a clean-`main`
baseline (CLAUDE.md: many integration tests read the real `~/.finfocus`); expect no new failures.

- [ ] **Step 3: Hand off**

Leave the changes unstaged (no `git add`/`git commit` in this environment).
Proposed message: `build(deps): bump finfocus-spec to v0.6.2 for usage and allocation RPCs`

---

### Task 5: Send provider and region in the engine's `Supports` request

**Why:** since v0.6.2 a plugin's `Supports` answer reaches the host. The engine's
`checkPluginSupports` (`internal/engine/engine.go:194`) sends only `resource_type`, so
finfocus-plugin-aws-public (`internal/plugin/supports.go`) answers `Provider "" not supported` for
everything. Once aws-public ships a v0.6.2 build, the engine would cache "unsupported" and never
price anything with it. The fix must land before any pricing plugin upgrades to v0.6.2.

**Files:**

- Modify: `internal/engine/engine.go` (`checkPluginSupports` :194-232, `filterUnsupportedPlugins`
  :246-259, and its three callers at :309, :344, :355)
- Test: `internal/engine/engine_supports_test.go` (new)

**Interfaces:**

- Consumes: `ConvertToProto(map[string]interface{}) map[string]string` and
  `proto.ResolveSKUAndRegion(ctx, provider, resourceType, props) (sku, region string)`, the same pair
  the batch path uses (`internal/engine/engine_batch.go:309-315`).
- Produces: `func (e *Engine) checkPluginSupports(ctx context.Context, client *pluginhost.Client,
  resource ResourceDescriptor, feature string) bool` and
  `func (e *Engine) filterUnsupportedPlugins(ctx context.Context, matches []PluginMatch,
  resource ResourceDescriptor, feature string) []PluginMatch`. The cache key includes provider and
  region: `client.Name + ":" + provider + ":" + resourceType + ":" + region + ":" + feature`.

- [ ] **Step 1: Write the failing test** (`internal/engine/engine_supports_test.go`)

```go
package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// recordingSupportsClient answers Supports like a region-bound plugin and
// records every request it receives.
type recordingSupportsClient struct {
	mockCostSourceClient
	region string
	seen   []*pbc.ResourceDescriptor
}

func (c *recordingSupportsClient) Supports(
	_ context.Context, req *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	c.seen = append(c.seen, req.GetResource())
	r := req.GetResource()
	return &pbc.SupportsResponse{Supported: r.GetProvider() == "aws" && r.GetRegion() == c.region}, nil
}

func supportsEngine(api proto.CostSourceClient) (*Engine, *pluginhost.Client) {
	client := &pluginhost.Client{Name: "aws-public", API: api}
	return New([]*pluginhost.Client{client}, nil), client
}

func TestCheckPluginSupports_SendsProviderRegionAndSKU(t *testing.T) {
	api := &recordingSupportsClient{region: "us-east-1"}
	e, client := supportsEngine(api)

	ok := e.checkPluginSupports(context.Background(), client, ResourceDescriptor{
		Type: "aws:ec2/instance:Instance", ID: "web", Provider: "aws",
		Properties: map[string]interface{}{"instanceType": "m5.large", "availabilityZone": "us-east-1a"},
	}, "ProjectedCosts")

	assert.True(t, ok)
	require.Len(t, api.seen, 1)
	assert.Equal(t, "aws", api.seen[0].GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", api.seen[0].GetResourceType())
	assert.Equal(t, "us-east-1", api.seen[0].GetRegion())
	assert.Equal(t, "m5.large", api.seen[0].GetSku())
}

func TestCheckPluginSupports_CacheKeyIncludesRegion(t *testing.T) {
	api := &recordingSupportsClient{region: "us-east-1"}
	e, client := supportsEngine(api)
	res := func(az string) ResourceDescriptor {
		return ResourceDescriptor{Type: "aws:ec2/instance:Instance", Provider: "aws",
			Properties: map[string]interface{}{"instanceType": "t3.micro", "availabilityZone": az}}
	}

	assert.True(t, e.checkPluginSupports(context.Background(), client, res("us-east-1a"), "ProjectedCosts"))
	assert.False(t, e.checkPluginSupports(context.Background(), client, res("us-west-2a"), "ProjectedCosts"),
		"a different region must not reuse the us-east-1 answer")
	assert.True(t, e.checkPluginSupports(context.Background(), client, res("us-east-1b"), "ProjectedCosts"))
	assert.Len(t, api.seen, 2, "same provider, type, region, and feature is served from cache")
}
```

Before writing, confirm `mockCostSourceClient` (`internal/engine/budget_engine_test.go:21`)
implements `proto.CostSourceClient` with a usable zero value, and that `New(clients, nil)` accepts a
nil loader (`engine.go:133`). If either differs, adapt the setup and keep the assertions.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/engine/ -run TestCheckPluginSupports -v`
Expected: FAIL to compile, since `checkPluginSupports` still takes a `resourceType string`.

- [ ] **Step 3: Implement**

In `checkPluginSupports`, build the descriptor from the resource and key the cache on everything
that can change the answer:

```go
func (e *Engine) checkPluginSupports(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
	feature string,
) bool {
	props := ConvertToProto(resource.Properties)
	sku, region := proto.ResolveSKUAndRegion(ctx, resource.Provider, resource.Type, props)
	cacheKey := strings.Join([]string{client.Name, resource.Provider, resource.Type, region, feature}, ":")

	e.supportsMu.RLock()
	if result, ok := e.supportsCache[cacheKey]; ok {
		e.supportsMu.RUnlock()
		return result
	}
	e.supportsMu.RUnlock()

	resp, err := client.API.Supports(ctx, &pbc.SupportsRequest{
		Resource: &pbc.ResourceDescriptor{
			Provider:     resource.Provider,
			ResourceType: resource.Type,
			Sku:          sku,
			Region:       region,
			Tags:         props,
		},
	})
	// ... remainder unchanged (fail-open on error, debug log, cache, return)
}
```

`filterUnsupportedPlugins` takes `resource ResourceDescriptor` instead of `resourceType string` and
passes it through; its three callers pass `resource` instead of `resource.Type`. Update the
function's doc comment: fail-open now covers only plugins built on SDKs older than v0.6.2 and RPC
failures. Add a `resource_region` field to the existing debug log.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/engine/... && make test && make lint`
Expected: PASS.

- [ ] **Step 5: Hand off**

Leave unstaged. Proposed message:
`fix(engine): send provider, region, and SKU in plugin Supports checks`

Add to CLAUDE.md (Engine gotchas): "`checkPluginSupports` sends provider, type, SKU, and region, and
caches per provider+type+region+feature; plugins on finfocus-spec ≥ v0.6.2 answer `Supports` for
real, so a region-bound plugin (aws-public) declines other regions instead of being called and
failing."
