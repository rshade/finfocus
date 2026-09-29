package allocate

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
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

func podRowsWithUsage(ns, pod, node string, cpuReq, memReq, cpuUse, memUse float64) []*pbc.UsageRow {
	s := map[string]string{
		"kind": "workload", "namespace": ns, "pod": pod, "node": node, "cluster": "c",
		"controller_kind": "Deployment", "controller": pod + "-deploy",
	}
	return []*pbc.UsageRow{
		{Subject: s, Metric: "cpu_request", Amount: cpuReq, Unit: "core"},
		{Subject: s, Metric: "mem_request", Amount: memReq, Unit: "GiB"},
		{Subject: s, Metric: "cpu_usage", Amount: cpuUse, Unit: "core"},
		{Subject: s, Metric: "mem_usage", Amount: memUse, Unit: "GiB"},
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

// assertValidAllocation checks resp against the SDK's own response
// invariants (row kinds, idle-row node keys, per-row cost totals, currency,
// exactly one idle row per priced node) and conservation (rows sum to the
// priced total within tolerance), so every fixture is validated the same way
// a host would validate a plugin's response.
func assertValidAllocation(t *testing.T, req *pbc.AllocateRequest, resp *pbc.AllocateResponse) {
	t.Helper()
	assert.NoError(t, pluginsdk.ValidateAllocateResponse(req, resp))
	assert.NoError(t, pluginsdk.CheckConservation(req, resp, pluginsdk.DefaultConservationEpsilon))
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
	assertValidAllocation(t, req, resp)
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
			assertValidAllocation(t, tt.req, resp)
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
	assertValidAllocation(t, req, resp)
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
	assertValidAllocation(t, req, resp)
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

	_, err = Allocate(
		&pbc.AllocateRequest{Priced: []*pbc.PricedResource{pricedNode("n1", 10, nil), pricedNode("n1", 5, nil)}},
	)
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
	req := &pbc.AllocateRequest{
		Usage:  concat(nodeRows("n1", 2, 8), nodeRows("n2", 2, 8)),
		Priced: []*pbc.PricedResource{eur, blank},
	}
	resp, err := Allocate(req)
	require.NoError(t, err)
	for _, r := range resp.GetRows() {
		assert.Equal(t, "EUR", r.GetCurrency())
	}
	assertValidAllocation(t, req, resp)
}

func TestAllocate_EmptyRequestReturnsEffectivePolicy(t *testing.T) {
	req := &pbc.AllocateRequest{}
	resp, err := Allocate(req)
	require.NoError(t, err)
	assert.Empty(t, resp.GetRows())
	assert.True(t, json.Valid(resp.GetEffectivePolicyJson()))
	assert.Contains(t, string(resp.GetEffectivePolicyJson()), `"version":1`)
	assert.Len(t, resp.GetPolicyDigest(), 64)
	assertValidAllocation(t, req, resp)
}

func TestAllocate_DeterministicOrder(t *testing.T) {
	req := &pbc.AllocateRequest{
		Usage: concat(nodeRows("n2", 2, 8), nodeRows("n1", 2, 8),
			podRows("b", "z", "n2", 1, 1), podRows("a", "y", "n1", 1, 1)),
		Priced: []*pbc.PricedResource{pricedNode("n2", 10, nil), pricedNode("n1", 10, nil)},
	}
	first, err := Allocate(req)
	require.NoError(t, err)
	assertValidAllocation(t, req, first)
	for i := 0; i < 5; i++ {
		again, allocErr := Allocate(req)
		require.NoError(t, allocErr)
		require.Len(t, again.GetRows(), len(first.GetRows()))
		for j := range first.GetRows() {
			assert.Equal(t, first.GetRows()[j].GetSubject(), again.GetRows()[j].GetSubject())
		}
	}
}

// TestAllocate_UnpricedNodeWithEmptyIDHasNoIdleRow covers fix-round-1 finding
// 1: pluginsdk.ValidateAllocateRequest accepts an unpriced node with an empty
// resource.id (only priced=true nodes must have a non-empty id), but an idle
// row always needs a non-empty "node" subject key. Allocate must not emit one
// for this node -- its cost is 0 regardless, so nothing is lost.
func TestAllocate_UnpricedNodeWithEmptyIDHasNoIdleRow(t *testing.T) {
	req := &pbc.AllocateRequest{
		Priced: []*pbc.PricedResource{{
			Resource: &pbc.ResourceDescriptor{Tags: map[string]string{"kind": "node"}, Id: ""},
			Priced:   false,
		}},
	}
	resp, err := Allocate(req)
	require.NoError(t, err)
	for _, r := range resp.GetRows() {
		assert.NotEqual(t, "__idle__", r.GetSubject()["kind"], "unpriced node with empty id must not emit an idle row")
	}
	assertValidAllocation(t, req, resp)
}

// TestAllocate_EKSControlPlaneRowHasNoNote covers final-review finding 1: the
// collector (usage/nodes.go ControlPlaneDescriptor) tags the EKS control-plane
// priceable resource "kind": "cluster" (a priceable-tag value), which used to
// be compared against the pluginsdk.KindCluster row-subject constant
// ("__cluster__") and always mismatched, giving every control-plane row a
// spurious "unallocated priced resource kind" note.
func TestAllocate_EKSControlPlaneRowHasNoNote(t *testing.T) {
	eks := &pbc.PricedResource{
		Resource: &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:eks/cluster:Cluster",
			Sku: "cluster", Region: "us-east-1", Id: "c", Tags: map[string]string{"kind": "cluster"}},
		Cost: 73, Currency: "USD", Priced: true,
	}
	req := &pbc.AllocateRequest{Priced: []*pbc.PricedResource{eks}}
	resp, err := Allocate(req)
	require.NoError(t, err)
	row := rowFor(t, resp, "__cluster__", "cluster", "c")
	assert.Empty(t, row.GetNote())
	assertValidAllocation(t, req, resp)
}

// TestAllocate_ChargeUsesMaxRequestUsage covers fix-round-1 finding 2: rule 3
// charges max(request, usage) per resource, so it must be exercised on both
// sides -- usage above request and request above usage -- with exact
// expected shares, not just aggregate conservation.
func TestAllocate_ChargeUsesMaxRequestUsage(t *testing.T) {
	const cost = 70.0
	tests := []struct {
		name                           string
		cpuReq, memReq, cpuUse, memUse float64
	}{
		{"usage exceeds request", 0.5, 1, 1.5, 3},
		{"request exceeds usage", 1.5, 5, 0.5, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pbc.AllocateRequest{
				Usage: concat(nodeRows("n1", 2, 8),
					podRowsWithUsage("a", "p", "n1", tt.cpuReq, tt.memReq, tt.cpuUse, tt.memUse)),
				Priced: []*pbc.PricedResource{pricedNode("n1", cost, nil)},
			}
			resp, err := Allocate(req)
			require.NoError(t, err)
			row := rowFor(t, resp, "workload", "pod", "p")

			cpuPortion := cost * m5CPUFraction
			memPortion := cost * (1 - m5CPUFraction)
			wantCPU := cpuPortion * (max(tt.cpuReq, tt.cpuUse) / 2)
			wantMem := memPortion * (max(tt.memReq, tt.memUse) / 8)

			assert.InDelta(t, wantCPU, row.GetCpuCost(), 1e-9)
			assert.InDelta(t, wantMem, row.GetMemCost(), 1e-9)
			assertValidAllocation(t, req, resp)
		})
	}
}

// TestAllocate_SanitizesNonFiniteAndNegativeUsage covers fix-round-1 finding
// 4 (controller ruling): a NaN cpu_request and a negative mem_allocatable
// must be treated as 0 rather than propagated, so malformed upstream usage
// cannot break conservation.
func TestAllocate_SanitizesNonFiniteAndNegativeUsage(t *testing.T) {
	nodeUsage := []*pbc.UsageRow{
		{Subject: map[string]string{"kind": "node", "node": "n1", "cluster": "c"},
			Metric: "cpu_allocatable", Amount: 2, Unit: "core"},
		{Subject: map[string]string{"kind": "node", "node": "n1", "cluster": "c"},
			Metric: "mem_allocatable", Amount: -8, Unit: "GiB"},
	}
	podUsage := podRows("a", "p", "n1", 0.5, 1)
	podUsage[0].Amount = math.NaN() // cpu_request

	req := &pbc.AllocateRequest{
		Usage:  concat(nodeUsage, podUsage),
		Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)},
	}
	resp, err := Allocate(req)
	require.NoError(t, err)
	assert.InDelta(t, 70, sumRows(resp), 70e-6)
	for _, r := range resp.GetRows() {
		assert.GreaterOrEqual(t, r.GetTotalCost(), 0.0, "row %v", r.GetSubject())
	}
	assertValidAllocation(t, req, resp)
}
