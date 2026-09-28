// Package kubernetes is a finfocus plugin that reports Kubernetes workload
// usage (UsageSourceService) and allocates node costs to workloads
// (AllocatorService).
package kubernetes

import (
	"context"
	"fmt"
	"maps"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/labels"
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
	sel, err := labelSelector(selector)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid label selector: %v", err))
	}
	return usage.Collect(ctx, cluster.Client, usage.Options{
		Cluster:       cluster.Context,
		Namespace:     ns,
		LabelSelector: sel,
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
