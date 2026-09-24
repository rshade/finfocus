package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	mockplugin "github.com/rshade/finfocus/test/mocks/plugin"
)

const realTerraformStatePath = "../../test/fixtures/terraform/aws-realistic/terraform.tfstate"

// mockServerLauncher satisfies pluginhost.Launcher by dialing an in-process
// mock plugin, so pluginhost.NewClient runs its real GetPluginInfo and
// capability-conversion path over gRPC.
type mockServerLauncher struct {
	server *mockplugin.MockServer
}

func (l mockServerLauncher) Start(
	ctx context.Context, _ string, _ ...string,
) (*grpc.ClientConn, func() error, error) {
	conn, err := l.server.Dial(ctx)
	if err != nil {
		return nil, nil, err
	}
	return conn, conn.Close, nil
}

func newMockResolverClient(
	t *testing.T, configure func(*mockplugin.MockPlugin),
) (*pluginhost.Client, *mockplugin.MockPlugin) {
	t.Helper()
	mock := mockplugin.NewMockPlugin()
	mock.Configure(mockplugin.MockConfig{PluginSpecVersion: pluginsdk.SpecVersion, SupportsAll: true})
	configure(mock)

	server, err := mockplugin.StartMockServerWithPlugin(mock)
	require.NoError(t, err)
	t.Cleanup(server.Stop)

	client, err := pluginhost.NewClient(context.Background(), mockServerLauncher{server: server}, "mock")
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func loadRealTerraformResources(t *testing.T) []engine.ResourceDescriptor {
	t.Helper()
	resources, err := loadAndMapTerraformResources(context.Background(), realTerraformStatePath, nil)
	require.NoError(t, err)
	return resources
}

func resourcesByID(resources []engine.ResourceDescriptor) map[string]engine.ResourceDescriptor {
	byID := make(map[string]engine.ResourceDescriptor, len(resources))
	for _, r := range resources {
		byID[r.ID] = r
	}
	return byID
}

func TestLoadAndMapTerraformResources_RealStateMatchesIngest(t *testing.T) {
	state, err := ingest.LoadTerraformState(realTerraformStatePath)
	require.NoError(t, err)
	want, err := ingest.MapTerraformResources(state.GetManagedResources())
	require.NoError(t, err)

	got := loadRealTerraformResources(t)
	assert.Equal(t, want, got)
	assert.Len(t, got, 14)
}

func TestResolveResourceTypes_RealStateViaMockPlugin(t *testing.T) {
	client, mock := newMockResolverClient(t, func(m *mockplugin.MockPlugin) {
		m.ConfigureTerraformResolver(mockplugin.AWSTerraformTypeMappings())
	})
	require.True(t, client.HasCapability(capabilityResolveResourceTypes))

	var requested []string
	mock.SetResolveResourceTypesHook(func(req *pbc.ResolveResourceTypesRequest) {
		assert.Equal(t, pbc.SourceFormat_SOURCE_FORMAT_TERRAFORM, req.GetSourceFormat())
		requested = append(requested, req.GetSourceTypes()...)
	})

	store, err := cache.NewBoltStore(context.Background(), t.TempDir(), true, 3600, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	in := loadRealTerraformResources(t)
	require.NotContains(t, resourcesByID(in)["aws_ebs_volume.data"].Properties, "volumeType")
	out := resolveResourceTypes(context.Background(), []*pluginhost.Client{client}, store, in)

	assert.Equal(t, int64(1), mock.GetResolveCallCount(), "one RPC per provider")
	assert.ElementsMatch(t, uniqueTerraformTypes(in), requested)

	byID := resourcesByID(out)
	for _, r := range out {
		assert.Contains(t, r.Type, ":", "%s left unresolved", r.ID)
	}
	for _, id := range []string{
		"aws_instance.web", "module.app.aws_instance.app[0]", "module.app.aws_instance.app[1]",
		`aws_instance.worker["small"]`, `aws_instance.worker["large"]`,
	} {
		assert.Equal(t, "aws:ec2/instance:Instance", byID[id].Type, id)
	}
	assert.Equal(t, "m5.large", byID["module.app.aws_instance.app[1]"].Properties["instanceType"])
	assert.Equal(t, "c5.xlarge", byID[`aws_instance.worker["large"]`].Properties["instanceType"])

	volume := byID["aws_ebs_volume.data"]
	assert.Equal(t, "aws:ebs/volume:Volume", volume.Type)
	assert.Equal(t, "gp3", volume.Properties["volumeType"], "property_mappings type -> volumeType")
	assert.Equal(t, "gp3", volume.Properties["type"])

	db := byID["aws_db_instance.main"]
	assert.Equal(t, "aws:rds/instance:Instance", db.Type)
	assert.Equal(t, "db.t3.micro", db.Properties["instanceClass"])

	for _, r := range in {
		assert.NotContains(t, r.Type, ":", "input descriptors must not be mutated")
	}

	t.Run("second run served from resolve_types cache", func(t *testing.T) {
		out2 := resolveResourceTypes(context.Background(), []*pluginhost.Client{client}, store, in)
		assert.Equal(t, int64(1), mock.GetResolveCallCount())
		assert.Equal(t, out, out2)
	})
}

func TestResolveResourceTypes_RealStateFallbacks(t *testing.T) {
	instanceOnly := map[string]*pbc.ResourceTypeMapping{
		"aws_instance": mockplugin.AWSTerraformTypeMappings()["aws_instance"],
	}
	tests := []struct {
		name          string
		configure     func(*mockplugin.MockPlugin)
		wantCalls     int64
		wantResolved  map[string]string
		wantRawSample []string
	}{
		{
			name: "partial resolution keeps raw types for unmapped",
			configure: func(m *mockplugin.MockPlugin) {
				m.ConfigureTerraformResolver(instanceOnly)
			},
			wantCalls:     1,
			wantResolved:  map[string]string{"aws_instance.web": "aws:ec2/instance:Instance"},
			wantRawSample: []string{"aws_ebs_volume.data", "aws_db_instance.main", "aws_s3_bucket.assets"},
		},
		{
			name: "RPC error falls back to raw types",
			configure: func(m *mockplugin.MockPlugin) {
				m.ConfigureTerraformResolver(mockplugin.AWSTerraformTypeMappings())
				m.SetError("ResolveResourceTypes", mockplugin.ErrorUnavailable)
			},
			wantCalls:     1,
			wantRawSample: []string{"aws_instance.web", "aws_ebs_volume.data", "aws_db_instance.main"},
		},
		{
			name: "plugin without capability is never asked",
			configure: func(m *mockplugin.MockPlugin) {
				m.SetPluginInfo([]string{"aws"}, []pbc.PluginCapability{
					pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
				})
				m.SetResourceTypeMappings(mockplugin.AWSTerraformTypeMappings())
			},
			wantCalls:     0,
			wantRawSample: []string{"aws_instance.web", "aws_ebs_volume.data"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := newMockResolverClient(t, tt.configure)
			in := loadRealTerraformResources(t)

			out := resolveResourceTypes(context.Background(), []*pluginhost.Client{client}, nil, in)
			require.Len(t, out, len(in))
			assert.Equal(t, tt.wantCalls, mock.GetResolveCallCount())

			byID := resourcesByID(out)
			for id, token := range tt.wantResolved {
				assert.Equal(t, token, byID[id].Type, id)
			}
			for _, id := range tt.wantRawSample {
				assert.NotContains(t, byID[id].Type, ":", "%s should keep its raw type", id)
			}
		})
	}
}
