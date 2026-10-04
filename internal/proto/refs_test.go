package proto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

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
		context.Background(), resource.ID, resource.Provider, resource.Type, properties, nil,
	)
	assert.Equal(t, want.GetSku(), got.GetSku())
	assert.Equal(t, want.GetRegion(), got.GetRegion())
	assert.Equal(t, want.GetTags(), got.GetTags())
	assert.Empty(t, got.GetSku())
	assert.Equal(t, "eastus", got.GetRegion())
	assert.NotContains(t, got.GetTags(), "servicePlanId")
	assert.Equal(t, "S1", got.GetTags()["ref.servicePlanId.sku"])
}

func TestPrepareProjectedDescriptorCopiesAttributes(t *testing.T) {
	t.Parallel()

	properties := map[string]string{"instanceType": "m5.large", "region": "us-east-1"}
	attrs, err := structpb.NewStruct(map[string]any{
		"spec": map[string]any{"replicas": float64(3)},
	})
	require.NoError(t, err)

	without := PrepareProjectedDescriptor(
		context.Background(), "urn:a", "aws", "aws:ec2/instance:Instance", properties, nil,
	)
	with := PrepareProjectedDescriptor(
		context.Background(), "urn:a", "aws", "aws:ec2/instance:Instance", properties, attrs,
	)

	assert.Nil(t, without.GetAttributes())
	assert.Same(t, attrs, with.GetAttributes())
	assert.Equal(t, without.GetTags(), with.GetTags())
	assert.Equal(t, without.GetSku(), with.GetSku())
	assert.Equal(t, without.GetRegion(), with.GetRegion())
}

func TestAdapterGetProjectedCostSendsAttributes(t *testing.T) {
	t.Parallel()

	attrs, err := structpb.NewStruct(map[string]any{
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{}}},
	})
	require.NoError(t, err)

	var got *pbc.ResourceDescriptor
	adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
		getProjectedCostFunc: func(
			_ context.Context, in *pbc.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*pbc.GetProjectedCostResponse, error) {
			got = in.GetResource()
			return &pbc.GetProjectedCostResponse{Currency: "USD"}, nil
		},
	}}

	_, err = adapter.GetProjectedCost(context.Background(), &GetProjectedCostRequest{
		Resources: []*ResourceDescriptor{{
			ID:         "urn:deploy",
			Type:       "kubernetes:apps/v1:Deployment",
			Provider:   "kubernetes",
			Properties: map[string]string{"kind": "Deployment"},
			Attributes: attrs,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Same(t, attrs, got.GetAttributes())
	assert.Equal(t, "Deployment", got.GetTags()["kind"])
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
