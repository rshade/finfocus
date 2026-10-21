package pluginhost_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/esc/esctest"
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

func TestNewClient_ESCRestartsOnce(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", t.TempDir())
	pluginhost.ResetPluginEnvDigestsForTest()
	t.Cleanup(pluginhost.ResetPluginEnvDigestsForTest)

	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/prod": {
			ETag: "etag-1",
			Variables: map[string]esctest.Variable{
				"FINFOCUS_RECORDER_MOCK_RESPONSE": {Value: "from-esc"},
			},
		},
	})
	t.Cleanup(api.Close)
	list := []string{"acme/billing/prod"}
	src := esc.NewSource(esc.Options{
		Global: &list,
		Getenv: func(key string) string {
			switch key {
			case "PULUMI_ACCESS_TOKEN":
				return "tok"
			case "PULUMI_BACKEND_URL":
				return api.URL
			default:
				return ""
			}
		},
		Budget: time.Second,
	})

	srv := &mockCostSourceServer{
		name: "recorder",
		pluginInfo: &pbc.GetPluginInfoResponse{
			Name:        "recorder",
			Version:     "1.0.0",
			SpecVersion: pluginsdk.SpecVersion,
			Settings: []*pbc.PluginSetting{{
				Key: "mock_response",
				Env: "FINFOCUS_RECORDER_MOCK_RESPONSE",
			}},
		},
	}
	inner, cleanup := setupMockServer(t, srv)
	t.Cleanup(cleanup)
	launcher := &recordingLauncher{inner: inner}
	ctx := esc.WithSource(pluginhost.WithInstallName(context.Background(), "recorder"), src)
	client, err := pluginhost.NewClient(ctx, launcher, "recorder")
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	require.Len(t, launcher.envs, 2)
	assert.NotContains(t, launcher.envs[0], "FINFOCUS_RECORDER_MOCK_RESPONSE=from-esc")
	assert.Contains(t, launcher.envs[1], "FINFOCUS_RECORDER_MOCK_RESPONSE=from-esc")
	assert.Equal(t, 1, api.Opens("acme", "billing", "prod"))
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
	delivered, _ := pluginhost.DeliveredEnvFromContext(ctx)
	r.envs = append(r.envs, append([]string(nil), delivered.Pairs...))
	return r.inner.Start(ctx, path, args...)
}
