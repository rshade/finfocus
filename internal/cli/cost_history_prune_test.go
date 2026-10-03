package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestPrune_Keep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "1", "--force")
	require.NoError(t, runPrune(cmd, dir))
	out := stdout.String()
	assert.Contains(t, out, "Stack: dev")
	assert.Contains(t, out, "Pruning 2 snapshots beyond the 1 most recent...")
	assert.Contains(t, out, "Removed 2 snapshots and 2 annotations.")
	assert.Contains(t, out, "Compacted database:")
	assert.Contains(t, out, "Done.")
	assertPruneVersions(t, dir, 3)
}

func TestPrune_OlderThan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, stdout := preparedHistoryCmd(
		t, NewCostHistoryPruneCmd(), "--stack", "dev", "--older-than", "10d", "--force",
	)
	require.NoError(t, runPrune(cmd, dir))
	assert.Contains(t, stdout.String(), "older than ")
	assertPruneVersions(t, dir, 2, 3)
}

func TestPrune_KeepAndOlderThan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, _ := preparedHistoryCmd(
		t, NewCostHistoryPruneCmd(),
		"--stack", "dev", "--keep", "1", "--older-than", "10d", "--force",
	)
	require.NoError(t, runPrune(cmd, dir))
	assertPruneVersions(t, dir, 3)
}

func TestPrune_DryRunFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, stdout := preparedHistoryCmd(
		t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "1", "--dry-run", "--force",
	)
	require.NoError(t, runPrune(cmd, dir))
	out := stdout.String()
	assert.Contains(t, out, "Total snapshots: 3")
	assert.Contains(t, out, "Snapshots to prune: 2 (beyond the 1 most recent)")
	assert.Contains(t, out, "Snapshots to keep: 1")
	assert.Contains(t, out, "Estimated space freed:")
	assert.Contains(t, out, "Run without --dry-run to execute.")
	assert.NotContains(t, out, "Removed")
	assertPruneVersions(t, dir, 1, 2, 3)
}

func TestPrune_DryRunContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "1", "--force")
	cmd.SetContext(ax.WithDryRun(context.Background(), true))
	require.NoError(t, runPrune(cmd, dir))
	assert.Contains(t, stdout.String(), "Run without --dry-run to execute.")
	assertPruneVersions(t, dir, 1, 2, 3)
}

func TestPrune_Cancel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "1")
	cmd.SetIn(strings.NewReader("n\n"))
	err := runPrune(cmd, dir)
	require.ErrorContains(t, err, "prune cancelled")
	assert.NotContains(t, stdout.String(), "Removed")
	assertPruneVersions(t, dir, 1, 2, 3)
}

func TestPrune_YesSkipsPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "1", "--yes")
	require.NoError(t, runPrune(cmd, dir))
	assertPruneVersions(t, dir, 3)
}

func TestPrune_NothingToPrune(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)
	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "10", "--force")
	require.NoError(t, runPrune(cmd, dir))
	assert.Contains(t, stdout.String(), "Nothing to prune.")
	assert.NotContains(t, stdout.String(), "Compacted database:")
	assertPruneVersions(t, dir, 1, 2, 3)
}

func TestPrune_RejectsMissingPolicyAndBadFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedPruneHistory(t, dir)

	cmd, _ := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--force")
	err := runPrune(cmd, dir)
	require.ErrorContains(t, err, "set --keep or --older-than")

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--keep", "-1", "--force")
	err = runPrune(cmd, dir)
	require.ErrorContains(t, err, "--keep must be >= 0")

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "dev", "--older-than", "10w", "--force")
	err = runPrune(cmd, dir)
	require.ErrorContains(t, err, "invalid duration")

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--keep", "1", "--force")
	err = runPrune(cmd, dir)
	require.ErrorIs(t, err, ErrStackRequired)

	cmd, _ = preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--stack", "missing", "--keep", "1", "--force")
	err = runPrune(cmd, t.TempDir())
	require.ErrorContains(t, err, "no cost history for stack")
	assertPruneVersions(t, dir, 1, 2, 3)
}

func seedPruneHistory(t *testing.T, dir string) {
	t.Helper()
	now := time.Now().UTC()
	seedHistory(t, dir, "dev",
		pruneView(1, now.AddDate(0, 0, -40)),
		pruneView(2, now.AddDate(0, 0, -5)),
		pruneView(3, now.AddDate(0, 0, -1)),
	)
}

func pruneView(version int, at time.Time) history.CostSnapshot {
	snap := viewSnap(version, time.January, float64(version), map[string]float64{"aws": float64(version)})
	snap.Timestamp = at
	return snap
}

func openPruneDB(t *testing.T, dir string) *history.CostDB {
	t.Helper()
	path, err := historyPath(dir, "dev")
	require.NoError(t, err)
	db, err := history.OpenCostDBRead(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertPruneVersions(t *testing.T, dir string, versions ...int) {
	t.Helper()
	db := openPruneDB(t, dir)
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, len(versions))
	for i, version := range versions {
		assert.Equal(t, version, snaps[i].Version)
	}
	anns, err := db.Annotations(time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, anns, len(versions))
}
