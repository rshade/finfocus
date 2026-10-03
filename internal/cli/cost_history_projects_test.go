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

const (
	appAURN = "urn:pulumi:dev::appa::aws:ec2/instance:Instance::web"
	appBURN = "urn:pulumi:dev::appb::aws:ec2/instance:Instance::web"
)

func writeLegacyHistory(t *testing.T, dir, stack, urn string) string {
	t.Helper()
	path := filepath.Join(dir, stack+".history.db")
	db, err := history.OpenCostDB(path, stack)
	require.NoError(t, err)
	if urn != "" {
		snapshot := history.ZeroSnapshot(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1)
		snapshot.Resources = []history.CostResource{{URN: urn}}
		require.NoError(t, db.Put(snapshot, history.CostAnnotation{Version: 1}))
	}
	require.NoError(t, db.Close())
	return path
}

func writeScopedHistory(t *testing.T, dir, project, stack string) string {
	t.Helper()
	name, err := history.CostFileNameFor(project, stack)
	require.NoError(t, err)
	path := filepath.Join(dir, name)
	db, err := history.OpenCostDBFor(path, project, stack)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return path
}

func TestResolveHistoryTarget(t *testing.T) {
	t.Parallel()

	t.Run("two projects with one stack name get separate databases", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		first, err := resolveHistoryTargetFor(dir, "dev", "appa")
		require.NoError(t, err)
		second, err := resolveHistoryTargetFor(dir, "dev", "appb")
		require.NoError(t, err)

		assert.Equal(t, filepath.Join(dir, "appa@dev.history.db"), first.Path)
		assert.Equal(t, filepath.Join(dir, "appb@dev.history.db"), second.Path)
		assert.Equal(t, "appa", first.Project)
		assert.Equal(t, "dev", first.Stack)
	})

	t.Run("a legacy database whose URNs name this project is kept", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		legacy := writeLegacyHistory(t, dir, "dev", appAURN)

		owner, err := resolveHistoryTargetFor(dir, "dev", "appa")
		require.NoError(t, err)
		assert.Equal(t, legacy, owner.Path)
		assert.Equal(t, "appa", owner.Project)

		other, err := resolveHistoryTargetFor(dir, "dev", "appb")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "appb@dev.history.db"), other.Path)
	})

	t.Run("a legacy database with no project clues is adopted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		legacy := writeLegacyHistory(t, dir, "dev", "")

		target, err := resolveHistoryTargetFor(dir, "dev", "appa")
		require.NoError(t, err)
		assert.Equal(t, legacy, target.Path)
		assert.Equal(t, "appa", target.Project)
	})

	t.Run("a project database wins over a legacy one", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeLegacyHistory(t, dir, "dev", appAURN)
		scoped := writeScopedHistory(t, dir, "appa", "dev")

		target, err := resolveHistoryTargetFor(dir, "dev", "appa")
		require.NoError(t, err)
		assert.Equal(t, scoped, target.Path)
	})

	t.Run("a fully qualified stack names its project", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target, err := resolveHistoryTargetFor(dir, "org/appa/dev", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "appa@dev.history.db"), target.Path)
		assert.Equal(t, "dev", target.Stack)
	})

	t.Run("with no project a legacy database is used", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		legacy := writeLegacyHistory(t, dir, "dev", appAURN)
		writeScopedHistory(t, dir, "appb", "dev")

		target, err := resolveHistoryTargetFor(dir, "dev", "")
		require.NoError(t, err)
		assert.Equal(t, legacy, target.Path)
		assert.Empty(t, target.Project)
	})

	t.Run("with no project a single project database is used", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		scoped := writeScopedHistory(t, dir, "appa", "dev")
		writeScopedHistory(t, dir, "appa", "prod")

		target, err := resolveHistoryTargetFor(dir, "dev", "")
		require.NoError(t, err)
		assert.Equal(t, scoped, target.Path)
		assert.Equal(t, "appa", target.Project)
	})

	t.Run("with no project several project databases are ambiguous", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeScopedHistory(t, dir, "appa", "dev")
		writeScopedHistory(t, dir, "appb", "dev")

		_, err := resolveHistoryTargetFor(dir, "dev", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "appa")
		assert.Contains(t, err.Error(), "appb")
		assert.Contains(t, err.Error(), "several projects")
	})

	t.Run("with no project and no database the legacy name is used", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target, err := resolveHistoryTargetFor(dir, "dev", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "dev.history.db"), target.Path)
	})

	t.Run("a missing history directory is not an error", func(t *testing.T) {
		t.Parallel()
		target, err := resolveHistoryTargetFor(filepath.Join(t.TempDir(), "missing"), "dev", "")
		require.NoError(t, err)
		assert.Empty(t, target.Project)
	})
}

func enterProject(t *testing.T, name string) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "Pulumi.yaml"), []byte("name: "+name+"\nruntime: go\n"), 0o600))
	t.Chdir(root)
}

func collectStack(t *testing.T, dir, urn string, versions int) error {
	t.Helper()
	rows := make([]map[string]any, 0, versions)
	exports := make(map[int][]byte, versions)
	for v := 1; v <= versions; v++ {
		rows = append(
			rows,
			histRow(v, time.Date(2025, time.Month(v), 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), "update"),
		)
		exports[v] = exportBody(urn, "t3.micro")
	}
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryCollectCmd(), "--stack", "dev", "--parallel", "1")
	return runCollect(cmd, collectDeps{
		dir:     dir,
		look:    pulumiLook,
		run:     scriptedHistoryPulumi(historyBody(t, rows...), exports, []byte(`[{"name":"dev"}]`)),
		price:   fixedPrice(5, "aws-public"),
		confirm: func(string) bool { t.Error("collect asked to reset history"); return false },
	})
}

func snapshotCount(t *testing.T, path string) int {
	t.Helper()
	db, err := history.OpenCostDBRead(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	return len(snaps)
}

//nolint:paralleltest // t.Chdir changes the process-wide working directory
func TestCollect_SameStackNameInTwoProjectsKeepsSeparateHistories(t *testing.T) {
	dir := t.TempDir()

	enterProject(t, "appa")
	require.NoError(t, collectStack(t, dir, appAURN, 5))

	enterProject(t, "appb")
	require.NoError(t, collectStack(t, dir, appBURN, 3))

	assert.Equal(t, 5, snapshotCount(t, filepath.Join(dir, "appa@dev.history.db")))
	assert.Equal(t, 3, snapshotCount(t, filepath.Join(dir, "appb@dev.history.db")))
	assert.NoFileExists(t, filepath.Join(dir, "dev.history.db"))

	enterProject(t, "appa")
	require.NoError(t, collectStack(t, dir, appAURN, 5))
	assert.Equal(t, 5, snapshotCount(t, filepath.Join(dir, "appa@dev.history.db")))
}

//nolint:paralleltest // t.Chdir changes the process-wide working directory
func TestCollect_LegacyHistoryStaysWithItsProject(t *testing.T) {
	dir := t.TempDir()
	legacy := writeLegacyHistory(t, dir, "dev", appAURN)

	enterProject(t, "appb")
	require.NoError(t, collectStack(t, dir, appBURN, 2))

	assert.Equal(t, 1, snapshotCount(t, legacy))
	assert.Equal(t, 2, snapshotCount(t, filepath.Join(dir, "appb@dev.history.db")))

	enterProject(t, "appa")
	require.NoError(t, collectStack(t, dir, appAURN, 2))
	assert.Equal(t, 2, snapshotCount(t, legacy))
}

//nolint:paralleltest // t.Chdir changes the process-wide working directory
func TestListShowsTheProjectOfEachDatabase(t *testing.T) {
	dir := t.TempDir()
	writeScopedHistory(t, dir, "appa", "dev")
	writeScopedHistory(t, dir, "appb", "dev")
	writeLegacyHistory(t, dir, "old", "")

	stats, err := history.ListCostDBs(dir)
	require.NoError(t, err)
	labels := make([]string, 0, len(stats))
	for _, item := range historyListItems(stats) {
		labels = append(labels, item.Project+"/"+item.Stack)
	}
	assert.Equal(t, []string{"appa/dev", "appb/dev", "/old"}, labels)
}
