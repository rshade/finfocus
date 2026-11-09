package webui

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestRecommendationQueryItemNames(t *testing.T) {
	t.Parallel()

	fixture := []engine.Recommendation{
		{
			ID: "urn", ResourceID: "urn:pulumi:dev::proj::comp:index:Node$aws:ec2/instance:Instance::appConnector",
			Type: "RECOMMENDATION_ACTION_TYPE_MODIFY", EstimatedSavings: 62, Currency: "USD",
		},
		{ID: "cloud", ResourceID: "i-0abc", Type: "RIGHTSIZE", EstimatedSavings: 5, Currency: "USD"},
	}
	s := newRecommendationServer(t, func(context.Context, bool) (*engine.RecommendationsResult, error) {
		return &engine.RecommendationsResult{Recommendations: append([]engine.Recommendation(nil), fixture...)}, nil
	})

	resp := overviewRequest(t, s, "POST", "/api/recommendations/query", `{"sort":"savings"}`)
	require.Equal(t, 200, resp.StatusCode)
	var page viewmodel.RecommendationPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	require.Len(t, page.Items, 2)
	require.Len(t, page.ItemNames, 2)
	for i, item := range page.Items {
		assert.Equal(t, engine.ExtractResourceDisplayName(item.ResourceID), page.ItemNames[i], item.ID)
	}
	assert.Equal(t, []string{"appConnector", "i-0abc"}, page.ItemNames)
}

func TestDetailRecommendationActionDisplay(t *testing.T) {
	t.Parallel()

	recs := []engine.Recommendation{
		{ID: "modify", Type: "RECOMMENDATION_ACTION_TYPE_MODIFY", EstimatedSavings: 62},
		{ID: "short", Type: "RIGHTSIZE", EstimatedSavings: 5},
		{ID: "none"},
	}

	overview := viewmodel.OverviewDetailRecommendations(recs)
	cost := viewmodel.LinkedCostRecommendations(recs)
	require.Len(t, overview, 3)
	require.Len(t, cost, 3)
	for i, rec := range recs {
		want := proto.ActionTypeLabelFromString(rec.Type)
		assert.Equal(t, want, overview[i].ActionDisplay, rec.ID)
		assert.Equal(t, want, cost[i].ActionDisplay, rec.ID)
	}
	assert.Equal(t, "Modify", overview[0].ActionDisplay)
	assert.Equal(t, "Rightsize", cost[1].ActionDisplay)

	data, err := json.Marshal(overview)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"actionDisplay":"Modify"`)
	data, err = json.Marshal(cost)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"actionDisplay":"Modify"`)
}
