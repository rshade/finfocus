package config

import (
	"errors"
	"fmt"
	"maps"
	"strings"
)

const overviewKeyClusterContexts = "cluster_contexts"

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

// getOverviewValue reads overview.<key>. A cluster_contexts entry is
// addressed by everything after `cluster_contexts.`, so a key may itself
// contain dots.
func (c *Config) getOverviewValue(parts []string) (any, error) {
	if len(parts) < 1 {
		return c.Overview, nil
	}
	if parts[0] != overviewKeyClusterContexts {
		return nil, fmt.Errorf("unknown overview setting: %s", parts[0])
	}
	var contexts map[string]string
	if c.Overview != nil {
		contexts = c.Overview.ClusterContexts
	}
	if len(parts) == 1 {
		return contexts, nil
	}
	cluster := strings.Join(parts[1:], ".")
	ctx, ok := contexts[cluster]
	if !ok {
		return nil, fmt.Errorf("no overview.cluster_contexts entry for %q", cluster)
	}
	return ctx, nil
}

// setOverviewValue sets overview.cluster_contexts.<cluster> to a kubeconfig
// context. An empty value removes the entry.
func (c *Config) setOverviewValue(parts []string, value string) error {
	if len(parts) < 2 || parts[0] != overviewKeyClusterContexts {
		return errors.New("overview key required (e.g. overview.cluster_contexts.<cluster>)")
	}
	cluster := strings.Join(parts[1:], ".")
	next := OverviewConfig{}
	if c.Overview != nil {
		next.ClusterContexts = maps.Clone(c.Overview.ClusterContexts)
	}
	if value == "" {
		delete(next.ClusterContexts, cluster)
	} else {
		if next.ClusterContexts == nil {
			next.ClusterContexts = map[string]string{}
		}
		next.ClusterContexts[cluster] = value
	}
	c.Overview = &next
	return nil
}
