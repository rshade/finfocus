package history_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func newTestEntryWithTime(urn, cloudID string, lastSeen int64) history.ResourceHistoryEntry {
	now := time.Now().Unix()
	return history.ResourceHistoryEntry{
		URN:       urn,
		CloudID:   cloudID,
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now - 86400,
		LastSeen:  lastSeen,
		Source:    history.SourceStateSnapshot,
		Tags:      make(map[string]string),
	}
}

// TestCleanupExpired_RemovesOldEntries verifies that entries older than the
// retention window are deleted from the store.
func TestCleanupExpired_RemovesOldEntries(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := history.NewBoltStore(ctx, tempDir, true, 365)
	require.NoError(t, err)
	defer store.Close()

	oldTimestamp := time.Now().Add(-120 * 24 * time.Hour).Unix()
	entry := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:test", "i-old123", oldTimestamp)

	err = store.Upsert(entry)
	require.NoError(t, err)

	stackCtx := history.StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "dev",
	}

	allBefore, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allBefore, 1, "entry should exist before cleanup")

	count, err := store.CleanupExpired(90)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "cleanup should remove 1 entry")

	allAfter, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allAfter, 0, "entry should be removed after cleanup")
}

// TestCleanupExpired_KeepsRecentEntries verifies that entries within the
// retention window are not deleted.
func TestCleanupExpired_KeepsRecentEntries(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := history.NewBoltStore(ctx, tempDir, true, 365)
	require.NoError(t, err)
	defer store.Close()

	recentTimestamp := time.Now().Unix()
	entry := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:test", "i-recent123", recentTimestamp)

	err = store.Upsert(entry)
	require.NoError(t, err)

	stackCtx := history.StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "dev",
	}

	allBefore, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allBefore, 1, "entry should exist before cleanup")

	count, err := store.CleanupExpired(90)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "cleanup should not remove any entries")

	allAfter, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allAfter, 1, "entry should remain after cleanup")
}

// TestCleanupExpired_BoundaryExact verifies that entries at the exact boundary
// (90 days ago) are kept, not removed.
func TestCleanupExpired_BoundaryExact(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := history.NewBoltStore(ctx, tempDir, true, 365)
	require.NoError(t, err)
	defer store.Close()

	boundaryTimestamp := time.Now().Add(-90 * 24 * time.Hour).Unix()
	entry := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:test", "i-boundary123", boundaryTimestamp)

	err = store.Upsert(entry)
	require.NoError(t, err)

	stackCtx := history.StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "dev",
	}

	allBefore, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allBefore, 1, "entry should exist before cleanup")

	count, err := store.CleanupExpired(90)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "cleanup should not remove entry at boundary")

	allAfter, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allAfter, 1, "entry at boundary should be kept")
}

// TestCleanupExpired_ReturnsCorrectCount verifies that cleanup returns the
// correct count of deleted entries and leaves recent ones intact.
func TestCleanupExpired_ReturnsCorrectCount(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := history.NewBoltStore(ctx, tempDir, true, 365)
	require.NoError(t, err)
	defer store.Close()

	oldTimestamp := time.Now().Add(-120 * 24 * time.Hour).Unix()
	recentTimestamp := time.Now().Unix()

	oldEntry1 := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:old1", "i-old1", oldTimestamp)
	oldEntry2 := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:old2", "i-old2", oldTimestamp)
	oldEntry3 := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:old3", "i-old3", oldTimestamp)
	recentEntry1 := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:recent1", "i-recent1", recentTimestamp)
	recentEntry2 := newTestEntryWithTime("urn:pulumi:aws:ec2:instance:recent2", "i-recent2", recentTimestamp)

	err = store.Upsert(oldEntry1)
	require.NoError(t, err)
	err = store.Upsert(oldEntry2)
	require.NoError(t, err)
	err = store.Upsert(oldEntry3)
	require.NoError(t, err)
	err = store.Upsert(recentEntry1)
	require.NoError(t, err)
	err = store.Upsert(recentEntry2)
	require.NoError(t, err)

	stackCtx := history.StackContext{
		Organization: "org",
		Project:      "proj",
		Stack:        "dev",
	}

	allBefore, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allBefore, 5, "all 5 entries should exist before cleanup")

	count, err := store.CleanupExpired(90)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "cleanup should remove 3 old entries")

	allAfter, err := store.GetAllForStack(stackCtx.Hash(), 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, allAfter, 2, "only 2 recent entries should remain")
}

// TestCleanupExpired_EmptyStore verifies that cleanup on an empty store
// returns 0 with no error.
func TestCleanupExpired_EmptyStore(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	store, err := history.NewBoltStore(ctx, tempDir, true, 365)
	require.NoError(t, err)
	defer store.Close()

	count, err := store.CleanupExpired(90)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "cleanup on empty store should return 0")
}

// TestCleanupExpired_DisabledStore verifies that cleanup on a disabled store
// is a no-op and returns 0 with no error.
func TestCleanupExpired_DisabledStore(t *testing.T) {
	ctx := context.Background()

	store, err := history.NewBoltStore(ctx, "", false, 90)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	count, err := store.CleanupExpired(90)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "cleanup on disabled store should be no-op")
}
