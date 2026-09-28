package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
)

func fakeClusters(t *testing.T) ClusterFactory {
	t.Helper()
	return func(scope string) (*Cluster, error) {
		if scope == "missing" {
			return nil, status.Error(codes.InvalidArgument, `kubeconfig context "missing" not found`)
		}
		return &Cluster{Client: fake.NewSimpleClientset(), Context: "kind-test"}, nil
	}
}

func TestGetStats_RejectsHistorical(t *testing.T) {
	p := New(fakeClusters(t))
	_, err := p.GetStats(context.Background(), &pbc.GetStatsRequest{Start: timestamppb.Now()})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "run-rate only")
}

func TestGetStats_UnknownContext(t *testing.T) {
	_, err := New(fakeClusters(t)).GetStats(context.Background(), &pbc.GetStatsRequest{Scope: "missing"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetStats_ValidResponse(t *testing.T) {
	resp, err := New(fakeClusters(t)).GetStats(context.Background(),
		&pbc.GetStatsRequest{Selector: map[string]string{"namespace": "a", "app": "web"}})
	require.NoError(t, err)
	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
}

func TestInfo_ExplicitCapabilitiesOnly(t *testing.T) {
	info := Info("v0.1.0")
	assert.ElementsMatch(t, []pbc.PluginCapability{
		pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS,
		pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION,
	}, info.Capabilities)
}

func TestPlugin_ImplementsProviders(t *testing.T) {
	var p any = New(fakeClusters(t))
	_, ok := p.(pluginsdk.UsageSourceProvider)
	assert.True(t, ok)
	_, ok = p.(pluginsdk.AllocatorProvider)
	assert.True(t, ok)
}

func TestAllocatorConformance(t *testing.T) {
	plugintesting.RunAllocatorConformance(t, New(fakeClusters(t)))
}

// Hosts consult Supports before routing price queries. Since finfocus-spec
// v0.6.2 (#507) the SDK reaches SupportsProvider without a registry, so the
// plugin's own "false" is what hosts see; without it, `cost projected` would
// record a NotSupported error per resource.
func TestSupports_OptsOutOfPricingThroughSDK(t *testing.T) {
	srv := pluginsdk.NewServerWithOptions(New(fakeClusters(t)), nil, nil, Info("v0.1.0"))
	for _, rt := range []string{"aws:ec2/instance:Instance", "ec2", "kubernetes:apps/v1:Deployment"} {
		resp, err := srv.Supports(context.Background(), &pbc.SupportsRequest{
			Resource: &pbc.ResourceDescriptor{ResourceType: rt},
		})
		require.NoError(t, err, rt)
		assert.False(t, resp.GetSupported(), rt)
	}
}
