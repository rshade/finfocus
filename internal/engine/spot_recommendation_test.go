package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestRecommendationSpotDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category PricingCategory
		risk     float64
		want     string
	}{
		{"spot with risk", PricingCategoryDynamic, 0.12, "SPOT · risk 0.12"},
		{"spot unknown risk", PricingCategoryDynamic, 0, "SPOT · risk unknown"},
		{"spot tiny risk", PricingCategoryDynamic, 0.001, "SPOT · risk <0.01"},
		{"standard", PricingCategoryStandard, 0.4, ""},
		{"unspecified", "", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := Recommendation{PricingCategory: tt.category, SpotInterruptionRisk: tt.risk}
			assert.Equal(t, tt.want, rec.SpotDisplay())
			assert.Equal(t, tt.want, SpotRiskDisplay(tt.category, tt.risk))
		})
	}
}

func TestConvertProtoRecommendation_SpotFields(t *testing.T) {
	t.Parallel()

	got := convertProtoRecommendation(&proto.Recommendation{
		ID:                   "rec-spot",
		ActionType:           "RECOMMENDATION_ACTION_TYPE_MODIFY",
		PricingCategory:      "DYNAMIC",
		SpotInterruptionRisk: 0.12,
		Impact:               &proto.RecommendationImpact{EstimatedSavings: 62, Currency: "USD"},
	})
	assert.Equal(t, PricingCategoryDynamic, got.PricingCategory)
	assert.InDelta(t, 0.12, got.SpotInterruptionRisk, 1e-12)
	assert.Equal(t, "SPOT · risk 0.12", got.SpotDisplay())
}

func TestRecommendationJSON_SpotFields(t *testing.T) {
	t.Parallel()

	spot, err := json.Marshal(Recommendation{
		ID: "a", PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
	})
	require.NoError(t, err)
	assert.Contains(t, string(spot), `"pricingCategory":"DYNAMIC"`)
	assert.Contains(t, string(spot), `"spotInterruptionRisk":0.12`)

	plain, err := json.Marshal(Recommendation{ID: "b"})
	require.NoError(t, err)
	assert.NotContains(t, string(plain), "pricingCategory")
	assert.NotContains(t, string(plain), "spotInterruptionRisk")
}

func TestRecommendationsCacheKey_FormatVersioned(t *testing.T) {
	t.Parallel()

	eng := New(nil, nil)
	resources := []ResourceDescriptor{ec2("r-0")}
	legacy := eng.generateRecommendationsCacheKey(resources, nil)

	key := eng.recommendationsCacheKey(resources, nil, false)
	assert.Equal(t, legacy+recommendationsCacheFormat, key)

	dismissed := eng.recommendationsCacheKey(resources, nil, true)
	assert.Equal(t, legacy+"/include-dismissed"+recommendationsCacheFormat, dismissed)
}

func TestGetRecommendationsForResources_SpotSurvivesCache(t *testing.T) {
	t.Parallel()

	plugin := &recordingRecClient{
		name: "p",
		recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
			return []*proto.Recommendation{{
				ID:                   "rec-spot",
				ResourceID:           "r-0",
				ActionType:           "RECOMMENDATION_ACTION_TYPE_MODIFY",
				PricingCategory:      "DYNAMIC",
				SpotInterruptionRisk: 0.12,
				Impact:               &proto.RecommendationImpact{EstimatedSavings: 62, Currency: "USD"},
			}}
		},
	}
	store := newRecCache(t)
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithCache(store)
	resources := []ResourceDescriptor{ec2("r-0")}
	ctx := context.Background()

	_, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	cached, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	assert.Equal(t, 1, plugin.calls(), "the second call is served from the cache")
	require.Len(t, cached.Recommendations, 1)
	assert.Equal(t, "SPOT · risk 0.12", cached.Recommendations[0].SpotDisplay())
}
