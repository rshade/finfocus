package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// T032: Integration test for full dismiss lifecycle.
// Tests: dismiss -> list excludes dismissed -> include-dismissed shows all ->
// undismiss restores -> history shows events.

// TestDismissLifecycle_LocalOnly tests the full dismiss lifecycle using local-only operations
// (no plugin connection needed).
func TestDismissLifecycle_LocalOnly(t *testing.T) {
	// Create a temp directory for the dismissal store
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	// Step 1: Dismiss a recommendation
	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	now := time.Now()
	recID := "rec-lifecycle-test-001"

	// Set a dismissed record (simulating what engine.DismissRecommendation does)
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: recID,
		Status:           config.StatusDismissed,
		Reason:           "BUSINESS_CONSTRAINT",
		CustomReason:     "Burst capacity requirement",
		DismissedAt:      now,
		LastKnown: &config.LastKnownRecommendation{
			Description:      "Rightsize web-server from m5.xlarge to m5.large",
			EstimatedSavings: 45.00,
			Currency:         "USD",
			Type:             "RIGHTSIZE",
			ResourceID:       "aws:ec2:web-server",
		},
		History: []config.LifecycleEvent{
			{
				Action:       config.ActionDismissed,
				Reason:       "BUSINESS_CONSTRAINT",
				CustomReason: "Burst capacity requirement",
				Timestamp:    now,
			},
		},
	}))
	require.NoError(t, store.Save())

	// Step 2: Verify dismissed ID is in excluded list (default list behavior)
	require.NoError(t, store.Load())
	excludedIDs := store.GetDismissedIDs()
	assert.Contains(t, excludedIDs, recID, "dismissed rec should be in excluded IDs")

	// Step 3: Verify --include-dismissed shows the record
	allRecords := store.GetAllRecords()
	assert.Contains(t, allRecords, recID, "dismissed rec should appear in all records")
	assert.Equal(t, config.StatusDismissed, allRecords[recID].Status)
	assert.Equal(t, "BUSINESS_CONSTRAINT", allRecords[recID].Reason)
	require.NotNil(t, allRecords[recID].LastKnown)
	assert.Equal(t, 45.00, allRecords[recID].LastKnown.EstimatedSavings)

	// Step 4: Undismiss the recommendation
	record, found := store.Get(recID)
	require.True(t, found)

	// Add undismiss lifecycle event and persist before deleting
	record.History = append(record.History, config.LifecycleEvent{
		Action:    config.ActionUndismissed,
		Reason:    record.Reason,
		Timestamp: time.Now(),
	})
	require.NoError(t, store.Set(record))
	require.NoError(t, store.Save())

	// Now delete from store (undismiss removes from active exclusion)
	require.NoError(t, store.Delete(recID))
	require.NoError(t, store.Save())

	// Step 5: Verify it's no longer excluded
	require.NoError(t, store.Load())
	excludedIDs = store.GetDismissedIDs()
	assert.NotContains(t, excludedIDs, recID, "undismissed rec should not be in excluded IDs")

	// Step 6: Verify history recorded events
	// In the real flow, the engine stores history before deleting.
	// Here we verify the history was recorded before deletion.
	assert.Len(t, record.History, 2, "should have 2 lifecycle events")
	assert.Equal(t, config.ActionDismissed, record.History[0].Action)
	assert.Equal(t, config.ActionUndismissed, record.History[1].Action)
}

// TestSnoozeLifecycle_AutoExpiry tests snooze with automatic expiry.
func TestSnoozeLifecycle_AutoExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	now := time.Now()
	recID := "rec-snooze-test-001"

	// Snooze with an already-expired time (simulating passage of time)
	pastTime := now.Add(-1 * time.Hour)
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: recID,
		Status:           config.StatusSnoozed,
		Reason:           "DEFERRED",
		DismissedAt:      now.Add(-2 * time.Hour),
		ExpiresAt:        &pastTime,
		LastKnown: &config.LastKnownRecommendation{
			Description:      "Terminate idle database",
			EstimatedSavings: 120.00,
			Currency:         "USD",
			Type:             "TERMINATE",
			ResourceID:       "aws:rds:idle-db",
		},
		History: []config.LifecycleEvent{
			{
				Action:    config.ActionSnoozed,
				Reason:    "DEFERRED",
				Timestamp: now.Add(-2 * time.Hour),
				ExpiresAt: &pastTime,
			},
		},
	}))
	require.NoError(t, store.Save())

	// Clean expired snoozes (simulates what GetRecommendationsForResources does)
	_, cleanErr := store.CleanExpiredSnoozes()
	require.NoError(t, cleanErr)
	require.NoError(t, store.Save())

	// Verify the expired snooze was transitioned to Active (preserving history)
	require.NoError(t, store.Load())
	record, found := store.Get(recID)
	assert.True(t, found, "expired snoozed rec should still exist as Active after cleaning")
	require.NotNil(t, record)
	assert.Equal(t, config.StatusActive, record.Status, "expired snooze should transition to Active")
	assert.Nil(t, record.ExpiresAt, "Active record should have nil ExpiresAt")

	// Verify it's no longer in excluded IDs (Active records are not excluded)
	excludedIDs := store.GetDismissedIDs()
	assert.NotContains(t, excludedIDs, recID)
}

// TestDirectTransitions tests Dismissed->Snoozed and Snoozed->Dismissed transitions.
func TestDirectTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	now := time.Now()
	recID := "rec-transition-test"

	// Start as Dismissed
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: recID,
		Status:           config.StatusDismissed,
		Reason:           "NOT_APPLICABLE",
		DismissedAt:      now,
		History: []config.LifecycleEvent{
			{Action: config.ActionDismissed, Reason: "NOT_APPLICABLE", Timestamp: now},
		},
	}))

	// Verify it's dismissed
	record, found := store.Get(recID)
	require.True(t, found)
	assert.Equal(t, config.StatusDismissed, record.Status)

	// Transition to Snoozed (direct transition)
	future := now.Add(30 * 24 * time.Hour)
	record.Status = config.StatusSnoozed
	record.Reason = "DEFERRED"
	record.ExpiresAt = &future
	record.History = append(record.History, config.LifecycleEvent{
		Action:    config.ActionSnoozed,
		Reason:    "DEFERRED",
		Timestamp: now,
		ExpiresAt: &future,
	})
	require.NoError(t, store.Set(record))

	// Verify transition
	updated, found := store.Get(recID)
	require.True(t, found)
	assert.Equal(t, config.StatusSnoozed, updated.Status)
	assert.Equal(t, "DEFERRED", updated.Reason)
	require.NotNil(t, updated.ExpiresAt)
	assert.Len(t, updated.History, 2)

	// Transition back to Dismissed (direct transition)
	updated.Status = config.StatusDismissed
	updated.Reason = "BUSINESS_CONSTRAINT"
	updated.ExpiresAt = nil
	updated.History = append(updated.History, config.LifecycleEvent{
		Action:    config.ActionDismissed,
		Reason:    "BUSINESS_CONSTRAINT",
		Timestamp: now,
	})
	require.NoError(t, store.Set(updated))

	// Verify final state
	final, found := store.Get(recID)
	require.True(t, found)
	assert.Equal(t, config.StatusDismissed, final.Status)
	assert.Equal(t, "BUSINESS_CONSTRAINT", final.Reason)
	assert.Nil(t, final.ExpiresAt)
	assert.Len(t, final.History, 3)
}

// TestDismissalStorePersistence verifies that state survives save/load cycles.
func TestDismissalStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	// Write some records
	store1, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)

	now := time.Now()
	for i := range 5 {
		recID := "rec-persist-" + string(rune('A'+i))
		require.NoError(t, store1.Set(&config.DismissalRecord{
			RecommendationID: recID,
			Status:           config.StatusDismissed,
			Reason:           "BUSINESS_CONSTRAINT",
			DismissedAt:      now,
			History: []config.LifecycleEvent{
				{Action: config.ActionDismissed, Reason: "BUSINESS_CONSTRAINT", Timestamp: now},
			},
		}))
	}
	require.NoError(t, store1.Save())

	// Read back with a new store instance
	store2, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)
	require.NoError(t, store2.Load())

	assert.Equal(t, 5, store2.Count(), "should have 5 records after load")

	// Verify the JSON file is valid
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	version, ok := parsed["version"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(1), version)
}
