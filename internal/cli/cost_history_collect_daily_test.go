package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

func TestCollectDailyPoints_IgnoresEvenSplit(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(72 * time.Hour)
	points, report := collectDailyPoints(&engine.CostResultWithErrors{
		Results: []engine.CostResult{{
			ResourceID: "urn:odd",
			Currency:   "USD",
			StartDate:  start,
			EndDate:    end,
			TotalCost:  100,
			DailyCosts: []float64{50, 50},
			DailyObservations: []engine.DailyObservation{{
				Timestamp: start, Cost: 4, Currency: "USD",
			}},
		}},
	}, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))

	require.Len(t, points, 1)
	assert.InDelta(t, 4, points[0].Amount, 0)
	assert.Equal(t, start, points[0].Day)
	assert.Empty(t, report.Errors)
	assert.Equal(t, 0, report.SkippedResources)
}

func TestCollectDaily_DefaultWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 31, 15, 4, 0, 0, time.UTC)
	var gotFrom, gotTo time.Time
	cmd, _, _ := dailyCmd(t)
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return now },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:a"}}, nil
		},
		price: func(
			_ context.Context, _ *cobra.Command, _ []engine.ResourceDescriptor, from, to time.Time,
		) (*engine.CostResultWithErrors, error) {
			gotFrom, gotTo = from, to
			return &engine.CostResultWithErrors{}, nil
		},
		cfg:     config.New(),
		dir:     t.TempDir(),
		project: "demo",
		stack:   "dev",
	})
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), gotFrom)
	assert.Equal(t, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), gotTo)
	assert.Equal(t, 30*24*time.Hour, gotTo.Sub(gotFrom))
}

func TestCollectDaily_StoresReplacesAndKeepsTimeline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedTimeline(t, dir)
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:bad",
		Day:        time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Currency:   "USD",
		Amount:     7,
		WrittenAt:  time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	})

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	written := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	calls := 0
	deps := dailyCollectDeps{
		now: func() time.Time { return written },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:ok"}}, nil
		},
		price:   scriptedDailyPrice(&calls, start, end),
		cfg:     config.New(),
		dir:     dir,
		project: "demo",
		stack:   "dev",
	}

	cmd, stdout, stderr := dailyCmd(t, "--from", "2026-10-01", "--to", "2026-10-03", "--output", "json")
	err := runDailyCollect(cmd, deps)
	require.ErrorContains(t, err, "1 resources failed")
	assert.Contains(t, stderr.String(), "urn:skip has no daily costs")
	assert.Contains(t, stderr.String(), "plugin down")

	var report dailyCollectReport
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	assert.Equal(t, 2, report.StoredResources)
	assert.Equal(t, 6, report.StoredPoints)
	assert.Equal(t, 1, report.SkippedResources)
	require.Len(t, report.Errors, 1)
	assert.Equal(t, "urn:bad", report.Errors[0].Resource)

	cmd, stdout, _ = dailyCmd(t, "--from", "2026-10-01", "--to", "2026-10-03")
	require.NoError(t, runDailyCollect(cmd, deps))
	assert.Contains(t, stdout.String(), "Stored 1 resources (1 points).")

	points := readDaily(t, dir)
	assert.InDelta(t, 1, dailyAmount(points, "urn:ok", start, "USD"), 0)
	assert.InDelta(t, 5, dailyAmount(points, "urn:ok", start.AddDate(0, 0, 1), "USD"), 0)
	assert.InDelta(t, 8, dailyAmount(points, "urn:ok", start, "EUR"), 0)
	require.True(t, dailyHas(points, "urn:zero", start, "USD"))
	assert.InDelta(t, 0, dailyAmount(points, "urn:zero", start, "USD"), 0)
	assert.InDelta(t, 7, dailyAmount(points, "urn:bad", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "USD"), 0)
	assert.False(t, dailyHas(points, "urn:skip", start, "USD"))

	snaps := readTimeline(t, dir)
	require.Len(t, snaps, 1)
	assert.InDelta(t, 12, snaps[0].TotalMonthly, 0)
	assert.Equal(t, 3, snaps[0].Version)
}

func TestCollectDaily_PrunesByAgeOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldDay := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:old", Day: oldDay, Currency: "USD", Amount: 3,
		WrittenAt: oldDay,
	})
	seedDaily(t, dir, history.DailyPoint{
		ResourceID: "urn:fail", Day: oldDay, Currency: "USD", Amount: 8,
		WrittenAt: oldDay,
	})
	cfg := config.New()
	cfg.Cost.History.Retention.AutoPrune = true
	cfg.Cost.History.Retention.MaxSnapshots = 1
	cfg.Cost.History.Retention.MaxAgeDays = 1
	now := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	kept := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	cmd, stdout, _ := dailyCmd(t, "--from", "2026-10-02", "--to", "2026-10-03")
	err := runDailyCollect(cmd, dailyCollectDeps{
		now: func() time.Time { return now },
		load: func(context.Context, *cobra.Command) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{{ID: "urn:new"}}, nil
		},
		price: func(
			_ context.Context, _ *cobra.Command, _ []engine.ResourceDescriptor, _, _ time.Time,
		) (*engine.CostResultWithErrors, error) {
			return &engine.CostResultWithErrors{
				Results: []engine.CostResult{{
					ResourceID: "urn:new", Currency: "USD",
					StartDate: kept, EndDate: kept.AddDate(0, 0, 1),
					DailyObservations: []engine.DailyObservation{{
						Timestamp: kept, Cost: 4, Currency: "USD",
					}},
				}},
				Errors: []engine.ErrorDetail{{
					ResourceID: "urn:fail", Error: errors.New("plugin down"),
				}},
			}, nil
		},
		cfg: cfg, dir: dir, project: "demo", stack: "dev",
	})
	require.ErrorContains(t, err, "1 resources failed")
	assert.Contains(t, stdout.String(), "Pruned 1 daily points")
	points := readDaily(t, dir)
	require.Len(t, points, 2)
	assert.InDelta(t, 4, dailyAmount(points, "urn:new", kept, "USD"), 0)
	assert.InDelta(t, 8, dailyAmount(points, "urn:fail", oldDay, "USD"), 0)
	assert.False(t, dailyHas(points, "urn:old", oldDay, "USD"))
}

func TestCollectDaily_RequiresFromWhenToIsSet(t *testing.T) {
	t.Parallel()

	cmd, _, _ := dailyCmd(t, "--to", "2026-10-03")
	err := runDailyCollect(cmd, dailyCollectDeps{
		now:     func() time.Time { return time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC) },
		dir:     t.TempDir(),
		project: "demo",
		stack:   "dev",
		cfg:     config.New(),
	})
	require.ErrorContains(t, err, "--from is required")
}

func scriptedDailyPrice(calls *int, start, end time.Time) dailyPricer {
	return func(
		_ context.Context, _ *cobra.Command, _ []engine.ResourceDescriptor, _, _ time.Time,
	) (*engine.CostResultWithErrors, error) {
		*calls++
		if *calls > 1 {
			day := start.AddDate(0, 0, 1)
			return &engine.CostResultWithErrors{Results: []engine.CostResult{{
				ResourceID: "urn:ok", Currency: "USD",
				StartDate: day, EndDate: end,
				DailyObservations: []engine.DailyObservation{{
					Timestamp: day, Cost: 5, Currency: "USD",
				}},
			}}}, nil
		}
		return &engine.CostResultWithErrors{
			Results: []engine.CostResult{
				{
					ResourceID: "urn:ok", Currency: "USD", StartDate: start, EndDate: end,
					TotalCost: 99,
					DailyObservations: []engine.DailyObservation{
						{Timestamp: start, Cost: 1, Currency: "USD"},
						{Timestamp: start.AddDate(0, 0, 1), Cost: 2, Currency: "USD"},
					},
				},
				{
					ResourceID: "urn:skip", Currency: "USD", StartDate: start, EndDate: end,
					TotalCost: 50, DailyCosts: []float64{25, 25},
				},
				{
					ResourceID: "urn:zero", Currency: "USD", StartDate: start, EndDate: end,
					DailyObservations: []engine.DailyObservation{
						{Timestamp: start, Cost: 0, Currency: "USD"},
						{Timestamp: start.AddDate(0, 0, 1), Cost: 4, Currency: "USD"},
					},
				},
				{
					ResourceID: "urn:ok", Currency: "EUR", StartDate: start, EndDate: end,
					DailyObservations: []engine.DailyObservation{
						{Timestamp: start, Cost: 8, Currency: "EUR"},
						{Timestamp: start.AddDate(0, 0, 1), Cost: 9, Currency: "EUR"},
					},
				},
			},
			Errors: []engine.ErrorDetail{{ResourceID: "urn:bad", Error: errors.New("plugin down")}},
		}, nil
	}
}

func dailyCmd(t *testing.T, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryCollectDailyCmd(), args...)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd, stdout, stderr
}

func seedTimeline(t *testing.T, dir string) {
	t.Helper()
	name, err := history.CostFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenCostDBFor(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	require.NoError(t, db.Put(history.CostSnapshot{
		Timestamp:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Version:      3,
		Currency:     "USD",
		TotalMonthly: 12,
	}, history.CostAnnotation{Version: 3}))
	require.NoError(t, db.Close())
}

func seedDaily(t *testing.T, dir string, point history.DailyPoint) {
	t.Helper()
	name, err := history.DailyFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenDailyDB(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	require.NoError(t, db.Replace([]history.DailyPoint{point}))
	require.NoError(t, db.Close())
}

func readDaily(t *testing.T, dir string) []history.DailyPoint {
	t.Helper()
	name, err := history.DailyFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenDailyDB(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	points, err := db.Points()
	require.NoError(t, err)
	return points
}

func readTimeline(t *testing.T, dir string) []history.CostSnapshot {
	t.Helper()
	name, err := history.CostFileNameFor("demo", "dev")
	require.NoError(t, err)
	db, err := history.OpenCostDBFor(filepath.Join(dir, name), "demo", "dev")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	return snaps
}

func dailyAmount(points []history.DailyPoint, id string, day time.Time, currency string) float64 {
	for _, point := range points {
		if point.ResourceID == id && point.Day.Equal(day) && point.Currency == currency {
			return point.Amount
		}
	}
	return 0
}

func dailyHas(points []history.DailyPoint, id string, day time.Time, currency string) bool {
	for _, point := range points {
		if point.ResourceID == id && point.Day.Equal(day) && point.Currency == currency {
			return true
		}
	}
	return false
}
