package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestRecommendationWebDisplayParity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		amount         float64
		currency, want string
	}{{"default", 12, "", "$12.00 USD"}, {"explicit", 12, "USD", "$12.00 USD"}, {"eur", 12, "EUR", "$12.00 EUR"}, {"negative", -12, "", "$-12.00 USD"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := engine.Recommendation{EstimatedSavings: tc.amount, Currency: tc.currency}
			row := NewRecommendationRow(rec)
			assert.Equal(t, tc.want, row.Savings)
			assert.Equal(t, row.Savings, viewmodel.RecommendationSavingsDisplay(rec))
		})
	}
}

func TestRecommendationSpotWebDisplayParity(t *testing.T) {
	t.Parallel()
	recs := []engine.Recommendation{
		{ID: "a", PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12},
		{ID: "b", PricingCategory: engine.PricingCategoryDynamic},
		{ID: "c"},
	}
	page := viewmodel.BuildRecommendationPage(
		&engine.RecommendationsResult{Recommendations: recs}, "", viewmodel.SortByResourceID, 1,
	)
	require.Len(t, page.ItemSpot, len(page.Items))
	for i, item := range page.Items {
		assert.Equal(t, NewRecommendationRow(item).Spot, page.ItemSpot[i], item.ID)
	}
	assert.Equal(t, "SPOT · risk 0.12", NewRecommendationRow(recs[0]).Spot)
}
