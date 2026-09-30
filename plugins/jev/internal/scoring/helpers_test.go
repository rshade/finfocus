package scoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func f64(v float64) *float64 { return &v }

func noulAnswer(v float64) jevapi.Answer { return jevapi.Answer{Type: "noul", NoulValue: f64(v)} }

func scoreAnswer(v float64) jevapi.Answer { return jevapi.Answer{Type: "score", Score: f64(v)} }

// fakeBackend records requests and answers every question through respond.
type fakeBackend struct {
	mu       sync.Mutex
	requests []jevapi.Request
	calls    atomic.Int32
	inFlight atomic.Int32
	peak     atomic.Int32
	delay    func()
	respond  func(req jevapi.Request, n int) (*jevapi.Response, error)
}

func (f *fakeBackend) SystemOne(_ context.Context, req jevapi.Request) (*jevapi.Response, error) {
	n := int(f.calls.Add(1))
	cur := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		peak := f.peak.Load()
		if cur <= peak || f.peak.CompareAndSwap(peak, cur) {
			break
		}
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	if f.delay != nil {
		f.delay()
	}
	if f.respond != nil {
		return f.respond(req, n)
	}
	return defaultResponse(req, n), nil
}

func (f *fakeBackend) recorded() []jevapi.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]jevapi.Request(nil), f.requests...)
}

func defaultResponse(req jevapi.Request, n int) *jevapi.Response {
	answers := make(map[string]jevapi.Answer, len(req.Questions))
	for name, q := range req.Questions {
		switch {
		case q.Type == "score":
			answers[name] = scoreAnswer(1.5)
		case strings.HasPrefix(name, "duplicate:"):
			answers[name] = noulAnswer(0.1)
		default:
			answers[name] = noulAnswer(0.3)
		}
	}
	return &jevapi.Response{
		Model:     "jev-1.13.0",
		Answers:   answers,
		Usage:     jevapi.Usage{InputTokens: 100},
		RequestID: fmt.Sprintf("req-%d", n),
	}
}

func makeRec(id, resourceID string) *pbc.Recommendation {
	return &pbc.Recommendation{
		Id:          id,
		Category:    pbc.RecommendationCategory_RECOMMENDATION_CATEGORY_COST,
		ActionType:  pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
		Description: "Downsize " + id,
		Resource: &pbc.ResourceRecommendationInfo{
			Id: resourceID, Provider: "aws", ResourceType: "ec2:instance",
			Tags: map[string]string{"env": "prod"},
		},
		Impact: &pbc.RecommendationImpact{EstimatedSavings: 42, Currency: "USD"},
	}
}

// questionKey returns the key req uses to ask signal about the recommendation
// with id, found by its description in the state list. ok is false when req
// does not ask that question.
func questionKey(req jevapi.Request, signal, id string) (string, bool) {
	state, _ := req.State.([]any)
	for pos, item := range state {
		record, _ := item.(map[string]any)
		if record["description"] != "Downsize "+id {
			continue
		}
		key := questionName(signal, pos)
		_, ok := req.Questions[key]
		return key, ok
	}
	return "", false
}

func makeRecs(n int) []*pbc.Recommendation {
	recs := make([]*pbc.Recommendation, n)
	for i := range recs {
		recs[i] = makeRec(fmt.Sprintf("rec-%03d", i), fmt.Sprintf("res-%03d", i))
	}
	return recs
}

func resultScores(r *pbc.RecommendationScoreResult) *pbc.RecommendationScores {
	return r.GetScores()
}

// mergedRequest combines the questions of every recorded request, which is
// how tests compare the whole question set regardless of request kinds.
func mergedRequest(f *fakeBackend) jevapi.Request {
	merged := jevapi.Request{Questions: map[string]jevapi.Question{}}
	for _, r := range f.recorded() {
		merged.Model = r.Model
		for name, q := range r.Questions {
			merged.Questions[name] = q
		}
	}
	return merged
}
