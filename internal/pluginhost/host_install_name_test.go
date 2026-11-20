package pluginhost_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestNewClient_RecordsInstallName(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", t.TempDir())
	pluginhost.ResetPluginEnvDigestsForTest()
	t.Cleanup(pluginhost.ResetPluginEnvDigestsForTest)

	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"opened by the registry", pluginhost.WithInstallName(context.Background(), "aws-public"), "aws-public"},
		{"opened without an install name", context.Background(), ""},
	}
	for _, tc := range tests { //nolint:paralleltest // the parent sets FINFOCUS_HOME with t.Setenv
		t.Run(tc.name, func(t *testing.T) {
			srv := &mockCostSourceServer{
				name: "finfocus-plugin-aws-public",
				pluginInfo: &pbc.GetPluginInfoResponse{
					Name:        "finfocus-plugin-aws-public",
					Version:     "0.2.5",
					SpecVersion: pluginsdk.SpecVersion,
				},
			}
			launcher, cleanup := setupMockServer(t, srv)
			t.Cleanup(cleanup)

			client, err := pluginhost.NewClient(tc.ctx, launcher, "finfocus-plugin-aws-public")
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			assert.Equal(t, "finfocus-plugin-aws-public", client.Name)
			assert.Equal(t, tc.want, client.InstallName)
			assert.Empty(t, client.Aliases, "the registry, not the host, sets aliases")
		})
	}
}
