package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// Keep the independent state/preview merge as the baseline while each adapter
// moves its input loading into stackload.
//
//nolint:paralleltest // History configuration is isolated with process environment.
func TestOverviewLoaderSnapshotParity(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()
	statePath := overviewFixture(t, "state-no-changes.json")
	export, err := ingest.LoadStackExportWithContext(ctx, statePath)
	require.NoError(t, err)
	state := testOverviewStateResources(t, export.GetCustomResourcesWithContext(ctx))
	for _, planName := range []string{"", "plan-no-changes.json", "plan-mixed-changes.json"} {
		t.Run(planName, func(t *testing.T) {
			params := overviewParams{
				pulumiState: statePath,
				stateOnly:   planName == "",
				yes:         true,
				cfg:         &config.Config{},
			}
			var want []engine.OverviewRow
			if planName == "" {
				want = engine.NewRowsFromState(ctx, state)
			} else {
				params.pulumiJSON = overviewFixture(t, planName)
				plan, loadErr := ingest.LoadPulumiPlanWithContext(ctx, params.pulumiJSON)
				require.NoError(t, loadErr)
				want, err = engine.MergeResourcesForOverview(ctx, state, testOverviewPlanSteps(plan.Steps))
				require.NoError(t, err)
			}
			for _, loader := range []struct {
				name string
				load OverviewPipelineLoader
			}{
				{"plain", plainOverviewLoader}, {"terminal", tuiOverviewLoader}, {"web", autoDetectOverviewLoader},
			} {
				t.Run(loader.name, func(t *testing.T) {
					pipe := newOverviewPipeline(overviewPipelineConfig{cmd: &cobra.Command{}, params: params})
					data, loadErr := loader.load(ctx, pipe)
					require.NoError(t, loadErr)
					require.NotNil(t, data.Snapshot)
					assert.Equal(t, want, data.Rows)
					assert.Equal(t, planName == "", data.IsStateOnly)
				})
			}
		})
	}
}

//nolint:paralleltest // History configuration is isolated with process environment.
func TestOverviewStandalonePlanLoaderParity(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	ctx := context.Background()
	params := overviewParams{pulumiJSON: overviewFixture(t, "plan-mixed-changes.json"), cfg: &config.Config{}}
	plan, err := ingest.LoadPulumiPlanWithContext(ctx, params.pulumiJSON)
	require.NoError(t, err)
	want, err := engine.MergeResourcesForOverview(ctx, nil, testOverviewPlanSteps(plan.Steps))
	require.NoError(t, err)
	for _, loader := range []struct {
		name string
		load OverviewPipelineLoader
	}{
		{"plain", plainOverviewLoader}, {"terminal", tuiOverviewLoader}, {"web", autoDetectOverviewLoader},
	} {
		t.Run(loader.name, func(t *testing.T) {
			pipe := newOverviewPipeline(overviewPipelineConfig{cmd: &cobra.Command{}, params: params})
			data, loadErr := loader.load(ctx, pipe)
			require.NoError(t, loadErr)
			assert.Equal(t, want, data.Rows)
			assert.False(t, data.Snapshot.StateStatus.Loaded)
			assert.True(t, data.Snapshot.PlanStatus.Loaded)
		})
	}
}
