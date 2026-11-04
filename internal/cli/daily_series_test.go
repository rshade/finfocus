package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

func TestDailySeriesTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		stack          string
		explicitPulumi bool
		wantOk         bool
		wantPath       string
		wantOwner      historyTarget
		wantErr        bool
	}{
		{
			name:     "a bare stack names the legacy daily file",
			stack:    "prod",
			wantOk:   true,
			wantPath: "prod.daily.db",
			wantOwner: historyTarget{
				Project: "",
				Stack:   "prod",
			},
		},
		{
			name:     "a qualified stack names the project's daily file",
			stack:    "myorg/myproj/dev",
			wantOk:   true,
			wantPath: "myproj@dev.daily.db",
			wantOwner: historyTarget{
				Project: "myproj",
				Stack:   "dev",
			},
		},
		{
			name:   "no stack at all is no series",
			stack:  "  ",
			wantOk: false,
		},
		{
			name:    "a malformed stack name is an error",
			stack:   "with space",
			wantErr: true,
		},
		{
			name:           "an explicit Pulumi input names the stack the caller asked for",
			stack:          "myorg/myproj/dev",
			explicitPulumi: true,
			wantOk:         true,
			wantPath:       "myproj@dev.daily.db",
			wantOwner: historyTarget{
				Project: "myproj",
				Stack:   "dev",
			},
		},
		{
			name:           "an explicit Pulumi input with no stack is no series",
			stack:          "  ",
			explicitPulumi: true,
			wantOk:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target, ok, err := dailySeriesTarget(t.Context(), "testdata/daily-series-none", tt.stack, tt.explicitPulumi)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOk, ok)
			if !tt.wantOk {
				assert.Empty(t, target.Path)
				return
			}
			assert.Equal(t, tt.wantPath, filepath.Base(target.Path))
			assert.Equal(t, tt.wantOwner.Project, target.Project)
			assert.Equal(t, tt.wantOwner.Stack, target.Stack)
			assert.Empty(t, target.Label)
		})
	}
}

// onePoint is a shorthand stored day.
func onePoint(offset int, amount float64) history.DailyPoint {
	return history.DailyPoint{
		ResourceID: "urn:one",
		Day:        time.Date(2026, 10, 1+offset, 0, 0, 0, 0, time.UTC),
		Currency:   "USD",
		Amount:     amount,
	}
}

func TestReadDailySeries_MissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	target := historyTarget{Path: filepath.Join(t.TempDir(), "absent.daily.db")}
	points, err := readDailySeries(target)
	require.NoError(t, err)
	assert.Nil(t, points, "a stack that has never been collected is no series")
}

func TestReadDailySeries_ReturnsStoredPoints(t *testing.T) {
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, "myproj", "dev", []history.DailyPoint{onePoint(0, 90)})

	target := historyTarget{
		Path:    filepath.Join(dir, name),
		Project: "myproj",
		Stack:   "dev",
	}
	points, err := readDailySeries(target)
	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.Equal(t, "urn:one", points[0].ResourceID)
	assert.InDelta(t, 90, points[0].Amount, 1e-9)
}

func TestTrimDailySeries(t *testing.T) {
	t.Parallel()

	points := []history.DailyPoint{
		onePoint(0, 80),  // Oct 1, inside
		onePoint(1, 85),  // Oct 2, the observation
		onePoint(2, 90),  // today: partial
		onePoint(3, 95),  // tomorrow: partial
		onePoint(-1, 75), // before the window
	}
	from := engine.DateRange{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC),
	}
	// now is anchored to UTC so the day cut is identical on every machine: a
	// local-timestamped fixture would flip the partial-day boundary on a
	// UTC+13 box even though the reader only ever reads UTC days.
	now := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)

	t.Run("the window and the partial day are both applied", func(t *testing.T) {
		t.Parallel()
		kept := trimDailySeries(points, from, now)
		require.Len(t, kept, 2)
		assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), kept[0].Day)
		assert.Equal(t, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), kept[1].Day)
	})

	t.Run("a zero window filters nothing but the partial day", func(t *testing.T) {
		t.Parallel()
		kept := trimDailySeries(points, engine.DateRange{}, now)
		require.Len(t, kept, 3, "the local timezone day never touches the UTC day cut")
		assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), kept[0].Day)
		assert.Equal(t, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), kept[1].Day)
		assert.Equal(t, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), kept[2].Day)
	})

	t.Run("no point is zero-filled for a missing day", func(t *testing.T) {
		t.Parallel()
		kept := trimDailySeries([]history.DailyPoint{onePoint(0, 80), onePoint(2, 90)}, engine.DateRange{}, now)
		require.Len(t, kept, 1, "day one is absent and stays absent, not zero at midnight")
		assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), kept[0].Day)
	})
}

func TestLoadDailyAnomalySeries_TrimmedEndToEnd(t *testing.T) {
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, "myproj", "dev", []history.DailyPoint{
		onePoint(0, 80),
		onePoint(2, 90), // today: dropped
	})

	now := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	points, err := loadDailyAnomalySeries(t.Context(), dir, "myorg/myproj/dev", engine.DateRange{}, now, false)
	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), points[0].Day)
}
