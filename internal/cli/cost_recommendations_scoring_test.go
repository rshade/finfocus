package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	"github.com/rshade/finfocus/internal/cli/pagination"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func f64(v float64) *float64 { return &v }

func scoredRec(id string, risk float64, savings float64) engine.Recommendation {
	return engine.Recommendation{
		ID: id, ResourceID: "res-" + id, Type: "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE",
		Description: "d " + id, EstimatedSavings: savings, Currency: "USD",
		Status: engine.RecommendationStatusActive,
		Scores: &engine.RecommendationScores{Risk: f64(risk), Priority: f64(2), NeedsReview: risk >= 0.3},
	}
}

func TestParseScoreFilter(t *testing.T) {
	tests := []struct {
		expr    string
		want    scoreFilter
		isScore bool
		wantErr bool
	}{
		{"risk<=0.3", scoreFilter{"risk", "<=", 0.3}, true, false},
		{" worth_acting >= 0.5 ", scoreFilter{"worth_acting", ">=", 0.5}, true, false},
		{"priority>1", scoreFilter{"priority", ">", 1}, true, false},
		{"false_positive<0.2", scoreFilter{"false_positive", "<", 0.2}, true, false},
		{"risk=0.5", scoreFilter{"risk", "=", 0.5}, true, false},
		{"action=MIGRATE", scoreFilter{}, false, false},
		{"", scoreFilter{}, false, false},
		{"risk<=abc", scoreFilter{}, true, true},
		{"risk<=", scoreFilter{}, true, true},
		{"risk!=0.3", scoreFilter{}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, isScore, err := parseScoreFilter(tt.expr)
			assert.Equal(t, tt.isScore, isScore)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplyScoreFilters(t *testing.T) {
	recs := []engine.Recommendation{
		scoredRec("a", 0.1, 10),
		scoredRec("b", 0.3, 20),
		scoredRec("c", 0.9, 30),
		{ID: "d", ResourceID: "res-d"},
	}

	got, err := applyScoreFilters(recs, []string{"risk<=0.3", "action=MIGRATE"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "b", got[1].ID)

	got, err = applyScoreFilters(recs, []string{"risk>0.2", "priority>=2"})
	require.NoError(t, err)
	assert.Len(t, got, 2, "filters combine with AND and drop unscored rows")

	got, err = applyScoreFilters(recs, []string{"action=MIGRATE"})
	require.NoError(t, err)
	assert.Len(t, got, 4, "non-score filters leave rows untouched")

	_, err = applyScoreFilters(recs, []string{"risk<=abc"})
	require.Error(t, err)
}

func TestValidateScoringFlags(t *testing.T) {
	disabled := config.ResolvedScoring{}
	enabled := config.ResolvedScoring{Enabled: true, Plugin: "scorer"}

	tests := []struct {
		name    string
		params  costRecommendationsParams
		cfg     config.ResolvedScoring
		wantErr string
	}{
		{"defaults are valid when disabled", costRecommendationsParams{}, disabled, ""},
		{"action sort valid when disabled", costRecommendationsParams{sort: "savings:asc"}, disabled, ""},
		{"score sort needs scoring", costRecommendationsParams{sort: "risk"}, disabled, "requires scoring"},
		{
			"score filter needs scoring",
			costRecommendationsParams{filter: []string{"risk<=0.3"}},
			disabled,
			"requires scoring",
		},
		{"dry run needs scoring", costRecommendationsParams{scoringDryRun: true}, disabled, "scoring.enabled"},
		{
			"no-scoring blocks score sort",
			costRecommendationsParams{sort: "risk", noScoring: true},
			enabled,
			"requires scoring",
		},
		{
			"no-scoring with dry run conflicts",
			costRecommendationsParams{scoringDryRun: true, noScoring: true},
			enabled,
			"cannot be combined",
		},
		{"score sort with scoring", costRecommendationsParams{sort: "priority:desc"}, enabled, ""},
		{
			"bad filter surfaces error",
			costRecommendationsParams{filter: []string{"risk<=abc"}},
			enabled,
			"invalid score filter",
		},
		{"dry run with scoring", costRecommendationsParams{scoringDryRun: true}, enabled, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScoringFlags(tt.params, tt.cfg)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestFindScorerClient(t *testing.T) {
	scorer := &pluginhost.Client{Name: "scorer", Metadata: &proto.PluginMetadata{
		Version: "1.2.3", Capabilities: []string{pluginhost.CapabilityRecommendationScoring},
	}}
	plain := &pluginhost.Client{
		Name:     "plain",
		Metadata: &proto.PluginMetadata{Capabilities: []string{"recommendations"}},
	}

	got, warn := findScorerClient([]*pluginhost.Client{plain, scorer}, "scorer")
	assert.Same(t, scorer, got)
	assert.Empty(t, warn)

	got, warn = findScorerClient([]*pluginhost.Client{plain, scorer}, "missing")
	assert.Nil(t, got)
	assert.Contains(t, warn, "not found")

	got, warn = findScorerClient([]*pluginhost.Client{plain}, "plain")
	assert.Nil(t, got)
	assert.Contains(t, warn, pluginhost.CapabilityRecommendationScoring)
}

func TestScoreRecommendations_EndToEndWithMockScorer(t *testing.T) {
	harness := plugintesting.NewScorerHarness(plugintesting.NewMockRecommendationScorer())
	harness.Start(t)
	t.Cleanup(harness.Stop)

	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		{ID: "r1", ResourceID: "i-1", Type: "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE", Description: "d",
			Status: engine.RecommendationStatusActive},
	}}
	cfg := config.ResolvedScoring{
		Enabled: true, Plugin: "mock", IdentifierMode: config.ScoringIdentifierPseudonymized, TimeoutSeconds: 5,
		RiskThreshold: 0.3, FalsePositiveLimit: 0.3, InsufficientEvidence: 0.5, DeadBand: 0.1,
	}
	var out bytes.Buffer

	done, err := scoreRecommendations(context.Background(), &out, harness.Client(), cfg, nil, "1.0.0", result, false)

	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, result.Scoring)
	assert.Equal(t, 1, result.Scoring.Scored)
	require.NotNil(t, result.Recommendations[0].Scores)
	assert.Empty(t, out.String())
}

func TestScoreRecommendations_DryRunPrintsExactRequestsWithoutIdentifiers(t *testing.T) {
	harness := plugintesting.NewScorerHarness(plugintesting.NewMockRecommendationScorer())
	harness.Start(t)
	t.Cleanup(harness.Stop)

	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		{ID: "plugin-secret-id", ResourceID: "i-0abc123def456", Type: "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE",
			Description: "resize i-0abc123def456", Status: engine.RecommendationStatusActive,
			ResourceInfo: &engine.RecommendationResourceInfo{Name: "prod-db-primary", Provider: "aws"}},
	}}
	cfg := config.ResolvedScoring{Enabled: true, Plugin: "mock", IdentifierMode: config.ScoringIdentifierPseudonymized,
		TimeoutSeconds: 5}
	var out bytes.Buffer

	done, err := scoreRecommendations(context.Background(), &out, harness.Client(), cfg, nil, "", result, true)

	require.NoError(t, err)
	assert.True(t, done)
	text := out.String()
	assert.NotContains(t, text, "i-0abc123def456")
	assert.NotContains(t, text, "prod-db-primary")
	assert.NotContains(t, text, "plugin-secret-id")

	var decoded struct {
		Requests []map[string]any `json:"requests"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &decoded))
	require.Len(t, decoded.Requests, 1)
	assert.Nil(t, result.Recommendations[0].Scores, "dry run scores nothing")
}

func TestScoreRecommendations_ScorerFailureIsAWarningNotAnError(t *testing.T) {
	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		{ID: "r1", ResourceID: "i-1", Status: engine.RecommendationStatusActive},
	}}
	cfg := config.ResolvedScoring{Enabled: true, Plugin: "mock", IdentifierMode: config.ScoringIdentifierPseudonymized,
		TimeoutSeconds: 1}

	done, err := scoreRecommendations(
		context.Background(),
		&bytes.Buffer{},
		failingScorer{},
		cfg,
		nil,
		"",
		result,
		false,
	)

	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, result.Scoring)
	assert.Equal(t, 1, result.Scoring.Unscored)
	assert.NotEmpty(t, result.Scoring.Warnings)
	assert.Nil(t, result.Recommendations[0].Scores)
	assert.Len(t, result.Recommendations, 1)
}

func renderRecsTable(t *testing.T, result *engine.RecommendationsResult, verbose bool) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsTableWithVerbose(&buf, result, verbose))
	return buf.String()
}

func TestRenderTable_ScoreColumnsOnlyWhenScored(t *testing.T) {
	plain := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		{ID: "a", ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 5},
	}}
	out := renderRecsTable(t, plain, true)
	assert.NotContains(t, out, "RISK")
	assert.NotContains(t, out, "REVIEW")

	scored := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{scoredRec("a", 0.42, 5)},
		Scoring:         &engine.ScoringSummary{Scorer: "mock", Calibration: "ranking_only", Requested: 1, Scored: 1},
	}
	scored.Recommendations[0].Scores.DuplicateGroupID = "dup-1"
	out = renderRecsTable(t, scored, true)
	assert.Contains(t, out, "RISK")
	assert.Contains(t, out, "0.42")
	assert.Contains(t, out, "review")
	assert.Contains(t, out, "dup-1")
	assert.Contains(t, out, "Scoring: mock (ranking_only)")
	assert.Contains(t, out, "1 need review")
}

func TestRenderTable_KeepsScoreOrderWhenSorted(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{
			scoredRec("low-savings-high-risk", 0.9, 1),
			scoredRec("high-savings-low-risk", 0.1, 100),
		},
		Scoring: &engine.ScoringSummary{Scored: 2, Requested: 2, OrderedBy: "risk"},
	}

	out := renderRecsTable(t, result, true)

	assert.Contains(t, out, "SORTED BY RISK")
	assert.Less(t, strings.Index(out, "res-low-savings-high-risk"), strings.Index(out, "res-high-savings-low-risk"))
}

func TestRenderTable_NonVerboseHeaderNamesTheSort(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{scoredRec("a", 0.9, 1)},
		Scoring:         &engine.ScoringSummary{Scored: 1, Requested: 1, OrderedBy: "worth_acting"},
	}

	assert.Contains(t, renderRecsTable(t, result, false), "TOP 1 RECOMMENDATIONS BY WORTH ACTING")
}

func TestRenderTable_SortsBySavingsWithoutScoreSort(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{scoredRec("small", 0.9, 1), scoredRec("big", 0.1, 100)},
		Scoring:         &engine.ScoringSummary{Scored: 2, Requested: 2},
	}

	out := renderRecsTable(t, result, true)

	assert.Contains(t, out, "SORTED BY SAVINGS")
	assert.Less(t, strings.Index(out, "res-big"), strings.Index(out, "res-small"))
}

func TestRenderJSON_ScoresAndSummary(t *testing.T) {
	rec := scoredRec("a", 0.42, 5)
	rec.Scores.DuplicateGroupID = "dup-1"
	rec.Scores.FalsePositive = f64(0.2)
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{rec},
		Scoring: &engine.ScoringSummary{
			Scorer: "mock", Model: "m1", Calibration: "ranking_only", Requested: 1, Scored: 1,
			Warnings: []string{"careful"},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsJSON(&buf, result, nil))

	var out struct {
		Scoring         map[string]any   `json:"scoring"`
		Recommendations []map[string]any `json:"recommendations"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	scores, ok := out.Recommendations[0]["scores"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 0.42, scores["risk"], 1e-9)
	assert.InDelta(t, 0.2, scores["false_positive"], 1e-9)
	assert.InDelta(t, 2.0, scores["priority"], 1e-9)
	assert.Equal(t, "dup-1", scores["duplicate_group_id"])
	assert.Equal(t, true, scores["needs_review"])
	assert.NotContains(t, scores, "worth_acting")

	assert.Equal(t, "mock", out.Scoring["scorer"])
	assert.Equal(t, "m1", out.Scoring["model"])
	assert.Equal(t, "ranking_only", out.Scoring["calibration"])
	assert.InDelta(t, 1.0, out.Scoring["scored"], 1e-9)
	assert.Equal(t, []any{"careful"}, out.Scoring["warnings"])
}

func TestRenderJSON_NoScoringKeysWhenUnscored(t *testing.T) {
	result := &engine.RecommendationsResult{Recommendations: []engine.Recommendation{
		{ResourceID: "r", Type: "RIGHTSIZE", Description: "d"},
	}}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsJSON(&buf, result, nil))

	assert.NotContains(t, buf.String(), `"scores"`)
	assert.NotContains(t, buf.String(), `"scoring"`)
}

func TestRenderNDJSON_ScoresAndSummary(t *testing.T) {
	result := &engine.RecommendationsResult{
		Recommendations: []engine.Recommendation{scoredRec("a", 0.42, 5)},
		Scoring:         &engine.ScoringSummary{Scorer: "mock", Requested: 1, Scored: 1},
	}

	var buf bytes.Buffer
	require.NoError(t, renderRecommendationsNDJSON(&buf, result, &pagination.PaginationMeta{}))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"scoring"`)
	assert.Contains(t, lines[1], `"scores"`)
	assert.Contains(t, lines[1], `"needs_review":true`)
}

func TestNewCostRecommendationsCmd_ScoringFlags(t *testing.T) {
	cmd := NewCostRecommendationsCmd()

	for _, name := range []string{"scoring-dry-run", "no-scoring"} {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag, name)
		assert.Equal(t, "false", flag.DefValue, "scoring stays off unless configured or requested")
	}
}

type failingScorer struct{}

func (failingScorer) ScoreRecommendations(
	context.Context,
	*pbc.ScoreRecommendationsRequest,
	...grpc.CallOption,
) (*pbc.ScoreRecommendationsResponse, error) {
	return nil, status.Error(codes.Unavailable, "backend down")
}
