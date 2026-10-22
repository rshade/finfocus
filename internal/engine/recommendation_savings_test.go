package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func modifyRec(resource string, savings float64, current, recommended map[string]string) Recommendation {
	return Recommendation{
		ResourceID: resource, Type: "MODIFY", Currency: "USD", EstimatedSavings: savings,
		ActionDetail: &RecommendationActionDetail{Modify: &ModifyActionDetail{
			CurrentConfig: current, RecommendedConfig: recommended,
		}},
	}
}

func generationUpgrade(resource string, savings float64) Recommendation {
	return modifyRec(resource, savings,
		map[string]string{"instance_type": "t3.micro"},
		map[string]string{"instance_type": "t3a.micro"})
}

func graviton(resource string, savings float64) Recommendation {
	return modifyRec(resource, savings,
		map[string]string{"instance_type": "t3.micro", "architecture": "x86_64"},
		map[string]string{"instance_type": "t4g.micro", "architecture": "arm64"})
}

// demoPRRecommendations are the recommendations of rshade/finfocus-demo#1: the
// plain sum is 26.72 and the achievable figure is 20.15.
func demoPRRecommendations() []Recommendation {
	return []Recommendation{
		generationUpgrade("demo-instance", 0.73),
		graviton("demo-instance", 1.46),
		generationUpgrade("legacy-instance", 7.01),
		generationUpgrade("demo-instance-large", 5.84),
		graviton("demo-instance-large", 11.68),
	}
}

const demoPRAchievable = 20.15

func TestAchievableSavings(t *testing.T) {
	t.Parallel()

	iops := func(resource string, savings float64) Recommendation {
		return modifyRec(resource, savings, map[string]string{"iops": "3000"}, map[string]string{"iops": "4000"})
	}
	throughput := func(resource string, savings float64) Recommendation {
		return modifyRec(resource, savings,
			map[string]string{"throughput": "125"}, map[string]string{"throughput": "250"})
	}
	undetailed := func(resource, recType string, savings float64) Recommendation {
		return Recommendation{ResourceID: resource, Type: recType, EstimatedSavings: savings, Currency: "USD"}
	}

	tests := []struct {
		name string
		recs []Recommendation
		want float64
	}{
		{name: "none", recs: nil, want: 0},
		{name: "demo PR alternatives take the largest", recs: demoPRRecommendations(), want: demoPRAchievable},
		{
			name: "independent changes on one resource sum",
			recs: []Recommendation{iops("vol-1", 10), throughput("vol-1", 5)},
			want: 15,
		},
		{
			name: "same change on different resources sums",
			recs: []Recommendation{generationUpgrade("i-1", 1), generationUpgrade("i-2", 2)},
			want: 3,
		},
		{
			name: "different types on one resource sum",
			recs: []Recommendation{generationUpgrade("i-1", 3), undetailed("i-1", "TERMINATE", 4)},
			want: 7,
		},
		{
			name: "recommendations without detail share a group per resource and type",
			recs: []Recommendation{undetailed("i-1", "RIGHTSIZE", 3), undetailed("i-1", "RIGHTSIZE", 5)},
			want: 5,
		},
		{
			name: "recommendations without a resource id are never alternatives",
			recs: []Recommendation{undetailed("", "RIGHTSIZE", 3), undetailed("", "RIGHTSIZE", 5)},
			want: 8,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, AchievableSavings(tt.recs), 1e-9)
		})
	}
}

func TestBuildRecommendationSummary_TotalIsAchievable(t *testing.T) {
	t.Parallel()

	summary := BuildRecommendationSummary(demoPRRecommendations())

	assert.InDelta(t, demoPRAchievable, summary.TotalSavings, 1e-9)
	assert.InDelta(t, 26.72, summary.SavingsByAction["MODIFY"], 1e-9, "the per-type breakdown stays a plain sum")
	assert.Equal(t, 5, summary.TotalCount)
}

func TestSummarizeOverviewRows_TotalSavingsIsAchievable(t *testing.T) {
	t.Parallel()

	recsByResource := map[string][]Recommendation{}
	var order []string
	for _, rec := range demoPRRecommendations() {
		if _, seen := recsByResource[rec.ResourceID]; !seen {
			order = append(order, rec.ResourceID)
		}
		recsByResource[rec.ResourceID] = append(recsByResource[rec.ResourceID], rec)
	}
	var rows []OverviewRow
	for _, resource := range order {
		rows = append(rows, OverviewRow{
			URN: "urn:" + resource, Status: StatusActive, Recommendations: recsByResource[resource],
		})
	}
	dismissed := graviton("demo-instance", 100)
	dismissed.Status = RecommendationStatusDismissed
	snoozed := graviton("legacy-instance", 100)
	snoozed.Status = RecommendationStatusSnoozed
	rows[0].Recommendations = append(rows[0].Recommendations, dismissed)
	rows[1].Recommendations = append(rows[1].Recommendations, snoozed)

	assert.InDelta(t, demoPRAchievable, summarizeOverviewRows(rows).TotalSavings, 1e-9)
}

func TestSummarizeOverviewRows_RecommendationsWithoutResourceIDStayPerRow(t *testing.T) {
	t.Parallel()

	rec := func(savings float64) Recommendation {
		return Recommendation{Type: "RIGHTSIZE", EstimatedSavings: savings, Currency: "USD"}
	}
	rows := []OverviewRow{
		{URN: "urn:a", Status: StatusActive, Recommendations: []Recommendation{rec(3), rec(5)}},
		{URN: "urn:b", Status: StatusActive, Recommendations: []Recommendation{rec(4)}},
	}

	assert.InDelta(t, 9, summarizeOverviewRows(rows).TotalSavings, 1e-9, "5 for urn:a plus 4 for urn:b")
}

func demoPluginRecommendations() []*proto.Recommendation {
	var out []*proto.Recommendation
	for _, rec := range demoPRRecommendations() {
		out = append(out, &proto.Recommendation{
			ResourceID: rec.ResourceID,
			ActionType: rec.Type,
			Impact:     &proto.RecommendationImpact{EstimatedSavings: rec.EstimatedSavings, Currency: rec.Currency},
			ActionDetail: &proto.RecommendationActionDetail{Modify: &proto.ModifyActionDetail{
				CurrentConfig:     rec.ActionDetail.Modify.CurrentConfig,
				RecommendedConfig: rec.ActionDetail.Modify.RecommendedConfig,
			}},
		})
	}
	return out
}

func TestGetRecommendationsForResources_TotalSavingsIsAchievable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		resources int
	}{
		{name: "sequential", resources: 1},
		{name: "batched", resources: batchProcessingThreshold + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plugin := &recordingRecClient{
				name: "p",
				recs: func(req *proto.GetRecommendationsRequest) []*proto.Recommendation {
					if len(req.TargetResources) == 0 || req.TargetResources[0].ID != "r-0" {
						return nil
					}
					return demoPluginRecommendations()
				},
			}
			eng := New([]*pluginhost.Client{plugin.client()}, nil)
			descriptors := make([]ResourceDescriptor, tt.resources)
			for i := range descriptors {
				descriptors[i] = ec2(fmt.Sprintf("r-%d", i))
			}

			result, err := eng.GetRecommendationsForResources(context.Background(), descriptors)
			require.NoError(t, err)
			require.Len(t, result.Recommendations, 5)
			assert.InDelta(t, demoPRAchievable, result.TotalSavings, 1e-9)
		})
	}
}

func TestGetRecommendationsForResources_StaleCachedTotalIsRecomputed(t *testing.T) {
	t.Parallel()

	plugin := &recordingRecClient{
		name: "p",
		recs: func(*proto.GetRecommendationsRequest) []*proto.Recommendation { return demoPluginRecommendations() },
	}
	store := newRecCache(t)
	eng := New([]*pluginhost.Client{plugin.client()}, nil).WithCache(store)
	resources := []ResourceDescriptor{ec2("r-0")}
	ctx := context.Background()

	_, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)

	key := eng.recommendationsCacheKey(resources, nil, false)
	entry, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, entry)
	var stale RecommendationsResult
	require.NoError(t, json.Unmarshal(entry.Data, &stale))
	stale.TotalSavings = 26.72
	data, err := json.Marshal(stale)
	require.NoError(t, err)
	require.NoError(t, store.Set(key, json.RawMessage(data)))

	result, err := eng.GetRecommendationsForResources(ctx, resources)
	require.NoError(t, err)
	assert.Equal(t, 1, plugin.calls(), "the second call is served from the cache")
	assert.InDelta(t, demoPRAchievable, result.TotalSavings, 1e-9)
}
