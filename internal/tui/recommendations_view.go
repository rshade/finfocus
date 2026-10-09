package tui

import (
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// Display constants for recommendations.
const (
	// maxDescLen is the maximum length for description display.
	maxDescLen = 40

	// maxResourceIDLen is the maximum length for resource ID display.
	maxResourceIDLen = 30
)

// RecommendationRow represents a single row in the recommendations table.
type RecommendationRow struct {
	// ResourceID is the resource identifier (truncated for display).
	ResourceID string

	// ActionType is the recommendation action type (e.g., "RIGHTSIZE").
	ActionType string

	// Description is the recommendation description (truncated for display).
	Description string

	// Savings is the formatted savings string (e.g., "$87.60 USD").
	Savings string

	// HasSavings indicates if there are savings > 0 (for styling).
	HasSavings bool
}

// NewRecommendationRow converts an engine.Recommendation into a display-ready row.
func NewRecommendationRow(rec engine.Recommendation) RecommendationRow {
	resourceID := rec.ResourceID
	if len(resourceID) > maxResourceIDLen {
		resourceID = resourceID[:maxResourceIDLen-3] + "..."
	}

	description := rec.Description
	if len(description) > maxDescLen {
		description = description[:maxDescLen-3] + "..."
	}

	savings := viewmodel.RecommendationSavingsDisplay(rec)
	hasSavings := rec.EstimatedSavings > 0

	return RecommendationRow{
		ResourceID:  resourceID,
		ActionType:  rec.Type,
		Description: description,
		Savings:     savings,
		HasSavings:  hasSavings,
	}
}
