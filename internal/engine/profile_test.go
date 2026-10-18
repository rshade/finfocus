package engine

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type recordingCostClient struct {
	mockCostSourceClient

	mu              sync.Mutex
	projected       []*proto.GetProjectedCostRequest
	recommendations []*proto.GetRecommendationsRequest
	batchCalls      int
}

func (r *recordingCostClient) GetProjectedCost(
	_ context.Context,
	req *proto.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	r.mu.Lock()
	r.projected = append(r.projected, req)
	r.mu.Unlock()
	return &proto.GetProjectedCostResponse{
		Results: []*proto.CostResult{{
			Currency:    "USD",
			MonthlyCost: 10,
			HourlyCost:  0.01,
		}},
	}, nil
}

func (r *recordingCostClient) GetRecommendations(
	_ context.Context,
	req *proto.GetRecommendationsRequest,
	_ ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	r.mu.Lock()
	r.recommendations = append(r.recommendations, req)
	r.mu.Unlock()
	return &proto.GetRecommendationsResponse{}, nil
}

func (r *recordingCostClient) BatchCost(
	_ context.Context,
	_ *pbc.BatchCostRequest,
	_ ...grpc.CallOption,
) (*pbc.BatchCostResponse, error) {
	r.mu.Lock()
	r.batchCalls++
	r.mu.Unlock()
	return &pbc.BatchCostResponse{}, nil
}

func profileResource() ResourceDescriptor {
	return ResourceDescriptor{
		ID:       "i-1",
		Provider: "aws",
		Type:     "aws:ec2/instance:Instance",
	}
}

func TestGetProjectedCostSendsUsageProfile(t *testing.T) {
	t.Parallel()

	plugin := &recordingCostClient{}
	eng := New([]*pluginhost.Client{{Name: "aws", API: plugin}}, nil).
		WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_DEV)

	_, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{profileResource()})
	require.NoError(t, err)
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	require.NotEmpty(t, plugin.projected)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_DEV, plugin.projected[0].UsageProfile)
	assert.Zero(t, plugin.batchCalls)
}

func TestHourlyRateLookupOmitsUsageProfile(t *testing.T) {
	t.Parallel()

	plugin := &recordingCostClient{}
	eng := New([]*pluginhost.Client{{Name: "aws", API: plugin}}, nil).
		WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_DEV)

	hourly, err := eng.getHourlyRateForResource(context.Background(), profileResource())
	require.NoError(t, err)
	assert.InDelta(t, 0.01, hourly, 1e-9)
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	require.NotEmpty(t, plugin.projected)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_UNSPECIFIED, plugin.projected[0].UsageProfile)
}

func TestGetRecommendationsSendsUsageProfile(t *testing.T) {
	t.Parallel()

	plugin := &recordingCostClient{}
	store, err := config.NewDismissalStore(t.TempDir() + "/dismissed.json")
	require.NoError(t, err)
	eng := New([]*pluginhost.Client{{Name: "aws", API: plugin}}, nil).
		WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_BURST).
		WithDismissalStore(store)

	result, err := eng.GetRecommendationsForResources(context.Background(), []ResourceDescriptor{profileResource()})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "burst", result.Profile)
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	require.NotEmpty(t, plugin.recommendations)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_BURST, plugin.recommendations[0].UsageProfile)
}

func TestProjectedBatchSkippedWhenProfileSet(t *testing.T) {
	t.Parallel()

	plugin := &recordingCostClient{}
	eng := New([]*pluginhost.Client{{
		Name:     "aws",
		API:      plugin,
		Metadata: &proto.PluginMetadata{Capabilities: []string{"batch_cost"}},
	}}, nil).WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_PROD)

	_, err := eng.GetProjectedCost(context.Background(), []ResourceDescriptor{profileResource()})
	require.NoError(t, err)
	plugin.mu.Lock()
	defer plugin.mu.Unlock()
	assert.Zero(t, plugin.batchCalls)
	require.NotEmpty(t, plugin.projected)
	assert.Equal(t, pbc.UsageProfile_USAGE_PROFILE_PROD, plugin.projected[0].UsageProfile)
}

func TestUsageProfileCacheSuffix(t *testing.T) {
	t.Parallel()

	resource := profileResource()
	plain := New(nil, nil)
	dev := New(nil, nil).WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_DEV)
	prod := New(nil, nil).WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_PROD)
	unknown := New(nil, nil).WithUsageProfile(pbc.UsageProfile(99))

	plainKey, err := plain.projectedCostCacheKey(resource)
	require.NoError(t, err)
	devKey, err := dev.projectedCostCacheKey(resource)
	require.NoError(t, err)
	prodKey, err := prod.projectedCostCacheKey(resource)
	require.NoError(t, err)
	unknownKey, err := unknown.projectedCostCacheKey(resource)
	require.NoError(t, err)

	assert.NotContains(t, plainKey, "/profile-")
	assert.Contains(t, devKey, "/profile-dev")
	assert.Contains(t, prodKey, "/profile-prod")
	assert.Equal(t, plainKey, unknownKey)
	assert.NotEqual(t, plainKey, devKey)
	assert.NotEqual(t, devKey, prodKey)

	plainRec := plain.recommendationsCacheKey([]ResourceDescriptor{resource}, nil, true)
	devRec := dev.recommendationsCacheKey([]ResourceDescriptor{resource}, nil, true)
	assert.Contains(t, plainRec, "/include-dismissed")
	assert.NotContains(t, plainRec, "/profile-")
	assert.Contains(t, devRec, "/include-dismissed")
	assert.Contains(t, devRec, "/profile-dev")
	assert.Greater(t, len(devRec), len("/profile-dev"))
	assert.Equal(t, "/profile-dev", devRec[len(devRec)-len("/profile-dev"):])
}

func TestProjectedDiffNamesProfile(t *testing.T) {
	t.Parallel()

	eng := New(nil, nil).WithUsageProfile(pbc.UsageProfile_USAGE_PROFILE_DEV)
	diff, err := eng.GetProjectedCostDiff(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, diff)
	assert.Equal(t, "dev", diff.Profile)

	var buf bytes.Buffer
	require.NoError(t, renderDiffTable(&buf, diff, false))
	assert.Contains(t, buf.String(), "Profile: dev")
	assert.Contains(t, buf.String(), "COST DIFF")

	buf.Reset()
	require.NoError(t, renderDiffJSON(&buf, &DiffResult{Summary: DiffSummary{Currency: "USD"}}))
	assert.Contains(t, buf.String(), `"profile": "default"`)
}
