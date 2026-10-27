package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/stackload"
	"github.com/rshade/finfocus/internal/tui"
)

func testOverviewStateResources(t *testing.T, resources []ingest.StackExportResource) []engine.StateResource {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	data, err := json.Marshal(map[string]any{"version": 3, "deployment": map[string]any{"resources": resources}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	snapshot, err := stackload.LoadState(context.Background(), stackload.Source{StatePath: path})
	require.NoError(t, err)
	return snapshot.State
}
func testOverviewPlanSteps(steps []ingest.PulumiStep) []engine.PlanStep {
	return (&stackload.Snapshot{}).WithPlan(&ingest.PulumiPlan{Steps: steps}).Steps
}
func testOverviewProjectedProperties(step ingest.PulumiStep) map[string]any {
	step.Op = "update"
	return testOverviewPlanSteps([]ingest.PulumiStep{step})[0].ProjectedProperties
}

//nolint:unparam // Mirrors the shared loader arguments used by existing cases.
func testLoadOverviewPlan(
	ctx context.Context,
	params overviewParams,
	dir, stack string,
	pw *string,
) ([]engine.PlanStep, error) {
	snapshot, err := stackload.LoadPlan(
		ctx,
		stackload.Source{PlanPath: params.pulumiJSON, ProjectDir: dir, Stack: stack, Passphrase: pw},
	)
	if err != nil {
		return nil, err
	}
	return snapshot.Steps, nil
}

func testLoadOverviewState(
	ctx context.Context,
	params overviewParams,
	pw *string,
) ([]engine.StateResource, string, string, string, error) {
	snapshot, err := loadOverviewSnapshot(ctx, params, pw, true)
	if err != nil {
		return nil, "", "", "", err
	}
	return snapshot.State, snapshot.ManifestTime, snapshot.ProjectDir, snapshot.StackName, nil
}

//nolint:unparam // Existing row cases exercise both staged and plan-backed loading.
func testBuildOverviewRows(
	ctx context.Context,
	stateOnly bool,
	state []engine.StateResource,
	params overviewParams,
	dir, stack string,
	pw *string,
) ([]engine.OverviewRow, []engine.PlanStep, bool, int, error) {
	snapshot := &stackload.Snapshot{State: state}
	if !stateOnly {
		preview, err := stackload.LoadPlan(
			ctx,
			stackload.Source{PlanPath: params.pulumiJSON, ProjectDir: dir, Stack: stack, Passphrase: pw},
		)
		if err != nil {
			return nil, nil, false, 0, err
		}
		snapshot = snapshot.WithPlan(preview.Plan)
	}
	rows, err := snapshot.Rows(ctx)
	changed, count := snapshot.PendingChanges(ctx)
	return rows, snapshot.Steps, changed, count, err
}

func testRunOverviewPreview(
	ctx context.Context,
	params overviewParams,
	dir, stack string,
	pw *string,
) tui.OverviewChangesReadyMsg {
	pipe := newOverviewPipeline(overviewPipelineConfig{params: params})
	pipe.passphrase = pw
	pipe.result = OverviewPipelineResult{ProjectDir: dir, StackName: stack}
	msg, _ := pipe.RunPreviewWithError(ctx)
	return msg
}
func testOverviewPreviewCmd(ctx context.Context, params overviewParams, dir, stack string, pw *string) tea.Cmd {
	return func() tea.Msg { return testRunOverviewPreview(ctx, params, dir, stack, pw) }
}

func testResolveOverviewData(
	ctx context.Context,
	params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, error) {
	snapshot, err := loadOverviewSnapshot(ctx, params, nil, false)
	if err != nil {
		return nil, nil, "", err
	}
	return snapshot.State, snapshot.Steps, snapshot.StackName, nil
}

func testLoadPlainOverviewData(
	ctx context.Context,
	cmd *cobra.Command,
	params overviewParams,
) ([]engine.StateResource, []engine.PlanStep, string, bool, error) {
	snapshot, _, stateOnly, err := loadPlainOverviewSnapshot(ctx, cmd, params, nil)
	if err != nil {
		return nil, nil, "", false, err
	}
	return snapshot.State, snapshot.Steps, snapshot.StackName, stateOnly, nil
}
