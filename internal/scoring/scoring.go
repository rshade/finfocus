package scoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/logging"
)

const (
	// MaxConcurrentBatches bounds how many scorer calls run at once.
	MaxConcurrentBatches = 8

	// DefaultProbeBatchSize is the size of the first call, made before the scorer has
	// reported its max_batch_size. Batches of 5 to 80 scored equally well in the probe.
	DefaultProbeBatchSize = 20

	maxWarningDetail = 160
	probeShrinkLimit = 6
	calibrationRank  = "ranking_only"
	calibrationProb  = "probability"
	calibrationNone  = "unspecified"
)

// Scorer is the client side of finfocus.v1.RecommendationScorerService.
type Scorer interface {
	ScoreRecommendations(
		ctx context.Context,
		in *pbc.ScoreRecommendationsRequest,
		opts ...grpc.CallOption,
	) (*pbc.ScoreRecommendationsResponse, error)
}

// Options configures a Service.
type Options struct {
	// IdentifierMode is "pseudonymized" (default), "omitted" or "raw".
	IdentifierMode string
	// FieldAllowlist limits the recommendation fields sent. Empty sends all fields.
	FieldAllowlist []string
	// Timeout bounds each scorer call. Zero means the configured default.
	Timeout time.Duration
	// ProbeBatchSize overrides DefaultProbeBatchSize.
	ProbeBatchSize int
	// Cache stores extracted scores. Nil or disabled turns caching off.
	Cache cache.Cache
	// ScorerName and PluginVersion key the score cache.
	ScorerName    string
	PluginVersion string
	// Review sets the "needs review" thresholds.
	Review ReviewPolicy
}

// OptionsFromConfig builds Options from resolved scoring configuration.
func OptionsFromConfig(cfg config.ResolvedScoring, store cache.Cache, pluginVersion string) Options {
	return Options{
		IdentifierMode: cfg.IdentifierMode,
		FieldAllowlist: cfg.FieldAllowlist,
		Timeout:        time.Duration(cfg.TimeoutSeconds) * time.Second,
		Cache:          store,
		ScorerName:     cfg.Plugin,
		PluginVersion:  pluginVersion,
		Review:         NewReviewPolicy(cfg),
	}
}

// Service scores recommendations through a scorer plugin.
type Service struct {
	scorer Scorer
	opts   Options
}

// New returns a Service. The caller has already checked that the plugin advertises
// PLUGIN_CAPABILITY_RECOMMENDATION_SCORING.
func New(scorer Scorer, opts Options) *Service {
	if opts.IdentifierMode == "" {
		opts.IdentifierMode = config.ScoringIdentifierPseudonymized
	}
	if opts.Timeout <= 0 {
		opts.Timeout = time.Duration(config.DefaultScoringTimeoutSeconds) * time.Second
	}
	if opts.ProbeBatchSize <= 0 {
		opts.ProbeBatchSize = DefaultProbeBatchSize
	}
	return &Service{scorer: scorer, opts: opts}
}

// Outcome is the result of a scoring run.
type Outcome struct {
	// Summary describes what happened. It is always set.
	Summary *engine.ScoringSummary
	// Requests holds the exact requests that would be sent. It is filled only for a dry run.
	Requests []*pbc.ScoreRecommendationsRequest
}

type item struct {
	index int
	hash  string
	rec   engine.Recommendation
}

type modelInfo struct {
	Model        string `json:"model,omitempty"`
	Calibration  string `json:"calibration,omitempty"`
	MaxBatchSize int32  `json:"max_batch_size,omitempty"`
}

type cachedScores struct {
	Risk                 *float64 `json:"risk,omitempty"`
	FalsePositive        *float64 `json:"false_positive,omitempty"`
	WorthActing          *float64 `json:"worth_acting,omitempty"`
	Priority             *float64 `json:"priority,omitempty"`
	InsufficientEvidence *float64 `json:"insufficient_evidence,omitempty"`
}

type run struct {
	svc     *Service
	recs    []engine.Recommendation
	summary *engine.ScoringSummary

	mu         sync.Mutex
	warnings   map[string]int
	groups     map[string]string
	batchCount int
	model      modelInfo
}

// Score rates the active recommendations in recs and stores the scores on them in place.
// It never removes, hides, dismisses or reorders recommendations, and it never returns an
// error for scorer trouble: failures are reported in Outcome.Summary.Warnings and the
// affected recommendations stay unscored. With dryRun set it sends nothing and returns
// the exact requests instead, after serving what it can from the cache.
func (s *Service) Score(ctx context.Context, recs []engine.Recommendation, dryRun bool) (*Outcome, error) {
	r := &run{
		svc:      s,
		recs:     recs,
		summary:  &engine.ScoringSummary{Scorer: s.opts.ScorerName},
		warnings: map[string]int{},
		groups:   map[string]string{},
	}
	outcome := &Outcome{Summary: r.summary}

	items := r.eligible()
	if len(items) == 0 {
		r.finish()
		return outcome, nil
	}

	r.loadModel()
	pending := r.serveFromCache(items)
	if len(pending) > 0 {
		if dryRun {
			reqs, err := r.buildAll(pending)
			if err != nil {
				return nil, err
			}
			outcome.Requests = reqs
		} else {
			r.execute(ctx, pending)
		}
	}
	r.finish()
	return outcome, nil
}

func (r *run) warn(msg string) {
	r.mu.Lock()
	r.warnings[msg]++
	r.mu.Unlock()
}

func (r *run) eligible() []item {
	hashIDs := forHashing(r.svc.opts.IdentifierMode)
	seen := map[string]bool{}
	var items []item
	var noID, dupID, unconvertible int

	for i, rec := range r.recs {
		if rec.Status == engine.RecommendationStatusDismissed || rec.Status == engine.RecommendationStatusSnoozed {
			continue
		}
		r.summary.Requested++
		if rec.ID == "" {
			noID++
			continue
		}
		if seen[rec.ID] {
			dupID++
			continue
		}
		seen[rec.ID] = true

		msg, ok := toProto(rec, hashIDs)
		if !ok {
			unconvertible++
			continue
		}
		msg.Id = ""
		applyAllowlist(msg, r.svc.opts.FieldAllowlist)
		items = append(items, item{index: i, hash: r.contentHash(msg), rec: rec})
	}

	if noID > 0 {
		r.warn(fmt.Sprintf("%d recommendation(s) have no id and were left unscored", noID))
	}
	if dupID > 0 {
		r.warn(fmt.Sprintf("%d recommendation(s) repeat an id already scored and were left unscored", dupID))
	}
	if unconvertible > 0 {
		r.warn(fmt.Sprintf("%d recommendation(s) could not be converted and were left unscored", unconvertible))
	}
	return items
}

func (r *run) contentHash(msg *pbc.Recommendation) string {
	data, err := gproto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return ""
	}
	data = append(data, []byte("\x00"+r.svc.opts.IdentifierMode)...)
	sum := sha256Sum(data)
	return hex.EncodeToString(sum[:16])
}

func (r *run) cacheEnabled() bool {
	c := r.svc.opts.Cache
	return c != nil && c.IsEnabled() && r.svc.opts.ScorerName != ""
}

func (r *run) loadModel() {
	if !r.cacheEnabled() {
		return
	}
	entry, err := r.svc.opts.Cache.Get(cache.BuildScoreModelKey(r.svc.opts.ScorerName, r.svc.opts.PluginVersion))
	if err != nil || entry == nil {
		return
	}
	var info modelInfo
	if json.Unmarshal(entry.Data, &info) == nil {
		r.model = info
	}
}

func (r *run) storeModel() {
	if !r.cacheEnabled() {
		return
	}
	data, err := json.Marshal(r.model)
	if err != nil {
		return
	}
	key := cache.BuildScoreModelKey(r.svc.opts.ScorerName, r.svc.opts.PluginVersion)
	_ = r.svc.opts.Cache.Set(key, data)
}

func (r *run) scoreKey(hash string) string {
	return cache.BuildScoreKey(r.svc.opts.ScorerName, r.svc.opts.PluginVersion, r.model.Model, hash)
}

func (r *run) serveFromCache(items []item) []item {
	if !r.cacheEnabled() || r.model.Model == "" {
		return items
	}
	var pending []item
	for _, it := range items {
		entry, err := r.svc.opts.Cache.Get(r.scoreKey(it.hash))
		if err != nil || entry == nil {
			pending = append(pending, it)
			continue
		}
		var cached cachedScores
		if json.Unmarshal(entry.Data, &cached) != nil {
			pending = append(pending, it)
			continue
		}
		r.recs[it.index].Scores = &engine.RecommendationScores{
			Risk:                 cached.Risk,
			FalsePositive:        cached.FalsePositive,
			WorthActing:          cached.WorthActing,
			Priority:             cached.Priority,
			InsufficientEvidence: cached.InsufficientEvidence,
		}
		r.summary.FromCache++
		r.summary.Scored++
	}
	if r.summary.FromCache > 0 {
		r.summary.Model = r.model.Model
		r.summary.Calibration = r.model.Calibration
	}
	return pending
}

func (r *run) batchSize() int {
	if r.model.MaxBatchSize > 0 {
		return int(r.model.MaxBatchSize)
	}
	return r.svc.opts.ProbeBatchSize
}

func chunk(items []item, size int) [][]item {
	if size < 1 {
		size = 1
	}
	var out [][]item
	for len(items) > 0 {
		n := min(size, len(items))
		out = append(out, items[:n])
		items = items[n:]
	}
	return out
}

func (r *run) buildRequest(items []item) (*pbc.ScoreRecommendationsRequest, error) {
	ids, err := newIdentifiers(r.svc.opts.IdentifierMode)
	if err != nil {
		return nil, err
	}
	req := &pbc.ScoreRecommendationsRequest{
		IdentifierMode: identifierModeProto(r.svc.opts.IdentifierMode),
	}
	for i, it := range items {
		msg, ok := toProto(it.rec, ids)
		if !ok {
			return nil, errors.New("recommendation without id reached request building")
		}
		if r.svc.opts.IdentifierMode != config.ScoringIdentifierRaw {
			msg.Id = fmt.Sprintf("rec-%d", i)
		}
		applyAllowlist(msg, r.svc.opts.FieldAllowlist)
		req.Recommendations = append(req.Recommendations, msg)
	}
	return req, nil
}

func (r *run) buildAll(items []item) ([]*pbc.ScoreRecommendationsRequest, error) {
	var reqs []*pbc.ScoreRecommendationsRequest
	for _, group := range chunk(items, r.batchSize()) {
		req, err := r.buildRequest(group)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

func (r *run) call(
	ctx context.Context,
	req *pbc.ScoreRecommendationsRequest,
) (*pbc.ScoreRecommendationsResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, r.svc.opts.Timeout)
	defer cancel()
	resp, err := r.svc.scorer.ScoreRecommendations(callCtx, req)
	if err != nil {
		return nil, err
	}
	if validateErr := pluginsdk.ValidateScoreRecommendationsResponse(req, resp); validateErr != nil {
		return nil, fmt.Errorf("invalid scorer response: %w", validateErr)
	}
	return resp, nil
}

func (r *run) execute(ctx context.Context, pending []item) {
	log := logging.FromContext(ctx)

	first, rest := r.probe(ctx, pending)
	if first == nil {
		return
	}

	sem := make(chan struct{}, MaxConcurrentBatches)
	var wg sync.WaitGroup
	for _, group := range chunk(rest, r.batchSize()) {
		wg.Add(1)
		sem <- struct{}{}
		go func(group []item) {
			defer wg.Done()
			defer func() { <-sem }()
			req, err := r.buildRequest(group)
			if err != nil {
				r.warn(err.Error())
				return
			}
			resp, callErr := r.call(ctx, req)
			if callErr != nil {
				r.failBatch(ctx, len(group), callErr)
				return
			}
			r.apply(group, resp)
		}(group)
	}
	wg.Wait()

	log.Debug().Ctx(ctx).Str("component", "scoring").
		Int("scored", r.summary.Scored).Int("requested", r.summary.Requested).
		Msg("scoring complete")
}

// probe makes the first call. Before the scorer has reported max_batch_size, the batch
// may be too large; an INVALID_ARGUMENT halves it and retries. It returns the response
// and the items not yet sent, or nil when the scorer could not be used.
func (r *run) probe(ctx context.Context, pending []item) (*pbc.ScoreRecommendationsResponse, []item) {
	size := min(r.batchSize(), len(pending))
	var lastErr error
	for range probeShrinkLimit {
		req, err := r.buildRequest(pending[:size])
		if err != nil {
			r.warn(err.Error())
			return nil, nil
		}
		resp, callErr := r.call(ctx, req)
		if callErr == nil {
			r.apply(pending[:size], resp)
			r.mu.Lock()
			r.model.MaxBatchSize = resp.GetMaxBatchSize()
			r.mu.Unlock()
			r.storeModel()
			return resp, pending[size:]
		}
		lastErr = callErr
		if status.Code(callErr).String() != "InvalidArgument" || size == 1 {
			break
		}
		size = max(1, size/2) //nolint:mnd // halve until the scorer accepts the batch
	}
	r.failBatch(ctx, len(pending), lastErr)
	return nil, nil
}

func (r *run) failBatch(ctx context.Context, count int, err error) {
	logging.FromContext(ctx).Warn().Ctx(ctx).Str("component", "scoring").Err(err).
		Int("recommendations", count).Msg("scorer call failed; recommendations left unscored")

	detail := err.Error()
	if st, ok := status.FromError(err); ok {
		detail = st.Code().String()
		if msg := st.Message(); msg != "" {
			detail += ": " + msg
		}
	}
	if len(detail) > maxWarningDetail {
		detail = detail[:maxWarningDetail] + "..."
	}
	r.warn(fmt.Sprintf("scorer call failed (%s); affected recommendations are unscored", detail))
}

func calibrationName(c pbc.ScoreCalibration) string {
	switch c {
	case pbc.ScoreCalibration_SCORE_CALIBRATION_PROBABILITY:
		return calibrationProb
	case pbc.ScoreCalibration_SCORE_CALIBRATION_RANKING_ONLY:
		return calibrationRank
	case pbc.ScoreCalibration_SCORE_CALIBRATION_UNSPECIFIED:
		return calibrationNone
	default:
		return calibrationNone
	}
}

func (r *run) apply(group []item, resp *pbc.ScoreRecommendationsResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.batchCount++
	batch := r.batchCount
	r.summary.Model = resp.GetScorer().GetModel()
	r.summary.Calibration = calibrationName(resp.GetScorer().GetCalibration())
	r.model.Model = resp.GetScorer().GetModel()
	r.model.Calibration = r.summary.Calibration
	if r.summary.Scorer == "" {
		r.summary.Scorer = resp.GetScorer().GetName()
	}

	itemErrors := map[string]int{}
	for i, it := range group {
		res := resp.GetResults()[i]
		if res.GetError() != nil {
			itemErrors[fmt.Sprintf("code %d", res.GetError().GetCode())]++
			continue
		}
		scores := res.GetScores()
		out := &engine.RecommendationScores{
			Risk:                 scores.Risk,
			FalsePositive:        scores.FalsePositive,
			WorthActing:          scores.WorthActing,
			Priority:             scores.Priority,
			InsufficientEvidence: scores.InsufficientEvidence,
		}
		if gid := scores.GetDuplicateGroupId(); gid != "" {
			key := fmt.Sprintf("%d/%s", batch, gid)
			if _, ok := r.groups[key]; !ok {
				r.groups[key] = fmt.Sprintf("dup-%d", len(r.groups)+1)
			}
			out.DuplicateGroupID = r.groups[key]
		}
		r.recs[it.index].Scores = out
		r.summary.Scored++
		r.storeScores(it, out)
	}
	for code, n := range itemErrors {
		r.warnings[fmt.Sprintf("scorer could not score %d recommendation(s) (%s)", n, code)]++
	}
}

func (r *run) storeScores(it item, scores *engine.RecommendationScores) {
	if !r.cacheEnabled() {
		return
	}
	data, err := json.Marshal(cachedScores{
		Risk:                 scores.Risk,
		FalsePositive:        scores.FalsePositive,
		WorthActing:          scores.WorthActing,
		Priority:             scores.Priority,
		InsufficientEvidence: scores.InsufficientEvidence,
	})
	if err != nil {
		return
	}
	_ = r.svc.opts.Cache.Set(r.scoreKey(it.hash), data)
}

func (r *run) finish() {
	r.svc.opts.Review.Mark(r.recs)
	r.summary.Unscored = r.summary.Requested - r.summary.Scored

	msgs := make([]string, 0, len(r.warnings))
	for msg := range r.warnings {
		msgs = append(msgs, msg)
	}
	sort.Strings(msgs)
	if len(msgs) > 0 {
		r.summary.Warnings = msgs
	}
}

func sha256Sum(data []byte) [32]byte { return sha256.Sum256(data) }
