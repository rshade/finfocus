package scoring

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func dupRecs() []*pbc.Recommendation {
	return []*pbc.Recommendation{
		makeRec("a1", "res-a"),
		makeRec("b1", "res-b"),
		makeRec("a2", "res-a"),
		makeRec("c1", "res-c"),
		makeRec("a3", "res-a"),
	}
}

func sameFor(pairs ...string) func(jevapi.Request, int) (*jevapi.Response, error) {
	return func(req jevapi.Request, n int) (*jevapi.Response, error) {
		resp := defaultResponse(req, n)
		state, _ := req.State.([]any)
		for pos, item := range state {
			p, _ := item.(map[string]any)
			a, _ := p["a"].(map[string]any)
			b, _ := p["b"].(map[string]any)
			key := descOf(a) + "+" + descOf(b)
			for _, want := range pairs {
				if want == key {
					resp.Answers[pairQuestionName(pos)] = noulAnswer(0.9)
				}
			}
		}
		return resp, nil
	}
}

func descOf(m map[string]any) string {
	d, _ := m["description"].(string)
	return strings.TrimPrefix(d, "Downsize ")
}

func TestDuplicateBlocks(t *testing.T) {
	blocks := duplicateBlocks(dupRecs(), 10)
	assert.Equal(t, [][]int{{0, 2, 4}}, blocks)
	assert.Equal(t, []pair{{0, 2}, {0, 4}, {2, 4}}, blockPairs(blocks))
}

func TestDuplicateBlocks_CapAndEmptyIDs(t *testing.T) {
	recs := []*pbc.Recommendation{
		makeRec("1", ""),
		makeRec("2", ""),
		makeRec("3", "x"),
		makeRec("4", "x"),
		makeRec("5", "x"),
	}
	assert.Equal(t, [][]int{{2, 3}}, duplicateBlocks(recs, 2), "block capped, empty ids never grouped")
}

func TestDuplicateGroups_ConnectedComponents(t *testing.T) {
	pairs := []pair{{0, 2}, {0, 4}, {2, 4}, {5, 6}}
	groups := duplicateGroups(8, pairs, []bool{true, false, false, true})
	assert.Equal(t, map[int]string{0: "dup-1", 2: "dup-1", 5: "dup-2", 6: "dup-2"}, groups)

	transitive := duplicateGroups(5, []pair{{0, 2}, {2, 4}}, []bool{true, true})
	assert.Equal(t, map[int]string{0: "dup-1", 2: "dup-1", 4: "dup-1"}, transitive)

	assert.Empty(t, duplicateGroups(3, []pair{{0, 1}}, []bool{false}))
}

func TestScore_DuplicateGroupingBlocksByResourceAndUsesThreshold(t *testing.T) {
	fb := &fakeBackend{respond: sameFor("a1+a2", "a2+a3")}
	req := &pbc.ScoreRecommendationsRequest{
		Recommendations: dupRecs(),
		Signals:         []pbc.ScoreSignal{pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP},
	}
	resp := score(t, newScorer(fb, nil), req)

	ids := make([]string, 0, len(resp.GetResults()))
	for _, r := range resp.GetResults() {
		ids = append(ids, resultScores(r).GetDuplicateGroupId())
	}
	assert.Equal(t, []string{"dup-1", "", "dup-1", "", "dup-1"}, ids, "a1, a2 and a3 chain into one group")

	reqs := fb.recorded()
	require.Len(t, reqs, 1, "only the pair batch is sent")
	assert.Len(t, reqs[0].Questions, 3, "three pairs inside the res-a block; other resources are never compared")
	for name := range reqs[0].Questions {
		assert.True(t, strings.HasPrefix(name, "duplicate:"))
	}
	assert.Nil(t, resultScores(resp.GetResults()[0]).Risk)
}

func TestScore_DuplicateThresholdIsConfigurable(t *testing.T) {
	fb := &fakeBackend{respond: sameFor("a1+a2")}
	strict := newScorer(fb, func(c *Config) { c.DuplicateThreshold = 0.95 })
	resp := score(t, strict, &pbc.ScoreRecommendationsRequest{
		Recommendations: dupRecs(),
		Signals:         []pbc.ScoreSignal{pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP},
	})
	for _, r := range resp.GetResults() {
		assert.Empty(t, resultScores(r).GetDuplicateGroupId())
	}
}

func TestScore_DuplicateGroupingSkippedWhenIdentifiersOmitted(t *testing.T) {
	fb := &fakeBackend{}
	req := &pbc.ScoreRecommendationsRequest{
		Recommendations: dupRecs(),
		IdentifierMode:  pbc.IdentifierMode_IDENTIFIER_MODE_OMITTED,
	}
	resp := score(t, newScorer(fb, nil), req)

	for _, r := range fb.recorded() {
		for name := range r.Questions {
			assert.False(t, strings.HasPrefix(name, "duplicate:"), name)
		}
	}
	for _, r := range resp.GetResults() {
		assert.Empty(t, resultScores(r).GetDuplicateGroupId())
	}
}

func TestScore_DuplicateOnlyWithoutBlocksMakesNoBackendCall(t *testing.T) {
	fb := &fakeBackend{}
	resp := score(t, newScorer(fb, nil), &pbc.ScoreRecommendationsRequest{
		Recommendations: makeRecs(3),
		Signals:         []pbc.ScoreSignal{pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP},
	})
	assert.Zero(t, fb.calls.Load())
	assert.Equal(t, testModel, resp.GetScorer().GetModel())
	assert.Empty(t, resp.GetScorer().GetProviderRequestId())
}

func TestScore_DuplicatesAlongsideOtherSignals(t *testing.T) {
	fb := &fakeBackend{respond: sameFor("a1+a2")}
	resp := score(t, newScorer(fb, nil), &pbc.ScoreRecommendationsRequest{Recommendations: dupRecs()})

	assert.Equal(t, "dup-1", resultScores(resp.GetResults()[0]).GetDuplicateGroupId())
	assert.Equal(t, "dup-1", resultScores(resp.GetResults()[2]).GetDuplicateGroupId())
	assert.Empty(t, resultScores(resp.GetResults()[4]).GetDuplicateGroupId())
	assert.NotNil(t, resultScores(resp.GetResults()[4]).Risk)
	assert.Equal(t, int32(7), fb.calls.Load(), "one scoring batch, five priority requests and one pair batch")
	assert.Len(t, strings.Split(resp.GetScorer().GetProviderRequestId(), ","), 7)
}

func TestScore_FailedPairBatchFailsOnlyItsMembers(t *testing.T) {
	fb := &fakeBackend{respond: func(req jevapi.Request, n int) (*jevapi.Response, error) {
		for name := range req.Questions {
			if strings.HasPrefix(name, "duplicate:") {
				return nil, &jevapi.APIError{Kind: jevapi.KindInvalidRequest, Status: 422, Message: "bad"}
			}
		}
		return defaultResponse(req, n), nil
	}}
	resp := score(t, newScorer(fb, nil), &pbc.ScoreRecommendationsRequest{Recommendations: dupRecs()})

	for _, i := range []int{0, 2, 4} {
		assert.NotNil(t, resp.GetResults()[i].GetError(), "member %d", i)
	}
	assert.NotNil(t, resultScores(resp.GetResults()[1]))
	assert.NotNil(t, resultScores(resp.GetResults()[3]))
}
