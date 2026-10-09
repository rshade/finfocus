package viewmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestActualCostDisplayCurrency(t *testing.T) {
	t.Parallel()
	page, err := BuildActualCostPage(
		context.Background(),
		[]engine.CostResult{{ResourceID: "eu", Currency: "EUR", TotalCost: 1234}},
		engine.GroupByNone,
		"",
		SortByCost,
		1,
		nil,
		"",
	)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	assert.Equal(t, "$1,234.00 EUR", page.Rows[0].Cost)
	assert.Equal(t, "$1,234.00 EUR", page.TotalDisplay)
}

func TestTimeAggregateSortCostPeriodAndTies(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	results := make([]engine.CostResult, 3)
	for i, total := range []float64{1, 10, 10} {
		day := start.AddDate(0, i, 0)
		results[i] = engine.CostResult{ResourceID: fmt.Sprintf("r-%d", i), Currency: "USD",
			TotalCost: total, DailyCosts: []float64{total}, StartDate: day, EndDate: day.AddDate(0, 0, 1)}
	}
	for _, tc := range []struct {
		field SortField
		want  []string
	}{{SortByCost, []string{"2026-02", "2026-03", "2026-01"}},
		{SortByName, []string{"2026-01", "2026-02", "2026-03"}}} {
		page, err := BuildActualCostPage(context.Background(), results, engine.GroupByMonthly,
			"", tc.field, 1, nil, "")
		require.NoError(t, err)
		periods := make([]string, len(page.Aggregations))
		for i, aggregate := range page.Aggregations {
			periods[i] = aggregate.Period
		}
		assert.Equal(t, tc.want, periods)
	}
}

func TestActualCostPageGroupingFilteringAndPagination(t *testing.T) {
	t.Parallel()
	results := make([]engine.CostResult, 301)
	for i := range results {
		results[i] = engine.CostResult{
			ResourceID:   fmt.Sprintf("resource-%03d", i),
			ResourceType: "aws:ec2:Instance",
			TotalCost:    float64(i),
			Currency:     "USD",
		}
	}
	page, err := BuildActualCostPage(
		context.Background(),
		results,
		engine.GroupByNone,
		"",
		SortByName,
		2,
		map[string]string{"resource-250": "▁▂▃▄▅▆█"},
		"total",
	)
	require.NoError(t, err)
	assert.Equal(t, 2, page.TotalPages)
	require.Len(t, page.Rows, 51)
	assert.Equal(t, "resource-250", page.Rows[0].ID)
	assert.Equal(t, "0,28 12,24 24,20 36,16 48,12 60,8 72,0", page.Rows[0].TrendPoints)
	assert.Equal(t, "total", page.TotalTrend)
	assert.Equal(t, "resource-000", results[0].ResourceID)
	filtered, err := BuildActualCostPage(
		context.Background(),
		results,
		engine.GroupByNone,
		"RESOURCE-010",
		SortByCost,
		999,
		nil,
		"",
	)
	require.NoError(t, err)
	require.Len(t, filtered.Rows, 1)
	assert.Equal(t, 1, filtered.Page)
	assert.Equal(t, "resource-010", filtered.Rows[0].ID)
	assert.Empty(t, SparklinePoints("bad"))
}

func TestActualCostTimeAggregationError(t *testing.T) {
	t.Parallel()
	_, err := BuildActualCostPage(
		context.Background(),
		[]engine.CostResult{
			{ResourceID: "r", TotalCost: 1, Currency: "USD"},
			{ResourceID: "s", Currency: "EUR", TotalCost: 2},
		},
		engine.GroupByDaily,
		"",
		SortByCost,
		1,
		nil,
		"",
	)
	assert.Error(t, err)
}

func TestRecommendationPaginationSummary(t *testing.T) {
	t.Parallel()
	recs := make([]engine.Recommendation, 301)
	for i := range recs {
		recs[i] = engine.Recommendation{
			ID:               strconv.Itoa(i),
			ResourceID:       fmt.Sprintf("r-%03d", i),
			Type:             "Resize",
			EstimatedSavings: 1,
			Currency:         "USD",
		}
	}
	page := BuildRecommendationPage(&engine.RecommendationsResult{Recommendations: recs}, "", SortByResourceID, 2)
	require.Len(t, page.Items, 51)
	assert.Equal(t, 301, page.Summary.TotalCount)
	assert.InDelta(t, 301.0, page.Summary.TotalSavings, 1e-9)
	assert.Equal(t, 2, page.TotalPages)
	require.Len(t, page.Actions, 1)
	assert.Equal(t, "$301.00 USD", page.Actions[0].SavingsDisplay)
	empty := BuildRecommendationPage(&engine.RecommendationsResult{}, "", SortBySavings, 1)
	assert.NotNil(t, empty.Items)
	assert.Empty(t, empty.Items)
}

func TestSharedMoneyFormatPreservesTUIConvention(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		value          float64
		currency, want string
	}{{0, "", "$0.00"}, {-1234.5, "EUR", "-$1,234.50 EUR"}, {math.NaN(), "USD", "$0.00 USD"}, {math.Inf(1), "USD", "$0.00 USD"}} {
		assert.Equal(t, tc.want, FormatMoney(tc.value, tc.currency))
	}
}

func TestActualTimePageAndCarbon(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	results := []engine.CostResult{
		{
			ResourceID:     "r",
			ResourceType:   "aws:ec2:Instance",
			Currency:       "USD",
			TotalCost:      12,
			StartDate:      start,
			EndDate:        start.AddDate(0, 0, 2),
			DailyCosts:     []float64{5, 7},
			Sustainability: map[string]engine.SustainabilityMetric{"carbon_footprint": {Value: 1000, Unit: "g"}},
		},
	}
	page, err := BuildActualCostPage(context.Background(), results, engine.GroupByDaily, "", SortByCost, 1, nil, "")
	require.NoError(t, err)
	require.NotEmpty(t, page.Rows)
	assert.False(t, page.Rows[0].CanDetail)
	assert.Equal(t, "1.00 kg", page.CarbonDisplay)
	assert.Equal(t, page.Carbon, page.Summary.Carbon)
}

func TestActualDetailDisplay(t *testing.T) {
	t.Parallel()
	cost := engine.CostResult{
		TotalCost:       12,
		Currency:        "USD",
		Breakdown:       map[string]float64{"storage": 2, "compute": 10},
		Recommendations: []engine.Recommendation{{ID: "r", EstimatedSavings: 3, Currency: "USD"}},
	}
	detail := BuildActualCostDetail(cost)
	assert.Equal(t, cost, detail.Result)
	assert.Equal(t, "$12.00 USD", detail.CostDisplay)
	require.Len(t, detail.Breakdown, 2)
	assert.Equal(t, "compute", detail.Breakdown[0].Name)
	assert.Equal(t, "$10.0000", detail.Breakdown[0].CostDisplay)
	require.Len(t, detail.Recommendations, 1)
	assert.Equal(t, "$3.00 USD", detail.Recommendations[0].SavingsDisplay)
}

func TestRecommendationDetailKeysPreserveIDsAndDisambiguate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "id", RecommendationDetailKey(engine.Recommendation{ID: "id"}))
	first := engine.Recommendation{ResourceID: "r", Type: "Resize", Description: "resize"}
	second := engine.Recommendation{ResourceID: "r", Type: "Delete", Description: "delete"}
	assert.NotEqual(t, RecommendationDetailKey(first), RecommendationDetailKey(second))
	reloaded := engine.Recommendation{
		ResourceID:  "r",
		Type:        "Resize",
		Description: "resize",
		Status:      engine.RecommendationStatusDismissed,
	}
	assert.Equal(t, RecommendationDetailKey(first), RecommendationDetailKey(reloaded))
}

func TestEmptyActualCostPageUsesAnArray(t *testing.T) {
	t.Parallel()
	page, err := BuildActualCostPage(context.Background(), nil, engine.GroupByNone, "", SortByCost, 1, nil, "")
	require.NoError(t, err)
	results, ok := page.Results.([]engine.CostResult)
	require.True(t, ok)
	assert.NotNil(t, results)
	assert.Empty(t, results)
}

func TestActualEmptyFilterGroupsAreSuccessfulArrays(t *testing.T) {
	t.Parallel()
	for _, group := range []engine.GroupBy{engine.GroupByNone, engine.GroupByDaily, engine.GroupByMonthly} {
		t.Run(string(group), func(t *testing.T) {
			t.Parallel()
			page, err := BuildActualCostPage(
				context.Background(),
				[]engine.CostResult{{ResourceID: "known", Currency: "USD"}},
				group,
				"missing",
				SortByCost,
				1,
				nil,
				"",
			)
			require.NoError(t, err)
			assert.Equal(t, 1, page.Page)
			assert.Equal(t, 1, page.TotalPages)
			assert.Empty(t, page.Rows)
			encoded, err := json.Marshal(page.Results)
			require.NoError(t, err)
			assert.JSONEq(t, "[]", string(encoded))
		})
	}
}
