package router

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// specCountingPlugin misses every projected-cost request and counts the
// GetPricingSpec calls it receives. The embedded interface stays nil: a call
// the test does not expect panics instead of passing silently.
type specCountingPlugin struct {
	proto.CostSourceClient

	specCalls atomic.Int32
}

func (p *specCountingPlugin) Supports(
	context.Context, *pbc.SupportsRequest, ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (p *specCountingPlugin) GetProjectedCost(
	context.Context, *proto.GetProjectedCostRequest, ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	return &proto.GetProjectedCostResponse{}, nil
}

func (p *specCountingPlugin) GetPricingSpec(
	context.Context, *pbc.GetPricingSpecRequest, ...grpc.CallOption,
) (*pbc.GetPricingSpecResponse, error) {
	p.specCalls.Add(1)
	return &pbc.GetPricingSpecResponse{}, nil
}

// TestEngine_PricingSpecFallbackKeepsRoutingExclusions runs the real router
// behind the engine adapter. A plugin routed only for ActualCosts must not
// receive GetPricingSpec for a projected miss, even though it lists
// pricing_spec: the router's empty PricingSpec route is not "no routing".
func TestEngine_PricingSpecFallbackKeepsRoutingExclusions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cfg           *config.RoutingConfig
		wantSpecCalls int32
	}{
		{
			name: "a plugin routed only for another feature is not asked",
			cfg: &config.RoutingConfig{Plugins: []config.PluginRouting{
				{Name: "projected", Features: []string{"ProjectedCosts"}, Priority: 20},
				{Name: "spec-only", Features: []string{"ActualCosts"}, Priority: 10},
			}},
			wantSpecCalls: 0,
		},
		{
			name: "a plugin routed for ProjectedCosts is asked",
			cfg: &config.RoutingConfig{Plugins: []config.PluginRouting{
				{Name: "projected", Features: []string{"ProjectedCosts"}, Priority: 20},
				{Name: "spec-only", Features: []string{"ProjectedCosts"}, Priority: 10},
			}},
			wantSpecCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectedPlugin := &specCountingPlugin{}
			specPlugin := &specCountingPlugin{}
			withCaps := func(name string, api proto.CostSourceClient, caps ...string) *pluginhost.Client {
				client := mockClient(name, []string{"aws"})
				client.Metadata.Capabilities = caps
				client.API = api
				return client
			}
			clients := []*pluginhost.Client{
				withCaps("projected", projectedPlugin, "projected_costs"),
				withCaps("spec-only", specPlugin, "pricing_spec"),
			}
			realRouter, err := NewRouter(WithClients(clients), WithConfig(tt.cfg))
			require.NoError(t, err)

			eng := engine.New(clients, nil).
				WithRouter(NewEngineAdapter(realRouter)).
				WithPricingSpecFallback(true)

			resource := engine.ResourceDescriptor{
				Type:       "aws:ec2/instance:Instance",
				ID:         "i-123",
				Provider:   "aws",
				Properties: map[string]any{"instanceType": "t3.micro"},
			}
			_, err = eng.GetProjectedCost(context.Background(), []engine.ResourceDescriptor{resource})
			require.NoError(t, err)

			assert.Equal(t, tt.wantSpecCalls, specPlugin.specCalls.Load())
			assert.Zero(t, projectedPlugin.specCalls.Load(), "a plugin without pricing_spec is never asked")
		})
	}
}
