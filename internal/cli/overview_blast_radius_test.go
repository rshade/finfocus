package cli

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

const (
	blastDBURN  = "urn:pulumi:prod::myapp::aws:rds/instance:Instance::orders-db"
	blastAPIURN = "urn:pulumi:prod::myapp::aws:ecs/service:Service::orders-api"
	blastLBURN  = "urn:pulumi:prod::myapp::aws:lb/loadBalancer:LoadBalancer::public-lb"
)

func blastRadiusFixture(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", "blast_radius_state.json"))
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.NoError(t, err)
	return path
}

// detailViews drives the real pipeline into the overview model and returns the
// rendered detail view of every row, keyed by URN.
func detailViews(t *testing.T) map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	cmd, _ := overviewCmd(nil)
	pipe := newOverviewPipeline(overviewPipelineConfig{
		cmd: cmd,
		params: overviewParams{
			pulumiState: blastRadiusFixture(t),
			stateOnly:   true,
			cfg:         &config.Config{},
		},
		dateRange: engine.DateRange{Start: time.Now().Add(-72 * time.Hour), End: time.Now()},
		audit:     newAuditContext(ctx, "overview", nil),
		loader:    tuiOverviewLoader,
	})

	var graph engine.DependencyGraph
	var graphSent, dataSent bool
	var rows []engine.OverviewRow
	pipe.OnGraph = func(g engine.DependencyGraph) { graph, graphSent = g, true }
	pipe.OnDataReady = func(r []engine.OverviewRow, _ int, _ string) {
		rows, dataSent = r, true
	}
	require.NoError(t, pipe.Run(ctx))
	t.Cleanup(func() {
		if cleanup := pipe.Cleanup(); cleanup != nil {
			cleanup()
		}
	})
	require.True(t, graphSent, "pipeline must publish the graph")
	require.True(t, dataSent)

	views := make(map[string]string, len(rows))
	for i, row := range rows {
		model := newBlastModel(ctx, rows, graph)
		for range i {
			model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		views[row.URN] = model.View().Content
	}
	return views
}

func newBlastModel(ctx context.Context, rows []engine.OverviewRow, graph engine.DependencyGraph) tea.Model {
	var model tea.Model
	model, _ = tui.NewOverviewModel(ctx, nil, 0, nil, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	model, _ = model.Update(tui.OverviewGraphReadyMsg{Graph: graph})
	model, _ = model.Update(tui.OverviewDataReadyMsg{Rows: rows, TotalCount: len(rows), StackName: "prod"})
	model, _ = model.Update(tui.OverviewAllResourcesLoadedMsg{})
	return model
}

// Not parallel: sets FINFOCUS_HOME and the global config.
func TestOverviewBlastRadius_DetailView(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	views := detailViews(t)
	require.Len(t, views, 3, "the stack component is not an overview row")

	db := views[blastDBURN]
	assert.Contains(t, db, "BLAST RADIUS")
	assert.Contains(t, db, "2 dependents")
	assert.Contains(t, db, "orders-api (aws:ecs/service:Service)")
	assert.Contains(t, db, "public-lb (aws:lb/loadBalancer:LoadBalancer)")

	assert.Contains(t, views[blastAPIURN], "1 dependent")
	assert.Contains(t, views[blastAPIURN], "public-lb")
	assert.NotContains(t, views[blastAPIURN], "orders-db (")

	assert.Contains(t, views[blastLBURN], "nothing depends on this resource")
}

// FR-011: opening a detail view prices nothing. The model is built with no
// engine and no plugin clients, so it can render the section only from rows
// and the graph it already holds.
// Not parallel: sets FINFOCUS_HOME and the global config.
func TestOverviewBlastRadius_DetailNeedsNoPricing(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	cost := &engine.ProjectedCostData{MonthlyCost: 12, Currency: "USD"}
	rows := []engine.OverviewRow{
		{URN: blastDBURN, Type: "aws:rds/instance:Instance", Status: engine.StatusActive, ProjectedCost: cost},
		{URN: blastAPIURN, Type: "aws:ecs/service:Service", Status: engine.StatusActive, ProjectedCost: cost},
	}
	graph := engine.DependencyGraph{Available: true, Nodes: map[string]engine.DependencyNode{
		blastDBURN:  {URN: blastDBURN},
		blastAPIURN: {URN: blastAPIURN, DependsOn: []string{blastDBURN}},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	model := newBlastModel(ctx, rows, graph)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := model.View().Content
	assert.Contains(t, view, "BLAST RADIUS")
	assert.Contains(t, view, "1 dependent, $12.00/month")
}

var blastTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T[0-9:.]+(?:Z|[-+]\d{2}:\d{2})`)

// FR-013: the non-interactive formats are byte-identical to what they were
// before the blast radius existed. The goldens were generated by the code
// before this feature; UPDATE_GOLDEN=1 rewrites them.
// Not parallel: sets FINFOCUS_HOME and the global config.
func TestOverviewNonInteractiveOutputUnchanged(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	for _, format := range []string{outputFormatTable, outputFormatJSON, outputFormatNDJSON} { //nolint:paralleltest // subtests share the process-wide config and FINFOCUS_HOME set above
		t.Run(format, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			t.Cleanup(cancel)
			cmd, out := overviewCmd(strings.NewReader(""))
			cmd.SetContext(ctx)
			require.NoError(t, executeOverview(cmd, overviewParams{
				pulumiState: blastRadiusFixture(t),
				stateOnly:   true,
				plain:       true,
				yes:         true,
				output:      format,
				cfg:         &config.Config{},
			}))
			got := blastTimestamp.ReplaceAllString(out.String(), "<time>")
			assert.NotContains(t, got, "BLAST RADIUS")
			assert.NotContains(t, got, "blastRadius")

			golden := filepath.Join("testdata", "blast_radius_output_"+format+".golden")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				require.NoError(t, os.WriteFile(golden, []byte(got), 0o600))
			}
			want, err := os.ReadFile(golden)
			require.NoError(t, err, "golden %s missing; run UPDATE_GOLDEN=1", golden)
			assert.Equal(t, string(want), got)
		})
	}
}
