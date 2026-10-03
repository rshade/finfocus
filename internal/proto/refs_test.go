package proto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestAdapterGetProjectedCostUsesPreparedDescriptor(t *testing.T) {
	t.Parallel()

	properties := map[string]string{
		"location":                 "eastus",
		"servicePlanId":            UnknownPulumiValue,
		"ref.servicePlanId.sku":    "S1",
		"ref.servicePlanId.region": "westeurope",
		"ref.servicePlanId.urn":    "urn:plan",
		"ref.servicePlanId.type":   "azure:appservice/servicePlan:ServicePlan",
	}
	resource := &ResourceDescriptor{
		ID:         "urn:app",
		Type:       "azure:appservice/linuxWebApp:LinuxWebApp",
		Provider:   "azure",
		Properties: properties,
	}

	var got *pbc.ResourceDescriptor
	adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
		getProjectedCostFunc: func(
			_ context.Context, in *pbc.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*pbc.GetProjectedCostResponse, error) {
			got = in.GetResource()
			return &pbc.GetProjectedCostResponse{Currency: "USD", CostPerMonth: 0}, nil
		},
	}}

	_, err := adapter.GetProjectedCost(context.Background(), &GetProjectedCostRequest{
		Resources: []*ResourceDescriptor{resource},
	})
	require.NoError(t, err)
	require.NotNil(t, got)

	want := PrepareProjectedDescriptor(
		context.Background(), resource.ID, resource.Provider, resource.Type, properties,
	)
	assert.Equal(t, want.GetSku(), got.GetSku())
	assert.Equal(t, want.GetRegion(), got.GetRegion())
	assert.Equal(t, want.GetTags(), got.GetTags())
	assert.Empty(t, got.GetSku())
	assert.Equal(t, "eastus", got.GetRegion())
	assert.NotContains(t, got.GetTags(), "servicePlanId")
	assert.Equal(t, "S1", got.GetTags()["ref.servicePlanId.sku"])
}

func TestGetProjectedCostWithErrorsAllowsRefWithoutSKU(t *testing.T) {
	t.Parallel()

	called := false
	client := &mockCostSourceClient{
		getProjectedFunc: func(
			_ context.Context, in *GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*GetProjectedCostResponse, error) {
			called = true
			require.Len(t, in.Resources, 1)
			assert.Equal(t, "S1", in.Resources[0].Properties["ref.servicePlanId.sku"])
			return &GetProjectedCostResponse{
				Results: []*CostResult{{Currency: "USD", MonthlyCost: 0, Notes: "cost is on the plan"}},
			}, nil
		},
	}

	result := GetProjectedCostWithErrors(context.Background(), client, "azure", []*ResourceDescriptor{{
		ID:       "urn:app",
		Type:     "azure:appservice/linuxWebApp:LinuxWebApp",
		Provider: "azure",
		Properties: map[string]string{
			"location":                 "eastus",
			"ref.servicePlanId.sku":    "S1",
			"ref.servicePlanId.region": "westeurope",
			"ref.servicePlanId.urn":    "urn:plan",
		},
	}})

	assert.True(t, called)
	require.Len(t, result.Results, 1)
	assert.NotContains(t, result.Results[0].Notes, "VALIDATION:")
	assert.Empty(t, result.Errors)
}
