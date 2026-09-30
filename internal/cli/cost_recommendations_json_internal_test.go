package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func fullEngineRecommendation() engine.Recommendation {
	score := 0.75
	implCost := 20.0
	effort := 2.5
	createdAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	return engine.Recommendation{
		ID:               "rec-1",
		ResourceID:       "i-123",
		Type:             "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE",
		Category:         "RECOMMENDATION_CATEGORY_COST",
		Priority:         "RECOMMENDATION_PRIORITY_HIGH",
		ConfidenceScore:  &score,
		Source:           "kubecost",
		CreatedAt:        &createdAt,
		Metadata:         map[string]string{"team": "core"},
		Description:      "Downsize",
		EstimatedSavings: 40,
		Currency:         "USD",
		Reasoning:        []string{"check arm64"},
		ImpactDetail: &engine.RecommendationImpactDetail{
			ProjectionPeriod:     "monthly",
			CurrentCost:          100,
			ProjectedCost:        60,
			SavingsPercentage:    40,
			ImplementationCost:   &implCost,
			MigrationEffortHours: &effort,
		},
		ResourceInfo: &engine.RecommendationResourceInfo{
			Name:         "web",
			Provider:     "aws",
			ResourceType: "aws:ec2/instance:Instance",
			Region:       "us-east-1",
			SKU:          "m5.large",
			Tags:         map[string]string{"env": "prod"},
			Utilization: &engine.RecommendationUtilizationInfo{
				CPUPercent:    12.5,
				CustomMetrics: map[string]float64{"iops": 100},
			},
		},
	}
}

func TestRenderRecommendationsJSON_FullRecordAdditiveKeys(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{fullEngineRecommendation()},
		TotalSavings:    40,
		Currency:        "USD",
	}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsJSON(&buf, result, nil))

	var out struct {
		Recommendations []map[string]any `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Len(t, out.Recommendations, 1)
	rec := out.Recommendations[0]

	assert.Equal(t, "i-123", rec["resource_id"])
	assert.Equal(t, "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE", rec["action_type"])
	assert.Equal(t, "Downsize", rec["description"])
	assert.InDelta(t, 40.0, rec["estimated_savings"], 1e-9)
	assert.Equal(t, "USD", rec["currency"])

	assert.Equal(t, "rec-1", rec["id"])
	assert.Equal(t, "RECOMMENDATION_CATEGORY_COST", rec["category"])
	assert.Equal(t, "RECOMMENDATION_PRIORITY_HIGH", rec["priority"])
	assert.InDelta(t, 0.75, rec["confidence_score"], 1e-9)
	assert.Equal(t, "kubecost", rec["source"])
	assert.Equal(t, "2026-03-01T12:00:00Z", rec["created_at"])
	assert.Equal(t, map[string]any{"team": "core"}, rec["metadata"])
	assert.Equal(t, []any{"check arm64"}, rec["reasoning"])

	impact, ok := rec["impact"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "monthly", impact["projection_period"])
	assert.InDelta(t, 100.0, impact["current_cost"], 1e-9)
	assert.InDelta(t, 60.0, impact["projected_cost"], 1e-9)
	assert.InDelta(t, 40.0, impact["savings_percentage"], 1e-9)
	assert.InDelta(t, 20.0, impact["implementation_cost"], 1e-9)
	assert.InDelta(t, 2.5, impact["migration_effort_hours"], 1e-9)

	res, ok := rec["resource"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "web", res["name"])
	assert.Equal(t, "aws", res["provider"])
	assert.Equal(t, "aws:ec2/instance:Instance", res["resource_type"])
	assert.Equal(t, "us-east-1", res["region"])
	assert.Equal(t, "m5.large", res["sku"])
	assert.Equal(t, map[string]any{"env": "prod"}, res["tags"])
	util, ok := res["utilization"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 12.5, util["cpu_percent"], 1e-9)
	assert.Equal(t, map[string]any{"iops": 100.0}, util["custom_metrics"])
}

func TestRenderRecommendationsJSON_LegacyRecordKeysUnchanged(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{
			{ResourceID: "r", Type: "RIGHTSIZE", Description: "d", EstimatedSavings: 5, Currency: "USD"},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsJSON(&buf, result, nil))

	var out struct {
		Recommendations []map[string]any `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Len(t, out.Recommendations, 1)

	keys := make([]string, 0)
	for k := range out.Recommendations[0] {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t,
		[]string{"resource_id", "action_type", "description", "estimated_savings", "currency"}, keys)
}

func TestRenderRecommendationsNDJSON_FullRecord(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{fullEngineRecommendation()},
	}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsNDJSON(&buf, result, nil))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)

	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &rec))
	assert.Equal(t, "rec-1", rec["id"])
	assert.Equal(t, "RECOMMENDATION_PRIORITY_HIGH", rec["priority"])
	assert.Contains(t, rec, "impact")
	assert.Contains(t, rec, "resource")
}

func TestMergeDismissalRecordsIntoResult_SkipsActiveByRecommendationID(t *testing.T) {
	t.Parallel()

	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{
			{ID: "rec-1", ResourceID: "i-123", Type: "RIGHTSIZE"},
		},
	}
	records := map[string]*config.DismissalRecord{
		"rec-1": {
			RecommendationID: "rec-1",
			Status:           config.StatusDismissed,
			LastKnown:        &config.LastKnownRecommendation{ResourceID: "different-resource"},
		},
		"rec-2": {
			RecommendationID: "rec-2",
			Status:           config.StatusDismissed,
			LastKnown:        &config.LastKnownRecommendation{ResourceID: "other"},
		},
	}

	mergeDismissalRecordsIntoResult(records, result)

	require.Len(t, result.Recommendations, 2)
	assert.Equal(t, "other", result.Recommendations[1].ResourceID)
}
