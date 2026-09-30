package proto

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestClientAdapter_GetRecommendations_RetainsFullRecord(t *testing.T) {
	createdAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	confidence := 0.82
	implCost := 25.0
	effort := 4.5

	mockGRPC := &mockPbcCostSourceServiceClient{
		getRecommendationsFunc: func(
			_ context.Context, _ *pbc.GetRecommendationsRequest, _ ...grpc.CallOption,
		) (*pbc.GetRecommendationsResponse, error) {
			return &pbc.GetRecommendationsResponse{
				Recommendations: []*pbc.Recommendation{{
					Id:              "rec-1",
					Category:        pbc.RecommendationCategory_RECOMMENDATION_CATEGORY_COST,
					ActionType:      pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
					Priority:        pbc.RecommendationPriority_RECOMMENDATION_PRIORITY_HIGH,
					ConfidenceScore: &confidence,
					Description:     "Downsize",
					Source:          "kubecost",
					CreatedAt:       timestamppb.New(createdAt),
					Metadata:        map[string]string{"team": "core"},
					Resource: &pbc.ResourceRecommendationInfo{
						Id:           "i-123",
						Name:         "web",
						Provider:     "aws",
						ResourceType: "aws:ec2/instance:Instance",
						Region:       "us-east-1",
						Sku:          "m5.large",
						Tags:         map[string]string{"env": "prod"},
						Utilization: &pbc.ResourceUtilization{
							CpuPercent:    12.5,
							MemoryPercent: 30,
							CustomMetrics: map[string]float64{"iops": 100},
						},
					},
					Impact: &pbc.RecommendationImpact{
						EstimatedSavings:     40,
						Currency:             "USD",
						ProjectionPeriod:     "monthly",
						CurrentCost:          100,
						ProjectedCost:        60,
						SavingsPercentage:    40,
						ImplementationCost:   &implCost,
						MigrationEffortHours: &effort,
					},
				}},
			}, nil
		},
	}

	adapter := &clientAdapter{client: mockGRPC}
	resp, err := adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Recommendations, 1)

	rec := resp.Recommendations[0]
	assert.Equal(t, "rec-1", rec.ID)
	assert.Equal(t, "RECOMMENDATION_PRIORITY_HIGH", rec.Priority)
	require.NotNil(t, rec.ConfidenceScore)
	assert.InDelta(t, 0.82, *rec.ConfidenceScore, 1e-9)
	require.NotNil(t, rec.CreatedAt)
	assert.True(t, createdAt.Equal(*rec.CreatedAt))
	assert.Equal(t, "i-123", rec.ResourceID)

	require.NotNil(t, rec.Resource)
	assert.Equal(t, "web", rec.Resource.Name)
	assert.Equal(t, "aws", rec.Resource.Provider)
	assert.Equal(t, "aws:ec2/instance:Instance", rec.Resource.ResourceType)
	assert.Equal(t, "us-east-1", rec.Resource.Region)
	assert.Equal(t, "m5.large", rec.Resource.SKU)
	assert.Equal(t, map[string]string{"env": "prod"}, rec.Resource.Tags)
	require.NotNil(t, rec.Resource.Utilization)
	assert.InDelta(t, 12.5, rec.Resource.Utilization.CPUPercent, 1e-9)
	assert.InDelta(t, 30.0, rec.Resource.Utilization.MemoryPercent, 1e-9)
	assert.Equal(t, map[string]float64{"iops": 100}, rec.Resource.Utilization.CustomMetrics)

	require.NotNil(t, rec.Impact)
	assert.Equal(t, "monthly", rec.Impact.ProjectionPeriod)
	require.NotNil(t, rec.Impact.ImplementationCost)
	assert.InDelta(t, 25.0, *rec.Impact.ImplementationCost, 1e-9)
	require.NotNil(t, rec.Impact.MigrationEffortHours)
	assert.InDelta(t, 4.5, *rec.Impact.MigrationEffortHours, 1e-9)
}

func TestClientAdapter_GetRecommendations_SparseRecord(t *testing.T) {
	mockGRPC := &mockPbcCostSourceServiceClient{
		getRecommendationsFunc: func(
			_ context.Context, _ *pbc.GetRecommendationsRequest, _ ...grpc.CallOption,
		) (*pbc.GetRecommendationsResponse, error) {
			return &pbc.GetRecommendationsResponse{
				Recommendations: []*pbc.Recommendation{{Id: "rec-2", Description: "x"}},
			}, nil
		},
	}

	adapter := &clientAdapter{client: mockGRPC}
	resp, err := adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Recommendations, 1)

	rec := resp.Recommendations[0]
	assert.Nil(t, rec.ConfidenceScore)
	assert.Nil(t, rec.CreatedAt)
	assert.Nil(t, rec.Resource)
	assert.Nil(t, rec.Impact)
}
