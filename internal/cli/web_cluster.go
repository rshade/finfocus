package cli

import (
	"context"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/webui"
)

// cluster answers the dashboard's `cost cluster` query with the session's open
// plugins and query engine, through the same builder the CLI calls. It holds
// the query gate for the whole run, so a plugin reload cannot swap the clients
// or the engine underneath it.
func (d *webData) cluster(ctx context.Context, query webui.ClusterQuery) (engine.ClusterReport, error) {
	in, err := parseClusterQuery(ctx, webClusterQuery(query))
	if err != nil {
		return engine.ClusterReport{}, err
	}
	eng, release, err := d.queryEngine(ctx)
	if err != nil {
		return engine.ClusterReport{}, err
	}
	defer release()
	return buildClusterReport(ctx, d.clients, eng, in)
}

// clusterPolicy answers the dashboard's `cost cluster --show-policy` query. It
// needs no engine, only the query gate that keeps the clients from changing.
func (d *webData) clusterPolicy(ctx context.Context, query webui.ClusterPolicyQuery) (engine.ClusterPolicy, error) {
	in, err := parseClusterQuery(ctx, webClusterQuery(webui.ClusterQuery{
		Allocator: query.Allocator, Policy: query.Policy,
	}))
	if err != nil {
		return engine.ClusterPolicy{}, err
	}
	unlock, err := d.lock(ctx)
	if err != nil {
		return engine.ClusterPolicy{}, err
	}
	defer unlock()
	return buildClusterPolicy(ctx, d.clients, in)
}

// defaultClusterGroupBy is the grouping `cost cluster` uses when none is given.
const defaultClusterGroupBy = "namespace"

// webClusterQuery is a dashboard query in the CLI's terms. An omitted field
// takes the flag's default; policy is text, and a discovered policy file is
// named by its tier, never its path.
func webClusterQuery(q webui.ClusterQuery) clusterQuery {
	groupBy := q.GroupBy
	if groupBy == "" {
		groupBy = defaultClusterGroupBy
	}
	return clusterQuery{
		context: q.Context, namespace: q.Namespace, selectors: q.Selector, groupBy: groupBy,
		from: q.From, to: q.To, usageSource: q.UsageSource, allocator: q.Allocator,
		policyText: &q.Policy, policyByTier: true,
	}
}
