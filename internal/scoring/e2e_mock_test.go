package scoring

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	"github.com/rshade/finfocus/internal/engine"
)

// recordingMock wraps the spec's reference scorer so tests can inspect exactly what a
// real scorer receives over gRPC.
type recordingMock struct {
	inner *plugintesting.MockRecommendationScorer

	mu       sync.Mutex
	received []*pbc.ScoreRecommendationsRequest
}

func (r *recordingMock) ScoreRecommendations(
	ctx context.Context,
	req *pbc.ScoreRecommendationsRequest,
) (*pbc.ScoreRecommendationsResponse, error) {
	r.mu.Lock()
	r.received = append(r.received, req)
	r.mu.Unlock()
	return r.inner.ScoreRecommendations(ctx, req)
}

func startMock(t *testing.T, opts ...plugintesting.MockScorerOption) (*recordingMock, *plugintesting.ScorerHarness) {
	t.Helper()
	mock := &recordingMock{inner: plugintesting.NewMockRecommendationScorer(opts...)}
	harness := plugintesting.NewScorerHarness(mock)
	harness.Start(t)
	t.Cleanup(harness.Stop)
	return mock, harness
}

func TestE2E_MockScorerContract(t *testing.T) {
	const rawIDPrefix = "i-secret-"

	build := func(n int) []engine.Recommendation {
		out := make([]engine.Recommendation, n)
		for i := range n {
			out[i] = rec("plugin-"+strconv.Itoa(i), rawIDPrefix+strconv.Itoa(i))
			out[i].EstimatedSavings = float64(10 + i)
		}
		return out
	}

	t.Run("mock scorer never receives raw resource ids or names", func(t *testing.T) {
		mock, harness := startMock(t)
		recs := build(4)

		outcome, err := newService(t, harness.Client(), Options{}).Score(context.Background(), recs, false)

		require.NoError(t, err)
		assert.Equal(t, 4, outcome.Summary.Scored)
		require.NotEmpty(t, mock.received)
		for _, req := range mock.received {
			blob, marshalErr := protojson.Marshal(req)
			require.NoError(t, marshalErr)
			assert.NotContains(t, string(blob), rawIDPrefix)
			assert.NotContains(t, string(blob), "web-server")
			assert.NotContains(t, string(blob), "plugin-")
			assert.Equal(t, pbc.IdentifierMode_IDENTIFIER_MODE_PSEUDONYMIZED, req.GetIdentifierMode())
		}
		for _, r := range recs {
			require.NotNil(t, r.Scores)
		}
	})

	t.Run("batches respect the scorer max_batch_size once it is known", func(t *testing.T) {
		mock, harness := startMock(t, plugintesting.WithScorerMaxBatchSize(3))
		recs := build(11)

		outcome, err := newService(t, harness.Client(), Options{}).Score(context.Background(), recs, false)

		require.NoError(t, err)
		assert.Equal(t, 11, outcome.Summary.Scored)
		sent := 0
		for _, req := range mock.received {
			if n := len(req.GetRecommendations()); n <= 3 {
				sent += n
			}
		}
		assert.Equal(t, 11, sent, "accepted batches (at most 3) cover every recommendation exactly once")
		last := mock.received[len(mock.received)-1]
		assert.LessOrEqual(t, len(last.GetRecommendations()), 3, "once max_batch_size is learned, batches respect it")
	})

	t.Run("a failing item degrades only that recommendation", func(t *testing.T) {
		_, harness := startMock(t)
		recs := build(3)
		recs[1].ResourceID = ""
		recs[1].ResourceInfo = nil

		outcome, err := newService(t, harness.Client(), Options{}).Score(context.Background(), recs, false)

		require.NoError(t, err)
		assert.NotNil(t, recs[0].Scores)
		assert.Nil(t, recs[1].Scores)
		assert.NotNil(t, recs[2].Scores)
		assert.Equal(t, 2, outcome.Summary.Scored)
		assert.Equal(t, 1, outcome.Summary.Unscored)
		assert.NotEmpty(t, outcome.Summary.Warnings)
		assert.Len(t, recs, 3, "no recommendation is dropped")
	})

	t.Run("an unavailable scorer leaves recommendations unscored with a warning", func(t *testing.T) {
		_, harness := startMock(t)
		harness.Stop()
		recs := build(3)

		outcome, err := newService(t, harness.Client(), Options{}).Score(context.Background(), recs, false)

		require.NoError(t, err)
		for _, r := range recs {
			assert.Nil(t, r.Scores)
		}
		assert.Equal(t, 3, outcome.Summary.Unscored)
		require.Len(t, outcome.Summary.Warnings, 1)
		assert.Contains(t, outcome.Summary.Warnings[0], "scorer call failed")
	})
}
