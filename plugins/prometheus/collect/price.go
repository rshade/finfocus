package collect

import (
	"context"

	"github.com/prometheus/common/model"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/prometheus/identity"
)

// addPriceables resolves a descriptor for each node that appears on a usage
// row. A label-only node with no usage row is not a priceable. Recorded
// kube_node_labels and kube_node_info win over the live object.
func addPriceables(
	ctx context.Context, resp *pbc.GetStatsResponse, data vectors, opts Options, cluster string,
) {
	priced, warnings := identity.Priceables(
		ctx, usageNodeNames(resp), recordedIdentity(data, cluster),
		opts.Live, cluster, opts.APIHost, opts.KubeconfigErr,
	)
	resp.Priceable = append(resp.Priceable, priced...)
	resp.Warnings = append(resp.Warnings, warnings...)
}

func usageNodeNames(resp *pbc.GetStatsResponse) []string {
	seen := make(map[string]struct{})
	var names []string
	for _, row := range resp.GetRows() {
		node := row.GetSubject()[pluginsdk.SubjectNode]
		if node == "" {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}
		names = append(names, node)
	}
	return names
}

func recordedIdentity(data vectors, cluster string) map[string]identity.NodeLabels {
	type pair struct {
		labels model.Metric
		info   model.Metric
	}
	byNode := map[string]*pair{}
	take := func(vec model.Vector, info bool) {
		for _, sample := range vec {
			if !clusterSample(sample.Metric, cluster) {
				continue
			}
			name := string(sample.Metric["node"])
			if name == "" {
				continue
			}
			slot := byNode[name]
			if slot == nil {
				slot = &pair{}
				byNode[name] = slot
			}
			if info {
				slot.info = sample.Metric
				continue
			}
			slot.labels = sample.Metric
		}
	}
	take(data.nodeLabels, false)
	take(data.nodeInfo, true)
	out := make(map[string]identity.NodeLabels, len(byNode))
	for name, slot := range byNode {
		out[name] = identity.JoinNode(name, cluster, metricMap(slot.labels), metricMap(slot.info))
	}
	return out
}

// clusterSample keeps a sample with no cluster label, and a sample whose
// cluster label is the selected cluster. A different cluster label is dropped
// so a leaked series cannot price the wrong node.
func clusterSample(metric model.Metric, cluster string) bool {
	got := string(metric["cluster"])
	return got == "" || got == cluster
}

func metricMap(metric model.Metric) map[string]string {
	if metric == nil {
		return nil
	}
	out := make(map[string]string, len(metric))
	for key, value := range metric {
		out[string(key)] = string(value)
	}
	return out
}
