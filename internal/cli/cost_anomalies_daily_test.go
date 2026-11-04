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
func seedDailyStore(t *testing.T, dir, name string, points []history.DailyPoint) {
	t.Helper()
	db, err := history.OpenDailyDB(filepath.Join(dir, name), "myproj", "dev")
	require.NoError(t, err)
	require.NoError(t, db.Replace(points))
	require.NoError(t, db.Close())
}

// dailySeriesReader returns a series seam that reads the fixture store in dir
// for the stack the tests collected, the way the command's own reader does.
func dailySeriesReader(dir string) func(
	context.Context,
	string,
	string,
	engine.DateRange,
	time.Time,
) ([]history.DailyPoint, error) {
	return func(
		ctx context.Context,
		_ string,
		_ string,
		window engine.DateRange,
		now time.Time,
	) ([]history.DailyPoint, error) {
		return loadDailyAnomalySeries(ctx, dir, "myorg/myproj/dev", "", window, now, false)
	}
}

// dailyProjectReader watches the stack and project identity the command hands
// its series reader, then reads the fixture store as the production reader
// would with them.
func dailyProjectReader(
	dir string,
	seen *anomalySeriesIdentity,
) func(context.Context, string, string, engine.DateRange, time.Time) ([]history.DailyPoint, error) {
	return func(
		ctx context.Context,
		stack, project string,
		window engine.DateRange,
		now time.Time,
	) ([]history.DailyPoint, error) {
		seen.stack, seen.project = stack, project
		return loadDailyAnomalySeries(ctx, dir, stack, project, window, now, false)
	}
}

// anomalySeriesIdentity records what the command resolved before a series read.
type anomalySeriesIdentity struct {
	stack   string
	project string
}

func TestCostAnomalies_FixtureStoreShowsRobustMedianMethod(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, dailySpikeAmounts())

	rows := dailySpikeRow()

	deps := anomaliesTestDeps(fakePipelineLoader(rows))
	deps.dailySeries = dailySeriesReader(dir)

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
	deps.dailySeries = dailySeriesReader(dir)

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
	seedDailyStore(t, dir, name, dailySpikeAmounts())

	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = dailySeriesReader(dir)

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
	seedDailyStore(t, dir, name, dailySpikeAmounts())

	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = dailySeriesReader(dir)

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

// TestCostAnomalies_UsesTheResolvedStackWithoutStackFlag keeps the method
// usable without --stack: the stack the overview resolved names the stored
// series, so a bare `finfocus cost anomalies` still scores the day spikes.
func TestCostAnomalies_UsesTheResolvedStackWithoutStackFlag(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, dailySpikeAmounts())

	var seen anomalySeriesIdentity
	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = dailyProjectReader(dir, &seen)

	out, err := runAnomaliesTest(t, deps, "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "dev", seen.stack, "the stack the pipeline resolved names the series")
	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1)
	assert.Equal(t, "robust-median", decoded.Anomalies[0].Method)
}

// TestCostAnomalies_StackFlagWinsOverTheResolvedStack keeps the caller's
// choice authoritative: the flag names the series even when the pipeline
// resolved another stack.
func TestCostAnomalies_StackFlagWinsOverTheResolvedStack(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	var seen anomalySeriesIdentity
	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = func(
		_ context.Context,
		stack, project string,
		_ engine.DateRange,
		_ time.Time,
	) ([]history.DailyPoint, error) {
		seen.stack, seen.project = stack, project
		return nil, nil
	}

	cmd := newCostAnomaliesCmd(deps)
	cmd.PersistentFlags().String("stack", "", "")
	require.NoError(t, cmd.ParseFlags([]string{"--stack", "myorg/myproj/prod"}))
	require.NoError(t, cmd.RunE(cmd, nil))

	assert.Equal(t, "myorg/myproj/prod", seen.stack)
}

func TestAnomalySeriesStack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flag     string
		resolved string
		want     string
	}{
		{name: "the flag wins", flag: "myorg/myproj/dev", resolved: "other", want: "myorg/myproj/dev"},
		{name: "the resolved stack is the fallback", flag: "", resolved: "dev", want: "dev"},
		{name: "a blank flag falls back", flag: "   ", resolved: "dev", want: "dev"},
		{name: "no stack at all is no series", flag: "", resolved: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, anomalySeriesStack(tt.flag, tt.resolved))
		})
	}
}

// TestCostAnomalies_LoadedURNStackWinsOverTheResolvedName keeps the read on
// the stack that produced the rows. A saved state file resolves to a name taken
// from the file, not from the stack, so the series would be read from a store
// no collector wrote; the loaded URNs name the real stack.
func TestCostAnomalies_LoadedURNStackWinsOverTheResolvedName(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())

	var seen anomalySeriesIdentity
	deps := anomaliesTestDeps(func(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
		data, err := fakePipelineLoader(dailySpikeRow())(ctx, p)
		if err != nil {
			return nil, err
		}
		data.StackName = "state"
		return data, nil
	})
	deps.dailySeries = func(
		_ context.Context,
		stack, project string,
		_ engine.DateRange,
		_ time.Time,
	) ([]history.DailyPoint, error) {
		seen.stack, seen.project = stack, project
		return nil, nil
	}

	_, err := runAnomaliesTest(t, deps, "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "dev", seen.stack, "the loaded URNs name the stack, not the state file")
}

// TestCostAnomalies_LearnedRowsProjectReachesTheSeriesRead keeps the read on
// the store the collector wrote: the loaded rows' URNs name the Pulumi
// project, so without --stack the daily read still opens
// `<project>@<stack>.daily.db` the way "cost history collect-daily" names it,
// even when the working directory is not the project's and the resolved stack
// name alone would only ever find a legacy-named file.
func TestCostAnomalies_LearnedRowsProjectReachesTheSeriesRead(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, dailySpikeAmounts())

	var seen anomalySeriesIdentity
	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = dailyProjectReader(dir, &seen)

	out, err := runAnomaliesTest(t, deps, "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "dev", seen.stack, "the stack the pipeline resolved names the series")
	assert.Equal(t, "myproj", seen.project, "the loaded rows' URNs name the project")
	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1, "one row per resource")
	assert.Equal(t, "robust-median", decoded.Anomalies[0].Method)
}

// TestCostAnomalies_BareStackFlagResolvesTheStoredProject keeps --stack dev
// reading the same file the collector wrote: a bare stack value carries no
// project, so the project comes from the loaded rows' URNs instead of the
// working directory, and a legacy-name miss can never happen.
func TestCostAnomalies_BareStackFlagResolvesTheStoredProject(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	dir := t.TempDir()
	name, err := history.DailyFileNameFor("myproj", "dev")
	require.NoError(t, err)
	seedDailyStore(t, dir, name, dailySpikeAmounts())

	var seen anomalySeriesIdentity
	deps := anomaliesTestDeps(fakePipelineLoader(dailySpikeRow()))
	deps.dailySeries = dailyProjectReader(dir, &seen)

	out, err := runAnomaliesTest(t, deps, "--stack", "dev", "--output", "json")
	require.NoError(t, err)

	assert.Equal(t, "dev", seen.stack, "the asked-for stack names the series")
	assert.Equal(t, "myproj", seen.project, "the loaded rows' URNs name the project")
	var decoded engine.AnomalyReport
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Len(t, decoded.Anomalies, 1, "one row per resource")
	assert.Equal(t, "robust-median", decoded.Anomalies[0].Method)
}
