package config

// OverviewConfig configures the `finfocus overview` command.
type OverviewConfig struct {
	// ClusterContexts maps a Pulumi-declared Kubernetes cluster (by cluster
	// name or full URN) to the kubeconfig context used for live cost
	// allocation. When a cluster has no entry, its `name` property (or the
	// name segment of its ARN) is used as the context name.
	ClusterContexts map[string]string `yaml:"cluster_contexts,omitempty" json:"cluster_contexts,omitempty"`
}

// ResolveClusterContext returns the configured kubeconfig context for a
// cluster name or URN, or "" when no mapping exists. The full URN wins over
// the bare cluster name when both are configured.
func (o *OverviewConfig) ResolveClusterContext(name, urn string) string {
	if o == nil {
		return ""
	}
	if ctx, ok := o.ClusterContexts[urn]; ok {
		return ctx
	}
	if ctx, ok := o.ClusterContexts[name]; ok {
		return ctx
	}
	return ""
}
