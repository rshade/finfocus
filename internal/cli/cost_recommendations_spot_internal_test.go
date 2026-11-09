package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func spotRecommendation(risk float64) engine.Recommendation {
	return engine.Recommendation{
		ID:                   "rec-spot",
		ResourceID:           "i-spot",
		Type:                 "RECOMMENDATION_ACTION_TYPE_MODIFY",
		Description:          "Move to spot",
		EstimatedSavings:     62,
		Currency:             "USD",
		PricingCategory:      engine.PricingCategoryDynamic,
		SpotInterruptionRisk: risk,
	}
}

func TestRenderRecommendationsTable_SpotColumn(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		spotRecommendation(0.12),
		{
			ID:               "rec-2",
			ResourceID:       "i-plain",
			Type:             "RIGHTSIZE",
			Description:      "Downsize",
			EstimatedSavings: 5,
			Currency:         "USD",
		},
	}}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, result, true))
	lines := strings.Split(buf.String(), "\n")

	var header, spotRow, plainRow string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "RESOURCE"):
			header = line
		case strings.HasPrefix(line, "i-spot"):
			spotRow = line
		case strings.HasPrefix(line, "i-plain"):
			plainRow = line
		}
	}
	require.NotEmpty(t, header)
	assert.True(t, strings.HasSuffix(strings.TrimRight(header, " "), "PRICING"))
	assert.True(t, strings.HasSuffix(spotRow, "62.00 USD  SPOT · risk 0.12"), spotRow)
	assert.True(t, strings.HasSuffix(strings.TrimRight(plainRow, " "), "5.00 USD"), plainRow)
}

func TestRenderRecommendationsTable_UnknownRisk(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{spotRecommendation(0)}}
	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, result, true))
	assert.Contains(t, buf.String(), "SPOT · risk unknown")
}

func TestRenderRecommendationsTable_NoSpotNoColumn(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		{
			ID:               "rec-2",
			ResourceID:       "i-plain",
			Type:             "RIGHTSIZE",
			Description:      "Downsize",
			EstimatedSavings: 5,
			Currency:         "USD",
		},
	}}
	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, result, true))
	assert.NotContains(t, buf.String(), "PRICING")
	assert.NotContains(t, buf.String(), "SPOT")
}

func TestRenderRecommendationsJSON_SpotFields(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		spotRecommendation(0.12), spotRecommendation(0),
	}}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsJSON(&buf, result, nil))
	var out struct {
		Recommendations []map[string]any `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Len(t, out.Recommendations, 2)
	assert.Equal(t, "DYNAMIC", out.Recommendations[0]["pricing_category"])
	assert.InDelta(t, 0.12, out.Recommendations[0]["spot_interruption_risk"], 1e-12)
	assert.Equal(t, "DYNAMIC", out.Recommendations[1]["pricing_category"])
	assert.NotContains(t, out.Recommendations[1], "spot_interruption_risk", "unknown risk is omitted")

	var nd bytes.Buffer
	require.NoError(t, renderRecommendationsNDJSON(&nd, result, nil))
	ndLines := strings.Split(strings.TrimSpace(nd.String()), "\n")
	require.Len(t, ndLines, 3)
	assert.Contains(t, ndLines[1], `"pricing_category":"DYNAMIC"`)
	assert.Contains(t, ndLines[1], `"spot_interruption_risk":0.12`)
}
