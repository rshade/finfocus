package history_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func newTestEntry(urn, cloudID string) history.ResourceHistoryEntry {
	now := time.Now().Unix()
	return history.ResourceHistoryEntry{
		URN:       urn,
		CloudID:   cloudID,
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now,
		LastSeen:  now,
		Source:    history.SourceStateSnapshot,
		Tags:      map[string]string{},
	}
}

func TestNewBoltStore_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	require.NotNil(t, store)

	assert.True(t, store.IsEnabled())

	err = store.Close()
	require.NoError(t, err)
}

func TestNewBoltStore_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	require.NotNil(t, store)

	assert.False(t, store.IsEnabled())

	entry := newTestEntry("urn:pulumi:aws::resource", "i-12345")
	err = store.Upsert("testhash", entry)
	require.NoError(t, err)

	stackHash := "testhash"
	urnHash := history.URNHash(entry.URN)

	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, results, 0)

	err = store.Close()
	require.NoError(t, err)
}

func TestNewBoltStore_EmptyDirectory(t *testing.T) {
	ctx := context.Background()

	store, err := history.NewBoltStore(ctx, "", true, 90)
	assert.Error(t, err)
	assert.Nil(t, store)
}

func TestNewBoltStore_DirectoryAutoCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "db")
	ctx := context.Background()

	store, err := history.NewBoltStore(ctx, dbPath, true, 90)
	require.NoError(t, err)
	require.NotNil(t, store)

	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database directory should be auto-created")

	err = store.Close()
	require.NoError(t, err)
}

func TestBoltStore_Upsert_NewEntry(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	entry := newTestEntry("urn:pulumi:aws:ec2:instance:MyInstance", "i-12345")
	err = store.Upsert("testhash", entry)
	require.NoError(t, err)

	stackHash := "testhash"
	urnHash := history.URNHash(entry.URN)

	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, entry.URN, result.URN)
	assert.Equal(t, entry.CloudID, result.CloudID)
	assert.Equal(t, entry.Type, result.Type)
	assert.Equal(t, entry.Provider, result.Provider)
}

func TestBoltStore_Upsert_UpdateLastSeen(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entry1 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
		CloudID:   "i-12345",
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now,
		LastSeen:  now,
		Source:    history.SourceStateSnapshot,
		Tags:      map[string]string{},
	}

	err = store.Upsert("testhash", entry1)
	require.NoError(t, err)

	laterTime := now + 3600
	entry2 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
		CloudID:   "i-12345",
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: laterTime,
		LastSeen:  laterTime,
		Source:    history.SourceStateSnapshot,
		Tags:      map[string]string{},
	}

	err = store.Upsert("testhash", entry2)
	require.NoError(t, err)

	stackHash := "testhash"
	urnHash := history.URNHash(entry1.URN)

	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, laterTime+3600)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, now, result.FirstSeen, "FirstSeen should not be updated")
	assert.Equal(t, laterTime, result.LastSeen, "LastSeen should be updated")
}

func TestBoltStore_Upsert_DifferentCloudID(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entry1 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
		CloudID:   "i-11111",
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now,
		LastSeen:  now,
		Source:    history.SourceStateSnapshot,
		Tags:      map[string]string{},
	}

	entry2 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
		CloudID:   "i-22222",
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now,
		LastSeen:  now,
		Source:    history.SourceStateSnapshot,
		Tags:      map[string]string{},
	}

	err = store.Upsert("testhash", entry1)
	require.NoError(t, err)

	err = store.Upsert("testhash", entry2)
	require.NoError(t, err)

	stackHash := "testhash"
	urnHash := history.URNHash(entry1.URN)

	results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, now+3600)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	cloudIDs := make(map[string]bool)
	for _, r := range results {
		cloudIDs[r.CloudID] = true
	}
	assert.True(t, cloudIDs["i-11111"])
	assert.True(t, cloudIDs["i-22222"])
}

func TestBoltStore_UpsertBatch(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws:ec2:instance:Instance1",
			CloudID:   "i-10001",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:Instance2",
			CloudID:   "i-10002",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:s3:bucket:Bucket1",
			CloudID:   "bucket-abc",
			Type:      "aws:s3/bucket:Bucket",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourcePlanLineage,
		},
		{
			URN:       "urn:pulumi:aws:rds:instance:DB1",
			CloudID:   "db-12345",
			Type:      "aws:rds/instance:Instance",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceAnalyzerEvent,
		},
		{
			URN:       "urn:pulumi:gcp:compute:instance:GCPInstance",
			CloudID:   "instance-abc",
			Type:      "gcp:compute/instance:Instance",
			Provider:  "gcp",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	stackHash := "testhash"

	for _, entry := range entries {
		urnHash := history.URNHash(entry.URN)
		results, err := store.GetCloudIDsForURN(stackHash, urnHash, 0, now+3600)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, entry.CloudID, results[0].CloudID)
	}
}

func TestBoltStore_GetCloudIDsForURN_TimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	baseTime := now - 10000

	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
			CloudID:   "i-old",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: baseTime,
			LastSeen:  baseTime + 1000,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
			CloudID:   "i-recent",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 2000,
			LastSeen:  now - 1000,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:MyInstance",
			CloudID:   "i-current",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 500,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	stackHash := "testhash"
	urnHash := history.URNHash("urn:pulumi:aws:ec2:instance:MyInstance")

	results, err := store.GetCloudIDsForURN(stackHash, urnHash, now-3000, now+1000)
	require.NoError(t, err)

	cloudIDs := make([]string, len(results))
	for i, r := range results {
		cloudIDs[i] = r.CloudID
	}
	assert.Contains(t, cloudIDs, "i-recent")
	assert.Contains(t, cloudIDs, "i-current")
}

func TestBoltStore_GetAllForStack(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws:ec2:instance:Instance1",
			CloudID:   "i-10001",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:Instance2",
			CloudID:   "i-10002",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:s3:bucket:Bucket1",
			CloudID:   "bucket-xyz",
			Type:      "aws:s3/bucket:Bucket",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourcePlanLineage,
		},
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	stackHash := "testhash"

	results, err := store.GetAllForStack(stackHash, 0, now+3600)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	urns := make(map[string]bool)
	for _, r := range results {
		urns[r.URN] = true
	}
	assert.True(t, urns["urn:pulumi:aws:ec2:instance:Instance1"])
	assert.True(t, urns["urn:pulumi:aws:ec2:instance:Instance2"])
	assert.True(t, urns["urn:pulumi:aws:s3:bucket:Bucket1"])
}

func TestBoltStore_GetAllForStack_TimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	baseTime := now - 10000

	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws:ec2:instance:Old",
			CloudID:   "i-old",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: baseTime,
			LastSeen:  baseTime + 1000,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:Recent",
			CloudID:   "i-recent",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 2000,
			LastSeen:  now - 1000,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:Current",
			CloudID:   "i-current",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 500,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		},
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	stackHash := "testhash"

	results, err := store.GetAllForStack(stackHash, now-3000, now+1000)
	require.NoError(t, err)

	assert.True(t, len(results) >= 2)

	urns := make(map[string]bool)
	for _, r := range results {
		urns[r.URN] = true
	}
	assert.True(t, urns["urn:pulumi:aws:ec2:instance:Recent"])
	assert.True(t, urns["urn:pulumi:aws:ec2:instance:Current"])
}

func TestBoltStore_CorruptionRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)

	entry := newTestEntry("urn:pulumi:aws:ec2:instance:MyInstance", "i-12345")
	err = store.Upsert("testhash", entry)
	require.NoError(t, err)

	err = store.Close()
	require.NoError(t, err)

	dbFile := filepath.Join(tmpDir, "history.db")
	corruptedData := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	err = os.WriteFile(dbFile, corruptedData, 0o600)
	require.NoError(t, err)

	store2, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err, "should handle corruption gracefully")
	require.NotNil(t, store2)

	results, err := store2.GetAllForStack(history.StackContext{
		Organization: "test-org",
		Project:      "test-proj",
		Stack:        "test-stack",
	}.Hash(), 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Empty(t, results, "corrupted store should be recovered and empty")

	err = store2.Close()
	require.NoError(t, err)
}

func TestBoltStore_LockTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store1, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store1.Close()

	done := make(chan error, 1)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		store2, err := history.NewBoltStore(ctx2, tmpDir, true, 90)
		if err == nil {
			defer store2.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		assert.Error(t, err, "second store should fail to acquire lock")
	case <-time.After(10 * time.Second):
		t.Fatal("lock acquisition should timeout within 10 seconds")
	}
}

func TestBoltStore_SameCloudID_DifferentURNs(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	cloudID := "shared-resource-123"

	entry1 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:OldInstance",
		CloudID:   cloudID,
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now - 5000,
		LastSeen:  now - 1000,
		Source:    history.SourceStateSnapshot,
	}

	entry2 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:NewInstance",
		CloudID:   cloudID,
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now - 500,
		LastSeen:  now,
		Source:    history.SourceStateSnapshot,
	}

	err = store.Upsert("testhash", entry1)
	require.NoError(t, err)

	err = store.Upsert("testhash", entry2)
	require.NoError(t, err)

	stackHash := "testhash"

	urnHash1 := history.URNHash(entry1.URN)
	results1, err := store.GetCloudIDsForURN(stackHash, urnHash1, 0, now+3600)
	require.NoError(t, err)
	assert.Len(t, results1, 1)
	assert.Equal(t, entry1.URN, results1[0].URN)

	urnHash2 := history.URNHash(entry2.URN)
	results2, err := store.GetCloudIDsForURN(stackHash, urnHash2, 0, now+3600)
	require.NoError(t, err)
	assert.Len(t, results2, 1)
	assert.Equal(t, entry2.URN, results2[0].URN)
}

func TestBoltStore_GetDeletedResources_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	stackHash := "testhash"

	now := time.Now().Unix()
	results, err := store.GetDeletedResources(stackHash, map[string]bool{}, now-10000, now+3600)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestBoltStore_GetDeletedResources_WithCurrent(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	entry1 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:StillExists",
		CloudID:   "i-10001",
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now - 5000,
		LastSeen:  now - 100,
		Source:    history.SourceStateSnapshot,
	}

	entry2 := history.ResourceHistoryEntry{
		URN:       "urn:pulumi:aws:ec2:instance:WasDeleted",
		CloudID:   "i-10002",
		Type:      "aws:ec2/instance:Instance",
		Provider:  "aws",
		FirstSeen: now - 10000,
		LastSeen:  now - 4000,
		Source:    history.SourceStateSnapshot,
	}

	err = store.UpsertBatch("testhash", []history.ResourceHistoryEntry{entry1, entry2})
	require.NoError(t, err)

	stackHash := "testhash"

	urnHash1 := history.URNHash(entry1.URN)
	currentURNHashes := map[string]bool{
		urnHash1: true,
	}

	results, err := store.GetDeletedResources(stackHash, currentURNHashes, now-20000, now+1000)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, entry2.URN, results[0].URN)
}

func TestBoltStore_CleanupExpired(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 7)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	daysAgo90 := now - (90 * 24 * 3600)
	daysAgo3 := now - (3 * 24 * 3600)

	entries := []history.ResourceHistoryEntry{
		{
			URN:       "urn:pulumi:aws:ec2:instance:Old",
			CloudID:   "i-old",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: daysAgo90,
			LastSeen:  daysAgo90,
			Source:    history.SourceStateSnapshot,
		},
		{
			URN:       "urn:pulumi:aws:ec2:instance:Recent",
			CloudID:   "i-recent",
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: daysAgo3,
			LastSeen:  daysAgo3,
			Source:    history.SourceStateSnapshot,
		},
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	count, err := store.CleanupExpired(7)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	stackHash := "testhash"

	results, err := store.GetAllForStack(stackHash, 0, now+3600)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "i-recent", results[0].CloudID)
}

func TestBoltStore_Disabled_UpsertBatch_NoOp(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer store.Close()

	entries := []history.ResourceHistoryEntry{
		newTestEntry("urn:pulumi:aws:ec2:instance:Instance1", "i-10001"),
		newTestEntry("urn:pulumi:aws:ec2:instance:Instance2", "i-10002"),
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	stackHash := "testhash"

	results, err := store.GetAllForStack(stackHash, 0, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestBoltStore_Disabled_CleanupExpired_NoOp(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer store.Close()

	count, err := store.CleanupExpired(7)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBoltStore_Disabled_GetDeletedResources_NoOp(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, false, 90)
	require.NoError(t, err)
	defer store.Close()

	results, err := store.GetDeletedResources("test-hash", map[string]bool{}, 0, time.Now().Unix())
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// ---------------------------------------------------------------------------
// T042: Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBoltStore_UpsertBatch(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	entries := make([]history.ResourceHistoryEntry, 500)
	for i := range entries {
		entries[i] = history.ResourceHistoryEntry{
			URN:       fmt.Sprintf("urn:pulumi:dev::app::aws:ec2/instance:Instance::resource-%d", i),
			CloudID:   fmt.Sprintf("i-%05d", i),
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
			Tags:      map[string]string{"env": "prod", "team": "platform"},
		}
	}

	b.ResetTimer()
	for b.Loop() {
		if err := store.UpsertBatch("testhash", entries); err != nil {
			b.Fatalf("UpsertBatch failed: %v", err)
		}
	}
}

func BenchmarkBoltStore_GetAllForStack(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	entries := make([]history.ResourceHistoryEntry, 500)
	for i := range entries {
		entries[i] = history.ResourceHistoryEntry{
			URN:       fmt.Sprintf("urn:pulumi:dev::app::aws:ec2/instance:Instance::resource-%d", i),
			CloudID:   fmt.Sprintf("i-%05d", i),
			Type:      "aws:ec2/instance:Instance",
			Provider:  "aws",
			FirstSeen: now - 3600,
			LastSeen:  now,
			Source:    history.SourceStateSnapshot,
		}
	}
	if err := store.UpsertBatch("testhash", entries); err != nil {
		b.Fatalf("setup UpsertBatch failed: %v", err)
	}

	stackHash := "testhash"

	b.ResetTimer()
	for b.Loop() {
		if _, err := store.GetAllForStack(stackHash, now-7200, now+3600); err != nil {
			b.Fatalf("GetAllForStack failed: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// T026: GetDeletedResources tests
// ---------------------------------------------------------------------------

func TestBoltStore_GetDeletedResources_ReturnsDeletedOnly(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()

	// Create two resources: "web" and "db"
	webEntry := history.ResourceHistoryEntry{
		URN: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", CloudID: "i-web123",
		Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now - 3600, LastSeen: now, Source: history.SourceStateSnapshot,
		Tags: map[string]string{},
	}
	dbEntry := history.ResourceHistoryEntry{
		URN: "urn:pulumi:dev::app::aws:rds/instance:Instance::db", CloudID: "db-abc",
		Type: "aws:rds/instance:Instance", Provider: "aws",
		FirstSeen: now - 3600, LastSeen: now, Source: history.SourceStateSnapshot,
		Tags: map[string]string{},
	}
	require.NoError(t, store.UpsertBatch("testhash", []history.ResourceHistoryEntry{webEntry, dbEntry}))

	// Current state has "web" but NOT "db" → "db" is deleted
	currentURNHashes := map[string]bool{
		history.URNHash(webEntry.URN): true,
	}

	results, getErr := store.GetDeletedResources("testhash", currentURNHashes, now-7200, now+3600)
	require.NoError(t, getErr)
	require.Len(t, results, 1, "only the deleted resource should be returned")
	assert.Equal(t, "db-abc", results[0].CloudID)
}

func TestBoltStore_GetDeletedResources_ExcludesCurrentResources(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()

	entry := history.ResourceHistoryEntry{
		URN: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", CloudID: "i-web123",
		Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now - 3600, LastSeen: now, Source: history.SourceStateSnapshot,
		Tags: map[string]string{},
	}
	require.NoError(t, store.Upsert("testhash", entry))

	// Resource IS in current state → should be excluded
	currentURNHashes := map[string]bool{
		history.URNHash(entry.URN): true,
	}

	results, getErr := store.GetDeletedResources("testhash", currentURNHashes, now-7200, now+3600)
	require.NoError(t, getErr)
	assert.Empty(t, results, "current resources should be excluded")
}

func TestBoltStore_GetDeletedResources_TimeRangeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()

	// Resource was only active a long time ago (outside query range)
	oldEntry := history.ResourceHistoryEntry{
		URN: "urn:pulumi:dev::app::aws:ec2/instance:Instance::old", CloudID: "i-old",
		Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now - 86400*60, LastSeen: now - 86400*30, Source: history.SourceStateSnapshot,
		Tags: map[string]string{},
	}
	// Resource was recently active (within query range)
	recentEntry := history.ResourceHistoryEntry{
		URN: "urn:pulumi:dev::app::aws:ec2/instance:Instance::recent", CloudID: "i-recent",
		Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now - 3600, LastSeen: now, Source: history.SourceStateSnapshot,
		Tags: map[string]string{},
	}
	require.NoError(t, store.UpsertBatch("testhash", []history.ResourceHistoryEntry{oldEntry, recentEntry}))

	// Neither in current state → both are deleted
	currentURNHashes := map[string]bool{}

	// Query only the last 24 hours
	results, getErr := store.GetDeletedResources("testhash", currentURNHashes, now-86400, now+3600)
	require.NoError(t, getErr)
	require.Len(t, results, 1, "only the resource within time range should be returned")
	assert.Equal(t, "i-recent", results[0].CloudID)
}

func TestBoltStore_GetDeletedResources_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 90)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	results, getErr := store.GetDeletedResources("testhash", map[string]bool{}, now-3600, now+3600)
	require.NoError(t, getErr)
	assert.Empty(t, results, "empty store should return empty results")
}

// ---------------------------------------------------------------------------
// T044: Coverage improvement tests — Validate, hash helpers, tag cleanup
// ---------------------------------------------------------------------------

func TestResourceHistoryEntry_Validate(t *testing.T) {
	now := time.Now().Unix()
	validEntry := history.ResourceHistoryEntry{
		URN: "urn:pulumi:dev::app::aws:ec2/instance:Instance::web", CloudID: "i-123",
		Type: "aws:ec2/instance:Instance", Provider: "aws",
		FirstSeen: now, LastSeen: now, Source: history.SourceStateSnapshot,
	}

	tests := []struct {
		name    string
		entry   *history.ResourceHistoryEntry
		wantErr string
	}{
		{name: "valid entry", entry: &validEntry},
		{name: "nil entry", entry: nil, wantErr: "nil"},
		{name: "empty URN", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.URN = ""
			return &e
		}(), wantErr: "URN is required"},
		{name: "URN too long", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.URN = strings.Repeat("x", 1025)
			return &e
		}(), wantErr: "URN exceeds maximum"},
		{name: "empty CloudID", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.CloudID = ""
			return &e
		}(), wantErr: "cloudID is required"},
		{name: "CloudID too long", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.CloudID = strings.Repeat("x", 513)
			return &e
		}(), wantErr: "cloudID exceeds maximum"},
		{name: "empty Type", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Type = ""
			return &e
		}(), wantErr: "type is required"},
		{name: "Type too long", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Type = strings.Repeat("x", 257)
			return &e
		}(), wantErr: "type exceeds maximum"},
		{name: "empty Provider", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Provider = ""
			return &e
		}(), wantErr: "provider is required"},
		{name: "Provider too long", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Provider = strings.Repeat("x", 65)
			return &e
		}(), wantErr: "provider exceeds maximum"},
		{name: "invalid Source", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Source = "unknown"
			return &e
		}(), wantErr: "source must be one of"},
		{name: "negative FirstSeen", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.FirstSeen = 0
			return &e
		}(), wantErr: "firstSeen must be a positive"},
		{name: "LastSeen before FirstSeen", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.LastSeen = now - 3600
			e.FirstSeen = now
			return &e
		}(), wantErr: "lastSeen"},
		{name: "too many tags", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Tags = make(map[string]string, 51)
			for i := range 51 {
				e.Tags[fmt.Sprintf("key%d", i)] = "val"
			}
			return &e
		}(), wantErr: "tags exceeds maximum"},
		{name: "tag key too long", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Tags = map[string]string{strings.Repeat("k", 129): "val"}
			return &e
		}(), wantErr: "tag key exceeds maximum"},
		{name: "tag value too long", entry: func() *history.ResourceHistoryEntry {
			e := validEntry
			e.Tags = map[string]string{"key": strings.Repeat("v", 257)}
			return &e
		}(), wantErr: "tag value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBuildHistoryKey(t *testing.T) {
	key := history.BuildHistoryKey("stackabc", "urnxyz", "i-123")
	assert.Equal(t, "stackabc/urnxyz/i-123", key)
}

func TestBuildTagKey(t *testing.T) {
	key := history.BuildTagKey("stackabc", "env", "prod", "urnxyz")
	assert.Equal(t, "stackabc/env:prod/urnxyz", key)
}

func TestBoltStore_CleanupExpired_WithTags(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	store, err := history.NewBoltStore(ctx, tmpDir, true, 365)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	daysAgo90 := now - (90 * 24 * 3600)

	entries := []history.ResourceHistoryEntry{
		{
			URN: "urn:pulumi:aws:ec2:instance:Expired", CloudID: "i-expired",
			Type: "aws:ec2/instance:Instance", Provider: "aws",
			FirstSeen: daysAgo90, LastSeen: daysAgo90, Source: history.SourceStateSnapshot,
			Tags: map[string]string{"env": "old", "team": "platform"},
		},
		{
			URN: "urn:pulumi:aws:ec2:instance:Active", CloudID: "i-active",
			Type: "aws:ec2/instance:Instance", Provider: "aws",
			FirstSeen: now - 3600, LastSeen: now, Source: history.SourceStateSnapshot,
			Tags: map[string]string{"env": "prod"},
		},
	}

	err = store.UpsertBatch("testhash", entries)
	require.NoError(t, err)

	count, cleanErr := store.CleanupExpired(7)
	require.NoError(t, cleanErr)
	assert.Equal(t, 1, count)

	stackHash := "testhash"

	remaining, getErr := store.GetAllForStack(stackHash, 0, now+3600)
	require.NoError(t, getErr)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "i-active", remaining[0].CloudID)
}
