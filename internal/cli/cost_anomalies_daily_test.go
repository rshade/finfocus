package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

// dailySeriesURN names a resource the URN way, so the series written into the
// fixture store matches the OverviewRow URN the pipeline produces.
func dailySeriesURN() string {
	return "urn:pulumi:dev::myproj::aws:ec2/instance:Instance::spiky"
}

// dailySpikeAmounts builds the 80..145 baseline over fourteen days and a 250
// observation, written as one stored point per UTC day.
func dailySpikeAmounts() []history.DailyPoint {
	points := make([]history.DailyPoint, 0, 15)
	first := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for offset := 0; offset < 14; offset++ {
		points = append(points, history.DailyPoint{
			ResourceID: dailySeriesURN(),
			Day:        first.AddDate(0, 0, offset),
			Currency:   "USD",
			Amount:     80 + 5*float64(offset),
		})
	}
	points = append(points, history.DailyPoint{
		ResourceID: dailySeriesURN(),
		Day:        first.AddDate(0, 0, 14),
		Currency:   "USD",
		Amount:     250,
	})
	return points
}

// seedDailyStore writes points into `<name>` under dir through the real
// store, then closes it, so the reader exercises the file like collect left it.
func seedDailyStore(t *testing.T, dir, name, project, stack string, points []history.DailyPoint) {
	t.Helper()
	db, err := history.OpenDailyDB(filepath.Join(dir, name), project, stack)
	require.NoError(t, err)
	require.NoError(t, db.Replace(points))
	require.NoError(t, db.Close())
}

func TestCostAnomalies_FixtureStoreShowsRobustMedianMethod(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, "myproj", "dev", dailySpikeAmounts())

	rows := dailySpikeRow()

	deps := anomaliesTestDeps(fakePipelineLoader(rows))
	deps.dailySeries = func(ctx context.Context, window engine.DateRange, now time.Time) ([]history.DailyPoint, error) {
		return loadDailyAnomalySeries(ctx, dir, "myorg/myproj/dev", window, now, false)
	}

	out, err := runAnomaliesTest(t, deps,
		"--from", "2026-10-01", "--to", "2026-10-31", "--output", "json")
	require.NoError(t, err)

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1, "one row per resource")
	anomaly := decoded.Anomalies[0]
	assert.Equal(t, dailySeriesURN(), anomaly.Resource)
	assert.Equal(t, "robust-median", anomaly.Method)
	assert.InDelta(t, 112.5, anomaly.Baseline, 1e-9, "the median is the baseline")
	assert.InDelta(t, 250, anomaly.Observed, 1e-9, "the latest complete day is the observation")
	assert.Equal(t, "warning", anomaly.Severity)
	require.NotNil(t, anomaly.BaselineWindow)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), anomaly.BaselineWindow.Start)
	assert.Equal(t, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), anomaly.BaselineWindow.End)
}

func TestCostAnomalies_MissingStoreKeepsDriftRows(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	rows := anomalyTestRows()

	deps := anomaliesTestDeps(fakePipelineLoader(rows))
	deps.dailySeries = func(ctx context.Context, window engine.DateRange, now time.Time) ([]history.DailyPoint, error) {
		return loadDailyAnomalySeries(ctx, dir, "myorg/myproj/dev", window, now, false)
	}

	out, err := runAnomaliesTest(t, deps, "--output", "json")
	require.NoError(t, err)

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1)
	assert.Equal(t, "drift", decoded.Anomalies[0].Method)
}

// dailySpikeRow is the overview row of the spiking resource: it carries a
// warning-level drift, so a series that does not qualify leaves a drift row
// behind.
func dailySpikeRow() []engine.OverviewRow {
	return []engine.OverviewRow{
		{
			URN:    dailySeriesURN(),
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{
				MTDCost:  300,
				Currency: "USD",
				Period: engine.DateRange{
					Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
					End:   time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
				},
			},
			CostDrift: &engine.CostDriftData{
				ExtrapolatedMonthly: 400, Projected: 300, Delta: 100, PercentDrift: 33.3, IsWarning: true,
			},
		},
	}
}

// TestCostAnomalies_StoredHistoryIsNotTruncatedByTheDefaultWindow keeps the
// daily store a rolling history: without an explicit date range the whole
// stored series is scored, so the method does not go silent in the first
// half of every month, when the default cost window holds few days.
func TestCostAnomalies_StoredHistoryIsNotTruncatedByTheDefaultWindow(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, "myproj", "dev", dailySpikeAmounts())

	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = func(ctx context.Context, window engine.DateRange, now time.Time) ([]history.DailyPoint, error) {
		return loadDailyAnomalySeries(ctx, dir, "myorg/myproj/dev", window, now, false)
	}

	out, err := runAnomaliesTest(t, deps, "--output", "json")
	require.NoError(t, err)

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1, "one row per resource")
	assert.Equal(t, "robust-median", decoded.Anomalies[0].Method)
	assert.InDelta(t, 112.5, decoded.Anomalies[0].Baseline, 1e-9)
	assert.InDelta(t, 250, decoded.Anomalies[0].Observed, 1e-9)
}

// TestCostAnomalies_ExplicitWindowBoundsTheStoredSeries keeps the requested
// window honest: days outside it are not part of the baseline, so a narrow
// window leaves too little history and the drift row stands.
func TestCostAnomalies_ExplicitWindowBoundsTheStoredSeries(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, "myproj", "dev", dailySpikeAmounts())

	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = func(ctx context.Context, window engine.DateRange, now time.Time) ([]history.DailyPoint, error) {
		return loadDailyAnomalySeries(ctx, dir, "myorg/myproj/dev", window, now, false)
	}

	// October 8 through October 15 holds seven earlier days: too few for the
	// method, so the drift row of the same resource remains the result.
	out, err := runAnomaliesTest(t, deps,
		"--from", "2026-10-08", "--to", "2026-10-31", "--output", "json")
	require.NoError(t, err)

	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1)
	assert.Equal(t, "drift", decoded.Anomalies[0].Method)
}
