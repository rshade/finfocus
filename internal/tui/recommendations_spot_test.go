package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func spotRec(risk float64) engine.Recommendation {
	return engine.Recommendation{
		ID:                   "rec-spot",
		ResourceID:           "i-spot",
		Type:                 "RECOMMENDATION_ACTION_TYPE_MODIFY",
		Description:          "Move batch-worker to spot capacity",
		EstimatedSavings:     62,
		Currency:             "USD",
		PricingCategory:      engine.PricingCategoryDynamic,
		SpotInterruptionRisk: risk,
	}
}

func TestRenderRecommendation_SpotLabel(t *testing.T) {
	t.Parallel()

	row := ansi.Strip(renderRecommendation(spotRec(0.12)))
	assert.Contains(t, row, "$62.00  SPOT · risk 0.12  Move", "the label sits between savings and description")

	unknown := ansi.Strip(renderRecommendation(spotRec(0)))
	assert.Contains(t, unknown, "SPOT · risk unknown")

	scored := spotRec(0.12)
	scored.Scores = &engine.RecommendationScores{Risk: scoreVal(0.4)}
	both := ansi.Strip(renderRecommendation(scored))
	assert.True(t, strings.HasSuffix(both, "  risk 0.40"), "the scorer signal still ends the row: %q", both)

	plain := renderRecommendation(engine.Recommendation{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 10})
	assert.NotContains(t, plain, "SPOT")
}

func TestRecommendationsList_SpotFitsTerminalWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{100, 120} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			t.Parallel()

			recs := []engine.Recommendation{
				spotRec(0.12),
				spotRec(0),
				{
					ResourceID:       "i-plain",
					Type:             "RIGHTSIZE",
					Description:      "Downsize the over-provisioned instance to save money",
					EstimatedSavings: 5,
					Currency:         "USD",
				},
			}
			recs[1].ResourceID = "i-unknown"
			model := NewRecommendationsViewModel(recs)
			model.Update(tea.WindowSizeMsg{Width: width, Height: 40})
			output := ansi.Strip(model.renderListView())
			if width == 100 {
				testGolden(t, "recommendations_list_spot_100", output)
			}

			var header string
			spotRows := 0
			for line := range strings.SplitSeq(output, "\n") {
				assert.LessOrEqual(t, lipgloss.Width(line), width, "line exceeds %d columns: %q", width, line)
				if strings.HasPrefix(line, "Resource") {
					header = line
				}
				if strings.Contains(line, "SPOT · risk") {
					spotRows++
				}
			}
			assert.Equal(t, 2, spotRows)
			assert.Contains(t, header, "Pricing")
			assert.Less(t, strings.Index(header, "Pricing"), strings.Index(header, "Description"))
		})
	}
}

func TestRecommendationsList_NoSpotKeepsLayout(t *testing.T) {
	t.Parallel()

	model := NewRecommendationsViewModel([]engine.Recommendation{
		{ResourceID: "i-plain", Type: "RIGHTSIZE", Description: "Downsize", EstimatedSavings: 5, Currency: "USD"},
	})
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	output := ansi.Strip(model.renderListView())
	assert.NotContains(t, output, "Pricing")
	assert.Contains(t, output, fmt.Sprintf("%-*s", recColWidthDescription, "Description"))
}

func TestGolden_RecommendationDetail_Spot(t *testing.T) {
	t.Parallel()

	output := RenderRecommendationDetail(spotRec(0.12), 100)
	testGolden(t, "recommendation_detail_spot", output)
	assert.Contains(t, output, "Pricing:     SPOT · risk 0.12\n")

	assert.NotContains(t, RenderRecommendationDetail(engine.Recommendation{ResourceID: "r1"}, 100), "Pricing:")
}

func TestGolden_DetailView_SpotRecommendation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, goldenDayOfMonth, 12, 0, 0, 0, time.UTC)
	rows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:prod::myapp::aws:ec2/instance:Instance::batch-worker",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{
				MTDCost: 50, Currency: "USD",
				Period: engine.DateRange{Start: now.Add(-14 * 24 * time.Hour), End: now},
			},
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
			Recommendations: []engine.Recommendation{
				spotRec(0),
				{Type: "RIGHTSIZE", Description: "Downsize", EstimatedSavings: 10, Currency: "USD"},
			},
		},
	}

	model := newGoldenModel(t, rows)
	output := model.renderDetailViewForDay(goldenDayOfMonth)
	testGolden(t, "detail_spot_recommendation", output)

	plain := ansi.Strip(output)
	require.Contains(t, plain, "Pricing: SPOT · risk unknown")
	assert.Equal(t, 1, strings.Count(plain, "Pricing: SPOT"), "only the spot recommendation has a pricing line")
}

func TestRenderRecommendationsSection_SpotLabel(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	renderRecommendationsSection(&content, []engine.Recommendation{
		spotRec(0.12),
		{Type: "RIGHTSIZE", Description: "Downsize", EstimatedSavings: 5, Currency: "USD"},
	})
	plain := ansi.Strip(content.String())
	assert.Contains(
		t,
		plain,
		"- [RECOMMENDATION_ACTION_TYPE_MODIFY] Move batch-worker to spot capacity ($62.00 USD/mo savings)  SPOT · risk 0.12\n",
	)
	assert.Contains(t, plain, "- [RIGHTSIZE] Downsize ($5.00 USD/mo savings)\n")
}
