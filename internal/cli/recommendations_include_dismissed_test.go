package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// includeDismissedClient records recommendation requests. Other RPCs are unused.
type includeDismissedClient struct {
	proto.CostSourceClient

	mu   sync.Mutex
	reqs []*proto.GetRecommendationsRequest
}

func (c *includeDismissedClient) GetRecommendations(
	_ context.Context,
	in *proto.GetRecommendationsRequest,
	_ ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, in)
	c.mu.Unlock()
	return &proto.GetRecommendationsResponse{
		Recommendations: []*proto.Recommendation{{ID: "rec-2", ResourceID: "i-1"}},
	}, nil
}

func (c *includeDismissedClient) Supports(
	_ context.Context,
	_ *pbc.SupportsRequest,
	_ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (c *includeDismissedClient) requests() []*proto.GetRecommendationsRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*proto.GetRecommendationsRequest, len(c.reqs))
	copy(out, c.reqs)
	return out
}

func TestFetchRecommendationsWithProgress_IncludeDismissedReachesPlugin(t *testing.T) {
	t.Parallel()

	store, err := config.NewDismissalStore(filepath.Join(t.TempDir(), "dismissed.json"))
	require.NoError(t, err)
	require.NoError(t, store.Load())
	require.NoError(t, store.Set(&config.DismissalRecord{
		RecommendationID: "rec-1",
		Status:           config.StatusDismissed,
		DismissedAt:      time.Now(),
	}))

	client := &includeDismissedClient{}
	eng := engine.New([]*pluginhost.Client{{Name: "p", API: client}}, nil).
		WithDismissalStore(store)
	resources := []engine.ResourceDescriptor{{
		Type:       "aws:ec2/instance:Instance",
		ID:         "i-1",
		Provider:   "aws",
		Properties: map[string]interface{}{"instanceType": "t3.micro"},
	}}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	_, err = fetchRecommendationsWithProgress(context.Background(), cmd, eng, resources, true)
	require.NoError(t, err)
	reqs := client.requests()
	require.Len(t, reqs, 1)
	assert.True(t, reqs[0].IncludeDismissed)
	assert.Equal(t, []string{"rec-1"}, reqs[0].ExcludedRecommendationIDs)

	_, err = fetchRecommendationsWithProgress(context.Background(), cmd, eng, resources, false)
	require.NoError(t, err)
	reqs = client.requests()
	require.Len(t, reqs, 2)
	assert.False(t, reqs[1].IncludeDismissed)
	assert.Equal(t, []string{"rec-1"}, reqs[1].ExcludedRecommendationIDs)
}
