package cli

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type stubResolverClient struct {
	proto.CostSourceClient

	calls int
	resp  *pbc.ResolveResourceTypesResponse
	err   error
}

func (s *stubResolverClient) ResolveResourceTypes(
	_ context.Context, _ *pbc.ResolveResourceTypesRequest, _ ...grpc.CallOption,
) (*pbc.ResolveResourceTypesResponse, error) {
	s.calls++
	return s.resp, s.err
}

func newResolverTestClient(withCapability bool, api proto.CostSourceClient) *pluginhost.Client {
	caps := []string{}
	if withCapability {
		caps = []string{capabilityResolveResourceTypes}
	}
	return &pluginhost.Client{
		Name: "aws-plugin",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-plugin",
			SupportedProviders: []string{"aws"},
			Capabilities:       caps,
		},
		API: api,
	}
}

func tfDescriptors() []engine.ResourceDescriptor {
	return []engine.ResourceDescriptor{
		{Type: "aws_instance", ID: "aws_instance.web", Provider: "aws",
			Properties: map[string]interface{}{"instanceType": "t3.micro"}},
		{Type: "aws_s3_bucket", ID: "aws_s3_bucket.assets", Provider: "aws",
			Properties: map[string]interface{}{}},
	}
}

func TestResolveResourceTypes_WithCapability(t *testing.T) {
	t.Parallel()

	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{
		Mappings: map[string]*pbc.ResourceTypeMapping{
			"aws_instance":  {PulumiToken: "aws:ec2/instance:Instance", Supported: true},
			"aws_s3_bucket": {PulumiToken: "aws:s3/bucket:Bucket", Supported: true},
		},
	}}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}

	out := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, "aws:ec2/instance:Instance", out[0].Type)
	assert.Equal(t, "aws:s3/bucket:Bucket", out[1].Type)
}

func TestResolveResourceTypes_CloudNamedPluginResolvesProviderPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		supported  string
		descriptor engine.ResourceDescriptor
		tfType     string
		token      string
	}{
		{
			name:       "azure plugin resolves azurerm types",
			supported:  "azure",
			descriptor: engine.ResourceDescriptor{Type: "azurerm_linux_virtual_machine", ID: "vm", Provider: "azure"},
			tfType:     "azurerm_linux_virtual_machine",
			token:      "azure:compute/linuxVirtualMachine:LinuxVirtualMachine",
		},
		{
			name:       "gcp plugin resolves google types",
			supported:  "gcp",
			descriptor: engine.ResourceDescriptor{Type: "google_compute_instance", ID: "vm", Provider: "gcp"},
			tfType:     "google_compute_instance",
			token:      "gcp:compute/instance:Instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{
				Mappings: map[string]*pbc.ResourceTypeMapping{
					tt.tfType: {PulumiToken: tt.token, Supported: true},
				},
			}}
			client := newResolverTestClient(true, stub)
			client.Metadata.SupportedProviders = []string{tt.supported}

			out := resolveResourceTypes(
				context.Background(), []*pluginhost.Client{client}, nil, []engine.ResourceDescriptor{tt.descriptor},
			)

			assert.Equal(t, 1, stub.calls)
			assert.Equal(t, tt.token, out[0].Type)
		})
	}
}

func TestResolveResourceTypes_FallbackNoCapability(t *testing.T) {
	t.Parallel()

	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{}}
	clients := []*pluginhost.Client{newResolverTestClient(false, stub)}

	out := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	assert.Equal(t, 0, stub.calls)
	assert.Equal(t, "aws_instance", out[0].Type)
	assert.Equal(t, "aws_s3_bucket", out[1].Type)
}

func TestResolveResourceTypes_PulumiTokensUntouched(t *testing.T) {
	t.Parallel()

	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{}}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}
	in := []engine.ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "urn:pulumi:web", Provider: "aws",
			Properties: map[string]interface{}{}},
	}

	out := resolveResourceTypes(context.Background(), clients, nil, in)
	assert.Equal(t, 0, stub.calls)
	assert.Equal(t, "aws:ec2/instance:Instance", out[0].Type)
}

func TestResolveResourceTypes_PropertyMappings(t *testing.T) {
	t.Parallel()

	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{
		Mappings: map[string]*pbc.ResourceTypeMapping{
			"aws_instance": {
				PulumiToken:      "aws:ec2/instance:Instance",
				Supported:        true,
				PropertyMappings: map[string]string{"volume_type": "volumeType"},
			},
		},
	}}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}
	in := []engine.ResourceDescriptor{
		{Type: "aws_instance", ID: "aws_instance.web", Provider: "aws",
			Properties: map[string]interface{}{"volume_type": "gp3"}},
	}

	out := resolveResourceTypes(context.Background(), clients, nil, in)
	assert.Equal(t, "gp3", out[0].Properties["volumeType"])
}

func TestResolveResourceTypes_PropertyMappingsRealFlow(t *testing.T) {
	t.Parallel()

	// Real data flow: MapTerraformResources camelCases attribute keys at
	// ingestion, so the plugin's snake_case mapping key only matches via the
	// camelCase fallback in applyTypeMappings.
	tfResources := []ingest.TerraformStateResource{
		{
			Mode:     "managed",
			Type:     "aws_ebs_volume",
			Name:     "data",
			Provider: `provider["registry.terraform.io/hashicorp/aws"]`,
			Instances: []ingest.TerraformStateInstance{
				{
					Attributes: map[string]interface{}{
						"id":          "vol-0123456789abcdef0",
						"volume_type": "gp3",
						"size":        float64(100),
					},
				},
			},
		},
	}
	in, err := ingest.MapTerraformResources(tfResources)
	require.NoError(t, err)
	require.Len(t, in, 1)
	require.Equal(t, "gp3", in[0].Properties["volumeType"])

	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{
		Mappings: map[string]*pbc.ResourceTypeMapping{
			"aws_ebs_volume": {
				PulumiToken:      "aws:ebs/volume:Volume",
				Supported:        true,
				PropertyMappings: map[string]string{"volume_type": "diskType"},
			},
		},
	}}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}

	out := resolveResourceTypes(context.Background(), clients, nil, in)
	assert.Equal(t, "aws:ebs/volume:Volume", out[0].Type)
	assert.Equal(t, "gp3", out[0].Properties["diskType"])
	assert.Equal(t, "gp3", out[0].Properties["volumeType"])
}

func TestResolveResourceTypes_RPCErrorFallsBack(t *testing.T) {
	t.Parallel()

	stub := &stubResolverClient{err: errors.New("boom")}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}

	out := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	assert.Equal(t, "aws_instance", out[0].Type)
}

func TestResolveResourceTypes_CacheRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := cache.NewBoltStore(context.Background(), t.TempDir(), true, 3600, 0)
	require.NoError(t, err)
	defer store.Close()

	stub1 := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{
		Mappings: map[string]*pbc.ResourceTypeMapping{
			"aws_instance":  {PulumiToken: "aws:ec2/instance:Instance", Supported: true},
			"aws_s3_bucket": {PulumiToken: "aws:s3/bucket:Bucket", Supported: true},
		},
	}}
	clients1 := []*pluginhost.Client{newResolverTestClient(true, stub1)}
	out1 := resolveResourceTypes(context.Background(), clients1, store, tfDescriptors())
	assert.Equal(t, 1, stub1.calls)
	assert.Equal(t, "aws:ec2/instance:Instance", out1[0].Type)

	// Second run with a fresh stub: results must come from the cache, not the RPC.
	stub2 := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{}}
	clients2 := []*pluginhost.Client{newResolverTestClient(true, stub2)}
	out2 := resolveResourceTypes(context.Background(), clients2, store, tfDescriptors())
	assert.Equal(t, 0, stub2.calls)
	assert.Equal(t, "aws:ec2/instance:Instance", out2[0].Type)
}

func TestResolveResourceTypes_InputNotMutated(t *testing.T) {
	t.Parallel()

	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{
		Mappings: map[string]*pbc.ResourceTypeMapping{
			"aws_instance": {
				PulumiToken:      "aws:ec2/instance:Instance",
				Supported:        true,
				PropertyMappings: map[string]string{"volume_type": "volumeType"},
			},
		},
	}}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}
	in := []engine.ResourceDescriptor{
		{Type: "aws_instance", ID: "aws_instance.web", Provider: "aws",
			Properties: map[string]interface{}{"volume_type": "gp3"}},
	}
	propsSnapshot := maps.Clone(in[0].Properties)

	out := resolveResourceTypes(context.Background(), clients, nil, in)
	assert.Equal(t, "aws:ec2/instance:Instance", out[0].Type)
	assert.Equal(t, "gp3", out[0].Properties["volumeType"])

	// The caller's input descriptors must be untouched.
	assert.Equal(t, "aws_instance", in[0].Type)
	assert.Equal(t, propsSnapshot, in[0].Properties)
}

func TestLoadAndMapTerraformResources(t *testing.T) {
	t.Parallel()

	resources, err := loadAndMapTerraformResources(
		context.Background(), "../../examples/plans/terraform-simple-state.json", nil)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, "aws_instance.web", resources[0].ID)
	assert.Equal(t, "aws", resources[0].Provider)
	assert.Equal(t, "aws_s3_bucket.assets", resources[1].ID)
}

func TestWarnNoTypeResolvingPlugin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		clients  []*pluginhost.Client
		wantWarn bool
	}{
		{
			name:     "no clients warns",
			clients:  nil,
			wantWarn: true,
		},
		{
			name:     "client without capability warns",
			clients:  []*pluginhost.Client{newResolverTestClient(false, &stubResolverClient{})},
			wantWarn: true,
		},
		{
			name:     "client with capability stays silent",
			clients:  []*pluginhost.Client{newResolverTestClient(true, &stubResolverClient{})},
			wantWarn: false,
		},
		{
			name: "one capable client among several stays silent",
			clients: []*pluginhost.Client{
				newResolverTestClient(false, &stubResolverClient{}),
				newResolverTestClient(true, &stubResolverClient{}),
			},
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{}
			var stderr strings.Builder
			cmd.SetErr(&stderr)

			warnNoTypeResolvingPlugin(cmd, "state.tfstate", tt.clients)

			if tt.wantWarn {
				warning := stderr.String()
				assert.Contains(t, warning, "resolve_resource_types")
				assert.Contains(t, warning, "finfocus-spec >= v0.6.1")
				assert.Contains(t, warning, terraformTypeResolutionDocsURL)
				assert.Equal(t, 1, strings.Count(warning, "resolve_resource_types"))
			} else {
				assert.Empty(t, stderr.String())
			}
		})
	}
}

func TestWarnNoTypeResolvingPlugin_NoTerraformState(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var stderr strings.Builder
	cmd.SetErr(&stderr)

	warnNoTypeResolvingPlugin(cmd, "", nil)

	assert.Empty(t, stderr.String())
}
