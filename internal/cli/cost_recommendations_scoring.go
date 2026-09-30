package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/scoring"
)

var scoreFilterPattern = regexp.MustCompile(`^\s*([a-z_]+)\s*(<=|>=|!=|==|<|>|=)\s*(.*?)\s*$`)

// scoreFilter is one parsed "signal<op>value" filter such as risk<=0.3.
type scoreFilter struct {
	signal string
	op     string
	value  float64
}

// parseScoreFilter parses a score filter expression. isScore reports whether the
// expression names a score signal at all; other expressions (for example
// "action=MIGRATE") are left to the action filter.
func parseScoreFilter(expr string) (scoreFilter, bool, error) {
	m := scoreFilterPattern.FindStringSubmatch(expr)
	if m == nil || !slices.Contains(engine.ScoreSignalNames(), m[1]) {
		return scoreFilter{}, false, nil
	}
	op := m[2]
	if op == "!=" {
		return scoreFilter{}, true, fmt.Errorf("invalid score filter %q: operator != is not supported", expr)
	}
	if op == "==" {
		op = "="
	}
	value, err := strconv.ParseFloat(m[3], 64)
	if err != nil {
		return scoreFilter{}, true, fmt.Errorf("invalid score filter %q: value must be a number", expr)
	}
	return scoreFilter{signal: m[1], op: op, value: value}, true, nil
}

func (f scoreFilter) matches(rec engine.Recommendation) bool {
	v, ok := rec.Scores.Signal(f.signal)
	if !ok {
		return false
	}
	switch f.op {
	case "<=":
		return v <= f.value
	case ">=":
		return v >= f.value
	case "<":
		return v < f.value
	case ">":
		return v > f.value
	default:
		return v == f.value
	}
}

// applyScoreFilters keeps the recommendations that satisfy every score filter. Filters
// that are not score filters are ignored here. Recommendations without the filtered
// signal are dropped, because they cannot satisfy it.
func applyScoreFilters(recs []engine.Recommendation, filters []string) ([]engine.Recommendation, error) {
	var parsed []scoreFilter
	for _, expr := range filters {
		f, isScore, err := parseScoreFilter(expr)
		if err != nil {
			return nil, err
		}
		if isScore {
			parsed = append(parsed, f)
		}
	}
	if len(parsed) == 0 {
		return recs, nil
	}

	kept := make([]engine.Recommendation, 0, len(recs))
	for _, rec := range recs {
		if slices.ContainsFunc(parsed, func(f scoreFilter) bool { return !f.matches(rec) }) {
			continue
		}
		kept = append(kept, rec)
	}
	return kept, nil
}

// scoreSortField returns the score signal named by a --sort expression, or "".
func scoreSortField(sortExpr string) string {
	field, _, _ := strings.Cut(strings.TrimSpace(sortExpr), ":")
	if slices.Contains(engine.ScoreSignalNames(), field) {
		return field
	}
	return ""
}

// validateScoringFlags rejects flag combinations that cannot work before any plugin runs.
func validateScoringFlags(params costRecommendationsParams, cfg config.ResolvedScoring) error {
	active := cfg.Enabled && !params.noScoring

	needsScores := scoreSortField(params.sort) != ""
	for _, expr := range params.filter {
		_, isScore, err := parseScoreFilter(expr)
		if err != nil {
			return err
		}
		needsScores = needsScores || isScore
	}

	if params.scoringDryRun && params.noScoring {
		return errors.New("--scoring-dry-run cannot be combined with --no-scoring")
	}
	if params.scoringDryRun && !cfg.Enabled {
		return errors.New("--scoring-dry-run requires scoring: set scoring.enabled and scoring.plugin in the config")
	}
	if needsScores && !active {
		return errors.New("sorting or filtering by a score requires scoring: " +
			"set scoring.enabled and scoring.plugin in the config, and do not pass --no-scoring")
	}
	return nil
}

// findScorerClient returns the named plugin when it advertises the scoring capability.
// Otherwise it returns a warning describing what is missing.
func findScorerClient(clients []*pluginhost.Client, name string) (*pluginhost.Client, string) {
	for _, c := range clients {
		if c.Name != name {
			continue
		}
		if !c.HasCapability(pluginhost.CapabilityRecommendationScoring) {
			return nil, fmt.Sprintf("plugin %q does not advertise the %s capability; recommendations are unscored",
				name, pluginhost.CapabilityRecommendationScoring)
		}
		return c, ""
	}
	return nil, fmt.Sprintf("scoring plugin %q not found; recommendations are unscored", name)
}

// resolveScorerClient locates the configured scorer among the open clients. When --adapter
// restricted the open plugins, it opens the scorer on its own. The returned cleanup is
// always safe to call.
func resolveScorerClient(
	ctx context.Context,
	adapter string,
	cfg config.ResolvedScoring,
	clients []*pluginhost.Client,
	audit *auditContext,
) (*pluginhost.Client, string, func()) {
	noop := func() {}
	if c, warning := findScorerClient(clients, cfg.Plugin); c != nil {
		return c, "", noop
	} else if adapter == "" || adapter == cfg.Plugin {
		return nil, warning, noop
	}

	extra, cleanup, err := openPlugins(ctx, cfg.Plugin, audit)
	if err != nil {
		return nil, fmt.Sprintf(
			"could not open scoring plugin %q: %v; recommendations are unscored",
			cfg.Plugin,
			err,
		), noop
	}
	c, warning := findScorerClient(extra, cfg.Plugin)
	if c == nil {
		cleanup()
		return nil, warning, noop
	}
	return c, "", cleanup
}

// runScoringStep runs the optional scoring step when scoring is enabled. done is true when a
// dry run printed its requests and the command should stop. The returned cleanup is always
// safe to call.
func runScoringStep(
	ctx context.Context,
	cmd *cobra.Command,
	params costRecommendationsParams,
	cfg config.ResolvedScoring,
	clients []*pluginhost.Client,
	store cache.Cache,
	audit *auditContext,
	result *engine.RecommendationsResult,
) (bool, func(), error) {
	noop := func() {}
	if !cfg.Enabled || params.noScoring {
		return false, noop, nil
	}

	scorer, warning, cleanup := resolveScorerClient(ctx, params.adapter, cfg, clients, audit)
	if scorer == nil {
		result.Scoring = &engine.ScoringSummary{
			Scorer: cfg.Plugin, Requested: len(result.Recommendations),
			Unscored: len(result.Recommendations), Warnings: []string{warning},
		}
		if params.scoringDryRun {
			return false, cleanup, errors.New(warning)
		}
		printScoringWarnings(cmd.ErrOrStderr(), result.Scoring)
		return false, cleanup, nil
	}

	done, err := scoreRecommendations(
		ctx, cmd.OutOrStdout(), pbc.NewRecommendationScorerServiceClient(scorer.Conn),
		cfg, store, scorerVersion(scorer), result, params.scoringDryRun,
	)
	if err != nil || done {
		return done, cleanup, err
	}
	printScoringWarnings(cmd.ErrOrStderr(), result.Scoring)
	return false, cleanup, nil
}

func scorerVersion(c *pluginhost.Client) string {
	if c == nil || c.Metadata == nil {
		return ""
	}
	return c.Metadata.Version
}

// scoreRecommendations runs the scoring step and records its summary on result. Scorer
// trouble never fails the command: it becomes warnings in the summary. With dryRun it
// prints the exact requests that would be sent to out and reports done so the caller can
// stop before rendering recommendations.
func scoreRecommendations(
	ctx context.Context,
	out io.Writer,
	scorer scoring.Scorer,
	cfg config.ResolvedScoring,
	store cache.Cache,
	version string,
	result *engine.RecommendationsResult,
	dryRun bool,
) (bool, error) {
	svc := scoring.New(scorer, scoring.OptionsFromConfig(cfg, store, version))
	outcome, err := svc.Score(ctx, result.Recommendations, dryRun)
	if err != nil {
		logging.FromContext(ctx).Warn().Ctx(ctx).Err(err).Str("component", "scoring").
			Msg("scoring step failed; recommendations left unscored")
		result.Scoring = &engine.ScoringSummary{
			Scorer:    cfg.Plugin,
			Requested: len(result.Recommendations),
			Unscored:  len(result.Recommendations),
			Warnings:  []string{"scoring failed: " + err.Error()},
		}
		return false, nil
	}
	result.Scoring = outcome.Summary
	if !dryRun {
		return false, nil
	}
	return true, printScoringDryRun(out, cfg, outcome.Requests)
}

type scoringDryRunOutput struct {
	DryRun         bool              `json:"dry_run"`
	IdentifierMode string            `json:"identifier_mode"`
	Note           string            `json:"note"`
	Requests       []json.RawMessage `json:"requests"`
}

func printScoringDryRun(w io.Writer, cfg config.ResolvedScoring, requests []*pbc.ScoreRecommendationsRequest) error {
	doc := scoringDryRunOutput{
		DryRun:         true,
		IdentifierMode: cfg.IdentifierMode,
		Note: "These requests are exactly what would be sent to the scorer plugin. " +
			"Nothing was sent. Recommendations already in the score cache are not included.",
		Requests: make([]json.RawMessage, 0, len(requests)),
	}
	for _, req := range requests {
		data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(req)
		if err != nil {
			return fmt.Errorf("encoding scoring request: %w", err)
		}
		doc.Requests = append(doc.Requests, data)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("writing scoring dry run: %w", err)
	}
	return nil
}

// printScoringWarnings reports degraded scoring on stderr without failing the command.
func printScoringWarnings(w io.Writer, summary *engine.ScoringSummary) {
	if summary == nil {
		return
	}
	for _, msg := range summary.Warnings {
		fmt.Fprintf(w, "Warning: scoring: %s\n", msg)
	}
}

func countNeedsReview(recs []engine.Recommendation) int {
	n := 0
	for _, r := range recs {
		if r.Scores != nil && r.Scores.NeedsReview {
			n++
		}
	}
	return n
}

func hasScores(recs []engine.Recommendation) bool {
	return slices.ContainsFunc(recs, func(r engine.Recommendation) bool { return r.Scores != nil })
}

func formatScoreCell(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *v)
}

// scoreCells renders the score columns for one table row.
func scoreCells(s *engine.RecommendationScores) string {
	if s == nil {
		return "-\t-\t-\t-\t\t"
	}
	review := ""
	if s.NeedsReview {
		review = "review"
	}
	return strings.Join([]string{
		formatScoreCell(s.Risk), formatScoreCell(s.FalsePositive), formatScoreCell(s.WorthActing),
		formatScoreCell(s.Priority), review, s.DuplicateGroupID,
	}, "\t")
}

func scoringSummaryLine(s *engine.ScoringSummary, recs []engine.Recommendation) string {
	name := s.Scorer
	if s.Model != "" {
		name += "/" + s.Model
	}
	calibration := s.Calibration
	if calibration == "" {
		calibration = "unspecified"
	}
	return fmt.Sprintf("Scoring: %s (%s): scored %d of %d, %d need review. Scores rank work for review; "+
		"they never dismiss or apply a recommendation.\n",
		name, calibration, s.Scored, s.Requested, countNeedsReview(recs))
}

// JSON representations of scores. Keys are omitted when the scorer did not compute them.
type recommendationScoresJSON struct {
	Risk                 *float64 `json:"risk,omitempty"`
	FalsePositive        *float64 `json:"false_positive,omitempty"`
	WorthActing          *float64 `json:"worth_acting,omitempty"`
	Priority             *float64 `json:"priority,omitempty"`
	InsufficientEvidence *float64 `json:"insufficient_evidence,omitempty"`
	DuplicateGroupID     string   `json:"duplicate_group_id,omitempty"`
	NeedsReview          bool     `json:"needs_review"`
}

type scoringJSON struct {
	Scorer      string   `json:"scorer,omitempty"`
	Model       string   `json:"model,omitempty"`
	Calibration string   `json:"calibration,omitempty"`
	Requested   int      `json:"requested"`
	Scored      int      `json:"scored"`
	FromCache   int      `json:"from_cache,omitempty"`
	Unscored    int      `json:"unscored,omitempty"`
	OrderedBy   string   `json:"ordered_by,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

func newScoresJSON(s *engine.RecommendationScores) *recommendationScoresJSON {
	if s == nil {
		return nil
	}
	return &recommendationScoresJSON{
		Risk:                 s.Risk,
		FalsePositive:        s.FalsePositive,
		WorthActing:          s.WorthActing,
		Priority:             s.Priority,
		InsufficientEvidence: s.InsufficientEvidence,
		DuplicateGroupID:     s.DuplicateGroupID,
		NeedsReview:          s.NeedsReview,
	}
}

func newScoringJSON(s *engine.ScoringSummary) *scoringJSON {
	if s == nil {
		return nil
	}
	return &scoringJSON{
		Scorer:      s.Scorer,
		Model:       s.Model,
		Calibration: s.Calibration,
		Requested:   s.Requested,
		Scored:      s.Scored,
		FromCache:   s.FromCache,
		Unscored:    s.Unscored,
		OrderedBy:   s.OrderedBy,
		Warnings:    s.Warnings,
	}
}
