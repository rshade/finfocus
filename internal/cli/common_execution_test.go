package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// mockRecommendationFetcher implements recommendationFetcher for testing.
type mockRecommendationFetcher struct {
	result *engine.RecommendationsResult
	err    error
}

func (m *mockRecommendationFetcher) GetRecommendationsForResources(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	return m.result, m.err
}

func TestFetchAndMergeRecommendations(t *testing.T) {
	tests := []struct {
		name        string
		resources   []engine.ResourceDescriptor
		results     []engine.CostResult
		fetcher     *mockRecommendationFetcher
		wantRecs    map[int][]string // result index -> expected rec types
		wantNilRecs []int            // result indices expected to have nil Recommendations
	}{
		{
			name: "successful merge by ResourceID",
			resources: []engine.ResourceDescriptor{
				{ID: "res-1", Type: "aws:ec2:Instance"},
				{ID: "res-2", Type: "aws:rds:Instance"},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
				{ResourceID: "res-2", Monthly: 50.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:       "res-1",
							Type:             "RIGHTSIZE",
							Description:      "Switch to t3.small",
							EstimatedSavings: 5.0,
							Currency:         "USD",
						},
						{
							ResourceID:  "res-2",
							Type:        "TERMINATE",
							Description: "Idle resource",
						},
					},
				},
			},
			wantRecs: map[int][]string{
				0: {"RIGHTSIZE"},
				1: {"TERMINATE"},
			},
		},
		{
			name: "empty recommendations no-op",
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: nil,
				},
			},
			wantNilRecs: []int{0},
		},
		{
			name: "nil result from fetcher no-op",
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: nil,
			},
			wantNilRecs: []int{0},
		},
		{
			name: "fetch error logs warning and returns gracefully",
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				err: errors.New("plugin unavailable"),
			},
			wantNilRecs: []int{0},
		},
		{
			name: "recommendation with empty ResourceID is skipped",
			results: []engine.CostResult{
				{ResourceID: "", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "",
							Type:        "RIGHTSIZE",
							Description: "Should be skipped",
						},
					},
				},
			},
			wantNilRecs: []int{0},
		},
		{
			name: "mixed empty and valid ResourceID recommendations only merge valid",
			resources: []engine.ResourceDescriptor{
				{ID: "res-1", Type: "aws:ec2:Instance"},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
				{ResourceID: "", Monthly: 1.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "",
							Type:        "TERMINATE",
							Description: "Skipped",
						},
						{
							ResourceID:  "res-1",
							Type:        "RIGHTSIZE",
							Description: "Resize",
						},
					},
				},
			},
			wantRecs:    map[int][]string{0: {"RIGHTSIZE"}},
			wantNilRecs: []int{1},
		},
		{
			name: "multiple resources with partial recommendation coverage",
			resources: []engine.ResourceDescriptor{
				{ID: "res-1", Type: "aws:ec2:Instance"},
				{ID: "res-2", Type: "aws:rds:Instance"},
				{ID: "res-3", Type: "aws:s3:Bucket"},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
				{ResourceID: "res-2", Monthly: 50.0},
				{ResourceID: "res-3", Monthly: 5.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "res-1",
							Type:        "RIGHTSIZE",
							Description: "Resize",
						},
						{
							ResourceID:       "res-1",
							Type:             "MIGRATE",
							Description:      "Use Graviton",
							EstimatedSavings: 8.0,
							Currency:         "USD",
						},
					},
				},
			},
			wantRecs:    map[int][]string{0: {"RIGHTSIZE", "MIGRATE"}},
			wantNilRecs: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchAndMergeRecommendations(context.Background(), tt.fetcher, tt.resources, tt.results)

			for idx, wantTypes := range tt.wantRecs {
				require.Len(t, tt.results[idx].Recommendations, len(wantTypes),
					"result[%d] recommendation count mismatch", idx)
				for j, wantType := range wantTypes {
					assert.Equal(t, wantType, tt.results[idx].Recommendations[j].Type,
						"result[%d].Recommendations[%d].Type", idx, j)
				}
			}
			for _, idx := range tt.wantNilRecs {
				assert.Nil(t, tt.results[idx].Recommendations,
					"result[%d].Recommendations should be nil", idx)
			}
		})
	}
}
