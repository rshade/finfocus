package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	mockplugin "github.com/rshade/finfocus/test/mocks/plugin"
)

//nolint:paralleltest // Dials the in-process mock plugin shared by these command preparation tests.
func TestEstimateAndRecommendationsResolveTerraformPlans(t *testing.T) {
	for _, command := range []string{"estimate", "recommendations"} {
		t.Run(command, func(t *testing.T) {
			client, mock := newMockResolverClient(t, func(m *mockplugin.MockPlugin) {
				m.ConfigureTerraformResolver(mockplugin.AWSTerraformTypeMappings())
			})
			source := []engine.ResourceDescriptor{{
				ID: "aws_ebs_volume.data", Type: "aws_ebs_volume", Provider: "aws", Operation: "replace",
				Properties: map[string]any{"type": "gp3"}, OldProperties: map[string]any{"type": "gp2"},
			}}
			clients := []*pluginhost.Client{client}
			var resolved []engine.ResourceDescriptor
			if command == "estimate" {
				resolved = prepareEstimateResources(context.Background(), clients, source)
			} else {
				resolved = prepareRecommendationResources(context.Background(), clients, nil, source)
			}
			require.Len(t, resolved, 1)
			assert.Equal(t, "aws:ebs/volume:Volume", resolved[0].Type)
			assert.Equal(t, "gp3", resolved[0].Properties["volumeType"])
			assert.Equal(t, "aws_ebs_volume.data", resolved[0].ID)
			assert.Equal(t, "replace", resolved[0].Operation)
			assert.Equal(t, int64(1), mock.GetResolveCallCount())
			assert.Equal(t, "aws_ebs_volume", source[0].Type)
		})
	}
}
