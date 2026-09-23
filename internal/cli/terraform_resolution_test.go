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

	out, err := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	require.NoError(t, err)
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, "aws:ec2/instance:Instance", out[0].Type)
	assert.Equal(t, "aws:s3/bucket:Bucket", out[1].Type)
}

func TestResolveResourceTypes_FallbackNoCapability(t *testing.T) {
	stub := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{}}
	clients := []*pluginhost.Client{newResolverTestClient(false, stub)}

	out, err := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	require.NoError(t, err)
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

	out, err := resolveResourceTypes(context.Background(), clients, nil, in)
	require.NoError(t, err)
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

	out, err := resolveResourceTypes(context.Background(), clients, nil, in)
	require.NoError(t, err)
	assert.Equal(t, "gp3", out[0].Properties["volumeType"])
}

func TestResolveResourceTypes_RPCErrorFallsBack(t *testing.T) {
	stub := &stubResolverClient{err: errors.New("boom")}
	clients := []*pluginhost.Client{newResolverTestClient(true, stub)}

	out, err := resolveResourceTypes(context.Background(), clients, nil, tfDescriptors())
	require.NoError(t, err)
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
	out1, err := resolveResourceTypes(context.Background(), clients1, store, tfDescriptors())
	require.NoError(t, err)
	assert.Equal(t, 1, stub1.calls)
	assert.Equal(t, "aws:ec2/instance:Instance", out1[0].Type)

	// Second run with a fresh stub: results must come from the cache, not the RPC.
	stub2 := &stubResolverClient{resp: &pbc.ResolveResourceTypesResponse{}}
	clients2 := []*pluginhost.Client{newResolverTestClient(true, stub2)}
	out2, err := resolveResourceTypes(context.Background(), clients2, store, tfDescriptors())
	require.NoError(t, err)
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

	out, err := resolveResourceTypes(context.Background(), clients, nil, in)
	require.NoError(t, err)
	assert.Equal(t, "aws:ec2/instance:Instance", out[0].Type)
	assert.Equal(t, "gp3", out[0].Properties["volumeType"])

	// The caller's input descriptors must be untouched.
	assert.Equal(t, "aws_instance", in[0].Type)
	assert.Equal(t, propsSnapshot, in[0].Properties)
}
