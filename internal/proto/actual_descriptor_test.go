package proto

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const descriptorTestURN = "urn:pulumi:dev::project::aws:ec2/instance:Instance::web"

func capturingActualClient(captured *[]*pbc.GetActualCostRequest) *clientAdapter {
	return &clientAdapter{client: &mockPbcCostSourceServiceClient{
		getActualCostFunc: func(
			_ context.Context, in *pbc.GetActualCostRequest, _ ...grpc.CallOption,
		) (*pbc.GetActualCostResponse, error) {
			*captured = append(*captured, in)
			return &pbc.GetActualCostResponse{
				Results: []*pbc.ActualCostResult{{Cost: 12.5, Source: "mock"}},
			}, nil
		},
	}}
}

func actualRequestWithResource(resource *ResourceDescriptor) *GetActualCostRequest {
	return &GetActualCostRequest{
		ResourceIDs:  []string{descriptorTestURN},
		StartTime:    time.Now().Add(-24 * time.Hour).Unix(),
		EndTime:      time.Now().Unix(),
		Provider:     "aws",
		ResourceType: "aws:ec2/instance:Instance",
		Properties: map[string]any{
			"pulumi:cloudId":   "i-0abc123def456",
			"instanceType":     "t3.medium",
			"availabilityZone": "us-west-2a",
			"tags":             map[string]any{"region": "prod-eu"},
		},
		Resource: resource,
	}
}

func ec2Resource(t *testing.T) *ResourceDescriptor {
	t.Helper()
	attrs, err := structpb.NewStruct(map[string]any{"instanceType": "t3.medium", "count": 3})
	require.NoError(t, err)
	return &ResourceDescriptor{
		ID:       descriptorTestURN,
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		Properties: map[string]string{
			"instanceType":     "t3.medium",
			"availabilityZone": "us-west-2a",
			"count":            "3",
		},
		Attributes: attrs,
	}
}

func TestClientAdapter_GetActualCost_SendsResourceDescriptor(t *testing.T) {
	t.Parallel()

	var captured []*pbc.GetActualCostRequest
	resource := ec2Resource(t)
	resp, err := capturingActualClient(&captured).GetActualCost(
		context.Background(), actualRequestWithResource(resource))
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.Len(t, captured, 1)

	got := captured[0].GetResource()
	require.NotNil(t, got)
	assert.Equal(t, descriptorTestURN, got.GetId())
	assert.Equal(t, "aws", got.GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", got.GetResourceType())
	assert.Equal(t, "t3.medium", got.GetSku())
	assert.Equal(t, "us-west-2", got.GetRegion())
	assert.Equal(t, "3", got.GetTags()["count"])
	assert.True(t, protobuf.Equal(resource.Attributes, got.GetAttributes()))

	// tags keep their pre-v0.7.4 shape so older plugins are unaffected:
	// the cloud tag wins over the injected region.
	assert.Equal(t, "prod-eu", captured[0].GetTags()["region"])
	assert.Equal(t, "t3.medium", captured[0].GetTags()["sku"])
}

func TestClientAdapter_GetActualCost_OmitsResourceDescriptor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(t *testing.T, req *GetActualCostRequest)
	}{
		{
			name:   "no resource from the engine",
			mutate: func(_ *testing.T, req *GetActualCostRequest) { req.Resource = nil },
		},
		{
			name: "no provider",
			mutate: func(t *testing.T, req *GetActualCostRequest) {
				req.Resource = ec2Resource(t)
				req.Resource.Provider = ""
			},
		},
		{
			name:   "no resource type",
			mutate: func(t *testing.T, req *GetActualCostRequest) { req.Resource = ec2Resource(t); req.Resource.Type = "" },
		},
		{
			name: "descriptor over the SDK limits",
			mutate: func(t *testing.T, req *GetActualCostRequest) {
				req.Resource = ec2Resource(t)
				req.Resource.Properties[strings.Repeat("k", 200)] = "v"
			},
		},
		{
			name: "more than one resource id",
			mutate: func(t *testing.T, req *GetActualCostRequest) {
				req.Resource = ec2Resource(t)
				req.Properties = nil
				req.ResourceIDs = []string{"i-1", "i-2"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := actualRequestWithResource(nil)
			tt.mutate(t, req)

			var captured []*pbc.GetActualCostRequest
			_, err := capturingActualClient(&captured).GetActualCost(context.Background(), req)
			require.NoError(t, err)
			require.NotEmpty(t, captured, "the request is still sent without a descriptor")
			for _, sent := range captured {
				assert.Nil(t, sent.GetResource())
			}
		})
	}
}

func TestGetActualCostWithErrors_SendsResourceDescriptor(t *testing.T) {
	t.Parallel()

	var captured []*pbc.GetActualCostRequest
	result := GetActualCostWithErrors(
		context.Background(), capturingActualClient(&captured), "mock", actualRequestWithResource(ec2Resource(t)))
	require.Empty(t, result.Errors)
	require.Len(t, result.Results, 1)
	require.Len(t, captured, 1)
	assert.Equal(t, "t3.medium", captured[0].GetResource().GetSku())
}

func TestGetActualCostWithErrors_OversizeDescriptorIsNotAValidationError(t *testing.T) {
	t.Parallel()

	req := actualRequestWithResource(ec2Resource(t))
	req.Resource.Properties[strings.Repeat("k", 200)] = "v"

	var captured []*pbc.GetActualCostRequest
	result := GetActualCostWithErrors(context.Background(), capturingActualClient(&captured), "mock", req)
	require.Empty(t, result.Errors)
	require.Len(t, captured, 1)
	assert.Nil(t, captured[0].GetResource())
}
