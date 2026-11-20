package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// requestPolicySource labels allocation policy text a dashboard request
// supplied, in errors and in the report's policy source.
const requestPolicySource = "request body"

// clusterQuery holds `cost cluster` inputs as the flags and the dashboard
// query carry them. policyPath is the CLI's --policy file; the dashboard sends
// policyText instead, so the server never opens a path the page names. A nil
// or blank policyText falls back to the discovered policy. policyByTier names
// a discovered policy file by its tier instead of its path, because the
// dashboard never shows server paths.
type clusterQuery struct {
	context      string
	namespace    string
	selectors    []string
	groupBy      string
	from         string
	to           string
	usageSource  string
	allocator    string
	policyPath   string
	policyText   *string
	policyByTier bool
}

// clusterInput is a validated clusterQuery.
type clusterInput struct {
	scope       string
	namespace   string
	selector    map[string]string
	groupBy     string
	from        time.Time
	to          time.Time
	usageSource string
	allocator   string
	policy      config.AllocationPolicy
}

// parseClusterQuery validates a query before any plugin is involved. Every
// failure is the caller's input, a *engine.ClusterInputError.
func parseClusterQuery(ctx context.Context, q clusterQuery) (clusterInput, error) {
	if err := engine.ValidateClusterGroupBy(q.groupBy); err != nil {
		return clusterInput{}, &engine.ClusterInputError{Err: err}
	}
	selector, err := parseSelectors(q.selectors)
	if err != nil {
		return clusterInput{}, &engine.ClusterInputError{Err: err}
	}
	from, to, err := resolveClusterWindow(q.from, q.to)
	if err != nil {
		return clusterInput{}, &engine.ClusterInputError{Err: err}
	}
	policy, err := resolveClusterPolicy(ctx, q)
	if err != nil {
		return clusterInput{}, &engine.ClusterInputError{Err: err}
	}
	return clusterInput{
		scope: q.context, namespace: q.namespace, selector: selector, groupBy: q.groupBy, from: from, to: to,
		usageSource: q.usageSource, allocator: q.allocator, policy: policy,
	}, nil
}

// resolveClusterPolicy reads policy text when the query carries some, and
// otherwise the --policy file or the discovered one. A policy file is the
// user's, whether named or discovered.
func resolveClusterPolicy(ctx context.Context, q clusterQuery) (config.AllocationPolicy, error) {
	switch {
	case q.policyText != nil && strings.TrimSpace(*q.policyText) != "":
		return config.ParseAllocationPolicy(ctx, strings.NewReader(*q.policyText), requestPolicySource)
	case q.policyByTier:
		return config.ResolveDiscoveredAllocationPolicy(ctx)
	default:
		return config.ResolveAllocationPolicy(ctx, q.policyPath)
	}
}

// buildClusterReport runs one cluster allocation with already open clients and
// returns the `cost cluster` JSON document. The CLI and the dashboard both
// call it. A failure the caller can fix is a *engine.ClusterInputError, and a
// missing plugin capability is a *engine.ClusterPluginMissingError.
func buildClusterReport(
	ctx context.Context, clients []*pluginhost.Client, pricer engine.ResourcePricer, in clusterInput,
) (engine.ClusterReport, error) {
	usage, usageName, alloc, err := clusterPlugins(ctx, clients, in)
	if err != nil {
		return engine.ClusterReport{}, err
	}
	res, err := engine.RunClusterAllocation(ctx, usage, alloc, pricer, engine.ClusterRequest{
		Scope:      in.scope,
		Namespace:  in.namespace,
		Selector:   in.selector,
		PolicyJSON: in.policy.JSON,
		From:       in.from,
		To:         in.to,
	})
	if err != nil {
		return engine.ClusterReport{}, clusterRunError(err, usageName)
	}
	groups, err := engine.GroupClusterRows(res.Rows, in.groupBy)
	if err != nil {
		return engine.ClusterReport{}, err
	}
	policy := engine.ClusterPolicy{
		Source: policySourceLabel(in.policy.Source), Digest: res.PolicyDigest, Effective: res.EffectivePolicy,
	}
	return engine.NewClusterReport(res, groups, in.groupBy, policy), nil
}

// buildClusterPolicy asks the allocator for its effective policy without
// gathering usage: the `--show-policy` document.
func buildClusterPolicy(
	ctx context.Context, clients []*pluginhost.Client, in clusterInput,
) (engine.ClusterPolicy, error) {
	allocClient, err := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, in.allocator)
	if err != nil {
		return engine.ClusterPolicy{}, pluginSelectionError(err)
	}
	effective, digest, err := engine.ShowAllocationPolicy(ctx, clusterAllocator(allocClient), in.policy.JSON)
	if err != nil {
		return engine.ClusterPolicy{}, policyError(err, allocClient.Name, in.policy)
	}
	return engine.ClusterPolicy{Source: policySourceLabel(in.policy.Source), Digest: digest, Effective: effective}, nil
}

// clusterPlugins selects the usage source (and its name, empty for a set) and
// the allocator for a cluster run. A policy file is the user's input, so the
// allocator checks it with the same usage-free call --show-policy makes,
// before any usage is gathered or priced: a bad policy fails fast and cannot
// be confused with an allocator rejecting the plugins' data.
func clusterPlugins(
	ctx context.Context, clients []*pluginhost.Client, in clusterInput,
) (engine.UsageSource, string, engine.Allocator, error) {
	usage, usageName, err := selectUsageSource(clients, in.usageSource)
	if err != nil {
		return nil, "", nil, pluginSelectionError(err)
	}
	allocClient, err := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, in.allocator)
	if err != nil {
		return nil, "", nil, pluginSelectionError(err)
	}
	alloc := clusterAllocator(allocClient)
	if len(in.policy.JSON) > 0 {
		if _, _, checkErr := engine.ShowAllocationPolicy(ctx, alloc, in.policy.JSON); checkErr != nil {
			return nil, "", nil, policyError(checkErr, allocClient.Name, in.policy)
		}
	}
	return usage, usageName, alloc, nil
}

// clusterRunError types a RunClusterAllocation failure. Two failures are the
// caller's to fix: several installed usage sources serving the requested mode
// (choose with --usage-source), and a usage source answering InvalidArgument,
// because the GetStats request holds only the context, namespace, selector,
// and window. usageName names the source in that message when one was
// selected. Other failures, including an allocator rejecting a request that
// carries the plugins' usage and prices, are not.
func clusterRunError(err error, usageName string) error {
	if ambiguous, ok := errors.AsType[*engine.AmbiguousUsageSourceError](err); ok {
		return &engine.ClusterInputError{Err: fmt.Errorf("%w; choose one with --usage-source", ambiguous)}
	}
	if rejected, ok := errors.AsType[*engine.UsageRequestError](err); ok {
		return &engine.ClusterInputError{Err: &engine.UsageRequestError{Source: usageName, Err: rejected.Err}}
	}
	return err
}

// policyError types a failure of the usage-free allocator call. An allocator
// rejecting a policy the caller supplied is input and names the allocator and
// the policy source; the built-in default policy is not the caller's, so a
// rejection of it stays an ordinary allocator failure.
func policyError(err error, allocator string, policy config.AllocationPolicy) error {
	rejected, ok := errors.AsType[*engine.PolicyRejectedError](err)
	if !ok || len(policy.JSON) == 0 {
		return err
	}
	return &engine.ClusterInputError{Err: &engine.PolicyRejectedError{
		Allocator: allocator, Source: policy.Source, Err: rejected.Err,
	}}
}

// pluginSelectionError types a *pluginChoiceError as input; other selection
// failures, such as a missing capability, pass through.
func pluginSelectionError(err error) error {
	if _, ok := errors.AsType[*pluginChoiceError](err); ok {
		return &engine.ClusterInputError{Err: err}
	}
	return err
}

// clusterCommandError maps a builder error to the CLI's exit codes: input
// exits 2 with the inner message (an *ax.Error, such as a policy syntax error,
// keeps its own code), and anything else exits 1.
func clusterCommandError(ctx context.Context, err error) error {
	if input, ok := errors.AsType[*engine.ClusterInputError](err); ok {
		return toValidationError(ctx, input.Err)
	}
	return err
}
