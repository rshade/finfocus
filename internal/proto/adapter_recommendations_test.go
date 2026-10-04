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
	t.Parallel()

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
					PrimaryReason: pbc.RecommendationReason_RECOMMENDATION_REASON_OVER_PROVISIONED,
					SecondaryReasons: []pbc.RecommendationReason{
						pbc.RecommendationReason_RECOMMENDATION_REASON_IDLE,
						pbc.RecommendationReason_RECOMMENDATION_REASON_UNSPECIFIED,
					},
					ActionDetail: &pbc.Recommendation_Kubernetes{Kubernetes: &pbc.KubernetesAction{
						ClusterId:           "cluster-1",
						Namespace:           "default",
						ControllerKind:      "Deployment",
						ControllerName:      "web",
						ContainerName:       "app",
						CurrentRequests:     &pbc.KubernetesResources{Cpu: "500m", Memory: "256Mi"},
						RecommendedRequests: &pbc.KubernetesResources{Cpu: "250m", Memory: "128Mi"},
						Algorithm:           "vpa",
					}},
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

	assert.Equal(t, "RECOMMENDATION_REASON_OVER_PROVISIONED", rec.PrimaryReason)
	assert.Equal(t, []string{"RECOMMENDATION_REASON_IDLE"}, rec.SecondaryReasons,
		"unspecified secondary reasons are dropped")

	require.NotNil(t, rec.ActionDetail)
	k8s := rec.ActionDetail.Kubernetes
	require.NotNil(t, k8s)
	assert.Equal(t, "cluster-1", k8s.ClusterID)
	assert.Equal(t, "default", k8s.Namespace)
	assert.Equal(t, "Deployment", k8s.ControllerKind)
	assert.Equal(t, "web", k8s.ControllerName)
	assert.Equal(t, "app", k8s.ContainerName)
	require.NotNil(t, k8s.CurrentRequests)
	assert.Equal(t, "500m", k8s.CurrentRequests.CPU)
	assert.Equal(t, "256Mi", k8s.CurrentRequests.Memory)
	require.NotNil(t, k8s.RecommendedRequests)
	assert.Equal(t, "250m", k8s.RecommendedRequests.CPU)
	assert.Nil(t, k8s.CurrentLimits)
	assert.Nil(t, k8s.RecommendedLimits)
	assert.Equal(t, "vpa", k8s.Algorithm)
}

func TestClientAdapter_GetRecommendations_SparseRecord(t *testing.T) {
	t.Parallel()

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
	assert.Nil(t, rec.ActionDetail)
	assert.Empty(t, rec.PrimaryReason)
	assert.Empty(t, rec.SecondaryReasons)
}

// TestClientAdapter_GetRecommendations_ActionDetailVariants covers every action_detail
// oneof variant plus the absent case.
func TestClientAdapter_GetRecommendations_ActionDetailVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  *pbc.Recommendation
		want func(t *testing.T, got *RecommendationActionDetail)
	}{
		{
			name: "rightsize",
			rec: &pbc.Recommendation{
				Id: "rec-rs",
				ActionDetail: &pbc.Recommendation_Rightsize{Rightsize: &pbc.RightsizeAction{
					CurrentSku:              "m5.large",
					RecommendedSku:          "m5.xlarge",
					CurrentInstanceType:     "m5.large",
					RecommendedInstanceType: "m5.xlarge",
					ProjectedUtilization:    &pbc.ResourceUtilization{CpuPercent: 55, MemoryPercent: 65},
				}},
			},
			want: func(t *testing.T, got *RecommendationActionDetail) {
				t.Helper()
				rs := got.Rightsize
				require.NotNil(t, rs)
				assert.Equal(t, "m5.large", rs.CurrentSKU)
				assert.Equal(t, "m5.xlarge", rs.RecommendedSKU)
				assert.Equal(t, "m5.large", rs.CurrentInstanceType)
				assert.Equal(t, "m5.xlarge", rs.RecommendedInstanceType)
				require.NotNil(t, rs.ProjectedUtilization)
				assert.InDelta(t, 55.0, rs.ProjectedUtilization.CPUPercent, 1e-9)
				assert.InDelta(t, 65.0, rs.ProjectedUtilization.MemoryPercent, 1e-9)
			},
		},
		{
			name: "terminate",
			rec: &pbc.Recommendation{
				Id: "rec-t",
				ActionDetail: &pbc.Recommendation_Terminate{Terminate: &pbc.TerminateAction{
					TerminationReason: "idle",
					IdleDays:          45,
				}},
			},
			want: func(t *testing.T, got *RecommendationActionDetail) {
				t.Helper()
				term := got.Terminate
				require.NotNil(t, term)
				assert.Equal(t, "idle", term.TerminationReason)
				assert.Equal(t, int32(45), term.IdleDays)
			},
		},
		{
			name: "commitment",
			rec: &pbc.Recommendation{
				Id: "rec-c",
				ActionDetail: &pbc.Recommendation_Commitment{Commitment: &pbc.CommitmentAction{
					CommitmentType:      "savings_plan",
					Term:                "1_year",
					PaymentOption:       "no_upfront",
					RecommendedQuantity: 2,
					Scope:               "region",
				}},
			},
			want: func(t *testing.T, got *RecommendationActionDetail) {
				t.Helper()
				c := got.Commitment
				require.NotNil(t, c)
				assert.Equal(t, "savings_plan", c.CommitmentType)
				assert.Equal(t, "1_year", c.Term)
				assert.Equal(t, "no_upfront", c.PaymentOption)
				assert.InDelta(t, 2.0, c.RecommendedQuantity, 1e-9)
				assert.Equal(t, "region", c.Scope)
			},
		},
		{
			name: "kubernetes with limits",
			rec: &pbc.Recommendation{
				Id: "rec-k",
				ActionDetail: &pbc.Recommendation_Kubernetes{Kubernetes: &pbc.KubernetesAction{
					ClusterId:         "cluster-2",
					CurrentLimits:     &pbc.KubernetesResources{Cpu: "1", Memory: "1Gi"},
					RecommendedLimits: &pbc.KubernetesResources{Cpu: "500m", Memory: "512Mi"},
				}},
			},
			want: func(t *testing.T, got *RecommendationActionDetail) {
				t.Helper()
				k := got.Kubernetes
				require.NotNil(t, k)
				assert.Equal(t, "cluster-2", k.ClusterID)
				require.NotNil(t, k.CurrentLimits)
				assert.Equal(t, "1", k.CurrentLimits.CPU)
				require.NotNil(t, k.RecommendedLimits)
				assert.Equal(t, "512Mi", k.RecommendedLimits.Memory)
				assert.Nil(t, k.CurrentRequests)
			},
		},
		{
			name: "modify",
			rec: &pbc.Recommendation{
				Id: "rec-m",
				ActionDetail: &pbc.Recommendation_Modify{Modify: &pbc.ModifyAction{
					ModificationType:  "storage_class",
					CurrentConfig:     map[string]string{"class": "gp2"},
					RecommendedConfig: map[string]string{"class": "gp3"},
				}},
			},
			want: func(t *testing.T, got *RecommendationActionDetail) {
				t.Helper()
				m := got.Modify
				require.NotNil(t, m)
				assert.Equal(t, "storage_class", m.ModificationType)
				assert.Equal(t, map[string]string{"class": "gp2"}, m.CurrentConfig)
				assert.Equal(t, map[string]string{"class": "gp3"}, m.RecommendedConfig)
			},
		},
		{
			name: "absent",
			rec:  &pbc.Recommendation{Id: "rec-none"},
			want: func(t *testing.T, got *RecommendationActionDetail) {
				t.Helper()
				assert.Nil(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockGRPC := &mockPbcCostSourceServiceClient{
				getRecommendationsFunc: func(
					_ context.Context, _ *pbc.GetRecommendationsRequest, _ ...grpc.CallOption,
				) (*pbc.GetRecommendationsResponse, error) {
					return &pbc.GetRecommendationsResponse{Recommendations: []*pbc.Recommendation{tt.rec}}, nil
				},
			}

			adapter := &clientAdapter{client: mockGRPC}
			resp, err := adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{})
			require.NoError(t, err)
			require.Len(t, resp.Recommendations, 1)
			tt.want(t, resp.Recommendations[0].ActionDetail)
		})
	}
}

func TestClientAdapter_GetRecommendations_IncludeDismissed(t *testing.T) {
	t.Parallel()

	var got []*pbc.GetRecommendationsRequest
	mockGRPC := &mockPbcCostSourceServiceClient{
		getRecommendationsFunc: func(
			_ context.Context, in *pbc.GetRecommendationsRequest, _ ...grpc.CallOption,
		) (*pbc.GetRecommendationsResponse, error) {
			got = append(got, in)
			return &pbc.GetRecommendationsResponse{}, nil
		},
	}
	adapter := &clientAdapter{client: mockGRPC}

	_, err := adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{
		IncludeDismissed:          true,
		ExcludedRecommendationIDs: []string{"rec-1"},
		ProjectionPeriod:          "monthly",
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].GetIncludeDismissed())
	assert.Equal(t, []string{"rec-1"}, got[0].GetExcludedRecommendationIds())

	_, err = adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{
		ExcludedRecommendationIDs: []string{"rec-1"},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.False(t, got[1].GetIncludeDismissed())
	assert.Equal(t, []string{"rec-1"}, got[1].GetExcludedRecommendationIds())
}
