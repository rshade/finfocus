package scoring

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
)

const rawInstance = "i-0abc123def456"

func rec(id, resourceID string) engine.Recommendation {
	score := 0.8
	return engine.Recommendation{
		ID:               id,
		ResourceID:       resourceID,
		Type:             "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE",
		Category:         "RECOMMENDATION_CATEGORY_COST",
		Priority:         "RECOMMENDATION_PRIORITY_HIGH",
		ConfidenceScore:  &score,
		Source:           "kubecost",
		Description:      "Downsize " + resourceID + " (web-server) to save money",
		Reasoning:        []string{"check " + resourceID},
		Metadata:         map[string]string{"owner": "team-a"},
		EstimatedSavings: 40,
		Currency:         "USD",
		Status:           engine.RecommendationStatusActive,
		ImpactDetail:     &engine.RecommendationImpactDetail{CurrentCost: 100, ProjectedCost: 60},
		ResourceInfo: &engine.RecommendationResourceInfo{
			Name:         "web-server",
			Provider:     "aws",
			ResourceType: "aws:ec2/instance:Instance",
			Region:       "us-east-1",
			Tags:         map[string]string{"env": "prod", "instance": resourceID},
		},
	}
}

func recs(n int) []engine.Recommendation {
	out := make([]engine.Recommendation, n)
	for i := range n {
		out[i] = rec("plugin-rec-"+strconv.Itoa(i), "i-"+strconv.Itoa(i))
		out[i].EstimatedSavings = 40 + float64(i)
	}
	return out
}

type fakeScorer struct {
	mu       sync.Mutex
	requests []*pbc.ScoreRecommendationsRequest
	handler  func(req *pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error)
}

func (f *fakeScorer) ScoreRecommendations(
	_ context.Context,
	req *pbc.ScoreRecommendationsRequest,
	_ ...grpc.CallOption,
) (*pbc.ScoreRecommendationsResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	if f.handler != nil {
		return f.handler(req)
	}
	return okResponse(req, "model-1", 100), nil
}

func (f *fakeScorer) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeScorer) sentRecommendations() []*pbc.Recommendation {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pbc.Recommendation
	for _, r := range f.requests {
		out = append(out, r.GetRecommendations()...)
	}
	return out
}

func ptr(v float64) *float64 { return &v }

func okResponse(req *pbc.ScoreRecommendationsRequest, model string, maxBatch int32) *pbc.ScoreRecommendationsResponse {
	resp := &pbc.ScoreRecommendationsResponse{
		MaxBatchSize: maxBatch,
		Scorer: &pbc.ScorerInfo{
			Name: "fake", Model: model, Calibration: pbc.ScoreCalibration_SCORE_CALIBRATION_RANKING_ONLY,
		},
		SupportedSignals: []pbc.ScoreSignal{
			pbc.ScoreSignal_SCORE_SIGNAL_RISK, pbc.ScoreSignal_SCORE_SIGNAL_PRIORITY,
			pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP,
		},
	}
	for i, r := range req.GetRecommendations() {
		risk := 0.1 + float64(i%5)*0.2
		resp.Results = append(resp.Results, &pbc.RecommendationScoreResult{
			RecommendationId: r.GetId(),
			Result: &pbc.RecommendationScoreResult_Scores{Scores: &pbc.RecommendationScores{
				Risk:     &risk,
				Priority: ptr(2),
			}},
		})
	}
	return resp
}

func newService(t *testing.T, scorer Scorer, opts Options) *Service {
	t.Helper()
	if opts.IdentifierMode == "" {
		opts.IdentifierMode = config.ScoringIdentifierPseudonymized
	}
	if opts.ScorerName == "" {
		opts.ScorerName = "fake"
	}
	return New(scorer, opts)
}

type memCache struct {
	mu      sync.Mutex
	entries map[string]json.RawMessage
}

func newMemCache() *memCache { return &memCache{entries: map[string]json.RawMessage{}} }

func (m *memCache) Get(key string) (*cache.CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.entries[key]
	if !ok {
		return nil, cache.ErrCacheNotFound
	}
	return &cache.CacheEntry{Key: key, Data: data}, nil
}

func (m *memCache) Set(key string, data json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = data
	return nil
}

func (m *memCache) SetWithTTL(key string, data json.RawMessage, _ int) error { return m.Set(key, data) }
func (m *memCache) IsEnabled() bool                                          { return true }
func (m *memCache) Close() error                                             { return nil }
func (m *memCache) InvalidateByPrefix(string) (int, error)                   { return 0, nil }

func TestScore_PseudonymizedNeverSendsRawIdentifiers(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{})
	input := recs(3)

	_, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)
	require.NotEmpty(t, scorer.requests)

	for _, req := range scorer.requests {
		assert.Equal(t, pbc.IdentifierMode_IDENTIFIER_MODE_PSEUDONYMIZED, req.GetIdentifierMode())
		blob, marshalErr := protojson.Marshal(req)
		require.NoError(t, marshalErr)
		text := string(blob)
		assert.NotContains(t, text, "i-0")
		assert.NotContains(t, text, "web-server")
		assert.NotContains(t, text, "plugin-rec")
		for i, r := range req.GetRecommendations() {
			assert.Equal(t, "rec-"+strconv.Itoa(i), r.GetId(), "recommendation ids are opaque per-request indexes")
			assert.True(t, strings.HasPrefix(r.GetResource().GetId(), "res-"))
			assert.True(t, strings.HasPrefix(r.GetResource().GetName(), "name-"))
		}
	}
}

func TestScore_PseudonymTokensAreStablePerResourceAndFreshPerRequest(t *testing.T) {
	t.Parallel()

	a := rec("rec-a", rawInstance)
	b := rec("rec-b", strings.ToUpper(rawInstance))
	c := rec("rec-c", "i-other")

	ids, err := newIdentifiers(config.ScoringIdentifierPseudonymized)
	require.NoError(t, err)
	pa, _ := toProto(a, ids)
	pb, _ := toProto(b, ids)
	pc, _ := toProto(c, ids)
	assert.Equal(t, pa.GetResource().GetId(), pb.GetResource().GetId(), "normalized ids match across plugins")
	assert.NotEqual(t, pa.GetResource().GetId(), pc.GetResource().GetId())

	other, err := newIdentifiers(config.ScoringIdentifierPseudonymized)
	require.NoError(t, err)
	pa2, _ := toProto(a, other)
	assert.NotEqual(t, pa.GetResource().GetId(), pa2.GetResource().GetId(), "each request uses its own key")
}

func TestScore_RawModeKeepsIdentifiersOnExplicitOptIn(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{IdentifierMode: config.ScoringIdentifierRaw})

	_, err := svc.Score(context.Background(), recs(1), false)
	require.NoError(t, err)

	sent := scorer.sentRecommendations()
	require.Len(t, sent, 1)
	assert.Equal(t, pbc.IdentifierMode_IDENTIFIER_MODE_RAW, scorer.requests[0].GetIdentifierMode())
	assert.Equal(t, "plugin-rec-0", sent[0].GetId())
	assert.Equal(t, "i-0", sent[0].GetResource().GetId())
	assert.Equal(t, "web-server", sent[0].GetResource().GetName())
}

func TestScore_OmittedModeRemovesIdentifiers(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{IdentifierMode: config.ScoringIdentifierOmitted})

	_, err := svc.Score(context.Background(), recs(2), false)
	require.NoError(t, err)

	assert.Equal(t, pbc.IdentifierMode_IDENTIFIER_MODE_OMITTED, scorer.requests[0].GetIdentifierMode())
	for _, r := range scorer.sentRecommendations() {
		assert.Empty(t, r.GetResource().GetId())
		assert.Empty(t, r.GetResource().GetName())
		assert.NotContains(t, r.GetDescription(), "web-server")
		assert.Contains(t, r.GetDescription(), redacted)
		assert.Equal(t, "aws", r.GetResource().GetProvider())
	}
}

func TestScore_ScrubsRawIdentifiersFromFreeText(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{})
	input := []engine.Recommendation{rec("rec-1", rawInstance)}

	_, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	sent := scorer.sentRecommendations()[0]
	assert.NotContains(t, sent.GetDescription(), rawInstance)
	assert.NotContains(t, sent.GetReasoning()[0], rawInstance)
	assert.NotContains(t, sent.GetResource().GetTags()["instance"], rawInstance)
	assert.Contains(t, sent.GetDescription(), sent.GetResource().GetId())
	assert.Equal(t, "prod", sent.GetResource().GetTags()["env"])
}

func TestScore_FieldAllowlist(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{FieldAllowlist: []string{"impact", "category"}})

	_, err := svc.Score(context.Background(), recs(1), false)
	require.NoError(t, err)

	sent := scorer.sentRecommendations()[0]
	assert.NotEmpty(t, sent.GetId())
	assert.NotNil(t, sent.GetImpact())
	assert.Equal(t, pbc.RecommendationCategory_RECOMMENDATION_CATEGORY_COST, sent.GetCategory())
	assert.Nil(t, sent.GetResource())
	assert.Empty(t, sent.GetDescription())
	assert.Empty(t, sent.GetReasoning())
	assert.Empty(t, sent.GetMetadata())
	assert.Empty(t, sent.GetSource())
	assert.Nil(t, sent.ConfidenceScore)
	assert.Equal(t, pbc.RecommendationPriority_RECOMMENDATION_PRIORITY_UNSPECIFIED, sent.GetPriority())
	assert.Equal(t, pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_UNSPECIFIED, sent.GetActionType())
}

func TestScore_AppliesScoresAndSummary(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{})
	input := recs(3)

	outcome, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	for _, r := range input {
		require.NotNil(t, r.Scores)
		require.NotNil(t, r.Scores.Risk)
		require.NotNil(t, r.Scores.Priority)
		assert.InDelta(t, 2.0, *r.Scores.Priority, 1e-9)
		assert.Nil(t, r.Scores.WorthActing, "signals the scorer did not compute stay unset")
	}
	assert.Equal(t, 3, outcome.Summary.Requested)
	assert.Equal(t, 3, outcome.Summary.Scored)
	assert.Equal(t, 0, outcome.Summary.Unscored)
	assert.Equal(t, "fake", outcome.Summary.Scorer)
	assert.Equal(t, "model-1", outcome.Summary.Model)
	assert.Equal(t, "ranking_only", outcome.Summary.Calibration)
	assert.Empty(t, outcome.Summary.Warnings)
}

func TestScore_WithMockScorerGroupsDuplicates(t *testing.T) {
	t.Parallel()

	harness := plugintesting.NewScorerHarness(plugintesting.NewMockRecommendationScorer())
	harness.Start(t)
	t.Cleanup(harness.Stop)

	input := []engine.Recommendation{
		rec("rec-1", "i-a"), rec("rec-2", "i-b"), rec("rec-3", "i-c"), rec("rec-4", "i-c"),
	}
	svc := newService(t, harness.Client(), Options{})

	outcome, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	assert.Equal(t, 4, outcome.Summary.Scored)
	require.NotNil(t, input[2].Scores)
	require.NotNil(t, input[3].Scores)
	assert.NotEmpty(t, input[2].Scores.DuplicateGroupID)
	assert.Equal(t, input[2].Scores.DuplicateGroupID, input[3].Scores.DuplicateGroupID)
	assert.Empty(t, input[0].Scores.DuplicateGroupID)
	assert.Len(t, input, 4, "duplicate groups never hide or merge recommendations")
}

func TestScore_OmittedModeDoesNotGroup(t *testing.T) {
	t.Parallel()

	harness := plugintesting.NewScorerHarness(plugintesting.NewMockRecommendationScorer())
	harness.Start(t)
	t.Cleanup(harness.Stop)

	input := []engine.Recommendation{rec("rec-1", "i-c"), rec("rec-2", "i-c")}
	svc := newService(t, harness.Client(), Options{IdentifierMode: config.ScoringIdentifierOmitted})

	_, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)
	assert.Empty(t, input[0].Scores.DuplicateGroupID)
}

func TestScore_SplitsByLearnedMaxBatchSize(t *testing.T) {
	t.Parallel()

	harness := plugintesting.NewScorerHarness(plugintesting.NewMockRecommendationScorer(
		plugintesting.WithScorerMaxBatchSize(3),
	))
	harness.Start(t)
	t.Cleanup(harness.Stop)

	input := recs(10)
	svc := newService(t, harness.Client(), Options{ProbeBatchSize: 20})

	outcome, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	assert.Equal(t, 10, outcome.Summary.Scored, "probe shrinks until the scorer accepts it, then splits the rest")
	assert.Empty(t, outcome.Summary.Warnings)
}

func TestScore_RunsAtMostEightBatchesConcurrently(t *testing.T) {
	t.Parallel()

	var inFlight, peak int32
	scorer := &fakeScorer{
		handler: func(req *pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error) {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			return okResponse(req, "model-1", 2), nil
		},
	}
	svc := newService(t, scorer, Options{ProbeBatchSize: 2})

	outcome, err := svc.Score(context.Background(), recs(60), false)
	require.NoError(t, err)

	assert.Equal(t, 60, outcome.Summary.Scored)
	assert.LessOrEqual(t, int(atomic.LoadInt32(&peak)), MaxConcurrentBatches)
	assert.Greater(t, int(atomic.LoadInt32(&peak)), 1, "batches after the probe run concurrently")
}

type blockingScorer struct{}

func (blockingScorer) ScoreRecommendations(
	ctx context.Context,
	_ *pbc.ScoreRecommendationsRequest,
	_ ...grpc.CallOption,
) (*pbc.ScoreRecommendationsResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestScore_TimeoutDegradesToUnscored(t *testing.T) {
	t.Parallel()

	svc := newService(t, blockingScorer{}, Options{Timeout: 30 * time.Millisecond})
	input := recs(2)

	start := time.Now()
	outcome, err := svc.Score(context.Background(), input, false)

	require.NoError(t, err)
	assert.Less(t, time.Since(start), 400*time.Millisecond)
	assert.Nil(t, input[0].Scores)
	assert.Equal(t, 2, outcome.Summary.Unscored)
	require.NotEmpty(t, outcome.Summary.Warnings)
	assert.Contains(t, outcome.Summary.Warnings[0], "unscored")
}

func TestScore_UnavailableScorerDegradesToUnscored(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{handler: func(*pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error) {
		return nil, status.Error(codes.Unavailable, "backend down")
	}}
	svc := newService(t, scorer, Options{})
	input := recs(3)

	outcome, err := svc.Score(context.Background(), input, false)

	require.NoError(t, err)
	assert.Len(t, input, 3)
	for _, r := range input {
		assert.Nil(t, r.Scores)
	}
	assert.Equal(t, 0, outcome.Summary.Scored)
	assert.Equal(t, 3, outcome.Summary.Unscored)
	require.Len(t, outcome.Summary.Warnings, 1)
	assert.Contains(t, outcome.Summary.Warnings[0], "Unavailable")
	assert.Equal(t, 1, scorer.calls(), "a failed probe stops further calls")
}

func TestScore_PerItemErrorsLeaveOthersScored(t *testing.T) {
	t.Parallel()

	harness := plugintesting.NewScorerHarness(plugintesting.NewMockRecommendationScorer())
	harness.Start(t)
	t.Cleanup(harness.Stop)

	bare := rec("rec-2", "")
	bare.ResourceInfo = nil
	input := []engine.Recommendation{rec("rec-1", "i-1"), bare, rec("rec-3", "i-3")}
	svc := newService(t, harness.Client(), Options{})

	outcome, err := svc.Score(context.Background(), input, false)

	require.NoError(t, err)
	assert.NotNil(t, input[0].Scores)
	assert.Nil(t, input[1].Scores)
	assert.NotNil(t, input[2].Scores)
	assert.Equal(t, 2, outcome.Summary.Scored)
	assert.Equal(t, 1, outcome.Summary.Unscored)
	require.NotEmpty(t, outcome.Summary.Warnings)
}

func TestScore_InvalidResponseIsTreatedAsFailure(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{
		handler: func(req *pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error) {
			resp := okResponse(req, "model-1", 100)
			resp.Results = resp.GetResults()[:len(resp.GetResults())-1]
			return resp, nil
		},
	}
	svc := newService(t, scorer, Options{})
	input := recs(2)

	outcome, err := svc.Score(context.Background(), input, false)

	require.NoError(t, err)
	assert.Nil(t, input[0].Scores)
	assert.Equal(t, 2, outcome.Summary.Unscored)
	require.NotEmpty(t, outcome.Summary.Warnings)
	assert.Contains(t, outcome.Summary.Warnings[0], "invalid")
}

func TestScore_SkipsDismissedRecommendationsAndNeverChangesStatus(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{})
	input := recs(3)
	input[1].Status = engine.RecommendationStatusDismissed
	input[2].Status = engine.RecommendationStatusSnoozed

	outcome, err := svc.Score(context.Background(), input, false)

	require.NoError(t, err)
	assert.Len(t, scorer.sentRecommendations(), 1)
	assert.Equal(t, 1, outcome.Summary.Requested)
	assert.Nil(t, input[1].Scores)
	assert.Equal(t, engine.RecommendationStatusActive, input[0].Status)
	assert.Equal(t, engine.RecommendationStatusDismissed, input[1].Status)
	assert.Equal(t, engine.RecommendationStatusSnoozed, input[2].Status)
}

func TestScore_MissingAndDuplicateIDsStayUnscored(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{})
	input := []engine.Recommendation{rec("", "i-1"), rec("same", "i-2"), rec("same", "i-3"), rec("ok", "i-4")}

	outcome, err := svc.Score(context.Background(), input, false)

	require.NoError(t, err)
	assert.Nil(t, input[0].Scores)
	assert.NotNil(t, input[1].Scores)
	assert.Nil(t, input[2].Scores)
	assert.NotNil(t, input[3].Scores)
	assert.Equal(t, 4, outcome.Summary.Requested)
	assert.Equal(t, 2, outcome.Summary.Unscored)
	require.NotEmpty(t, outcome.Summary.Warnings)
}

func TestScore_CacheServesRepeatRunsWithoutCallingScorer(t *testing.T) {
	t.Parallel()

	store := newMemCache()
	scorer := &fakeScorer{}
	opts := Options{Cache: store, PluginVersion: "1.0.0"}

	first := recs(4)
	_, err := newService(t, scorer, opts).Score(context.Background(), first, false)
	require.NoError(t, err)
	require.Equal(t, 1, scorer.calls())

	second := recs(4)
	outcome, err := newService(t, scorer, opts).Score(context.Background(), second, false)
	require.NoError(t, err)

	assert.Equal(t, 1, scorer.calls(), "second run must be served from the cache")
	assert.Equal(t, 4, outcome.Summary.FromCache)
	assert.Equal(t, 4, outcome.Summary.Scored)
	require.NotNil(t, second[0].Scores)
	require.NotNil(t, first[0].Scores.Risk)
	assert.InDelta(t, *first[0].Scores.Risk, *second[0].Scores.Risk, 1e-9)
	assert.Equal(t, "model-1", outcome.Summary.Model)
}

func TestScore_CacheKeyedOnScorerVersionAndContent(t *testing.T) {
	t.Parallel()

	store := newMemCache()
	scorer := &fakeScorer{}

	_, err := newService(t, scorer, Options{Cache: store, PluginVersion: "1.0.0"}).
		Score(context.Background(), recs(2), false)
	require.NoError(t, err)

	_, err = newService(t, scorer, Options{Cache: store, PluginVersion: "2.0.0"}).
		Score(context.Background(), recs(2), false)
	require.NoError(t, err)
	assert.Equal(t, 2, scorer.calls(), "a new plugin version must not reuse cached scores")

	changed := recs(2)
	changed[0].Description = "a different recommendation"
	_, err = newService(t, scorer, Options{Cache: store, PluginVersion: "2.0.0"}).
		Score(context.Background(), changed, false)
	require.NoError(t, err)
	assert.Equal(t, 3, scorer.calls(), "changed content is rescored")
	assert.Len(t, scorer.requests[2].GetRecommendations(), 1, "only the changed recommendation is sent")
}

func TestScore_CacheStoresOnlyScoreValues(t *testing.T) {
	t.Parallel()

	store := newMemCache()
	scorer := &fakeScorer{}

	_, err := newService(t, scorer, Options{Cache: store, PluginVersion: "1.0.0"}).
		Score(context.Background(), recs(2), false)
	require.NoError(t, err)

	require.NotEmpty(t, store.entries)
	for key, data := range store.entries {
		assert.True(t, strings.HasPrefix(key, cache.BucketScores+"/"), key)
		text := string(data)
		assert.NotContains(t, text, "Downsize")
		assert.NotContains(t, text, "i-0")
		assert.NotContains(t, text, "web-server")
		assert.NotContains(t, text, "provider_request_id")
	}
}

func TestScore_DryRunSendsNothingAndReportsRequests(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{ProbeBatchSize: 2})

	outcome, err := svc.Score(context.Background(), recs(5), true)

	require.NoError(t, err)
	assert.Equal(t, 0, scorer.calls())
	require.Len(t, outcome.Requests, 3)
	total := 0
	for _, req := range outcome.Requests {
		total += len(req.GetRecommendations())
		blob, marshalErr := protojson.Marshal(req)
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(blob), "i-0")
		assert.NotContains(t, string(blob), "web-server")
	}
	assert.Equal(t, 5, total)
	assert.Equal(t, 0, outcome.Summary.Scored)
}

func TestScore_DryRunOmitsCachedRecommendations(t *testing.T) {
	t.Parallel()

	store := newMemCache()
	scorer := &fakeScorer{}
	opts := Options{Cache: store, PluginVersion: "1.0.0"}

	_, err := newService(t, scorer, opts).Score(context.Background(), recs(2), false)
	require.NoError(t, err)

	more := recs(3)
	outcome, err := newService(t, scorer, opts).Score(context.Background(), more, true)
	require.NoError(t, err)

	require.Len(t, outcome.Requests, 1)
	assert.Len(t, outcome.Requests[0].GetRecommendations(), 1, "cached recommendations would not be sent")
	assert.Equal(t, 2, outcome.Summary.FromCache)
}

func TestScore_MarksNeedsReview(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{
		handler: func(req *pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error) {
			resp := okResponse(req, "model-1", 100)
			values := []float64{0.1, 0.26, 0.9}
			for i, res := range resp.GetResults() {
				res.GetScores().Risk = ptr(values[i])
			}
			return resp, nil
		},
	}
	policy := ReviewPolicy{Risk: 0.3, FalsePositive: 0.3, InsufficientEvidence: 0.5, DeadBand: 0.1}
	svc := newService(t, scorer, Options{Review: policy})
	input := recs(3)

	_, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	assert.False(t, input[0].Scores.NeedsReview)
	assert.True(t, input[1].Scores.NeedsReview, "inside the dead band still goes to review")
	assert.True(t, input[2].Scores.NeedsReview)
}

func TestReviewPolicy_NeedsReview(t *testing.T) {
	t.Parallel()

	p := ReviewPolicy{Risk: 0.3, FalsePositive: 0.4, InsufficientEvidence: 0.5, DeadBand: 0.1}

	assert.False(t, p.NeedsReview(nil))
	assert.False(t, p.NeedsReview(&engine.RecommendationScores{}))
	assert.False(t, p.NeedsReview(&engine.RecommendationScores{Risk: ptr(0.24)}))
	assert.True(t, p.NeedsReview(&engine.RecommendationScores{Risk: ptr(0.25)}))
	assert.True(t, p.NeedsReview(&engine.RecommendationScores{FalsePositive: ptr(0.35)}))
	assert.True(t, p.NeedsReview(&engine.RecommendationScores{InsufficientEvidence: ptr(0.9)}))
	assert.False(t, p.NeedsReview(&engine.RecommendationScores{WorthActing: ptr(0.99), Priority: ptr(3)}))
}

func TestScore_NamespacesDuplicateGroupsAcrossBatches(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{
		handler: func(req *pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error) {
			resp := okResponse(req, "model-1", 2)
			for _, res := range resp.GetResults() {
				res.GetScores().DuplicateGroupId = "g1"
			}
			return resp, nil
		},
	}
	svc := newService(t, scorer, Options{ProbeBatchSize: 2})
	input := recs(4)

	_, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	assert.Equal(t, input[0].Scores.DuplicateGroupID, input[1].Scores.DuplicateGroupID)
	assert.Equal(t, input[2].Scores.DuplicateGroupID, input[3].Scores.DuplicateGroupID)
	assert.NotEqual(t, input[0].Scores.DuplicateGroupID, input[2].Scores.DuplicateGroupID,
		"group ids from different responses must not collide")
}

func TestScore_EmptyInputCallsNothing(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	outcome, err := newService(t, scorer, Options{}).Score(context.Background(), nil, false)

	require.NoError(t, err)
	assert.Equal(t, 0, scorer.calls())
	assert.Equal(t, 0, outcome.Summary.Requested)
}

func TestOptionsFromConfig(t *testing.T) {
	t.Parallel()

	cfg := config.ResolvedScoring{
		Plugin: "scorer", IdentifierMode: config.ScoringIdentifierOmitted, FieldAllowlist: []string{"impact"},
		TimeoutSeconds: 7, RiskThreshold: 0.4, FalsePositiveLimit: 0.5, InsufficientEvidence: 0.6, DeadBand: 0.2,
	}

	opts := OptionsFromConfig(cfg, nil, "1.2.3")

	assert.Equal(t, "scorer", opts.ScorerName)
	assert.Equal(t, "1.2.3", opts.PluginVersion)
	assert.Equal(t, 7*time.Second, opts.Timeout)
	assert.Equal(t, config.ScoringIdentifierOmitted, opts.IdentifierMode)
	assert.Equal(t, []string{"impact"}, opts.FieldAllowlist)
	assert.Equal(t, ReviewPolicy{Risk: 0.4, FalsePositive: 0.5, InsufficientEvidence: 0.6, DeadBand: 0.2}, opts.Review)
}

func TestScore_ReportsCalibrationFromScorer(t *testing.T) {
	t.Parallel()

	tests := map[pbc.ScoreCalibration]string{
		pbc.ScoreCalibration_SCORE_CALIBRATION_PROBABILITY:  "probability",
		pbc.ScoreCalibration_SCORE_CALIBRATION_RANKING_ONLY: "ranking_only",
		pbc.ScoreCalibration_SCORE_CALIBRATION_UNSPECIFIED:  "unspecified",
	}
	for calibration, want := range tests {
		scorer := &fakeScorer{
			handler: func(req *pbc.ScoreRecommendationsRequest) (*pbc.ScoreRecommendationsResponse, error) {
				resp := okResponse(req, "m", 10)
				resp.Scorer.Calibration = calibration
				return resp, nil
			},
		}

		outcome, err := newService(t, scorer, Options{}).Score(context.Background(), recs(1), false)

		require.NoError(t, err)
		assert.Equal(t, want, outcome.Summary.Calibration)
	}
}

func TestScore_IgnoresCorruptCacheEntries(t *testing.T) {
	t.Parallel()

	store := newMemCache()
	scorer := &fakeScorer{}
	opts := Options{Cache: store, PluginVersion: "1.0.0"}

	_, err := newService(t, scorer, opts).Score(context.Background(), recs(1), false)
	require.NoError(t, err)
	for key := range store.entries {
		if !strings.HasSuffix(key, "/_model") {
			store.entries[key] = json.RawMessage(`{not json`)
		}
	}

	outcome, err := newService(t, scorer, opts).Score(context.Background(), recs(1), false)

	require.NoError(t, err)
	assert.Equal(t, 2, scorer.calls(), "a corrupt entry is treated as a miss")
	assert.Equal(t, 0, outcome.Summary.FromCache)
	assert.Equal(t, 1, outcome.Summary.Scored)
}

func TestToProto_CarriesFullRecordAndSkipsMissingID(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	full := rec("plugin-1", "i-1")
	full.CreatedAt = &created
	full.ImpactDetail.ImplementationCost = ptr(5)
	full.ImpactDetail.MigrationEffortHours = ptr(2)
	full.ResourceInfo.SKU = "m5.large"
	full.ResourceInfo.Utilization = &engine.RecommendationUtilizationInfo{
		CPUPercent: 10, MemoryPercent: 20, StoragePercent: 30, NetworkInMbps: 1, NetworkOutMbps: 2,
		CustomMetrics: map[string]float64{"iops": 5},
	}
	full.PrimaryReason = "RECOMMENDATION_REASON_OVER_PROVISIONED"
	full.SecondaryReasons = []string{"RECOMMENDATION_REASON_IDLE"}
	full.ActionDetail = &engine.RecommendationActionDetail{Rightsize: &engine.RightsizeActionDetail{
		CurrentSKU:     "m5.large",
		RecommendedSKU: "t3.large",
	}}
	ids, err := newIdentifiers(config.ScoringIdentifierRaw)
	require.NoError(t, err)

	got, ok := toProto(full, ids)

	require.True(t, ok)
	assert.Equal(t, created.Unix(), got.GetCreatedAt().AsTime().Unix())
	assert.InDelta(t, 5.0, got.GetImpact().GetImplementationCost(), 1e-9)
	assert.InDelta(t, 2.0, got.GetImpact().GetMigrationEffortHours(), 1e-9)
	assert.Equal(t, "m5.large", got.GetResource().GetSku())
	assert.InDelta(t, 20.0, got.GetResource().GetUtilization().GetMemoryPercent(), 1e-9)
	assert.Equal(t, map[string]float64{"iops": 5}, got.GetResource().GetUtilization().GetCustomMetrics())
	assert.Equal(t, pbc.RecommendationReason_RECOMMENDATION_REASON_OVER_PROVISIONED, got.GetPrimaryReason())
	assert.Equal(t,
		[]pbc.RecommendationReason{pbc.RecommendationReason_RECOMMENDATION_REASON_IDLE},
		got.GetSecondaryReasons())
	require.NotNil(t, got.GetRightsize())
	assert.Equal(t, "m5.large", got.GetRightsize().GetCurrentSku())
	assert.Equal(t, "t3.large", got.GetRightsize().GetRecommendedSku())

	_, ok = toProto(engine.Recommendation{}, ids)
	assert.False(t, ok)

	bare, ok := toProto(engine.Recommendation{ID: "x"}, ids)
	require.True(t, ok)
	assert.Nil(t, bare.GetImpact())
	assert.Nil(t, bare.GetResource())
	assert.Nil(t, bare.GetActionDetail())
	assert.Equal(t, pbc.RecommendationReason_RECOMMENDATION_REASON_UNSPECIFIED, bare.GetPrimaryReason())
	assert.Empty(t, bare.GetSecondaryReasons())
}

//nolint:paralleltest // subtests share the parent-scoped fixture full = rec(...)
func TestApplyAllowlist_KeepsOnlyListedFields(t *testing.T) {
	ids, err := newIdentifiers(config.ScoringIdentifierRaw)
	require.NoError(t, err)
	full := rec("plugin-1", "i-1")
	created := time.Now()
	full.CreatedAt = &created
	full.PrimaryReason = "RECOMMENDATION_REASON_OVER_PROVISIONED"
	full.SecondaryReasons = []string{"RECOMMENDATION_REASON_IDLE"}
	full.ActionDetail = &engine.RecommendationActionDetail{Terminate: &engine.TerminateActionDetail{
		TerminationReason: "idle",
		IdleDays:          30,
	}}

	t.Run("unlisted fields are cleared", func(t *testing.T) {
		all, _ := toProto(full, ids)
		applyAllowlist(all, []string{"created_at", "metadata", "reasoning", "priority", "source", "confidence_score"})

		assert.NotNil(t, all.GetCreatedAt())
		assert.NotEmpty(t, all.GetMetadata())
		assert.NotEmpty(t, all.GetReasoning())
		assert.NotEmpty(t, all.GetSource())
		assert.NotNil(t, all.ConfidenceScore)
		assert.NotEqual(t, pbc.RecommendationPriority_RECOMMENDATION_PRIORITY_UNSPECIFIED, all.GetPriority())
		assert.Nil(t, all.GetImpact())
		assert.Nil(t, all.GetResource())
		assert.Empty(t, all.GetDescription())
		assert.Nil(t, all.GetActionDetail())
		assert.Equal(t, pbc.RecommendationReason_RECOMMENDATION_REASON_UNSPECIFIED, all.GetPrimaryReason())
		assert.Empty(t, all.GetSecondaryReasons())
		assert.NotEmpty(t, all.GetId(), "id survives the allowlist")
	})

	t.Run("listed fields are kept", func(t *testing.T) {
		all, _ := toProto(full, ids)
		applyAllowlist(all, []string{"action_detail", "primary_reason", "secondary_reasons"})

		require.NotNil(t, all.GetTerminate())
		assert.Equal(t, "idle", all.GetTerminate().GetTerminationReason())
		assert.Equal(t, pbc.RecommendationReason_RECOMMENDATION_REASON_OVER_PROVISIONED, all.GetPrimaryReason())
		assert.Equal(t,
			[]pbc.RecommendationReason{pbc.RecommendationReason_RECOMMENDATION_REASON_IDLE},
			all.GetSecondaryReasons())
		assert.Nil(t, all.GetImpact())
		assert.Empty(t, all.GetDescription())
	})
}

func k8sActionDetail() *engine.RecommendationActionDetail {
	return &engine.RecommendationActionDetail{Kubernetes: &engine.KubernetesActionDetail{
		ClusterID:           "prod-cluster",
		Namespace:           "payments",
		ControllerKind:      "Deployment",
		ControllerName:      "web-server",
		ContainerName:       "app",
		CurrentRequests:     &engine.KubernetesResourceValues{CPU: "500m", Memory: "256Mi"},
		RecommendedRequests: &engine.KubernetesResourceValues{CPU: "250m", Memory: "128Mi"},
		Algorithm:           "vpa",
	}}
}

// TestScore_SendsActionDetailAndReasons builds a scoring request from a recommendation
// carrying action_detail, primary_reason and secondary_reasons and asserts all three
// reach the scorer.
func TestScore_SendsActionDetailAndReasons(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{IdentifierMode: config.ScoringIdentifierRaw})

	input := []engine.Recommendation{rec("plugin-rec-0", "i-0")}
	input[0].PrimaryReason = "RECOMMENDATION_REASON_OVER_PROVISIONED"
	input[0].SecondaryReasons = []string{"RECOMMENDATION_REASON_IDLE", "RECOMMENDATION_REASON_REDUNDANT"}
	input[0].ActionDetail = k8sActionDetail()

	_, err := svc.Score(context.Background(), input, false)
	require.NoError(t, err)

	sent := scorer.sentRecommendations()
	require.Len(t, sent, 1)
	assert.Equal(t, pbc.RecommendationReason_RECOMMENDATION_REASON_OVER_PROVISIONED, sent[0].GetPrimaryReason())
	assert.Equal(t,
		[]pbc.RecommendationReason{
			pbc.RecommendationReason_RECOMMENDATION_REASON_IDLE,
			pbc.RecommendationReason_RECOMMENDATION_REASON_REDUNDANT,
		},
		sent[0].GetSecondaryReasons())

	k8s := sent[0].GetKubernetes()
	require.NotNil(t, k8s, "action_detail must reach the scorer")
	assert.Equal(t, "prod-cluster", k8s.GetClusterId())
	assert.Equal(t, "payments", k8s.GetNamespace())
	assert.Equal(t, "Deployment", k8s.GetControllerKind())
	assert.Equal(t, "web-server", k8s.GetControllerName())
	assert.Equal(t, "app", k8s.GetContainerName())
	assert.Equal(t, "500m", k8s.GetCurrentRequests().GetCpu())
	assert.Equal(t, "128Mi", k8s.GetRecommendedRequests().GetMemory())
	assert.Equal(t, "vpa", k8s.GetAlgorithm())
}

// TestScore_ActionDetailIdentifierModes proves the identifier mode reaches identifiers
// inside action_detail: Kubernetes cluster and workload names, and free text such as a
// termination reason.
func TestScore_ActionDetailIdentifierModes(t *testing.T) {
	t.Parallel()

	newInput := func() []engine.Recommendation {
		out := []engine.Recommendation{
			rec("plugin-rec-0", rawInstance), rec("plugin-rec-1", "i-1"), rec("plugin-rec-2", "i-2"),
		}
		out[0].ActionDetail = k8sActionDetail()
		out[1].ResourceID = rawInstance
		out[1].ActionDetail = &engine.RecommendationActionDetail{Terminate: &engine.TerminateActionDetail{
			TerminationReason: "instance " + rawInstance + " is idle",
			IdleDays:          45,
		}}
		// out[2] carries no action detail: the absent case.
		return out
	}

	t.Run("pseudonymized", func(t *testing.T) {
		t.Parallel()
		scorer := &fakeScorer{}
		svc := newService(t, scorer, Options{IdentifierMode: config.ScoringIdentifierPseudonymized})

		_, err := svc.Score(context.Background(), newInput(), false)
		require.NoError(t, err)

		sent := scorer.sentRecommendations()
		require.Len(t, sent, 3)

		k8s := sent[0].GetKubernetes()
		require.NotNil(t, k8s)
		assert.True(t, strings.HasPrefix(k8s.GetClusterId(), "res-"), "cluster id is pseudonymized")
		assert.NotEqual(t, "prod-cluster", k8s.GetClusterId())
		for _, name := range []string{k8s.GetNamespace(), k8s.GetControllerName(), k8s.GetContainerName()} {
			assert.True(t, strings.HasPrefix(name, "name-"), "%q is pseudonymized", name)
		}
		assert.Equal(t, sent[0].GetResource().GetName(), k8s.GetControllerName(),
			"a workload named after the resource gets the same token")
		assert.Equal(t, "Deployment", k8s.GetControllerKind())
		assert.Equal(t, "vpa", k8s.GetAlgorithm())

		term := sent[1].GetTerminate()
		require.NotNil(t, term)
		assert.NotContains(t, term.GetTerminationReason(), rawInstance)
		assert.Equal(t, int32(45), term.GetIdleDays())

		blob, marshalErr := protojson.Marshal(scorer.requests[0])
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(blob), "prod-cluster")
		assert.NotContains(t, string(blob), "payments")

		assert.Nil(t, sent[2].GetActionDetail(), "absent action detail stays absent")
	})

	t.Run("omitted", func(t *testing.T) {
		t.Parallel()
		scorer := &fakeScorer{}
		svc := newService(t, scorer, Options{IdentifierMode: config.ScoringIdentifierOmitted})

		_, err := svc.Score(context.Background(), newInput(), false)
		require.NoError(t, err)

		sent := scorer.sentRecommendations()
		require.Len(t, sent, 3)

		k8s := sent[0].GetKubernetes()
		require.NotNil(t, k8s)
		assert.Empty(t, k8s.GetClusterId())
		assert.Empty(t, k8s.GetNamespace())
		assert.Empty(t, k8s.GetControllerName())
		assert.Empty(t, k8s.GetContainerName())
		assert.Equal(t, "Deployment", k8s.GetControllerKind(), "non-identifier fields survive")

		term := sent[1].GetTerminate()
		require.NotNil(t, term)
		assert.NotContains(t, term.GetTerminationReason(), rawInstance)
		assert.Contains(t, term.GetTerminationReason(), redacted)
	})

	t.Run("raw", func(t *testing.T) {
		t.Parallel()
		scorer := &fakeScorer{}
		svc := newService(t, scorer, Options{IdentifierMode: config.ScoringIdentifierRaw})

		_, err := svc.Score(context.Background(), newInput(), false)
		require.NoError(t, err)

		sent := scorer.sentRecommendations()
		require.Len(t, sent, 3)

		k8s := sent[0].GetKubernetes()
		require.NotNil(t, k8s)
		assert.Equal(t, "prod-cluster", k8s.GetClusterId())
		assert.Equal(t, "payments", k8s.GetNamespace())
		assert.Equal(t, "web-server", k8s.GetControllerName())
		assert.Equal(t, "app", k8s.GetContainerName())

		term := sent[1].GetTerminate()
		require.NotNil(t, term)
		assert.Contains(t, term.GetTerminationReason(), rawInstance, "raw mode keeps identifiers verbatim")
	})
}

// TestScore_DryRunRequestsCarryActionDetailAndReasons proves the dry-run request output
// includes the three fields after identifier handling, without sending anything.
func TestScore_DryRunRequestsCarryActionDetailAndReasons(t *testing.T) {
	t.Parallel()

	scorer := &fakeScorer{}
	svc := newService(t, scorer, Options{})

	input := []engine.Recommendation{rec("plugin-rec-0", "i-0")}
	input[0].PrimaryReason = "RECOMMENDATION_REASON_OVER_PROVISIONED"
	input[0].SecondaryReasons = []string{"RECOMMENDATION_REASON_IDLE"}
	input[0].ActionDetail = k8sActionDetail()

	outcome, err := svc.Score(context.Background(), input, true)
	require.NoError(t, err)
	assert.Equal(t, 0, scorer.calls())

	require.Len(t, outcome.Requests, 1)
	blob, marshalErr := protojson.Marshal(outcome.Requests[0])
	require.NoError(t, marshalErr)
	text := string(blob)
	assert.Contains(t, text, "primaryReason")
	assert.Contains(t, text, "OVER_PROVISIONED")
	assert.Contains(t, text, "secondaryReasons")
	assert.Contains(t, text, "kubernetes", "protojson emits the set action_detail variant")
	assert.NotContains(t, text, "prod-cluster", "dry-run output is already pseudonymized")
}

// TestScore_ActionDetailVariantsRoundTrip covers the remaining action_detail variants
// and the empty-string identifier case through toProto.
func TestScore_ActionDetailVariantsRoundTrip(t *testing.T) {
	t.Parallel()

	ids, err := newIdentifiers(config.ScoringIdentifierRaw)
	require.NoError(t, err)

	tests := []struct {
		name string
		rec  engine.Recommendation
		want func(t *testing.T, got *pbc.Recommendation)
	}{
		{
			name: "rightsize with projected utilization",
			rec: engine.Recommendation{
				ID: "r1",
				ActionDetail: &engine.RecommendationActionDetail{Rightsize: &engine.RightsizeActionDetail{
					CurrentSKU:              "m5.large",
					RecommendedSKU:          "t3.large",
					CurrentInstanceType:     "m5.large",
					RecommendedInstanceType: "t3.large",
					ProjectedUtilization:    &engine.RecommendationUtilizationInfo{CPUPercent: 55},
				}},
			},
			want: func(t *testing.T, got *pbc.Recommendation) {
				t.Helper()
				rs := got.GetRightsize()
				require.NotNil(t, rs)
				assert.Equal(t, "m5.large", rs.GetCurrentSku())
				assert.Equal(t, "t3.large", rs.GetRecommendedInstanceType())
				require.NotNil(t, rs.GetProjectedUtilization())
				assert.InDelta(t, 55.0, rs.GetProjectedUtilization().GetCpuPercent(), 1e-9)
			},
		},
		{
			name: "commitment",
			rec: engine.Recommendation{
				ID: "r2",
				ActionDetail: &engine.RecommendationActionDetail{Commitment: &engine.CommitmentActionDetail{
					CommitmentType:      "savings_plan",
					Term:                "1_year",
					PaymentOption:       "no_upfront",
					RecommendedQuantity: 2,
					Scope:               "region",
				}},
			},
			want: func(t *testing.T, got *pbc.Recommendation) {
				t.Helper()
				c := got.GetCommitment()
				require.NotNil(t, c)
				assert.Equal(t, "savings_plan", c.GetCommitmentType())
				assert.Equal(t, "1_year", c.GetTerm())
				assert.Equal(t, "no_upfront", c.GetPaymentOption())
				assert.InDelta(t, 2.0, c.GetRecommendedQuantity(), 1e-9)
				assert.Equal(t, "region", c.GetScope())
			},
		},
		{
			name: "modify scrubs raw identifiers from config values",
			rec: engine.Recommendation{
				ID:         "r3",
				ResourceID: rawInstance,
				ActionDetail: &engine.RecommendationActionDetail{Modify: &engine.ModifyActionDetail{
					ModificationType:  "storage_class",
					CurrentConfig:     map[string]string{"volume": rawInstance},
					RecommendedConfig: map[string]string{"class": "gp3"},
				}},
			},
			want: func(t *testing.T, got *pbc.Recommendation) {
				t.Helper()
				m := got.GetModify()
				require.NotNil(t, m)
				assert.Equal(t, "storage_class", m.GetModificationType())
				assert.Equal(t, map[string]string{"class": "gp3"}, m.GetRecommendedConfig())
			},
		},
		{
			name: "kubernetes with empty identifiers",
			rec: engine.Recommendation{
				ID:           "r4",
				ActionDetail: &engine.RecommendationActionDetail{Kubernetes: &engine.KubernetesActionDetail{}},
			},
			want: func(t *testing.T, got *pbc.Recommendation) {
				t.Helper()
				k := got.GetKubernetes()
				require.NotNil(t, k)
				assert.Empty(t, k.GetClusterId())
				assert.Nil(t, k.GetCurrentRequests())
			},
		},
		{
			name: "unknown reason names become unspecified",
			rec: engine.Recommendation{
				ID:               "r5",
				PrimaryReason:    "NOT_A_REASON",
				SecondaryReasons: []string{"ALSO_NOT_A_REASON"},
			},
			want: func(t *testing.T, got *pbc.Recommendation) {
				t.Helper()
				assert.Equal(t, pbc.RecommendationReason_RECOMMENDATION_REASON_UNSPECIFIED, got.GetPrimaryReason())
				assert.Equal(t,
					[]pbc.RecommendationReason{pbc.RecommendationReason_RECOMMENDATION_REASON_UNSPECIFIED},
					got.GetSecondaryReasons())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toProto(tt.rec, ids)
			require.True(t, ok)
			tt.want(t, got)
		})
	}

	t.Run("pseudonymized modify config is scrubbed", func(t *testing.T) {
		t.Parallel()
		pseudo, perr := newIdentifiers(config.ScoringIdentifierPseudonymized)
		require.NoError(t, perr)
		got, ok := toProto(engine.Recommendation{
			ID:         "r6",
			ResourceID: rawInstance,
			ActionDetail: &engine.RecommendationActionDetail{Modify: &engine.ModifyActionDetail{
				CurrentConfig: map[string]string{"volume": rawInstance},
			}},
		}, pseudo)
		require.True(t, ok)
		assert.NotContains(t, got.GetModify().GetCurrentConfig()["volume"], rawInstance)
	})
}
