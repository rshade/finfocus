package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// T007: Unit tests for Engine.DismissRecommendation.
// T022: Unit tests for Engine.UndismissRecommendation.
// T026: Unit tests for Engine.GetRecommendationHistory.

// createTestStore creates a DismissalStore backed by a temporary file.
func createTestStore(t *testing.T) *config.DismissalStore {
	t.Helper()
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)
	require.NoError(t, store.Load())

	return store
}

// createTestEngine creates a minimal Engine for dismiss testing (no plugins).
func createTestEngine() *Engine {
	return New(nil, nil)
}

// T007: Test Engine.DismissRecommendation with various scenarios.
func TestEngine_DismissRecommendation(t *testing.T) {
	ctx := context.Background()

	t.Run("dismiss with valid reason", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		req := DismissRequest{
			RecommendationID: "rec-123",
			Reason:           "business-constraint",
			CustomReason:     "Intentional oversizing for burst capacity",
		}

		result, err := engine.DismissRecommendation(ctx, store, req)
		require.NoError(t, err)
		assert.Equal(t, "rec-123", result.RecommendationID)
		assert.True(t, result.LocalPersisted)
		assert.False(t, result.PluginDismissed) // No plugins

		// Verify store persistence
		record, ok := store.Get("rec-123")
		require.True(t, ok)
		assert.Equal(t, config.StatusDismissed, record.Status)
		assert.Equal(t, "DISMISSAL_REASON_BUSINESS_CONSTRAINT", record.Reason)
		assert.Equal(t, "Intentional oversizing for burst capacity", record.CustomReason)
	})

	t.Run("snooze with expiry", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		req := DismissRequest{
			RecommendationID: "rec-456",
			Reason:           "deferred",
			CustomReason:     "Q2 review",
			ExpiresAt:        &expiresAt,
		}

		result, err := engine.DismissRecommendation(ctx, store, req)
		require.NoError(t, err)
		assert.True(t, result.LocalPersisted)

		record, ok := store.Get("rec-456")
		require.True(t, ok)
		assert.Equal(t, config.StatusSnoozed, record.Status)
		require.NotNil(t, record.ExpiresAt)
		assert.WithinDuration(t, expiresAt, *record.ExpiresAt, time.Second)
	})

	t.Run("empty recommendation ID returns error", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		req := DismissRequest{
			RecommendationID: "",
			Reason:           "business-constraint",
		}

		_, err := engine.DismissRecommendation(ctx, store, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recommendation ID is required")
	})

	t.Run("invalid reason returns error", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		req := DismissRequest{
			RecommendationID: "rec-invalid",
			Reason:           "not-a-valid-reason",
		}

		_, err := engine.DismissRecommendation(ctx, store, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid dismissal reason")
	})

	t.Run("direct transition dismissed to snoozed", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// First dismiss permanently
		req1 := DismissRequest{
			RecommendationID: "rec-transition",
			Reason:           "business-constraint",
		}
		_, err := engine.DismissRecommendation(ctx, store, req1)
		require.NoError(t, err)

		record, ok := store.Get("rec-transition")
		require.True(t, ok)
		assert.Equal(t, config.StatusDismissed, record.Status)
		assert.Nil(t, record.ExpiresAt)

		// Now snooze (direct transition Dismissed -> Snoozed, FR-010a)
		expiresAt := time.Now().Add(30 * 24 * time.Hour)
		req2 := DismissRequest{
			RecommendationID: "rec-transition",
			Reason:           "deferred",
			ExpiresAt:        &expiresAt,
		}
		result, err := engine.DismissRecommendation(ctx, store, req2)
		require.NoError(t, err)
		assert.True(t, result.LocalPersisted)

		record, ok = store.Get("rec-transition")
		require.True(t, ok)
		assert.Equal(t, config.StatusSnoozed, record.Status)
		require.NotNil(t, record.ExpiresAt)
		// History should have 2 events
		assert.Len(t, record.History, 2)
	})

	t.Run("re-snooze updates expiry date", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// First snooze
		expiresAt1 := time.Now().Add(7 * 24 * time.Hour)
		req1 := DismissRequest{
			RecommendationID: "rec-resnooze",
			Reason:           "deferred",
			ExpiresAt:        &expiresAt1,
		}
		_, err := engine.DismissRecommendation(ctx, store, req1)
		require.NoError(t, err)

		// Re-snooze with new date (FR-010a)
		expiresAt2 := time.Now().Add(60 * 24 * time.Hour)
		req2 := DismissRequest{
			RecommendationID: "rec-resnooze",
			Reason:           "deferred",
			ExpiresAt:        &expiresAt2,
		}
		_, err = engine.DismissRecommendation(ctx, store, req2)
		require.NoError(t, err)

		record, ok := store.Get("rec-resnooze")
		require.True(t, ok)
		assert.Equal(t, config.StatusSnoozed, record.Status)
		require.NotNil(t, record.ExpiresAt)
		assert.WithinDuration(t, expiresAt2, *record.ExpiresAt, time.Second)
		// History should have 2 snooze events
		assert.Len(t, record.History, 2)
	})

	t.Run("snoozed to dismissed direct transition", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// First snooze
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		req1 := DismissRequest{
			RecommendationID: "rec-snooze-to-dismiss",
			Reason:           "deferred",
			ExpiresAt:        &expiresAt,
		}
		_, err := engine.DismissRecommendation(ctx, store, req1)
		require.NoError(t, err)

		// Dismiss permanently (direct transition Snoozed -> Dismissed, FR-010a)
		req2 := DismissRequest{
			RecommendationID: "rec-snooze-to-dismiss",
			Reason:           "business-constraint",
			CustomReason:     "Decided to keep permanently",
			ExpiresAt:        nil,
		}
		result, err := engine.DismissRecommendation(ctx, store, req2)
		require.NoError(t, err)
		assert.True(t, result.LocalPersisted)

		record, ok := store.Get("rec-snooze-to-dismiss")
		require.True(t, ok)
		assert.Equal(t, config.StatusDismissed, record.Status)
		assert.Nil(t, record.ExpiresAt)
		assert.Len(t, record.History, 2)
	})

	t.Run("preserves LastKnown recommendation details", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		req := DismissRequest{
			RecommendationID: "rec-with-details",
			Reason:           "not-applicable",
			Recommendation: &Recommendation{
				ResourceID:       "i-abc123",
				Type:             "RIGHTSIZE",
				Description:      "Resize instance to t3.small",
				EstimatedSavings: 25.50,
				Currency:         "USD",
			},
		}

		result, err := engine.DismissRecommendation(ctx, store, req)
		require.NoError(t, err)
		assert.True(t, result.LocalPersisted)

		record, ok := store.Get("rec-with-details")
		require.True(t, ok)
		require.NotNil(t, record.LastKnown)
		assert.Equal(t, "i-abc123", record.LastKnown.ResourceID)
		assert.Equal(t, "RIGHTSIZE", record.LastKnown.Type)
		assert.Equal(t, "Resize instance to t3.small", record.LastKnown.Description)
		assert.Equal(t, 25.50, record.LastKnown.EstimatedSavings)
		assert.Equal(t, "USD", record.LastKnown.Currency)
	})

	t.Run("all valid dismissal reasons", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		reasons := []string{
			"not-applicable",
			"already-implemented",
			"business-constraint",
			"technical-constraint",
			"deferred",
			"inaccurate",
			"other",
		}

		for i, reason := range reasons {
			req := DismissRequest{
				RecommendationID: "rec-reason-" + reason,
				Reason:           reason,
				CustomReason:     "Note for " + reason,
			}

			result, err := engine.DismissRecommendation(ctx, store, req)
			require.NoError(t, err, "reason %d (%s) should not error", i, reason)
			assert.True(t, result.LocalPersisted, "reason %d (%s) should persist locally", i, reason)
		}
	})
}

// T022: Test Engine.UndismissRecommendation.
func TestEngine_UndismissRecommendation(t *testing.T) {
	ctx := context.Background()

	t.Run("undismiss dismissed record", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// First dismiss
		req := DismissRequest{
			RecommendationID: "rec-undismiss",
			Reason:           "business-constraint",
		}
		_, err := engine.DismissRecommendation(ctx, store, req)
		require.NoError(t, err)

		// Verify dismissed
		_, ok := store.Get("rec-undismiss")
		require.True(t, ok)

		// Undismiss
		result, err := engine.UndismissRecommendation(ctx, store, "rec-undismiss")
		require.NoError(t, err)
		assert.True(t, result.WasDismissed)
		assert.Contains(t, result.Message, "undismissed")

		// Verify record is preserved with StatusActive and history
		record, ok := store.Get("rec-undismiss")
		require.True(t, ok, "record should be preserved after undismiss")
		assert.Equal(t, config.StatusActive, record.Status)
		require.Len(t, record.History, 2) // dismissed + undismissed
		assert.Equal(t, config.ActionUndismissed, record.History[1].Action)
	})

	t.Run("undismiss snoozed record", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// First snooze
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		req := DismissRequest{
			RecommendationID: "rec-undismiss-snooze",
			Reason:           "deferred",
			ExpiresAt:        &expiresAt,
		}
		_, err := engine.DismissRecommendation(ctx, store, req)
		require.NoError(t, err)

		// Undismiss before expiry
		result, err := engine.UndismissRecommendation(ctx, store, "rec-undismiss-snooze")
		require.NoError(t, err)
		assert.True(t, result.WasDismissed)

		// Verify record is preserved with StatusActive
		record, ok := store.Get("rec-undismiss-snooze")
		require.True(t, ok, "record should be preserved after undismiss")
		assert.Equal(t, config.StatusActive, record.Status)
		assert.Nil(t, record.ExpiresAt)
	})

	t.Run("undismiss non-dismissed ID returns informational message", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		result, err := engine.UndismissRecommendation(ctx, store, "rec-never-dismissed")
		require.NoError(t, err)
		assert.False(t, result.WasDismissed)
		assert.Contains(t, result.Message, "not dismissed")
	})

	t.Run("empty recommendation ID returns error", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		_, err := engine.UndismissRecommendation(ctx, store, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recommendation ID is required")
	})
}

// T026: Test Engine.GetRecommendationHistory.
func TestEngine_GetRecommendationHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("returns history in chronological order", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// Dismiss
		req1 := DismissRequest{
			RecommendationID: "rec-history",
			Reason:           "business-constraint",
		}
		_, err := engine.DismissRecommendation(ctx, store, req1)
		require.NoError(t, err)

		// Get history
		history, err := engine.GetRecommendationHistory(ctx, store, "rec-history")
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, config.ActionDismissed, history[0].Action)
	})

	t.Run("returns empty for unknown ID", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		history, err := engine.GetRecommendationHistory(ctx, store, "rec-unknown")
		require.NoError(t, err)
		assert.Empty(t, history)
	})

	t.Run("returns empty for active ID", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// Never dismissed recommendation should have no history
		history, err := engine.GetRecommendationHistory(ctx, store, "rec-active")
		require.NoError(t, err)
		assert.Empty(t, history)
	})

	t.Run("empty recommendation ID returns error", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		_, err := engine.GetRecommendationHistory(ctx, store, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recommendation ID is required")
	})

	t.Run("multiple lifecycle events in history", func(t *testing.T) {
		engine := createTestEngine()
		store := createTestStore(t)

		// Dismiss
		req1 := DismissRequest{
			RecommendationID: "rec-multi-history",
			Reason:           "business-constraint",
		}
		_, err := engine.DismissRecommendation(ctx, store, req1)
		require.NoError(t, err)

		// Snooze (direct transition)
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		req2 := DismissRequest{
			RecommendationID: "rec-multi-history",
			Reason:           "deferred",
			ExpiresAt:        &expiresAt,
		}
		_, err = engine.DismissRecommendation(ctx, store, req2)
		require.NoError(t, err)

		// Get history
		history, err := engine.GetRecommendationHistory(ctx, store, "rec-multi-history")
		require.NoError(t, err)
		require.Len(t, history, 2)
		assert.Equal(t, config.ActionDismissed, history[0].Action)
		assert.Equal(t, config.ActionSnoozed, history[1].Action)
	})
}

// TestDismissalStore_ExcludedIDs_ForEngineFiltering validates the DismissalStore load/extract path.
func TestDismissalStore_ExcludedIDs_ForEngineFiltering(t *testing.T) {
	// This test verifies that T012 is implemented correctly:
	// GetRecommendationsForResources should load DismissalStore,
	// extract dismissed IDs, and pass them to plugins.

	// Note: Full integration testing with plugins requires mock plugins.
	// This test verifies the store loading and ID extraction paths.

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	// Pre-populate the store with some dismissed recommendations
	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)
	require.NoError(t, store.Load())

	// Add a dismissed record
	record := &config.DismissalRecord{
		RecommendationID: "rec-excluded",
		Status:           config.StatusDismissed,
		Reason:           "DISMISSAL_REASON_BUSINESS_CONSTRAINT",
		DismissedAt:      time.Now(),
		History: []config.LifecycleEvent{
			{
				Action:    config.ActionDismissed,
				Reason:    "DISMISSAL_REASON_BUSINESS_CONSTRAINT",
				Timestamp: time.Now(),
			},
		},
	}
	require.NoError(t, store.Set(record))
	require.NoError(t, store.Save())

	// Verify the dismissed IDs are retrievable
	ids := store.GetDismissedIDs()
	assert.Contains(t, ids, "rec-excluded")
}

// Test snooze expiry handling in GetDismissedIDs.
func TestEngine_ExpiredSnoozeHandling(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "dismissed.json")

	store, err := config.NewDismissalStore(storePath)
	require.NoError(t, err)
	require.NoError(t, store.Load())

	// Add a snoozed record that has already expired
	expiredTime := time.Now().Add(-24 * time.Hour) // Yesterday
	record := &config.DismissalRecord{
		RecommendationID: "rec-expired-snooze",
		Status:           config.StatusSnoozed,
		Reason:           "DISMISSAL_REASON_DEFERRED",
		DismissedAt:      time.Now().Add(-48 * time.Hour),
		ExpiresAt:        &expiredTime,
		History: []config.LifecycleEvent{
			{
				Action:    config.ActionSnoozed,
				Reason:    "DISMISSAL_REASON_DEFERRED",
				Timestamp: time.Now().Add(-48 * time.Hour),
				ExpiresAt: &expiredTime,
			},
		},
	}
	require.NoError(t, store.Set(record))
	require.NoError(t, store.Save())

	// Clean expired snoozes
	_, err = store.CleanExpiredSnoozes()
	require.NoError(t, err)

	// Expired snooze should be preserved with StatusActive
	record2, ok := store.Get("rec-expired-snooze")
	require.True(t, ok, "expired snooze should be preserved as active")
	assert.Equal(t, config.StatusActive, record2.Status)
	assert.Nil(t, record2.ExpiresAt)
}

// Verify dismissal state file path is correct.
func TestDismissalStore_DefaultPath(t *testing.T) {
	// Skip if running in CI without HOME set
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	store, err := config.NewDismissalStore("")
	require.NoError(t, err)

	expectedPath := filepath.Join(homeDir, ".finfocus", "dismissed.json")
	// We can verify the store's file path matches the expected default
	assert.Equal(t, expectedPath, store.FilePath())
}
