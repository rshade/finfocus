package cli

import (
	"context"
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
	fromStr     string
	toStr       string
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
	f.StringVar(&params.context, "context", "", "Kubeconfig context (default: current context)")
	f.StringVar(&params.namespace, "namespace", "",
		"Only allocate workloads in this namespace (omits idle and cluster rows)")
	f.StringArrayVar(&params.selectors, "selector", nil, "Pod label selector key=value (repeatable)")
	f.StringVar(&params.groupBy, "group-by", "namespace",
		"Group by: namespace, controller, pod, node, pulumi-stack, label:<key>")
	f.StringVar(&params.policyPath, "policy", "",
		"Allocation policy file (default: project or global allocation.hujson)")
	f.BoolVar(&params.showPolicy, "show-policy", false, "Print the effective allocation policy and exit")
	f.StringVar(&params.usageSource, "usage-source", "", "Usage-source plugin to use when several are installed")
	f.StringVar(&params.allocator, "allocator", "", "Allocator plugin to use when several are installed")
	f.StringVar(&params.output, "output", "",
		"Output format: table, json, or ndjson (default from output.default_format, else table)")
	f.StringVar(&params.fromStr, "from", "", "Window start (2006-01-02 or RFC3339)")
	f.StringVar(&params.toStr, "to", "", "Window end (2006-01-02 or RFC3339); defaults to now when --from is set")
	return cmd
}

func runCostCluster(cmd *cobra.Command, params costClusterParams) error {
	ctx := cmd.Context()
	format := resolveOutputFormatWithConfig(cmd, params.output)
	if !isValidOutputFormat(engine.OutputFormat(format)) {
		return unsupportedOutputError(ctx, format, "table, json, ndjson")
	}
	// Every check before openPlugins is user input, so each exits 2. A
	// policy file is the user's, whether named by --policy or discovered.
	if err := engine.ValidateClusterGroupBy(params.groupBy); err != nil {
		return toValidationError(ctx, err)
	}
	selector, err := parseSelectors(params.selectors)
	if err != nil {
		return toValidationError(ctx, err)
	}
	from, to, err := resolveClusterWindow(params.fromStr, params.toStr)
	if err != nil {
		return toValidationError(ctx, err)
	}
	policy, err := config.ResolveAllocationPolicy(ctx, params.policyPath)
	if err != nil {
		return toValidationError(ctx, err)
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
			return pluginSelectionError(ctx, selErr)
		}
		effective, digest, showErr := engine.ShowAllocationPolicy(ctx,
			pbc.NewAllocatorServiceClient(allocClient.Conn), policy.JSON)
		if showErr != nil {
			return policyError(ctx, showErr, allocClient.Name, policy)
		}
		policyOut.Digest, policyOut.Effective = digest, effective
		return renderPolicy(cmd, format, policyOut)
	}

	usage, usageName, alloc, err := clusterPlugins(ctx, clients, params, policy)
	if err != nil {
		return err
	}
	cfg := config.GetGlobalConfig()
	eng, _, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(cfg.SpecDir), cfg)
	defer cacheCleanup()
	if profileErr := applyUsageProfile(cmd, eng, cfg); profileErr != nil {
		return toValidationError(ctx, profileErr)
	}

	res, err := engine.RunClusterAllocation(
		ctx,
		usage,
		alloc,
		eng,
		engine.ClusterRequest{
			Scope:      params.context,
			Namespace:  params.namespace,
			Selector:   selector,
			PolicyJSON: policy.JSON,
			From:       from,
			To:         to,
		},
	)
	if err != nil {
		return clusterRunError(ctx, err, usageName)
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

// clusterPlugins selects the usage source (and its name, empty for a set) and
// the allocator for a cluster run. A policy file is the user's input, so the
// allocator checks it with the same usage-free call --show-policy makes,
// before any usage is gathered or priced: a bad policy fails fast and cannot
// be confused with an allocator rejecting the plugins' data.
func clusterPlugins(
	ctx context.Context, clients []*pluginhost.Client, params costClusterParams, policy config.AllocationPolicy,
) (engine.UsageSource, string, engine.Allocator, error) {
	usage, usageName, err := selectUsageSource(clients, params.usageSource)
	if err != nil {
		return nil, "", nil, pluginSelectionError(ctx, err)
	}
	allocClient, err := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, params.allocator)
	if err != nil {
		return nil, "", nil, pluginSelectionError(ctx, err)
	}
	alloc := pbc.NewAllocatorServiceClient(allocClient.Conn)
	if len(policy.JSON) > 0 {
		if _, _, checkErr := engine.ShowAllocationPolicy(ctx, alloc, policy.JSON); checkErr != nil {
			return nil, "", nil, policyError(ctx, checkErr, allocClient.Name, policy)
		}
	}
	return usage, usageName, alloc, nil
}

// clusterRunError maps a RunClusterAllocation failure. Two failures are the
// user's to fix, so they exit 2: several installed usage sources serving the
// requested mode (choose with --usage-source), and a usage source answering
// InvalidArgument, because the GetStats request holds only --context,
// --namespace, --selector, and the window. usageName names the source in that
// message when one was selected. Other failures, including an allocator
// rejecting a request that carries the plugins' usage and prices, exit 1.
func clusterRunError(ctx context.Context, err error, usageName string) error {
	if ambiguous, ok := errors.AsType[*engine.AmbiguousUsageSourceError](err); ok {
		return toValidationError(ctx, fmt.Errorf("%w; choose one with --usage-source", ambiguous))
	}
	if rejected, ok := errors.AsType[*engine.UsageRequestError](err); ok {
		return toValidationError(ctx, &engine.UsageRequestError{Source: usageName, Err: rejected.Err})
	}
	return err
}

// policyError maps a failure of the usage-free allocator call. An allocator
// rejecting a policy file the user supplied exits 2 and names the allocator
// and the file; the built-in default policy is not the user's, so a rejection
// of it stays exit 1 like any other allocator failure.
func policyError(ctx context.Context, err error, allocator string, policy config.AllocationPolicy) error {
	rejected, ok := errors.AsType[*engine.PolicyRejectedError](err)
	if !ok || len(policy.JSON) == 0 {
		return err
	}
	return toValidationError(ctx, &engine.PolicyRejectedError{
		Allocator: allocator, Source: policy.Source, Err: rejected.Err,
	})
}

// pluginChoiceError reports a --usage-source or --allocator value that names no
// capable plugin, or several capable plugins and no flag to choose between
// them. The flag fixes both. No capable plugin installed at all is not one.
type pluginChoiceError struct{ msg string }

func (e *pluginChoiceError) Error() string { return e.msg }

// pluginSelectionError makes a *pluginChoiceError exit 2; other selection failures
// pass through.
func pluginSelectionError(ctx context.Context, err error) error {
	if _, ok := errors.AsType[*pluginChoiceError](err); ok {
		return toValidationError(ctx, err)
	}
	return err
}

// selectUsageSource returns the usage source for a cluster run and its plugin
// name: the explicit --usage-source plugin, the only installed one, or, when
// several are installed, an engine.UsageSourceSet (name empty) that asks each
// one and uses the single source that accepts the request's mode.
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
		return pbc.NewUsageSourceServiceClient(client.Conn), client.Name, nil
	}
	return engine.NewUsageSourceSet(sources), "", nil
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
		return nil, fmt.Errorf("no installed plugin provides %s; install one with: finfocus plugin install kubernetes",
			capability)
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
