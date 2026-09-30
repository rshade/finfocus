package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func scoreVal(v float64) *float64 { return &v }

func TestRenderRecommendation_UnscoredRowIsUnchanged(t *testing.T) {
	rec := engine.Recommendation{ResourceID: "r1", Type: "RIGHTSIZE", Description: "d", EstimatedSavings: 10}

	assert.NotContains(t, renderRecommendation(rec, false), "risk")
}

func TestRenderRecommendation_ShowsRiskAndReviewMarker(t *testing.T) {
	rec := engine.Recommendation{
		ResourceID: "r1", Type: "RIGHTSIZE", Description: "d", EstimatedSavings: 10,
		Scores: &engine.RecommendationScores{Risk: scoreVal(0.42), NeedsReview: true, DuplicateGroupID: "dup-1"},
	}

	row := renderRecommendation(rec, false)

	assert.Contains(t, row, "risk 0.42")
	assert.Contains(t, row, "REVIEW")
	assert.Contains(t, row, "dup-1")
}

func TestRenderRecommendation_ScoredWithoutRisk(t *testing.T) {
	rec := engine.Recommendation{
		ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 10,
		Scores: &engine.RecommendationScores{Priority: scoreVal(2)},
	}

	row := renderRecommendation(rec, false)

	assert.Contains(t, row, "risk -")
	assert.NotContains(t, row, "REVIEW")
}

func TestRenderRecommendationDetail_ListsScores(t *testing.T) {
	rec := engine.Recommendation{
		ResourceID: "r1", Type: "RIGHTSIZE", Description: "d", EstimatedSavings: 10, Currency: "USD",
		Scores: &engine.RecommendationScores{
			Risk: scoreVal(0.3), FalsePositive: scoreVal(0.1), WorthActing: scoreVal(0.9),
			Priority: scoreVal(2), InsufficientEvidence: scoreVal(0.05),
			DuplicateGroupID: "dup-2", NeedsReview: true,
		},
	}

	out := RenderRecommendationDetail(rec, 80)

	for _, want := range []string{"Risk:", "0.30", "False positive:", "Worth acting:", "Priority:",
		"Thin evidence:", "Duplicate group: dup-2", "Needs review"} {
		assert.Contains(t, out, want)
	}
}

func TestRenderRecommendationDetail_NoScoresSection(t *testing.T) {
	rec := engine.Recommendation{ResourceID: "r1", Type: "RIGHTSIZE", Description: "d"}

	assert.NotContains(t, RenderRecommendationDetail(rec, 80), "Risk:")
}
