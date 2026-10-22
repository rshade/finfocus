package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/terraform"
)

//nolint:paralleltest // Changes the working directory to exercise project detection.
func TestTerraformHistoryCommandsUseProjectIdentity(t *testing.T) {
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
	t.Chdir(project)
	dir := t.TempDir()
	identity, err := terraform.ResolveIdentity(context.Background(), project, false)
	require.NoError(t, err)
	name, err := history.CostFileNameFor(identity.Project, identity.Stack)
	require.NoError(t, err)
	db, err := history.OpenCostDBFor(filepath.Join(dir, name), identity.Project, identity.Stack)
	require.NoError(t, err)
	require.NoError(
		t,
		db.Put(
			history.CostSnapshot{
				Timestamp:    time.Now(),
				Version:      1,
				Currency:     "USD",
				TotalMonthly: 12,
			},
			history.CostAnnotation{},
		),
	)
	require.NoError(t, db.Close())
	other, err := history.OpenCostDBFor(
		filepath.Join(dir, "other@unscoped.history.db"),
		"other",
		"unscoped",
	)
	require.NoError(t, err)
	require.NoError(t, other.Close())
	t.Run("view", func(t *testing.T) {
		cmd, out := preparedHistoryCmd(t, NewCostHistoryViewCmd(), "--output", "json")
		require.NoError(t, runView(cmd, viewDeps{dir: dir}))
		assert.Contains(t, out.String(), `"stack": "unscoped"`)
		assert.Contains(t, out.String(), `"total_monthly": 12`)
	})
	t.Run("export", func(t *testing.T) {
		cmd, out := preparedHistoryCmd(t, NewCostHistoryExportCmd(), "--format", "json")
		require.NoError(t, runExport(cmd, dir))
		assert.Contains(t, out.String(), "unscoped")
	})
	t.Run("diff", func(t *testing.T) {
		cmd, out := preparedHistoryCmd(
			t,
			NewCostHistoryDiffCmd(),
			"--from",
			"v1",
			"--output",
			"json",
		)
		require.NoError(t, runDiff(cmd, dir))
		assert.Contains(t, out.String(), `"stack": "unscoped"`)
	})
	t.Run("list", func(t *testing.T) {
		cmd, out := preparedHistoryCmd(t, NewCostHistoryListCmd(), "--format", "json")
		require.NoError(t, runList(cmd, dir))
		assert.Contains(t, out.String(), identity.Project)
		assert.NotContains(t, out.String(), "other@")
	})
	t.Run("prune", func(t *testing.T) {
		cmd, out := preparedHistoryCmd(t, NewCostHistoryPruneCmd(), "--keep", "1", "--dry-run")
		require.NoError(t, runPrune(cmd, dir))
		assert.Contains(t, out.String(), "unscoped")
	})
}

func TestTerraformHistoryFalseFlagOverridesConfig(t *testing.T) {
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
	t.Chdir(project)
	cfg := config.New()
	cfg.Cost.Terraform.IncludeWorkspace = true
	previous := config.GetGlobalConfig()
	config.SetGlobalConfig(cfg)
	t.Cleanup(func() { config.SetGlobalConfig(previous) })
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd())
	cmd.Flags().Bool("include-workspace", false, "")
	require.NoError(t, cmd.Flags().Set("include-workspace", "false"))
	target, err := resolveHistoryCommandTarget(cmd, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "unscoped", target.Stack)
}

func TestHistoryIncludeWorkspaceIgnoredForPulumi(t *testing.T) {
	project := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(project, "Pulumi.yaml"),
			[]byte("name: example\nruntime: go\n"),
			0o600,
		),
	)
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
	t.Chdir(project)
	cfg := config.New()
	cfg.Cost.Terraform.IncludeWorkspace = true
	previous := config.GetGlobalConfig()
	config.SetGlobalConfig(cfg)
	t.Cleanup(func() { config.SetGlobalConfig(previous) })
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd(), "--stack", "dev")
	addIncludeWorkspaceFlag(cmd)
	require.NoError(t, cmd.Flags().Set("include-workspace", "true"))
	dir := t.TempDir()
	target, err := resolveHistoryCommandTarget(cmd, dir)
	require.NoError(t, err)
	assert.Equal(t, "example", target.Project)
	assert.Equal(t, "dev", target.Stack)
	assert.Equal(t, filepath.Join(dir, "example@dev.history.db"), target.Path)
}
