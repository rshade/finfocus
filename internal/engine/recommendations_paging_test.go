package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

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
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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
		respond: func(_ context.Context, _ *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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
		respond: func(_ context.Context, _ *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
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

func shortenRecommendationPageTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	prev := recommendationPageTimeout
	recommendationPageTimeout = timeout
	t.Cleanup(func() { recommendationPageTimeout = prev })
}

func threePageResponder(
	_ context.Context,
	req *proto.GetRecommendationsRequest,
) (*proto.GetRecommendationsResponse, error) {
	pages := map[string]*proto.GetRecommendationsResponse{
		"":   recPage("p2", "rec-1"),
		"p2": recPage("p3", "rec-2"),
		"p3": recPage("", "rec-3"),
	}
	return pages[req.PageToken], nil
}

// A page that stalls after a token must fail its plugin at the page deadline; the
// other plugin's rows still come back.
//
//nolint:paralleltest // shortens the package-level recommendationPageTimeout
func TestGetRecommendationsForResources_StalledPageFailsPluginAtDeadline(t *testing.T) {
	shortenRecommendationPageTimeout(t, 20*time.Millisecond)

	ctx := context.Background()
	stalled := &recordingRecClient{
		name: "stalled",
		respond: func(ctx context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			if req.PageToken == "" {
				return recPage("p2", "rec-1"), nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	healthy := &recordingRecClient{
		name: "healthy",
		recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
			return []*proto.Recommendation{{ID: "rec-healthy"}}
		},
	}
	eng := New([]*pluginhost.Client{stalled.client(), healthy.client()}, nil).
		WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	require.Len(t, result.Errors, 1)
	assert.Equal(t, "stalled", result.Errors[0].PluginName)
	assert.Contains(t, result.Errors[0].Error, "page 2")
	assert.Contains(t, result.Errors[0].Error, context.DeadlineExceeded.Error())
	assert.Equal(t, []string{"rec-healthy"}, recIDs(result.Recommendations))
}

func TestGetRecommendationsForResources_PageCapAcceptsEmptyTokenOnFinalPage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	issued := 0
	plugin := &recordingRecClient{
		name: "p",
		respond: func(_ context.Context, _ *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			issued++
			next := ""
			if issued < maxRecommendationPages {
				next = fmt.Sprintf("t%d", issued)
			}
			return recPage(next, fmt.Sprintf("rec-%d", issued)), nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Len(t, result.Recommendations, maxRecommendationPages)
	assert.Equal(t, maxRecommendationPages, plugin.calls())
}

func TestFetchRecommendationPages_FollowUpRequestsKeepEveryFieldButToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{
		name:    "p",
		respond: threePageResponder,
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil)
	req := &proto.GetRecommendationsRequest{
		TargetResources:           []*proto.ResourceDescriptor{{ID: "i-1", Type: "aws:ec2/instance:Instance"}},
		ProjectionPeriod:          "annual",
		ExcludedRecommendationIDs: []string{"rec-dismissed"},
		IncludeDismissed:          true,
		UsageProfile:              pbc.UsageProfile_USAGE_PROFILE_DEV,
	}

	recs, err := eng.fetchRecommendationPages(ctx, plugin.client(), req)
	require.NoError(t, err)
	assert.Len(t, recs, 3)

	reqs := plugin.reqs
	require.Len(t, reqs, 3)
	for i, sent := range reqs {
		assert.Equal(t, "annual", sent.ProjectionPeriod, "page %d", i+1)
		assert.Equal(t, []string{"rec-dismissed"}, sent.ExcludedRecommendationIDs, "page %d", i+1)
		assert.True(t, sent.IncludeDismissed, "page %d", i+1)
		assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_DEV, sent.UsageProfile, "page %d", i+1)
		assert.Equal(t, req.TargetResources, sent.TargetResources, "page %d", i+1)
	}
	assert.Equal(t, []string{"", "p2", "p3"}, []string{reqs[0].PageToken, reqs[1].PageToken, reqs[2].PageToken})
}

func TestGetRecommendationsForResources_FirstUSDRowIsNotReplacedByLaterPlugin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	usd := &recordingRecClient{name: "usd", recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
		return []*proto.Recommendation{{ID: "rec-usd", Impact: &proto.RecommendationImpact{Currency: "USD"}}}
	}}
	eur := &recordingRecClient{name: "eur", recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
		return []*proto.Recommendation{{ID: "rec-eur", Impact: &proto.RecommendationImpact{Currency: "EUR"}}}
	}}
	eng := New([]*pluginhost.Client{usd.client(), eur.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, []string{"rec-usd", "rec-eur"}, recIDs(result.Recommendations))
}

func TestGetRecommendationsForResources_LaterPageSetsCurrencyWhenNoneSelected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{
		name: "p",
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			if req.PageToken == "" {
				return recPage("p2", "rec-1"), nil
			}
			resp := recPage("", "rec-2")
			resp.Recommendations[0].Impact = &proto.RecommendationImpact{Currency: "EUR"}
			return resp, nil
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	assert.Equal(t, "EUR", result.Currency)
}

func TestGetRecommendationsForResources_DefaultCurrencyWhenNoRowNamesOne(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{name: "p", recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
		return []*proto.Recommendation{{ID: "rec-1"}}
	}}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)

	assert.Equal(t, defaultCurrency, result.Currency)
}

func TestGetRecommendationsForResources_ErroredResultIsNotStored(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	failing := true
	plugin := &recordingRecClient{
		name: "p",
		respond: func(_ context.Context, req *proto.GetRecommendationsRequest) (*proto.GetRecommendationsResponse, error) {
			if req.PageToken == "" {
				return recPage("p2", "rec-1"), nil
			}
			if failing {
				return nil, errors.New("transient")
			}
			return recPage("", "rec-2"), nil
		},
	}
	store := newRecCache(t)
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(store).
		WithDismissalStore(newRecDismissals(t))
	resources := []ResourceDescriptor{ec2("i-1")}

	first, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	require.Len(t, first.Errors, 1)

	failing = false
	second, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	assert.Empty(t, second.Errors)
	assert.Equal(t, []string{"rec-1", "rec-2"}, recIDs(second.Recommendations))
	assert.Equal(t, 4, plugin.calls(), "the errored run was not cached, so the second run queries the plugin again")
}

func TestGetRecommendationsForResources_CachedEntryWithErrorsIsBypassed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{name: "p", respond: threePageResponder}
	store := newRecCache(t)
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(store).
		WithDismissalStore(newRecDismissals(t))
	resources := []ResourceDescriptor{ec2("i-1")}

	excluded := loadExcludedRecommendationIDs(ctx, eng.dismissalStore)
	key := eng.recommendationsCacheKey(resources, excluded, false)
	require.True(t, strings.HasSuffix(key, "/recs-v2"))
	stale, err := json.Marshal(RecommendationsResult{
		Currency:        "USD",
		Recommendations: []Recommendation{{ID: "stale"}},
		Errors:          []RecommendationError{{PluginName: "p", Error: "old failure"}},
	})
	require.NoError(t, err)
	require.NoError(t, store.Set(key, stale))

	result, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)

	assert.Empty(t, result.Errors)
	assert.Equal(t, []string{"rec-1", "rec-2", "rec-3"}, recIDs(result.Recommendations))
	assert.Equal(t, 3, plugin.calls())
}

func TestGetRecommendationsForResources_IgnoresLegacyRecsV1CacheEntry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{name: "p", respond: threePageResponder}
	store := newRecCache(t)
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(store).
		WithDismissalStore(newRecDismissals(t))
	resources := []ResourceDescriptor{ec2("i-1")}

	excluded := loadExcludedRecommendationIDs(ctx, eng.dismissalStore)
	v2Key := eng.recommendationsCacheKey(resources, excluded, false)
	require.True(t, strings.HasSuffix(v2Key, "/recs-v2"))
	legacyKey := strings.TrimSuffix(v2Key, "/recs-v2") + "/recs-v1"
	legacy, err := json.Marshal(RecommendationsResult{
		Currency:        "USD",
		Recommendations: []Recommendation{{ID: "rec-1"}},
	})
	require.NoError(t, err)
	require.NoError(t, store.Set(legacyKey, legacy))

	result, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)

	assert.Equal(t, []string{"rec-1", "rec-2", "rec-3"}, recIDs(result.Recommendations))
	assert.Equal(t, 3, plugin.calls(), "the legacy entry is not read; all three pages are fetched")

	cached, err := store.Get(v2Key)
	require.NoError(t, err)
	require.NotNil(t, cached, "the fresh result is stored under the /recs-v2 key")
}
