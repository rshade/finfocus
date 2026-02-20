package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// ---------------------------------------------------------------------------
// ApplyDismissalDeltaToRow
// ---------------------------------------------------------------------------

func TestApplyDismissalDeltaToRow_NoRecords(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-123", Type: "aws:ec2:Instance"}
	ApplyDismissalDeltaToRow(&row, nil)
	assert.Empty(t, row.Recommendations)
}

func TestApplyDismissalDeltaToRow_EmptyRecords(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-123", Type: "aws:ec2:Instance"}
	ApplyDismissalDeltaToRow(&row, map[string]*config.DismissalRecord{})
	assert.Empty(t, row.Recommendations)
}

func TestApplyDismissalDeltaToRow_ActiveRecordSkipped(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-123", Type: "aws:ec2:Instance"}
	records := map[string]*config.DismissalRecord{
		"rec-1": {
			RecommendationID: "rec-1",
			Status:           config.StatusActive,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID: "i-123",
				Type:       "Right-sizing",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	assert.Empty(t, row.Recommendations)
}

func TestApplyDismissalDeltaToRow_DismissedRecAppended(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-123", Type: "aws:ec2:Instance"}
	records := map[string]*config.DismissalRecord{
		"rec-1": {
			RecommendationID: "rec-1",
			Status:           config.StatusDismissed,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID:       "i-123",
				Type:             "Right-sizing",
				Description:      "Downsize to t3.small",
				EstimatedSavings: 20.0,
				Currency:         "USD",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	require.Len(t, row.Recommendations, 1)
	rec := row.Recommendations[0]
	assert.Equal(t, RecommendationStatusDismissed, rec.Status)
	assert.Equal(t, "i-123", rec.ResourceID)
	assert.Equal(t, "Right-sizing", rec.Type)
	assert.Equal(t, "Downsize to t3.small", rec.Description)
	assert.Equal(t, 20.0, rec.EstimatedSavings)
	assert.Equal(t, "USD", rec.Currency)
}

func TestApplyDismissalDeltaToRow_SnoozedRecAppended(t *testing.T) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	row := OverviewRow{URN: "urn:test", ResourceID: "i-456", Type: "aws:ec2:Instance"}
	records := map[string]*config.DismissalRecord{
		"rec-2": {
			RecommendationID: "rec-2",
			Status:           config.StatusSnoozed,
			ExpiresAt:        &expiresAt,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID:       "i-456",
				Type:             "Terminate",
				Description:      "Instance not in use",
				EstimatedSavings: 50.0,
				Currency:         "USD",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	require.Len(t, row.Recommendations, 1)
	assert.Equal(t, RecommendationStatusSnoozed, row.Recommendations[0].Status)
}

func TestApplyDismissalDeltaToRow_NoLastKnownSkipped(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-789", Type: "aws:ec2:Instance"}
	records := map[string]*config.DismissalRecord{
		"rec-3": {
			RecommendationID: "rec-3",
			Status:           config.StatusDismissed,
			LastKnown:        nil, // No LastKnown — cannot match to a row.
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	assert.Empty(t, row.Recommendations)
}

func TestApplyDismissalDeltaToRow_URNFallbackMatch(t *testing.T) {
	// ResourceID is empty; match by URN instead.
	row := OverviewRow{
		URN:        "urn:pulumi:stack::proj::aws:ec2:Instance::my-instance",
		ResourceID: "",
		Type:       "aws:ec2:Instance",
	}
	records := map[string]*config.DismissalRecord{
		"rec-4": {
			RecommendationID: "rec-4",
			Status:           config.StatusDismissed,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID: "urn:pulumi:stack::proj::aws:ec2:Instance::my-instance",
				Type:       "Right-sizing",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	require.Len(t, row.Recommendations, 1)
	assert.Equal(t, RecommendationStatusDismissed, row.Recommendations[0].Status)
}

func TestApplyDismissalDeltaToRow_NonMatchingResourceIDSkipped(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-111", Type: "aws:ec2:Instance"}
	records := map[string]*config.DismissalRecord{
		"rec-5": {
			RecommendationID: "rec-5",
			Status:           config.StatusDismissed,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID: "i-999", // Different resource — should not match.
				Type:       "Right-sizing",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	assert.Empty(t, row.Recommendations)
}

func TestApplyDismissalDeltaToRow_MultipleMatchesAppended(t *testing.T) {
	row := OverviewRow{URN: "urn:test", ResourceID: "i-222", Type: "aws:ec2:Instance"}
	records := map[string]*config.DismissalRecord{
		"rec-a": {
			RecommendationID: "rec-a",
			Status:           config.StatusDismissed,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID: "i-222",
				Type:       "Right-sizing",
			},
		},
		"rec-b": {
			RecommendationID: "rec-b",
			Status:           config.StatusSnoozed,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID: "i-222",
				Type:       "Terminate",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	assert.Len(t, row.Recommendations, 2)

	statusSet := make(map[RecommendationStatus]bool)
	for _, r := range row.Recommendations {
		statusSet[r.Status] = true
	}
	assert.True(t, statusSet[RecommendationStatusDismissed])
	assert.True(t, statusSet[RecommendationStatusSnoozed])
}

func TestApplyDismissalDeltaToRow_PreservesExistingActiveRecs(t *testing.T) {
	row := OverviewRow{
		URN:        "urn:test",
		ResourceID: "i-333",
		Type:       "aws:ec2:Instance",
		Recommendations: []Recommendation{
			{ResourceID: "i-333", Type: "Purchase Commitment", Status: RecommendationStatusActive},
		},
	}
	records := map[string]*config.DismissalRecord{
		"rec-z": {
			RecommendationID: "rec-z",
			Status:           config.StatusDismissed,
			LastKnown: &config.LastKnownRecommendation{
				ResourceID: "i-333",
				Type:       "Right-sizing",
			},
		},
	}
	ApplyDismissalDeltaToRow(&row, records)
	require.Len(t, row.Recommendations, 2)
	// First rec remains active.
	assert.Equal(t, RecommendationStatusActive, row.Recommendations[0].Status)
	// Second rec is the appended dismissed stub.
	assert.Equal(t, RecommendationStatusDismissed, row.Recommendations[1].Status)
}

// ---------------------------------------------------------------------------
// CountRecsActiveAndDismissed
// ---------------------------------------------------------------------------

func TestCountRecsActiveAndDismissed(t *testing.T) {
	tests := []struct {
		name          string
		recs          []Recommendation
		wantActive    int
		wantDismissed int
	}{
		{
			name:          "empty",
			recs:          nil,
			wantActive:    0,
			wantDismissed: 0,
		},
		{
			name: "all_active",
			recs: []Recommendation{
				{Status: RecommendationStatusActive},
				{Status: ""},
			},
			wantActive:    2,
			wantDismissed: 0,
		},
		{
			name: "all_dismissed",
			recs: []Recommendation{
				{Status: RecommendationStatusDismissed},
				{Status: RecommendationStatusSnoozed},
			},
			wantActive:    0,
			wantDismissed: 2,
		},
		{
			name: "mixed",
			recs: []Recommendation{
				{Status: RecommendationStatusActive},
				{Status: RecommendationStatusDismissed},
				{Status: ""},
				{Status: RecommendationStatusSnoozed},
			},
			wantActive:    2,
			wantDismissed: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, dismissed := CountRecsActiveAndDismissed(tt.recs)
			assert.Equal(t, tt.wantActive, active, "active count mismatch")
			assert.Equal(t, tt.wantDismissed, dismissed, "dismissed count mismatch")
		})
	}
}
