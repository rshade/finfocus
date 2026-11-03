package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func spotRows() []engine.OverviewRow {
	onDemand := 57.63
	return []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::batch-worker",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusCreating,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost:               21.90,
				Currency:                  "USD",
				PricingCategory:           engine.PricingCategoryDynamic,
				SpotInterruptionRisk:      0.12,
				OnDemandEquivalentMonthly: &onDemand,
				SpotSavingsPercent:        engine.SpotSavingsPercent(engine.PricingCategoryDynamic, 21.90, &onDemand),
			},
		},
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::cache-node",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusCreating,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost:     73.00,
				Currency:        "USD",
				PricingCategory: engine.PricingCategoryStandard,
			},
		},
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::unscored",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusCreating,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost:     30.00,
				Currency:        "USD",
				PricingCategory: engine.PricingCategoryDynamic,
			},
		},
	}
}

func TestGolden_ListView_SpotRows(t *testing.T) {
	t.Parallel()

	model := newListModel(t, spotRows())
	output := model.renderListView()
	testGolden(t, "overview_spot_list", output)

	lines := strings.Split(ansi.Strip(output), "\n")
	var spot, plain int
	for _, line := range lines {
		switch {
		case strings.Contains(line, "batch-worker"), strings.Contains(line, "unscored"):
			assert.Contains(t, line, "SPOT")
			spot++
		case strings.Contains(line, "cache-node"):
			assert.NotContains(t, line, "SPOT")
			plain++
		}
	}
	assert.Equal(t, 2, spot)
	assert.Equal(t, 1, plain)
}

func TestGolden_DetailView_SpotResource(t *testing.T) {
	t.Parallel()

	model := newGoldenModel(t, spotRows()[:1])
	testGolden(t, "detail_spot_resource", model.renderDetailViewForDay(goldenDayOfMonth))
}

func TestGolden_DetailView_SpotRiskUnknown(t *testing.T) {
	t.Parallel()

	model := newGoldenModel(t, spotRows()[2:])
	testGolden(t, "detail_spot_risk_unknown", model.renderDetailViewForDay(goldenDayOfMonth))
}

func TestDetailView_NonSpotHasNoPricingLine(t *testing.T) {
	t.Parallel()

	model := newGoldenModel(t, spotRows()[1:2])
	output := ansi.Strip(model.renderDetailViewForDay(goldenDayOfMonth))
	assert.NotContains(t, output, "Pricing:")
	assert.NotContains(t, output, "SPOT")
}

func TestProjectedCell(t *testing.T) {
	t.Parallel()

	rows := computeRowResults(spotRows())
	assert.Equal(t, "$21.90 SPOT", projectedCell(rows[0]))
	assert.Equal(t, "$73.00", projectedCell(rows[1]))
}

func TestSpotColumnWidensOnlyForSpotLists(t *testing.T) {
	t.Parallel()

	spotModel := newListModel(t, spotRows())
	plainModel := newListModel(t, spotRows()[1:2])
	assert.Equal(t, len(spotListSuffix), spotModel.spotColumnExtra())
	assert.Zero(t, plainModel.spotColumnExtra())
}

func TestSpotStyleKeepsTagTextUnderNoColor(t *testing.T) {
	t.Parallel()

	rendered := SpotStyle.Render("SPOT · risk 0.12")
	assert.Equal(t, "SPOT · risk 0.12", ansi.Strip(rendered))
}

func TestCostRowsAndDetailShowSpot(t *testing.T) {
	t.Parallel()

	results := []engine.CostResult{
		{
			ResourceType: "aws:ec2:Instance", ResourceID: "spot", Currency: "USD", Monthly: 21.9,
			Hourly: 0.03, PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
		},
		{ResourceType: "aws:ec2:Instance", ResourceID: "std", Currency: "USD", Monthly: 73, Hourly: 0.1},
	}
	assert.Equal(t, "SPOT · risk 0.12 · spot savings unavailable", NewResourceRow(results[0]).SpotDisplay)
	assert.Empty(t, NewResourceRow(results[1]).SpotDisplay)

	spotDetail := ansi.Strip(RenderDetailView(results[0], 100))
	assert.Contains(t, spotDetail, "Pricing:")
	assert.Contains(t, spotDetail, "SPOT · risk 0.12 · spot savings unavailable")

	onDemand := 57.63
	results[0].OnDemandEquivalentMonthly = &onDemand
	results[0].SpotSavingsPercent = engine.SpotSavingsPercent(results[0].PricingCategory, results[0].Monthly, &onDemand)
	assert.Contains(t, ansi.Strip(RenderDetailView(results[0], 100)), "SPOT · risk 0.12 · -62% vs on-demand")
	assert.NotContains(t, ansi.Strip(RenderDetailView(results[1], 100)), "SPOT")

	table := NewResultTable(results, 10)
	rows := table.Rows()
	require.Len(t, rows, 2)
	assert.Equal(t, "$21.90 SPOT", rows[0][3])
	assert.Equal(t, "$73.00", rows[1][3])
}

func TestRenderCostComparisonWithSpot(t *testing.T) {
	t.Parallel()

	withSpot := ansi.Strip(RenderCostComparisonWithSpot(8, 20, "USD", "", "SPOT · risk unknown"))
	lines := strings.Split(withSpot, "\n")
	require.GreaterOrEqual(t, len(lines), 2)
	assert.NotContains(t, lines[0], "SPOT")
	assert.Contains(t, lines[1], "SPOT · risk unknown")

	assert.Equal(t, RenderCostComparison(8, 20, "USD"), RenderCostComparisonWithSpot(8, 20, "USD", "", ""))
}
