// Package kubernetes is a finfocus plugin that reports Kubernetes workload
// usage (UsageSourceService), allocates node costs to workloads
// (AllocatorService), and estimates the projected cost of workloads declared
// in a plan from their resource requests and configured rates.
package kubernetes

import (
	"context"
	"fmt"
	"maps"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/plugins/kubernetes/allocate"
	"github.com/rshade/finfocus/plugins/kubernetes/usage"
	"github.com/rshade/finfocus/plugins/kubernetes/workload"
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

// Plugin serves GetStats, Allocate, and projected cost for declared workloads.
type Plugin struct {
	*pluginsdk.BasePlugin

	clusters ClusterFactory
	config   Config
}

// New builds the plugin. Projected pricing reads only cfg and the request's
// attributes; it never uses clusters.
func New(clusters ClusterFactory, cfg Config) *Plugin {
	return &Plugin{BasePlugin: pluginsdk.NewBasePlugin(PluginName), clusters: clusters, config: cfg}
}

// Info declares capabilities explicitly. Projected cost is the only pricing
// capability; Supports declines every resource it cannot price.
func Info(version string) *pluginsdk.PluginInfo {
	return pluginsdk.NewPluginInfo(PluginName, version,
		pluginsdk.WithProviders("kubernetes"),
		pluginsdk.WithCapabilities(
			pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
			pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
			pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
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
	sel, err := labelSelector(selector)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid label selector: %v", err))
	}
	resp, err := usage.Collect(ctx, cluster.Client, usage.Options{
		Cluster:       cluster.Context,
		Namespace:     ns,
		LabelSelector: sel,
		APIServerHost: cluster.Host,
	})
	if err != nil {
		return nil, err
	}
	applyMetricFilter(resp, req.GetMetrics())
	return resp, nil
}

// servedMetric reports whether name is a metric this run-rate source reports.
// The usage metrics (cpu_usage, mem_usage) are not implemented.
func servedMetric(name string) bool {
	switch name {
	case pluginsdk.MetricCPURequest, pluginsdk.MetricMemRequest,
		pluginsdk.MetricCPUAllocatable, pluginsdk.MetricMemAllocatable:
		return true
	default:
		return false
	}
}

// applyMetricFilter enforces the GetStats metrics contract: an empty request
// list keeps the source's defaults; a non-empty list filters rows to the
// requested metrics and reports each unknown name once in warnings. Priceable
// node entries whose node no longer has a row are dropped so the response
// stays valid for plugintesting.ValidateStatsResponse.
func applyMetricFilter(resp *pbc.GetStatsResponse, requested []string) {
	if len(requested) == 0 {
		return
	}
	wanted := make(map[string]bool, len(requested))
	reported := make(map[string]bool, len(requested))
	for _, name := range requested {
		if servedMetric(name) {
			wanted[name] = true
			continue
		}
		if !reported[name] {
			reported[name] = true
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("unknown metric %q ignored", name))
		}
	}
	rows := make([]*pbc.UsageRow, 0, len(resp.GetRows()))
	nodes := make(map[string]bool)
	for _, row := range resp.GetRows() {
		if !wanted[row.GetMetric()] {
			continue
		}
		rows = append(rows, row)
		if node := row.GetSubject()[pluginsdk.SubjectNode]; node != "" {
			nodes[node] = true
		}
	}
	resp.Rows = rows
	priceable := make([]*pbc.ResourceDescriptor, 0, len(resp.GetPriceable()))
	for _, d := range resp.GetPriceable() {
		if d.GetTags()[pluginsdk.SubjectKind] == pluginsdk.KindNode && !nodes[d.GetId()] {
			continue
		}
		priceable = append(priceable, d)
	}
	resp.Priceable = priceable
}

// Allocate splits priced resources across workloads.
func (p *Plugin) Allocate(_ context.Context, req *pbc.AllocateRequest) (*pbc.AllocateResponse, error) {
	return allocate.Allocate(req)
}

// Supports answers yes exactly when the workload can be priced. A decline
// reason is the only plugin text core keeps in a NO_COST_DATA note, so every
// reason names its fix.
func (p *Plugin) Supports(_ context.Context, req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	estimate := workload.Estimate(req.GetResource(), p.config)
	return &pbc.SupportsResponse{Supported: estimate.Priced(), Reason: estimate.Reason}, nil
}

// GetProjectedCost prices a declared workload. A decline is only reachable
// when a host skipped Supports, and returns FailedPrecondition with the reason.
// The price depends on this process's environment, which a host's cache key
// cannot see, so expires_at is now: hosts must not reuse it after a rate change.
func (p *Plugin) GetProjectedCost(
	_ context.Context, req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	estimate := workload.Estimate(req.GetResource(), p.config)
	if !estimate.Priced() {
		return nil, status.Error(codes.FailedPrecondition, estimate.Reason)
	}
	return &pbc.GetProjectedCostResponse{
		CostPerMonth:  estimate.Monthly,
		UnitPrice:     estimate.PodHourly,
		Currency:      "USD",
		BillingDetail: estimate.Note,
		ExpiresAt:     timestamppb.Now(),
	}, nil
}

// labelSelector renders key=value pairs in sorted order as a Kubernetes label
// selector, validating every key and value client-side. It returns an error
// naming the offending key rather than silently emitting a selector string
// corrupted by unescaped `,` or `=` in a value.
func labelSelector(m map[string]string) (string, error) {
	selector, err := labels.ValidatedSelectorFromSet(labels.Set(m))
	if err != nil {
		return "", err
	}
	return selector.String(), nil
}
