// Package collect queries Prometheus and assembles historical usage rows.
package collect

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/client-go/kubernetes"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/prometheus/identity"
	"github.com/rshade/finfocus/plugins/prometheus/promql"
)

// Options scopes one collection.
// Namespace is a PromQL matcher on pod series.
// Cluster is the request scope. It becomes the cluster subject when the store
// has no cluster label, and it selects one cluster when the store has several.
// PodLabels are exact Kubernetes label matches applied to kube_pod_labels.
// Live, APIHost, and KubeconfigErr are the node-identity fallback. A kubeconfig
// error is a warning; collection still returns the rows.
type Options struct {
	Namespace     string
	Cluster       string
	PodLabels     map[string]string
	Live          kubernetes.Interface
	APIHost       string
	KubeconfigErr error
}

// Collect issues the historical instant queries and returns usage rows.
// The response mode is left unset; the plugin sets STATS_MODE_HISTORICAL.
// HTTP calls use ctx, so a canceled request stops the queries.
//
// Cluster labels are read before any sum. sum by (node) drops the cluster
// label, so the matcher has to be on the later queries or two clusters would
// be added together. A store with no cluster label is not filtered by scope.
func Collect(ctx context.Context, api v1.API, from, to time.Time, opts Options) (*pbc.GetStatsResponse, error) {
	discovered, err := instant(ctx, api, promql.ClusterDiscovery(from, to))
	if err != nil {
		return nil, fmt.Errorf("query cluster discovery: %w", err)
	}
	clusters := clusterValues(discovered)
	selected, err := identity.SelectCluster(clusters, opts.Cluster)
	if err != nil {
		return nil, err
	}
	sel := promql.Selectors{Namespace: opts.Namespace}
	if len(clusters) > 0 {
		sel.Cluster = selected
	}
	data, err := fetch(ctx, api, from, to, sel)
	if err != nil {
		return nil, err
	}
	resp := &pbc.GetStatsResponse{}
	rowOpts := opts
	rowOpts.Cluster = selected
	addWorkloads(resp, data, rowOpts)
	addNodes(resp, data.cpuAlloc, data.memAlloc, selected)
	addRetentionGap(resp, data.storeStart, from)
	addPriceables(ctx, resp, data, opts, selected)
	return resp, nil
}

// addRetentionGap warns when the store's earliest node sample is more than
// one step after the window start. Missing data is a gap, not zero usage.
func addRetentionGap(resp *pbc.GetStatsResponse, vec model.Vector, from time.Time) {
	if len(vec) == 0 {
		return
	}
	start := time.Unix(int64(vec[0].Value), 0).UTC()
	if !start.After(from.Add(missingStepsOK * time.Minute)) {
		return
	}
	resp.Warnings = append(resp.Warnings, fmt.Sprintf(
		"%s stored data starts at %s, after the window start %s; Prometheus retention may be shorter than the window",
		identity.IncompletePrefix, start.Format(time.RFC3339), from.UTC().Format(time.RFC3339)))
}

func clusterValues(vec model.Vector) []string {
	var values []string
	for _, sample := range vec {
		if cluster := string(sample.Metric["cluster"]); cluster != "" {
			values = append(values, cluster)
		}
	}
	return values
}
