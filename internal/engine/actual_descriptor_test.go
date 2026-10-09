package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func scaleSetResource() ResourceDescriptor {
	return ResourceDescriptor{
		ID:       "urn:pulumi:dev::app::azure-native:compute:VirtualMachineScaleSet::vmss",
		Type:     "azure-native:compute:VirtualMachineScaleSet",
		Provider: "azure",
		Properties: map[string]any{
			"location":      "eastus",
			"sku":           map[string]any{"name": "Standard_D2s_v3", "capacity": float64(3)},
			"adminPassword": "hunter2",
		},
	}
}

func TestGetActualCostFromPlugin_SendsResource(t *testing.T) {
	t.Parallel()

	var captured *proto.GetActualCostRequest
	mockAPI := &mockBatchCostSourceClient{
		getActualCostFunc: func(
			_ context.Context, in *proto.GetActualCostRequest, _ ...grpc.CallOption,
		) (*proto.GetActualCostResponse, error) {
			captured = in
			return &proto.GetActualCostResponse{
				Results: []*proto.ActualCostResult{{Currency: "USD", TotalCost: 30}},
			}, nil
		},
	}
	client := makeBatchCapableClient("list-price", mockAPI)
	eng := New([]*pluginhost.Client{client}, nil)

	resource := scaleSetResource()
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := eng.getActualCostFromPlugin(context.Background(), client, resource, from, from.AddDate(0, 0, 30))
	require.NoError(t, err)
	require.NotNil(t, captured)
	require.NotNil(t, captured.Resource)

	assert.Equal(t, resource.ID, captured.Resource.ID)
	assert.Equal(t, resource.Type, captured.Resource.Type)
	assert.Equal(t, "azure", captured.Resource.Provider)
	assert.Equal(t, "3", captured.Resource.Properties["sku.capacity"])
	assert.NotContains(t, captured.Resource.Properties, "adminPassword")

	attrs := captured.Resource.Attributes.AsMap()
	assert.InDelta(t, 3.0, attrs["sku"].(map[string]any)["capacity"], 1e-9)
	assert.NotContains(t, attrs, "adminPassword")
}

func TestGenerateActualCostCacheKey_Descriptor(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	request := func(capacity float64) ActualCostRequest {
		r := scaleSetResource()
		r.Properties["sku"] = map[string]any{"name": "Standard_D2s_v3", "capacity": capacity}
		return ActualCostRequest{Resources: []ResourceDescriptor{r}, From: from, To: from.AddDate(0, 0, 30)}
	}

	assert.NotEqual(t, generateActualCostCacheKey(request(3)), generateActualCostCacheKey(request(5)),
		"a plugin can price from attributes, so a changed declared input must miss the cache")

	bare := ActualCostRequest{
		Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"}},
		From:      from,
		To:        from.AddDate(0, 0, 30),
	}
	redactedOnly := bare
	redactedOnly.Resources = []ResourceDescriptor{{
		Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws",
		Properties: map[string]any{"password": "hunter2"},
	}}
	assert.Equal(t, generateActualCostCacheKey(bare), generateActualCostCacheKey(redactedOnly),
		"a resource whose descriptor carries no tags or attributes keeps its existing key")

	withRef := func(urn string) ActualCostRequest {
		r := request(3)
		r.Resources[0].Properties["ref.subnetId.urn"] = urn
		return r
	}
	assert.NotEqual(t,
		generateActualCostCacheKey(withRef("urn:pulumi:dev::app::aws:ec2/subnet:Subnet::a")),
		generateActualCostCacheKey(withRef("urn:pulumi:dev::app::aws:ec2/subnet:Subnet::b")),
		"ref.* values reach the descriptor tags, so a changed reference must miss the cache")

	untyped := request(3)
	untyped.Resources[0].Provider = ""
	assert.Equal(t, generateActualCostCacheKey(untyped), generateActualCostCacheKey(func() ActualCostRequest {
		r := request(5)
		r.Resources[0].Provider = ""
		return r
	}()), "no descriptor is sent without a provider, so the key ignores its properties")
}

func TestActualCostFromPluginPreservesSustainability(t *testing.T) {
	t.Parallel()
	input := &proto.ActualCostResult{
		Currency:       "USD",
		TotalCost:      12.3456,
		Sustainability: map[string]proto.SustainabilityMetric{"carbon_footprint": {Value: 12.23456, Unit: "kgCO2e"}},
	}
	client := makeBatchCapableClient(
		"carbon",
		&mockBatchCostSourceClient{
			getActualCostFunc: func(context.Context, *proto.GetActualCostRequest, ...grpc.CallOption) (*proto.GetActualCostResponse, error) {
				return &proto.GetActualCostResponse{Results: []*proto.ActualCostResult{input}}, nil
			},
		},
	)
	eng := New([]*pluginhost.Client{client}, nil)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	resource := ResourceDescriptor{Type: "aws:ec2:Instance", ID: "vm", Provider: "aws"}
	actual, err := eng.getActualCostFromPlugin(context.Background(), client, resource, start, start.AddDate(0, 0, 6))
	require.NoError(t, err)
	require.NotEmpty(t, actual.Sustainability)
	assert.Equal(t, SustainabilityMetric{Value: 12.23456, Unit: "kgCO2e"}, actual.Sustainability["carbon_footprint"])
	batch := mapProtoActualCostResultToEngine(resource, "carbon", input, start, start.AddDate(0, 0, 6))
	assert.Equal(t, batch, actual, "single and batch must preserve the same canonical fields")
	actual.Sustainability["carbon_footprint"] = SustainabilityMetric{}
	assert.InDelta(t, 12.23456, input.Sustainability["carbon_footprint"].Value, 0.000001)
}
