package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/spec"
)

type costClusterParams struct {
	context     string
	namespace   string
	selectors   []string
	groupBy     string
	policyPath  string
	showPolicy  bool
	usageSource string
	allocator   string
	output      string
}

// NewCostClusterCmd creates `finfocus cost cluster`.
func NewCostClusterCmd() *cobra.Command {
	var params costClusterParams
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Break down Kubernetes cluster cost by namespace, controller, pod, node, or label",
		Long: `Allocates the monthly cost of a cluster's nodes (and control plane) to the
workloads running on them, using a usage-source plugin for requests and an
allocator plugin for the split. Idle capacity is reported as its own row.`,
		Example: `  finfocus cost cluster
  finfocus cost cluster --context prod --group-by controller
  finfocus cost cluster --namespace payments --output json
  finfocus cost cluster --group-by label:team --policy ./allocation.hujson
  finfocus cost cluster --show-policy`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCostCluster(cmd, params)
		},
	}
	f := cmd.Flags()
	f.StringVar(&params.context, "context", "", "Kubeconfig context (default: current context)")
	f.StringVar(&params.namespace, "namespace", "",
		"Only allocate workloads in this namespace (omits idle and cluster rows)")
	f.StringArrayVar(&params.selectors, "selector", nil, "Pod label selector key=value (repeatable)")
	f.StringVar(&params.groupBy, "group-by", "namespace", "Group by: namespace, controller, pod, node, label:<key>")
	f.StringVar(&params.policyPath, "policy", "",
		"Allocation policy file (default: project or global allocation.hujson)")
	f.BoolVar(&params.showPolicy, "show-policy", false, "Print the effective allocation policy and exit")
	f.StringVar(&params.usageSource, "usage-source", "", "Usage-source plugin to use when several are installed")
	f.StringVar(&params.allocator, "allocator", "", "Allocator plugin to use when several are installed")
	f.StringVar(&params.output, "output", config.GetDefaultOutputFormat(), "Output format (table, json, ndjson)")
	return cmd
}

func runCostCluster(cmd *cobra.Command, params costClusterParams) error {
	ctx := cmd.Context()
	format := config.GetOutputFormat(resolveOutputFormat(cmd, "output", params.output))
	if !isValidOutputFormat(engine.OutputFormat(format)) {
		return fmt.Errorf("unsupported output format: %s (supported: table, json, ndjson)", format)
	}
	if err := engine.ValidateClusterGroupBy(params.groupBy); err != nil {
		return err
	}
	selector, err := parseSelectors(params.selectors)
	if err != nil {
		return err
	}
	policy, err := config.ResolveAllocationPolicy(ctx, params.policyPath)
	if err != nil {
		return err
	}

	clients, cleanup, err := openPlugins(ctx, "", nil)
	if err != nil {
		return err
	}
	defer cleanup()

	policyOut := clusterPolicyOutput{Source: policySourceLabel(policy.Source)}

	if params.showPolicy {
		allocClient, selErr := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, params.allocator)
		if selErr != nil {
			return selErr
		}
		effective, digest, showErr := engine.ShowAllocationPolicy(ctx,
			pbc.NewAllocatorServiceClient(allocClient.Conn), policy.JSON)
		if showErr != nil {
			return showErr
		}
		policyOut.Digest, policyOut.Effective = digest, effective
		return renderPolicy(cmd, format, policyOut)
	}

	usageClient, err := selectCapablePlugin(clients, pluginhost.CapabilityUsageStats, params.usageSource)
	if err != nil {
		return err
	}
	allocClient, err := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, params.allocator)
	if err != nil {
		return err
	}
	allocator := pbc.NewAllocatorServiceClient(allocClient.Conn)
	cfg := config.GetGlobalConfig()
	eng, _, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(cfg.SpecDir), cfg)
	defer cacheCleanup()

	res, err := engine.RunClusterAllocation(
		ctx,
		pbc.NewUsageSourceServiceClient(usageClient.Conn),
		allocator,
		eng,
		engine.ClusterRequest{
			Scope:      params.context,
			Namespace:  params.namespace,
			Selector:   selector,
			PolicyJSON: policy.JSON,
		},
	)
	if err != nil {
		return err
	}
	groups, err := engine.GroupClusterRows(res.Rows, params.groupBy)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		cmd.PrintErrln("warning: " + w)
	}
	policyOut.Digest, policyOut.Effective = res.PolicyDigest, res.EffectivePolicy
	return renderClusterResult(cmd, format, newClusterOutput(res, groups, params.groupBy, policyOut))
}

// selectCapablePlugin picks the plugin serving capability: the explicit name,
// or the only installed plugin that declares it.
func selectCapablePlugin(clients []*pluginhost.Client, capability, explicit string) (*pluginhost.Client, error) {
	var names []string
	var candidates []*pluginhost.Client
	for _, c := range clients {
		if c.HasCapability(capability) {
			candidates = append(candidates, c)
			names = append(names, c.Name)
		}
	}
	slices.Sort(names)
	flag := map[string]string{
		pluginhost.CapabilityUsageStats: "--usage-source",
		pluginhost.CapabilityAllocation: "--allocator",
	}[capability]
	if explicit != "" {
		for _, c := range candidates {
			if c.Name == explicit {
				return c, nil
			}
		}
		return nil, fmt.Errorf("plugin %q is not installed or lacks %s (available: %s)",
			explicit, capability, strings.Join(names, ", "))
	}
	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("no installed plugin provides %s; install one with: finfocus plugin install kubernetes",
			capability)
	case 1:
		return candidates[0], nil
	default:
		return nil, fmt.Errorf("several plugins provide %s (%s); choose one with %s",
			capability, strings.Join(names, ", "), flag)
	}
}

func parseSelectors(values []string) (map[string]string, error) {
	out := make(map[string]string, len(values))
	for _, v := range values {
		k, val, ok := strings.Cut(v, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --selector %q: want key=value", v)
		}
		out[k] = val
	}
	return out, nil
}

func policySourceLabel(source string) string {
	if source == "" {
		return "built-in defaults"
	}
	return source
}
