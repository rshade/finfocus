package engine

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type recordingRecClient struct {
	proto.CostSourceClient

	name string
	host *pluginhost.Client
	mu   sync.Mutex
	reqs []*proto.GetRecommendationsRequest
	recs func(req *proto.GetRecommendationsRequest) []*proto.Recommendation
}

func (c *recordingRecClient) GetRecommendations(
	_ context.Context,
	in *proto.GetRecommendationsRequest,
	_ ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, in)
	c.mu.Unlock()
	resp := &proto.GetRecommendationsResponse{}
	if c.recs != nil {
		resp.Recommendations = c.recs(in)
	}
	return resp, nil
}

func (c *recordingRecClient) Supports(
	_ context.Context,
	_ *pbc.SupportsRequest,
	_ ...grpc.CallOption,
) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: true}, nil
}

func (c *recordingRecClient) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reqs)
}

func (c *recordingRecClient) requestedIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var ids []string
	for _, r := range c.reqs {
		for _, res := range r.TargetResources {
			ids = append(ids, res.ID)
		}
	}
	return ids
}

func (c *recordingRecClient) client() *pluginhost.Client {
	if c.host == nil {
		c.host = &pluginhost.Client{Name: c.name, API: c}
	}
	return c.host
}

func newRecCache(t *testing.T) *cache.BoltStore {
	t.Helper()
	store, err := cache.NewBoltStore(context.Background(), t.TempDir(), true, 3600, 10)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newRecDismissals(t *testing.T) *config.DismissalStore {
	t.Helper()
	store, err := config.NewDismissalStore(filepath.Join(t.TempDir(), "dismissed.json"))
	require.NoError(t, err)
	require.NoError(t, store.Load())
	return store
}

func ec2(id string) ResourceDescriptor {
	return ResourceDescriptor{
		Type:       "aws:ec2/instance:Instance",
		ID:         id,
		Provider:   "aws",
		Properties: map[string]interface{}{"instanceType": "t3.micro"},
	}
}

func TestConvertProtoRecommendation_RetainsFullRecord(t *testing.T) {
	createdAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	score := 0.7
	implCost := 12.0
	effort := 3.0

	got := convertProtoRecommendation(&proto.Recommendation{
		ID:          "rec-1",
		Category:    "RECOMMENDATION_CATEGORY_COST",
		ActionType:  "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE",
		Description: "Downsize",
		ResourceID:  "i-123",
		Source:      "kubecost",
		Metadata:    map[string]string{"team": "core"},
		Reasoning:   []string{"check arm64"},
		Priority:    "RECOMMENDATION_PRIORITY_HIGH",

		ConfidenceScore: &score,
		CreatedAt:       &createdAt,
		Impact: &proto.RecommendationImpact{
			EstimatedSavings:     40,
			Currency:             "USD",
			ProjectionPeriod:     "monthly",
			CurrentCost:          100,
			ProjectedCost:        60,
			SavingsPercentage:    40,
			ImplementationCost:   &implCost,
			MigrationEffortHours: &effort,
		},
		Resource: &proto.RecommendationResource{
			Name:         "web",
			Provider:     "aws",
			ResourceType: "aws:ec2/instance:Instance",
			Region:       "us-east-1",
			SKU:          "m5.large",
			Tags:         map[string]string{"env": "prod"},
			Utilization: &proto.RecommendationUtilization{
				CPUPercent:     12.5,
				MemoryPercent:  30,
				StoragePercent: 40,
				NetworkInMbps:  1,
				NetworkOutMbps: 2,
				CustomMetrics:  map[string]float64{"iops": 100},
			},
		},
	})

	assert.Equal(t, "rec-1", got.ID)
	assert.Equal(t, "i-123", got.ResourceID)
	assert.Equal(t, "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE", got.Type)
	assert.Equal(t, "RECOMMENDATION_CATEGORY_COST", got.Category)
	assert.Equal(t, "RECOMMENDATION_PRIORITY_HIGH", got.Priority)
	assert.Equal(t, "kubecost", got.Source)
	assert.Equal(t, map[string]string{"team": "core"}, got.Metadata)
	assert.Equal(t, []string{"check arm64"}, got.Reasoning)
	assert.InDelta(t, 40.0, got.EstimatedSavings, 1e-9)
	assert.Equal(t, "USD", got.Currency)

	require.NotNil(t, got.ConfidenceScore)
	assert.InDelta(t, 0.7, *got.ConfidenceScore, 1e-9)
	require.NotNil(t, got.CreatedAt)
	assert.True(t, createdAt.Equal(*got.CreatedAt))

	require.NotNil(t, got.ImpactDetail)
	assert.Equal(t, "monthly", got.ImpactDetail.ProjectionPeriod)
	assert.InDelta(t, 100.0, got.ImpactDetail.CurrentCost, 1e-9)
	assert.InDelta(t, 60.0, got.ImpactDetail.ProjectedCost, 1e-9)
	assert.InDelta(t, 40.0, got.ImpactDetail.SavingsPercentage, 1e-9)
	require.NotNil(t, got.ImpactDetail.ImplementationCost)
	assert.InDelta(t, 12.0, *got.ImpactDetail.ImplementationCost, 1e-9)
	require.NotNil(t, got.ImpactDetail.MigrationEffortHours)
	assert.InDelta(t, 3.0, *got.ImpactDetail.MigrationEffortHours, 1e-9)

	require.NotNil(t, got.ResourceInfo)
	assert.Equal(t, "web", got.ResourceInfo.Name)
	assert.Equal(t, "aws", got.ResourceInfo.Provider)
	assert.Equal(t, "aws:ec2/instance:Instance", got.ResourceInfo.ResourceType)
	assert.Equal(t, "us-east-1", got.ResourceInfo.Region)
	assert.Equal(t, "m5.large", got.ResourceInfo.SKU)
	assert.Equal(t, map[string]string{"env": "prod"}, got.ResourceInfo.Tags)
	require.NotNil(t, got.ResourceInfo.Utilization)
	assert.InDelta(t, 12.5, got.ResourceInfo.Utilization.CPUPercent, 1e-9)
	assert.InDelta(t, 30.0, got.ResourceInfo.Utilization.MemoryPercent, 1e-9)
	assert.InDelta(t, 40.0, got.ResourceInfo.Utilization.StoragePercent, 1e-9)
	assert.InDelta(t, 1.0, got.ResourceInfo.Utilization.NetworkInMbps, 1e-9)
	assert.InDelta(t, 2.0, got.ResourceInfo.Utilization.NetworkOutMbps, 1e-9)
	assert.Equal(t, map[string]float64{"iops": 100}, got.ResourceInfo.Utilization.CustomMetrics)
}

func TestConvertProtoRecommendation_UnspecifiedEnumsAreEmpty(t *testing.T) {
	got := convertProtoRecommendation(&proto.Recommendation{
		ID:         "rec-2",
		Category:   "RECOMMENDATION_CATEGORY_UNSPECIFIED",
		ActionType: "RECOMMENDATION_ACTION_TYPE_UNSPECIFIED",
		Priority:   "",
	})

	assert.Equal(t, "rec-2", got.ID)
	assert.Empty(t, got.Category)
	assert.Empty(t, got.Priority)
	assert.Nil(t, got.ConfidenceScore)
	assert.Nil(t, got.CreatedAt)
	assert.Nil(t, got.ImpactDetail)
	assert.Nil(t, got.ResourceInfo)
}

func TestRecommendation_JSONKeysAdditive(t *testing.T) {
	t.Run("legacy record emits only legacy keys", func(t *testing.T) {
		data, err := json.Marshal(Recommendation{
			ResourceID:       "r",
			Type:             "RIGHTSIZE",
			Description:      "d",
			EstimatedSavings: 1,
			Currency:         "USD",
			Reasoning:        []string{"x"},
		})
		require.NoError(t, err)

		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		assert.ElementsMatch(t,
			[]string{"resourceId", "type", "description", "estimatedSavings", "currency", "reasoning"}, keys)
	})

	t.Run("full record round trips", func(t *testing.T) {
		score := 0.5
		in := Recommendation{
			ID:              "rec-1",
			Type:            "RIGHTSIZE",
			Category:        "RECOMMENDATION_CATEGORY_COST",
			Priority:        "RECOMMENDATION_PRIORITY_LOW",
			ConfidenceScore: &score,
			Source:          "aws",
			Metadata:        map[string]string{"k": "v"},
			ImpactDetail:    &RecommendationImpactDetail{CurrentCost: 10},
			ResourceInfo:    &RecommendationResourceInfo{Name: "n", Tags: map[string]string{"a": "b"}},
		}
		data, err := json.Marshal(in)
		require.NoError(t, err)

		var out Recommendation
		require.NoError(t, json.Unmarshal(data, &out))
		assert.Equal(t, in, out)
	})
}

func TestGetRecommendationsForResources_CacheKeyIsolatesStacks(t *testing.T) {
	ctx := context.Background()
	plugin := &recordingRecClient{name: "p"}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(newRecCache(t)).
		WithDismissalStore(newRecDismissals(t))

	stackA := []ResourceDescriptor{ec2("i-a1"), ec2("i-a2")}
	stackB := []ResourceDescriptor{ec2("i-b1"), ec2("i-b2")}

	_, err := eng.GetRecommendationsForResources(ctx, stackA)
	require.NoError(t, err)
	require.Equal(t, 1, plugin.calls())

	_, err = eng.GetRecommendationsForResources(ctx, stackB)
	require.NoError(t, err)
	assert.Equal(t, 2, plugin.calls(), "different resources with identical type mix must not share a cache entry")

	_, err = eng.GetRecommendationsForResources(ctx, stackA)
	require.NoError(t, err)
	assert.Equal(t, 2, plugin.calls(), "identical request should hit the cache")

	reordered := []ResourceDescriptor{stackA[1], stackA[0]}
	_, err = eng.GetRecommendationsForResources(ctx, reordered)
	require.NoError(t, err)
	assert.Equal(t, 2, plugin.calls(), "resource order must not affect the cache key")
}

func TestGetRecommendationsForResources_CacheKeyReflectsProperties(t *testing.T) {
	ctx := context.Background()
	plugin := &recordingRecClient{name: "p"}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(newRecCache(t)).
		WithDismissalStore(newRecDismissals(t))

	small := ec2("i-1")
	large := ec2("i-1")
	large.Properties = map[string]interface{}{"instanceType": "m5.4xlarge"}

	_, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{small})
	require.NoError(t, err)
	_, err = eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{large})
	require.NoError(t, err)
	assert.Equal(t, 2, plugin.calls())
}

func TestGetRecommendationsForResources_NewDismissalInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	plugin := &recordingRecClient{
		name: "p",
		recs: func(req *proto.GetRecommendationsRequest) []*proto.Recommendation {
			for _, id := range req.ExcludedRecommendationIDs {
				if id == "rec-1" {
					return nil
				}
			}
			return []*proto.Recommendation{{ID: "rec-1", ResourceID: "i-1", Description: "d"}}
		},
	}
	dismissals := newRecDismissals(t)
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(newRecCache(t)).
		WithDismissalStore(dismissals)
	resources := []ResourceDescriptor{ec2("i-1")}

	first, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	require.Len(t, first.Recommendations, 1)

	require.NoError(t, dismissals.Set(&config.DismissalRecord{
		RecommendationID: "rec-1",
		Status:           config.StatusDismissed,
		DismissedAt:      time.Now(),
	}))

	second, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	assert.Empty(t, second.Recommendations, "dismissed recommendation must not be served from cache")
	assert.Equal(t, 2, plugin.calls())
}

func TestGetRecommendationsForResources_LegacyCacheEntryIsIgnored(t *testing.T) {
	ctx := context.Background()
	store := newRecCache(t)
	legacyKey := "recommendations/multi/aws:ec2/instance:Instance"
	require.NoError(
		t,
		store.Set(legacyKey, json.RawMessage(`{"recommendations":[{"type":"OLD","description":"stale"}]}`)),
	)

	plugin := &recordingRecClient{
		name: "p",
		recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
			return []*proto.Recommendation{{ID: "rec-new", ResourceID: "i-1", Description: "fresh"}}
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(store).
		WithDismissalStore(newRecDismissals(t))

	result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
	require.NoError(t, err)
	require.Len(t, result.Recommendations, 1)
	assert.Equal(t, "fresh", result.Recommendations[0].Description)
	assert.Equal(t, "rec-new", result.Recommendations[0].ID)
}

func TestGetRecommendationsForResources_CachedResultKeepsFullRecord(t *testing.T) {
	ctx := context.Background()
	score := 0.9
	plugin := &recordingRecClient{
		name: "p",
		recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation {
			return []*proto.Recommendation{{
				ID:              "rec-1",
				ResourceID:      "i-1",
				Priority:        "RECOMMENDATION_PRIORITY_HIGH",
				ConfidenceScore: &score,
				Resource:        &proto.RecommendationResource{Name: "web", Tags: map[string]string{"env": "prod"}},
			}}
		},
	}
	eng := New([]*pluginhost.Client{plugin.client()}, nil).
		WithCache(newRecCache(t)).
		WithDismissalStore(newRecDismissals(t))
	resources := []ResourceDescriptor{ec2("i-1")}

	_, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	cached, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)

	require.Equal(t, 1, plugin.calls())
	require.Len(t, cached.Recommendations, 1)
	rec := cached.Recommendations[0]
	assert.Equal(t, "rec-1", rec.ID)
	assert.Equal(t, "RECOMMENDATION_PRIORITY_HIGH", rec.Priority)
	require.NotNil(t, rec.ConfidenceScore)
	assert.InDelta(t, 0.9, *rec.ConfidenceScore, 1e-9)
	require.NotNil(t, rec.ResourceInfo)
	assert.Equal(t, map[string]string{"env": "prod"}, rec.ResourceInfo.Tags)
}

func TestGetRecommendationsForResources_UsesRouter(t *testing.T) {
	ctx := context.Background()

	t.Run("only routed plugins receive each resource", func(t *testing.T) {
		awsPlugin := &recordingRecClient{name: "aws-plugin"}
		gcpPlugin := &recordingRecClient{name: "gcp-plugin"}
		router := &mockRouter{
			selectPluginsFunc: func(_ context.Context, r ResourceDescriptor, feature string) []PluginMatch {
				assert.Equal(t, "Recommendations", feature)
				if r.Provider == "aws" {
					return []PluginMatch{{Client: awsPlugin.client()}}
				}
				return []PluginMatch{{Client: gcpPlugin.client()}}
			},
		}
		eng := New([]*pluginhost.Client{awsPlugin.client(), gcpPlugin.client()}, nil).
			WithRouter(router).
			WithDismissalStore(newRecDismissals(t))

		gcp := ResourceDescriptor{Type: "gcp:compute/instance:Instance", ID: "g-1", Provider: "gcp"}
		_, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1"), gcp})
		require.NoError(t, err)

		assert.Equal(t, []string{"i-1"}, awsPlugin.requestedIDs())
		assert.Equal(t, []string{"g-1"}, gcpPlugin.requestedIDs())
	})

	t.Run("plugin with no routed resources is not called", func(t *testing.T) {
		awsPlugin := &recordingRecClient{name: "aws-plugin"}
		idle := &recordingRecClient{name: "idle-plugin"}
		router := &mockRouter{
			selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
				return []PluginMatch{{Client: awsPlugin.client()}}
			},
		}
		eng := New([]*pluginhost.Client{awsPlugin.client(), idle.client()}, nil).
			WithRouter(router).
			WithDismissalStore(newRecDismissals(t))

		_, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
		require.NoError(t, err)
		assert.Equal(t, 0, idle.calls())
	})

	t.Run("without router every plugin is queried", func(t *testing.T) {
		p1 := &recordingRecClient{name: "p1"}
		p2 := &recordingRecClient{name: "p2"}
		eng := New([]*pluginhost.Client{p1.client(), p2.client()}, nil).
			WithDismissalStore(newRecDismissals(t))

		_, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
		require.NoError(t, err)
		assert.Equal(t, []string{"i-1"}, p1.requestedIDs())
		assert.Equal(t, []string{"i-1"}, p2.requestedIDs())
	})

	t.Run("internal pulumi types are never sent to plugins", func(t *testing.T) {
		p := &recordingRecClient{name: "p"}
		eng := New([]*pluginhost.Client{p.client()}, nil).
			WithDismissalStore(newRecDismissals(t))

		stack := ResourceDescriptor{Type: "pulumi:pulumi:Stack", ID: "stack", Provider: "pulumi"}
		_, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{stack, ec2("i-1")})
		require.NoError(t, err)
		assert.Equal(t, []string{"i-1"}, p.requestedIDs())
	})

	t.Run("routing applies on the batched path", func(t *testing.T) {
		awsPlugin := &recordingRecClient{name: "aws-plugin"}
		other := &recordingRecClient{name: "other"}
		router := &mockRouter{
			selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch {
				return []PluginMatch{{Client: awsPlugin.client()}}
			},
		}
		eng := New([]*pluginhost.Client{awsPlugin.client(), other.client()}, nil).
			WithRouter(router).
			WithDismissalStore(newRecDismissals(t))

		resources := make([]ResourceDescriptor, 0, batchProcessingThreshold+5)
		for i := range batchProcessingThreshold + 5 {
			resources = append(resources, ec2("i-"+strconv.Itoa(i)))
		}
		_, err := eng.GetRecommendationsForResources(ctx, resources)
		require.NoError(t, err)
		assert.Len(t, awsPlugin.requestedIDs(), len(resources))
		assert.Equal(t, 0, other.calls())
	})

	t.Run("scorer-only plugins are not queried as cost sources", func(t *testing.T) {
		source := &recordingRecClient{name: "source"}
		scorer := &recordingRecClient{name: "scorer"}
		scorerHost := scorer.client()
		scorerHost.Metadata = &proto.PluginMetadata{Capabilities: []string{"recommendation_scoring"}}
		hybridHost := &pluginhost.Client{
			Name:     "hybrid",
			API:      source,
			Metadata: &proto.PluginMetadata{Capabilities: []string{"recommendations", "recommendation_scoring"}},
		}
		eng := New([]*pluginhost.Client{source.client(), scorerHost, hybridHost}, nil).
			WithDismissalStore(newRecDismissals(t))

		result, err := eng.GetRecommendationsForResources(ctx, []ResourceDescriptor{ec2("i-1")})
		require.NoError(t, err)
		assert.Empty(t, result.Errors)
		assert.Equal(t, 0, scorer.calls())
		assert.Equal(t, 2, source.calls(), "the source plugin and the hybrid plugin (same mock API) are queried")
	})
}

func TestRecommendationScores_Signal(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	s := &RecommendationScores{
		Risk: v(0.1), FalsePositive: v(0.2), WorthActing: v(0.3), Priority: v(2), InsufficientEvidence: v(0.5),
	}

	want := map[string]float64{
		ScoreSignalRisk: 0.1, ScoreSignalFalsePositive: 0.2, ScoreSignalWorthActing: 0.3,
		ScoreSignalPriority: 2, ScoreSignalInsufficientEvidence: 0.5,
	}
	for _, name := range ScoreSignalNames() {
		got, ok := s.Signal(name)
		require.True(t, ok, name)
		assert.InDelta(t, want[name], got, 1e-9, name)
	}

	_, ok := (&RecommendationScores{}).Signal(ScoreSignalRisk)
	assert.False(t, ok)
	_, ok = (*RecommendationScores)(nil).Signal(ScoreSignalRisk)
	assert.False(t, ok)
	_, ok = s.Signal("bogus")
	assert.False(t, ok)
}
