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
