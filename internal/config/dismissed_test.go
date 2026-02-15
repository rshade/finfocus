package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDismissalStore(t *testing.T) {
	t.Parallel()

	t.Run("with explicit path", func(t *testing.T) {
		t.Parallel()
		expected := filepath.Join(t.TempDir(), "test-dismissed.json")
		store, err := NewDismissalStore(expected)
		require.NoError(t, err)
		assert.Equal(t, expected, store.FilePath())
	})

	t.Run("with empty path defaults to home dir", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore("")
		require.NoError(t, err)
		assert.Contains(t, store.FilePath(), "dismissed.json")
	})
}

func TestDismissalStore_LoadSave(t *testing.T) {
	t.Parallel()

	t.Run("missing file starts empty", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		store, err := NewDismissalStore(filepath.Join(dir, "dismissed.json"))
		require.NoError(t, err)

		err = store.Load()
		require.NoError(t, err)
		assert.Equal(t, 0, store.Count())
	})

	t.Run("save and load round trip", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "dismissed.json")

		// Create and save
		store1, err := NewDismissalStore(filePath)
		require.NoError(t, err)

		now := time.Now().Truncate(time.Second)
		record := &DismissalRecord{
			RecommendationID: "rec-123",
			Status:           StatusDismissed,
			Reason:           "BUSINESS_CONSTRAINT",
			CustomReason:     "Burst capacity",
			DismissedAt:      now,
			ExpiresAt:        nil,
			LastKnown: &LastKnownRecommendation{
				Description:      "Rightsize instance",
				EstimatedSavings: 45.0,
				Currency:         "USD",
				Type:             "RIGHTSIZE",
				ResourceID:       "aws:ec2:Instance::web-server",
			},
			History: []LifecycleEvent{
				{
					Action:    ActionDismissed,
					Reason:    "BUSINESS_CONSTRAINT",
					Timestamp: now,
				},
			},
		}

		require.NoError(t, store1.Set(record))
		require.NoError(t, store1.Save())

		// Load in new store instance
		store2, err := NewDismissalStore(filePath)
		require.NoError(t, err)
		require.NoError(t, store2.Load())

		assert.Equal(t, 1, store2.Count())

		loaded, ok := store2.Get("rec-123")
		require.True(t, ok)
		assert.Equal(t, "rec-123", loaded.RecommendationID)
		assert.Equal(t, StatusDismissed, loaded.Status)
		assert.Equal(t, "BUSINESS_CONSTRAINT", loaded.Reason)
		assert.Equal(t, "Burst capacity", loaded.CustomReason)
		require.NotNil(t, loaded.LastKnown)
		assert.Equal(t, "Rightsize instance", loaded.LastKnown.Description)
		assert.InDelta(t, 45.0, loaded.LastKnown.EstimatedSavings, 0.01)
		assert.Len(t, loaded.History, 1)
		assert.Equal(t, ActionDismissed, loaded.History[0].Action)
	})

	t.Run("corrupted file returns ErrStoreCorrupted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "dismissed.json")

		require.NoError(t, os.WriteFile(filePath, []byte("{invalid json"), 0o644))

		store, err := NewDismissalStore(filePath)
		require.NoError(t, err)

		err = store.Load()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrStoreCorrupted))
		assert.Equal(t, 0, store.Count())
	})

	t.Run("version mismatch returns ErrStoreCorrupted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		filePath := filepath.Join(dir, "dismissed.json")

		data := []byte(`{"version": 99, "dismissals": {}}`)
		require.NoError(t, os.WriteFile(filePath, data, 0o644))

		store, err := NewDismissalStore(filePath)
		require.NoError(t, err)

		err = store.Load()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrStoreCorrupted))
		assert.Equal(t, 0, store.Count())
	})
}

func TestDismissalStore_GetSetDelete(t *testing.T) {
	t.Parallel()

	t.Run("get nonexistent returns false", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		record, ok := store.Get("nonexistent")
		assert.False(t, ok)
		assert.Nil(t, record)
	})

	t.Run("set and get", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		record := &DismissalRecord{
			RecommendationID: "rec-1",
			Status:           StatusDismissed,
			Reason:           "DEFERRED",
			DismissedAt:      time.Now(),
		}

		require.NoError(t, store.Set(record))

		loaded, ok := store.Get("rec-1")
		require.True(t, ok)
		assert.Equal(t, "rec-1", loaded.RecommendationID)
		assert.Equal(t, StatusDismissed, loaded.Status)
	})

	t.Run("set nil record returns error", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		err = store.Set(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("set empty ID returns error", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		err = store.Set(&DismissalRecord{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recommendation ID cannot be empty")
	})

	t.Run("delete existing record", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		record := &DismissalRecord{
			RecommendationID: "rec-del",
			Status:           StatusDismissed,
			Reason:           "OTHER",
			DismissedAt:      time.Now(),
		}

		require.NoError(t, store.Set(record))
		assert.Equal(t, 1, store.Count())

		require.NoError(t, store.Delete("rec-del"))
		assert.Equal(t, 0, store.Count())

		_, ok := store.Get("rec-del")
		assert.False(t, ok)
	})

	t.Run("delete nonexistent is no-op", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		err = store.Delete("nonexistent")
		require.NoError(t, err)
	})

	t.Run("delete empty ID returns error", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		err = store.Delete("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recommendation ID cannot be empty")
	})

	t.Run("set overwrites existing record", func(t *testing.T) {
		t.Parallel()
		store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
		require.NoError(t, err)

		record1 := &DismissalRecord{
			RecommendationID: "rec-overwrite",
			Status:           StatusDismissed,
			Reason:           "DEFERRED",
			DismissedAt:      time.Now(),
		}
		require.NoError(t, store.Set(record1))

		record2 := &DismissalRecord{
			RecommendationID: "rec-overwrite",
			Status:           StatusSnoozed,
			Reason:           "BUSINESS_CONSTRAINT",
			DismissedAt:      time.Now(),
		}
		require.NoError(t, store.Set(record2))

		loaded, ok := store.Get("rec-overwrite")
		require.True(t, ok)
		assert.Equal(t, StatusSnoozed, loaded.Status)
		assert.Equal(t, "BUSINESS_CONSTRAINT", loaded.Reason)
		assert.Equal(t, 1, store.Count())
	})
}

func TestDismissalStore_GetDismissedIDs(t *testing.T) {
	t.Parallel()

	store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
	require.NoError(t, err)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	// Permanently dismissed
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-dismissed",
		Status:           StatusDismissed,
		Reason:           "DEFERRED",
		DismissedAt:      now,
	}))

	// Active snooze (future expiry)
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-snoozed-active",
		Status:           StatusSnoozed,
		Reason:           "DEFERRED",
		DismissedAt:      now,
		ExpiresAt:        &future,
	}))

	// Expired snooze (past expiry)
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-snoozed-expired",
		Status:           StatusSnoozed,
		Reason:           "DEFERRED",
		DismissedAt:      past,
		ExpiresAt:        &past,
	}))

	ids := store.GetDismissedIDs()

	// Should include dismissed and active snooze, but NOT expired snooze
	assert.Contains(t, ids, "rec-dismissed")
	assert.Contains(t, ids, "rec-snoozed-active")
	assert.NotContains(t, ids, "rec-snoozed-expired")
	assert.Len(t, ids, 2)
}

func TestDismissalStore_GetAllRecords(t *testing.T) {
	t.Parallel()

	store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
	require.NoError(t, err)

	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-1",
		Status:           StatusDismissed,
		DismissedAt:      time.Now(),
	}))
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-2",
		Status:           StatusSnoozed,
		DismissedAt:      time.Now(),
	}))

	records := store.GetAllRecords()
	assert.Len(t, records, 2)
	assert.Contains(t, records, "rec-1")
	assert.Contains(t, records, "rec-2")
}

func TestDismissalStore_GetExpiredSnoozes(t *testing.T) {
	t.Parallel()

	store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
	require.NoError(t, err)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-expired",
		Status:           StatusSnoozed,
		DismissedAt:      past,
		ExpiresAt:        &past,
	}))
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-active",
		Status:           StatusSnoozed,
		DismissedAt:      now,
		ExpiresAt:        &future,
	}))
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-permanent",
		Status:           StatusDismissed,
		DismissedAt:      now,
	}))

	expired := store.GetExpiredSnoozes()
	assert.Len(t, expired, 1)
	assert.Equal(t, "rec-expired", expired[0].RecommendationID)
}

func TestDismissalStore_CleanExpiredSnoozes(t *testing.T) {
	t.Parallel()

	store, err := NewDismissalStore(filepath.Join(t.TempDir(), "d.json"))
	require.NoError(t, err)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-expired",
		Status:           StatusSnoozed,
		Reason:           "DEFERRED",
		DismissedAt:      past,
		ExpiresAt:        &past,
		History: []LifecycleEvent{
			{Action: ActionSnoozed, Reason: "DEFERRED", Timestamp: past},
		},
	}))
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-active",
		Status:           StatusSnoozed,
		DismissedAt:      now,
		ExpiresAt:        &future,
	}))
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-permanent",
		Status:           StatusDismissed,
		DismissedAt:      now,
	}))

	_, cleanErr := store.CleanExpiredSnoozes()
	require.NoError(t, cleanErr)

	// Expired snooze should be preserved with StatusActive and history
	record, ok := store.Get("rec-expired")
	require.True(t, ok, "expired snooze should be preserved as active")
	assert.Equal(t, StatusActive, record.Status)
	assert.Nil(t, record.ExpiresAt)
	require.Len(t, record.History, 2) // snoozed + undismissed
	assert.Equal(t, ActionUndismissed, record.History[1].Action)

	// Active snooze and permanent dismissal remain
	_, ok = store.Get("rec-active")
	assert.True(t, ok)
	_, ok = store.Get("rec-permanent")
	assert.True(t, ok)

	assert.Equal(t, 3, store.Count())
}

func TestDismissalStore_SaveCreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "nested", "deep", "dismissed.json")

	store, err := NewDismissalStore(nestedPath)
	require.NoError(t, err)

	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "rec-1",
		Status:           StatusDismissed,
		DismissedAt:      time.Now(),
	}))

	require.NoError(t, store.Save())

	// Verify file was created
	_, err = os.Stat(nestedPath)
	require.NoError(t, err)
}

func TestNewDismissalStore_ProjectAware(t *testing.T) {
	// Not parallel: subtests mutate package-level resolvedProjectDir and use t.Setenv.

	t.Run("uses project dir when set", func(t *testing.T) {
		// Set up project dir
		projectDir := filepath.Join(t.TempDir(), "project", ".finfocus")

		// Save and restore original resolved project dir
		orig := GetResolvedProjectDir()
		t.Cleanup(func() { SetResolvedProjectDir(orig) })

		SetResolvedProjectDir(projectDir)

		store, err := NewDismissalStore("")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(projectDir, "dismissed.json"), store.FilePath())
	})

	t.Run("falls back to ResolveConfigDir when no project", func(t *testing.T) {
		// Clear project dir
		orig := GetResolvedProjectDir()
		t.Cleanup(func() { SetResolvedProjectDir(orig) })
		SetResolvedProjectDir("")

		store, err := NewDismissalStore("")
		require.NoError(t, err)

		// Should use ResolveConfigDir() which uses FINFOCUS_HOME, etc
		expected := filepath.Join(ResolveConfigDir(), "dismissed.json")
		assert.Equal(t, expected, store.FilePath())
	})

	t.Run("explicit filePath takes precedence over project dir", func(t *testing.T) {
		orig := GetResolvedProjectDir()
		t.Cleanup(func() { SetResolvedProjectDir(orig) })
		SetResolvedProjectDir("/some/project/.finfocus")

		explicitPath := filepath.Join(t.TempDir(), "custom-dismissed.json")
		store, err := NewDismissalStore(explicitPath)
		require.NoError(t, err)
		assert.Equal(t, explicitPath, store.FilePath())
	})

	t.Run("respects FINFOCUS_HOME when no project", func(t *testing.T) {
		orig := GetResolvedProjectDir()
		t.Cleanup(func() { SetResolvedProjectDir(orig) })
		SetResolvedProjectDir("")

		customHome := t.TempDir()
		t.Setenv("FINFOCUS_HOME", customHome)

		store, err := NewDismissalStore("")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(customHome, "dismissed.json"), store.FilePath())
	})
}

func TestNewDismissalStore_LoadWithProjectContext(t *testing.T) {
	// Not parallel: mutates package-level resolvedProjectDir.

	projectDir := filepath.Join(t.TempDir(), "myproject", ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	orig := GetResolvedProjectDir()
	t.Cleanup(func() { SetResolvedProjectDir(orig) })
	SetResolvedProjectDir(projectDir)

	// Simulates what loadDismissalStore does in the CLI
	store, err := NewDismissalStore("")
	require.NoError(t, err)

	// Load should succeed (empty file = empty store)
	require.NoError(t, store.Load())
	assert.Equal(t, 0, store.Count())
	assert.Equal(t, filepath.Join(projectDir, "dismissed.json"), store.FilePath())

	// Set and save a record
	require.NoError(t, store.Set(&DismissalRecord{
		RecommendationID: "test-rec",
		Status:           StatusDismissed,
		Reason:           "OTHER",
		DismissedAt:      time.Now(),
	}))
	require.NoError(t, store.Save())

	// Verify file was created in project dir
	_, err = os.Stat(filepath.Join(projectDir, "dismissed.json"))
	require.NoError(t, err, "dismissed.json should be created in project dir")
}

func TestDismissalStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "concurrent-dismissed.json")

	store, err := NewDismissalStore(storePath)
	require.NoError(t, err)

	const goroutines = 10
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				recID := fmt.Sprintf("rec-g%d-i%d", id, i)
				now := time.Now()

				// Concurrent Set
				err := store.Set(&DismissalRecord{
					RecommendationID: recID,
					Status:           StatusDismissed,
					Reason:           "BUSINESS_CONSTRAINT",
					DismissedAt:      now,
					History: []LifecycleEvent{
						{Action: ActionDismissed, Reason: "BUSINESS_CONSTRAINT", Timestamp: now},
					},
				})
				assert.NoError(t, err)

				// Concurrent Get
				_, _ = store.Get(recID)

				// Concurrent GetDismissedIDs
				_ = store.GetDismissedIDs()

				// Concurrent Count
				_ = store.Count()
			}
		}(g)
	}

	wg.Wait()

	// Verify final state is consistent
	finalCount := store.Count()
	assert.Equal(t, goroutines*iterations, finalCount,
		"all records should be present after concurrent writes")

	allRecords := store.GetAllRecords()
	assert.Len(t, allRecords, goroutines*iterations)
}
