package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/stackload"
)

// Both web endpoints must use the same plan-plus-state inventory as CLI pricing.
func TestWebStackLiveInventoryParity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, plan := range []string{"preview-default.json", "preview-show-sames.json", "preview-create-only.json"} {
		t.Run(plan, func(t *testing.T) {
			t.Parallel()
			const state = "testdata/pulumi/stack-export-dev.json"
			planPath := "testdata/pulumi/" + plan
			cmd := &cobra.Command{}
			cmd.Flags().String("stack", "dev", "")
			cli, _, err := loadProjectedResources(
				ctx,
				cmd,
				costProjectedParams{pulumiState: state, pulumiJSONPath: planPath},
				nil,
			)
			require.NoError(t, err)
			snapshot, err := stackload.Load(ctx, stackload.Source{StatePath: state, PlanPath: planPath})
			require.NoError(t, err)
			data := newWebData(&cobra.Command{}, overviewParams{cfg: config.New()}, engine.DateRange{}, nil)
			require.NoError(t, data.setPipelineSource(ctx, OverviewPipelineResult{Snapshot: snapshot}))
			estimates, err := data.estimateResources(ctx)
			require.NoError(t, err)
			var recommended []engine.ResourceDescriptor
			data.fetchRecommendations = func(_ context.Context, _ *cobra.Command, _ *engine.Engine, resources []engine.ResourceDescriptor, _ bool) (*engine.RecommendationsResult, error) {
				recommended = resources
				return &engine.RecommendationsResult{}, nil
			}
			_, err = data.recommendations(ctx, false)
			require.NoError(t, err)
			assert.Equal(t, cli, estimates)
			assert.Equal(t, cli, recommended)
			deployed, err := data.loadResources(ctx)
			require.NoError(t, err)
			assert.Equal(t, snapshot.DeployedDescriptors(), deployed)
			for _, resource := range estimates {
				if resource.Operation == "create" {
					assert.NotContains(t, resourcesByID(deployed), resource.ID)
					assert.Contains(t, resourcesByID(recommended), resource.ID)
				}
			}
		})
	}
}
