package engine

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func (c *recordingRecClient) snapshot() []*proto.GetRecommendationsRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*proto.GetRecommendationsRequest, len(c.reqs))
	copy(out, c.reqs)
	return out
}

func TestGetRecommendationsForResourcesWithDismissed_SetsFieldAndIsolatesCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{
		name: "p",
		recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
			return []*proto.Recommendation{{ID: "rec-2", ResourceID: "i-1", Description: "live"}}
		},
	}
	dismissals := newRecDismissals(t)
	require.NoError(t, dismissals.Set(&config.DismissalRecord{
		RecommendationID: "rec-1",
		Status:           config.StatusDismissed,
		DismissedAt:      time.Now(),
	}))
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(newRecCache(t)).
		WithDismissalStore(dismissals)
	resources := []ResourceDescriptor{ec2("i-1")}

	_, err := eng.GetRecommendationsForResourcesWithDismissed(ctx, resources)
	require.NoError(t, err)
	require.Equal(t, 1, plugin.calls())

	flagged := plugin.snapshot()
	require.Len(t, flagged, 1)
	assert.True(t, flagged[0].IncludeDismissed)
	assert.Equal(t, []string{"rec-1"}, flagged[0].ExcludedRecommendationIDs)

	_, err = eng.GetRecommendationsForResourcesWithDismissed(ctx, resources)
	require.NoError(t, err)
	assert.Equal(t, 1, plugin.calls(), "repeated flagged fetch hits its own cache")

	_, err = eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	assert.Equal(t, 2, plugin.calls(), "default fetch must not read the flagged cache")

	reqs := plugin.snapshot()
	require.Len(t, reqs, 2)
	assert.False(t, reqs[1].IncludeDismissed)
	assert.Equal(t, []string{"rec-1"}, reqs[1].ExcludedRecommendationIDs)

	_, err = eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	assert.Equal(t, 2, plugin.calls(), "repeated default fetch hits its own cache")
}

func TestGetRecommendationsForResourcesWithDismissed_BatchSetsField(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plugin := &recordingRecClient{name: "p"}
	dismissals := newRecDismissals(t)
	require.NoError(t, dismissals.Set(&config.DismissalRecord{
		RecommendationID: "rec-1",
		Status:           config.StatusDismissed,
		DismissedAt:      time.Now(),
	}))
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithDismissalStore(dismissals)

	resources := make([]ResourceDescriptor, batchProcessingThreshold+1)
	for i := range resources {
		resources[i] = ec2("i-" + strconv.Itoa(i))
	}

	_, err := eng.GetRecommendationsForResourcesWithDismissed(ctx, resources)
	require.NoError(t, err)
	reqs := plugin.snapshot()
	require.GreaterOrEqual(t, len(reqs), 2, "more than one batch must be sent")
	for i, req := range reqs {
		assert.True(t, req.IncludeDismissed, "batch %d", i)
		assert.Equal(t, []string{"rec-1"}, req.ExcludedRecommendationIDs, "batch %d", i)
		assert.NotEmpty(t, req.TargetResources, "batch %d", i)
	}
}
