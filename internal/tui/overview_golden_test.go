package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// goldenDayOfMonth is the fixed day used for all golden file tests so that
// delta / cost-impact calculations produce deterministic output regardless
// of the actual date the tests are run.
const goldenDayOfMonth = 15

// goldenWidth is the terminal width used for golden file tests.
const goldenWidth = 100

// newGoldenModel creates a minimal OverviewModel in ViewStateDetail with
// fixed width for deterministic golden file output.
func newGoldenModel(t *testing.T, rows []engine.OverviewRow) OverviewModel {
	t.Helper()
	// Pre-populate ComputedDelta with a fixed day for deterministic output.
	engine.PopulateComputedDeltas(rows, goldenDayOfMonth)
	ctx := context.Background()
	model, _ := NewOverviewModel(ctx, rows, len(rows), nil, nil)
	model.state = ViewStateDetail
	model.selected = 0
	model.width = goldenWidth
	model.allRows = computeRowResults(rows)
	model.rows = computeRowResults(rows)
	return model
}

// TestGolden_DetailView_ActiveResource verifies the detail view for an active
// resource with both actual and projected cost data.
func TestGolden_DetailView_ActiveResource(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{
				MTDCost:  75.50,
				Currency: "USD",
				Period:   period,
				Breakdown: map[string]float64{
					"compute": 60.00,
					"storage": 15.50,
				},
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
				Breakdown: map[string]float64{
					"compute": 120.00,
					"storage": 30.00,
				},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_active_resource", output)
}

// TestGolden_DetailView_UpdatingResource verifies the detail view for a resource
// being updated, including property changes and cost impact.
func TestGolden_DetailView_UpdatingResource(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusUpdating,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "instanceType", OldValue: "t3.medium", NewValue: "t3.large"},
				{Key: "rootBlockDevice", OldValue: "20", NewValue: "50"},
			},
			ActualCost: &engine.ActualCostData{
				MTDCost:  50.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_updating_resource", output)
}

// TestGolden_DetailView_CreatingResource verifies the detail view for a new
// resource being created with projected cost and cost impact.
func TestGolden_DetailView_CreatingResource(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:s3/bucket:Bucket::data-lake",
			Type:   "aws:s3/bucket:Bucket",
			Status: engine.StatusCreating,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 33.87,
				Currency:    "USD",
				Breakdown: map[string]float64{
					"storage":  25.00,
					"requests": 8.87,
				},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_creating_resource", output)
}

// TestGolden_DetailView_DeletingResource verifies the detail view for a resource
// being deleted with actual cost and cost impact (savings).
func TestGolden_DetailView_DeletingResource(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::legacy",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusDeleting,
			ActualCost: &engine.ActualCostData{
				MTDCost:  45.00,
				Currency: "USD",
				Period:   period,
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_deleting_resource", output)
}

// TestGolden_DetailView_ReplacingResource verifies the detail view for a resource
// being replaced, including property changes and cost impact.
func TestGolden_DetailView_ReplacingResource(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::api-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusReplacing,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "ami", OldValue: "ami-old12345", NewValue: "ami-new67890"},
				{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.xlarge"},
			},
			ActualCost: &engine.ActualCostData{
				MTDCost:  30.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 120.00,
				Currency:    "USD",
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_replacing_resource", output)
}

// TestGolden_DetailView_WithRecommendations verifies the detail view with
// active recommendations displayed.
func TestGolden_DetailView_WithRecommendations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	period := engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now}

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::overprovisioned",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{
				MTDCost:  200.00,
				Currency: "USD",
				Period:   period,
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 400.00,
				Currency:    "USD",
			},
			Recommendations: []engine.Recommendation{
				{
					Type:             "Right-sizing",
					Description:      "Downsize from m5.2xlarge to m5.large based on CPU utilization (avg 12%)",
					EstimatedSavings: 150.00,
					Currency:         "USD",
					Status:           engine.RecommendationStatusActive,
				},
				{
					Type:             "Purchase Commitment",
					Description:      "Purchase 1-year reserved instance for 30% savings",
					EstimatedSavings: 120.00,
					Currency:         "USD",
					Status:           engine.RecommendationStatusActive,
				},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_with_recommendations", output)
}

// TestGolden_DetailView_WithError verifies the detail view when a resource
// has an error condition.
func TestGolden_DetailView_WithError(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:rds/instance:Instance::database",
			Type:   "aws:rds/instance:Instance",
			Status: engine.StatusActive,
			Error: &engine.OverviewRowError{
				URN:       "urn:pulumi:prod::myapp::aws:rds/instance:Instance::database",
				ErrorType: engine.ErrorTypeNetwork,
				Message:   "connection refused: dial tcp 10.0.1.5:443: i/o timeout",
				Retryable: true,
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_with_error", output)
}

// TestGolden_ListView_SourceWarningWraps verifies a long source warning wraps
// to the terminal width and shrinks the table by the rows it occupies.
func TestGolden_ListView_SourceWarningWraps(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::web-server",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 150.00,
				Currency:    "USD",
			},
		},
	}
	const warning = "preview-only resources could not be fully inventoried because the " +
		"pending preview was incomplete, so totals may omit resources that are still being created"

	base := newListModel(t, rows)
	base.height = 40
	base.rebuildTable()

	model := newListModel(t, rows)
	model.height = 40
	model.sourceWarning = warning
	model.rebuildTable()

	require.Greater(t, model.sourceWarningRows(), 1, "warning must wrap")
	assert.Equal(t, base.table.Height()-model.sourceWarningRows(), model.table.Height())
	testGolden(t, "overview_source_warning_wrapped", model.renderListView())
}

const blastGoldenPrefix = "urn:pulumi:prod::myapp::"

func blastGoldenRow(kind, name string, monthly float64) engine.OverviewRow {
	return engine.OverviewRow{
		URN:           blastGoldenPrefix + kind + "::" + name,
		Type:          kind,
		Status:        engine.StatusActive,
		ProjectedCost: &engine.ProjectedCostData{MonthlyCost: monthly, Currency: "USD"},
	}
}

func blastGoldenChain() ([]engine.OverviewRow, engine.DependencyGraph) {
	rows := []engine.OverviewRow{
		blastGoldenRow("aws:rds/instance:Instance", "orders-db", 182.50),
		blastGoldenRow("aws:ecs/service:Service", "orders-api", 61.20),
		blastGoldenRow("aws:lb/loadBalancer:LoadBalancer", "public-lb", 18.40),
	}
	graph := engine.DependencyGraph{
		Available: true,
		Nodes: map[string]engine.DependencyNode{
			rows[0].URN: {URN: rows[0].URN},
			rows[1].URN: {URN: rows[1].URN, DependsOn: []string{rows[0].URN}},
			rows[2].URN: {URN: rows[2].URN, DependsOn: []string{rows[1].URN}},
		},
	}
	return rows, graph
}

// TestGolden_DetailView_BlastRadius verifies the BLAST RADIUS section for a
// populated tree, a resource nothing depends on, and an unavailable graph.
func TestGolden_DetailView_BlastRadius(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		golden   string
		selected int
		graph    func(engine.DependencyGraph) engine.DependencyGraph
	}{
		{name: "populated tree", golden: "detail_blast_radius_tree", selected: 0},
		{name: "nothing depends on it", golden: "detail_blast_radius_empty", selected: 2},
		{
			name:   "graph unavailable",
			golden: "detail_blast_radius_unavailable",
			graph:  func(engine.DependencyGraph) engine.DependencyGraph { return engine.DependencyGraph{} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows, graph := blastGoldenChain()
			if tt.graph != nil {
				graph = tt.graph(graph)
			}
			model := newGoldenModel(t, rows)
			model.selected = tt.selected
			model.graph = graph
			model.graphLoaded = true
			output := model.renderDetailViewForDay(goldenDayOfMonth)
			testGolden(t, tt.golden, output)
		})
	}
}

// TestGolden_DetailView_BlastRadiusMidChange verifies the markers for a new,
// a deleting, an unpriced and a component dependent, and the left-out note.
func TestGolden_DetailView_BlastRadiusMidChange(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		blastGoldenRow("aws:rds/instance:Instance", "orders-db", 182.50),
		blastGoldenRow("aws:ecs/service:Service", "orders-api", 61.20),
		blastGoldenRow("aws:elasticache/cluster:Cluster", "orders-cache", 44.00),
		blastGoldenRow("aws:sqs/queue:Queue", "orders-queue", 0),
		blastGoldenRow("aws:cloudwatch/dashboard:Dashboard", "orders-dash", 0),
		blastGoldenRow("demo:index:Group", "orders-group", 0),
	}
	rows[1].Status = engine.StatusCreating
	rows[2].Status = engine.StatusDeleting
	rows[4].ProjectedCost = nil
	graph := engine.DependencyGraph{
		Available: true,
		Nodes: map[string]engine.DependencyNode{
			rows[0].URN: {URN: rows[0].URN},
			rows[1].URN: {URN: rows[1].URN, DependsOn: []string{rows[0].URN}},
			rows[2].URN: {URN: rows[2].URN, DependsOn: []string{rows[0].URN}},
			rows[3].URN: {URN: rows[3].URN, DependsOn: []string{rows[1].URN}},
			rows[4].URN: {URN: rows[4].URN, DependsOn: []string{rows[1].URN}},
			rows[5].URN: {URN: rows[5].URN, DependsOn: []string{rows[0].URN}, Component: true},
		},
	}
	model := newGoldenModel(t, rows)
	model.graph = graph
	model.graphLoaded = true

	testGolden(t, "detail_blast_radius_midchange", model.renderDetailViewForDay(goldenDayOfMonth))
}

// TestGolden_DetailView_BlastRadiusAfterPreview verifies that the section
// follows the rows and the graph after a preview on a state-only run.
func TestGolden_DetailView_BlastRadiusAfterPreview(t *testing.T) {
	t.Parallel()

	rows, graph := blastGoldenChain()
	stateGraph := engine.DependencyGraph{
		Available: true,
		Nodes: map[string]engine.DependencyNode{
			rows[0].URN: graph.Nodes[rows[0].URN],
			rows[1].URN: graph.Nodes[rows[1].URN],
			rows[2].URN: {URN: rows[2].URN},
		},
	}
	model := newGoldenModel(t, rows)
	model.state = ViewStateList
	model.isStateOnly = true
	model.graph = stateGraph
	model.graphLoaded = true
	before := model.renderDetailViewForDay(goldenDayOfMonth)
	require.Contains(t, before, "1 dependent, $61.20/month")

	updated, _ := model.Update(OverviewChangesReadyMsg{
		StatusByURN: map[string]engine.ResourceStatus{rows[1].URN: engine.StatusReplacing},
	})
	updated, _ = updated.(OverviewModel).Update(OverviewGraphReadyMsg{Graph: graph})
	model = updated.(OverviewModel)
	model.state = ViewStateDetail
	model.selected = 0

	testGolden(t, "detail_blast_radius_preview", model.renderDetailViewForDay(goldenDayOfMonth))
}

func TestBlastRadiusCopy(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "nothing depends on this resource", strings.TrimSpace(blastEmptyMsg))
	assert.Equal(t, "dependency information is not available", strings.TrimSpace(blastUnavailableMsg))
}

// TestDetailView_BlastRadiusScrollKeepsSummaryWithHeading checks that on a tall
// tree the count and total sit on the line right under the heading, so
// scrolling to the section always brings both into view together.
func TestDetailView_BlastRadiusScrollKeepsSummaryWithHeading(t *testing.T) {
	t.Parallel()

	const dependents = 40
	rows := []engine.OverviewRow{blastGoldenRow("aws:rds/instance:Instance", "orders-db", 100)}
	nodes := map[string]engine.DependencyNode{rows[0].URN: {URN: rows[0].URN}}
	for i := range dependents {
		row := blastGoldenRow("aws:ecs/service:Service", fmt.Sprintf("svc-%02d", i), 2)
		rows = append(rows, row)
		nodes[row.URN] = engine.DependencyNode{URN: row.URN, DependsOn: []string{rows[0].URN}}
	}
	model := newGoldenModel(t, rows)
	model.selected = 0
	model.graph = engine.DependencyGraph{Available: true, Nodes: nodes}
	model.graphLoaded = true

	content := model.renderDetailViewForDay(goldenDayOfMonth)
	lines := strings.Split(content, "\n")
	heading := -1
	for i, line := range lines {
		if strings.Contains(line, "BLAST RADIUS") {
			heading = i
			break
		}
	}
	require.GreaterOrEqual(t, heading, 0, "the section must be present")
	require.Greater(t, len(lines), 24, "the tree must be taller than the window")

	shown, offset := visibleDetail(content, heading, 12)
	require.Equal(t, heading, offset)
	shownLines := strings.Split(shown, "\n")
	assert.Contains(t, shownLines[0], "BLAST RADIUS")
	assert.Contains(t, shownLines[1], "40 dependents, $80.00/month")
	assert.Contains(t, shown, "svc-")
}
