package engine

import (
	"github.com/rshade/finfocus/internal/config"
)

// ApplyDismissalDeltaToRow appends dismissed and snoozed recommendations to the
// row's Recommendations slice by looking up each non-active DismissalRecord whose
// LastKnown.ResourceID matches the row's ResourceID (or URN as fallback).
//
// This enables the count badge to show the total including dismissed recs,
// e.g. "3(-1)" when 3 total and 1 is dismissed, without listing the dismissed
// ones in full in the detail view. It is a no-op when records is empty or nil.
func ApplyDismissalDeltaToRow(row *OverviewRow, records map[string]*config.DismissalRecord) {
	if len(records) == 0 {
		return
	}

	for _, record := range records {
		// Only include dismissed and snoozed records.
		if record.Status == config.StatusActive {
			continue
		}
		if record.LastKnown == nil {
			continue
		}

		// Match record to this row by resource ID or URN.
		if record.LastKnown.ResourceID != row.ResourceID &&
			record.LastKnown.ResourceID != row.URN {
			continue
		}

		// Map config status to engine recommendation status.
		status := RecommendationStatusDismissed
		if record.Status == config.StatusSnoozed {
			status = RecommendationStatusSnoozed
		}

		row.Recommendations = append(row.Recommendations, Recommendation{
			ResourceID:       record.LastKnown.ResourceID,
			Type:             record.LastKnown.Type,
			Description:      record.LastKnown.Description,
			EstimatedSavings: record.LastKnown.EstimatedSavings,
			Currency:         record.LastKnown.Currency,
			Status:           status,
		})
	}
}

// CountRecsActiveAndDismissed returns the count of active and dismissed/snoozed
// recommendations in a slice. Active means Status is empty or "Active";
// dismissed means Status is "Dismissed" or "Snoozed".
func CountRecsActiveAndDismissed(recs []Recommendation) (int, int) {
	active, dismissed := 0, 0
	for _, rec := range recs {
		if rec.Status == RecommendationStatusDismissed || rec.Status == RecommendationStatusSnoozed {
			dismissed++
		} else {
			active++
		}
	}
	return active, dismissed
}
