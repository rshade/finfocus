package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

const expansionClusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"

// clusterExpansionRows returns a cluster row with two projected workload
// children, grouped the way engine.ExpandClustersProjected produces them.
func clusterExpansionRows() []engine.OverviewRow {
	rows := []engine.OverviewRow{
		{
			URN:    expansionClusterURN,
			Type:   "aws:eks/cluster:Cluster",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 73.00,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api",
			Type:   "kubernetes:apps/v1:Deployment",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 54.75,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::worker",
			Type:   "kubernetes:apps/v1:Deployment",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 27.38,
				Currency:    "USD",
			},
		},
	}
	return engine.ExpandClustersProjected(rows)
}

// listGoldenWidth is the terminal width used for list-view golden tests; the
// 9-column overview table needs more than the detail-view goldenWidth (100)
// to keep the Resource and Type columns readable.
const listGoldenWidth = 120

// newListModel builds an OverviewModel in ViewStateList at the golden width.
func newListModel(t *testing.T, rows []engine.OverviewRow) OverviewModel {
	t.Helper()
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, rows, len(rows), nil, nil)
	model.state = ViewStateList
	model.width = listGoldenWidth
	model.allRows = rows
	model.rows = rows
	model.rebuildTable()
	return model
}

func TestGolden_ListView_ClusterCollapsed(t *testing.T) {
	t.Parallel()

	model := newListModel(t, clusterExpansionRows())
	output := model.renderListView()
	testGolden(t, "overview_collapsed_cluster", output)
}

func TestGolden_ListView_ClusterExpanded(t *testing.T) {
	t.Parallel()

	model := newListModel(t, clusterExpansionRows())
	model.expanded[expansionClusterURN] = true
	model.rebuildTable()
	output := model.renderListView()
	testGolden(t, "overview_expanded_cluster", output)
}

func TestOverviewClusterExpansion_Keys(t *testing.T) {
	t.Parallel()

	pressKey := func(m OverviewModel, msg tea.KeyPressMsg) OverviewModel {
		updated, _ := m.Update(msg)
		return updated.(OverviewModel)
	}

	t.Run("e toggles expansion on a cluster row", func(t *testing.T) {
		t.Parallel()
		model := newListModel(t, clusterExpansionRows())
		// Sort by cost puts the cluster (73.00) first.
		require.Equal(
			t,
			expansionClusterURN,
			model.rows[model.displayIndex(model.table.Cursor())].URN,
		)

		assert.Len(t, model.displayEntries(), 1, "collapsed: children hidden")

		model = pressKey(model, tea.KeyPressMsg{Text: "e"})
		assert.True(t, model.expanded[expansionClusterURN])
		assert.Len(t, model.displayEntries(), 3, "expanded: children visible")

		model = pressKey(model, tea.KeyPressMsg{Text: "e"})
		assert.False(t, model.expanded[expansionClusterURN])
		assert.Len(t, model.displayEntries(), 1)
	})

	t.Run("right expands and left collapses", func(t *testing.T) {
		t.Parallel()
		model := newListModel(t, clusterExpansionRows())
		model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyRight})
		assert.True(t, model.expanded[expansionClusterURN])
		model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyLeft})
		assert.False(t, model.expanded[expansionClusterURN])
	})

	t.Run("e on a non-cluster row is a no-op", func(t *testing.T) {
		t.Parallel()
		rows := clusterExpansionRows()
		model := newListModel(t, rows)
		model.expanded[expansionClusterURN] = true
		model.rebuildTable()
		model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyDown}) // first child row
		require.Equal(t, "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api",
			model.rows[model.displayIndex(model.table.Cursor())].URN)
		model = pressKey(model, tea.KeyPressMsg{Text: "e"})
		assert.True(t, model.expanded[expansionClusterURN], "child row must not toggle expansion")
	})

	t.Run("enter on an expanded child opens its detail view", func(t *testing.T) {
		t.Parallel()
		model := newListModel(t, clusterExpansionRows())
		model.expanded[expansionClusterURN] = true
		model.rebuildTable()
		model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyDown}) // child: api
		model = pressKey(model, tea.KeyPressMsg{Code: tea.KeyEnter})
		assert.Equal(t, ViewStateDetail, model.state)
		selected := model.rows[model.selected]
		assert.Equal(t, "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api", selected.URN)
	})
}

func TestOverviewExpansionReadyMsg(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{URN: expansionClusterURN, Type: "aws:eks/cluster:Cluster", Status: engine.StatusActive},
	}
	model := newListModel(t, rows)

	expanded := engine.ExpandClustersProjected(append(rows,
		engine.OverviewRow{
			URN:    "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api",
			Type:   "kubernetes:apps/v1:Deployment",
			Status: engine.StatusActive,
		},
	))
	notes := []string{"live cluster data preferred for prod-cluster"}
	updated, _ := model.Update(OverviewExpansionReadyMsg{Rows: expanded, Notes: notes})
	model = updated.(OverviewModel)

	require.Len(t, model.allRows, 2)
	assert.Equal(t, notes, model.expansionNotes)
	view := model.renderListView()
	assert.Contains(t, view, "† live cluster data preferred for prod-cluster")
}
