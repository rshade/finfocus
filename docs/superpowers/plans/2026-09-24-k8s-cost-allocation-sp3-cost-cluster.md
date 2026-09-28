# SP3 — `finfocus cost cluster` Implementation Plan

<!-- markdownlint-configure-file { "MD010": { "code_blocks": false } } -->

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `finfocus cost cluster`, which gathers usage from a usage-source plugin, prices the reported nodes through the existing projected-cost engine, has an allocator plugin divide the cost, verifies conservation, and renders grouped rows as table, JSON, or NDJSON.

**Architecture:** Core stays Kubernetes-agnostic. `internal/engine/cluster.go` orchestrates against three small interfaces (usage source, allocator, pricer) so tests inject fakes without `openPlugins`. Nodes arrive as ordinary `ResourceDescriptor`s and are priced by `Engine.GetProjectedCostWithErrors`. Grouping is generic aggregation over a string map. The allocation policy file is resolved by `internal/config` and passed through as opaque JSON.

**Tech Stack:** Go 1.27.1, Cobra, ax-go (`ax.ParseConfig`, `axtest.Run`, MCP), gRPC generated clients from finfocus-spec (SP1 release), testify, kind (E2E).

**Spec:** `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md` (§4 CLI + core pipeline, §5, §6 SP3 + E2E)

## Prerequisites

- SP1 Task 4 done: `go.mod` pins a finfocus-spec release with `usage.proto` and `allocation.proto`.
- PR #1510 merged into `main` (provides `resolveOutputFormat`, `machineOutputRequested`, `writeJSON`, `internal/cli/testdata/mcp/tools.golden`, `TestMCPSchemaToolAllowList`, `TestMCPServer_ToolListMatchesGoldenForBothEntryPoints`). Branch from `main` after the merge.
- Task 9 (kind E2E) additionally needs SP2 merged.

## Global Constraints

- Command: `finfocus cost cluster [--context] [--namespace] [--selector k=v]... [--group-by namespace|controller|pod|node|label:<key>] [--policy <file>] [--show-policy] [--usage-source <plugin>] [--allocator <plugin>] [--output table|json|ndjson]`; default `--group-by namespace`.
- Plugin choice: explicit flag > the single installed plugin with the capability; zero or several candidates → error listing candidates. (Config routing for these capabilities is deferred; see spec deviations in the index.)
- Policy resolution: `--policy` > `$PROJECT/.finfocus/allocation.hujson` > `<ResolveConfigDir()>/allocation.hujson` > none; first found wins; a discovered file that fails to parse is fatal.
- Run-rate costs are monthly (730 h, `engine.HoursPerMonth`) because nodes are priced with projected monthly cost; allocation uses ratios only, so no unit scaling is applied to usage.
- Conservation check: `pluginsdk.DefaultConservationEpsilon` (1e-6 relative, 1e-9 absolute floor for
  zero totals), computed by the SDK's `pluginsdk.CheckConservation(req, resp, relEpsilon)`
  (finfocus-spec 052 FR-032) so core,
  allocators, and the conformance suite apply one rule. Currency resolution likewise uses
  `pluginsdk.ResolveCurrency(priced)` (an empty currency takes the others' single currency).
- A priceable whose projected monthly cost is `<= 0` or carries a `StructuredError` is **unpriced** (aws-public returns `$0` with "not found in pricing data" instead of an error; a real node is never free).
- `--namespace` omits `__idle__` and `__cluster__` rows and says so.
- Validate `--output` and `--group-by` before loading plugins or state (CLAUDE.md gotcha).
- Every test that touches config sets `FINFOCUS_HOME` to a temp dir (`isolateConfig(t)` in `internal/cli`).
- Use `RunE`, `cmd.Printf`/`cmd.OutOrStdout()`; errors exit 1.
- testify `require`/`assert` only.
- Never `git commit`; stage and hand off. `make lint` (long) + `make test` before completion.

## Review Focus

- **`$0` node price treated as a real price.** aws-public answers unknown instance types with a `$0` response, not an error; without the `<= 0 ⇒ unpriced` rule every workload on that node looks free and nothing is flagged (Task 2 test `zero-cost node is unpriced`).
- **Installed kubernetes plugin polluting other commands.** `cost projected` asks every plugin; the plugin must be skipped via its `Supports=false` (SP2) so no per-resource "not supported" errors appear (Task 7 integration test).
- **Priced result matched to the wrong descriptor.** Results must be joined by `(type, id)`, not by slice index (Task 2 test `results out of order`).
- **Label grouping on pods without the label** must group under `<none>`, not drop cost (Task 3 test).
- **`--show-policy` with no usage plugin installed** should still work when only an allocator is installed, and must not contact the cluster (Task 5 test).

---

### Task 1: Capability names for the new services

**Files:**

- Modify: `internal/pluginhost/host.go` (`ConvertCapabilities`, ~:156-213)
- Test: `internal/pluginhost/host_test.go` (or the file holding existing `ConvertCapabilities` tests — find with `grep -rn "ConvertCapabilities" internal/pluginhost/*_test.go`)

**Interfaces:**

- Produces: constants `pluginhost.CapabilityUsageStats = "usage_stats"`, `pluginhost.CapabilityAllocation = "allocation"`; `ConvertCapabilities` maps enums 14/15 to them; `(*Client).HasCapability` works for both.

- [ ] **Step 1: Write the failing test**

```go
func TestConvertCapabilities_UsageAndAllocation(t *testing.T) {
	got := ConvertCapabilities([]pbc.PluginCapability{
		pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
		pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
	})
	assert.Equal(t, []string{CapabilityUsageStats, CapabilityAllocation}, got)

	c := &Client{Metadata: &proto.PluginMetadata{Capabilities: got}}
	assert.True(t, c.HasCapability(CapabilityUsageStats))
	assert.True(t, c.HasCapability(CapabilityAllocation))
}
```

(Import `proto "github.com/rshade/finfocus/internal/proto"` if the test file does not already.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/pluginhost/ -run TestConvertCapabilities_UsageAndAllocation`
Expected: FAIL — `undefined: CapabilityUsageStats`.

- [ ] **Step 3: Implement**

In `host.go`:

```go
// Capability names for the usage-source and allocator services.
const (
	CapabilityUsageStats = "usage_stats"
	CapabilityAllocation = "allocation"
)
```

and in the `ConvertCapabilities` switch:

```go
		case pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS:
			result = append(result, CapabilityUsageStats)
		case pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION:
			result = append(result, CapabilityAllocation)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/pluginhost/...`
Expected: PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/pluginhost/host.go internal/pluginhost/*_test.go
```

Proposed message: `feat(pluginhost): recognize usage_stats and allocation capabilities`

---

### Task 2: Engine cluster pipeline

**Files:**

- Create: `internal/engine/cluster.go`
- Test: `internal/engine/cluster_test.go`

**Interfaces:**

- Consumes: `Engine.GetProjectedCostWithErrors(ctx, []ResourceDescriptor) (*CostResultWithErrors, error)`; `CostResult{ResourceType, ResourceID, Currency, Monthly, Notes, Error *StructuredError}`; `ErrMixedCurrencies`; generated `pbc` types (SP1).
- Produces:

  ```go
  type UsageSource interface {
      GetStats(ctx context.Context, in *pbc.GetStatsRequest, opts ...grpc.CallOption) (*pbc.GetStatsResponse, error)
  }
  type Allocator interface {
      Allocate(ctx context.Context, in *pbc.AllocateRequest, opts ...grpc.CallOption) (*pbc.AllocateResponse, error)
  }
  type ResourcePricer interface {
      GetProjectedCostWithErrors(ctx context.Context, resources []ResourceDescriptor) (*CostResultWithErrors, error)
  }
  type ClusterRequest struct {
      Scope      string
      Namespace  string
      Selector   map[string]string
      PolicyJSON []byte
  }
  type ClusterRow struct {
      Subject   map[string]string `json:"subject"`
      CPUCost   float64           `json:"cpu_cost"`
      MemCost   float64           `json:"mem_cost"`
      TotalCost float64           `json:"total_cost"`
      Note      string            `json:"note,omitempty"`
  }
  type PricedSummary struct {
      Kind         string  `json:"kind"`
      ID           string  `json:"id"`
      ResourceType string  `json:"resource_type"`
      SKU          string  `json:"sku"`
      Monthly      float64 `json:"monthly"`
      Priced       bool    `json:"priced"`
      Note         string  `json:"note,omitempty"`
  }
  type ClusterResult struct {
      Mode            string          // "run-rate"
      Currency        string
      Rows            []ClusterRow
      Priced          []PricedSummary
      Total           float64
      Idle            float64
      NamespaceScoped bool
      Incomplete      bool
      PolicyDigest    string
      EffectivePolicy json.RawMessage
      Warnings        []string
  }
  func RunClusterAllocation(ctx context.Context, usage UsageSource, alloc Allocator, pricer ResourcePricer, req ClusterRequest) (*ClusterResult, error)
  func ShowAllocationPolicy(ctx context.Context, alloc Allocator, policyJSON []byte) (json.RawMessage, string, error)
  func VerifyConservation(req *pbc.AllocateRequest, resp *pbc.AllocateResponse) error // wraps pluginsdk.ValidateAllocateResponse + CheckConservation
  func PriceableToResource(d *pbc.ResourceDescriptor) ResourceDescriptor
  const ModeRunRate = "run-rate"
  var ErrConservation = errors.New("allocator violated cost conservation")
  var ErrHistoricalUnsupported = errors.New("historical usage is not supported yet")
  ```

  The generated `pbc.UsageSourceServiceClient` and `pbc.AllocatorServiceClient` satisfy `UsageSource` and `Allocator` directly; `*Engine` satisfies `ResourcePricer`.

- [ ] **Step 1: Write failing tests** (`internal/engine/cluster_test.go`)

```go
package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type fakeUsage struct {
	resp *pbc.GetStatsResponse
	err  error
	got  *pbc.GetStatsRequest
}

func (f *fakeUsage) GetStats(_ context.Context, in *pbc.GetStatsRequest, _ ...grpc.CallOption) (*pbc.GetStatsResponse, error) {
	f.got = in
	return f.resp, f.err
}

type fakeAlloc struct {
	fn  func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error)
	got *pbc.AllocateRequest
}

func (f *fakeAlloc) Allocate(_ context.Context, in *pbc.AllocateRequest, _ ...grpc.CallOption) (*pbc.AllocateResponse, error) {
	f.got = in
	return f.fn(in)
}

type fakePricer struct{ results []CostResult }

func (f fakePricer) GetProjectedCostWithErrors(_ context.Context, _ []ResourceDescriptor) (*CostResultWithErrors, error) {
	return &CostResultWithErrors{Results: f.results}, nil
}

func nodeDesc(id string) *pbc.ResourceDescriptor {
	return &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:ec2/instance:Instance",
		Sku: "m5.large", Region: "us-east-1", Id: id, Tags: map[string]string{"kind": "node"}}
}

// conservingAlloc splits each priced resource 60/40 into a workload row and an idle row.
func conservingAlloc() *fakeAlloc {
	return &fakeAlloc{fn: func(r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
		resp := &pbc.AllocateResponse{PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`)}
		for _, p := range r.GetPriced() {
			if !p.GetPriced() {
				continue
			}
			id := p.GetResource().GetId()
			resp.Rows = append(resp.Rows,
				&pbc.AllocationRow{Subject: map[string]string{"kind": "workload", "namespace": "app", "pod": "p-" + id, "node": id},
					CpuCost: p.GetCost() * 0.6, TotalCost: p.GetCost() * 0.6, Currency: "USD"},
				&pbc.AllocationRow{Subject: map[string]string{"kind": "__idle__", "node": id},
					CpuCost: p.GetCost() * 0.4, TotalCost: p.GetCost() * 0.4, Currency: "USD"},
			)
		}
		return resp, nil
	}}
}

func TestRunClusterAllocation_HappyPath(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2")},
		Warnings:  []string{"w1"},
	}}
	pricer := fakePricer{results: []CostResult{ // deliberately out of order
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 30, Currency: "USD"},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD"},
	}}
	alloc := conservingAlloc()

	res, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{
		Scope: "prod", Selector: map[string]string{"app": "web"}, PolicyJSON: []byte(`{}`),
	})
	require.NoError(t, err)

	assert.Equal(t, "prod", usage.got.GetScope())
	assert.Equal(t, "web", usage.got.GetSelector()["app"])
	require.Len(t, alloc.got.GetPriced(), 2)
	byID := map[string]float64{}
	for _, p := range alloc.got.GetPriced() {
		byID[p.GetResource().GetId()] = p.GetCost()
	}
	assert.InDelta(t, 70, byID["n1"], 1e-9, "results out of order must still match by id")
	assert.InDelta(t, 30, byID["n2"], 1e-9)
	assert.Equal(t, []byte(`{}`), alloc.got.GetPolicyJson())

	assert.Equal(t, ModeRunRate, res.Mode)
	assert.InDelta(t, 100, res.Total, 1e-9)
	assert.InDelta(t, 40, res.Idle, 1e-9)
	assert.False(t, res.Incomplete)
	assert.Equal(t, "USD", res.Currency)
	assert.Equal(t, "d", res.PolicyDigest)
	assert.Contains(t, res.Warnings, "w1")
	assert.Len(t, res.Priced, 2)
}

func TestRunClusterAllocation_UnpricedResources(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode:      pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2"), nodeDesc("n3")},
	}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 70, Currency: "USD"},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 0, Currency: "USD",
			Notes: `EC2 instance type "x9.huge" not found in pricing data`},
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n3", Currency: "USD",
			Error: &StructuredError{Code: ErrCodeNoCostData, Message: "No pricing information available"}},
	}}
	alloc := conservingAlloc()
	res, err := RunClusterAllocation(context.Background(), usage, alloc, pricer, ClusterRequest{})
	require.NoError(t, err)

	priced := map[string]*pbc.PricedResource{}
	for _, p := range alloc.got.GetPriced() {
		priced[p.GetResource().GetId()] = p
	}
	assert.True(t, priced["n1"].GetPriced())
	assert.False(t, priced["n2"].GetPriced(), "zero-cost node is unpriced")
	assert.Contains(t, priced["n2"].GetNote(), "not found in pricing data")
	assert.False(t, priced["n3"].GetPriced())
	assert.True(t, res.Incomplete)
	assert.InDelta(t, 70, res.Total, 1e-9)
}

func TestRunClusterAllocation_AllUnpricedIsFatal(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE, Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}}
	pricer := fakePricer{results: []CostResult{{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1"}}}
	_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no priceable resource could be priced")
}

func TestRunClusterAllocation_NamespaceScopeDropsSharedRows(t *testing.T) {
	usage := &fakeUsage{resp: &pbc.GetStatsResponse{
		Mode: pbc.StatsMode_STATS_MODE_RUN_RATE, Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}}
	pricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 100, Currency: "USD"}}}
	res, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer,
		ClusterRequest{Namespace: "app"})
	require.NoError(t, err)
	assert.Equal(t, "app", usage.got.GetSelector()["namespace"])
	assert.True(t, res.NamespaceScoped)
	for _, r := range res.Rows {
		assert.Equal(t, "workload", r.Subject["kind"])
	}
	assert.InDelta(t, 60, res.Total, 1e-9)
	assert.Zero(t, res.Idle)
}

func TestRunClusterAllocation_Errors(t *testing.T) {
	okUsage := &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
		Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1")}}
	okPricer := fakePricer{results: []CostResult{
		{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 100, Currency: "USD"}}}

	t.Run("usage error wrapped", func(t *testing.T) {
		_, err := RunClusterAllocation(context.Background(), &fakeUsage{err: errors.New("forbidden")},
			conservingAlloc(), okPricer, ClusterRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get usage stats: forbidden")
	})
	t.Run("historical mode rejected", func(t *testing.T) {
		_, err := RunClusterAllocation(context.Background(),
			&fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_HISTORICAL}},
			conservingAlloc(), okPricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrHistoricalUnsupported)
	})
	t.Run("conservation violation", func(t *testing.T) {
		broken := &fakeAlloc{fn: func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
			return &pbc.AllocateResponse{
				PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`),
				Rows: []*pbc.AllocationRow{
					{Subject: map[string]string{"kind": "workload", "namespace": "app", "pod": "p"},
						CpuCost: 88, TotalCost: 88, Currency: "USD"},
					{Subject: map[string]string{"kind": "__idle__", "node": "n1"}, Currency: "USD"},
				},
			}, nil
		}}
		_, err := RunClusterAllocation(context.Background(), &fakeUsage{resp: okUsage}, broken, okPricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrConservation)
		assert.Contains(t, err.Error(), "88", "message reports the actual total")
	})
	t.Run("contract violation", func(t *testing.T) {
		noIdle := &fakeAlloc{fn: func(*pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
			return &pbc.AllocateResponse{
				PolicyDigest: "d", EffectivePolicyJson: []byte(`{"version":1}`),
				Rows: []*pbc.AllocationRow{{Subject: map[string]string{"kind": "workload", "namespace": "app", "pod": "p"},
					CpuCost: 100, TotalCost: 100, Currency: "USD"}},
			}, nil
		}}
		_, err := RunClusterAllocation(context.Background(), &fakeUsage{resp: okUsage}, noIdle, okPricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrConservation, "a priced node without an idle row is rejected even when totals balance")
	})
	t.Run("mixed currencies", func(t *testing.T) {
		usage := &fakeUsage{resp: &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE,
			Priceable: []*pbc.ResourceDescriptor{nodeDesc("n1"), nodeDesc("n2")}}}
		pricer := fakePricer{results: []CostResult{
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n1", Monthly: 1, Currency: "USD"},
			{ResourceType: "aws:ec2/instance:Instance", ResourceID: "n2", Monthly: 1, Currency: "EUR"}}}
		_, err := RunClusterAllocation(context.Background(), usage, conservingAlloc(), pricer, ClusterRequest{})
		require.ErrorIs(t, err, ErrMixedCurrencies)
	})
}

func TestShowAllocationPolicy(t *testing.T) {
	alloc := &fakeAlloc{fn: func(r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
		assert.Empty(t, r.GetUsage())
		assert.Empty(t, r.GetPriced())
		return &pbc.AllocateResponse{EffectivePolicyJson: []byte(`{"version":1}`), PolicyDigest: "abc"}, nil
	}}
	pol, digest, err := ShowAllocationPolicy(context.Background(), alloc, []byte(`{"idle":"separate"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":1}`, string(pol))
	assert.Equal(t, "abc", digest)
	assert.Equal(t, []byte(`{"idle":"separate"}`), alloc.got.GetPolicyJson())
}

func TestPriceableToResource(t *testing.T) {
	r := PriceableToResource(&pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:eks/cluster:Cluster",
		Sku: "cluster", Region: "us-west-2", Id: "prod", Tags: map[string]string{"kind": "cluster"}})
	assert.Equal(t, "aws", r.Provider)
	assert.Equal(t, "aws:eks/cluster:Cluster", r.Type)
	assert.Equal(t, "prod", r.ID)
	assert.Equal(t, "cluster", r.Properties["sku"])
	assert.Equal(t, "us-west-2", r.Properties["region"])
	assert.Equal(t, "cluster", r.Properties["kind"])
	require.NoError(t, r.Validate())
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/engine/ -run 'Cluster|ShowAllocationPolicy|PriceableToResource'`
Expected: FAIL — `undefined: RunClusterAllocation`.

- [ ] **Step 3: Implement** (`internal/engine/cluster.go`)

Property keys: the adapter's `resolveSKUAndRegion` (`internal/proto/adapter.go:1016`) reads AWS SKU via `mapping.ExtractAWSSKU` then the fallback keys `dbInstanceClass, sku, type, tier`, and region via `region`; so `sku` and `region` properties reach the plugin unchanged.

```go
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// ModeRunRate labels results computed from point-in-time usage and monthly prices.
const ModeRunRate = "run-rate"

var (
	// ErrConservation reports allocator output that does not sum to the priced total.
	ErrConservation = errors.New("allocator violated cost conservation")
	// ErrHistoricalUnsupported reports historical usage, which needs actual-cost pricing.
	ErrHistoricalUnsupported = errors.New("historical usage is not supported yet")
)

// UsageSource is satisfied by pbc.UsageSourceServiceClient.
type UsageSource interface {
	GetStats(ctx context.Context, in *pbc.GetStatsRequest, opts ...grpc.CallOption) (*pbc.GetStatsResponse, error)
}

// Allocator is satisfied by pbc.AllocatorServiceClient.
type Allocator interface {
	Allocate(ctx context.Context, in *pbc.AllocateRequest, opts ...grpc.CallOption) (*pbc.AllocateResponse, error)
}

// ResourcePricer is satisfied by *Engine.
type ResourcePricer interface {
	GetProjectedCostWithErrors(ctx context.Context, resources []ResourceDescriptor) (*CostResultWithErrors, error)
}

// ClusterRequest selects the cluster and policy for an allocation run.
type ClusterRequest struct {
	Scope      string
	Namespace  string
	Selector   map[string]string
	PolicyJSON []byte
}

// ClusterRow is one allocator row.
type ClusterRow struct {
	Subject   map[string]string `json:"subject"`
	CPUCost   float64           `json:"cpu_cost"`
	MemCost   float64           `json:"mem_cost"`
	TotalCost float64           `json:"total_cost"`
	Note      string            `json:"note,omitempty"`
}

// PricedSummary reports how one priceable resource was priced.
type PricedSummary struct {
	Kind         string  `json:"kind"`
	ID           string  `json:"id"`
	ResourceType string  `json:"resource_type"`
	SKU          string  `json:"sku"`
	Monthly      float64 `json:"monthly"`
	Priced       bool    `json:"priced"`
	Note         string  `json:"note,omitempty"`
}

// ClusterResult is the outcome of RunClusterAllocation.
type ClusterResult struct {
	Mode            string
	Currency        string
	Rows            []ClusterRow
	Priced          []PricedSummary
	Total           float64
	Idle            float64
	NamespaceScoped bool
	Incomplete      bool
	PolicyDigest    string
	EffectivePolicy json.RawMessage
	Warnings        []string
}

// RunClusterAllocation gathers usage, prices the reported resources through the
// projected-cost path, allocates, and verifies that rows sum to the priced total.
func RunClusterAllocation(
	ctx context.Context,
	usage UsageSource,
	alloc Allocator,
	pricer ResourcePricer,
	req ClusterRequest,
) (*ClusterResult, error) {
	selector := map[string]string{}
	for k, v := range req.Selector {
		selector[k] = v
	}
	if req.Namespace != "" {
		selector["namespace"] = req.Namespace
	}
	stats, err := usage.GetStats(ctx, &pbc.GetStatsRequest{Scope: req.Scope, Selector: selector})
	if err != nil {
		return nil, fmt.Errorf("get usage stats: %w", err)
	}
	if stats.GetMode() != pbc.StatsMode_STATS_MODE_RUN_RATE {
		return nil, fmt.Errorf("%w (usage source returned %s)", ErrHistoricalUnsupported, stats.GetMode())
	}

	priced, summaries, currency, err := priceResources(ctx, pricer, stats.GetPriceable())
	if err != nil {
		return nil, err
	}

	allocReq := &pbc.AllocateRequest{
		Usage: stats.GetRows(), Priced: priced, PolicyJson: req.PolicyJSON, Mode: stats.GetMode(),
	}
	resp, err := alloc.Allocate(ctx, allocReq)
	if err != nil {
		return nil, fmt.Errorf("allocate: %w", err)
	}
	if err := VerifyConservation(allocReq, resp); err != nil {
		return nil, err
	}

	res := &ClusterResult{
		Mode:            ModeRunRate,
		Currency:        currency,
		Priced:          summaries,
		NamespaceScoped: req.Namespace != "",
		PolicyDigest:    resp.GetPolicyDigest(),
		EffectivePolicy: json.RawMessage(resp.GetEffectivePolicyJson()),
		Warnings:        append(append([]string{}, stats.GetWarnings()...), resp.GetWarnings()...),
	}
	for _, s := range summaries {
		if !s.Priced {
			res.Incomplete = true
		}
	}
	for _, r := range resp.GetRows() {
		kind := r.GetSubject()["kind"]
		if res.NamespaceScoped && kind != "workload" {
			continue
		}
		res.Rows = append(res.Rows, ClusterRow{
			Subject: r.GetSubject(), CPUCost: r.GetCpuCost(), MemCost: r.GetMemCost(),
			TotalCost: r.GetTotalCost(), Note: r.GetNote(),
		})
		res.Total += r.GetTotalCost()
		if kind == "__idle__" {
			res.Idle += r.GetTotalCost()
		}
	}
	return res, nil
}

// ShowAllocationPolicy asks the allocator for its effective policy without usage.
func ShowAllocationPolicy(ctx context.Context, alloc Allocator, policyJSON []byte) (json.RawMessage, string, error) {
	resp, err := alloc.Allocate(ctx, &pbc.AllocateRequest{PolicyJson: policyJSON})
	if err != nil {
		return nil, "", fmt.Errorf("allocate: %w", err)
	}
	return json.RawMessage(resp.GetEffectivePolicyJson()), resp.GetPolicyDigest(), nil
}

// VerifyConservation rejects allocator output that breaks the allocation
// contract (row kinds, idle rows, negative or non-finite costs, currency) or
// does not sum to the successfully priced total. Both checks are the SDK's, so
// core, allocators, and the conformance suite apply one rule.
func VerifyConservation(req *pbc.AllocateRequest, resp *pbc.AllocateResponse) error {
	if err := pluginsdk.ValidateAllocateResponse(req, resp); err != nil {
		return fmt.Errorf("%w: %w", ErrConservation, err)
	}
	if err := pluginsdk.CheckConservation(req, resp, pluginsdk.DefaultConservationEpsilon); err != nil {
		return fmt.Errorf("%w: %w", ErrConservation, err)
	}
	return nil
}

// PriceableToResource converts a usage source's priceable descriptor into the
// engine's resource shape; sku and region travel as properties.
func PriceableToResource(d *pbc.ResourceDescriptor) ResourceDescriptor {
	props := map[string]interface{}{"sku": d.GetSku(), "region": d.GetRegion()}
	for k, v := range d.GetTags() {
		props[k] = v
	}
	return ResourceDescriptor{Type: d.GetResourceType(), ID: d.GetId(), Provider: d.GetProvider(), Properties: props}
}

func priceResources(
	ctx context.Context,
	pricer ResourcePricer,
	descs []*pbc.ResourceDescriptor,
) ([]*pbc.PricedResource, []PricedSummary, string, error) {
	resources := make([]ResourceDescriptor, len(descs))
	for i, d := range descs {
		resources[i] = PriceableToResource(d)
	}
	results, err := pricer.GetProjectedCostWithErrors(ctx, resources)
	if err != nil {
		return nil, nil, "", fmt.Errorf("price cluster resources: %w", err)
	}
	byKey := make(map[string]CostResult, len(results.Results))
	for _, r := range results.Results {
		byKey[r.ResourceType+"\x00"+r.ResourceID] = r
	}

	priced := make([]*pbc.PricedResource, 0, len(descs))
	summaries := make([]PricedSummary, 0, len(descs))
	anyPriced := false
	for _, d := range descs {
		r, found := byKey[d.GetResourceType()+"\x00"+d.GetId()]
		p := &pbc.PricedResource{Resource: d}
		switch {
		case !found:
			p.Note = "no pricing result returned"
		case r.Error != nil:
			p.Note = r.Error.Message
		case r.Monthly <= 0:
			p.Note = firstNonEmpty(r.Notes, "priced at $0; treated as unpriced")
		default:
			p.Priced, p.Cost, p.Currency, p.Note = true, r.Monthly, r.Currency, r.Notes
			anyPriced = true
		}
		priced = append(priced, p)
		summaries = append(summaries, PricedSummary{
			Kind: d.GetTags()["kind"], ID: d.GetId(), ResourceType: d.GetResourceType(), SKU: d.GetSku(),
			Monthly: p.GetCost(), Priced: p.GetPriced(), Note: p.GetNote(),
		})
	}
	if len(descs) > 0 && !anyPriced {
		return nil, nil, "", errors.New("no priceable resource could be priced; check that a pricing plugin " +
			"(e.g. aws-public for the nodes' region) is installed")
	}
	currency, err := pluginsdk.ResolveCurrency(priced)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w: %w", ErrMixedCurrencies, err)
	}
	return priced, summaries, currency, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
```

If a `firstNonEmpty` helper already exists in the package, reuse it and delete this copy.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/engine/ -run 'Cluster|ShowAllocationPolicy|PriceableToResource' -v && go test ./internal/engine/...`
Expected: PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/engine/cluster.go internal/engine/cluster_test.go
```

Proposed message: `feat(engine): cluster allocation pipeline with conservation check`

---

### Task 3: Grouping

**Files:**

- Create: `internal/engine/cluster_group.go`
- Test: `internal/engine/cluster_group_test.go`

**Interfaces:**

- Consumes: `ClusterRow` (Task 2).
- Produces:

  ```go
  type ClusterGroup struct {
      Key       string   `json:"key"`
      CPUCost   float64  `json:"cpu_cost"`
      MemCost   float64  `json:"mem_cost"`
      TotalCost float64  `json:"total_cost"`
      Rows      int      `json:"rows"`
      Notes     []string `json:"notes,omitempty"`
  }
  func ValidateClusterGroupBy(groupBy string) error
  func GroupClusterRows(rows []ClusterRow, groupBy string) ([]ClusterGroup, error)
  const GroupKeyNone = "<none>"
  ```

Key rules: `__cluster__` rows → key `__cluster__` for every dimension; `__idle__` rows → the node name when grouping by `node`, else `__idle__`; workload rows: `namespace` → `namespace`; `controller` → `namespace/controller_kind/controller`; `pod` → `namespace/pod`; `node` → `node`; `label:<k>` → `label.<k>` value. Missing value → `<none>`. Notes are de-duplicated and sorted. Groups sort by `TotalCost` descending, then key ascending.

- [ ] **Step 1: Write failing tests**

```go
package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clusterRows() []ClusterRow {
	w := func(ns, pod, node, team string, cost float64, note string) ClusterRow {
		s := map[string]string{"kind": "workload", "namespace": ns, "pod": pod, "node": node,
			"controller_kind": "Deployment", "controller": "api"}
		if team != "" {
			s["label.team"] = team
		}
		return ClusterRow{Subject: s, CPUCost: cost / 2, MemCost: cost / 2, TotalCost: cost, Note: note}
	}
	return []ClusterRow{
		w("payments", "api-1", "n1", "pay", 10, ""),
		w("payments", "api-2", "n2", "pay", 10, "spot node priced on-demand"),
		w("search", "api-1", "n1", "", 5, ""),
		{Subject: map[string]string{"kind": "__idle__", "node": "n1"}, TotalCost: 7},
		{Subject: map[string]string{"kind": "__cluster__", "cluster": "prod"}, TotalCost: 73},
	}
}

func groupTotals(gs []ClusterGroup) map[string]float64 {
	m := map[string]float64{}
	for _, g := range gs {
		m[g.Key] = g.TotalCost
	}
	return m
}

func TestGroupClusterRows(t *testing.T) {
	tests := []struct {
		groupBy string
		want    map[string]float64
	}{
		{"namespace", map[string]float64{"payments": 20, "search": 5, "__idle__": 7, "__cluster__": 73}},
		{"controller", map[string]float64{"payments/Deployment/api": 20, "search/Deployment/api": 5, "__idle__": 7, "__cluster__": 73}},
		{"pod", map[string]float64{"payments/api-1": 10, "payments/api-2": 10, "search/api-1": 5, "__idle__": 7, "__cluster__": 73}},
		{"node", map[string]float64{"n1": 22, "n2": 10, "__cluster__": 73}},
		{"label:team", map[string]float64{"pay": 20, GroupKeyNone: 5, "__idle__": 7, "__cluster__": 73}},
	}
	for _, tt := range tests {
		t.Run(tt.groupBy, func(t *testing.T) {
			gs, err := GroupClusterRows(clusterRows(), tt.groupBy)
			require.NoError(t, err)
			assert.Equal(t, tt.want, groupTotals(gs))
			var sum float64
			for _, g := range gs {
				sum += g.TotalCost
			}
			assert.InDelta(t, 115, sum, 1e-9, "grouping never loses cost")
		})
	}
}

func TestGroupClusterRows_OrderAndNotes(t *testing.T) {
	gs, err := GroupClusterRows(clusterRows(), "namespace")
	require.NoError(t, err)
	assert.Equal(t, "__cluster__", gs[0].Key, "highest total first")
	for _, g := range gs {
		if g.Key == "payments" {
			assert.Equal(t, []string{"spot node priced on-demand"}, g.Notes)
			assert.Equal(t, 2, g.Rows)
		}
	}
}

func TestValidateClusterGroupBy(t *testing.T) {
	for _, ok := range []string{"namespace", "controller", "pod", "node", "label:team", "label:app.kubernetes.io/name"} {
		assert.NoError(t, ValidateClusterGroupBy(ok), ok)
	}
	for _, bad := range []string{"", "daily", "label:", "labels:team", "Namespace"} {
		err := ValidateClusterGroupBy(bad)
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), "namespace, controller, pod, node, label:<key>")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/engine/ -run 'GroupClusterRows|ValidateClusterGroupBy'`
Expected: FAIL — `undefined: GroupClusterRows`.

- [ ] **Step 3: Implement** (`internal/engine/cluster_group.go`)

```go
package engine

import (
	"fmt"
	"sort"
	"strings"
)

// GroupKeyNone groups rows that lack the requested dimension.
const GroupKeyNone = "<none>"

const labelGroupPrefix = "label:"

// ClusterGroup is the aggregate of cluster rows sharing a group key.
type ClusterGroup struct {
	Key       string   `json:"key"`
	CPUCost   float64  `json:"cpu_cost"`
	MemCost   float64  `json:"mem_cost"`
	TotalCost float64  `json:"total_cost"`
	Rows      int      `json:"rows"`
	Notes     []string `json:"notes,omitempty"`
}

// ValidateClusterGroupBy rejects unknown grouping dimensions.
func ValidateClusterGroupBy(groupBy string) error {
	switch groupBy {
	case "namespace", "controller", "pod", "node":
		return nil
	}
	if strings.HasPrefix(groupBy, labelGroupPrefix) && len(groupBy) > len(labelGroupPrefix) {
		return nil
	}
	return fmt.Errorf("invalid --group-by %q: use one of namespace, controller, pod, node, label:<key>", groupBy)
}

// GroupClusterRows aggregates rows by the requested dimension.
func GroupClusterRows(rows []ClusterRow, groupBy string) ([]ClusterGroup, error) {
	if err := ValidateClusterGroupBy(groupBy); err != nil {
		return nil, err
	}
	groups := map[string]*ClusterGroup{}
	notes := map[string]map[string]bool{}
	for _, r := range rows {
		key := clusterGroupKey(r.Subject, groupBy)
		g := groups[key]
		if g == nil {
			g = &ClusterGroup{Key: key}
			groups[key] = g
			notes[key] = map[string]bool{}
		}
		g.CPUCost += r.CPUCost
		g.MemCost += r.MemCost
		g.TotalCost += r.TotalCost
		g.Rows++
		if r.Note != "" {
			notes[key][r.Note] = true
		}
	}
	out := make([]ClusterGroup, 0, len(groups))
	for key, g := range groups {
		for n := range notes[key] {
			g.Notes = append(g.Notes, n)
		}
		sort.Strings(g.Notes)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalCost != out[j].TotalCost {
			return out[i].TotalCost > out[j].TotalCost
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func clusterGroupKey(s map[string]string, groupBy string) string {
	switch s["kind"] {
	case "__cluster__":
		return "__cluster__"
	case "__idle__":
		if groupBy == "node" {
			return orNone(s["node"])
		}
		return "__idle__"
	}
	switch groupBy {
	case "namespace":
		return orNone(s["namespace"])
	case "controller":
		return orNone(s["namespace"]) + "/" + orNone(s["controller_kind"]) + "/" + orNone(s["controller"])
	case "pod":
		return orNone(s["namespace"]) + "/" + orNone(s["pod"])
	case "node":
		return orNone(s["node"])
	default:
		return orNone(s["label."+strings.TrimPrefix(groupBy, labelGroupPrefix)])
	}
}

func orNone(v string) string {
	if v == "" {
		return GroupKeyNone
	}
	return v
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/engine/...`
Expected: PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/engine/cluster_group.go internal/engine/cluster_group_test.go
```

Proposed message: `feat(engine): group cluster allocation rows by namespace, controller, pod, node, or label`

---

### Task 4: Allocation policy resolution

**Files:**

- Create: `internal/config/allocation_policy.go`
- Test: `internal/config/allocation_policy_test.go`

**Interfaces:**

- Consumes: `ResolveConfigDir() string` (`config.go:206`), `GetResolvedProjectDir() string` / `SetResolvedProjectDir(string)` (`project.go`), `ax.ParseConfig(ctx, io.Reader, any) error` (as used in `config.go:516-534`).
- Produces:

  ```go
  const AllocationPolicyFile = "allocation.hujson"
  type AllocationPolicy struct {
      JSON   []byte // standard JSON; nil = plugin defaults
      Source string // path, or "" for built-in defaults
  }
  func ResolveAllocationPolicy(ctx context.Context, flagPath string) (AllocationPolicy, error)
  ```

- [ ] **Step 1: Write failing tests**

```go
package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o750))
	p := filepath.Join(dir, AllocationPolicyFile)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func isolatePolicyDirs(t *testing.T) (home, project string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	project = filepath.Join(t.TempDir(), ".finfocus")
	SetResolvedProjectDir("")
	t.Cleanup(func() { SetResolvedProjectDir("") })
	return home, project
}

func TestResolveAllocationPolicy_Precedence(t *testing.T) {
	home, project := isolatePolicyDirs(t)
	ctx := context.Background()

	got, err := ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Nil(t, got.JSON, "no file → plugin defaults")
	assert.Empty(t, got.Source)

	globalPath := writePolicy(t, home, `{ "version": 1, /* global */ }`)
	got, err = ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, globalPath, got.Source)
	assert.JSONEq(t, `{"version":1}`, string(got.JSON), "hujson comments and trailing commas standardized")

	projectPath := writePolicy(t, project, `{"idle": "separate"}`)
	SetResolvedProjectDir(project)
	got, err = ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, projectPath, got.Source)
	assert.JSONEq(t, `{"idle":"separate"}`, string(got.JSON), "project replaces global, no merge")

	flagPath := writePolicy(t, t.TempDir(), `{"version": 1}`)
	got, err = ResolveAllocationPolicy(ctx, flagPath)
	require.NoError(t, err)
	assert.Equal(t, flagPath, got.Source)
}

func TestResolveAllocationPolicy_Errors(t *testing.T) {
	home, _ := isolatePolicyDirs(t)
	ctx := context.Background()

	_, err := ResolveAllocationPolicy(ctx, filepath.Join(t.TempDir(), "missing.hujson"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.hujson")

	broken := writePolicy(t, home, `{"version": }`)
	_, err = ResolveAllocationPolicy(ctx, "")
	require.Error(t, err, "a discovered broken file never falls back to defaults")
	assert.Contains(t, err.Error(), broken)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run ResolveAllocationPolicy`
Expected: FAIL — `undefined: ResolveAllocationPolicy`.

- [ ] **Step 3: Implement** (`internal/config/allocation_policy.go`)

```go
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ax "github.com/rshade/ax-go"
)

// AllocationPolicyFile is the allocation policy file name in a project or global config dir.
const AllocationPolicyFile = "allocation.hujson"

// AllocationPolicy is a resolved policy document, standardized to JSON.
type AllocationPolicy struct {
	JSON   []byte
	Source string
}

// ResolveAllocationPolicy finds the allocation policy: flagPath, then the project
// config dir, then the global config dir. The first file found wins; files are
// never merged. No file yields an empty policy (plugin defaults).
func ResolveAllocationPolicy(ctx context.Context, flagPath string) (AllocationPolicy, error) {
	if flagPath != "" {
		return readAllocationPolicy(ctx, flagPath)
	}
	var candidates []string
	if dir := GetResolvedProjectDir(); dir != "" {
		candidates = append(candidates, filepath.Join(dir, AllocationPolicyFile))
	}
	candidates = append(candidates, filepath.Join(ResolveConfigDir(), AllocationPolicyFile))
	for _, path := range candidates {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		return readAllocationPolicy(ctx, path)
	}
	return AllocationPolicy{}, nil
}

func readAllocationPolicy(ctx context.Context, path string) (AllocationPolicy, error) {
	f, err := os.Open(path) //nolint:gosec // path is user-selected policy file
	if err != nil {
		return AllocationPolicy{}, fmt.Errorf("open allocation policy %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	var data json.RawMessage
	if err := ax.ParseConfig(ctx, f, &data); err != nil {
		return AllocationPolicy{}, fmt.Errorf("parse allocation policy %s: %w", path, err)
	}
	return AllocationPolicy{JSON: data, Source: path}, nil
}
```

Check the ax import alias against `internal/config/config.go`'s existing import and match it exactly. If `ax.ParseConfig` accepts an empty/comment-only document as `null`, that is fine (the plugin treats empty/`null`? — no: the plugin's `Decode` only short-circuits on whitespace). Guard it: after parsing, if `string(bytes.TrimSpace(data)) == "null"`, set `data = nil`, and add a test case `writePolicy(t, home, "// nothing yet\n")` expecting `JSON == nil` with `Source` set.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/config/allocation_policy.go internal/config/allocation_policy_test.go
```

Proposed message: `feat(config): resolve allocation.hujson from flag, project, or global dir`

---

### Task 5: `cost cluster` command and rendering

**Files:**

- Create: `internal/cli/cost_cluster.go`, `internal/cli/cost_cluster_render.go`
- Modify: `internal/cli/root.go:291` (register in `newCostCmd`)
- Test: `internal/cli/cost_cluster_test.go`, `internal/cli/cost_cluster_render_test.go`

**Interfaces:**

- Consumes: Tasks 1-4; `openPlugins(ctx, adapter, audit)` and `newEngineWithCache(ctx, cmd, clients, loader, cfg)` (`common_execution.go`); `resolveOutputFormat`, `writeJSON` (PR #1510); `isValidOutputFormat`, `outputFormat*` constants; `config.New()`, `spec.NewLoader`.
- Produces:

  ```go
  func NewCostClusterCmd() *cobra.Command
  func selectCapablePlugin(clients []*pluginhost.Client, capability, explicit string) (*pluginhost.Client, error)
  type clusterOutput struct { … }  // JSON shape below
  func renderClusterResult(cmd *cobra.Command, format string, out clusterOutput) error
  ```

JSON shape (stable contract for agents and the E2E test):

```go
type clusterPolicyOutput struct {
	Source    string          `json:"source"` // path or "built-in defaults"
	Digest    string          `json:"digest"`
	Effective json.RawMessage `json:"effective"`
}

type clusterOutput struct {
	Mode            string                 `json:"mode"`   // "run-rate"
	Period          string                 `json:"period"` // "monthly"
	Currency        string                 `json:"currency"`
	GroupBy         string                 `json:"group_by"`
	Total           float64                `json:"total"`
	Idle            *float64               `json:"idle,omitempty"` // nil when namespace-scoped
	NamespaceScoped bool                   `json:"namespace_scoped"`
	Incomplete      bool                   `json:"incomplete"`
	Groups          []engine.ClusterGroup  `json:"groups"`
	Priced          []engine.PricedSummary `json:"priced"`
	Policy          clusterPolicyOutput    `json:"policy"`
	Warnings        []string               `json:"warnings,omitempty"`
}
```

NDJSON: first line `{"type":"summary", …clusterOutput without groups…}`, then one `{"type":"group", …ClusterGroup}` per group (mirrors `cost recommendations`). Table: tabwriter columns `GROUP  CPU  MEMORY  TOTAL  NOTES` with `%.2f` money, then a footer:

```text
Mode:    run-rate (monthly, 730 h)
Total:   $1,234.56 USD
Idle:    $345.67 (28.0%)            ← or "Idle:    omitted (--namespace scoped)"
Policy:  .finfocus/allocation.hujson · 3f2a9c1b4d5e   ← or "built-in defaults · …"
Incomplete: 2 resources could not be priced (n7: EC2 instance type "x9" not found …)   ← only when incomplete
```

Warnings go to stderr (`cmd.PrintErrln`), not stdout.

- [ ] **Step 1: Write failing tests**

`internal/cli/cost_cluster_test.go` (package `cli`, internal test so `selectCapablePlugin` is reachable):

```go
package cli

import (
	"context"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func capClient(name string, caps ...string) *pluginhost.Client {
	return &pluginhost.Client{Name: name, Metadata: &proto.PluginMetadata{Capabilities: caps}}
}

func TestSelectCapablePlugin(t *testing.T) {
	k8s := capClient("kubernetes", pluginhost.CapabilityUsageStats, pluginhost.CapabilityAllocation)
	prom := capClient("prometheus", pluginhost.CapabilityUsageStats)
	aws := capClient("aws-public", "projected_costs")

	got, err := selectCapablePlugin([]*pluginhost.Client{aws, k8s}, pluginhost.CapabilityUsageStats, "")
	require.NoError(t, err)
	assert.Equal(t, "kubernetes", got.Name)

	_, err = selectCapablePlugin([]*pluginhost.Client{aws}, pluginhost.CapabilityUsageStats, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finfocus plugin install kubernetes")

	_, err = selectCapablePlugin([]*pluginhost.Client{k8s, prom}, pluginhost.CapabilityUsageStats, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes, prometheus")
	assert.Contains(t, err.Error(), "--usage-source")

	got, err = selectCapablePlugin([]*pluginhost.Client{k8s, prom}, pluginhost.CapabilityUsageStats, "prometheus")
	require.NoError(t, err)
	assert.Equal(t, "prometheus", got.Name)

	_, err = selectCapablePlugin([]*pluginhost.Client{k8s}, pluginhost.CapabilityUsageStats, "aws-public")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"aws-public" is not installed or lacks usage_stats`)
}

func TestCostCluster_ValidatesFlagsBeforeLoadingPlugins(t *testing.T) {
	isolateConfig(t)
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"cost", "cluster", "--output", "xml"}, "unsupported output format: xml"},
		{[]string{"cost", "cluster", "--group-by", "daily"}, "invalid --group-by"},
		{[]string{"cost", "cluster", "--selector", "novalue"}, "invalid --selector"},
		{[]string{"cost", "cluster", "--policy", "/nonexistent/p.hujson"}, "/nonexistent/p.hujson"},
	}
	for _, tt := range tests {
		t.Run(tt.wantErr, func(t *testing.T) {
			res := axtest.Run(context.Background(), t, NewRootCmd("test"), tt.args)
			require.NotEqual(t, 0, res.ExitCode)
			assert.Contains(t, string(res.Stderr), tt.wantErr)
		})
	}
}

func TestCostCluster_NoPluginsInstalled(t *testing.T) {
	isolateConfig(t)
	res := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"cost", "cluster"})
	require.Equal(t, 1, res.ExitCode)
	assert.Contains(t, string(res.Stderr), "usage_stats")
	assert.Contains(t, string(res.Stderr), "finfocus plugin install kubernetes")
}
```

`--show-policy` with only an allocator must skip the usage source: cover it in `selectCapablePlugin`-level logic by structuring `runCostCluster` so the usage plugin is resolved only when `!params.showPolicy`, and assert that in the E2E (Task 9) via `--show-policy` against a context that does not exist (`--context does-not-exist --show-policy` must still succeed).

`internal/cli/cost_cluster_render_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func sampleClusterOutput(scoped bool) clusterOutput {
	idle := 7.0
	out := clusterOutput{
		Mode: engine.ModeRunRate, Period: "monthly", Currency: "USD", GroupBy: "namespace",
		Total: 100, Idle: &idle,
		Groups: []engine.ClusterGroup{
			{Key: "payments", CPUCost: 40, MemCost: 20, TotalCost: 60, Rows: 2, Notes: []string{"spot node priced on-demand"}},
			{Key: "__idle__", TotalCost: 7, Rows: 1},
		},
		Policy: clusterPolicyOutput{Source: "built-in defaults", Digest: strings.Repeat("a", 64),
			Effective: json.RawMessage(`{"version":1}`)},
	}
	if scoped {
		out.Idle, out.NamespaceScoped = nil, true
	}
	return out
}

func renderTo(t *testing.T, format string, out clusterOutput) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	require.NoError(t, renderClusterResult(cmd, format, out))
	return buf.String()
}

func TestRenderCluster_Table(t *testing.T) {
	s := renderTo(t, outputFormatTable, sampleClusterOutput(false))
	assert.Contains(t, s, "GROUP")
	assert.Contains(t, s, "payments")
	assert.Contains(t, s, "60.00")
	assert.Contains(t, s, "spot node priced on-demand")
	assert.Contains(t, s, "run-rate (monthly, 730 h)")
	assert.Contains(t, s, "Idle:")
	assert.Contains(t, s, "7.0%")
	assert.Contains(t, s, "built-in defaults · aaaaaaaaaaaa")

	scoped := renderTo(t, outputFormatTable, sampleClusterOutput(true))
	assert.Contains(t, scoped, "omitted (--namespace scoped)")
}

func TestRenderCluster_JSON(t *testing.T) {
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, sampleClusterOutput(false))), &got))
	assert.Equal(t, "run-rate", got["mode"])
	assert.InDelta(t, 100, got["total"], 1e-9)
	assert.Len(t, got["groups"], 2)
	assert.Equal(t, map[string]any{"version": float64(1)}, got["policy"].(map[string]any)["effective"])

	var scoped map[string]any
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, sampleClusterOutput(true))), &scoped))
	_, hasIdle := scoped["idle"]
	assert.False(t, hasIdle)
	assert.Equal(t, true, scoped["namespace_scoped"])
}

func TestRenderCluster_NDJSON(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(renderTo(t, outputFormatNDJSON, sampleClusterOutput(false))), "\n")
	require.Len(t, lines, 3)
	var first, second map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
	assert.Equal(t, "summary", first["type"])
	assert.NotContains(t, first, "groups")
	assert.Equal(t, "group", second["type"])
	assert.Equal(t, "payments", second["key"])
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'SelectCapablePlugin|CostCluster|RenderCluster'`
Expected: FAIL — `undefined: selectCapablePlugin`.

- [ ] **Step 3: Implement `cost_cluster.go`**

```go
package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/spec"
)

type costClusterParams struct {
	context     string
	namespace   string
	selectors   []string
	groupBy     string
	policyPath  string
	showPolicy  bool
	usageSource string
	allocator   string
	output      string
}

// NewCostClusterCmd creates `finfocus cost cluster`.
func NewCostClusterCmd() *cobra.Command {
	var params costClusterParams
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Break down Kubernetes cluster cost by namespace, controller, pod, node, or label",
		Long: `Allocates the monthly cost of a cluster's nodes (and control plane) to the
workloads running on them, using a usage-source plugin for requests and an
allocator plugin for the split. Idle capacity is reported as its own row.`,
		Example: `  finfocus cost cluster
  finfocus cost cluster --context prod --group-by controller
  finfocus cost cluster --namespace payments --output json
  finfocus cost cluster --group-by label:team --policy ./allocation.hujson
  finfocus cost cluster --show-policy`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCostCluster(cmd, params)
		},
	}
	f := cmd.Flags()
	f.StringVar(&params.context, "context", "", "Kubeconfig context (default: current context)")
	f.StringVar(&params.namespace, "namespace", "", "Only allocate workloads in this namespace (omits idle and cluster rows)")
	f.StringArrayVar(&params.selectors, "selector", nil, "Pod label selector key=value (repeatable)")
	f.StringVar(&params.groupBy, "group-by", "namespace", "Group by: namespace, controller, pod, node, label:<key>")
	f.StringVar(&params.policyPath, "policy", "", "Allocation policy file (default: project or global allocation.hujson)")
	f.BoolVar(&params.showPolicy, "show-policy", false, "Print the effective allocation policy and exit")
	f.StringVar(&params.usageSource, "usage-source", "", "Usage-source plugin to use when several are installed")
	f.StringVar(&params.allocator, "allocator", "", "Allocator plugin to use when several are installed")
	f.StringVar(&params.output, "output", config.GetDefaultOutputFormat(), "Output format (table, json, ndjson)")
	return cmd
}

func runCostCluster(cmd *cobra.Command, params costClusterParams) error {
	ctx := cmd.Context()
	format := config.GetOutputFormat(resolveOutputFormat(cmd, "output", params.output))
	if !isValidOutputFormat(engine.OutputFormat(format)) {
		return fmt.Errorf("unsupported output format: %s (supported: table, json, ndjson)", format)
	}
	if err := engine.ValidateClusterGroupBy(params.groupBy); err != nil {
		return err
	}
	selector, err := parseSelectors(params.selectors)
	if err != nil {
		return err
	}
	policy, err := config.ResolveAllocationPolicy(ctx, params.policyPath)
	if err != nil {
		return err
	}

	clients, cleanup, err := openPlugins(ctx, "", nil)
	if err != nil {
		return err
	}
	defer cleanup()

	allocClient, err := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, params.allocator)
	if err != nil {
		return err
	}
	allocator := pbc.NewAllocatorServiceClient(allocClient.Conn)
	policyOut := clusterPolicyOutput{Source: policySourceLabel(policy.Source)}

	if params.showPolicy {
		effective, digest, err := engine.ShowAllocationPolicy(ctx, allocator, policy.JSON)
		if err != nil {
			return err
		}
		policyOut.Digest, policyOut.Effective = digest, effective
		return renderPolicy(cmd, format, policyOut)
	}

	usageClient, err := selectCapablePlugin(clients, pluginhost.CapabilityUsageStats, params.usageSource)
	if err != nil {
		return err
	}
	cfg := config.New()
	eng, _, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(cfg.SpecDir), cfg)
	defer cacheCleanup()

	res, err := engine.RunClusterAllocation(ctx,
		pbc.NewUsageSourceServiceClient(usageClient.Conn), allocator, eng,
		engine.ClusterRequest{Scope: params.context, Namespace: params.namespace, Selector: selector, PolicyJSON: policy.JSON})
	if err != nil {
		return err
	}
	groups, err := engine.GroupClusterRows(res.Rows, params.groupBy)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		cmd.PrintErrln("warning: " + w)
	}
	policyOut.Digest, policyOut.Effective = res.PolicyDigest, res.EffectivePolicy
	return renderClusterResult(cmd, format, newClusterOutput(res, groups, params.groupBy, policyOut))
}

// selectCapablePlugin picks the plugin serving capability: the explicit name,
// or the only installed plugin that declares it.
func selectCapablePlugin(clients []*pluginhost.Client, capability, explicit string) (*pluginhost.Client, error) {
	var names []string
	var candidates []*pluginhost.Client
	for _, c := range clients {
		if c.HasCapability(capability) {
			candidates = append(candidates, c)
			names = append(names, c.Name)
		}
	}
	slices.Sort(names)
	flag := map[string]string{
		pluginhost.CapabilityUsageStats: "--usage-source",
		pluginhost.CapabilityAllocation: "--allocator",
	}[capability]
	if explicit != "" {
		for _, c := range candidates {
			if c.Name == explicit {
				return c, nil
			}
		}
		return nil, fmt.Errorf("plugin %q is not installed or lacks %s (available: %s)",
			explicit, capability, strings.Join(names, ", "))
	}
	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("no installed plugin provides %s; install one with: finfocus plugin install kubernetes",
			capability)
	case 1:
		return candidates[0], nil
	default:
		return nil, fmt.Errorf("several plugins provide %s (%s); choose one with %s",
			capability, strings.Join(names, ", "), flag)
	}
}

func parseSelectors(values []string) (map[string]string, error) {
	out := make(map[string]string, len(values))
	for _, v := range values {
		k, val, ok := strings.Cut(v, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --selector %q: want key=value", v)
		}
		out[k] = val
	}
	return out, nil
}

func policySourceLabel(source string) string {
	if source == "" {
		return "built-in defaults"
	}
	return source
}

var _ = context.Background
```

Remove the trailing `var _ = context.Background` if `context` is unused. Before relying on it, confirm with `grep -n "func newEngineWithCache" internal/cli/common_execution.go` that it wires the router the same way `cost projected` does; if it does not, add `.WithRouter(createRouterForEngine(ctx, cfg, clients))` exactly as `cost_estimate.go:350-363` does.

Register in `internal/cli/root.go` `newCostCmd()`:

```go
	cmd.AddCommand(NewCostProjectedCmd(), NewCostActualCmd(), NewCostRecommendationsCmd(), NewCostEstimateCmd(),
		NewCostClusterCmd())
```

- [ ] **Step 4: Implement `cost_cluster_render.go`**

```go
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
)

const digestDisplayLen = 12

// clusterPolicyOutput and clusterOutput: copy the two struct definitions from
// the "JSON shape" block above into this file verbatim.

func newClusterOutput(res *engine.ClusterResult, groups []engine.ClusterGroup, groupBy string,
	policy clusterPolicyOutput) clusterOutput {
	out := clusterOutput{
		Mode: res.Mode, Period: "monthly", Currency: res.Currency, GroupBy: groupBy,
		Total: res.Total, NamespaceScoped: res.NamespaceScoped, Incomplete: res.Incomplete,
		Groups: groups, Priced: res.Priced, Policy: policy, Warnings: res.Warnings,
	}
	if !res.NamespaceScoped {
		idle := res.Idle
		out.Idle = &idle
	}
	return out
}

func renderClusterResult(cmd *cobra.Command, format string, out clusterOutput) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(cmd, out)
	case outputFormatNDJSON:
		enc := json.NewEncoder(cmd.OutOrStdout())
		summary := out
		summary.Groups = nil
		if err := enc.Encode(struct {
			Type string `json:"type"`
			clusterOutput
		}{"summary", summary}); err != nil {
			return err
		}
		for _, g := range out.Groups {
			if err := enc.Encode(struct {
				Type string `json:"type"`
				engine.ClusterGroup
			}{"group", g}); err != nil {
				return err
			}
		}
		return nil
	default:
		return renderClusterTable(cmd, out)
	}
}

func renderClusterTable(cmd *cobra.Command, out clusterOutput) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)
	fmt.Fprintln(w, "GROUP\tCPU\tMEMORY\tTOTAL\tNOTES")
	for _, g := range out.Groups {
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%s\n", g.Key, g.CPUCost, g.MemCost, g.TotalCost,
			strings.Join(g.Notes, "; "))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	cmd.Println()
	cmd.Printf("Mode:    %s (monthly, %d h)\n", out.Mode, engine.HoursPerMonth)
	cmd.Printf("Total:   $%.2f %s\n", out.Total, out.Currency)
	if out.Idle == nil {
		cmd.Println("Idle:    omitted (--namespace scoped)")
	} else {
		pct := 0.0
		if out.Total > 0 {
			pct = *out.Idle / out.Total * 100
		}
		cmd.Printf("Idle:    $%.2f (%.1f%%)\n", *out.Idle, pct)
	}
	cmd.Printf("Policy:  %s · %s\n", out.Policy.Source, shortDigest(out.Policy.Digest))
	if out.Incomplete {
		var missing []string
		for _, p := range out.Priced {
			if !p.Priced {
				missing = append(missing, fmt.Sprintf("%s: %s", p.ID, p.Note))
			}
		}
		cmd.Printf("Incomplete: %d resources could not be priced (%s)\n", len(missing), strings.Join(missing, "; "))
	}
	return nil
}

func renderPolicy(cmd *cobra.Command, format string, p clusterPolicyOutput) error {
	if format != outputFormatTable {
		return writeJSON(cmd, p)
	}
	cmd.Printf("# source: %s\n# digest: %s\n", p.Source, p.Digest)
	var pretty map[string]any
	if err := json.Unmarshal(p.Effective, &pretty); err != nil {
		return fmt.Errorf("decode effective policy: %w", err)
	}
	return writeJSON(cmd, pretty)
}

func shortDigest(d string) string {
	if len(d) > digestDisplayLen {
		return d[:digestDisplayLen]
	}
	return d
}
```

If `tabPadding` is not visible from this file (it is declared in `cost_recommendations.go:566` in the same package), reuse it; do not redeclare. If the engine's `HoursPerMonth` is a float constant, change the `%d` verb to `%.0f`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/cli/ -run 'SelectCapablePlugin|CostCluster|RenderCluster' -v && go test ./internal/cli/...`
Expected: PASS. Also confirm isolation: `HOME=$(mktemp -d) go test ./internal/cli/ -run 'CostCluster'` — PASS.

- [ ] **Step 6: Visual check of the table**

Render `sampleClusterOutput(false)` through `renderClusterResult` in a scratch test run with `-v` (or temporarily `t.Log` the string) and read it: columns aligned, money right of the key, footer lines in the order Mode, Total, Idle, Policy. Remove any temporary logging afterward.

- [ ] **Step 7: Stage and hand off**

```bash
git add internal/cli/cost_cluster.go internal/cli/cost_cluster_render.go internal/cli/cost_cluster_test.go internal/cli/cost_cluster_render_test.go internal/cli/root.go
```

Proposed message: `feat(cli): add cost cluster command for Kubernetes cost allocation`

---

### Task 6: MCP tool list

**Files:**

- Modify: `internal/cli/testdata/mcp/tools.golden` (regenerated)

**Interfaces:**

- Produces: `cost cluster` exposed as an MCP tool (it takes no positional args, so ax-go lists it automatically).

- [ ] **Step 1: Confirm the golden test fails**

Run: `go test ./internal/cli/ -run TestMCPSchemaToolAllowList`
Expected: FAIL — diff shows a new `cost_cluster` tool.

- [ ] **Step 2: Decide expose vs exclude**

Expose it: it is read-only and bounded. (If exposing, nothing else changes. If a future reason arises to exclude it, add it to `mcpExcludedCommands` in `internal/cli/mcp.go` instead.)

- [ ] **Step 3: Regenerate and verify**

```bash
UPDATE_GOLDEN=1 go test ./internal/cli/ -run TestMCPSchemaToolAllowList
go test ./internal/cli/ -run TestMCP
make build
go test ./test/integration/ -run TestMCPServer_ToolListMatchesGoldenForBothEntryPoints
```

Expected: all PASS; `git diff internal/cli/testdata/mcp/tools.golden` shows only the added `cost cluster` tool. (The integration test builds with a version ldflag; follow its existing setup — CLAUDE.md notes the MCP handshake rejects `dev` versions.)

- [ ] **Step 4: Stage and hand off**

```bash
git add internal/cli/testdata/mcp/tools.golden
```

Proposed message: `test(mcp): expose cost cluster as an MCP tool`

---

### Task 7: Integration tests — no plugins, and no pollution of `cost projected`

**Files:**

- Create: `test/integration/cost_cluster_test.go`

**Interfaces:**

- Consumes: `helpers.NewCLIHelper(t)` with `WithEnv` (see existing integration tests for the exact option name: `grep -n "func WithEnv\|func NewCLIHelper" test/integration/helpers/*.go`); the SP2 plugin source at `plugins/kubernetes` (build it in-test).

- [ ] **Step 1: Write the tests**

```go
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "plugins"), 0o750))
	return home
}

func TestCostCluster_NoPluginsInstalled(t *testing.T) {
	home := isolatedHome(t)
	h := helpers.NewCLIHelper(t, helpers.WithEnv("FINFOCUS_HOME", home), helpers.WithEnv("HOME", home))
	_, err := h.Execute("cost", "cluster")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finfocus plugin install kubernetes")
}

// An installed kubernetes plugin declines Supports, so other commands never
// record per-resource "not supported" errors for it.
func TestKubernetesPlugin_DoesNotPolluteCostProjected(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the kubernetes plugin")
	}
	home := isolatedHome(t)
	dir := filepath.Join(home, "plugins", "kubernetes", "v0.1.0")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	build := exec.Command("go", "-C", "../../plugins/kubernetes", "build",
		"-o", filepath.Join(dir, "finfocus-plugin-kubernetes"), "./cmd")
	build.Stderr = os.Stderr
	require.NoError(t, build.Run())

	h := helpers.NewCLIHelper(t, helpers.WithEnv("FINFOCUS_HOME", home), helpers.WithEnv("HOME", home))
	out, err := h.Execute("cost", "projected", "--pulumi-json", "../../examples/plans/aws-simple-plan.json")
	require.NoError(t, err)
	assert.NotContains(t, out, "not supported")
	assert.NotContains(t, out, "kubernetes:")
}
```

Adjust `h.Execute`'s return signature and the `WithEnv` option to the helper's actual API (read `test/integration/helpers` first). If `plugin list` is needed to confirm discovery, add `h.Execute("plugin", "list")` and assert it contains `kubernetes`.

- [ ] **Step 2: Run**

Run: `go test ./test/integration/ -run 'CostCluster|KubernetesPlugin' -v`
Expected: PASS. (Requires SP2 merged; if running before SP2, the second test is expected to fail to build — do not skip it, sequence the task after SP2.)

- [ ] **Step 3: Stage and hand off**

```bash
git add test/integration/cost_cluster_test.go
```

Proposed message: `test(integration): cost cluster errors and kubernetes plugin isolation`

---

### Task 8: Documentation

**Files:**

- Create: `docs/src/content/docs/guides/cluster-costs.md`
- Modify: `CLAUDE.md` (Package-Specific Gotchas: new "Cluster allocation" subsection)
- Modify: `README.md` (feature list line)

- [ ] **Step 1: Write the guide**

Sections: prerequisites (`finfocus plugin install kubernetes`, `aws-public` with `--metadata region=<nodes' region>`); first run; `--group-by` dimensions with example output; reading the footer (run-rate/monthly, idle, policy digest, incomplete); `--namespace` semantics (exact workload rows, idle omitted); allocation policy (file locations and precedence, `--show-policy`, strict validation, link to `plugins/kubernetes/README.md` for fields); RBAC (link `plugins/kubernetes/deploy/clusterrole.yaml`); limitations (no history yet, spot priced on-demand, Fargate `$0`); JSON/NDJSON shapes for automation and MCP. Match the front-matter format of an existing guide (`head -10 docs/src/content/docs/guides/mcp.md`).

- [ ] **Step 2: CLAUDE.md gotchas**

Add under "Package-Specific Gotchas":

```markdown
### Cluster allocation (`internal/engine/cluster*.go`, `plugins/kubernetes/`)

- **Core never interprets Kubernetes**: nodes arrive from `GetStats` as ordinary
  `ResourceDescriptor`s (`sku`/`region` passed as properties) and are priced by
  `GetProjectedCostWithErrors`; grouping is string-map aggregation
- **`$0` price = unpriced**: aws-public returns `$0` (not an error) for unknown
  instance types, so `priceResources` treats `Monthly <= 0` as unpriced
- **Conservation is enforced in core** (`VerifyConservation`, rel 1e-6, delegating to
  `pluginsdk.CheckConservation`) even though the conformance suite also checks it —
  third-party allocators exist, and one shared SDK rule keeps core and plugins in agreement
- **`plugins/kubernetes` is a nested module** and must not import core packages
  (`make check-plugin-boundaries`); it declines `Supports` so `cost projected`
  skips it (finfocus-spec ≥ v0.6.2 delivers plugin `Supports` answers to hosts;
  earlier SDKs errored and the engine failed open)
```

- [ ] **Step 3: Lint**

Run: `make docs-lint` and `markdownlint -c .markdownlint.json CLAUDE.md README.md`
Expected: pass.

- [ ] **Step 4: Stage and hand off**

```bash
git add docs/src/content/docs/guides/cluster-costs.md CLAUDE.md README.md
```

Proposed message: `docs: cluster cost allocation guide`

---

### Task 9: kind E2E (no AWS credentials)

**Files:**

- Create: `test/e2e/kind/setup.sh`, `test/e2e/kind/workloads.yaml`, `test/e2e/cluster_kind_test.go`
- Modify: `Makefile` (`test-e2e-kind`), `.github/workflows/ci.yml` (job `e2e-kind`)

**Interfaces:**

- Consumes: `bin/finfocus`, `make install-kubernetes` (SP2), `finfocus plugin install aws-public --metadata region=us-east-1` (released aws-public with embedded us-east-1 pricing), JSON shape from Task 5 (`groups`, `priced`, `total`, `idle`, `namespace_scoped`).

- [ ] **Step 1: Cluster setup script** (`test/e2e/kind/setup.sh`)

```bash
#!/usr/bin/env bash
# Create a kind cluster whose nodes look like us-east-1 m5.large EC2 instances.
set -euo pipefail
CLUSTER=${KIND_CLUSTER:-finfocus-e2e}
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! kind get clusters | grep -qx "$CLUSTER"; then
  kind create cluster --name "$CLUSTER" --wait 120s
fi
kubectl --context "kind-$CLUSTER" label nodes --all --overwrite \
  node.kubernetes.io/instance-type=m5.large \
  topology.kubernetes.io/region=us-east-1 \
  finfocus.dev/provider=aws
kubectl --context "kind-$CLUSTER" apply -f "$DIR/workloads.yaml"
kubectl --context "kind-$CLUSTER" -n e2e rollout status deployment/web --timeout=120s
kubectl --context "kind-$CLUSTER" -n e2e rollout status statefulset/db --timeout=120s
kubectl --context "kind-$CLUSTER" -n e2e rollout status daemonset/agent --timeout=120s
```

- [ ] **Step 2: Workloads** (`test/e2e/kind/workloads.yaml`)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: e2e
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: e2e}
spec:
  replicas: 2
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web, team: storefront}}
    spec:
      containers:
        - name: web
          image: registry.k8s.io/pause:3.10
          resources: {requests: {cpu: 250m, memory: 256Mi}}
---
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db, namespace: e2e}
spec:
  serviceName: db
  replicas: 1
  selector: {matchLabels: {app: db}}
  template:
    metadata: {labels: {app: db, team: data}}
    spec:
      containers:
        - name: db
          image: registry.k8s.io/pause:3.10
          resources: {requests: {cpu: 500m, memory: 1Gi}}
---
apiVersion: apps/v1
kind: DaemonSet
metadata: {name: agent, namespace: e2e}
spec:
  selector: {matchLabels: {app: agent}}
  template:
    metadata: {labels: {app: agent}}
    spec:
      tolerations: [{operator: Exists}]
      containers:
        - name: agent
          image: registry.k8s.io/pause:3.10
          resources: {requests: {cpu: 50m, memory: 64Mi}}
```

- [ ] **Step 3: The test** (`test/e2e/cluster_kind_test.go`)

Expected Deployment cost is computed from the live node allocatable and the node price the command itself reports (`priced[]`), so the test asserts exact money without hardcoding aws-public's price table; a sanity band checks the m5.large price is plausible.

```go
//go:build e2e_kind

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
)

const kindContext = "kind-finfocus-e2e"

type clusterJSON struct {
	Total           float64  `json:"total"`
	Idle            *float64 `json:"idle"`
	NamespaceScoped bool     `json:"namespace_scoped"`
	Incomplete      bool     `json:"incomplete"`
	Groups          []struct {
		Key       string  `json:"key"`
		TotalCost float64 `json:"total_cost"`
	} `json:"groups"`
	Priced []struct {
		Kind    string  `json:"kind"`
		ID      string  `json:"id"`
		Monthly float64 `json:"monthly"`
		Priced  bool    `json:"priced"`
	} `json:"priced"`
}

func runCluster(t *testing.T, args ...string) (clusterJSON, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, findFinFocusBinary(),
		append([]string{"cost", "cluster", "--context", kindContext, "--output", "json"}, args...)...)
	out, err := cmd.Output()
	var res clusterJSON
	if err == nil {
		require.NoError(t, json.Unmarshal(out, &res), string(out))
	}
	return res, out, err
}

type nodeAlloc struct{ cpu, memGiB float64 }

func kubectlJSON(t *testing.T, v any, args ...string) {
	t.Helper()
	out, err := exec.Command("kubectl", append([]string{"--context", kindContext}, args...)...).Output()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(out, v))
}

func nodeAllocatable(t *testing.T) map[string]nodeAlloc {
	var list struct {
		Items []struct {
			Metadata struct{ Name string } `json:"metadata"`
			Status   struct {
				Allocatable map[string]string `json:"allocatable"`
			} `json:"status"`
		} `json:"items"`
	}
	kubectlJSON(t, &list, "get", "nodes", "-o", "json")
	out := map[string]nodeAlloc{}
	for _, n := range list.Items {
		cpu := resource.MustParse(n.Status.Allocatable["cpu"])
		mem := resource.MustParse(n.Status.Allocatable["memory"])
		out[n.Metadata.Name] = nodeAlloc{float64(cpu.MilliValue()) / 1000, float64(mem.Value()) / (1 << 30)}
	}
	return out
}

func webPodNodes(t *testing.T) []string {
	var list struct {
		Items []struct {
			Spec struct{ NodeName string } `json:"spec"`
		} `json:"items"`
	}
	kubectlJSON(t, &list, "-n", "e2e", "get", "pods", "-l", "app=web", "-o", "json")
	var nodes []string
	for _, p := range list.Items {
		nodes = append(nodes, p.Spec.NodeName)
	}
	return nodes
}

func TestCostCluster_Kind(t *testing.T) {
	res, out, err := runCluster(t, "--group-by", "controller")
	require.NoError(t, err, string(out))
	require.False(t, res.Incomplete, "every kind node is labeled m5.large/us-east-1")

	nodePrice := map[string]float64{}
	var pricedTotal float64
	for _, p := range res.Priced {
		require.True(t, p.Priced, p.ID)
		nodePrice[p.ID] = p.Monthly
		pricedTotal += p.Monthly
		assert.InDelta(t, 70, p.Monthly, 15, "m5.large us-east-1 monthly price is roughly $70")
	}

	var groupSum float64
	groups := map[string]float64{}
	for _, g := range res.Groups {
		groupSum += g.TotalCost
		groups[g.Key] = g.TotalCost
	}
	assert.InDelta(t, pricedTotal, groupSum, pricedTotal*1e-6, "rows add up to the bill")
	assert.InDelta(t, pricedTotal, res.Total, pricedTotal*1e-6)
	require.NotNil(t, res.Idle)
	assert.Greater(t, *res.Idle, 0.0)
	assert.InDelta(t, *res.Idle, groups["__idle__"], 1e-9)

	// Deployment web: 2 × (250m, 256Mi). Default split weights 0.031611/core-h, 0.004237/GiB-h.
	alloc := nodeAllocatable(t)
	var want float64
	for _, node := range webPodNodes(t) {
		a := alloc[node]
		cpuW, memW := a.cpu*0.031611, a.memGiB*0.004237
		price := nodePrice[node]
		want += price*cpuW/(cpuW+memW)*(0.25/a.cpu) + price*memW/(cpuW+memW)*(0.25/a.memGiB)
	}
	assert.InDelta(t, want, groups["e2e/Deployment/web"], want*1e-6)
}

func TestCostCluster_Kind_NamespaceScope(t *testing.T) {
	res, out, err := runCluster(t, "--namespace", "e2e")
	require.NoError(t, err, string(out))
	assert.True(t, res.NamespaceScoped)
	assert.Nil(t, res.Idle)
	for _, g := range res.Groups {
		assert.NotEqual(t, "__idle__", g.Key)
	}
}

func TestCostCluster_Kind_BadPolicyFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "allocation.hujson")
	require.NoError(t, os.WriteFile(p, []byte(`{"idel": "share"}`), 0o600))
	_, _, err := runCluster(t, "--policy", p)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, string(exitErr.Stderr), `unknown field "idel"`)
}

func TestCostCluster_Kind_ShowPolicyNeedsNoCluster(t *testing.T) {
	out, err := exec.Command(findFinFocusBinary(), "cost", "cluster", "--context", "does-not-exist",
		"--show-policy", "--output", "json").Output()
	require.NoError(t, err)
	var p struct {
		Digest    string         `json:"digest"`
		Effective map[string]any `json:"effective"`
	}
	require.NoError(t, json.Unmarshal(out, &p))
	assert.Len(t, p.Digest, 64)
	assert.Equal(t, float64(1), p.Effective["version"])
}
```

`k8s.io/apimachinery` must be added to `test/e2e/go.mod` (`cd test/e2e && go get k8s.io/apimachinery@<version used by plugins/kubernetes> && go mod tidy`). `findFinFocusBinary()` exists in `test/e2e/utils.go:75`.

- [ ] **Step 4: Make target**

```make
.PHONY: test-e2e-kind
test-e2e-kind: build install-kubernetes
	./test/e2e/kind/setup.sh
	./bin/finfocus plugin install aws-public --metadata region=us-east-1 --force
	cd test/e2e && FINFOCUS_BINARY=$(CURDIR)/bin/finfocus go test -tags e2e_kind -run TestCostCluster_Kind -v -timeout 10m ./...
```

- [ ] **Step 5: CI job** (in `ci.yml`, runs on PRs)

```yaml
  e2e-kind:
    name: E2E (kind)
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ env.GO_VERSION }}
      - uses: helm/kind-action@v1
        with:
          install_only: true
      - name: Run kind E2E
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: make test-e2e-kind
```

Check the current major version of `helm/kind-action` before committing (`gh api repos/helm/kind-action/releases/latest -q .tag_name`) and pin to it.

- [ ] **Step 6: Run locally**

Run: `make test-e2e-kind` (requires Docker + kind + kubectl).
Expected: all four tests PASS. Then `kind delete cluster --name finfocus-e2e`.

- [ ] **Step 7: Full verification**

Run: `make test` and `make lint` (allow >5 minutes).
Expected: PASS.

- [ ] **Step 8: Stage and hand off**

```bash
git add test/e2e/kind/ test/e2e/cluster_kind_test.go test/e2e/go.mod test/e2e/go.sum Makefile .github/workflows/ci.yml
```

Proposed message: `test(e2e): kind-based cost cluster E2E with real plugins`
