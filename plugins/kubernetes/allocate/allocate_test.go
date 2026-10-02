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
	return nodeRowsIn("c", node, cpu, mem)
}

func nodeRowsIn(cluster, node string, cpu, mem float64) []*pbc.UsageRow {
	s := map[string]string{"kind": "node", "node": node, "cluster": cluster}
	return []*pbc.UsageRow{
		{Subject: s, Metric: "cpu_allocatable", Amount: cpu, Unit: "core"},
		{Subject: s, Metric: "mem_allocatable", Amount: mem, Unit: "GiB"},
	}
}

func podRows(ns, pod, node string, cpu, mem float64) []*pbc.UsageRow {
	return podRowsIn("c", ns, pod, node, cpu, mem)
}

func podRowsIn(cluster, ns, pod, node string, cpu, mem float64) []*pbc.UsageRow {
	s := map[string]string{
		"kind": "workload", "namespace": ns, "pod": pod, "node": node, "cluster": cluster,
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
	t.Parallel()

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
	t.Parallel()

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
			t.Parallel()
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
	t.Parallel()

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
	t.Parallel()

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

// TestAllocate_WorkloadKeyIncludesCluster covers #1576: the workload key used
// to be "namespace/pod" only, so same-named pods in different clusters (or on
// different nodes) merged into one allocation row and misattributed cost.
func TestAllocate_WorkloadKeyIncludesCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *pbc.AllocateRequest
		// wantWorkloads is the expected number of workload rows; the old key
		// merged the two-cluster and two-node cases into a single row.
		wantWorkloads int
		// wantWorkloadTotal maps cluster to the summed total of its workload
		// rows, proving cost is attributed to the right cluster.
		wantWorkloadTotal map[string]float64
		// wantClusterTotal maps cluster to the summed total of all its rows
		// (workload + idle), i.e. per-cluster conservation.
		wantClusterTotal map[string]float64
	}{
		{
			name: "same namespace and pod in two clusters stay separate",
			req: &pbc.AllocateRequest{
				Usage: concat(nodeRowsIn("c1", "n1", 2, 8), podRowsIn("c1", "app", "web", "n1", 0.5, 2),
					nodeRowsIn("c2", "n2", 2, 8), podRowsIn("c2", "app", "web", "n2", 1, 4)),
				Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil), pricedNode("n2", 140, nil)},
			},
			wantWorkloads:     2,
			wantWorkloadTotal: map[string]float64{"c1": 70 * 0.25, "c2": 140 * 0.5},
			wantClusterTotal:  map[string]float64{"c1": 70, "c2": 140},
		},
		{
			name: "same pod on two nodes in one cluster stays separate",
			req: &pbc.AllocateRequest{
				Usage: concat(nodeRows("n1", 2, 8), nodeRows("n2", 2, 8),
					podRows("app", "web", "n1", 0.5, 2), podRows("app", "web", "n2", 0.25, 1)),
				Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil), pricedNode("n2", 70, nil)},
			},
			wantWorkloads:     2,
			wantWorkloadTotal: map[string]float64{"c": 70*0.25 + 70*0.125},
			wantClusterTotal:  map[string]float64{"c": 140},
		},
		{
			name: "single cluster still merges one pod's metric rows into one row",
			req: &pbc.AllocateRequest{
				Usage:  concat(nodeRows("n1", 2, 8), podRows("app", "web", "n1", 0.5, 2)),
				Priced: []*pbc.PricedResource{pricedNode("n1", 70, nil)},
			},
			wantWorkloads:     1,
			wantWorkloadTotal: map[string]float64{"c": 70 * 0.25},
			wantClusterTotal:  map[string]float64{"c": 70},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := Allocate(tt.req)
			require.NoError(t, err)

			workloadTotal := map[string]float64{}
			clusterTotal := map[string]float64{}
			seen := map[string]bool{}
			var workloads int
			for _, r := range resp.GetRows() {
				s := r.GetSubject()
				clusterTotal[s["cluster"]] += r.GetTotalCost()
				if s["kind"] != "workload" {
					continue
				}
				workloads++
				identity := s["cluster"] + "\x00" + s["namespace"] + "\x00" + s["pod"] + "\x00" + s["node"]
				assert.False(t, seen[identity], "duplicate workload row for %v", s)
				seen[identity] = true
				workloadTotal[s["cluster"]] += r.GetTotalCost()
			}
			assert.Equal(t, tt.wantWorkloads, workloads)
			for cluster, want := range tt.wantWorkloadTotal {
				assert.InDelta(t, want, workloadTotal[cluster], want*1e-6+1e-12, "cluster %s workload total", cluster)
			}
			for cluster, want := range tt.wantClusterTotal {
				assert.InDelta(t, want, clusterTotal[cluster], want*1e-6+1e-12, "cluster %s total", cluster)
			}
			assertValidAllocation(t, tt.req, resp)
		})
	}
}

func TestAllocate_Errors(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
			t.Parallel()
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
	t.Parallel()

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

func podRowsKind(ns, pod, node, kind string, cpu, mem float64) []*pbc.UsageRow {
	rows := podRows(ns, pod, node, cpu, mem)
	rows[0].Subject["controller_kind"] = kind
	return rows
}

func allocateWithPolicy(
	t *testing.T, doc string, usage []*pbc.UsageRow, priced ...*pbc.PricedResource,
) *pbc.AllocateResponse {
	t.Helper()
	req := &pbc.AllocateRequest{Usage: usage, Priced: priced, PolicyJson: []byte(doc)}
	resp, err := Allocate(req)
	require.NoError(t, err)
	assertValidAllocation(t, req, resp)
	return resp
}

func TestAllocate_ShareIdle(t *testing.T) {
	t.Parallel()

	const cost = 70.08
	priced := pricedNode("n1", cost, nil)

	t.Run("one workload absorbs the node", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"idle":"share"}`,
			concat(nodeRows("n1", 2, 8), podRows("app", "api-1", "n1", 0.5, 2)), priced)
		api := rowFor(t, resp, "workload", "pod", "api-1")
		idle := rowFor(t, resp, "__idle__", "node", "n1")
		assert.InDelta(t, cost, api.GetTotalCost(), 1e-9)
		assert.InDelta(t, 0, idle.GetTotalCost(), 1e-9)
	})

	t.Run("equal workloads split idle evenly", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"idle":"share"}`,
			concat(nodeRows("n1", 2, 8), podRows("app", "a", "n1", 0.5, 2), podRows("app", "b", "n1", 0.5, 2)),
			priced)
		assert.InDelta(t, cost/2, rowFor(t, resp, "workload", "pod", "a").GetTotalCost(), 1e-9)
		assert.InDelta(t, cost/2, rowFor(t, resp, "workload", "pod", "b").GetTotalCost(), 1e-9)
		assert.InDelta(t, 0, rowFor(t, resp, "__idle__", "node", "n1").GetTotalCost(), 1e-9)
	})

	t.Run("fully packed matches separate", func(t *testing.T) {
		t.Parallel()
		usage := concat(nodeRows("n1", 2, 8), podRows("app", "api-1", "n1", 2, 8))
		shared := allocateWithPolicy(t, `{"idle":"share"}`, usage, priced)
		separate := allocateWithPolicy(t, `{"idle":"separate"}`, usage, pricedNode("n1", cost, nil))
		assert.InDelta(t, separate.GetRows()[0].GetTotalCost(), shared.GetRows()[0].GetTotalCost(), 1e-9)
		assert.InDelta(t, 0, rowFor(t, shared, "__idle__", "node", "n1").GetTotalCost(), 1e-9)
	})

	t.Run("empty node keeps idle", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"idle":"share"}`, nodeRows("n1", 2, 8), priced)
		assert.Len(t, resp.GetRows(), 1)
		idle := rowFor(t, resp, "__idle__", "node", "n1")
		assert.InDelta(t, cost, idle.GetTotalCost(), 1e-9)
	})

	t.Run("other node is independent", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"idle":"share"}`,
			concat(nodeRows("n1", 2, 8), nodeRows("n2", 2, 8), podRows("app", "api-1", "n1", 0.5, 2)),
			priced, pricedNode("n2", 10, nil))
		assert.InDelta(t, cost, rowFor(t, resp, "workload", "pod", "api-1").GetTotalCost(), 1e-9)
		assert.InDelta(t, 0, rowFor(t, resp, "__idle__", "node", "n1").GetTotalCost(), 1e-9)
		assert.InDelta(t, 10, rowFor(t, resp, "__idle__", "node", "n2").GetTotalCost(), 1e-9)
	})
}

func TestAllocate_ShareSystemWorkloads(t *testing.T) {
	t.Parallel()

	const cost = 70.08
	priced := pricedNode("n1", cost, nil)
	usage := concat(nodeRows("n1", 2, 8),
		podRows("app", "api-1", "n1", 0.5, 2),
		podRows("kube-system", "coredns", "n1", 0.5, 2),
		podRowsKind("app", "logger", "n1", "DaemonSet", 0.5, 2))

	t.Run("kube-system and daemonset fold into the app", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"system_workloads":"share"}`, usage, priced)
		api := rowFor(t, resp, "workload", "pod", "api-1")
		idle := rowFor(t, resp, "__idle__", "node", "n1")
		assert.InDelta(t, cost*0.75, api.GetTotalCost(), 1e-9)
		assert.InDelta(t, cost*0.25, idle.GetTotalCost(), 1e-9)
		for _, row := range resp.GetRows() {
			assert.NotEqual(t, "kube-system", row.GetSubject()["namespace"])
			assert.NotEqual(t, "DaemonSet", row.GetSubject()["controller_kind"])
		}
	})

	t.Run("only system workloads stay visible", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"system_workloads":"share"}`,
			concat(nodeRows("n1", 2, 8), podRows("kube-system", "coredns", "n1", 0.5, 2)), priced)
		core := rowFor(t, resp, "workload", "pod", "coredns")
		assert.InDelta(t, cost*0.25, core.GetTotalCost(), 1e-9)
	})

	t.Run("idle share then system fold gives the app the node", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"idle":"share","system_workloads":"share"}`, usage, priced)
		api := rowFor(t, resp, "workload", "pod", "api-1")
		assert.InDelta(t, cost, api.GetTotalCost(), 1e-9)
		assert.InDelta(t, 0, rowFor(t, resp, "__idle__", "node", "n1").GetTotalCost(), 1e-9)
		assert.Len(t, resp.GetRows(), 2)
	})

	t.Run("idle share leaves system rows in place", func(t *testing.T) {
		t.Parallel()
		resp := allocateWithPolicy(t, `{"idle":"share","system_workloads":"separate"}`, usage, priced)
		assert.InDelta(t, cost/3, rowFor(t, resp, "workload", "pod", "coredns").GetTotalCost(), 1e-9)
		assert.InDelta(t, 0, rowFor(t, resp, "__idle__", "node", "n1").GetTotalCost(), 1e-9)
	})
}
