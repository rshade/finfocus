package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/terraform"
)

func TestRecordTerraformProjection(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			project := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
			t.Chdir(project)
			dir := filepath.Join(t.TempDir(), "history")
			t.Setenv(config.HistoryEnvDir, dir)
			t.Setenv(config.HistoryEnvEnabled, "")
			cfg := config.New()
			cfg.Cost.History.Enabled = &enabled
			previous := config.GetGlobalConfig()
			config.SetGlobalConfig(cfg)
			t.Cleanup(func() { config.SetGlobalConfig(previous) })
			cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd())
			recordTerraformProjection(
				context.Background(),
				cmd,
				[]engine.CostResult{
					{
						ResourceID:   "aws_instance.web",
						ResourceType: "aws_instance",
						Monthly:      12,
						Currency:     "USD",
					},
				},
			)
			if !enabled {
				_, err := os.Stat(dir)
				assert.True(t, os.IsNotExist(err))
				return
			}
			identity, err := terraform.ResolveIdentity(context.Background(), project, false)
			require.NoError(t, err)
			filename, err := history.CostFileNameFor(identity.Project, identity.Stack)
			require.NoError(t, err)
			db, err := history.OpenCostDBRead(filepath.Join(dir, filename))
			require.NoError(t, err)
			defer func() { require.NoError(t, db.Close()) }()
			stats, err := db.Stats()
			require.NoError(t, err)
			assert.Equal(t, identity.Project, stats.Project)
			assert.Equal(t, "unscoped", stats.Stack)
			assert.EqualValues(t, 1, stats.Schema)
			snaps, err := db.Snapshots(time.Time{}, time.Time{})
			require.NoError(t, err)
			require.Len(t, snaps, 1)
			assert.InDelta(t, 12.0, snaps[0].TotalMonthly, 0.001)
			assert.InDelta(t, 12.0, snaps[0].ByProvider["aws"], 0.001)
			require.Len(t, snaps[0].Resources, 1)
			assert.Equal(t, "aws_instance.web", snaps[0].Resources[0].URN)
		})
	}
}

func TestRecordTerraformProjectionSkipsExplicitStateInputs(t *testing.T) {
	for _, flag := range []string{"pulumi-json", "pulumi-state", "terraform-state"} {
		t.Run(flag, func(t *testing.T) {
			project := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
			t.Chdir(project)
			dir := filepath.Join(t.TempDir(), "history")
			t.Setenv(config.HistoryEnvDir, dir)
			t.Setenv(config.HistoryEnvEnabled, "true")
			cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd())
			cmd.Flags().String(flag, "input.json", "")
			recordTerraformProjection(context.Background(), cmd, nil)
			_, err := os.Stat(dir)
			assert.True(
				t,
				os.IsNotExist(err),
				"explicit state inputs must not create Terraform plan history",
			)
		})
	}
}

func TestRecordTerraformProjectionMixedCurrencyVersions(t *testing.T) {
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
	t.Chdir(project)
	dir := t.TempDir()
	t.Setenv(config.HistoryEnvDir, dir)
	t.Setenv(config.HistoryEnvEnabled, "true")
	previous := config.GetGlobalConfig()
	config.SetGlobalConfig(config.New())
	t.Cleanup(func() { config.SetGlobalConfig(previous) })
	cmd, _ := preparedHistoryCmd(t, NewCostHistoryViewCmd())
	recordTerraformProjection(context.Background(), cmd, []engine.CostResult{
		{ResourceID: "a", ResourceType: "aws_instance", Monthly: 12, Currency: "USD"},
		{ResourceID: "b", ResourceType: "aws_instance", Monthly: 8, Currency: "EUR"},
	})
	target, err := resolveHistoryCommandTarget(cmd, dir)
	require.NoError(t, err)
	db, err := history.OpenCostDBRead(target.Path)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	assert.NotEqual(t, snaps[0].Version, snaps[1].Version)
	assert.NotEqual(t, snaps[0].Currency, snaps[1].Currency)
}

func TestTerraformProjectionCommandRecordsHistory(t *testing.T) {
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
	plan := filepath.Join(project, "plan.json")
	require.NoError(
		t,
		os.WriteFile(plan, []byte(`{"format_version":"1.2","resource_changes":[]}`), 0o600),
	)
	t.Chdir(project)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	dir := t.TempDir()
	t.Setenv(config.HistoryEnvDir, dir)
	t.Setenv(config.HistoryEnvEnabled, "true")
	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"cost", "projected", "--terraform-plan", plan},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	identity, err := terraform.ResolveIdentity(context.Background(), project, false)
	require.NoError(t, err)
	name, err := history.CostFileNameFor(identity.Project, identity.Stack)
	require.NoError(t, err)
	db, err := history.OpenCostDBRead(filepath.Join(dir, name))
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	snapshots, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Zero(t, snapshots[0].TotalMonthly)
}
