package proto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestClientAdapterCopiesUsageProfile(t *testing.T) {
	t.Parallel()

	var projected *pbc.GetProjectedCostRequest
	var recommendations *pbc.GetRecommendationsRequest
	adapter := &clientAdapter{client: &mockPbcCostSourceServiceClient{
		getProjectedCostFunc: func(
			_ context.Context, in *pbc.GetProjectedCostRequest, _ ...grpc.CallOption,
		) (*pbc.GetProjectedCostResponse, error) {
			projected = in
			return &pbc.GetProjectedCostResponse{}, nil
		},
		getRecommendationsFunc: func(
			_ context.Context, in *pbc.GetRecommendationsRequest, _ ...grpc.CallOption,
		) (*pbc.GetRecommendationsResponse, error) {
			recommendations = in
			return &pbc.GetRecommendationsResponse{}, nil
		},
	}}

	_, err := adapter.GetProjectedCost(context.Background(), &GetProjectedCostRequest{
		Resources:    []*ResourceDescriptor{{ID: "i", Provider: "aws", Type: "aws:ec2/instance:Instance"}},
		UsageProfile: pbc.UsageProfile_USAGE_PROFILE_DEV,
	})
	require.NoError(t, err)
	require.NotNil(t, projected)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_DEV, projected.GetUsageProfile())

	_, err = adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{
		UsageProfile: pbc.UsageProfile_USAGE_PROFILE_BURST,
	})
	require.NoError(t, err)
	require.NotNil(t, recommendations)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_BURST, recommendations.GetUsageProfile())
}
