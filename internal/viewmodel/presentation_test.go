package viewmodel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestActualDetailPresentation(t *testing.T) {
	t.Parallel()
	cost := engine.CostResult{
		ResourceID:     "vm",
		ResourceType:   "aws:ec2:Instance",
		Currency:       "USD",
		TotalCost:      12.3456,
		StartDate:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndDate:        time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
		Breakdown:      map[string]float64{"compute": 12.3456},
		Sustainability: map[string]engine.SustainabilityMetric{"carbon": {Value: 0.23456, Unit: "kgCO2e"}},
	}
	detail := BuildActualCostDetail(cost)
	require.Len(t, detail.Breakdown, 1)
	assert.Equal(t, "$12.3456", detail.Breakdown[0].CostDisplay)
	data, err := json.Marshal(detail)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"provider":"aws"`)
	assert.Contains(t, string(data), `"periodDisplay":"2026-10-01 - 2026-10-03"`)
	assert.Contains(t, string(data), `"value":"0.23 kgCO2e"`)
}

func TestActualSummaryPresentation(t *testing.T) {
	t.Parallel()
	page, err := BuildActualCostPage(
		context.Background(),
		[]engine.CostResult{
			{
				ResourceID:      "vm",
				ResourceType:    "aws:ec2:Instance",
				Currency:        "USD",
				TotalCost:       12.3456,
				Recommendations: []engine.Recommendation{{Type: "RIGHTSIZE"}},
			},
		},
		engine.GroupByNone,
		"",
		SortByCost,
		1,
		nil,
		"",
	)
	require.NoError(t, err)
	data, err := json.Marshal(page)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"recommendationCount":1`)
	assert.Contains(t, string(data), `"resourceCount":1`)
	assert.Contains(t, string(data), `"costDisplay":"$12.35"`)
	assert.Contains(t, string(data), `"shareDisplay":"100.0%"`)
}

func TestRecommendationActionPresentation(t *testing.T) {
	t.Parallel()
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{
			{Type: "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE"},
			{Type: "FUTURE_ACTION"},
		},
	}
	page := BuildRecommendationPage(result, "", SortBySavings, 1)
	data, err := json.Marshal(page)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"actionDisplay":"Rightsize"`)
	assert.Contains(t, string(data), `"actionDisplay":"FUTURE_ACTION"`)
}

func TestCostPresentationFallbacksAndOrdering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	results := []engine.CostResult{
		{
			ResourceType:    "aws:ec2:Instance",
			Monthly:         10,
			TotalCost:       -2,
			Recommendations: []engine.Recommendation{{EstimatedSavings: 2}},
		},
		{ResourceType: "gcp:compute:Instance", TotalCost: 20},
		{ResourceType: "azure:compute:VM", TotalCost: 10},
	}
	summary := BuildCostSummaryDisplay(ctx, results)
	assert.Equal(t, "$40.00", summary.TotalDisplay)
	assert.Equal(
		t,
		[]ProviderCostDisplay{
			{Name: "gcp", CostDisplay: "$20.00", ShareDisplay: "50.0%"},
			{Name: "aws", CostDisplay: "$10.00", ShareDisplay: "25.0%"},
			{Name: "azure", CostDisplay: "$10.00", ShareDisplay: "25.0%"},
		},
		summary.Providers,
	)
	zero := BuildCostSummaryDisplay(ctx, []engine.CostResult{{ResourceType: "aws:ec2:Instance"}})
	assert.Equal(t, "0.0%", zero.Providers[0].ShareDisplay)
	cost := engine.CostResult{
		Currency: "EUR",
		Monthly:  12.3456,
		Hourly:   0.123456,
		Delta:    -2.555,
		Error:    &engine.StructuredError{Message: "detail failure"},
	}
	detail := BuildActualCostDetail(cost)
	assert.Empty(t, detail.CostDisplay)
	assert.Equal(t, "$12.35 EUR", detail.MonthlyDisplay)
	assert.Equal(t, "$0.1235 EUR", detail.HourlyDisplay)
	assert.Equal(t, "-$2.56 ↓", detail.DeltaDisplay)
	assert.Equal(t, "detail failure", detail.NotesDisplay)
	assert.Equal(t, "$0.00 →", CostDeltaDisplay(0.0001))
	assert.Equal(t, "+$2.56 ↑", CostDeltaDisplay(2.555))
	assert.Equal(t, "aws:$12 gcp:$23", AggregationProvidersDisplay(map[string]float64{"gcp": 23.4, "aws": 12.4}))
	recs := LinkedCostRecommendations(
		[]engine.Recommendation{
			{ID: "zero"},
			{ID: "high", EstimatedSavings: 3},
			{ID: "equal", EstimatedSavings: 3},
			{ID: "low", EstimatedSavings: 1},
		},
	)
	require.Len(t, recs, 4)
	assert.Equal(t, "high", recs[0].Recommendation.ID)
	assert.Equal(t, "equal", recs[1].Recommendation.ID)
	assert.Empty(t, recs[3].SavingsDisplay)
	carbon := BuildCostSummaryDisplay(
		ctx,
		[]engine.CostResult{
			{Sustainability: map[string]engine.SustainabilityMetric{"carbon_footprint": {Value: 10, Unit: "kgCO2e"}}},
		},
	)
	assert.NotEmpty(t, carbon.CarbonEquivalency)
}

func TestScorePresentationMissingZeroAndAnnotations(t *testing.T) {
	t.Parallel()
	zero, risk := 0.0, 0.2
	assert.Empty(t, ScoreDisplay(nil))
	fields := ScoreDisplay(
		&engine.RecommendationScores{
			Risk:             &risk,
			FalsePositive:    &zero,
			DuplicateGroupID: "duplicates",
			NeedsReview:      true,
		},
	)
	assert.Equal(
		t,
		[]DisplayField{
			{Name: "Risk", Value: "0.20"},
			{Name: "False positive", Value: "0.00"},
			{Name: "Worth acting", Value: "-"},
			{Name: "Priority", Value: "-"},
			{Name: "Thin evidence", Value: "-"},
			{Name: "Duplicate group", Value: "duplicates"},
			{Name: "Needs review", Value: "Yes"},
		},
		fields,
	)
}

func TestOverviewDisplayStatusAndDrift(t *testing.T) {
	t.Parallel()
	delta := 12.0
	for _, status := range []engine.ResourceStatus{engine.StatusUpdating, engine.StatusReplacing, engine.StatusCreating, engine.StatusDeleting, engine.StatusActive} {
		row := engine.OverviewRow{
			Status:        status,
			ActualCost:    &engine.ActualCostData{MTDCost: 10},
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 42},
		}
		result := engine.ComputeOverviewRowResult(row)
		result.Delta = &delta
		display := BuildOverviewDetailDisplay(result, 10)
		if status == engine.StatusActive {
			assert.Empty(t, display.Impact)
			continue
		}
		require.NotEmpty(t, display.Impact)
		assert.Equal(t, DisplayField{Name: "Delta", Value: "+$12.00"}, display.Impact[len(display.Impact)-1])
	}
	assert.Empty(t, OverviewImpactDisplay(engine.OverviewRowResult{Status: engine.StatusUpdating}, 1))
	row := engine.OverviewRowResult{
		Status: engine.StatusUpdating,
		Delta:  &delta,
		Source: engine.OverviewRow{
			BaselineProjectedCost: &engine.ProjectedCostData{MonthlyCost: 30},
			ProjectedCost:         &engine.ProjectedCostData{MonthlyCost: 42},
		},
		CostDrift: &engine.CostDriftData{
			ExtrapolatedMonthly: 30.3456,
			Projected:           40,
			Delta:               9.6544,
			PercentDrift:        31.814,
		},
	}
	display := BuildOverviewDetailDisplay(row, 10)
	assert.Equal(t, "$30.00", display.Impact[0].Value)
	assert.Equal(
		t,
		[]DisplayField{
			{Name: "Extrapolated Monthly", Value: "$30.35"},
			{Name: "Projected", Value: "$40.00"},
			{Name: "Delta", Value: "$9.65"},
			{Name: "Drift", Value: "31.8%"},
		},
		display.Drift,
	)
}
