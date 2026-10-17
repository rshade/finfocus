package pluginhost_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestNewClient_RestartsOnceForAdvertisedDefault(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", t.TempDir())
	pluginhost.ResetPluginEnvDigestsForTest()
	t.Cleanup(pluginhost.ResetPluginEnvDigestsForTest)

	srv := &mockCostSourceServer{
		name: "recorder",
		pluginInfo: &pbc.GetPluginInfoResponse{
			Name:        "recorder",
			Version:     "1.0.0",
			SpecVersion: pluginsdk.SpecVersion,
			Settings: []*pbc.PluginSetting{{
				Key:          "mock_response",
				Env:          "FINFOCUS_RECORDER_MOCK_RESPONSE",
				DefaultValue: "from-default",
			}},
		},
	}
	inner, cleanup := setupMockServer(t, srv)
	t.Cleanup(cleanup)
	launcher := &recordingLauncher{inner: inner}

	client, err := pluginhost.NewClient(
		pluginhost.WithInstallName(context.Background(), "recorder"), launcher, "recorder")
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	require.Len(t, launcher.envs, 2)
	assert.Empty(t, launcher.envs[0])
	assert.Equal(t, []string{"FINFOCUS_RECORDER_MOCK_RESPONSE=from-default"}, launcher.envs[1])
	assert.NotEmpty(t, client.DeliveredDigest)
	assert.Empty(t, pluginhost.PluginEnvDigest())
}

type recordingLauncher struct {
	inner *grpcMockLauncher
	envs  [][]string
}

func (r *recordingLauncher) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	pairs, _ := pluginhost.DeliveredEnvFromContext(ctx)
	r.envs = append(r.envs, append([]string(nil), pairs...))
	return r.inner.Start(ctx, path, args...)
}
