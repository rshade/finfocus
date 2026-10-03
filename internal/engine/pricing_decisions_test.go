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

// AWS documents two month lengths: the Pricing Calculator estimates an hourly rate
// over 730 hours (365 x 24 / 12), and its pricing-page billing examples bill a rate
// over a 30-day month, 720 hours ("$0.0225 * 24 hours * 30 days" on the ELB page;
// "30 days * 24 hours = 720 hours" on the CloudWatch page). Core follows each where
// it applies, so $1/hour and $24/day are deliberately not the same monthly price.
func TestPluginSpecFollowsTheAWSMonthConventions(t *testing.T) {
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

	assert.InDelta(t, 730.0, hourly.Monthly, 1e-9, "an hourly rate uses the Pricing Calculator's 730 hours")
	assert.InDelta(t, 720.0, daily.Monthly, 1e-9, "a daily rate uses AWS's 30-day billing month")
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

// finfocus-spec defines tiered pricing as graduated: "Usage in each tier is
// calculated as min(usage, max) - min" (docs/ADVANCED_PATTERNS.md), and its S3
// example reads "First 50 TB at $0.023, next 400 TB at $0.022, over 450 TB at
// $0.021". The rates below are that example, in GB.
func s3TieredSpec() *pbc.PricingSpec {
	return &pbc.PricingSpec{
		BillingMode: billingTiered, Unit: "GB-month", Currency: "USD",
		PricingTiers: []*pbc.PricingTier{
			{MinQuantity: 0, MaxQuantity: 50000, RatePerUnit: 0.023},
			{MinQuantity: 50000, MaxQuantity: 450000, RatePerUnit: 0.022},
			{MinQuantity: 450000, MaxQuantity: 0, RatePerUnit: 0.021},
		},
	}
}

func TestPluginSpecTiersArePricedGraduated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		sizeGB float64
		want   float64
	}{
		{"inside the first tier", 100, 100 * 0.023},
		{"exactly at a boundary", 50000, 50000 * 0.023},
		{"across two tiers", 100000, 50000*0.023 + 50000*0.022},
		{"into the open last tier", 500000, 50000*0.023 + 400000*0.022 + 50000*0.021},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := ResourceDescriptor{
				Type: "aws:s3:Bucket", ID: "b", Provider: "aws", Properties: map[string]any{"sizeGb": tt.sizeGB},
			}
			got, ok := costFromPluginPricingSpec(resource, s3TieredSpec(), "aws-public")
			require.True(t, ok)
			assert.InDelta(t, tt.want, got.Monthly, 1e-6)
		})
	}
}

func TestPluginSpecTiersThatDoNotCoverTheQuantityAreUnusable(t *testing.T) {
	t.Parallel()

	bounded := &pbc.PricingSpec{
		BillingMode: billingTiered, Unit: "GB-month",
		PricingTiers: []*pbc.PricingTier{
			{MinQuantity: 0, MaxQuantity: 50, RatePerUnit: 0.10},
			{MinQuantity: 50, MaxQuantity: 100, RatePerUnit: 0.05},
		},
	}
	gap := &pbc.PricingSpec{
		BillingMode: billingTiered, Unit: "GB-month",
		PricingTiers: []*pbc.PricingTier{
			{MinQuantity: 0, MaxQuantity: 10, RatePerUnit: 0.10},
			{MinQuantity: 20, MaxQuantity: 0, RatePerUnit: 0.05},
		},
	}
	late := &pbc.PricingSpec{
		BillingMode: billingTiered, Unit: "GB-month",
		PricingTiers: []*pbc.PricingTier{{MinQuantity: 10, MaxQuantity: 0, RatePerUnit: 0.05}},
	}

	tests := []struct {
		name   string
		spec   *pbc.PricingSpec
		sizeGB float64
	}{
		{"past the last bounded tier", bounded, 500},
		{"inside a gap", gap, 15},
		{"below the first tier", late, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := ResourceDescriptor{
				Type: "aws:s3:Bucket", ID: "b", Provider: "aws", Properties: map[string]any{"sizeGb": tt.sizeGB},
			}
			got, ok := costFromPluginPricingSpec(resource, tt.spec, "p")
			assert.False(t, ok, "part of the quantity has no rate, so the price would be understated")
			assert.Nil(t, got)
		})
	}

	t.Run("tiers given out of order still price", func(t *testing.T) {
		t.Parallel()
		spec := s3TieredSpec()
		tiers := spec.GetPricingTiers()
		spec.PricingTiers = []*pbc.PricingTier{tiers[2], tiers[0], tiers[1]}
		resource := ResourceDescriptor{
			Type: "aws:s3:Bucket", ID: "b", Provider: "aws", Properties: map[string]any{"sizeGb": 100000},
		}
		got, ok := costFromPluginPricingSpec(resource, spec, "p")
		require.True(t, ok)
		assert.InDelta(t, 50000*0.023+50000*0.022, got.Monthly, 1e-6)
	})
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
