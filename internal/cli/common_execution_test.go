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
	t.Run("successful merge by ResourceID", func(t *testing.T) {
		resources := []engine.ResourceDescriptor{
			{ID: "res-1", Type: "aws:ec2:Instance"},
			{ID: "res-2", Type: "aws:rds:Instance"},
		}
		results := []engine.CostResult{
			{ResourceID: "res-1", Monthly: 10.0},
			{ResourceID: "res-2", Monthly: 50.0},
		}
		fetcher := &mockRecommendationFetcher{
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
		}

		fetchAndMergeRecommendations(context.Background(), fetcher, resources, results)

		require.Len(t, results[0].Recommendations, 1)
		assert.Equal(t, "RIGHTSIZE", results[0].Recommendations[0].Type)
		assert.Equal(t, 5.0, results[0].Recommendations[0].EstimatedSavings)
		require.Len(t, results[1].Recommendations, 1)
		assert.Equal(t, "TERMINATE", results[1].Recommendations[0].Type)
	})

	t.Run("empty recommendations no-op", func(t *testing.T) {
		results := []engine.CostResult{
			{ResourceID: "res-1", Monthly: 10.0},
		}
		fetcher := &mockRecommendationFetcher{
			result: &engine.RecommendationsResult{
				Recommendations: nil,
			},
		}

		fetchAndMergeRecommendations(context.Background(), fetcher, nil, results)

		assert.Nil(t, results[0].Recommendations)
	})

	t.Run("fetch error logs warning and returns gracefully", func(t *testing.T) {
		results := []engine.CostResult{
			{ResourceID: "res-1", Monthly: 10.0},
		}
		fetcher := &mockRecommendationFetcher{
			err: errors.New("plugin unavailable"),
		}

		// Should not panic or modify results
		fetchAndMergeRecommendations(context.Background(), fetcher, nil, results)

		assert.Nil(t, results[0].Recommendations)
	})

	t.Run("recommendation with empty ResourceID is skipped", func(t *testing.T) {
		results := []engine.CostResult{
			{ResourceID: "", Monthly: 10.0},
		}
		fetcher := &mockRecommendationFetcher{
			result: &engine.RecommendationsResult{
				Recommendations: []engine.Recommendation{
					{
						ResourceID:  "",
						Type:        "RIGHTSIZE",
						Description: "Should be skipped",
					},
				},
			},
		}

		fetchAndMergeRecommendations(context.Background(), fetcher, nil, results)

		assert.Nil(t, results[0].Recommendations)
	})

	t.Run("mixed empty and valid ResourceID recommendations only merge valid", func(t *testing.T) {
		resources := []engine.ResourceDescriptor{
			{ID: "res-1", Type: "aws:ec2:Instance"},
		}
		results := []engine.CostResult{
			{ResourceID: "res-1", Monthly: 10.0},
			{ResourceID: "", Monthly: 1.0},
		}
		fetcher := &mockRecommendationFetcher{
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
		}

		fetchAndMergeRecommendations(context.Background(), fetcher, resources, results)

		require.Len(t, results[0].Recommendations, 1)
		assert.Equal(t, "RIGHTSIZE", results[0].Recommendations[0].Type)
		assert.Nil(t, results[1].Recommendations)
	})

	t.Run("multiple resources with partial recommendation coverage", func(t *testing.T) {
		resources := []engine.ResourceDescriptor{
			{ID: "res-1", Type: "aws:ec2:Instance"},
			{ID: "res-2", Type: "aws:rds:Instance"},
			{ID: "res-3", Type: "aws:s3:Bucket"},
		}
		results := []engine.CostResult{
			{ResourceID: "res-1", Monthly: 10.0},
			{ResourceID: "res-2", Monthly: 50.0},
			{ResourceID: "res-3", Monthly: 5.0},
		}
		fetcher := &mockRecommendationFetcher{
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
		}

		fetchAndMergeRecommendations(context.Background(), fetcher, resources, results)

		// res-1 has two recommendations
		require.Len(t, results[0].Recommendations, 2)
		assert.Equal(t, "RIGHTSIZE", results[0].Recommendations[0].Type)
		assert.Equal(t, "MIGRATE", results[0].Recommendations[1].Type)
		// res-2 and res-3 have none
		assert.Nil(t, results[1].Recommendations)
		assert.Nil(t, results[2].Recommendations)
	})
}
