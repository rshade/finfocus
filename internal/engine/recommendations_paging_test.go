package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func recPage(next string, ids ...string) *proto.GetRecommendationsResponse {
	resp := &proto.GetRecommendationsResponse{NextPageToken: next}
	for _, id := range ids {
		resp.Recommendations = append(resp.Recommendations, &proto.Recommendation{ID: id})
	}
	return resp
}

func recIDs(recs []Recommendation) []string {
	ids := make([]string, 0, len(recs))
	for _, rec := range recs {
		ids = append(ids, rec.ID)
	}
	return ids
}

func TestGetRecommendationsForResources_FollowsPageTokensToEnd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pages := map[string]*proto.GetRecommendationsResponse{
		"":   recPage("p2", "rec-1"),
		"p2": recPage("p3", "rec-2"),
		"p3": recPage("", "rec-3"),
	}
	plugin := &recordingRecClient{
		name: "p",
		respond: func(req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			return pages[req.PageToken], nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Equal(t, []string{"rec-1", "rec-2", "rec-3"}, recIDs(result.Recommendations))
	require.Equal(t, 3, plugin.calls())
	tokens := make([]string, 0, 3)
	for _, req := range plugin.reqs {
		tokens = append(tokens, req.PageToken)
		assert.Zero(t, req.PageSize, "page size stays at the plugin default")
	}
	assert.Equal(t, []string{"", "p2", "p3"}, tokens)
}

func TestGetRecommendationsForResources_RepeatedPageTokenFailsPlugin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{
		name: "p",
		respond: func(req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			if req.PageToken == "" {
				return recPage("p2", "rec-1"), nil
			}
			return recPage("p2", "rec-2"), nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Equal(t, "p", result.Errors[0].PluginName)
	assert.Contains(t, result.Errors[0].Error, "repeated page token")
	assert.Empty(t, result.Recommendations, "a failed plugin contributes no rows, including its earlier pages")
	assert.Equal(t, 2, plugin.calls())
}

func TestGetRecommendationsForResources_PageCapFailsPlugin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	issued := 0
	plugin := &recordingRecClient{
		name: "p",
		respond: func(_ *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			issued++
			return recPage(fmt.Sprintf("t%d", issued), "rec"), nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error, "exceed")
	assert.Empty(t, result.Recommendations)
	assert.Equal(t, maxRecommendationPages, plugin.calls())
}

func TestGetRecommendationsForResources_LaterPageErrorDropsPluginRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{
		name: "p",
		respond: func(req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			if req.PageToken == "" {
				return recPage("p2", "rec-1"), nil
			}
			return nil, errors.New("upstream unavailable")
		},
	}
	other := &recordingRecClient{name: "other", recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
		return []*proto.Recommendation{{ID: "rec-other"}}
	}}
	eng := New([]*pluginhost.Client{plugin.client(), other.client()}, nil).
		WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Equal(t, "p", result.Errors[0].PluginName)
	assert.Contains(t, result.Errors[0].Error, "page 2")
	assert.Contains(t, result.Errors[0].Error, "upstream unavailable")
	assert.Equal(t, []string{"rec-other"}, recIDs(result.Recommendations),
		"the failed plugin's page 1 is dropped; the healthy plugin's rows remain")
}

func TestFetchRecommendationPages_StopsWhenContextCanceledBetweenPages(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	plugin := &recordingRecClient{
		name: "p",
		respond: func(_ *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			cancel()
			return recPage("p2", "rec-1"), nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil)

	recs, err := eng.fetchRecommendationPages(ctx, plugin.client(), &proto.GetRecommendationsRequest{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, recs)
	assert.Equal(t, 1, plugin.calls(), "no page is requested after cancellation")
}

func TestGetRecommendationsForResources_BatchedPathFollowsPageTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resources := make([]ResourceDescriptor, 0, batchProcessingThreshold+1)
	for i := range batchProcessingThreshold + 1 {
		resources = append(resources, ec2(fmt.Sprintf("i-%d", i)))
	}
	plugin := &recordingRecClient{
		name: "p",
		respond: func(req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			batchTag := req.TargetResources[0].ID
			if req.PageToken == "" {
				return recPage("p2", batchTag+"-a"), nil
			}
			return recPage("", batchTag+"-b"), nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Equal(t, []string{"i-0-a", "i-0-b", "i-100-a", "i-100-b"}, recIDs(result.Recommendations))
	assert.Equal(t, 4, plugin.calls(), "two batches, each with two pages")
}

func TestGetRecommendationsForResources_BatchFailureKeepsCompletedBatches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resources := make([]ResourceDescriptor, 0, batchProcessingThreshold+1)
	for i := range batchProcessingThreshold + 1 {
		resources = append(resources, ec2(fmt.Sprintf("i-%d", i)))
	}
	plugin := &recordingRecClient{
		name: "p",
		respond: func(req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			if req.TargetResources[0].ID == fmt.Sprintf("i-%d", batchProcessingThreshold) {
				return nil, errors.New("second batch failed")
			}
			return recPage("", "rec-first-batch"), nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Equal(t, "p", result.Errors[0].PluginName)
	assert.Contains(t, result.Errors[0].Error, "batch 1 failed")
	assert.Equal(t, []string{"rec-first-batch"}, recIDs(result.Recommendations),
		"the completed first batch is kept; its rows are not dropped with the failed batch")
	assert.Equal(t, 2, plugin.calls(), "the batch after the failure does not run")
}
