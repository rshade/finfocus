package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

const (
	expansionClusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"
	expansionAPIURN     = "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api"
	expansionWorkerURN  = "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::worker"
)

// loadClusterExpansionRows merges the cluster-expansion fixture into overview
// rows, carrying properties so cluster name/ARN resolution works.
func loadClusterExpansionRows(t *testing.T) []engine.OverviewRow {
	t.Helper()
	ctx := context.Background()

	state, err := ingest.LoadStackExportWithContext(
		ctx, filepath.Join(testdataDir(t), "state-cluster-expansion.json"))
	require.NoError(t, err)

	custom := state.GetCustomResourcesWithContext(ctx)
	stateResources := make([]engine.StateResource, len(custom))
	for i, r := range custom {
		stateResources[i] = engine.StateResource{
			URN:        r.URN,
			Type:       r.Type,
			ID:         r.ID,
			Custom:     r.Custom,
			Properties: ingest.MergeProperties(r.Outputs, r.Inputs),
		}
	}

	rows, err := engine.MergeResourcesForOverview(ctx, stateResources, nil)
	require.NoError(t, err)
	return rows
}

func expansionStackCtx(name string, total int, notes []string) engine.StackContext {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	return engine.StackContext{
		StackName:      name,
		TimeWindow:     engine.DateRange{Start: now.AddDate(0, 0, -15), End: now},
		TotalResources: total,
		GeneratedAt:    now,
		ExpansionNotes: notes,
	}
}

func TestIntegration_ClusterExpansion_Projected(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	ctx := context.Background()

	rows := engine.ExpandClustersProjected(loadClusterExpansionRows(t))

	// Contract invariants (contracts/json-ndjson-expansion.md).
	var cluster *engine.OverviewRow
	childByURN := map[string]engine.OverviewRow{}
	for i := range rows {
		if rows[i].URN == expansionClusterURN {
			cluster = &rows[i]
		}
		if rows[i].ParentURN != "" {
			childByURN[rows[i].URN] = rows[i]
		}
	}
	require.NotNil(t, cluster, "cluster row must exist")
	assert.Equal(t, []string{expansionAPIURN, expansionWorkerURN}, cluster.ChildURNs)
	require.Len(t, childByURN, 2)
	for urn, child := range childByURN {
		assert.Equal(t, expansionClusterURN, child.ParentURN, "child %s parent", urn)
		assert.Equal(t, engine.ExpansionSourceProjected, child.ExpansionSource)
	}

	// Children immediately follow their parent in display order.
	clusterPos, apiPos, workerPos := -1, -1, -1
	for i := range rows {
		switch rows[i].URN {
		case expansionClusterURN:
			clusterPos = i
		case expansionAPIURN:
			apiPos = i
		case expansionWorkerURN:
			workerPos = i
		}
	}
	assert.Equal(t, clusterPos+1, apiPos)
	assert.Equal(t, clusterPos+2, workerPos)

	stackCtx := expansionStackCtx("state-cluster-expansion", len(rows), nil)

	// Golden: plain table.
	var tableBuf bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsTable(&tableBuf, overviewResult(t, rows), stackCtx))
	assertGoldenFile(
		t,
		filepath.Join(goldenDir(t), "table-cluster-expansion.txt"),
		tableBuf.String(),
	)

	// Golden: JSON.
	var jsonBuf bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsJSON(ctx, &jsonBuf, overviewResult(t, rows), stackCtx, nil))
	assertGoldenFile(
		t,
		filepath.Join(goldenDir(t), "json-cluster-expansion.json"),
		jsonBuf.String(),
	)

	var parsed engine.OverviewJSONOutput
	require.NoError(t, json.Unmarshal(jsonBuf.Bytes(), &parsed))
	assert.Len(t, parsed.Resources, len(rows))

	// Golden: NDJSON — parent line first, children immediately after.
	var ndjsonBuf bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsNDJSON(&ndjsonBuf, overviewResult(t, rows)))
	assertGoldenFile(
		t,
		filepath.Join(goldenDir(t), "ndjson-cluster-expansion.ndjson"),
		ndjsonBuf.String(),
	)

	lines := strings.Split(strings.TrimSpace(ndjsonBuf.String()), "\n")
	assert.Len(t, lines, len(rows))
	var lineRows []engine.OverviewRow
	for _, line := range lines {
		var row engine.OverviewRow
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		lineRows = append(lineRows, row)
	}
	assert.Equal(t, rows[0].URN, lineRows[0].URN)
}

// TestIntegration_ClusterExpansion_NoRegression pins FR-008: a cluster row
// without workloads or plugins renders exactly like any other row.
func TestIntegration_ClusterExpansion_NoRegression(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	rows := loadClusterExpansionRows(t)
	// Drop the workload resources: cluster alone must not expand.
	flat := rows[:0]
	for _, r := range rows {
		if engine.IsWorkloadResource(r.Type) {
			continue
		}
		flat = append(flat, r)
	}
	out := engine.ExpandClustersProjected(flat)
	for _, r := range out {
		assert.Empty(t, r.ParentURN)
		assert.Empty(t, r.ChildURNs)
		assert.Empty(t, r.ExpansionSource)
	}

	// Table, JSON, and NDJSON output, compared byte for byte.
	render := func(rows []engine.OverviewRow) []string {
		stackCtx := expansionStackCtx("state-cluster-expansion", len(rows), nil)
		var table, js, nd bytes.Buffer
		require.NoError(t, engine.RenderOverviewAsTable(&table, overviewResult(t, rows), stackCtx))
		require.NoError(t, engine.RenderOverviewAsJSON(context.Background(), &js, overviewResult(t, rows), stackCtx, nil))
		require.NoError(t, engine.RenderOverviewAsNDJSON(&nd, overviewResult(t, rows)))
		return []string{table.String(), js.String(), nd.String()}
	}
	want := render(flat)
	got := render(out)
	assert.Equal(t, want, got)
	gotTable, gotJSON, gotNDJSON := got[0], got[1], got[2]
	assert.NotContains(t, gotJSON, "parentUrn")
	assert.NotContains(t, gotJSON, "childUrns")
	assert.NotContains(t, gotJSON, "expansionSource")
	assert.NotContains(t, gotTable, "↳")
	assert.NotContains(t, gotTable, "†")
	assertGoldenFile(t, filepath.Join(goldenDir(t), "table-cluster-no-expansion.txt"), gotTable)
	assertGoldenFile(t, filepath.Join(goldenDir(t), "json-cluster-no-expansion.json"), gotJSON)
	assertGoldenFile(t, filepath.Join(goldenDir(t), "ndjson-cluster-no-expansion.ndjson"), gotNDJSON)
}

func TestIntegration_ClusterExpansion_Live(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	ctx := context.Background()

	flat := loadClusterExpansionRows(t)
	rows := engine.ExpandClustersProjected(flat)

	live := &engine.ClusterResult{
		Mode:     engine.ModeRunRate,
		Currency: "USD",
		Rows: []engine.ClusterRow{
			{Subject: map[string]string{"kind": "workload", "namespace": "payments"},
				CPUCost: 80.10, MemCost: 40.40, TotalCost: 120.50},
			{Subject: map[string]string{"kind": "workload", "namespace": "default"},
				CPUCost: 10, MemCost: 20, TotalCost: 30},
			{Subject: map[string]string{"kind": "__idle__"}, CPUCost: 5, MemCost: 5, TotalCost: 10},
		},
		Total: 160.50,
		Idle:  10,
	}
	children, err := engine.LiveChildrenFromResult(expansionClusterURN, live)
	require.NoError(t, err)

	expanded, suppressed := engine.ApplyLiveExpansion(rows, expansionClusterURN, children)
	assert.Equal(t, 2, suppressed)

	notes := []string{
		"live cluster data preferred for prod-cluster; 2 projected workload rows hidden to avoid double counting",
		"live allocation rows re-allocate node cost shown by other rows; excluded from summary",
	}
	stackCtx := expansionStackCtx("state-cluster-expansion", len(flat), notes)

	var tableBuf bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsTable(&tableBuf, overviewResult(t, expanded), stackCtx))
	assertGoldenFile(
		t,
		filepath.Join(goldenDir(t), "table-cluster-expansion-live.txt"),
		tableBuf.String(),
	)

	var jsonBuf bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsJSON(ctx, &jsonBuf, overviewResult(t, expanded), stackCtx, nil))
	assertGoldenFile(
		t,
		filepath.Join(goldenDir(t), "json-cluster-expansion-live.json"),
		jsonBuf.String(),
	)

	var parsed engine.OverviewJSONOutput
	require.NoError(t, json.Unmarshal(jsonBuf.Bytes(), &parsed))
	// Live children excluded from summary totals; projected workloads suppressed.
	assert.InDelta(t, 0, parsed.Summary.ProjectedMonthly, 1e-9)
	assert.Equal(t, notes, parsed.Metadata.ExpansionNotes)

	var ndjsonBuf bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsNDJSON(&ndjsonBuf, overviewResult(t, expanded)))
	assertGoldenFile(
		t,
		filepath.Join(goldenDir(t), "ndjson-cluster-expansion-live.ndjson"),
		ndjsonBuf.String(),
	)
}

// overviewResult computes the overview result the renderers consume.
func overviewResult(t *testing.T, rows []engine.OverviewRow) engine.OverviewResult {
	t.Helper()
	result, err := engine.ComputeOverviewResult(rows, 15)
	require.NoError(t, err)
	return result
}
