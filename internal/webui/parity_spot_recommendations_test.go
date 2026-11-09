package webui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestSpotRecommendationParity(t *testing.T) {
	t.Parallel()

	fixture := []engine.Recommendation{
		{
			ID: "spot", ResourceID: "batch", Type: "RECOMMENDATION_ACTION_TYPE_MODIFY",
			EstimatedSavings: 62, Currency: "USD",
			PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
		},
		{
			ID: "unknown", ResourceID: "worker", Type: "RECOMMENDATION_ACTION_TYPE_MODIFY",
			EstimatedSavings: 40, Currency: "USD", PricingCategory: engine.PricingCategoryDynamic,
		},
		{ID: "plain", ResourceID: "db", Type: "RIGHTSIZE", EstimatedSavings: 5, Currency: "USD"},
	}
	s := newRecommendationServer(t, func(context.Context, bool) (*engine.RecommendationsResult, error) {
		return &engine.RecommendationsResult{Recommendations: append([]engine.Recommendation(nil), fixture...)}, nil
	})

	resp := overviewRequest(t, s, "POST", "/api/recommendations/query", `{"sort":"savings"}`)
	require.Equal(t, 200, resp.StatusCode)
	var page viewmodel.RecommendationPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	require.Len(t, page.Items, 3)
	require.Len(t, page.ItemSpot, 3)
	for i, item := range page.Items {
		assert.Equal(t, item.SpotDisplay(), page.ItemSpot[i], item.ID)
	}
	assert.Equal(t, []string{"SPOT · risk 0.12", "SPOT · risk unknown", ""}, page.ItemSpot)
	assert.Equal(t, fixture[0].PricingCategory, page.Items[0].PricingCategory, "the category survives redaction")
	assert.InDelta(t, 0.12, page.Items[0].SpotInterruptionRisk, 1e-12)

	for _, tc := range []struct{ id, want string }{
		{"spot", "SPOT · risk 0.12"},
		{"unknown", "SPOT · risk unknown"},
		{"plain", ""},
	} {
		item := overviewRequest(t, s, "GET", "/api/recommendations/item?id="+tc.id, "")
		require.Equal(t, 200, item.StatusCode)
		var detail map[string]any
		require.NoError(t, json.NewDecoder(item.Body).Decode(&detail))
		if tc.want == "" {
			assert.NotContains(t, detail, "spotDisplay")
			continue
		}
		assert.Equal(t, tc.want, detail["spotDisplay"])
	}
}

func TestOverviewDetailRecommendationSpotDisplay(t *testing.T) {
	t.Parallel()

	recs := []engine.Recommendation{
		{ID: "spot", PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12},
		{ID: "plain"},
		{ID: "gone", PricingCategory: engine.PricingCategoryDynamic, Status: engine.RecommendationStatusDismissed},
	}
	got := viewmodel.OverviewDetailRecommendations(recs)
	require.Len(t, got, 2)
	assert.Equal(t, recs[0].SpotDisplay(), got[0].SpotDisplay)
	assert.Empty(t, got[1].SpotDisplay)

	data, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"spotDisplay":"SPOT · risk 0.12"`)
	assert.Equal(t, 1, strings.Count(string(data), `"spotDisplay"`))
}

func TestLinkedCostRecommendationSpotDisplay(t *testing.T) {
	t.Parallel()

	spot := engine.Recommendation{ID: "spot", PricingCategory: engine.PricingCategoryDynamic}
	got := viewmodel.LinkedCostRecommendations([]engine.Recommendation{spot, {ID: "plain"}})
	require.Len(t, got, 2)
	assert.Equal(t, spot.SpotDisplay(), got[0].SpotDisplay)
	assert.Equal(t, "SPOT · risk unknown", got[0].SpotDisplay)
	assert.Empty(t, got[1].SpotDisplay)
}
