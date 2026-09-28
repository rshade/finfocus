# SP2 — Kubernetes Plugin Implementation Plan

<!-- markdownlint-configure-file { "MD010": { "code_blocks": false } } -->

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `plugins/kubernetes/`, a nested Go module whose binary serves `UsageSourceService.GetStats` (Kubernetes-API run-rate) and `AllocatorService.Allocate` (policy-driven cost split with idle and control-plane rows).

**Architecture:** Three pure-ish packages behind thin gRPC glue: `policy` (defaults, strict decode, canonical digest), `allocate` (usage + priced resources + policy → rows, conservation guaranteed), `usage` (client-go listing → usage rows + priceable descriptors). `plugin.go` adapts them to the pluginsdk provider interfaces; `cmd/main.go` serves with explicit capabilities `[USAGE_STATS, ALLOCATION]` so hosts never send it price queries.

**Tech Stack:** Go 1.27.1, finfocus-spec pluginsdk (release from SP1), `k8s.io/client-go` + `k8s.io/api` + `k8s.io/apimachinery`, zerolog, testify, client-go `kubernetes/fake`.

**Spec:** `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md` (§3 contracts, §4 "Kubernetes plugin" and "Allocation policy", §5, §6 SP2)

## Global Constraints

- Module path `github.com/rshade/finfocus/plugins/kubernetes`, `go 1.27.1`; `finfocus-spec` and `testify` versions equal to the root `go.mod` (CI `validate` sync check is extended to this module in Task 7).
- The module must not depend on `github.com/rshade/finfocus` (root) — enforced by `make check-plugin-boundaries`.
- Plugin binary name `finfocus-plugin-kubernetes`; plugin name `kubernetes`.
- Capabilities declared explicitly: `PLUGIN_CAPABILITY_USAGE_STATS`, `PLUGIN_CAPABILITY_ALLOCATION` only.
- Units: CPU in cores (`"core"`), memory in GiB (`"GiB"`, 2^30 bytes). Run-rate mode only; historical requests → `InvalidArgument`.
- Policy defaults exactly as the spec: `version 1`, `idle "separate"`, `system_workloads "separate"`, `node_split {method "unit-price-ratio"}`, `charge "max-request-usage"`, `control_plane "separate"`, `spot_nodes "on-demand-with-note"`. Unit-price weights: `cpu_core_hour 0.031611`, `mem_gib_hour 0.004237`.
- Conservation: Σ row `total_cost` = Σ `priced=true` cost, relative epsilon 1e-6.
- SDK helpers from finfocus-spec 052 (FR-011, FR-024, FR-031, FR-032) are the single source of these
  rules — do not re-implement them: `pluginsdk.DecodePolicy(data, &target)` for strict policy decoding
  (reports JSON paths such as `node_split.cpu`; Go's decoder alone omits them),
  `pluginsdk.ValidateAllocateRequest(req)` for request rules (nonzero cost on unpriced resources,
  duplicate `(kind, id)`, mixed currencies), and `pluginsdk.ResolveCurrency(priced)` for the
  resolved currency (empty takes the others' single currency; all empty → `USD`). All request and
  policy rejections are `InvalidArgument`. The SDK's request and currency errors already carry
  `InvalidArgument` (they implement `GRPCStatus()`), so return them unchanged. `policy.Decode` errors
  still need `status.Error(codes.InvalidArgument, …)`, because `Policy.Validate`'s version and value
  errors are plain.
- Allocator coverage target 95%; package coverage ≥ 80%.
- Logging to stderr only; stdout is the pluginsdk handshake.
- Never `git commit`; stage and hand off. `make lint` + `make test` before completion.

## Review Focus

- **Zero or missing allocatable** (cordoned/NotReady node reporting 0, or a priced node with no capacity rows) must send the whole node cost to `__idle__` — never NaN, never lost cost (Task 2 test `zero allocatable`).
- **Requests above allocatable** (static/mirror pods, future `usage > request`) must scale shares down so idle is never negative (Task 2 test `overcommitted node`).
- **Pods whose node is unknown or unpriced** (Pending with empty `nodeName`, node deleted mid-list, Fargate) — Pending pods are skipped; others get `$0` rows with a note and never become idle (Task 2, Task 4 tests).
- **Same pod name in two namespaces** must stay two rows (aggregation key is `namespace/pod`) (Task 2 test `duplicate pod names`).
- **A policy file that is `{}`, only comments, or omits `version`** decodes to defaults and yields the same digest as no policy (Task 1 test).

---

### Task 1: Module scaffold + `policy` package

**Files:**

- Create: `plugins/kubernetes/go.mod`
- Create: `plugins/kubernetes/policy/policy.go`
- Test: `plugins/kubernetes/policy/policy_test.go`

**Interfaces:**

- Produces:

  ```go
  package policy
  type NodeSplit struct {
      Method      string  `json:"method"`
      CPUCoreHour float64 `json:"cpu_core_hour"`
      MemGiBHour  float64 `json:"mem_gib_hour"`
  }
  type Policy struct {
      Version         int       `json:"version"`
      Idle            string    `json:"idle"`
      SystemWorkloads string    `json:"system_workloads"`
      NodeSplit       NodeSplit `json:"node_split"`
      Charge          string    `json:"charge"`
      ControlPlane    string    `json:"control_plane"`
      SpotNodes       string    `json:"spot_nodes"`
  }
  func Defaults() Policy
  func Decode(data []byte) (Policy, error)          // defaults + pluginsdk.DecodePolicy + Validate
  func (p Policy) Validate() error
  func (p Policy) Canonical() (canonical []byte, digest string, err error)
  ```

- [ ] **Step 1: Create the module**

```bash
mkdir -p plugins/kubernetes/policy
cd plugins/kubernetes
go mod init github.com/rshade/finfocus/plugins/kubernetes
go mod edit -go=1.27.1
SPEC=$(cd ../.. && go list -m -f '{{.Version}}' github.com/rshade/finfocus-spec)
TESTIFY=$(cd ../.. && go list -m -f '{{.Version}}' github.com/stretchr/testify)
go get github.com/rshade/finfocus-spec@"$SPEC" github.com/stretchr/testify@"$TESTIFY"
```

- [ ] **Step 2: Write failing tests** (`plugins/kubernetes/policy/policy_test.go`)

```go
package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode_EmptyInputsYieldDefaults(t *testing.T) {
	_, defDigest, err := Defaults().Canonical()
	require.NoError(t, err)
	for _, in := range []string{"", "  ", "{}", `{"version": 1}`} {
		p, err := Decode([]byte(in))
		require.NoError(t, err, "input %q", in)
		assert.Equal(t, Defaults(), p, "input %q", in)
		_, d, err := p.Canonical()
		require.NoError(t, err)
		assert.Equal(t, defDigest, d, "input %q must digest like defaults", in)
	}
}

func TestDecode_OverridesMergeOntoDefaults(t *testing.T) {
	p, err := Decode([]byte(`{"node_split": {"cpu_core_hour": 0.05}}`))
	require.NoError(t, err)
	assert.InDelta(t, 0.05, p.NodeSplit.CPUCoreHour, 1e-12)
	assert.Equal(t, "unit-price-ratio", p.NodeSplit.Method, "sibling field keeps default")
	assert.InDelta(t, 0.004237, p.NodeSplit.MemGiBHour, 1e-12)
	assert.Equal(t, "separate", p.Idle)
}

func TestDecode_Rejects(t *testing.T) {
	tests := []struct {
		name, in, wantErr string
	}{
		{"unknown top-level field", `{"idel": "share"}`, "idel"},
		{"unknown nested field reports JSON path", `{"node_split": {"cpu": 1}}`, "node_split.cpu"},
		{"unknown version", `{"version": 2}`, "unsupported policy version 2"},
		{"unsupported idle mode", `{"idle": "share"}`, `idle: unsupported value "share"`},
		{"negative weight", `{"node_split": {"mem_gib_hour": -1}}`, "node_split weights must be >= 0"},
		{"trailing data", `{} {}`, "allocation policy"},
		{"not json", `idle: separate`, "allocation policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.in))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCanonical_StableAndDistinct(t *testing.T) {
	a, da, err := Defaults().Canonical()
	require.NoError(t, err)
	b, db, err := Defaults().Canonical()
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.Equal(t, da, db)
	assert.Len(t, da, 64, "hex sha256")

	p := Defaults()
	p.NodeSplit.CPUCoreHour = 0.04
	_, dp, err := p.Canonical()
	require.NoError(t, err)
	assert.NotEqual(t, da, dp)
}
```

- [ ] **Step 3: Run to verify failure**

Run: `cd plugins/kubernetes && go test ./policy/`
Expected: FAIL — `undefined: Defaults`.

- [ ] **Step 4: Implement** (`plugins/kubernetes/policy/policy.go`)

```go
// Package policy defines the kubernetes allocator's policy document: built-in
// defaults, strict decoding of user overrides, and a canonical digest.
//
// Overrides are applied by decoding onto the defaults, so nested objects merge
// field by field. Any future list-valued field is replaced wholesale, not
// appended to.
package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// CurrentVersion is the only policy schema version this plugin understands.
const CurrentVersion = 1

// NodeSplit controls how a node's price is divided between CPU and memory.
type NodeSplit struct {
	Method      string  `json:"method"`
	CPUCoreHour float64 `json:"cpu_core_hour"`
	MemGiBHour  float64 `json:"mem_gib_hour"`
}

// Policy is the effective allocation policy.
type Policy struct {
	Version         int       `json:"version"`
	Idle            string    `json:"idle"`
	SystemWorkloads string    `json:"system_workloads"`
	NodeSplit       NodeSplit `json:"node_split"`
	Charge          string    `json:"charge"`
	ControlPlane    string    `json:"control_plane"`
	SpotNodes       string    `json:"spot_nodes"`
}

// Defaults returns the built-in policy.
func Defaults() Policy {
	return Policy{
		Version:         CurrentVersion,
		Idle:            "separate",
		SystemWorkloads: "separate",
		NodeSplit:       NodeSplit{Method: "unit-price-ratio", CPUCoreHour: 0.031611, MemGiBHour: 0.004237},
		Charge:          "max-request-usage",
		ControlPlane:    "separate",
		SpotNodes:       "on-demand-with-note",
	}
}

// Decode applies a JSON override document to the defaults. Unknown fields
// (reported with their JSON path), unsupported values, and unknown versions
// are errors. Strict decoding is delegated to pluginsdk.DecodePolicy so every
// allocator applies the same rules.
func Decode(data []byte) (Policy, error) {
	p := Defaults()
	if err := pluginsdk.DecodePolicy(data, &p); err != nil {
		return Policy{}, fmt.Errorf("allocation policy: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Policy{}, fmt.Errorf("allocation policy: %w", err)
	}
	return p, nil
}

// Validate checks that every field holds a value this version supports.
func (p Policy) Validate() error {
	if p.Version != CurrentVersion {
		return fmt.Errorf("unsupported policy version %d (supported: %d)", p.Version, CurrentVersion)
	}
	for _, f := range []struct{ name, got, want string }{
		{"idle", p.Idle, "separate"},
		{"system_workloads", p.SystemWorkloads, "separate"},
		{"node_split.method", p.NodeSplit.Method, "unit-price-ratio"},
		{"charge", p.Charge, "max-request-usage"},
		{"control_plane", p.ControlPlane, "separate"},
		{"spot_nodes", p.SpotNodes, "on-demand-with-note"},
	} {
		if f.got != f.want {
			return fmt.Errorf("%s: unsupported value %q (supported: %q)", f.name, f.got, f.want)
		}
	}
	if p.NodeSplit.CPUCoreHour < 0 || p.NodeSplit.MemGiBHour < 0 {
		return errors.New("node_split weights must be >= 0")
	}
	return nil
}

// Canonical returns the policy's canonical JSON and its hex SHA-256 digest.
func (p Policy) Canonical() ([]byte, string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, "", fmt.Errorf("marshal policy: %w", err)
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}
```

- [ ] **Step 5: Run tests**

Run: `cd plugins/kubernetes && go test -cover ./policy/`
Expected: PASS, coverage ≥ 95%.

- [ ] **Step 6: Stage and hand off**

```bash
git add plugins/kubernetes/go.mod plugins/kubernetes/go.sum plugins/kubernetes/policy/
```

Proposed message: `feat(kubernetes): add allocation policy with strict decoding and digest`

---

### Task 2: `allocate` package

**Files:**

- Create: `plugins/kubernetes/allocate/allocate.go`
- Test: `plugins/kubernetes/allocate/allocate_test.go`

**Interfaces:**

- Consumes: `policy.Decode`, `policy.Policy.Canonical` (Task 1); `pbc.AllocateRequest`, `pbc.AllocateResponse`, `pbc.AllocationRow`, `pbc.PricedResource`, `pbc.UsageRow` and pluginsdk subject/kind/metric constants (SP1).
- Produces: `func Allocate(req *pbc.AllocateRequest) (*pbc.AllocateResponse, error)` — errors are gRPC status errors, `InvalidArgument` for every request or policy rejection (per finfocus-spec 052 clarifications).

Rules implemented (from the spec, restated so this task stands alone):

1. Nodes: a priced resource with `tags["kind"]=="node"`, joined to usage rows by `resource.id == subject["node"]`. Capacity comes from usage rows with `subject["kind"]=="node"` (`cpu_allocatable`, `mem_allocatable`).
2. CPU portion = `cost × (cpu × cpu_core_hour) / (cpu × cpu_core_hour + mem × mem_gib_hour)`; memory portion = remainder. Both capacities 0 → CPU portion = cost, memory portion = 0.
3. Workloads: usage rows with `kind=="workload"` aggregated by `namespace/pod`. Charged amount per resource = `max(request, usage)`; share = amount / allocatable (0 when allocatable is 0). If Σshares > 1, divide every share by Σshares.
4. Workload row cost = portion × share. Idle row per node = portion × (1 − Σshare) for each resource, summed; always emitted (possibly 0) with subject `{kind: __idle__, node, cluster}`.
5. Unpriced node (`priced=false`): its workloads and idle row get 0 with note `"node <name> has no price"`. Spot (`tags["capacity_type"]=="spot"`): rows on it get note `"spot node priced on-demand"`.
6. Workloads whose node has no priced resource: 0-cost row with note — `"Fargate pricing not supported yet"` when the node name starts with `fargate-`, else `"node <name> not found in priced resources"`.
7. Priced resource with `kind=="cluster"` → `__cluster__` row with `total_cost = cost`. Any other kind → `__cluster__` row with note `"unallocated priced resource kind <k>"` (keeps conservation).
8. Request rules and currency come from the SDK: `pluginsdk.ValidateAllocateRequest(req)` runs first (nonzero cost on an unpriced resource, duplicate `(kind, id)`, mixed currencies → `InvalidArgument`), then `pluginsdk.ResolveCurrency(req.GetPriced())` gives the resolved currency (an empty currency takes the others' single currency; all empty → `USD`). Every row carries the resolved currency.
9. Output order: nodes by name, workloads by `namespace/pod`, then idle, then cluster rows — deterministic.

- [ ] **Step 1: Write failing tests** (`plugins/kubernetes/allocate/allocate_test.go`)

```go
package allocate

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func nodeRows(node string, cpu, mem float64) []*pbc.UsageRow {
	s := map[string]string{"kind": "node", "node": node, "cluster": "c"}
	return []*pbc.UsageRow{
		{Subject: s, Metric: "cpu_allocatable", Amount: cpu, Unit: "core"},
		{Subject: s, Metric: "mem_allocatable", Amount: mem, Unit: "GiB"},
	}
}

func podRows(ns, pod, node string, cpu, mem float64) []*pbc.UsageRow {
	s := map[string]string{
		"kind": "workload", "namespace": ns, "pod": pod, "node": node, "cluster": "c",
		"controller_kind": "Deployment", "controller": pod + "-deploy",
	}
	return []*pbc.UsageRow{
		{Subject: s, Metric: "cpu_request", Amount: cpu, Unit: "core"},
		{Subject: s, Metric: "mem_request", Amount: mem, Unit: "GiB"},
	}
}

func pricedNode(name string, cost float64, tags map[string]string) *pbc.PricedResource {
	t := map[string]string{"kind": "node"}
	for k, v := range tags {
		t[k] = v
	}
	return &pbc.PricedResource{
		Resource: &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:ec2/instance:Instance",
			Sku: "m5.large", Region: "us-east-1", Id: name, Tags: t},
		Cost: cost, Currency: "USD", Priced: true,
	}
}

func concat(parts ...[]*pbc.UsageRow) []*pbc.UsageRow {
	var out []*pbc.UsageRow
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func sumRows(resp *pbc.AllocateResponse) float64 {
	var s float64
	for _, r := range resp.GetRows() {
		s += r.GetTotalCost()
	}
	return s
}

func rowFor(t *testing.T, resp *pbc.AllocateResponse, kind, key, val string) *pbc.AllocationRow {
	t.Helper()
	for _, r := range resp.GetRows() {
		if r.GetSubject()["kind"] == kind && r.GetSubject()[key] == val {
			return r
		}
	}
	require.FailNowf(t, "row not found", "kind=%s %s=%s", kind, key, val)
	return nil
}

// m5.large: 2 cores, 8 GiB. cpu weight 2*0.031611=0.063222, mem weight 8*0.004237=0.033896.
const m5CPUFraction = 0.063222 / (0.063222 + 0.033896)

func TestAllocate_SingleNodeSplitAndIdle(t *testing.T) {
	req := &pbc.AllocateRequest{
		Usage:  concat(nodeRows("n1", 2, 8), podRows("app", "api-1", "n1", 0.5, 2)),
		Priced: []*pbc.PricedResource{pricedNode("n1", 70.08, nil)},
	}
	resp, err := Allocate(req)
	require.NoError(t, err)

	api := rowFor(t, resp, "workload", "pod", "api-1")
	assert.InDelta(t, 70.08*m5CPUFraction*0.25, api.GetCpuCost(), 1e-9)
	assert.InDelta(t, 70.08*(1-m5CPUFraction)*0.25, api.GetMemCost(), 1e-9)
	assert.InDelta(t, api.GetCpuCost()+api.GetMemCost(), api.GetTotalCost(), 1e-12)

	idle := rowFor(t, resp, "__idle__", "node", "n1")
	assert.InDelta(t, 70.08*0.75, idle.GetTotalCost(), 1e-9)
	assert.InDelta(t, 70.08, sumRows(resp), 70.08*1e-6)
	assert.Equal(t, "USD", api.GetCurrency())
	assert.Len(t, resp.GetPolicyDigest(), 64)
	assert.NotEmpty(t, resp.GetEffectivePolicyJson())
}

func TestAllocate_Conservation(t *testing.T) {
	eks := &pbc.PricedResource{
		Resource: &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:eks/cluster:Cluster",
			Sku: "cluster", Region: "us-east-1", Id: "c", Tags: map[string]string{"kind": "cluster"}},
		Cost: 73, Currency: "USD", Priced: true,
	}
	tests := []struct {
		name string
		req  *pbc.AllocateRequest
		want float64
	}{
		{"empty cluster is all idle", &pbc.AllocateRequest{
			Usage: nodeRows("n1", 2, 8), Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)}}, 70},
		{"fully packed node has zero idle", &pbc.AllocateRequest{
			Usage:  concat(nodeRows("n1", 2, 8), podRows("a", "p", "n1", 2, 8)),
			Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)}}, 70},
		{"overcommitted node scales shares", &pbc.AllocateRequest{
			Usage: concat(nodeRows("n1", 2, 8), podRows("a", "p1", "n1", 2, 8),
				podRows("a", "p2", "n1", 2, 8)),
			Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)}}, 70},
		{"zero allocatable sends all to idle", &pbc.AllocateRequest{
			Usage:  concat(nodeRows("n1", 0, 0), podRows("a", "p", "n1", 1, 1)),
			Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)}}, 70},
		{"priced node without capacity rows", &pbc.AllocateRequest{
			Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)}}, 70},
		{"multi-node plus control plane", &pbc.AllocateRequest{
			Usage: concat(nodeRows("n1", 2, 8), nodeRows("n2", 4, 16),
				podRows("a", "p1", "n1", 1, 4), podRows("b", "p2", "n2", 1, 1)),
			Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil), pricedNode("n2", 140, nil), eks}}, 283},
		{"unpriced node contributes nothing", &pbc.AllocateRequest{
			Usage: concat(nodeRows("n1", 2, 8), podRows("a", "p", "n1", 1, 1)),
			Priced: []*pbc.PricedResource{{Resource: pricedNode("n1", 0, nil).GetResource(),
				Priced: false, Note: "not found"}}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := Allocate(tt.req)
			require.NoError(t, err)
			assert.InDelta(t, tt.want, sumRows(resp), tt.want*1e-6+1e-12)
			for _, r := range resp.GetRows() {
				assert.GreaterOrEqual(t, r.GetTotalCost(), 0.0, "row %v", r.GetSubject())
			}
		})
	}
}

func TestAllocate_Notes(t *testing.T) {
	req := &pbc.AllocateRequest{
		Usage: concat(nodeRows("spot-1", 2, 8), podRows("a", "on-spot", "spot-1", 1, 1),
			podRows("a", "fg", "fargate-ip-10-0-0-1", 0.25, 0.5),
			podRows("a", "ghost", "deleted-node", 0.25, 0.5)),
		Priced: []*pbc.PricedResource{pricedNode("spot-1", 30, map[string]string{"capacity_type": "spot"})},
	}
	resp, err := Allocate(req)
	require.NoError(t, err)
	assert.Equal(t, "spot node priced on-demand", rowFor(t, resp, "workload", "pod", "on-spot").GetNote())
	fg := rowFor(t, resp, "workload", "pod", "fg")
	assert.Equal(t, "Fargate pricing not supported yet", fg.GetNote())
	assert.Zero(t, fg.GetTotalCost())
	assert.Contains(t, rowFor(t, resp, "workload", "pod", "ghost").GetNote(), "deleted-node")
	assert.InDelta(t, 30, sumRows(resp), 30e-6)
}

func TestAllocate_DuplicatePodNamesAcrossNamespaces(t *testing.T) {
	req := &pbc.AllocateRequest{
		Usage: concat(nodeRows("n1", 2, 8), podRows("team-a", "web", "n1", 0.5, 1),
			podRows("team-b", "web", "n1", 0.5, 1)),
		Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)},
	}
	resp, err := Allocate(req)
	require.NoError(t, err)
	var n int
	for _, r := range resp.GetRows() {
		if r.GetSubject()["kind"] == "workload" {
			n++
		}
	}
	assert.Equal(t, 2, n)
}

func TestAllocate_Errors(t *testing.T) {
	_, err := Allocate(&pbc.AllocateRequest{PolicyJson: []byte(`{"idel":"x"}`)})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), `unknown field "idel"`)

	_, err = Allocate(&pbc.AllocateRequest{PolicyJson: []byte(`{"node_split":{"cpu":1}}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node_split.cpu", "unknown nested field is reported with its JSON path")

	eur := pricedNode("n2", 10, nil)
	eur.Currency = "EUR"
	_, err = Allocate(&pbc.AllocateRequest{Priced: []*pbc.PricedResource{pricedNode("n1", 10, nil), eur}})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "mixed currencies")

	_, err = Allocate(&pbc.AllocateRequest{Priced: []*pbc.PricedResource{pricedNode("n1", 10, nil), pricedNode("n1", 5, nil)}})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "duplicate priced node")

	unpriced := pricedNode("n3", 7, nil)
	unpriced.Priced = false
	_, err = Allocate(&pbc.AllocateRequest{Priced: []*pbc.PricedResource{unpriced}})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "unpriced resource with nonzero cost")
}

func TestAllocate_EmptyCurrencyTakesResolvedCurrency(t *testing.T) {
	eur := pricedNode("n1", 10, nil)
	eur.Currency = "EUR"
	blank := pricedNode("n2", 10, nil)
	blank.Currency = ""
	resp, err := Allocate(&pbc.AllocateRequest{
		Usage:  concat(nodeRows("n1", 2, 8), nodeRows("n2", 2, 8)),
		Priced: []*pbc.PricedResource{eur, blank},
	})
	require.NoError(t, err)
	for _, r := range resp.GetRows() {
		assert.Equal(t, "EUR", r.GetCurrency())
	}
}

func TestAllocate_EmptyRequestReturnsEffectivePolicy(t *testing.T) {
	resp, err := Allocate(&pbc.AllocateRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.GetRows())
	assert.True(t, json.Valid(resp.GetEffectivePolicyJson()))
	assert.Contains(t, string(resp.GetEffectivePolicyJson()), `"version":1`)
	assert.Len(t, resp.GetPolicyDigest(), 64)
}

func TestAllocate_DeterministicOrder(t *testing.T) {
	req := &pbc.AllocateRequest{
		Usage: concat(nodeRows("n2", 2, 8), nodeRows("n1", 2, 8),
			podRows("b", "z", "n2", 1, 1), podRows("a", "y", "n1", 1, 1)),
		Priced: []*pbc.PricedResource{pricedNode("n2", 10, nil), pricedNode("n1", 10, nil)},
	}
	first, err := Allocate(req)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, err := Allocate(req)
		require.NoError(t, err)
		require.Len(t, again.GetRows(), len(first.GetRows()))
		for j := range first.GetRows() {
			assert.Equal(t, first.GetRows()[j].GetSubject(), again.GetRows()[j].GetSubject())
		}
	}
}
```

Run `go get google.golang.org/grpc@$(cd ../.. && go list -m -f '{{.Version}}' google.golang.org/grpc)` in `plugins/kubernetes` if the import is not yet resolvable.

- [ ] **Step 2: Run to verify failure**

Run: `cd plugins/kubernetes && go test ./allocate/`
Expected: FAIL — `undefined: Allocate`.

- [ ] **Step 3: Implement** (`plugins/kubernetes/allocate/allocate.go`)

```go
// Package allocate divides priced cluster resources across workloads.
package allocate

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/kubernetes/policy"
)

type workload struct {
	subject        map[string]string
	cpuReq, memReq float64
	cpuUse, memUse float64
}

func (w *workload) cpu() float64 { return max(w.cpuReq, w.cpuUse) }
func (w *workload) mem() float64 { return max(w.memReq, w.memUse) }

type node struct {
	name             string
	cpuAlloc         float64
	memAlloc         float64
	priced           *pbc.PricedResource
	workloads        []*workload
	cluster          string
}

// Allocate splits every priced node across the workloads scheduled on it and
// reports the remainder as idle. Rows always sum to the priced total.
func Allocate(req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
	if err := pluginsdk.ValidateAllocateRequest(req); err != nil {
		return nil, err // already carries codes.InvalidArgument (finfocus-spec 052 research R3)
	}
	pol, err := policy.Decode(req.GetPolicyJson())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	canonical, digest, err := pol.Canonical()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	currency, err := pluginsdk.ResolveCurrency(req.GetPriced())
	if err != nil {
		return nil, err
	}

	nodes := map[string]*node{}
	getNode := func(name string) *node {
		if nodes[name] == nil {
			nodes[name] = &node{name: name}
		}
		return nodes[name]
	}
	var clusterRes []*pbc.PricedResource
	for _, pr := range req.GetPriced() {
		switch pr.GetResource().GetTags()["kind"] {
		case "node":
			getNode(pr.GetResource().GetId()).priced = pr
		default:
			clusterRes = append(clusterRes, pr)
		}
	}

	workloads := map[string]*workload{}
	for _, u := range req.GetUsage() {
		s := u.GetSubject()
		switch s["kind"] {
		case "node":
			n := getNode(s["node"])
			n.cluster = s["cluster"]
			switch u.GetMetric() {
			case "cpu_allocatable":
				n.cpuAlloc = u.GetAmount()
			case "mem_allocatable":
				n.memAlloc = u.GetAmount()
			}
		case "workload":
			key := s["namespace"] + "/" + s["pod"]
			w := workloads[key]
			if w == nil {
				w = &workload{subject: s}
				workloads[key] = w
			}
			switch u.GetMetric() {
			case "cpu_request":
				w.cpuReq = u.GetAmount()
			case "mem_request":
				w.memReq = u.GetAmount()
			case "cpu_usage":
				w.cpuUse = u.GetAmount()
			case "mem_usage":
				w.memUse = u.GetAmount()
			}
		}
	}

	var orphans []*workload
	for _, key := range sortedKeys(workloads) {
		w := workloads[key]
		if n, ok := nodes[w.subject["node"]]; ok && n.priced != nil {
			n.workloads = append(n.workloads, w)
		} else {
			orphans = append(orphans, w)
		}
	}

	resp := &pbc.AllocateResponse{EffectivePolicyJson: canonical, PolicyDigest: digest}
	var idleRows []*pbc.AllocationRow
	for _, name := range sortedKeys(nodes) {
		n := nodes[name]
		if n.priced == nil {
			continue // capacity reported but nothing to split; workloads were orphaned above
		}
		rows, idle := allocateNode(n, pol, currency)
		resp.Rows = append(resp.Rows, rows...)
		idleRows = append(idleRows, idle)
	}
	for _, w := range orphans {
		resp.Rows = append(resp.Rows, zeroRow(w.subject, currency, orphanNote(w.subject["node"])))
	}
	resp.Rows = append(resp.Rows, idleRows...)
	for _, pr := range clusterRes {
		resp.Rows = append(resp.Rows, clusterRow(pr, currency))
	}
	return resp, nil
}

func allocateNode(n *node, pol policy.Policy, currency string) ([]*pbc.AllocationRow, *pbc.AllocationRow) {
	cost := 0.0
	note := ""
	if n.priced.GetPriced() {
		cost = n.priced.GetCost()
	} else {
		note = fmt.Sprintf("node %s has no price", n.name)
	}
	if note == "" && n.priced.GetResource().GetTags()["capacity_type"] == "spot" {
		note = "spot node priced on-demand"
	}

	cpuW := n.cpuAlloc * pol.NodeSplit.CPUCoreHour
	memW := n.memAlloc * pol.NodeSplit.MemGiBHour
	cpuPortion, memPortion := cost, 0.0
	if cpuW+memW > 0 {
		cpuPortion = cost * cpuW / (cpuW + memW)
		memPortion = cost - cpuPortion
	}

	cpuShares := shares(n.workloads, n.cpuAlloc, (*workload).cpu)
	memShares := shares(n.workloads, n.memAlloc, (*workload).mem)

	rows := make([]*pbc.AllocationRow, 0, len(n.workloads))
	var cpuUsed, memUsed float64
	for i, w := range n.workloads {
		c, m := cpuPortion*cpuShares[i], memPortion*memShares[i]
		cpuUsed += c
		memUsed += m
		rows = append(rows, &pbc.AllocationRow{
			Subject: w.subject, CpuCost: c, MemCost: m, TotalCost: c + m,
			Currency: currency, Note: note,
		})
	}
	idleCPU, idleMem := max(cpuPortion-cpuUsed, 0), max(memPortion-memUsed, 0)
	idle := &pbc.AllocationRow{
		Subject:  map[string]string{"kind": "__idle__", "node": n.name, "cluster": n.cluster},
		CpuCost:  idleCPU,
		MemCost:  idleMem,
		TotalCost: idleCPU + idleMem,
		Currency: currency,
		Note:     note,
	}
	return rows, idle
}

// shares returns each workload's fraction of capacity, scaled down so the
// total never exceeds 1.
func shares(ws []*workload, capacity float64, amount func(*workload) float64) []float64 {
	out := make([]float64, len(ws))
	if capacity <= 0 {
		return out
	}
	var total float64
	for i, w := range ws {
		out[i] = amount(w) / capacity
		total += out[i]
	}
	if total > 1 {
		for i := range out {
			out[i] /= total
		}
	}
	return out
}

func orphanNote(nodeName string) string {
	if strings.HasPrefix(nodeName, "fargate-") {
		return "Fargate pricing not supported yet"
	}
	return fmt.Sprintf("node %s not found in priced resources", nodeName)
}

func zeroRow(subject map[string]string, currency, note string) *pbc.AllocationRow {
	return &pbc.AllocationRow{Subject: subject, Currency: currency, Note: note}
}

func clusterRow(pr *pbc.PricedResource, currency string) *pbc.AllocationRow {
	r := pr.GetResource()
	row := &pbc.AllocationRow{
		Subject:   map[string]string{"kind": "__cluster__", "cluster": r.GetId()},
		Currency:  currency,
		TotalCost: 0,
	}
	if pr.GetPriced() {
		row.TotalCost = pr.GetCost()
	}
	if kind := r.GetTags()["kind"]; kind != "cluster" {
		row.Note = fmt.Sprintf("unallocated priced resource kind %q", kind)
	}
	return row
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
```

Note the idle-row floor `max(…, 0)` can only trigger from float rounding because `shares` caps Σ at 1.

- [ ] **Step 4: Run tests with coverage**

Run: `cd plugins/kubernetes && go test -coverprofile=/tmp/alloc.out ./allocate/ && go tool cover -func=/tmp/alloc.out | tail -1`
Expected: PASS; total ≥ 95%. If below, add a table case for the uncovered branch rather than lowering the target.

- [ ] **Step 5: Stage and hand off**

```bash
git add plugins/kubernetes/allocate/ plugins/kubernetes/go.mod plugins/kubernetes/go.sum
```

Proposed message: `feat(kubernetes): add request-based allocator with idle and control-plane rows`

---

### Task 3: `usage` package — requests and owners

**Files:**

- Create: `plugins/kubernetes/usage/requests.go`, `plugins/kubernetes/usage/owners.go`
- Test: `plugins/kubernetes/usage/requests_test.go`, `plugins/kubernetes/usage/owners_test.go`

**Interfaces:**

- Produces:

  ```go
  func EffectiveRequests(spec corev1.PodSpec) (cpuCores, memGiB float64)
  type OwnerIndex struct { /* unexported maps keyed "namespace/name" */ }
  func NewOwnerIndex(rs []appsv1.ReplicaSet, jobs []batchv1.Job) *OwnerIndex
  func (idx *OwnerIndex) Resolve(pod *corev1.Pod) (kind, name string)
  ```

- [ ] **Step 1: Add client-go dependencies**

```bash
cd plugins/kubernetes
go get k8s.io/api@latest k8s.io/apimachinery@latest k8s.io/client-go@latest
```

Confirm the three resolve to the same minor (e.g. all `v0.3x.y`); pin if `go get` picks mismatched versions.

- [ ] **Step 2: Write failing tests**

`requests_test.go`:

```go
package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ctr(cpu, mem string) corev1.Container {
	return corev1.Container{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem),
	}}}
}

func sidecar(cpu, mem string) corev1.Container {
	c := ctr(cpu, mem)
	always := corev1.ContainerRestartPolicyAlways
	c.RestartPolicy = &always
	return c
}

func TestEffectiveRequests(t *testing.T) {
	tests := []struct {
		name         string
		spec         corev1.PodSpec
		cpu, memGiB  float64
	}{
		{"sums app containers", corev1.PodSpec{Containers: []corev1.Container{ctr("250m", "1Gi"), ctr("250m", "1Gi")}}, 0.5, 2},
		{"large init dominates", corev1.PodSpec{
			InitContainers: []corev1.Container{ctr("2", "4Gi")},
			Containers:     []corev1.Container{ctr("500m", "1Gi")}}, 2, 4},
		{"sidecar adds to app sum", corev1.PodSpec{
			InitContainers: []corev1.Container{sidecar("100m", "256Mi")},
			Containers:     []corev1.Container{ctr("500m", "1Gi")}}, 0.6, 1.25},
		{"init after sidecar includes sidecar", corev1.PodSpec{
			InitContainers: []corev1.Container{sidecar("500m", "1Gi"), ctr("1", "1Gi")},
			Containers:     []corev1.Container{ctr("100m", "128Mi")}}, 1.5, 2},
		{"overhead added", corev1.PodSpec{
			Containers: []corev1.Container{ctr("1", "1Gi")},
			Overhead:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: resource.MustParse("512Mi")}},
			1.25, 1.5},
		{"no requests is zero", corev1.PodSpec{Containers: []corev1.Container{{}}}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem := EffectiveRequests(tt.spec)
			assert.InDelta(t, tt.cpu, cpu, 1e-9)
			assert.InDelta(t, tt.memGiB, mem, 1e-9)
		})
	}
}
```

`owners_test.go`:

```go
package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ctrl(kind, name string) []metav1.OwnerReference {
	yes := true
	return []metav1.OwnerReference{{Kind: kind, Name: name, Controller: &yes}}
}

func TestOwnerIndex_Resolve(t *testing.T) {
	rs := []appsv1.ReplicaSet{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "api-7d9", OwnerReferences: ctrl("Deployment", "api")}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "bare-rs"}},
	}
	jobs := []batchv1.Job{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "backup-2891", OwnerReferences: ctrl("CronJob", "backup")}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "one-off"}},
	}
	idx := NewOwnerIndex(rs, jobs)
	pod := func(ns, name string, owners []metav1.OwnerReference) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, OwnerReferences: owners}}
	}
	tests := []struct {
		name           string
		pod            *corev1.Pod
		kind, ctrlName string
	}{
		{"deployment via replicaset", pod("app", "api-7d9-x", ctrl("ReplicaSet", "api-7d9")), "Deployment", "api"},
		{"bare replicaset", pod("app", "b-x", ctrl("ReplicaSet", "bare-rs")), "ReplicaSet", "bare-rs"},
		{"replicaset not listed", pod("app", "c-x", ctrl("ReplicaSet", "gone")), "ReplicaSet", "gone"},
		{"cronjob via job", pod("ops", "backup-2891-x", ctrl("Job", "backup-2891")), "CronJob", "backup"},
		{"bare job", pod("ops", "one-off-x", ctrl("Job", "one-off")), "Job", "one-off"},
		{"statefulset", pod("db", "pg-0", ctrl("StatefulSet", "pg")), "StatefulSet", "pg"},
		{"daemonset", pod("kube-system", "aws-node-x", ctrl("DaemonSet", "aws-node")), "DaemonSet", "aws-node"},
		{"bare pod", pod("dev", "debug", nil), "Pod", "debug"},
		{"same rs name other namespace", pod("other", "api-7d9-x", ctrl("ReplicaSet", "api-7d9")), "ReplicaSet", "api-7d9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, name := idx.Resolve(tt.pod)
			assert.Equal(t, tt.kind, kind)
			assert.Equal(t, tt.ctrlName, name)
		})
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `cd plugins/kubernetes && go test ./usage/`
Expected: FAIL — `undefined: EffectiveRequests`.

- [ ] **Step 4: Implement**

`requests.go`:

```go
// Package usage reads a cluster through the Kubernetes API and reports
// per-workload requests and priceable nodes.
package usage

import (
	corev1 "k8s.io/api/core/v1"
)

const bytesPerGiB = 1 << 30

// EffectiveRequests returns a pod's scheduling requests using the Kubernetes
// rule: max(sum of app containers + all sidecars, the largest init container
// plus the sidecars started before it), plus pod overhead.
func EffectiveRequests(spec corev1.PodSpec) (cpuCores, memGiB float64) {
	cpu := effective(spec, corev1.ResourceCPU)
	mem := effective(spec, corev1.ResourceMemory)
	return cpu, mem / bytesPerGiB
}

func effective(spec corev1.PodSpec, name corev1.ResourceName) float64 {
	var sidecars, initPeak float64
	for _, c := range spec.InitContainers {
		req := quantity(c.Resources.Requests, name)
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			sidecars += req
			initPeak = max(initPeak, sidecars)
			continue
		}
		initPeak = max(initPeak, req+sidecars)
	}
	var app float64
	for _, c := range spec.Containers {
		app += quantity(c.Resources.Requests, name)
	}
	return max(app+sidecars, initPeak) + quantity(spec.Overhead, name)
}

func quantity(list corev1.ResourceList, name corev1.ResourceName) float64 {
	q, ok := list[name]
	if !ok {
		return 0
	}
	if name == corev1.ResourceCPU {
		return float64(q.MilliValue()) / 1000
	}
	return float64(q.Value())
}
```

`owners.go`:

```go
package usage

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OwnerIndex resolves pods to their top-level controller.
type OwnerIndex struct {
	rsOwner  map[string]*metav1.OwnerReference
	jobOwner map[string]*metav1.OwnerReference
}

// NewOwnerIndex indexes the controllers of ReplicaSets and Jobs.
func NewOwnerIndex(rs []appsv1.ReplicaSet, jobs []batchv1.Job) *OwnerIndex {
	idx := &OwnerIndex{
		rsOwner:  make(map[string]*metav1.OwnerReference, len(rs)),
		jobOwner: make(map[string]*metav1.OwnerReference, len(jobs)),
	}
	for i := range rs {
		idx.rsOwner[rs[i].Namespace+"/"+rs[i].Name] = metav1.GetControllerOf(&rs[i])
	}
	for i := range jobs {
		idx.jobOwner[jobs[i].Namespace+"/"+jobs[i].Name] = metav1.GetControllerOf(&jobs[i])
	}
	return idx
}

// Resolve returns the kind and name of the pod's top-level controller:
// ReplicaSet→Deployment and Job→CronJob are followed one level; a pod with no
// controller is reported as its own "Pod".
func (idx *OwnerIndex) Resolve(pod *corev1.Pod) (kind, name string) {
	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return "Pod", pod.Name
	}
	key := pod.Namespace + "/" + ref.Name
	var parent *metav1.OwnerReference
	switch ref.Kind {
	case "ReplicaSet":
		parent = idx.rsOwner[key]
	case "Job":
		parent = idx.jobOwner[key]
	}
	if parent != nil {
		return parent.Kind, parent.Name
	}
	return ref.Kind, ref.Name
}
```

- [ ] **Step 5: Run tests**

Run: `cd plugins/kubernetes && go test -cover ./usage/`
Expected: PASS.

- [ ] **Step 6: Stage and hand off**

```bash
git add plugins/kubernetes/usage/ plugins/kubernetes/go.mod plugins/kubernetes/go.sum
```

Proposed message: `feat(kubernetes): effective pod requests and controller resolution`

---

### Task 4: `usage` package — collector

**Files:**

- Create: `plugins/kubernetes/usage/collector.go`, `plugins/kubernetes/usage/nodes.go`
- Test: `plugins/kubernetes/usage/collector_test.go`, `plugins/kubernetes/usage/nodes_test.go`

**Interfaces:**

- Consumes: `EffectiveRequests`, `NewOwnerIndex`, `(*OwnerIndex).Resolve` (Task 3).
- Produces:

  ```go
  type Options struct {
      Cluster       string // cluster subject value (kube context name)
      Namespace     string // "" = all namespaces
      LabelSelector string // pod label selector
      APIServerHost string // used to detect EKS; "" skips control-plane detection
  }
  func Collect(ctx context.Context, cs kubernetes.Interface, opts Options) (*pbc.GetStatsResponse, error)
  func NodeDescriptor(n *corev1.Node) (*pbc.ResourceDescriptor, bool)
  func ControlPlaneDescriptor(host, cluster string) (*pbc.ResourceDescriptor, bool)
  const listPageSize = 500
  ```

Node descriptor rules: provider = label `finfocus.dev/provider` if set, else from `spec.providerID` scheme (`aws`→`aws`, `gce`→`gcp`, `azure`→`azure`); none → not priceable (warning). `sku` = label `node.kubernetes.io/instance-type` (fallback `beta.kubernetes.io/instance-type`); `region` = `topology.kubernetes.io/region` (fallback `failure-domain.beta.kubernetes.io/region`); missing sku or region → not priceable. `resource_type` = `aws:ec2/instance:Instance` for aws, else `<provider>:compute/instance`. Tags: `kind=node`, `provider_id`, `capacity_type` (`spot` when `eks.amazonaws.com/capacityType=SPOT` or `karpenter.sh/capacity-type=spot`, else `on-demand`). Nodes labeled `eks.amazonaws.com/compute-type=fargate` emit neither capacity rows nor priceable.

- [ ] **Step 1: Write failing tests**

`nodes_test.go`:

```go
package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mkNode(name, providerID string, labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.NodeSpec{ProviderID: providerID}}
}

func TestNodeDescriptor(t *testing.T) {
	std := map[string]string{
		"node.kubernetes.io/instance-type": "m5.large",
		"topology.kubernetes.io/region":    "us-east-1",
	}
	d, ok := NodeDescriptor(mkNode("n1", "aws:///us-east-1a/i-0abc", std))
	require.True(t, ok)
	assert.Equal(t, "aws", d.GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", d.GetResourceType())
	assert.Equal(t, "m5.large", d.GetSku())
	assert.Equal(t, "us-east-1", d.GetRegion())
	assert.Equal(t, "n1", d.GetId())
	assert.Equal(t, "node", d.GetTags()["kind"])
	assert.Equal(t, "on-demand", d.GetTags()["capacity_type"])
	assert.Equal(t, "aws:///us-east-1a/i-0abc", d.GetTags()["provider_id"])

	spot := map[string]string{"karpenter.sh/capacity-type": "spot"}
	for k, v := range std {
		spot[k] = v
	}
	d, ok = NodeDescriptor(mkNode("n2", "aws:///us-east-1a/i-0def", spot))
	require.True(t, ok)
	assert.Equal(t, "spot", d.GetTags()["capacity_type"])

	kind := map[string]string{"finfocus.dev/provider": "aws"}
	for k, v := range std {
		kind[k] = v
	}
	_, ok = NodeDescriptor(mkNode("kind-worker", "kind://docker/kind/kind-worker", kind))
	assert.True(t, ok, "provider label overrides unknown providerID scheme")

	_, ok = NodeDescriptor(mkNode("n3", "kind://docker/x", std))
	assert.False(t, ok, "unknown provider without label is not priceable")
	_, ok = NodeDescriptor(mkNode("n4", "aws:///us-east-1a/i-1", map[string]string{"topology.kubernetes.io/region": "us-east-1"}))
	assert.False(t, ok, "missing instance type is not priceable")
}

func TestControlPlaneDescriptor(t *testing.T) {
	d, ok := ControlPlaneDescriptor("https://ABCDEF.gr7.us-west-2.eks.amazonaws.com", "prod")
	require.True(t, ok)
	assert.Equal(t, "aws:eks/cluster:Cluster", d.GetResourceType())
	assert.Equal(t, "cluster", d.GetSku())
	assert.Equal(t, "us-west-2", d.GetRegion())
	assert.Equal(t, "prod", d.GetId())
	assert.Equal(t, "cluster", d.GetTags()["kind"])

	for _, host := range []string{"", "https://127.0.0.1:6443", "https://kind-control-plane:6443"} {
		_, ok := ControlPlaneDescriptor(host, "x")
		assert.False(t, ok, host)
	}
}
```

`collector_test.go`:

```go
package usage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func readyNode(name string, cpu, mem string, labels map[string]string) *corev1.Node {
	n := mkNode(name, "aws:///us-east-1a/i-"+name, labels)
	n.Status.Allocatable = corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem),
	}
	return n
}

func runningPod(ns, name, node string, phase corev1.PodPhase, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: labels},
		Spec:       corev1.PodSpec{NodeName: node, Containers: []corev1.Container{ctr("500m", "1Gi")}},
		Status:     corev1.PodStatus{Phase: phase},
	}
}

var awsLabels = map[string]string{
	"node.kubernetes.io/instance-type": "m5.large",
	"topology.kubernetes.io/region":    "us-east-1",
}

func rowsFor(resp *pbc.GetStatsResponse, key, val string) []*pbc.UsageRow {
	var out []*pbc.UsageRow
	for _, r := range resp.GetRows() {
		if r.GetSubject()[key] == val {
			out = append(out, r)
		}
	}
	return out
}

func TestCollect_RunRate(t *testing.T) {
	fargate := readyNode("fargate-ip-1", "2", "4Gi", map[string]string{"eks.amazonaws.com/compute-type": "fargate"})
	cs := fake.NewSimpleClientset(
		readyNode("n1", "2", "8Gi", awsLabels), fargate,
		runningPod("app", "api-1", "n1", corev1.PodRunning, map[string]string{"team": "payments"}),
		runningPod("app", "done", "n1", corev1.PodSucceeded, nil),
		runningPod("app", "pending", "", corev1.PodPending, nil),
		runningPod("app", "fg", "fargate-ip-1", corev1.PodRunning, nil),
	)
	resp, err := Collect(context.Background(), cs, Options{Cluster: "prod"})
	require.NoError(t, err)
	assert.Equal(t, pbc.StatsMode_STATS_MODE_RUN_RATE, resp.GetMode())

	api := rowsFor(resp, "pod", "api-1")
	require.Len(t, api, 2)
	s := api[0].GetSubject()
	assert.Equal(t, "workload", s["kind"])
	assert.Equal(t, "prod", s["cluster"])
	assert.Equal(t, "Pod", s["controller_kind"])
	assert.Equal(t, "payments", s["label.team"])

	assert.Empty(t, rowsFor(resp, "pod", "done"), "succeeded pods are skipped")
	assert.Empty(t, rowsFor(resp, "pod", "pending"), "unscheduled pods are skipped")
	assert.Len(t, rowsFor(resp, "pod", "fg"), 2, "fargate pods still reported")

	require.Len(t, resp.GetPriceable(), 1, "fargate node is not priceable")
	assert.Equal(t, "n1", resp.GetPriceable()[0].GetId())
	for _, r := range rowsFor(resp, "node", "fargate-ip-1") {
		assert.NotEqual(t, "node", r.GetSubject()["kind"], "no capacity rows for fargate nodes")
	}
}

func TestCollect_NamespaceScope(t *testing.T) {
	cs := fake.NewSimpleClientset(
		readyNode("n1", "2", "8Gi", awsLabels),
		runningPod("a", "p1", "n1", corev1.PodRunning, nil),
		runningPod("b", "p2", "n1", corev1.PodRunning, nil),
	)
	resp, err := Collect(context.Background(), cs, Options{Cluster: "c", Namespace: "a"})
	require.NoError(t, err)
	assert.NotEmpty(t, rowsFor(resp, "pod", "p1"))
	assert.Empty(t, rowsFor(resp, "pod", "p2"))
}

func TestCollect_Forbidden(t *testing.T) {
	for _, resourceName := range []string{"nodes", "pods", "replicasets", "jobs"} {
		t.Run(resourceName, func(t *testing.T) {
			cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
			cs.PrependReactor("list", resourceName, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: resourceName}, "", nil)
			})
			_, err := Collect(context.Background(), cs, Options{Cluster: "c"})
			require.Error(t, err)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
			assert.Contains(t, err.Error(), "list "+resourceName)
		})
	}
}

func TestCollect_ControlPlane(t *testing.T) {
	cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
	resp, err := Collect(context.Background(), cs, Options{
		Cluster: "prod", APIServerHost: "https://X.gr7.us-east-1.eks.amazonaws.com",
	})
	require.NoError(t, err)
	var kinds []string
	for _, d := range resp.GetPriceable() {
		kinds = append(kinds, d.GetTags()["kind"])
	}
	assert.ElementsMatch(t, []string{"node", "cluster"}, kinds)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd plugins/kubernetes && go test ./usage/ -run 'TestNodeDescriptor|TestControlPlaneDescriptor|TestCollect'`
Expected: FAIL — `undefined: NodeDescriptor`.

- [ ] **Step 3: Implement**

`nodes.go`:

```go
package usage

import (
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	labelProvider        = "finfocus.dev/provider"
	labelInstanceType    = "node.kubernetes.io/instance-type"
	labelInstanceTypeOld = "beta.kubernetes.io/instance-type"
	labelRegion          = "topology.kubernetes.io/region"
	labelRegionOld       = "failure-domain.beta.kubernetes.io/region"
	labelEKSCapacity     = "eks.amazonaws.com/capacityType"
	labelKarpenterCap    = "karpenter.sh/capacity-type"
	labelEKSComputeType  = "eks.amazonaws.com/compute-type"
)

var (
	providerSchemes = map[string]string{"aws": "aws", "gce": "gcp", "azure": "azure"}
	eksHostPattern  = regexp.MustCompile(`\.([a-z]{2}(?:-[a-z]+)+-\d)\.eks\.amazonaws\.com(?::\d+)?/?$`)
)

// IsFargate reports whether the node is an EKS Fargate virtual node.
func IsFargate(n *corev1.Node) bool {
	return n.Labels[labelEKSComputeType] == "fargate"
}

// NodeDescriptor maps a node to a priceable descriptor. It returns false when
// the provider, instance type, or region cannot be determined.
func NodeDescriptor(n *corev1.Node) (*pbc.ResourceDescriptor, bool) {
	provider := n.Labels[labelProvider]
	if provider == "" {
		scheme, _, _ := strings.Cut(n.Spec.ProviderID, "://")
		provider = providerSchemes[scheme]
	}
	sku := firstLabel(n, labelInstanceType, labelInstanceTypeOld)
	region := firstLabel(n, labelRegion, labelRegionOld)
	if provider == "" || sku == "" || region == "" {
		return nil, false
	}
	resourceType := provider + ":compute/instance"
	if provider == "aws" {
		resourceType = "aws:ec2/instance:Instance"
	}
	capacity := "on-demand"
	if strings.EqualFold(n.Labels[labelEKSCapacity], "spot") || strings.EqualFold(n.Labels[labelKarpenterCap], "spot") {
		capacity = "spot"
	}
	return &pbc.ResourceDescriptor{
		Provider: provider, ResourceType: resourceType, Sku: sku, Region: region, Id: n.Name,
		Tags: map[string]string{"kind": "node", "provider_id": n.Spec.ProviderID, "capacity_type": capacity},
	}, true
}

// ControlPlaneDescriptor detects an EKS API server host and returns the
// control plane as a priceable descriptor.
func ControlPlaneDescriptor(host, cluster string) (*pbc.ResourceDescriptor, bool) {
	m := eksHostPattern.FindStringSubmatch(host)
	if m == nil {
		return nil, false
	}
	return &pbc.ResourceDescriptor{
		Provider: "aws", ResourceType: "aws:eks/cluster:Cluster", Sku: "cluster", Region: m[1], Id: cluster,
		Tags: map[string]string{"kind": "cluster"},
	}, true
}

func firstLabel(n *corev1.Node, keys ...string) string {
	for _, k := range keys {
		if v := n.Labels[k]; v != "" {
			return v
		}
	}
	return ""
}
```

`collector.go`:

```go
package usage

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const listPageSize = 500

// Options scopes a collection.
type Options struct {
	Cluster       string
	Namespace     string
	LabelSelector string
	APIServerHost string
}

// Collect lists nodes, pods, ReplicaSets, and Jobs and returns run-rate usage.
func Collect(ctx context.Context, cs kubernetes.Interface, opts Options) (*pbc.GetStatsResponse, error) {
	nodes, err := listAll(ctx, "nodes", "", func(o metav1.ListOptions) ([]corev1.Node, string, error) {
		l, err := cs.CoreV1().Nodes().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, err := listAll(ctx, "pods", opts.Namespace, func(o metav1.ListOptions) ([]corev1.Pod, string, error) {
		l, err := cs.CoreV1().Pods(opts.Namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{LabelSelector: opts.LabelSelector})
	if err != nil {
		return nil, err
	}
	rs, err := listAll(ctx, "replicasets", opts.Namespace, func(o metav1.ListOptions) ([]appsv1.ReplicaSet, string, error) {
		l, err := cs.AppsV1().ReplicaSets(opts.Namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	jobs, err := listAll(ctx, "jobs", opts.Namespace, func(o metav1.ListOptions) ([]batchv1.Job, string, error) {
		l, err := cs.BatchV1().Jobs(opts.Namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resp := &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE}
	for i := range nodes {
		n := &nodes[i]
		if IsFargate(n) {
			continue
		}
		subject := map[string]string{"kind": "node", "node": n.Name, "cluster": opts.Cluster}
		cpu := quantity(n.Status.Allocatable, corev1.ResourceCPU)
		mem := quantity(n.Status.Allocatable, corev1.ResourceMemory) / bytesPerGiB
		resp.Rows = append(resp.Rows,
			&pbc.UsageRow{Subject: subject, Metric: "cpu_allocatable", Amount: cpu, Unit: "core"},
			&pbc.UsageRow{Subject: subject, Metric: "mem_allocatable", Amount: mem, Unit: "GiB"},
		)
		if d, ok := NodeDescriptor(n); ok {
			resp.Priceable = append(resp.Priceable, d)
		} else {
			resp.Warnings = append(resp.Warnings,
				fmt.Sprintf("node %s: cannot determine provider, instance type, or region; not priced", n.Name))
		}
	}
	if d, ok := ControlPlaneDescriptor(opts.APIServerHost, opts.Cluster); ok {
		resp.Priceable = append(resp.Priceable, d)
	}

	owners := NewOwnerIndex(rs, jobs)
	for i := range pods {
		p := &pods[i]
		if p.Spec.NodeName == "" || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		kind, name := owners.Resolve(p)
		subject := map[string]string{
			"kind": "workload", "cluster": opts.Cluster, "namespace": p.Namespace, "pod": p.Name,
			"node": p.Spec.NodeName, "controller_kind": kind, "controller": name,
		}
		for k, v := range p.Labels {
			subject["label."+k] = v
		}
		cpu, mem := EffectiveRequests(p.Spec)
		resp.Rows = append(resp.Rows,
			&pbc.UsageRow{Subject: subject, Metric: "cpu_request", Amount: cpu, Unit: "core"},
			&pbc.UsageRow{Subject: subject, Metric: "mem_request", Amount: mem, Unit: "GiB"},
		)
	}
	return resp, nil
}

// listAll pages through a list call and maps API errors to gRPC status codes.
func listAll[T any](
	ctx context.Context,
	resource, namespace string,
	list func(metav1.ListOptions) ([]T, string, error),
	opts metav1.ListOptions,
) ([]T, error) {
	var out []T
	opts.Limit = listPageSize
	for {
		items, cont, err := list(opts)
		if err != nil {
			return nil, apiError(resource, namespace, err)
		}
		out = append(out, items...)
		if cont == "" {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		opts.Continue = cont
	}
}

func apiError(resource, namespace string, err error) error {
	scope := "cluster-wide"
	if namespace != "" {
		scope = "in namespace " + namespace
	}
	switch {
	case apierrors.IsForbidden(err):
		return status.Errorf(codes.PermissionDenied,
			"kubernetes RBAC: cannot list %s %s (grant list on %s; see docs for the minimal ClusterRole): %v",
			resource, scope, resource, err)
	case apierrors.IsUnauthorized(err):
		return status.Errorf(codes.Unauthenticated, "kubernetes authentication failed listing %s: %v", resource, err)
	default:
		return status.Errorf(codes.Unavailable, "list %s %s: %v", resource, scope, err)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `cd plugins/kubernetes && go test -cover ./usage/`
Expected: PASS; coverage ≥ 80%.

- [ ] **Step 5: Stage and hand off**

```bash
git add plugins/kubernetes/usage/ plugins/kubernetes/go.mod plugins/kubernetes/go.sum
```

Proposed message: `feat(kubernetes): collect run-rate usage and priceable nodes from the API`

---

### Task 5: Plugin glue, entry point, conformance

**Files:**

- Create: `plugins/kubernetes/plugin.go`, `plugins/kubernetes/kubeconfig.go`, `plugins/kubernetes/cmd/main.go`, `plugins/kubernetes/plugin.manifest.json`
- Test: `plugins/kubernetes/plugin_test.go`

**Interfaces:**

- Consumes: `allocate.Allocate` (Task 2), `usage.Collect`, `usage.Options` (Task 4), pluginsdk `BasePlugin`, `NewPluginInfo`, `WithCapabilities`, `WithProviders`, `Serve`, `ServeConfig`, `UsageSourceProvider`, `AllocatorProvider` (SP1), plugintesting `RunAllocatorConformance`, `ValidateStatsResponse` (SP1).
- Produces:

  ```go
  package kubernetes
  type Cluster struct { Client k8s.Interface; Host, Context string }
  type ClusterFactory func(scope string) (*Cluster, error)
  type Plugin struct { *pluginsdk.BasePlugin; clusters ClusterFactory }
  func New(clusters ClusterFactory) *Plugin
  func (p *Plugin) GetStats(ctx context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error)
  func (p *Plugin) Allocate(ctx context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error)
  func KubeconfigClusters(scope string) (*Cluster, error)
  func Info(version string) *pluginsdk.PluginInfo
  ```

- [ ] **Step 1: Write failing tests** (`plugins/kubernetes/plugin_test.go`)

```go
package kubernetes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
)

func fakeClusters(t *testing.T) ClusterFactory {
	return func(scope string) (*Cluster, error) {
		if scope == "missing" {
			return nil, status.Error(codes.InvalidArgument, `kubeconfig context "missing" not found`)
		}
		return &Cluster{Client: fake.NewSimpleClientset(), Context: "kind-test"}, nil
	}
}

func TestGetStats_RejectsHistorical(t *testing.T) {
	p := New(fakeClusters(t))
	_, err := p.GetStats(context.Background(), &pbc.GetStatsRequest{Start: timestamppb.Now()})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "run-rate only")
}

func TestGetStats_UnknownContext(t *testing.T) {
	_, err := New(fakeClusters(t)).GetStats(context.Background(), &pbc.GetStatsRequest{Scope: "missing"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetStats_ValidResponse(t *testing.T) {
	resp, err := New(fakeClusters(t)).GetStats(context.Background(),
		&pbc.GetStatsRequest{Selector: map[string]string{"namespace": "a", "app": "web"}})
	require.NoError(t, err)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
}

func TestInfo_ExplicitCapabilitiesOnly(t *testing.T) {
	info := Info("v0.1.0")
	assert.ElementsMatch(t, []pbc.PluginCapability{
		pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
		pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
	}, info.Capabilities)
}

func TestPlugin_ImplementsProviders(t *testing.T) {
	var p any = New(fakeClusters(t))
	_, ok := p.(pluginsdk.UsageSourceProvider)
	assert.True(t, ok)
	_, ok = p.(pluginsdk.AllocatorProvider)
	assert.True(t, ok)
}

type allocServer struct {
	pbc.UnimplementedAllocatorServiceServer
	p *Plugin
}

func (s allocServer) Allocate(ctx context.Context, r *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
	return s.p.Allocate(ctx, r)
}

func TestAllocatorConformance(t *testing.T) {
	plugintesting.RunAllocatorConformance(t, allocServer{p: New(fakeClusters(t))})
}

// Hosts consult Supports before routing price queries. Since finfocus-spec
// v0.6.2 (#507) the SDK reaches SupportsProvider without a registry, so the
// plugin's own "false" is what hosts see; without it, `cost projected` would
// record a NotSupported error per resource.
func TestSupports_OptsOutOfPricingThroughSDK(t *testing.T) {
	srv := pluginsdk.NewServerWithOptions(New(fakeClusters(t)), nil, nil, Info("v0.1.0"))
	for _, rt := range []string{"aws:ec2/instance:Instance", "ec2", "kubernetes:apps/v1:Deployment"} {
		resp, err := srv.Supports(context.Background(), &pbc.SupportsRequest{
			Resource: &pbc.ResourceDescriptor{ResourceType: rt},
		})
		require.NoError(t, err, rt)
		assert.False(t, resp.GetSupported(), rt)
	}
}

var _ = errors.New
```

Remove the trailing `var _ = errors.New` if `errors` ends up unused.

- [ ] **Step 2: Run to verify failure**

Run: `cd plugins/kubernetes && go test .`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Implement**

`plugin.go`:

```go
// Package kubernetes is a finfocus plugin that reports Kubernetes workload
// usage (UsageSourceService) and allocates node costs to workloads
// (AllocatorService).
package kubernetes

import (
	"context"
	"maps"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/kubernetes/allocate"
	"github.com/rshade/finfocus/plugins/kubernetes/usage"
)

// PluginName is the registry and binary name suffix.
const PluginName = "kubernetes"

// Cluster is a connected cluster.
type Cluster struct {
	Client  k8s.Interface
	Host    string
	Context string
}

// ClusterFactory connects to the cluster selected by scope (a kubeconfig
// context; empty means the current context).
type ClusterFactory func(scope string) (*Cluster, error)

// Plugin serves GetStats and Allocate.
type Plugin struct {
	*pluginsdk.BasePlugin
	clusters ClusterFactory
}

// New builds the plugin.
func New(clusters ClusterFactory) *Plugin {
	return &Plugin{BasePlugin: pluginsdk.NewBasePlugin(PluginName), clusters: clusters}
}

// Info declares capabilities explicitly so hosts never route price queries here.
func Info(version string) *pluginsdk.PluginInfo {
	return pluginsdk.NewPluginInfo(PluginName, version,
		pluginsdk.WithProviders("kubernetes"),
		pluginsdk.WithCapabilities(
			pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
			pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
		),
	)
}

// GetStats reports run-rate requests for the selected cluster.
func (p *Plugin) GetStats(ctx context.Context, req *pbc.GetStatsRequest) (*pbc.GetStatsResponse, error) {
	if req.GetStart() != nil || req.GetEnd() != nil {
		return nil, status.Error(codes.InvalidArgument,
			"kubernetes usage source is run-rate only; use a metrics-backed usage source for history")
	}
	cluster, err := p.clusters(req.GetScope())
	if err != nil {
		return nil, err
	}
	selector := maps.Clone(req.GetSelector())
	ns := selector["namespace"]
	delete(selector, "namespace")
	return usage.Collect(ctx, cluster.Client, usage.Options{
		Cluster:       cluster.Context,
		Namespace:     ns,
		LabelSelector: labelSelector(selector),
		APIServerHost: cluster.Host,
	})
}

// Allocate splits priced resources across workloads.
func (p *Plugin) Allocate(_ context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
	return allocate.Allocate(req)
}

// Supports declines every pricing request; this plugin prices nothing.
func (p *Plugin) Supports(_ context.Context, _ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: false, Reason: "kubernetes plugin provides usage and allocation only"}, nil
}
```

`SupportsProvider` in finfocus-spec v0.6.2 is
`Supports(ctx context.Context, req *pbc.SupportsRequest) (*pbc.SupportsResponse, error)`. No custom
`RegistryLookup` is needed: v0.6.2 (#507) skips the provider/region check when no registry is
configured and calls `SupportsProvider` directly.

Add a small helper in the same file:

```go
// labelSelector renders key=value pairs in sorted order as a Kubernetes label selector.
func labelSelector(m map[string]string) string {
	keys := slices.Sorted(maps.Keys(m))
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ",")
}
```

(import `slices` and `strings`).

`kubeconfig.go`:

```go
package kubernetes

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeconfigClusters connects using standard kubeconfig loading (KUBECONFIG,
// ~/.kube/config) or in-cluster config, selecting scope as the context.
func KubeconfigClusters(scope string) (*Cluster, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules,
		&clientcmd.ConfigOverrides{CurrentContext: scope})
	raw, err := loader.RawConfig()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "load kubeconfig (%v): %v", rules.GetLoadingPrecedence(), err)
	}
	contextName := scope
	if contextName == "" {
		contextName = raw.CurrentContext
	}
	if scope != "" {
		if _, ok := raw.Contexts[scope]; !ok {
			return nil, status.Errorf(codes.InvalidArgument,
				"kubeconfig context %q not found (searched %v)", scope, rules.GetLoadingPrecedence())
		}
	}
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "build client config for context %q: %v", contextName, err)
	}
	client, err := k8s.NewForConfig(cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("create kubernetes client: %v", err))
	}
	return &Cluster{Client: client, Host: cfg.Host, Context: contextName}, nil
}
```

`cmd/main.go` (mirrors `plugins/recorder/cmd/main.go`):

```go
// Command finfocus-plugin-kubernetes serves the kubernetes usage source and allocator.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/plugins/kubernetes"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v0.0.0-dev"

func main() {
	os.Exit(run())
}

func run() int {
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("plugin", kubernetes.PluginName).Logger()
	if os.Getenv("FINFOCUS_LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := pluginsdk.ServeConfig{
		Plugin:     kubernetes.New(kubernetes.KubeconfigClusters),
		PluginInfo: kubernetes.Info(version),
		Logger:     &logger,
	}
	if err := pluginsdk.Serve(ctx, cfg); err != nil {
		logger.Error().Err(err).Msg("plugin server error")
		return 1
	}
	return 0
}
```

Update `scripts/release-plugin-assets.sh` usage in SP3b's workflow to pass `-ldflags "-X main.version=$version"`: add `-X main.version=$version` to the script's `-ldflags` string (`"-s -w -X main.version=$version"`).

`plugin.manifest.json`:

```json
{
  "name": "kubernetes",
  "version": "0.1.0",
  "description": "Kubernetes workload usage (run-rate) and cost allocation by namespace, controller, pod, and label",
  "author": "FinFocus Team",
  "supported_providers": ["kubernetes"],
  "protocols": ["grpc"],
  "binary": "finfocus-plugin-kubernetes",
  "metadata": {
    "repository": "https://github.com/rshade/finfocus",
    "docs": "https://github.com/rshade/finfocus/tree/main/plugins/kubernetes"
  }
}
```

Resolve imports: `cd plugins/kubernetes && go get github.com/rs/zerolog@$(cd ../.. && go list -m -f '{{.Version}}' github.com/rs/zerolog) && go mod tidy`.

- [ ] **Step 4: Run tests**

Run: `cd plugins/kubernetes && go test -cover ./...`
Expected: PASS, including `TestAllocatorConformance`.

- [ ] **Step 5: Smoke-test the handshake**

```bash
cd plugins/kubernetes && go build -o /tmp/finfocus-plugin-kubernetes ./cmd
timeout 3 /tmp/finfocus-plugin-kubernetes 2>/dev/null | head -1
```

Expected: a single `PORT=<n>` line on stdout, nothing else.

- [ ] **Step 6: Stage and hand off**

```bash
git add plugins/kubernetes/ scripts/release-plugin-assets.sh
```

Proposed message: `feat(kubernetes): serve GetStats and Allocate with explicit capabilities`

---

### Task 6: README with RBAC and policy reference

**Files:**

- Create: `plugins/kubernetes/README.md`
- Create: `plugins/kubernetes/deploy/clusterrole.yaml`

**Interfaces:**

- Produces: the minimal ClusterRole the RBAC error messages point to.

- [ ] **Step 1: Write the ClusterRole**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: finfocus-reader
rules:
  - apiGroups: [""]
    resources: ["nodes", "pods"]
    verbs: ["list"]
  - apiGroups: ["apps"]
    resources: ["replicasets"]
    verbs: ["list"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["list"]
```

- [ ] **Step 2: Write the README**

Sections: what it does (run-rate, requests-based); install (`make install-kubernetes` today, `finfocus plugin install kubernetes` after release); kubeconfig/context selection; required RBAC (link `deploy/clusterrole.yaml`); node pricing requirements (instance-type + region labels, provider from `providerID` or `finfocus.dev/provider`); allocation policy reference (every field, default, supported values, the "lists replace, not append" rule, `version`); rows (`__idle__`, `__cluster__`, notes for spot/Fargate/unpriced); limitations (no history, spot priced on-demand, Fargate $0, no idle sharing).

- [ ] **Step 3: Lint**

Run: `markdownlint -c .markdownlint.json plugins/kubernetes/README.md`
Expected: pass.

- [ ] **Step 4: Stage and hand off**

```bash
git add plugins/kubernetes/README.md plugins/kubernetes/deploy/clusterrole.yaml
```

Proposed message: `docs(kubernetes): document plugin RBAC, node labels, and policy`

---

### Task 7: Build, lint, boundary, and CI wiring

**Files:**

- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml` (`validate` go.mod sync step :171-196; `test`, `lint` jobs)
- Modify: `release-please-config.json`, `.release-please-manifest.json`

**Interfaces:**

- Produces: `make build-kubernetes`, `make install-kubernetes`, `make test-kubernetes`, `make lint-kubernetes`, `make check-plugin-boundaries`; `make test` and `make lint` include them. Release-please component `kubernetes` producing `kubernetes-vX.Y.Z` tags.

- [ ] **Step 1: Makefile targets** (next to the recorder targets)

```make
KUBERNETES_PLUGIN_DIR=plugins/kubernetes
KUBERNETES_VERSION=$(shell jq -r '."plugins/kubernetes" // "0.0.0"' .release-please-manifest.json)
KUBERNETES_INSTALL_DIR=$(HOME)/.finfocus/plugins/kubernetes/v$(KUBERNETES_VERSION)

.PHONY: build-kubernetes
build-kubernetes:
	@mkdir -p bin
	go -C $(KUBERNETES_PLUGIN_DIR) build -ldflags "-X main.version=v$(KUBERNETES_VERSION)" \
		-o $(CURDIR)/bin/finfocus-plugin-kubernetes ./cmd

.PHONY: install-kubernetes
install-kubernetes: build-kubernetes
	@mkdir -p $(KUBERNETES_INSTALL_DIR)
	cp bin/finfocus-plugin-kubernetes $(KUBERNETES_INSTALL_DIR)/
	cp $(KUBERNETES_PLUGIN_DIR)/plugin.manifest.json $(KUBERNETES_INSTALL_DIR)/
	chmod 644 $(KUBERNETES_INSTALL_DIR)/plugin.manifest.json
	@echo "Verify with: finfocus plugin list"

.PHONY: test-kubernetes
test-kubernetes:
	go -C $(KUBERNETES_PLUGIN_DIR) test -race ./...

.PHONY: lint-kubernetes
lint-kubernetes:
	cd $(KUBERNETES_PLUGIN_DIR) && $(GOLANGCI_LINT) run --allow-parallel-runners ./...

.PHONY: check-plugin-boundaries
check-plugin-boundaries:
	@if go -C $(KUBERNETES_PLUGIN_DIR) list -deps ./... | grep -E '^github.com/rshade/finfocus/(internal|pkg)(/|$$)'; then \
		echo "plugins/kubernetes must not import finfocus core packages" >&2; exit 1; fi
	@echo "plugin boundaries OK"
```

Add `test-kubernetes` to the `test` target's prerequisites and `lint-kubernetes check-plugin-boundaries` to `lint`'s. Add `build-kubernetes` to `build-all`. The install dir uses `v<semver>` to match what the registry installer writes (canonical version).

- [ ] **Step 2: Verify the boundary check fails when violated**

```bash
cat > plugins/kubernetes/boundary_probe.go <<'EOF'
package kubernetes

import _ "github.com/rshade/finfocus/internal/config"
EOF
make check-plugin-boundaries; echo "exit=$?"
rm plugins/kubernetes/boundary_probe.go
make check-plugin-boundaries
```

Expected: first run fails — either the compiler error "no required module provides package github.com/rshade/finfocus/internal/config" or the boundary message, with `exit=2`; after removal, `plugin boundaries OK`.

- [ ] **Step 3: CI**

In `ci.yml`:

- `test` job: add a step `make test-kubernetes`.
- `lint` job: add `make lint-kubernetes check-plugin-boundaries`.
- `validate` job: generalize the "Verify go.mod sync between root and e2e modules" script to loop over `test/e2e/go.mod plugins/kubernetes/go.mod`, checking the `go` directive and the `finfocus-spec` / `testify` versions against root. Add `go -C plugins/kubernetes mod tidy -diff`.
- `build` job: add `make build-kubernetes` for each matrix target.

- [ ] **Step 4: Release-please**

`release-please-config.json` — add to `packages`, and exclude the path from the root package:

```json
    "plugins/kubernetes": {
      "release-type": "go",
      "component": "kubernetes",
      "include-component-in-tag": true,
      "tag-separator": "-",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "changelog-path": "CHANGELOG.md"
    }
```

and in the `"."` package add `"exclude-paths": ["plugins/kubernetes"]`. `.release-please-manifest.json`: add `"plugins/kubernetes": "0.0.0"`. Verify the tag shape with `npx release-please release-pr --dry-run --repo-url=rshade/finfocus --token="$GH_TOKEN" --config-file=release-please-config.json --manifest-file=.release-please-manifest.json 2>&1 | grep -i kubernetes` (expect a `kubernetes-v0.1.0`-style tag in the plan output). If the dry run cannot authenticate locally, state that and rely on the first real release PR for confirmation.

- [ ] **Step 5: Full verification**

Run: `make test` then `make lint` (allow >5 minutes).
Expected: PASS.

- [ ] **Step 6: Stage and hand off**

```bash
git add Makefile .github/workflows/ci.yml release-please-config.json .release-please-manifest.json
```

Proposed message: `build(kubernetes): make targets, CI, boundary check, and release component`

---

### Task 8: Registry entry (after first release and SP3 Task 1)

**Gate:** SP3b merged, SP3's "installed kubernetes plugin adds no errors to `cost projected`" integration test passing, and a published `kubernetes-v0.1.0` release with assets exists.

**Files:**

- Modify: `internal/registry/registry.json`
- Test: `internal/registry/registry_json_test.go` (existing validation covers it)

- [ ] **Step 1: Add the entry**

```json
    "kubernetes": {
      "name": "kubernetes",
      "description": "Kubernetes workload usage and cost allocation by namespace, controller, pod, and label",
      "repository": "rshade/finfocus",
      "author": "FinFocus Team",
      "license": "Apache-2.0",
      "homepage": "https://github.com/rshade/finfocus/tree/main/plugins/kubernetes",
      "supported_providers": ["kubernetes"],
      "capabilities": ["usage_stats", "allocation"],
      "security_level": "official",
      "min_spec_version": "<finfocus-spec version from SP1 Task 4, without v>",
      "tag_prefix": "kubernetes-",
      "asset_hints": {
        "asset_prefix": "finfocus-plugin-kubernetes"
      }
    }
```

Replace the `min_spec_version` placeholder with the actual version before staging (the test rejects non-semver).

- [ ] **Step 2: Verify against the real release**

```bash
make build
HOME=$(mktemp -d) ./bin/finfocus plugin install kubernetes
HOME=$HOME ./bin/finfocus plugin list
```

Expected: installs into `~/.finfocus/plugins/kubernetes/v0.1.0/`; `plugin list` shows `kubernetes v0.1.0`.

- [ ] **Step 3: Run tests and stage**

Run: `go test ./internal/registry/...` — PASS.

```bash
git add internal/registry/registry.json
```

Proposed message: `feat(registry): add kubernetes plugin`
