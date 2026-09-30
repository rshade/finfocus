// Package allocate divides priced cluster resources across workloads.
package allocate

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/kubernetes/policy"
)

const (
	fargateNodePrefix = "fargate-"
	capacityTypeKey   = "capacity_type"
	capacityTypeSpot  = "spot"
	noteSpotOnDemand  = "spot node priced on-demand"
	noteFargate       = "Fargate pricing not supported yet"

	// subjectKind, subjectNode, etc. and kindNode, kindCluster, etc. are the
	// pluginsdk row-subject vocabulary used to build UsageRow/AllocationRow
	// Subject maps. They must NOT be used to read tags off a *priced*
	// resource (pbc.ResourceDescriptor.Tags) -- see priceableTagKind below.
	subjectNamespace = pluginsdk.SubjectNamespace
	subjectPod       = pluginsdk.SubjectPod
	subjectNode      = pluginsdk.SubjectNode
	subjectCluster   = pluginsdk.SubjectCluster
	subjectKind      = pluginsdk.SubjectKind

	kindNode     = pluginsdk.KindNode
	kindWorkload = pluginsdk.KindWorkload
	kindIdle     = pluginsdk.KindIdle
	kindCluster  = pluginsdk.KindCluster

	// priceableTagKind and priceableKind* describe the resource.tags["kind"]
	// vocabulary the collector (usage/nodes.go NodeDescriptor and
	// ControlPlaneDescriptor) attaches to *priced* resources: "node" and
	// "cluster". This is deliberately distinct from the pluginsdk row-subject
	// Kind* vocabulary above -- pluginsdk.KindCluster is "__cluster__",
	// reserved for allocator output rows, and is never a priced-resource tag
	// value. Comparing a priced resource's tag against pluginsdk.KindCluster
	// always fails and mislabels every EKS control-plane row.
	priceableTagKind     = "kind"
	priceableKindNode    = "node"
	priceableKindCluster = "cluster"

	metricCPUAlloc   = pluginsdk.MetricCPUAllocatable
	metricMemAlloc   = pluginsdk.MetricMemAllocatable
	metricCPURequest = pluginsdk.MetricCPURequest
	metricMemRequest = pluginsdk.MetricMemRequest
	metricCPUUsage   = pluginsdk.MetricCPUUsage
	metricMemUsage   = pluginsdk.MetricMemUsage
)

type workload struct {
	subject        map[string]string
	cpuReq, memReq float64
	cpuUse, memUse float64
}

func (w *workload) cpu() float64 { return max(w.cpuReq, w.cpuUse) }
func (w *workload) mem() float64 { return max(w.memReq, w.memUse) }

type node struct {
	name      string
	cpuAlloc  float64
	memAlloc  float64
	priced    *pbc.PricedResource
	workloads []*workload
	cluster   string
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

	nodes, clusterRes := groupPricedResources(req.GetPriced())
	workloads := collectWorkloadUsage(req.GetUsage(), nodes)
	orphans := attachWorkloadsToNodes(workloads, nodes)

	return &pbc.AllocateResponse{
		EffectivePolicyJson: canonical,
		PolicyDigest:        digest,
		Rows:                buildRows(nodes, orphans, clusterRes, pol, currency),
	}, nil
}

// groupPricedResources splits priced entries into nodes (keyed by resource
// id) and everything else (control plane, unallocated kinds).
func groupPricedResources(priced []*pbc.PricedResource) (map[string]*node, []*pbc.PricedResource) {
	nodes := map[string]*node{}
	var clusterRes []*pbc.PricedResource
	for _, pr := range priced {
		if pr.GetResource().GetTags()[priceableTagKind] == priceableKindNode {
			nodeFor(nodes, pr.GetResource().GetId()).priced = pr
			continue
		}
		clusterRes = append(clusterRes, pr)
	}
	return nodes, clusterRes
}

// collectWorkloadUsage folds usage rows into node capacity (mutating nodes in
// place) and per-workload amounts, keyed by cluster/namespace/pod/node.
func collectWorkloadUsage(usage []*pbc.UsageRow, nodes map[string]*node) map[string]*workload {
	workloads := map[string]*workload{}
	for _, u := range usage {
		s := u.GetSubject()
		switch s[subjectKind] {
		case kindNode:
			applyNodeCapacity(nodeFor(nodes, s[subjectNode]), s, u)
		case kindWorkload:
			applyWorkloadMetric(workloadFor(workloads, s), u)
		}
	}
	return workloads
}

func nodeFor(nodes map[string]*node, name string) *node {
	if nodes[name] == nil {
		nodes[name] = &node{name: name}
	}
	return nodes[name]
}

func applyNodeCapacity(n *node, subject map[string]string, u *pbc.UsageRow) {
	n.cluster = subject[subjectCluster]
	amount := sanitizeAmount(u.GetAmount())
	switch u.GetMetric() {
	case metricCPUAlloc:
		n.cpuAlloc = amount
	case metricMemAlloc:
		n.memAlloc = amount
	}
}

// sanitizeAmount treats a non-finite (NaN/±Inf) or negative usage amount as
// 0, so malformed upstream usage rows cannot corrupt shares or break
// conservation.
func sanitizeAmount(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

// workloadFor keys workloads on cluster/namespace/pod/node. Omitting the
// cluster merged same-named pods from different clusters into one allocation
// row (#1576); the node is part of the key because it decides which priced
// node the workload is charged against. The separator is NUL so that subject
// values containing "/" (e.g. kubeconfig context names) cannot collide.
func workloadFor(workloads map[string]*workload, subject map[string]string) *workload {
	key := strings.Join([]string{
		subject[subjectCluster], subject[subjectNamespace],
		subject[subjectPod], subject[subjectNode],
	}, "\x00")
	w := workloads[key]
	if w == nil {
		w = &workload{subject: subject}
		workloads[key] = w
	}
	return w
}

func applyWorkloadMetric(w *workload, u *pbc.UsageRow) {
	amount := sanitizeAmount(u.GetAmount())
	switch u.GetMetric() {
	case metricCPURequest:
		w.cpuReq = amount
	case metricMemRequest:
		w.memReq = amount
	case metricCPUUsage:
		w.cpuUse = amount
	case metricMemUsage:
		w.memUse = amount
	}
}

// attachWorkloadsToNodes assigns each workload to the priced node it runs on
// and returns those whose node has no priced entry.
func attachWorkloadsToNodes(workloads map[string]*workload, nodes map[string]*node) []*workload {
	var orphans []*workload
	for _, key := range sortedKeys(workloads) {
		w := workloads[key]
		if n, ok := nodes[w.subject[subjectNode]]; ok && n.priced != nil {
			n.workloads = append(n.workloads, w)
		} else {
			orphans = append(orphans, w)
		}
	}
	return orphans
}

// buildRows renders nodes (workload + idle rows), then orphaned workloads,
// then control-plane/unallocated rows, in deterministic order.
func buildRows(
	nodes map[string]*node, orphans []*workload, clusterRes []*pbc.PricedResource,
	pol policy.Policy, currency string,
) []*pbc.AllocationRow {
	var rows, idleRows []*pbc.AllocationRow
	for _, name := range sortedKeys(nodes) {
		n := nodes[name]
		if n.priced == nil {
			continue // capacity reported but nothing to split; workloads were orphaned above
		}
		nodeRows, idle := allocateNode(n, pol, currency)
		rows = append(rows, nodeRows...)
		if n.name != "" {
			// A node with an empty id can only be an unpriced entry (the SDK
			// rejects a priced node with an empty resource.id), so its idle
			// share is always 0. An idle row needs a non-empty "node"
			// subject key (SDK response validation), so it is dropped
			// rather than emitted with an empty value.
			idleRows = append(idleRows, idle)
		}
	}
	for _, w := range orphans {
		rows = append(rows, zeroRow(w.subject, currency, orphanNote(w.subject[subjectNode])))
	}
	rows = append(rows, idleRows...)
	for _, pr := range clusterRes {
		rows = append(rows, clusterRow(pr, currency))
	}
	return rows
}

func allocateNode(n *node, pol policy.Policy, currency string) ([]*pbc.AllocationRow, *pbc.AllocationRow) {
	cost := 0.0
	note := ""
	if n.priced.GetPriced() {
		cost = n.priced.GetCost()
	} else {
		note = fmt.Sprintf("node %s has no price", n.name)
	}
	if note == "" && n.priced.GetResource().GetTags()[capacityTypeKey] == capacityTypeSpot {
		note = noteSpotOnDemand
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
		Subject:   map[string]string{subjectKind: kindIdle, subjectNode: n.name, subjectCluster: n.cluster},
		CpuCost:   idleCPU,
		MemCost:   idleMem,
		TotalCost: idleCPU + idleMem,
		Currency:  currency,
		Note:      note,
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
	if strings.HasPrefix(nodeName, fargateNodePrefix) {
		return noteFargate
	}
	return fmt.Sprintf("node %s not found in priced resources", nodeName)
}

func zeroRow(subject map[string]string, currency, note string) *pbc.AllocationRow {
	return &pbc.AllocationRow{Subject: subject, Currency: currency, Note: note}
}

func clusterRow(pr *pbc.PricedResource, currency string) *pbc.AllocationRow {
	r := pr.GetResource()
	row := &pbc.AllocationRow{
		Subject:   map[string]string{subjectKind: kindCluster, subjectCluster: r.GetId()},
		Currency:  currency,
		TotalCost: 0,
	}
	if pr.GetPriced() {
		row.TotalCost = pr.GetCost()
	}
	if kind := r.GetTags()[priceableTagKind]; kind != priceableKindCluster {
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
