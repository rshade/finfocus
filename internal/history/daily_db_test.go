package history

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestDailyDB_ReplaceLeavesDaysOutsideTheWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "demo@dev.daily.db")
	db := openDaily(t, path)
	written := time.Date(2026, 10, 31, 1, 0, 0, 0, time.UTC)
	day1 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Replace([]DailyPoint{
		{ResourceID: "urn:a", Day: day1, Currency: "USD", Amount: 1, WrittenAt: written},
		{ResourceID: "urn:a", Day: day2, Currency: "USD", Amount: 2, WrittenAt: written},
		{ResourceID: "urn:a", Day: day1, Currency: "EUR", Amount: 9, WrittenAt: written},
	}))
	require.NoError(t, db.Close())

	db = openDaily(t, path)
	require.NoError(t, db.Replace([]DailyPoint{
		{ResourceID: "urn:a", Day: day2, Currency: "USD", Amount: 5, WrittenAt: written.Add(time.Hour)},
	}))
	points, err := db.Points()
	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.InDelta(t, 1, amountOn(points, day1, "USD"), 0)
	assert.InDelta(t, 5, amountOn(points, day2, "USD"), 0)
	assert.InDelta(t, 9, amountOn(points, day1, "EUR"), 0)

	pruned, err := db.PruneBefore(day2, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, pruned)
	left, err := db.Points()
	require.NoError(t, err)
	require.Len(t, left, 1)
	assert.Equal(t, day2, left[0].Day)
	require.NoError(t, db.Close())
}

func TestDailyDB_RefusesTheOtherStore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	timelinePath := filepath.Join(dir, "demo@dev.history.db")
	timeline, err := OpenCostDBFor(timelinePath, "demo", "dev")
	require.NoError(t, err)
	require.NoError(t, timeline.Put(CostSnapshot{
		Timestamp: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Version:   1,
		Currency:  "USD",
	}, CostAnnotation{Version: 1}))
	require.NoError(t, timeline.Close())

	_, err = OpenDailyDB(timelinePath, "demo", "dev")
	require.ErrorIs(t, err, ErrTimelineAsDaily)

	reopened, err := OpenCostDBFor(timelinePath, "demo", "dev")
	require.NoError(t, err)
	snaps, err := reopened.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	require.NoError(t, reopened.Close())

	dailyPath := filepath.Join(dir, "demo@dev.daily.db")
	daily := openDaily(t, dailyPath)
	require.NoError(t, daily.Replace([]DailyPoint{{
		ResourceID: "urn:a",
		Day:        time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Currency:   "USD",
		Amount:     4,
		WrittenAt:  time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
	}}))
	require.NoError(t, daily.Close())

	_, err = OpenCostDBFor(dailyPath, "demo", "dev")
	require.ErrorIs(t, err, ErrDailyFileAsTimeline)

	daily = openDaily(t, dailyPath)
	points, err := daily.Points()
	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.InDelta(t, 4, points[0].Amount, 0)
	require.NoError(t, daily.Close())
}

func TestDailyDB_RejectsSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs elevated privileges on Windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "outside.db")
	link := filepath.Join(dir, "demo@dev.daily.db")
	require.NoError(t, os.WriteFile(target, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(target, link))

	_, err := OpenDailyDB(link, "demo", "dev")
	require.ErrorContains(t, err, "not a regular file")
	body, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(body))
}

func TestCostDB_UnsupportedSchemaStillReported(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "demo@dev.history.db")
	db, err := bolt.Open(path, dbFilePermissions, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bolt.Tx) error {
		meta, createErr := tx.CreateBucket([]byte(costBucketMeta))
		require.NoError(t, createErr)
		return putUint64(meta, metaKeyVersion, 2)
	}))
	require.NoError(t, db.Close())

	_, err = OpenCostDBFor(path, "demo", "dev")
	require.ErrorContains(t, err, "unsupported cost history schema version 2")
}

func openDaily(t *testing.T, path string) *DailyDB {
	t.Helper()
	db, err := OpenDailyDB(path, "demo", "dev")
	require.NoError(t, err)
	return db
}

func amountOn(points []DailyPoint, day time.Time, currency string) float64 {
	for _, point := range points {
		if point.Day.Equal(day) && point.Currency == currency {
			return point.Amount
		}
	}
	return -1
}
