package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndOpen(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "dev.history.db")
	db := openCostDB(t, path)
	require.NoError(t, db.Close())
	again, err := OpenCostDB(path, "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = again.Close() })
	stats, err := again.Stats()
	require.NoError(t, err)
	assert.Equal(t, "dev", stats.Stack)
	assert.Equal(t, path, again.Path())
}

func TestStore_SchemaVersion(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Equal(t, costSchemaVersion, stats.Schema)
}

func TestStore_StoreSnapshot(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	when := date(2025, 6, 15)
	snap := ZeroSnapshot(when, 12)
	snap.TotalMonthly = 80
	snap.ByProvider["aws"] = 80
	require.NoError(t, db.Put(snap, AnnotationFrom(StackUpdate{Version: 12, Kind: "update", Message: "boot"})))
	got, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.InDelta(t, 80, got[0].TotalMonthly, 0.001)
	assert.Equal(t, 12, got[0].Version)
	assert.True(t, got[0].Timestamp.Equal(when))
}

func TestStore_StoreAnnotation(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	when := date(2025, 6, 15)
	require.NoError(t, db.Put(ZeroSnapshot(when, 12), CostAnnotation{
		Version: 12, Message: "boot", Kind: "update", ResourceChanges: map[string]int{"create": 1},
	}))
	got, err := db.Annotations(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "boot", got[0].Message)
	assert.Equal(t, 1, got[0].ResourceChanges["create"])
}

func TestStore_IncrementalVersion(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 1, 1), 1), AnnotationFrom(StackUpdate{Version: 1, Kind: "update"})),
	)
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 3, 1), 3), AnnotationFrom(StackUpdate{Version: 3, Kind: "update"})),
	)
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 2, 1), 2), AnnotationFrom(StackUpdate{Version: 2, Kind: "update"})),
	)
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), stats.LastVersion)
	assert.False(t, stats.CollectedAt.IsZero())
}

func TestStore_TimeRangeQuery(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 1, 15), 1), AnnotationFrom(StackUpdate{Version: 1, Kind: "update"})),
	)
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 6, 15), 2), AnnotationFrom(StackUpdate{Version: 2, Kind: "update"})),
	)
	sameSecond := date(2025, 6, 15)
	require.NoError(t, db.Put(ZeroSnapshot(sameSecond, 9), AnnotationFrom(StackUpdate{Version: 9, Kind: "update"})))
	got, err := db.Snapshots(date(2025, 6, 1), date(2025, 6, 30))
	require.NoError(t, err)
	require.Len(t, got, 2)
	anns, err := db.Annotations(date(2025, 6, 1), date(2025, 6, 30))
	require.NoError(t, err)
	assert.Len(t, anns, 2)
}

func TestStore_EmptyDatabase(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	got, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, got)
	anns, err := db.Annotations(time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, anns)
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Snapshots)
	assert.Positive(t, stats.Size)
}

func TestStore_VersionReset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db := openCostDB(t, filepath.Join(dir, "dev.history.db"))
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 1, 1), 50), AnnotationFrom(StackUpdate{Version: 50, Kind: "update"})),
	)
	require.NoError(t, db.Reset())
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), stats.LastVersion)
	assert.Equal(t, 0, stats.Snapshots)
	assert.Equal(t, "dev", stats.Stack)
	require.NoError(t, db.Close())

	listed, err := ListCostDBs(dir)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "dev", listed[0].Stack)
	_, err = OpenCostDB(filepath.Join(dir, "dev.history.db"), "other")
	require.ErrorContains(t, err, "does not match")

	bad := filepath.Join(t.TempDir(), "bad.history.db")
	require.NoError(t, os.WriteFile(bad, []byte("not a database"), 0o600))
	_, err = OpenCostDB(bad, "dev")
	require.Error(t, err)
	empty, err := ListCostDBs(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func openCostDB(t *testing.T, path string) *CostDB {
	t.Helper()
	db, err := OpenCostDB(path, "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
