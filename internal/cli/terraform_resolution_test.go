package cli

import (
	"context"
	"errors"
	"maps"
	"testing"

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

func TestResolveResourceTypes_FallbackNoCapability(t *testing.T) {
	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{}}
	clients := []*pluginhost.Client{newResolverTestClient(false, stub)}

	out := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	assert.Equal(t, 0, stub.calls)
	assert.Equal(t, "aws_instance", out[0].Type)
	assert.Equal(t, "aws_s3_bucket", out[1].Type)
}

func TestResolveResourceTypes_PulumiTokensUntouched(t *testing.T) {
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
	stub := &stubResolverClient{err: errors.New("boom")}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}

	out := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	assert.Equal(t, "aws_instance", out[0].Type)
}

func TestResolveResourceTypes_CacheRoundTrip(t *testing.T) {
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
	resources, err := loadAndMapTerraformResources(
		context.Background(), "../../examples/plans/terraform-simple-state.json", nil)
	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, "aws_instance.web", resources[0].ID)
	assert.Equal(t, "aws", resources[0].Provider)
	assert.Equal(t, "aws_s3_bucket.assets", resources[1].ID)
}
