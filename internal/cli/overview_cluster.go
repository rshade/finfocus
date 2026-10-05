package cli

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/tui"
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

// resolveClusterScope decides the usage-source scope (kubeconfig context)
// for a cluster row. Precedence (FR-012): explicit `overview.cluster_contexts`
// config mapping (full URN wins over bare name), then the cluster's `name`
// property or the name segment of its `arn`, then — single-cluster stacks
// only — the plugin's current context, flagged assumed so the caller can
// footnote it.
func resolveClusterScope(
	row engine.OverviewRow, clusterCount int, cfg *config.Config,
) (string, bool) {
	name := engine.ClusterRowName(row)
	if cfg != nil {
		if mapped := cfg.Overview.ResolveClusterContext(name, row.URN); mapped != "" {
			return mapped, false
		}
	}
	hasIdentity := false
	if n, ok := row.Properties["name"].(string); ok && n != "" {
		hasIdentity = true
	}
	if a, ok := row.Properties["arn"].(string); ok && a != "" {
		hasIdentity = true
	}
	if hasIdentity {
		return name, false
	}
	if clusterCount == 1 {
		return "", true
	}
	return name, false
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

	var clusterIdx []int
	for i := range rows {
		if rows[i].ParentURN == "" && engine.IsClusterResource(rows[i].Type) {
			clusterIdx = append(clusterIdx, i)
		}
	}
	if len(clusterIdx) == 0 {
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
		ctx, rows, clusterIdx,
		pbc.NewUsageSourceServiceClient(usageClient.Conn),
		pbc.NewAllocatorServiceClient(allocClient.Conn),
		pricer, policy.JSON, cfg,
	)
}

// runLiveExpansion runs RunClusterAllocation for each cluster row and applies
// the result. Per-cluster failures downgrade to the projected expansion with
// a logged warning (FR-007). Split from expandClustersLive so tests can drive
// it with fake UsageSource/Allocator implementations.
func runLiveExpansion(
	ctx context.Context,
	rows []engine.OverviewRow,
	clusterIdx []int,
	usage engine.UsageSource,
	alloc engine.Allocator,
	pricer engine.ResourcePricer,
	policyJSON []byte,
	cfg *config.Config,
) ([]engine.OverviewRow, []string) {
	log := logging.FromContext(ctx)

	var notes []string
	liveApplied := false
	for _, idx := range clusterIdx {
		cluster := rows[idx]
		scope, assumed := resolveClusterScope(cluster, len(clusterIdx), cfg)
		name := engine.ClusterRowName(cluster)

		res, err := engine.RunClusterAllocation(ctx, usage, alloc, pricer, engine.ClusterRequest{
			Scope:      scope,
			PolicyJSON: policyJSON,
		})
		if err != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "cli").
				Str("operation", "overview_cluster_expansion").
				Str("cluster", name).
				Err(err).
				Msg("live cluster expansion failed; keeping projected view")
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
		if assumed {
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

// sendClusterExpansionToTUI runs cluster expansion after enrichment and, when
// the row set changed, notifies the TUI via OverviewExpansionReadyMsg. It
// returns the expanded rows, or the original rows when nothing changed.
func sendClusterExpansionToTUI(
	ctx context.Context,
	p *tea.Program,
	rows []engine.OverviewRow,
	clients []*pluginhost.Client,
	pricer engine.ResourcePricer,
	cfg *config.Config,
) []engine.OverviewRow {
	expandedRows, notes := expandOverviewClusters(ctx, rows, clients, pricer, cfg)
	if !overviewRowsExpanded(expandedRows) {
		return rows
	}
	p.Send(tui.OverviewExpansionReadyMsg{Rows: expandedRows, Notes: notes})
	return expandedRows
}
