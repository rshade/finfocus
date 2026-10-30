package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestHistoryViewBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		from, to    string
		errContains string
	}{
		{name: "open"},
		{name: "dates", from: "2025-01-01", to: "2025-02-01"},
		{name: "rfc3339", from: "2025-01-01T10:00:00Z"},
		{name: "bad from", from: "January", errContains: `invalid date "January"`},
		{name: "bad to", to: "02/01/2025", errContains: `invalid date "02/01/2025"`},
		{name: "reversed", from: "2025-02-01", to: "2025-01-01", errContains: "--to is before --from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			from, to, err := historyViewBounds(tt.from, tt.to)
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				assert.True(t, history.IsQueryError(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.from == "", from.IsZero())
			assert.Equal(t, tt.to == "", to.IsZero())
		})
	}

	_, to, err := historyViewBounds("", "2025-01-31")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2025, 1, 31, 23, 59, 59, 999999999, time.UTC), to)
}

func TestReadHistoryView(t *testing.T) {
	t.Parallel()

	usd1 := viewSnap(1, time.January, 100, map[string]float64{"aws": 100})
	usd2 := viewSnap(2, time.February, 200, map[string]float64{"aws": 200})
	eur := viewSnap(3, time.March, 50, map[string]float64{"aws": 50})
	eur.Currency = "EUR"
	dir := t.TempDir()
	seedHistory(t, dir, "dev", usd1, usd2, eur)
	db := openSeeded(t, dir, "dev")

	doc, err := readHistoryView(db, "dev", time.Time{}, time.Time{}, history.CurrencyChoice{})
	require.NoError(t, err)
	assert.Equal(t, "dev", doc.Stack)
	assert.Equal(t, []int{1, 2}, snapshotVersions(doc.Snapshots))
	require.Len(t, doc.Annotations, 2)
	assert.Contains(t, doc.CurrencyWarning, "Showing USD snapshots only.")

	eurDoc, err := readHistoryView(db, "dev", time.Time{}, time.Time{}, history.CurrencyChoice{Currency: "eur"})
	require.NoError(t, err)
	assert.Equal(t, []int{3}, snapshotVersions(eurDoc.Snapshots))
	assert.Empty(t, eurDoc.CurrencyWarning)

	from, to, err := historyViewBounds("2025-02-01", "2025-02-28")
	require.NoError(t, err)
	ranged, err := readHistoryView(db, "dev", from, to, history.CurrencyChoice{})
	require.NoError(t, err)
	assert.Equal(t, []int{2}, snapshotVersions(ranged.Snapshots))

	_, err = readHistoryView(db, "dev", time.Time{}, time.Time{}, history.CurrencyChoice{Strict: true})
	require.ErrorIs(t, err, history.ErrMixedCurrencies)
	assert.True(t, history.IsQueryError(err))
}

func TestReadHistoryDiff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := viewSnap(1, time.January, 10, map[string]float64{"aws": 10})
	first.Resources = []history.CostResource{{URN: "urn:a", Type: "aws:s3:Bucket", MonthlyCost: 10}}
	second := viewSnap(2, time.February, 40, map[string]float64{"aws": 40})
	second.Resources = []history.CostResource{
		{URN: "urn:a", Type: "aws:s3:Bucket", MonthlyCost: 10},
		{URN: "urn:b", Type: "aws:rds:Instance", MonthlyCost: 30},
	}
	seedHistory(t, dir, "dev", first, second)
	db := openSeeded(t, dir, "dev")

	diff, err := readHistoryDiff(db, historyDiffQuery{From: "v1"})
	require.NoError(t, err)
	assert.Equal(t, 2, diff.To.Version)
	assert.InDelta(t, 30, diff.Delta, 0)
	require.Len(t, diff.Added, 1)
	assert.Equal(t, "urn:b", diff.Added[0].URN)

	hidden, err := readHistoryDiff(db, historyDiffQuery{From: "v1", To: "v2", Threshold: 30})
	require.NoError(t, err)
	assert.Empty(t, hidden.Added)

	for _, query := range []historyDiffQuery{
		{From: " "},
		{From: "v1", Threshold: -1},
		{From: "v9"},
		{From: "yesterday"},
		{From: "v1", To: "v7"},
	} {
		_, err = readHistoryDiff(db, query)
		require.Error(t, err, "%+v", query)
		assert.True(t, history.IsQueryError(err), "%+v: %v", query, err)
	}

	emptyDir := t.TempDir()
	seedHistory(t, emptyDir, "dev")
	_, err = readHistoryDiff(openSeeded(t, emptyDir, "dev"), historyDiffQuery{From: "v1"})
	require.ErrorIs(t, err, history.ErrNoSnapshots)
	assert.False(t, history.IsQueryError(err))
}

func TestOpenHistoryTargetReadOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seedScopedHistory(t, dir, "app", "dev", viewSnap(1, time.January, 10, map[string]float64{"aws": 10}))
	name, err := history.CostFileNameFor("app", "dev")
	require.NoError(t, err)
	path := filepath.Join(dir, name)
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	target := historyTarget{Path: path, Project: "app", Stack: "dev", Label: "terraform label"}
	db, err := openHistoryTargetReadOnly(target)
	require.NoError(t, err)
	_, err = readHistoryView(db, "dev", time.Time{}, time.Time{}, history.CurrencyChoice{})
	require.NoError(t, err)
	require.NoError(t, db.Close())
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the read-only open must not stamp the project label")

	_, err = openHistoryTargetReadOnly(historyTarget{Path: path, Stack: "prod"})
	require.ErrorContains(t, err, `cost history database stack "dev" does not match "prod"`)
	assert.False(t, history.IsNoHistory(err))

	_, err = openHistoryTargetReadOnly(historyTarget{Path: filepath.Join(dir, "missing.history.db"), Stack: "qa"})
	require.ErrorIs(t, err, history.ErrNoCostHistory)
	require.EqualError(t, err, "no cost history for stack 'qa'")

	_, err = openHistoryTargetReadOnly(historyTarget{Stack: "qa"})
	require.ErrorIs(t, err, history.ErrNoCostHistory)
}

func openSeeded(t *testing.T, dir, stack string) *history.CostDB {
	t.Helper()
	path, err := historyPath(dir, stack)
	require.NoError(t, err)
	db, err := history.OpenCostDBRead(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func snapshotVersions(snapshots []history.CostSnapshot) []int {
	versions := make([]int, len(snapshots))
	for i, snapshot := range snapshots {
		versions[i] = snapshot.Version
	}
	return versions
}
