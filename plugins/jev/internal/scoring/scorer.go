package scoring

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	// ScorerName is reported as ScorerInfo.name.
	ScorerName = "jev"

	// DefaultBatchSize is the number of recommendations in one backend request.
	DefaultBatchSize = 25
	// DefaultMaxRequestSize is the largest ScoreRecommendations request the
	// scorer accepts; it is split into backend batches of DefaultBatchSize.
	DefaultMaxRequestSize = 100
	// DefaultConcurrency is the number of backend requests in flight.
	DefaultConcurrency = 8
	// DefaultDuplicateThreshold is the yes probability at which two
	// recommendations count as duplicates.
	DefaultDuplicateThreshold = 0.5
	// DefaultMaxBlockSize is the most recommendations compared pairwise for
	// one resource.
	DefaultMaxBlockSize = 10

	maxPriority     = 3.0
	maxReportedIDs  = 8
	missingAnswerFm = "the backend returned no usable answer for %s"
)

// Backend is the part of the Jev client the scorer uses.
type Backend interface {
	SystemOne(ctx context.Context, req jevapi.Request) (*jevapi.Response, error)
}

// Config tunes a Scorer. Zero fields take the defaults above.
type Config struct {
	// Model is sent with every request; it should be a versioned id.
	Model string
	// BatchSize is the number of recommendations per backend request.
	BatchSize int
	// MaxRequestSize is reported as max_batch_size.
	MaxRequestSize int
	// Concurrency is the number of backend requests in flight.
	Concurrency int
	// DuplicateThreshold is the yes probability that marks a pair as the same.
	DuplicateThreshold float64
	// MaxBlockSize bounds pairwise duplicate comparison per resource.
	MaxBlockSize int
	// Limits caps free text sent to the backend.
	Limits Limits
	// Logger receives counts and timings, never payloads or the API key.
	Logger zerolog.Logger
}

// Scorer implements ScoreRecommendations on top of a Backend.
type Scorer struct {
	backend Backend
	cfg     Config
	render  renderer
}

// New builds a Scorer. A nil backend makes every call fail with
// UNAUTHENTICATED, which is how a plugin without an API key behaves.
func New(backend Backend, cfg Config) *Scorer {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.MaxRequestSize <= 0 {
		cfg.MaxRequestSize = DefaultMaxRequestSize
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultConcurrency
	}
	if cfg.DuplicateThreshold <= 0 || cfg.DuplicateThreshold > 1 {
		cfg.DuplicateThreshold = DefaultDuplicateThreshold
	}
	if cfg.MaxBlockSize < 2 { //nolint:mnd // Fewer than two members cannot form a pair.
		cfg.MaxBlockSize = DefaultMaxBlockSize
	}
	cfg.Limits = cfg.Limits.withDefaults()
	return &Scorer{backend: backend, cfg: cfg, render: renderer{limits: cfg.Limits}}
}

// MaxBatchSize is the value reported as max_batch_size.
func (s *Scorer) MaxBatchSize() int32 {
	return int32(min(s.cfg.MaxRequestSize, 1<<30)) //nolint:gosec,mnd // Bounded just above.
}

// SupportedSignals lists every signal the scorer can return.
func (s *Scorer) SupportedSignals() []pbc.ScoreSignal {
	return supportedSignals()
}

// itemState is the working record for one request entry.
type itemState struct {
	rec      *pbc.Recommendation
	rendered map[string]any
	values   map[pbc.ScoreSignal]float64
	group    string
	err      *pbc.ResourceError
}

func (it *itemState) fail(code codes.Code, message string) {
	if it.err == nil {
		it.err = &pbc.ResourceError{Code: int32(code), Message: message} //nolint:gosec // gRPC codes are small.
	}
}

// batch is one backend request and, once run, its outcome.
type batch struct {
	indexes []int
	signals []perRecordSignal
	pairs   []pair
	request jevapi.Request
	resp    *jevapi.Response
	err     error
}

// Score rates each recommendation. Whole-call failures (authentication,
// exhausted rate limits, outages, cancellation) return a gRPC status; a
// failure that touches only some recommendations is reported in their
// results.
func (s *Scorer) Score(
	ctx context.Context, req *pbc.ScoreRecommendationsRequest,
) (*pbc.ScoreRecommendationsResponse, error) {
	if err := pluginsdk.ValidateScoreRecommendationsRequest(req, s.MaxBatchSize()); err != nil {
		return nil, err
	}
	if s.backend == nil {
		return nil, status.Error(codes.Unauthenticated,
			"TYPESAFE_API_KEY is not set, so the jev scorer cannot call the backend")
	}
	started := time.Now()
	wanted := requestedSignals(req.GetSignals())
	items := s.prepare(req.GetRecommendations())

	scoreBatches := s.scoreBatches(items, wanted)
	pairBatches, pairs := s.pairBatches(req, items, wanted)
	all := slices.Concat(scoreBatches, pairBatches)
	if err := s.run(ctx, all); err != nil {
		s.cfg.Logger.Warn().Err(err).Int("recommendations", len(items)).Msg("scoring call failed")
		return nil, err
	}

	s.applyScores(items, scoreBatches)
	if wanted[pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP] {
		s.applyDuplicates(items, pairBatches, pairs)
	}
	resp := s.response(items, all, wanted)
	s.logSummary(len(items), all, resp, time.Since(started))
	return resp, nil
}

func requestedSignals(signals []pbc.ScoreSignal) map[pbc.ScoreSignal]bool {
	if len(signals) == 0 {
		signals = supportedSignals()
	}
	set := make(map[pbc.ScoreSignal]bool, len(signals))
	for _, sig := range signals {
		set[sig] = true
	}
	return set
}

func (s *Scorer) prepare(recs []*pbc.Recommendation) []*itemState {
	items := make([]*itemState, len(recs))
	for i, rec := range recs {
		it := &itemState{rec: rec, values: make(map[pbc.ScoreSignal]float64)}
		rendered, err := s.render.render(rec)
		if err != nil {
			it.fail(codes.InvalidArgument, "recommendation could not be encoded for scoring")
		}
		it.rendered = rendered
		items[i] = it
	}
	return items
}

func (s *Scorer) scoreBatches(items []*itemState, wanted map[pbc.ScoreSignal]bool) []*batch {
	var batched, single []perRecordSignal
	for _, sig := range perRecordSignals() {
		switch {
		case !wanted[sig.signal]:
		case sig.signal == pbc.ScoreSignal_SCORE_SIGNAL_PRIORITY:
			single = append(single, sig)
		default:
			batched = append(batched, sig)
		}
	}
	var usable []int
	for i, it := range items {
		if it.err == nil {
			usable = append(usable, i)
		}
	}
	var batches []*batch
	if len(batched) > 0 {
		for chunk := range slices.Chunk(usable, s.cfg.BatchSize) {
			batches = append(batches, s.recordBatch(items, chunk, batched))
		}
	}
	if len(single) > 0 {
		for chunk := range slices.Chunk(usable, 1) {
			batches = append(batches, s.recordBatch(items, chunk, single))
		}
	}
	return batches
}

// recordBatch builds one backend request asking signals about the items at
// indexes. Priority is always requested with a single item: scored next to
// other records its rank quality drops sharply.
func (s *Scorer) recordBatch(items []*itemState, indexes []int, signals []perRecordSignal) *batch {
	state := make([]any, len(indexes))
	questions := make(map[string]jevapi.Question, len(indexes)*len(signals))
	for pos, idx := range indexes {
		record := make(map[string]any, len(items[idx].rendered)+1)
		for k, v := range items[idx].rendered {
			record[k] = v
		}
		record["position"] = pos
		state[pos] = record
		for _, sig := range signals {
			questions[questionName(sig.name, pos)] = sig.build(pos)
		}
	}
	return &batch{
		indexes: slices.Clone(indexes),
		signals: signals,
		request: jevapi.Request{Model: s.cfg.Model, State: state, Questions: questions},
	}
}

func (s *Scorer) pairBatches(
	req *pbc.ScoreRecommendationsRequest, items []*itemState, wanted map[pbc.ScoreSignal]bool,
) ([]*batch, []pair) {
	if !wanted[pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP] ||
		req.GetIdentifierMode() == pbc.IdentifierMode_IDENTIFIER_MODE_OMITTED {
		return nil, nil
	}
	recs := make([]*pbc.Recommendation, len(items))
	for i, it := range items {
		if it.err == nil {
			recs[i] = it.rec
		} else {
			recs[i] = &pbc.Recommendation{}
		}
	}
	pairs := blockPairs(duplicateBlocks(recs, s.cfg.MaxBlockSize))
	var batches []*batch
	for chunk := range slices.Chunk(pairs, s.cfg.BatchSize) {
		state := make([]any, len(chunk))
		questions := make(map[string]jevapi.Question, len(chunk))
		for pos, p := range chunk {
			state[pos] = map[string]any{"position": pos, "a": items[p.a].rendered, "b": items[p.b].rendered}
			questions[pairQuestionName(pos)] = duplicateQuestion(pos)
		}
		batches = append(batches, &batch{
			pairs:   slices.Clone(chunk),
			request: jevapi.Request{Model: s.cfg.Model, State: state, Questions: questions},
		})
	}
	return batches, pairs
}

// run executes every batch with bounded concurrency. It returns a gRPC status
// when a backend failure makes the whole call unusable; other failures stay in
// the batch for the caller to report per item.
func (s *Scorer) run(ctx context.Context, batches []*batch) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu    sync.Mutex
		fatal error
		wg    sync.WaitGroup
		slots = make(chan struct{}, s.cfg.Concurrency)
	)
	for _, b := range batches {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			b.resp, b.err = s.backend.SystemOne(ctx, b.request)
			if b.err == nil {
				return
			}
			if whole, st := wholeCallStatus(b.err); whole {
				mu.Lock()
				if fatal == nil {
					fatal = st
					cancel()
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	return fatal
}

// wholeCallStatus maps a backend error to a gRPC status when it should fail
// the whole call.
func wholeCallStatus(err error) (bool, error) {
	var apiErr *jevapi.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case jevapi.KindUnauthenticated:
			return true, status.Error(codes.Unauthenticated, "jev backend rejected the API key: "+apiErr.Error())
		case jevapi.KindPermissionDenied:
			return true, status.Error(codes.PermissionDenied, "jev backend denied access: "+apiErr.Error())
		case jevapi.KindRateLimited:
			return true, status.Error(codes.ResourceExhausted, "jev backend rate limit: "+apiErr.Error())
		case jevapi.KindUnavailable:
			return true, status.Error(codes.Unavailable, "jev backend unavailable: "+apiErr.Error())
		case jevapi.KindInvalidRequest:
			return false, nil
		}
	}
	switch {
	case errors.Is(err, context.Canceled):
		return true, status.Error(codes.Canceled, "scoring canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return true, status.Error(codes.DeadlineExceeded, "jev backend did not answer in time")
	}
	return false, nil
}

// batchFailure describes a batch-level failure that is not a whole-call one.
func batchFailure(err error) (codes.Code, string) {
	var apiErr *jevapi.APIError
	if errors.As(err, &apiErr) && apiErr.Kind == jevapi.KindInvalidRequest {
		return codes.InvalidArgument, "the backend refused the request: " + apiErr.Error()
	}
	return codes.Internal, "the backend response could not be used: " + err.Error()
}

func (s *Scorer) applyScores(items []*itemState, batches []*batch) {
	for _, b := range batches {
		if b.err != nil {
			code, msg := batchFailure(b.err)
			for _, idx := range b.indexes {
				items[idx].fail(code, msg)
			}
			continue
		}
		for pos, idx := range b.indexes {
			it := items[idx]
			for _, sig := range b.signals {
				name := questionName(sig.name, pos)
				value, ok := answerValue(b.resp.Answers[name], sig.signal)
				if !ok {
					it.fail(codes.Internal, fmt.Sprintf(missingAnswerFm, sig.name))
					break
				}
				it.values[sig.signal] = value
			}
		}
	}
}

func answerValue(a jevapi.Answer, signal pbc.ScoreSignal) (float64, bool) {
	if signal == pbc.ScoreSignal_SCORE_SIGNAL_PRIORITY {
		if !a.HasScore() {
			return 0, false
		}
		return clamp(a.ScoreValue(), maxPriority), true
	}
	if !a.HasNoul() {
		return 0, false
	}
	return clamp(a.Noul(), 1), true
}

func clamp(v, upper float64) float64 {
	return max(0, min(v, upper))
}

func (s *Scorer) applyDuplicates(items []*itemState, batches []*batch, pairs []pair) {
	if len(pairs) == 0 {
		return
	}
	same := make([]bool, 0, len(pairs))
	for _, b := range batches {
		for pos, p := range b.pairs {
			if b.err != nil {
				code, msg := batchFailure(b.err)
				items[p.a].fail(code, msg)
				items[p.b].fail(code, msg)
				same = append(same, false)
				continue
			}
			a := b.resp.Answers[pairQuestionName(pos)]
			if !a.HasNoul() {
				msg := fmt.Sprintf(missingAnswerFm, "a duplicate comparison")
				items[p.a].fail(codes.Internal, msg)
				items[p.b].fail(codes.Internal, msg)
				same = append(same, false)
				continue
			}
			same = append(same, a.Noul() >= s.cfg.DuplicateThreshold)
		}
	}
	for idx, group := range duplicateGroups(len(items), pairs, same) {
		items[idx].group = group
	}
}

func (s *Scorer) response(
	items []*itemState, batches []*batch, wanted map[pbc.ScoreSignal]bool,
) *pbc.ScoreRecommendationsResponse {
	results := make([]*pbc.RecommendationScoreResult, len(items))
	for i, it := range items {
		result := &pbc.RecommendationScoreResult{RecommendationId: it.rec.GetId()}
		if it.err != nil {
			result.Result = &pbc.RecommendationScoreResult_Error{Error: it.err}
		} else {
			result.Result = &pbc.RecommendationScoreResult_Scores{Scores: buildScores(it, wanted)}
		}
		results[i] = result
	}
	model, ids := s.provenance(batches)
	return &pbc.ScoreRecommendationsResponse{
		Results:          results,
		MaxBatchSize:     s.MaxBatchSize(),
		SupportedSignals: supportedSignals(),
		Scorer: &pbc.ScorerInfo{
			Name:              ScorerName,
			Model:             model,
			Calibration:       pbc.ScoreCalibration_SCORE_CALIBRATION_RANKING_ONLY,
			ProviderRequestId: ids,
		},
	}
}

func buildScores(it *itemState, wanted map[pbc.ScoreSignal]bool) *pbc.RecommendationScores {
	scores := &pbc.RecommendationScores{}
	set := func(signal pbc.ScoreSignal, dst **float64) {
		if v, ok := it.values[signal]; ok && wanted[signal] {
			*dst = proto.Float64(v)
		}
	}
	set(pbc.ScoreSignal_SCORE_SIGNAL_RISK, &scores.Risk)
	set(pbc.ScoreSignal_SCORE_SIGNAL_FALSE_POSITIVE, &scores.FalsePositive)
	set(pbc.ScoreSignal_SCORE_SIGNAL_WORTH_ACTING, &scores.WorthActing)
	set(pbc.ScoreSignal_SCORE_SIGNAL_PRIORITY, &scores.Priority)
	set(pbc.ScoreSignal_SCORE_SIGNAL_INSUFFICIENT_EVIDENCE, &scores.InsufficientEvidence)
	if wanted[pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP] {
		scores.DuplicateGroupId = it.group
	}
	return scores
}

// provenance reports the model the API named and the provider request ids of
// the calls that answered.
func (s *Scorer) provenance(batches []*batch) (string, string) {
	model := s.cfg.Model
	var ids []string
	named := false
	for _, b := range batches {
		if b.resp == nil {
			continue
		}
		if !named && b.resp.Model != "" {
			model, named = b.resp.Model, true
		}
		if b.resp.RequestID != "" && len(ids) < maxReportedIDs {
			ids = append(ids, b.resp.RequestID)
		}
	}
	return model, strings.Join(ids, ",")
}

func (s *Scorer) logSummary(
	n int, batches []*batch, resp *pbc.ScoreRecommendationsResponse, elapsed time.Duration,
) {
	tokens, failed := 0, 0
	for _, b := range batches {
		if b.resp != nil {
			tokens += b.resp.Usage.InputTokens
		}
		if b.err != nil {
			failed++
		}
	}
	s.cfg.Logger.Info().
		Int("recommendations", n).
		Int("backend_requests", len(batches)).
		Int("failed_requests", failed).
		Int("input_tokens", tokens).
		Str("model", resp.GetScorer().GetModel()).
		Str("provider_request_id", resp.GetScorer().GetProviderRequestId()).
		Dur("elapsed", elapsed).
		Msg("scored recommendations")
}
