package jev

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"
	"github.com/rshade/finfocus/plugins/jev/internal/scoring"

	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	envEval        = "JEV_EVAL"
	pricePerMTok   = 0.042
	microPerUSD    = 1e6
	testdataDir    = "testdata"
	priorityCutoff = 2.0
)

// countingBackend sums billed input tokens so evaluation runs can report
// spend.
type countingBackend struct {
	inner  scoring.Backend
	tokens int
}

func (c *countingBackend) SystemOne(ctx context.Context, req jevapi.Request) (*jevapi.Response, error) {
	resp, err := c.inner.SystemOne(ctx, req)
	if resp != nil {
		c.tokens += resp.Usage.InputTokens
	}
	return resp, err
}

func (c *countingBackend) spendUSD() float64 {
	return float64(c.tokens) * pricePerMTok / microPerUSD
}

func liveScorer(t *testing.T) (*scoring.Scorer, *countingBackend) {
	t.Helper()
	cfg, err := ConfigFromEnv(os.Getenv)
	require.NoError(t, err)
	if cfg.APIKey == "" {
		t.Skipf("set %s to run live tests", EnvAPIKey)
	}
	client, err := jevapi.New(jevapi.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Timeout: cfg.Timeout})
	require.NoError(t, err)
	backend := &countingBackend{inner: client}
	return scoring.New(
		backend,
		scoring.Config{Model: cfg.Model, BatchSize: cfg.BatchSize, DuplicateThreshold: cfg.DuplicateThreshold},
	), backend
}

type dataRec struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Category        string  `json:"category"`
	ActionType      string  `json:"action_type"`
	Description     string  `json:"description"`
	ConfidenceScore float64 `json:"confidence_score"`
	Resource        struct {
		ID           string            `json:"id"`
		Name         string            `json:"name"`
		Provider     string            `json:"provider"`
		ResourceType string            `json:"resource_type"`
		Region       string            `json:"region"`
		SKU          string            `json:"sku"`
		Tags         map[string]string `json:"tags"`
		Utilization  struct {
			CPU     float64 `json:"cpu_percent"`
			Memory  float64 `json:"memory_percent"`
			Storage float64 `json:"storage_percent"`
		} `json:"utilization"`
	} `json:"resource"`
	Impact struct {
		EstimatedSavings float64 `json:"estimated_savings"`
		CurrentCost      float64 `json:"current_cost"`
		ProjectedCost    float64 `json:"projected_cost"`
		SavingsPct       float64 `json:"savings_percentage"`
		ImplementationCo float64 `json:"implementation_cost"`
		EffortHours      float64 `json:"migration_effort_hours"`
	} `json:"impact"`
	Metadata map[string]string `json:"metadata"`
}

func (d dataRec) proto(t *testing.T) *pbc.Recommendation {
	t.Helper()
	category, ok := pbc.RecommendationCategory_value["RECOMMENDATION_CATEGORY_"+d.Category]
	require.True(t, ok, d.Category)
	action, ok := pbc.RecommendationActionType_value["RECOMMENDATION_ACTION_TYPE_"+d.ActionType]
	require.True(t, ok, d.ActionType)
	return &pbc.Recommendation{
		Id:              d.ID,
		Category:        pbc.RecommendationCategory(category),
		ActionType:      pbc.RecommendationActionType(action),
		Description:     d.Description,
		Source:          d.Source,
		ConfidenceScore: proto.Float64(d.ConfidenceScore),
		Metadata:        d.Metadata,
		Resource: &pbc.ResourceRecommendationInfo{
			Id: d.Resource.ID, Name: d.Resource.Name, Provider: d.Resource.Provider,
			ResourceType: d.Resource.ResourceType, Region: d.Resource.Region, Sku: d.Resource.SKU,
			Tags: d.Resource.Tags,
			Utilization: &pbc.ResourceUtilization{
				CpuPercent:     d.Resource.Utilization.CPU,
				MemoryPercent:  d.Resource.Utilization.Memory,
				StoragePercent: d.Resource.Utilization.Storage,
			},
		},
		Impact: &pbc.RecommendationImpact{
			EstimatedSavings:     d.Impact.EstimatedSavings,
			CurrentCost:          d.Impact.CurrentCost,
			ProjectedCost:        d.Impact.ProjectedCost,
			SavingsPercentage:    d.Impact.SavingsPct,
			ImplementationCost:   proto.Float64(d.Impact.ImplementationCo),
			MigrationEffortHours: proto.Float64(d.Impact.EffortHours),
		},
	}
}

type label struct {
	ID       string `json:"id"`
	Safe     bool   `json:"safe_unattended"`
	FalsePos bool   `json:"false_positive"`
	Priority int    `json:"priority"`
}

func loadJSON[T any](t *testing.T, name string) T {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdataDir, name))
	require.NoError(t, err)
	var v T
	require.NoError(t, json.Unmarshal(raw, &v))
	return v
}

func TestTestdataConvertsToRecommendations(t *testing.T) {
	t.Parallel()

	recs := loadJSON[[]dataRec](t, "recommendations.json")
	require.Len(t, recs, 80)
	seen := map[string]bool{}
	for _, r := range recs {
		p := r.proto(t)
		assert.False(t, seen[p.GetId()], p.GetId())
		seen[p.GetId()] = true
		_, err := protojson.Marshal(p)
		require.NoError(t, err)
	}
	pairs := loadJSON[[]struct {
		A    dataRec `json:"a"`
		B    dataRec `json:"b"`
		Same bool    `json:"same_action"`
	}](t, "rec_pairs.json")
	assert.Len(t, pairs, 40)
	assert.Len(t, loadJSON[[]label](t, "labels_a.json"), 80)
	assert.Len(t, loadJSON[[]label](t, "labels_b.json"), 80)
}

func TestLive_ScoresRecommendations(t *testing.T) {
	t.Parallel()

	scorer, backend := liveScorer(t)
	data := loadJSON[[]dataRec](t, "recommendations.json")
	req := &pbc.ScoreRecommendationsRequest{}
	for _, d := range data[:3] {
		req.Recommendations = append(req.Recommendations, d.proto(t))
	}

	resp, err := scorer.Score(t.Context(), req)
	require.NoError(t, err)
	require.NoError(t, plugintesting.ValidateScoreRecommendationsResponse(req, resp))
	for _, r := range resp.GetResults() {
		require.NotNil(t, r.GetScores(), "result for %s: %v", r.GetRecommendationId(), r.GetError())
		assert.NotNil(t, r.GetScores().Risk)
		assert.NotNil(t, r.GetScores().Priority)
	}
	assert.Equal(t, "jev", resp.GetScorer().GetName())
	assert.Equal(t, DefaultModel, resp.GetScorer().GetModel(), "the pinned model answered")
	t.Logf("model=%s ids=%v tokens=%d spend=$%.5f",
		resp.GetScorer().GetModel(), resp.GetScorer().GetProviderRequestIds(),
		backend.tokens, backend.spendUSD())
}

func TestLive_ListsModels(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromEnv(os.Getenv)
	require.NoError(t, err)
	if cfg.APIKey == "" {
		t.Skipf("set %s to run live tests", EnvAPIKey)
	}
	client, err := jevapi.New(jevapi.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Timeout: cfg.Timeout})
	require.NoError(t, err)
	models, err := client.Models(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, models)
	for _, m := range models {
		t.Logf("model %s released %s", m.Name, m.ReleaseDate)
	}
}

// TestEvaluation scores the labelled synthetic dataset and reports ranking
// quality. It costs a few cents, so it needs JEV_EVAL=1 as well as a key.
func TestEvaluation(t *testing.T) {
	t.Parallel()

	if os.Getenv(envEval) != "1" {
		t.Skipf("set %s=1 and %s to run the evaluation", envEval, EnvAPIKey)
	}
	scorer, backend := liveScorer(t)
	ctx := t.Context()

	data := loadJSON[[]dataRec](t, "recommendations.json")
	labelsA := indexLabels(loadJSON[[]label](t, "labels_a.json"))
	labelsB := indexLabels(loadJSON[[]label](t, "labels_b.json"))
	req := &pbc.ScoreRecommendationsRequest{}
	for _, d := range data {
		req.Recommendations = append(req.Recommendations, d.proto(t))
	}
	resp, err := scorer.Score(ctx, req)
	require.NoError(t, err)
	require.NoError(t, plugintesting.ValidateScoreRecommendationsResponse(req, resp))

	var risk, fp, worth, prio, meanPrio []float64
	var unsafeLbl, fpLbl, worthLbl []bool
	for i, r := range resp.GetResults() {
		sc := r.GetScores()
		require.NotNil(t, sc, "result %d: %v", i, r.GetError())
		a, b := labelsA[data[i].ID], labelsB[data[i].ID]
		pm := float64(a.Priority+b.Priority) / 2
		worth = append(worth, sc.GetWorthActing())
		worthLbl = append(worthLbl, pm >= priorityCutoff)
		prio = append(prio, sc.GetPriority())
		meanPrio = append(meanPrio, pm)
		if a.Safe == b.Safe {
			risk = append(risk, sc.GetRisk())
			unsafeLbl = append(unsafeLbl, !a.Safe)
		}
		if a.FalsePos == b.FalsePos {
			fp = append(fp, sc.GetFalsePositive())
			fpLbl = append(fpLbl, a.FalsePos)
		}
	}
	t.Logf(
		"risk AUC %.2f (n=%d)  false_positive AUC %.2f (n=%d)  worth_acting AUC %.2f (n=%d)  priority Spearman %.2f",
		auc(
			risk,
			unsafeLbl,
		),
		len(risk),
		auc(fp, fpLbl),
		len(fp),
		auc(worth, worthLbl),
		len(worth),
		spearman(prio, meanPrio),
	)
	t.Logf("risk Brier %.3f (0.25 = uninformative)", brier(risk, unsafeLbl))
	t.Logf("model=%s calibration=%s", resp.GetScorer().GetModel(), resp.GetScorer().GetCalibration())

	pairs := loadJSON[[]struct {
		A    dataRec `json:"a"`
		B    dataRec `json:"b"`
		Same bool    `json:"same_action"`
	}](t, "rec_pairs.json")
	dupSignal := []pbc.ScoreSignal{pbc.ScoreSignal_SCORE_SIGNAL_DUPLICATE_GROUP}
	correct, merged, missed := 0, 0, 0
	for _, p := range pairs {
		pr := &pbc.ScoreRecommendationsRequest{
			Recommendations: []*pbc.Recommendation{p.A.proto(t), p.B.proto(t)},
			Signals:         dupSignal,
		}
		pres, callErr := scorer.Score(ctx, pr)
		require.NoError(t, callErr)
		ga := pres.GetResults()[0].GetScores().GetDuplicateGroupId()
		gb := pres.GetResults()[1].GetScores().GetDuplicateGroupId()
		predicted := ga != "" && ga == gb
		switch {
		case predicted == p.Same:
			correct++
		case predicted:
			merged++
		default:
			missed++
		}
	}
	t.Logf("duplicate accuracy, one pair per request: %d/%d (false merges %d, missed duplicates %d)",
		correct, len(pairs), merged, missed)

	all := &pbc.ScoreRecommendationsRequest{Signals: dupSignal}
	for _, p := range pairs {
		all.Recommendations = append(all.Recommendations, p.A.proto(t), p.B.proto(t))
	}
	allResp, err := scorer.Score(ctx, all)
	require.NoError(t, err)
	batchedCorrect := 0
	for i, p := range pairs {
		ga := allResp.GetResults()[2*i].GetScores().GetDuplicateGroupId()
		gb := allResp.GetResults()[2*i+1].GetScores().GetDuplicateGroupId()
		if (ga != "" && ga == gb) == p.Same {
			batchedCorrect++
		}
	}
	t.Logf("duplicate accuracy, pairs batched in one request: %d/%d", batchedCorrect, len(pairs))
	t.Logf("total input tokens %d, spend $%.4f", backend.tokens, backend.spendUSD())

	assert.False(t, math.IsNaN(auc(risk, unsafeLbl)))
	assert.Less(t, backend.spendUSD(), 0.25, "evaluation should stay cheap")
}

func indexLabels(ls []label) map[string]label {
	m := make(map[string]label, len(ls))
	for _, l := range ls {
		m[l.ID] = l
	}
	return m
}
