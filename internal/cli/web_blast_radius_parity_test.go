package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/stackload"
	"github.com/rshade/finfocus/internal/webui"
)

func blastParityRows() []engine.OverviewRow {
	row := func(urn, kind string, monthly float64) engine.OverviewRow {
		return engine.OverviewRow{
			URN: urn, Type: kind, Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: monthly, Currency: "USD"},
		}
	}
	return []engine.OverviewRow{
		row(blastDBURN, "aws:rds/instance:Instance", 182.50),
		row(blastAPIURN, "aws:ecs/service:Service", 61.20),
		row(blastLBURN, "aws:lb/loadBalancer:LoadBalancer", 18.40),
	}
}

func fetchWebBlastRadius(t *testing.T, srv *webui.Server, client *http.Client, urn string) engine.BlastRadius {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.BaseURL()+"/api/overview/resource?urn="+url.QueryEscape(urn), nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var detail struct {
		BlastRadius *engine.BlastRadius `json:"blastRadius"`
	}
	require.NoError(t, json.Unmarshal(raw, &detail))
	require.NotNil(t, detail.BlastRadius)
	return *detail.BlastRadius
}

// authedWebClient bootstraps the session cookie from the server's printed URL.
func authedWebClient(t *testing.T, srv *webui.Server) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	resp, err := client.Get(srv.URL())
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return client
}

// Not parallel: sets FINFOCUS_HOME.
func TestWebBlastRadiusMatchesTerminal(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	ctx := context.Background()

	snapshot, err := stackload.Load(ctx, stackload.Source{StatePath: blastRadiusFixture(t), SkipPreview: true})
	require.NoError(t, err)
	graph := snapshot.Graph()
	rows := blastParityRows()

	session := webui.NewSession(ctx, webui.SessionOptions{DayOfMonth: 15})
	session.SetData(nil, rows, engine.DateRange{}, "prod")
	session.SetDependencyGraph(graph)
	srv, err := webui.New(webui.Options{Session: session})
	require.NoError(t, err)
	require.NoError(t, srv.Listen())
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, srv.Shutdown(shutdownCtx))
	})
	client := authedWebClient(t, srv)

	for i, row := range rows {
		web := fetchWebBlastRadius(t, srv, client, row.URN)
		assert.Equal(t, engine.ComputeBlastRadius(graph, rows, row.URN), web, row.URN)

		model := newBlastModel(ctx, rows, graph)
		for range i {
			model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		view := model.View().Content

		if len(web.Dependents) == 0 {
			assert.Contains(t, view, "nothing depends on this resource", row.URN)
			continue
		}
		require.NotNil(t, web.MonthlyTotal, row.URN)
		assert.Contains(t, view, web.SummaryDisplay, row.URN)
		for _, d := range web.Dependents {
			assert.Contains(t, view, fmt.Sprintf("%s (%s)", d.DisplayName, d.Type), row.URN)
			assert.Contains(t, view, d.MonthlyDisplay, row.URN)
		}
	}
}

// Not parallel: sets FINFOCUS_HOME.
func TestWebBlastRadiusRecomputesAfterPreview(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	ctx := context.Background()
	const cacheURN = "urn:pulumi:prod::myapp::aws:elasticache/cluster:Cluster::orders-cache"

	state, err := stackload.Load(ctx, stackload.Source{StatePath: blastRadiusFixture(t), SkipPreview: true})
	require.NoError(t, err)
	stateRows := blastParityRows()

	session := webui.NewSession(ctx, webui.SessionOptions{DayOfMonth: 15})
	session.SetData(nil, stateRows, engine.DateRange{}, "prod")
	data := newWebData(nil, overviewParams{}, engine.DateRange{}, nil)
	data.publishGraph = session.SetDependencyGraph
	require.NoError(t, data.setPipelineSource(ctx, OverviewPipelineResult{Snapshot: state}))
	data.publishSnapshotGraph(state)

	srv, err := webui.New(webui.Options{Session: session})
	require.NoError(t, err)
	require.NoError(t, srv.Listen())
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, srv.Shutdown(shutdownCtx))
	})
	client := authedWebClient(t, srv)

	before := fetchWebBlastRadius(t, srv, client, blastDBURN)
	for _, d := range before.Dependents {
		require.NotEqual(t, cacheURN, d.URN)
	}

	step := ingest.PulumiStep{Op: "create", URN: cacheURN, Type: "aws:elasticache/cluster:Cluster"}
	step.NewState = &ingest.PulumiState{
		URN: cacheURN, Type: "aws:elasticache/cluster:Cluster", Dependencies: []string{blastDBURN},
	}
	previewed := state.WithPlan(&ingest.PulumiPlan{Steps: []ingest.PulumiStep{step}})
	_, err = data.commitPreview(ctx, OverviewPipelineResult{Snapshot: previewed}, stateRows)
	require.NoError(t, err)
	created := engine.OverviewRow{
		URN: cacheURN, Type: "aws:elasticache/cluster:Cluster", Status: engine.StatusCreating,
		ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 40, Currency: "USD"},
	}
	session.SetData(nil, append(blastParityRows(), created), engine.DateRange{}, "prod")

	after := fetchWebBlastRadius(t, srv, client, blastDBURN)
	var found bool
	for _, d := range after.Dependents {
		if d.URN == cacheURN {
			found = true
			assert.Equal(t, 1, d.Depth)
		}
	}
	require.True(t, found, "preview dependent missing from web blast radius")
	assert.Equal(t, before.Count+1, after.Count)
	require.NotNil(t, before.MonthlyTotal)
	require.NotNil(t, after.MonthlyTotal)
	assert.InDelta(t, *before.MonthlyTotal+40, *after.MonthlyTotal, 0.001)
}
