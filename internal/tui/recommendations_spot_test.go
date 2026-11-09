package tui

import (
	"strings"
	"testing"
	"time"

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

	row := ansi.Strip(renderRecommendation(spotRec(0.12), false))
	assert.True(t, strings.HasSuffix(strings.TrimRight(row, " "), "  SPOT · risk 0.12"), row)

	unknown := ansi.Strip(renderRecommendation(spotRec(0), false))
	assert.Contains(t, unknown, "SPOT · risk unknown")

	scored := spotRec(0.12)
	scored.Scores = &engine.RecommendationScores{Risk: scoreVal(0.4)}
	both := ansi.Strip(renderRecommendation(scored, false))
	assert.Contains(t, both, "SPOT · risk 0.12  risk 0.40", "the spot label precedes the scorer signal")

	plain := renderRecommendation(
		engine.Recommendation{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 10},
		false,
	)
	assert.NotContains(t, plain, "SPOT")
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
