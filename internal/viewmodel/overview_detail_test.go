package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestOverviewDetailRecommendations(t *testing.T) {
	t.Parallel()
	recs := []engine.Recommendation{
		{Description: "active", EstimatedSavings: 12.345},
		{Description: "dismissed", Status: engine.RecommendationStatusDismissed},
		{Description: "snoozed", Status: engine.RecommendationStatusSnoozed},
	}
	detail := OverviewDetailRecommendations(recs)
	require.Len(t, detail, 1)
	assert.Equal(t, "active", detail[0].Recommendation.Description)
	assert.Equal(t, engine.FormatOverviewCurrency(12.345), detail[0].SavingsDisplay)
}
