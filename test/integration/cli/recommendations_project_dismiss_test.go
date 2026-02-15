package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// TestCrossProjectDismissalIsolation verifies that dismissing a recommendation
// in one Pulumi project does not affect another project's dismissal state.
func TestCrossProjectDismissalIsolation(t *testing.T) {
	// Setup: two Pulumi projects with .finfocus directories
	projectA := filepath.Join(t.TempDir(), "project-a", ".finfocus")
	projectB := filepath.Join(t.TempDir(), "project-b", ".finfocus")
	require.NoError(t, os.MkdirAll(projectA, 0755))
	require.NoError(t, os.MkdirAll(projectB, 0755))

	// Save and restore original project dir
	origDir := config.GetResolvedProjectDir()
	t.Cleanup(func() { config.SetResolvedProjectDir(origDir) })

	recID := "rec-cross-project-001"
	now := time.Now()

	// Step 1: Dismiss recommendation in Project A
	config.SetResolvedProjectDir(projectA)
	storeA, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, storeA.Set(&config.DismissalRecord{
		RecommendationID: recID,
		Status:           config.StatusDismissed,
		Reason:           "BUSINESS_CONSTRAINT",
		DismissedAt:      now,
		History: []config.LifecycleEvent{
			{Action: config.ActionDismissed, Reason: "BUSINESS_CONSTRAINT", Timestamp: now},
		},
	}))
	require.NoError(t, storeA.Save())

	// Verify dismissal persists in Project A
	storeA2, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, storeA2.Load())
	record, ok := storeA2.Get(recID)
	require.True(t, ok, "dismissed rec should persist in Project A")
	assert.Equal(t, config.StatusDismissed, record.Status)

	// Step 2: Switch to Project B and verify recommendation is NOT dismissed
	config.SetResolvedProjectDir(projectB)
	storeB, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, storeB.Load())

	_, ok = storeB.Get(recID)
	assert.False(t, ok, "dismissed rec in Project A should NOT appear in Project B")
	assert.Empty(t, storeB.GetDismissedIDs(), "Project B should have no dismissals")

	// Step 3: Verify Project A still has the dismissal (switch back)
	config.SetResolvedProjectDir(projectA)
	storeA3, err := config.NewDismissalStore("")
	require.NoError(t, err)
	require.NoError(t, storeA3.Load())
	record, ok = storeA3.Get(recID)
	require.True(t, ok, "dismissal should still exist in Project A after checking Project B")
	assert.Equal(t, config.StatusDismissed, record.Status)

	// Step 4: Verify file paths are different
	assert.NotEqual(t, storeA.FilePath(), storeB.FilePath(),
		"stores for different projects should use different file paths")
	assert.Contains(t, storeA.FilePath(), "project-a")
	assert.Contains(t, storeB.FilePath(), "project-b")
}
