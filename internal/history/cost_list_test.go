package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCostDBsLenient_SkipsAnUnreadableDatabase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := openCostDB(t, filepath.Join(dir, "good.history.db"))
	require.NoError(t, good.Put(
		ZeroSnapshot(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1), CostAnnotation{Version: 1}))
	require.NoError(t, good.Close())
	bad := filepath.Join(dir, "bad.history.db")
	require.NoError(t, os.WriteFile(bad, []byte("this is not a bolt database"), 0o600))

	stats, issues, err := ListCostDBsLenient(dir)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "dev", stats[0].Stack)
	require.Len(t, issues, 1)
	assert.Equal(t, bad, issues[0].Path)
	require.Error(t, issues[0].Err)

	_, strictErr := ListCostDBs(dir)
	assert.Error(t, strictErr, "the strict listing still fails on a bad database")
}

func TestListCostDBsLenient_MissingDirectoryIsEmpty(t *testing.T) {
	t.Parallel()

	stats, issues, err := ListCostDBsLenient(filepath.Join(t.TempDir(), "missing"))

	require.NoError(t, err)
	assert.Empty(t, stats)
	assert.Empty(t, issues)
}

func TestStats_CountsAndBoundsWithoutReadingEverySnapshot(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	first := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	for i := range 40 {
		snapshot := ZeroSnapshot(first.AddDate(0, i, 0), i+1)
		require.NoError(t, db.Put(snapshot, CostAnnotation{Version: i + 1}))
	}

	stats, err := db.Stats()

	require.NoError(t, err)
	assert.Equal(t, 40, stats.Snapshots)
	assert.True(t, stats.First.Equal(first))
	assert.True(t, stats.Last.Equal(first.AddDate(0, 39, 0)))
	assert.Equal(t, uint64(40), stats.LastVersion)
}

func TestStats_EmptyDatabase(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))

	stats, err := db.Stats()

	require.NoError(t, err)
	assert.Zero(t, stats.Snapshots)
	assert.True(t, stats.First.IsZero())
	assert.True(t, stats.Last.IsZero())
}
