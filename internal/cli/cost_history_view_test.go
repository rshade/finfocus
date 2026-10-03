package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

func TestView_RendersChart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev",
		viewSnap(12, time.January, 800, map[string]float64{"aws": 800}),
		viewSnap(23, time.March, 1200, map[string]float64{"aws": 800, "gcp": 400}),
	)
	cmd, stdout := preparedHistoryCmd(
		t,
		NewCostHistoryViewCmd(),
		"--stack",
		"dev",
		"--width",
		"40",
		"--height",
		"8",
		"--no-budget",
	)
	require.NoError(t, runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}}))
	out := stdout.String()
	assert.Contains(t, out, "Stack: dev")
	assert.Contains(t, out, "Deployment Annotations:")
	assert.Contains(t, out, "┤")
}

func TestView_DateRangeFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev",
		viewSnap(12, time.January, 800, map[string]float64{"aws": 800}),
		viewSnap(23, time.June, 1200, map[string]float64{"aws": 1200}),
	)
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryViewCmd(),
		"--stack", "dev", "--from", "2025-06-01", "--width", "40", "--no-budget")
	require.NoError(t, runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}}))
	out := stdout.String()
	assert.Contains(t, out, "1 snapshot")
	assert.Contains(t, out, "v23")
	assert.NotContains(t, out, "v12")
}

func TestView_ProviderFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev",
		viewSnap(1, time.January, 100, map[string]float64{"aws": 80, "gcp": 20}),
		viewSnap(2, time.February, 140, map[string]float64{"aws": 100, "gcp": 40}),
	)
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryViewCmd(),
		"--stack", "dev", "--provider", "aws", "--width", "40", "--no-budget", "--no-annotations")
	require.NoError(t, runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}}))
	out := stdout.String()
	assert.Contains(t, out, "AWS")
	assert.NotContains(t, out, "GCP")
}

func TestView_JSONOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev", viewSnap(1, time.January, 80, map[string]float64{"aws": 80}))
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryViewCmd(), "--stack", "dev", "--output", "json")
	require.NoError(t, runView(cmd, viewDeps{dir: dir, cfg: &config.Config{}}))
	body := stdout.String()
	assert.Contains(t, body, `"stack": "dev"`)
	assert.Contains(t, body, `"total_monthly"`)

	plain, plainOut := preparedHistoryCmd(
		t,
		NewCostHistoryViewCmd(),
		"--stack",
		"dev",
		"--output",
		"json",
		"--plain",
		"--width",
		"20",
	)
	require.NoError(t, runView(plain, viewDeps{dir: dir, cfg: &config.Config{}}))
	assert.NotContains(t, plainOut.String(), `"total_monthly"`)
	assert.Contains(t, plainOut.String(), "1 snapshot")

	bad, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd(), "--stack", "dev", "--output", "yaml")
	err := runView(bad, viewDeps{dir: t.TempDir(), cfg: &config.Config{}})
	require.ErrorContains(t, err, "unsupported output format")

	budgetDir := t.TempDir()
	seedHistory(t, budgetDir, "dev",
		viewSnap(1, time.January, 100, map[string]float64{"aws": 100}),
		viewSnap(2, time.February, 200, map[string]float64{"aws": 200}),
	)
	cfg := &config.Config{Cost: config.CostConfig{Budgets: &config.BudgetsConfig{
		Global: &config.ScopedBudget{Amount: 150},
	}}}
	chart, chartOut := preparedHistoryCmd(
		t,
		NewCostHistoryViewCmd(),
		"--stack",
		"dev",
		"--width",
		"30",
		"--height",
		"6",
	)
	require.NoError(t, runView(chart, viewDeps{dir: budgetDir, cfg: cfg}))
	assert.Contains(t, chartOut.String(), "Budget")
}

func seedHistory(t *testing.T, dir, stack string, snaps ...history.CostSnapshot) {
	t.Helper()
	path, err := historyPath(dir, stack)
	require.NoError(t, err)
	db, err := history.OpenCostDB(path, stack)
	require.NoError(t, err)
	for _, snap := range snaps {
		require.NoError(t, db.Put(snap, history.AnnotationFrom(history.StackUpdate{
			Version: snap.Version, Kind: "update", Message: "change",
		})))
	}
	require.NoError(t, db.Close())
}

func viewSnap(version int, month time.Month, total float64, by map[string]float64) history.CostSnapshot {
	return history.CostSnapshot{
		Timestamp:    time.Date(2025, month, 15, 0, 0, 0, 0, time.UTC),
		Version:      version,
		TotalMonthly: total,
		Currency:     "USD",
		ByProvider:   by,
		ByType:       map[string]float64{},
		Resources:    []history.CostResource{},
	}
}
