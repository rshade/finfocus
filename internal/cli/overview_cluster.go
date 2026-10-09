package cli

import (
	"context"
	"fmt"
	"slices"

	"github.com/rs/zerolog"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// expandOverviewClusters groups declared workload rows under the stack's
// cluster row (projected expansion), then attempts live allocation per
// cluster when usage-source and allocator plugins are installed (live
// expansion). Live data wins when both are available; suppressed projected
// rows are reported via the returned notes. Expansion is never fatal: any
// failure downgrades to the projected path or to no expansion.
func expandOverviewClusters(
	ctx context.Context,
	rows []engine.OverviewRow,
	clients []*pluginhost.Client,
	pricer engine.ResourcePricer,
	cfg *config.Config,
) ([]engine.OverviewRow, []string) {
	rows = engine.ExpandClustersProjected(rows)
	return expandClustersLive(ctx, rows, clients, pricer, cfg)
}

// clusterScope is one kubeconfig context to try for a cluster row. An empty
// scope asks the usage source for its current context.
type clusterScope struct {
	scope    string
	assumed  bool
	explicit bool
}

// clusterScopeCandidates lists the kubeconfig contexts to try for a cluster
// row, in FR-012 precedence order. An explicit `overview.cluster_contexts`
// mapping (full URN wins over bare name) is the only candidate when present.
// Otherwise the cluster's `name` property, its full `arn` (the context name
// `aws eks update-kubeconfig` writes), and the ARN's name segment are tried,
// and, for single-cluster stacks only, the current context, flagged assumed.
// A multi-cluster stack row with no identity gets no candidates and is not
// expanded live.
func clusterScopeCandidates(
	row engine.OverviewRow, clusterCount int, cfg *config.Config,
) []clusterScope {
	if cfg != nil {
		if mapped := cfg.Overview.ResolveClusterContext(engine.ClusterRowName(row), row.URN); mapped != "" {
			return []clusterScope{{scope: mapped, explicit: true}}
		}
	}
	var names []string
	if n, ok := row.Properties["name"].(string); ok && n != "" {
		names = append(names, n)
	}
	if a, ok := row.Properties["arn"].(string); ok && a != "" {
		names = append(names, a)
		if seg := engine.ClusterARNName(a); seg != "" {
			names = append(names, seg)
		}
	}
	var candidates []clusterScope
	seen := map[string]bool{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			candidates = append(candidates, clusterScope{scope: n})
		}
	}
	if clusterCount == 1 {
		candidates = append(candidates, clusterScope{assumed: true})
	}
	return candidates
}

// expandClustersLive attempts live expansion for each top-level cluster row.
// Rows and notes are returned unchanged when no usage-source/allocator pair
// is installed or when selection is ambiguous.
func expandClustersLive(
	ctx context.Context,
	rows []engine.OverviewRow,
	clients []*pluginhost.Client,
	pricer engine.ResourcePricer,
	cfg *config.Config,
) ([]engine.OverviewRow, []string) {
	log := logging.FromContext(ctx)

	if !slices.ContainsFunc(rows, engine.IsExpandableCluster) {
		return rows, nil
	}

	usageClient, usageErr := selectCapablePlugin(clients, pluginhost.CapabilityUsageStats, "")
	allocClient, allocErr := selectCapablePlugin(clients, pluginhost.CapabilityAllocation, "")
	if usageErr != nil || allocErr != nil {
		// Not installed (or ambiguous): projected expansion (if any) stands.
		log.Debug().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_cluster_expansion").
			AnErr("usage_err", usageErr).
			AnErr("alloc_err", allocErr).
			Msg("live cluster expansion unavailable; using projected data")
		return rows, nil
	}

	policy, policyErr := config.ResolveAllocationPolicy(ctx, "")
	if policyErr != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_cluster_expansion").
			Err(policyErr).
			Msg("allocation policy resolution failed; using plugin defaults")
		policy = config.AllocationPolicy{}
	}

	return runLiveExpansion(
		ctx, rows,
		pbc.NewUsageSourceServiceClient(usageClient.Conn),
		pbc.NewAllocatorServiceClient(allocClient.Conn),
		pricer, policy.JSON, cfg,
	)
}

// runLiveExpansion runs RunClusterAllocation for each expandable cluster row,
// trying its scope candidates in order, and applies the first result. When
// every candidate fails the cluster keeps its projected expansion; the
// failure is a warning only for an explicitly mapped context (FR-007). Split
// from expandClustersLive so tests can drive it with fake UsageSource and
// Allocator implementations.
func runLiveExpansion(
	ctx context.Context,
	rows []engine.OverviewRow,
	usage engine.UsageSource,
	alloc engine.Allocator,
	pricer engine.ResourcePricer,
	policyJSON []byte,
	cfg *config.Config,
) ([]engine.OverviewRow, []string) {
	log := logging.FromContext(ctx)

	clusters, clusterCount := liveExpansionClusters(rows)
	var notes []string
	liveApplied := false
	for _, cluster := range clusters {
		name := engine.ClusterRowName(cluster)
		res, used, ok := allocateCluster(
			ctx, cluster, name, clusterScopeCandidates(cluster, clusterCount, cfg),
			usage, alloc, pricer, policyJSON,
		)
		if !ok {
			continue
		}
		children, childErr := engine.LiveChildrenFromResult(cluster.URN, res)
		if childErr != nil || len(children) == 0 {
			if childErr != nil {
				log.Warn().
					Ctx(ctx).
					Str("component", "cli").
					Str("operation", "overview_cluster_expansion").
					Str("cluster", name).
					Err(childErr).
					Msg("live allocation grouping failed; keeping projected view")
			}
			continue
		}

		var suppressed int
		rows, suppressed = engine.ApplyLiveExpansion(rows, cluster.URN, children)
		liveApplied = true
		if suppressed > 0 {
			notes = append(notes, fmt.Sprintf(
				"live cluster data preferred for %s; %s hidden to avoid double counting",
				name, pluralize(suppressed, "projected workload row", "projected workload rows"),
			))
		}
		if used.assumed {
			notes = append(notes, fmt.Sprintf(
				"kubeconfig context for %s assumed from the current context", name,
			))
		}
	}
	if liveApplied {
		notes = append(notes,
			"live allocation rows re-allocate node cost shown by other rows; excluded from summary")
	}
	return rows, notes
}

// liveExpansionClusters returns the clusters eligible for live expansion and
// the number of top-level clusters in the stack. It runs before any expansion
// because ApplyLiveExpansion reorders rows.
func liveExpansionClusters(rows []engine.OverviewRow) ([]engine.OverviewRow, int) {
	var clusters []engine.OverviewRow
	count := 0
	for i := range rows {
		if rows[i].ParentURN == "" && engine.IsClusterResource(rows[i].Type) {
			count++
			if engine.IsExpandableCluster(rows[i]) {
				clusters = append(clusters, rows[i])
			}
		}
	}
	return clusters, count
}

// allocateCluster tries each scope candidate until RunClusterAllocation
// succeeds. Failed candidates are logged at debug, except an explicit config
// mapping, whose failure is a warning because the user asked for it.
func allocateCluster(
	ctx context.Context,
	cluster engine.OverviewRow,
	name string,
	candidates []clusterScope,
	usage engine.UsageSource,
	alloc engine.Allocator,
	pricer engine.ResourcePricer,
	policyJSON []byte,
) (*engine.ClusterResult, clusterScope, bool) {
	log := logging.FromContext(ctx)
	for _, c := range candidates {
		res, err := engine.RunClusterAllocation(ctx, usage, alloc, pricer, engine.ClusterRequest{
			Scope:      c.scope,
			PolicyJSON: policyJSON,
		})
		if err == nil {
			return res, c, true
		}
		level := zerolog.DebugLevel
		if c.explicit {
			level = zerolog.WarnLevel
		}
		log.WithLevel(level).
			Ctx(ctx).
			Str("component", "cli").
			Str("operation", "overview_cluster_expansion").
			Str("cluster", name).
			Str("urn", cluster.URN).
			Str("scope", c.scope).
			Err(err).
			Msg("live cluster expansion failed for scope; keeping projected view")
	}
	return nil, clusterScope{}, false
}

// pluralize returns "1 singular" or "N plural".
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// overviewRowsExpanded reports whether any row participates in cluster
// expansion, so the TUI only receives an expansion message when something
// changed.
func overviewRowsExpanded(rows []engine.OverviewRow) bool {
	for i := range rows {
		if rows[i].ParentURN != "" || len(rows[i].ChildURNs) > 0 {
			return true
		}
	}
	return false
}
