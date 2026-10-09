package viewmodel

import (
	"fmt"

	"github.com/rshade/finfocus/internal/engine"
)

// FormatScore formats a scorer signal, retaining '-' for missing signals.
func FormatScore(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *value)
}

// ScoreDisplay provides every signal and scorer annotation without numeric JS formatting.
func ScoreDisplay(scores *engine.RecommendationScores) []DisplayField {
	if scores == nil {
		return []DisplayField{}
	}
	fields := []DisplayField{
		{Name: "Risk", Value: FormatScore(scores.Risk)},
		{Name: "False positive", Value: FormatScore(scores.FalsePositive)},
		{Name: "Worth acting", Value: FormatScore(scores.WorthActing)},
		{Name: "Priority", Value: FormatScore(scores.Priority)},
		{Name: "Thin evidence", Value: FormatScore(scores.InsufficientEvidence)},
	}
	if scores.DuplicateGroupID != "" {
		fields = append(fields, DisplayField{Name: "Duplicate group", Value: scores.DuplicateGroupID})
	}
	if scores.NeedsReview {
		fields = append(fields, DisplayField{Name: "Needs review", Value: "Yes"})
	}
	return fields
}
