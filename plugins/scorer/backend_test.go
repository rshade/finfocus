package scorer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/plugins/scorer/internal/backends/jev"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
)

// These are recorded Clef values replayed through both wire formats. Only
// answer keys are remapped to the scorer's generated questions; this does not
// claim that Clef or Jev answered these recommendation prompts live.
func recordedServer(t *testing.T, backend string) *httptest.Server {
	t.Helper()
	raw, err := os.ReadFile("testdata/clef-response.json")
	require.NoError(t, err)
	var envelope struct {
		Result struct {
			Model   string                     `json:"model"`
			Answers map[string]json.RawMessage `json:"answers"`
			Usage   jev.Usage                  `json:"usage"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &envelope))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, model := "/v1/systemone", "jev-1.13.0"
		if backend == "clef" {
			path = "/client/v4/accounts/0123456789abcdef0123456789abcdef/ai/run/@cf/cloudflare/clef"
			model = "clef"
		}
		assert.Equal(t, path, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer dummy-replay-token", r.Header.Get("Authorization"))
		var req jev.Request
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(t, model, req.Model)
		answers := make(map[string]json.RawMessage, len(req.Questions))
		for name, q := range req.Questions {
			source := "urgent"
			if q.Type == "score" {
				source = "severity"
			} else {
				assert.Equal(t, "noul", q.Type)
			}
			answers[name] = envelope.Result.Answers[source]
		}
		result := map[string]any{"model": envelope.Result.Model, "answers": answers, "usage": envelope.Result.Usage}
		var payload any = result
		if backend == "clef" {
			payload = map[string]any{"result": result, "success": true, "errors": []any{}, "messages": []any{}}
		}
		assert.NoError(t, json.NewEncoder(w).Encode(payload))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func backendConfig(t *testing.T, backend, origin string) Config {
	t.Helper()
	cfg, err := ConfigFromEnv(env(map[string]string{
		EnvBackend: backend, EnvAPIKey: "dummy-replay-token", EnvBaseURL: origin,
		EnvClefAccountID: "0123456789abcdef0123456789abcdef",
	}))
	require.NoError(t, err)
	return cfg
}

func TestBackends_RecordedScorerContract(t *testing.T) {
	t.Parallel()
	for _, backend := range []string{"jev", "clef"} {
		t.Run(backend, func(t *testing.T) {
			t.Parallel()
			p, err := New(backendConfig(t, backend, recordedServer(t, backend).URL))
			require.NoError(t, err)
			plugintesting.RunScorerConformance(t, p)
			req := &pbc.ScoreRecommendationsRequest{Recommendations: []*pbc.Recommendation{
				{Id: "a", Description: "Resize", Resource: &pbc.ResourceRecommendationInfo{Id: "shared"}},
				{Id: "b", Description: "Resize again", Resource: &pbc.ResourceRecommendationInfo{Id: "shared"}},
			}}
			resp, err := p.ScoreRecommendations(t.Context(), req)
			require.NoError(t, err)
			require.NoError(t, plugintesting.ValidateScoreRecommendationsResponse(req, resp))
			require.Len(t, resp.GetResults(), 2)
			for _, result := range resp.GetResults() {
				scores := result.GetScores()
				require.NotNil(t, scores)
				assert.InDelta(t, 0.9905, scores.GetRisk(), 1e-9)
				assert.InDelta(t, 0.9905, scores.GetFalsePositive(), 1e-9)
				assert.InDelta(t, 0.9905, scores.GetWorthActing(), 1e-9)
				assert.InDelta(t, 0.9905, scores.GetInsufficientEvidence(), 1e-9)
				assert.InDelta(t, 2.9571, scores.GetPriority(), 1e-9)
				assert.NotEmpty(t, scores.GetDuplicateGroupId())
			}
			assert.Equal(
				t,
				resp.GetResults()[0].GetScores().GetDuplicateGroupId(),
				resp.GetResults()[1].GetScores().GetDuplicateGroupId(),
			)
			assert.Equal(t, "scorer", resp.GetScorer().GetName())
			assert.Equal(
				t,
				"clef",
				resp.GetScorer().GetModel(),
				"report the returned model rather than the configured alias",
			)
			assert.Equal(t, pbc.ScoreCalibration_SCORE_CALIBRATION_RANKING_ONLY, resp.GetScorer().GetCalibration())
		})
	}
}

func TestBackends_RejectRedirectsAndPropagateCancellation(t *testing.T) {
	t.Parallel()
	for _, backend := range []string{"jev", "clef"} {
		t.Run(backend, func(t *testing.T) {
			t.Parallel()
			var hits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			}))
			defer origin.Close()
			client, err := newBackend(backendConfig(t, backend, origin.URL))
			require.NoError(t, err)
			_, err = client.SystemOne(t.Context(), jev.Request{})
			require.Error(t, err)
			assert.Zero(t, hits.Load(), "no redirected request may carry the token")
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			_, err = client.SystemOne(ctx, jev.Request{})
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestBackends_ConfigurationValidation(t *testing.T) {
	t.Parallel()
	for _, values := range []map[string]string{
		{EnvBackend: "clef", EnvClefAccountID: ""},
		{EnvBackend: "clef", EnvClefAccountID: "../account"},
		{EnvBackend: "clef", EnvBaseURL: "http://remote.example.com"},
		{EnvBackend: "clef", EnvBaseURL: "https://example.com/path"},
		{EnvBackend: "clef", EnvBaseURL: "https://example.com?query=x"},
		{EnvBackend: "jev", EnvBaseURL: "https://user:password@example.com"},
	} {
		cfg, err := ConfigFromEnv(env(values))
		require.NoError(t, err)
		cfg.APIKey = "dummy-validation-token"
		_, err = New(cfg)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), cfg.APIKey)
	}
	for _, name := range []string{"jev", "clef"} {
		cfg, err := ConfigFromEnv(env(map[string]string{EnvBackend: name}))
		require.NoError(t, err)
		p, err := New(cfg)
		require.NoError(t, err)
		_, err = p.ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
			Recommendations: []*pbc.Recommendation{{Id: "r1", Description: "Resize"}},
		})
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	}
}

func TestClef_FailedEnvelopeIsNotAZeroScore(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/clef-response.json")
	require.NoError(t, err)
	failed := strings.Replace(string(raw), `"success":true`, `"success":false`, 1)
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(failed)) }),
	)
	defer srv.Close()
	p, err := New(backendConfig(t, "clef", srv.URL))
	require.NoError(t, err)
	_, err = p.ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
		Recommendations: []*pbc.Recommendation{{Id: "r1", Description: "Resize"}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestBackend_MixedEnvironmentAndFileSettings(t *testing.T) {
	t.Parallel()

	// The host uses generic names before discovery, then advertised names
	// after restarting the plugin. Both launches must accept the same settings.
	for _, accountEnv := range []string{"FINFOCUS_PLUGIN_SCORER_CLEF_ACCOUNT_ID", EnvClefAccountID} {
		t.Run(accountEnv, func(t *testing.T) {
			t.Parallel()
			cfg, err := ConfigFromEnv(env(map[string]string{
				EnvBackend: "clef", EnvAPIKey: "dummy-replay-token",
				EnvBaseURL: recordedServer(t, "clef").URL,
				accountEnv: "0123456789abcdef0123456789abcdef",
			}))
			require.NoError(t, err)
			plugin, err := New(cfg)
			require.NoError(t, err, "plugin must start before the host can discover settings")
			resp, err := plugin.ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
				Recommendations: []*pbc.Recommendation{{Id: "mixed-config", Description: "Resize"}},
			})
			require.NoError(t, err)
			require.Len(t, resp.GetResults(), 1)
			assert.Equal(t, "mixed-config", resp.GetResults()[0].GetRecommendationId())
		})
	}
}
