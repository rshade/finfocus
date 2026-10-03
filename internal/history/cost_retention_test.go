package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOlderThan(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)

	got, err := ParseOlderThan("365d", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2025, time.June, 15, 12, 0, 0, 0, time.UTC), got)

	got, err = ParseOlderThan("6m", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2025, time.December, 15, 12, 0, 0, 0, time.UTC), got)

	got, err = ParseOlderThan("2y", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, time.June, 15, 12, 0, 0, 0, time.UTC), got)

	empty, err := ParseOlderThan("  ", now)
	require.NoError(t, err)
	assert.True(t, empty.IsZero())

	_, err = ParseOlderThan("10w", now)
	require.ErrorContains(t, err, "invalid duration")
	_, err = ParseOlderThan("365", now)
	require.ErrorContains(t, err, "invalid duration")
}

func TestApplyPrune_KeepAndAge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	path := t.TempDir() + "/dev.history.db"
	db, err := OpenCostDB(path, "dev")
	require.NoError(t, err)
	require.NoError(t, db.Put(pruneSnap(1, now.AddDate(0, 0, -40)), CostAnnotation{Version: 1, Message: "old"}))
	require.NoError(t, db.Put(pruneSnap(2, now.AddDate(0, 0, -20)), CostAnnotation{Version: 2, Message: "mid"}))
	require.NoError(t, db.Put(pruneSnap(3, now.AddDate(0, 0, -2)), CostAnnotation{Version: 3, Message: "new"}))

	planned, err := db.ApplyPrune(CostPrunePolicy{Keep: 1}, true)
	require.NoError(t, err)
	assert.Equal(t, 3, planned.Total)
	assert.Equal(t, 2, planned.Pruned)
	assert.Equal(t, 1, planned.Kept)
	still, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, still, 3)

	cutoff, err := ParseOlderThan("10d", now)
	require.NoError(t, err)
	aged, err := db.ApplyPrune(CostPrunePolicy{Cutoff: cutoff}, false)
	require.NoError(t, err)
	assert.Equal(t, 2, aged.Pruned)
	assert.Equal(t, 1, aged.Kept)
	kept, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, kept, 1)
	assert.Equal(t, 3, kept[0].Version)
	anns, err := db.Annotations(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, anns, 1)
	assert.Equal(t, 3, anns[0].Version)

	require.NoError(t, db.Close())
	before, after, err := CompactCostFile(path)
	require.NoError(t, err)
	assert.Positive(t, before)
	assert.Positive(t, after)
	reopened, err := OpenCostDBRead(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	finalSnaps, err := reopened.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, finalSnaps, 1)
	assert.Equal(t, 3, finalSnaps[0].Version)
	stats, err := reopened.Stats()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), stats.LastVersion)
}

func TestApplyPrune_StricterOfKeepAndAge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	path := t.TempDir() + "/dev.history.db"
	db, err := OpenCostDB(path, "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Put(pruneSnap(1, now.AddDate(0, 0, -40)), CostAnnotation{Version: 1}))
	require.NoError(t, db.Put(pruneSnap(2, now.AddDate(0, 0, -5)), CostAnnotation{Version: 2}))
	require.NoError(t, db.Put(pruneSnap(3, now.AddDate(0, 0, -1)), CostAnnotation{Version: 3}))

	cutoff, err := ParseOlderThan("10d", now)
	require.NoError(t, err)
	result, err := db.ApplyPrune(CostPrunePolicy{Keep: 1, Cutoff: cutoff}, false)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Pruned)
	assert.Equal(t, 1, result.Kept)
	kept, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, kept, 1)
	assert.Equal(t, 3, kept[0].Version)
}

func pruneSnap(version int, at time.Time) CostSnapshot {
	return CostSnapshot{
		Timestamp:    at,
		Version:      version,
		TotalMonthly: float64(version),
		Currency:     "USD",
		ByProvider:   map[string]float64{"aws": float64(version)},
		ByType:       map[string]float64{},
		Resources:    []CostResource{},
	}
}
