package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/spec"
)

// clusterCallTimeouts bounds the usage-source and allocator calls of cost
// cluster and overview cluster expansion.
var clusterCallTimeouts = engine.DefaultClusterCallTimeouts() //nolint:gochecknoglobals // tests shorten it

type costClusterParams struct {
	query      clusterQuery
	showPolicy bool
	output     string
}

// NewCostClusterCmd creates `finfocus cost cluster`.
func NewCostClusterCmd() *cobra.Command {
	var params costClusterParams
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Break down Kubernetes cluster cost by namespace, controller, pod, node, or label",
		Long: `Allocates the monthly cost of a cluster's nodes (and control plane) to the
workloads running on them, using a usage-source plugin for requests and an
allocator plugin for the split. Idle capacity is reported as its own row.

With --from and --to, the report prices actual spend over that window
(mode historical) instead of the monthly run rate.`,
		Example: `  finfocus cost cluster
  finfocus cost cluster --context prod --group-by controller
  finfocus cost cluster --namespace payments --output json
  finfocus cost cluster --group-by label:team --policy ./allocation.hujson
  finfocus cost cluster --group-by pulumi-stack
  finfocus cost cluster --from 2026-09-01 --to 2026-09-08
  finfocus cost cluster --show-policy`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCostCluster(cmd, params)
		},
	}
	f := cmd.Flags()
	f.StringVar(&params.query.context, "context", "", "Kubeconfig context (default: current context)")
	f.StringVar(&params.query.namespace, "namespace", "",
		"Only allocate workloads in this namespace (omits idle and cluster rows)")
	f.StringArrayVar(&params.query.selectors, "selector", nil, "Pod label selector key=value (repeatable)")
	f.StringVar(&params.query.groupBy, "group-by", defaultClusterGroupBy,
		"Group by: namespace, controller, pod, node, pulumi-stack, label:<key>")
	f.StringVar(&params.query.policyPath, "policy", "",
		"Allocation policy file (default: project or global allocation.hujson)")
	f.BoolVar(&params.showPolicy, "show-policy", false, "Print the effective allocation policy and exit")
	f.StringVar(&params.query.usageSource, "usage-source", "", "Usage-source plugin to use when several are installed")
	f.StringVar(&params.query.allocator, "allocator", "", "Allocator plugin to use when several are installed")
	f.StringVar(&params.output, "output", "",
		"Output format: table, json, or ndjson (default from output.default_format, else table)")
	f.StringVar(&params.query.from, "from", "", "Window start (2006-01-02 or RFC3339)")
	f.StringVar(&params.query.to, "to", "", "Window end (2006-01-02 or RFC3339); defaults to now when --from is set")
	return cmd
}

func runCostCluster(cmd *cobra.Command, params costClusterParams) error {
	ctx := cmd.Context()
	format := resolveOutputFormatWithConfig(cmd, params.output)
	if !isValidOutputFormat(engine.OutputFormat(format)) {
		return unsupportedOutputError(ctx, format, "table, json, ndjson")
	}
	// Every check before openPlugins is user input, so each exits 2.
	in, err := parseClusterQuery(ctx, params.query)
	if err != nil {
		return clusterCommandError(ctx, err)
	}

	clients, cleanup, err := openPlugins(ctx, "", nil)
	if err != nil {
		return err
	}
	defer cleanup()

	if params.showPolicy {
		policy, policyErr := buildClusterPolicy(ctx, clients, in)
		if policyErr != nil {
			return clusterCommandError(ctx, policyErr)
		}
		return renderPolicy(cmd, format, policy)
	}

	cfg := config.GetGlobalConfig()
	eng, _, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(cfg.SpecDir), cfg)
	defer cacheCleanup()
	if profileErr := applyUsageProfile(cmd, eng, cfg); profileErr != nil {
		return toValidationError(ctx, profileErr)
	}

	report, err := buildClusterReport(ctx, clients, eng, in)
	if err != nil {
		return clusterCommandError(ctx, err)
	}
	printClusterWarnings(cmd, format, report.Warnings)
	return renderClusterResult(cmd, format, report)
}

// pluginChoiceError reports a --usage-source or --allocator value that names no
// capable plugin, or several capable plugins and no flag to choose between
// them. The flag fixes both. No capable plugin installed at all is not one.
type pluginChoiceError struct{ msg string }

func (e *pluginChoiceError) Error() string { return e.msg }

// selectUsageSource returns the usage source for a cluster run and its plugin
// name: the explicit --usage-source plugin, the only installed one, or, when
// several are installed, an engine.UsageSourceSet (name empty) that asks each
// one and uses the single source that accepts the request's mode. Every
// GetStats call is bounded by clusterCallTimeouts.
func selectUsageSource(clients []*pluginhost.Client, explicit string) (engine.UsageSource, string, error) {
	var sources []engine.NamedUsageSource
	for _, c := range clients {
		if c.HasCapability(pluginhost.CapabilityUsageStats) {
			sources = append(sources, engine.NamedUsageSource{
				Name: c.Name, Source: pbc.NewUsageSourceServiceClient(c.Conn),
			})
		}
	}
	if explicit != "" || len(sources) < 2 {
		client, err := selectCapablePlugin(clients, pluginhost.CapabilityUsageStats, explicit)
		if err != nil {
			return nil, "", err
		}
		usage := engine.BoundUsageSource(client.Name, pbc.NewUsageSourceServiceClient(client.Conn), clusterCallTimeouts)
		return usage, client.Name, nil
	}
	return engine.NewUsageSourceSet(sources, clusterCallTimeouts), "", nil
}

// clusterAllocator is the allocator client of a cluster run, each call
// bounded by clusterCallTimeouts.
func clusterAllocator(client *pluginhost.Client) engine.Allocator {
	return engine.BoundAllocator(client.Name, pbc.NewAllocatorServiceClient(client.Conn), clusterCallTimeouts)
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
		return nil, &pluginChoiceError{msg: fmt.Sprintf("plugin %q is not installed or lacks %s (available: %s)",
			explicit, capability, strings.Join(names, ", "))}
	}
	switch len(candidates) {
	case 0:
		return nil, &engine.ClusterPluginMissingError{Capability: capability}
	case 1:
		return candidates[0], nil
	default:
		return nil, &pluginChoiceError{msg: fmt.Sprintf("several plugins provide %s (%s); choose one with %s",
			capability, strings.Join(names, ", "), flag)}
	}
}

// resolveClusterWindow parses --from and --to before any plugin is loaded.
// Both empty is a run-rate request. --to defaults to now when only --from
// is set. --from is required when --to is set.
func resolveClusterWindow(fromStr, toStr string) (time.Time, time.Time, error) {
	if fromStr == "" && toStr == "" {
		return time.Time{}, time.Time{}, nil
	}
	if fromStr == "" {
		return time.Time{}, time.Time{}, errors.New("--from is required when --to is set")
	}
	return ParseTimeRange(fromStr, defaultToNow(toStr))
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
