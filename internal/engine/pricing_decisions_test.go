package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestPluginSpecPricesTheSameWhetherQuotedPerHourOrPerDay(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"}
	hourly, ok := costFromPluginPricingSpec(
		resource,
		&pbc.PricingSpec{BillingMode: billingPerHour, RatePerUnit: 1},
		"p",
	)
	require.True(t, ok)
	daily, ok := costFromPluginPricingSpec(resource, &pbc.PricingSpec{BillingMode: billingPerDay, RatePerUnit: 24}, "p")
	require.True(t, ok)

	assert.InDelta(t, 730.0, hourly.Monthly, 1e-9)
	assert.InDelta(t, hourly.Monthly, daily.Monthly, 1e-9, "$1/h and $24/day are the same price")
}

func TestPluginSpecCompoundModesSayWhatTheyLeaveOut(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{Type: "aws:lb:LoadBalancer", ID: "lb-1", Provider: "aws"}
	for _, mode := range []string{"per_hour_plus_data", "per_hour_plus_lcu", "per_hour_plus_nlcu"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			got, ok := costFromPluginPricingSpec(resource, &pbc.PricingSpec{
				BillingMode: mode, Unit: "hour", RatePerUnit: 0.1,
			}, "aws-public")
			require.True(t, ok)
			assert.InDelta(t, 73.0, got.Monthly, 1e-9, "the hourly part is still priced")
			assert.Contains(t, got.Notes, mode)
			assert.Contains(t, got.Notes, "hourly rate only")
		})
	}

	t.Run("a plain mode has no such note", func(t *testing.T) {
		t.Parallel()
		got, ok := costFromPluginPricingSpec(resource, &pbc.PricingSpec{
			BillingMode: billingPerHour, Unit: "hour", RatePerUnit: 0.1,
		}, "aws-public")
		require.True(t, ok)
		assert.NotContains(t, got.Notes, "hourly rate only")
	})

	t.Run("an empty mode with a unit has no such note", func(t *testing.T) {
		t.Parallel()
		got, ok := costFromPluginPricingSpec(resource, &pbc.PricingSpec{Unit: "hour", RatePerUnit: 0.1}, "aws-public")
		require.True(t, ok)
		assert.NotContains(t, got.Notes, "hourly rate only")
	})
}

func TestMatchingPricingTierDoesNotGuess(t *testing.T) {
	t.Parallel()

	tiers := []*pbc.PricingTier{
		{MinQuantity: 10, MaxQuantity: 50, RatePerUnit: 0.10},
		{MinQuantity: 50, MaxQuantity: 100, RatePerUnit: 0.05},
	}

	assert.InDelta(t, 0.10, matchingPricingTier(tiers, 10).GetRatePerUnit(), 1e-9)
	assert.InDelta(t, 0.05, matchingPricingTier(tiers, 99).GetRatePerUnit(), 1e-9)
	assert.Nil(t, matchingPricingTier(tiers, 5), "below every tier")
	assert.Nil(t, matchingPricingTier(tiers, 100), "above every tier")
}

func TestPluginSpecTierGapFallsThroughInsteadOfUsingTheFirstRate(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{
		Type: "aws:s3:Bucket", ID: "b", Provider: "aws", Properties: map[string]any{"sizeGb": 500},
	}
	got, ok := costFromPluginPricingSpec(resource, &pbc.PricingSpec{
		BillingMode: billingTiered, Unit: "GB-month",
		PricingTiers: []*pbc.PricingTier{
			{MinQuantity: 0, MaxQuantity: 50, RatePerUnit: 0.10},
			{MinQuantity: 50, MaxQuantity: 100, RatePerUnit: 0.05},
		},
	}, "p")

	assert.False(t, ok)
	assert.Nil(t, got)
}

type supportsRecorder struct {
	mockCostSourceClient

	mu       sync.Mutex
	requests []*pbc.SupportsRequest
}

func (s *supportsRecorder) Supports(
	_ context.Context, in *pbc.SupportsRequest, _ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, in)
	return &pbc.SupportsResponse{Supported: true}, nil
}

func TestCheckPluginSupportsSendsTheRegionPricingWillSend(t *testing.T) {
	t.Parallel()

	plugin := &supportsRecorder{}
	client := &pluginhost.Client{Name: "azure", API: plugin}
	eng := New([]*pluginhost.Client{client}, nil)
	resource := ResourceDescriptor{
		Type:     "azure:containerservice/kubernetesClusterNodePool:KubernetesClusterNodePool",
		ID:       "urn:pool",
		Provider: "azure",
		Properties: map[string]any{
			"vmSize":                          "Standard_D2s_v3",
			"ref.kubernetesClusterId.urn":     "urn:cluster",
			"ref.kubernetesClusterId.type":    "azure:containerservice/kubernetesCluster:KubernetesCluster",
			"ref.kubernetesClusterId.region":  "westeurope",
			"ref.kubernetesClusterId.sku":     "Free",
			"nested":                          map[string]any{"a": "b"},
			"ref.kubernetesClusterId.unknown": "",
		},
	}

	supported, _ := eng.checkPluginSupports(context.Background(), client, resource, "ProjectedCosts")

	require.True(t, supported)
	require.Len(t, plugin.requests, 1)
	sent := plugin.requests[0].GetResource()
	assert.Equal(t, "westeurope", sent.GetRegion(), "Supports and GetProjectedCost must see the same region")
	assert.Equal(t, "Standard_D2s_v3", sent.GetSku())

	priced := proto.PrepareProjectedDescriptor(
		context.Background(), resource.ID, resource.Provider, resource.Type, ConvertToProto(resource.Properties))
	assert.Equal(t, priced.GetRegion(), sent.GetRegion())
	assert.Equal(t, priced.GetSku(), sent.GetSku())
}

func TestDiscoverPricingSpecIsCachedPerSKU(t *testing.T) {
	t.Parallel()

	plugin := &pricingSpecPlugin{spec: hourlyPricingSpec()}
	eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
	micro := ResourceDescriptor{
		Type: "aws:ec2:Instance", ID: "i", Provider: "aws",
		Properties: map[string]any{"instanceType": "t3.micro", "region": "us-east-1"},
	}
	large := micro
	large.Properties = map[string]any{"instanceType": "m5.large", "region": "us-east-1"}
	otherRegion := micro
	otherRegion.Properties = map[string]any{"instanceType": "t3.micro", "region": "eu-west-1"}

	eng.DiscoverPricingSpec(context.Background(), &micro)
	require.Equal(t, 1, plugin.specCalls)
	eng.DiscoverPricingSpec(context.Background(), &micro)
	assert.Equal(t, 1, plugin.specCalls, "the same SKU and region is served from the cache")

	eng.DiscoverPricingSpec(context.Background(), &large)
	assert.Equal(t, 2, plugin.specCalls, "an edited SKU asks the plugin again")
	eng.DiscoverPricingSpec(context.Background(), &otherRegion)
	assert.Equal(t, 3, plugin.specCalls, "a different region asks the plugin again")
}

func TestConvertToProtoNestedRefIsNotAReference(t *testing.T) {
	t.Parallel()

	tags := ConvertToProto(map[string]any{
		"ref":  map[string]any{"x": map[string]any{"region": "mars", "urn": "urn:fake"}},
		"sku":  map[string]any{"capacity": "2"},
		"name": "web",
	})

	for key := range tags {
		assert.NotContains(t, key, "ref.", "a user input named ref must not look like a resolved reference")
	}
	assert.False(t, proto.HasRefTags(tags))
	assert.Equal(t, "2", tags["sku.capacity"], "other nested inputs still flatten")
}

func TestConvertToProtoSkipsMoreCredentialNames(t *testing.T) {
	t.Parallel()

	tags := ConvertToProto(map[string]any{
		"config": map[string]any{
			"apiKey": "k", "api_key": "k", "accessKey": "a", "access_key": "a",
			"connectionString": "c", "connection_string": "c", "password": "p", "name": "n",
		},
	})

	assert.Equal(t, "n", tags["config.name"])
	for _, key := range []string{
		"config.apiKey", "config.api_key", "config.accessKey", "config.access_key",
		"config.connectionString", "config.connection_string", "config.password",
	} {
		assert.NotContains(t, tags, key)
	}
}
