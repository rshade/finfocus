package proto

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestClientAdapter_GetRecommendations_SpotFields(t *testing.T) {
	t.Parallel()

	dynamic := pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_DYNAMIC
	tests := []struct {
		name         string
		category     pbc.FocusPricingCategory
		risk         float64
		wantCategory string
		wantRisk     float64
	}{
		{"dynamic with risk", dynamic, 0.12, "DYNAMIC", 0.12},
		{"dynamic unknown risk", dynamic, 0, "DYNAMIC", 0},
		{"dynamic risk above one is dropped", dynamic, 1.5, "DYNAMIC", 0},
		{"dynamic NaN risk is dropped", dynamic, math.NaN(), "DYNAMIC", 0},
		{"standard drops risk", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_STANDARD, 0.3, "STANDARD", 0},
		{"unspecified", pbc.FocusPricingCategory_FOCUS_PRICING_CATEGORY_UNSPECIFIED, 0, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockGRPC := &mockPbcCostSourceServiceClient{
				getRecommendationsFunc: func(
					_ context.Context, _ *pbc.GetRecommendationsRequest, _ ...grpc.CallOption,
				) (*pbc.GetRecommendationsResponse, error) {
					return &pbc.GetRecommendationsResponse{Recommendations: []*pbc.Recommendation{{
						Id:                        "rec-spot",
						PricingCategory:           tt.category,
						SpotInterruptionRiskScore: tt.risk,
					}}}, nil
				},
			}
			adapter := &clientAdapter{client: mockGRPC}
			resp, err := adapter.GetRecommendations(context.Background(), &GetRecommendationsRequest{})
			require.NoError(t, err)
			require.Len(t, resp.Recommendations, 1)
			assert.Equal(t, tt.wantCategory, resp.Recommendations[0].PricingCategory)
			assert.InDelta(t, tt.wantRisk, resp.Recommendations[0].SpotInterruptionRisk, 1e-12)
		})
	}
}
