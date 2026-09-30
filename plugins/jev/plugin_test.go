package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const secretKey = "sk-test-secret-value"

// fakeJev answers every question: yes-probability 0.3, rating 1.5, and 0.9 for
// duplicate comparisons so that recommendations on one resource group.
func fakeJev(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"nope"}`))
			return
		}
		var req struct {
			Questions map[string]struct {
				Type string `json:"type"`
			} `json:"questions"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		answers := map[string]any{}
		for name, q := range req.Questions {
			switch {
			case q.Type == "score":
				answers[name] = map[string]any{"type": "score", "score": 1.5, "confidence": 0.9}
			case strings.HasPrefix(name, "duplicate:"):
				answers[name] = map[string]any{"type": "noul", "noul": 0.9}
			default:
				answers[name] = map[string]any{"type": "noul", "noul": 0.3}
			}
		}
		w.Header().Set("X-Typesafe-Request-Id", "req-fake")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"model":   "jev-1.13.0",
			"answers": answers,
			"usage":   map[string]int{"input_tokens": 10, "output_tokens": 1},
		}))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestPlugin(t *testing.T, srv *httptest.Server, key string) *Plugin {
	t.Helper()
	cfg := DefaultConfig()
	cfg.APIKey = key
	if srv != nil {
		cfg.BaseURL = srv.URL
	}
	p, err := New(cfg)
	require.NoError(t, err)
	return p
}

func TestInfo_DeclaresOnlyScoring(t *testing.T) {
	info := Info("v0.1.0")
	assert.Equal(t, "jev", info.Name)
	assert.Equal(t, []pbc.PluginCapability{pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATION_SCORING},
		info.Capabilities)
	require.NoError(t, info.Validate())
}

func TestPlugin_ImplementsScorerProvider(t *testing.T) {
	var p any = newTestPlugin(t, nil, "")
	_, ok := p.(pluginsdk.RecommendationScorerProvider)
	assert.True(t, ok)
}

func TestScorerConformance(t *testing.T) {
	plugintesting.RunScorerConformance(t, newTestPlugin(t, fakeJev(t, http.StatusOK), secretKey))
}

func TestSupports_OptsOutOfPricing(t *testing.T) {
	srv := pluginsdk.NewServerWithOptions(newTestPlugin(t, nil, ""), nil, nil, Info("v0.1.0"))
	resp, err := srv.Supports(context.Background(), &pbc.SupportsRequest{
		Resource: &pbc.ResourceDescriptor{ResourceType: "aws:ec2/instance:Instance"},
	})
	require.NoError(t, err)
	assert.False(t, resp.GetSupported())
}

func TestNew_WithoutKeyStartsAndScoringIsUnauthenticated(t *testing.T) {
	p := newTestPlugin(t, nil, "")
	rec := &pbc.Recommendation{Id: "r1", Description: "x"}
	_, err := p.ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
		Recommendations: []*pbc.Recommendation{rec},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestNew_RejectsUnsafeBaseURLOnlyWhenKeyIsSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "http://remote.example.com"
	_, err := New(cfg)
	require.NoError(t, err, "without a key nothing is ever sent")

	cfg.APIKey = secretKey
	_, err = New(cfg)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey)
}

func TestScoreRecommendations_BackendRejectingKeyIsUnauthenticated(t *testing.T) {
	p := newTestPlugin(t, fakeJev(t, http.StatusUnauthorized), secretKey)
	_, err := p.ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
		Recommendations: []*pbc.Recommendation{{Id: "r1"}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.NotContains(t, err.Error(), secretKey)
}

func TestScoreRecommendations_NothingObservableContainsTheKey(t *testing.T) {
	var logs bytes.Buffer
	cfg := DefaultConfig()
	cfg.APIKey = secretKey
	cfg.Logger = zerolog.New(&logs)

	for _, code := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity} {
		cfg.BaseURL = fakeJev(t, code).URL
		p, err := New(cfg)
		require.NoError(t, err)
		resp, callErr := p.ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
			Recommendations: []*pbc.Recommendation{{Id: "r1", Description: "x"}},
		})
		var out strings.Builder
		if callErr != nil {
			out.WriteString(callErr.Error())
		}
		if resp != nil {
			raw, marshalErr := json.Marshal(resp)
			require.NoError(t, marshalErr)
			out.Write(raw)
		}
		assert.NotContains(t, out.String(), secretKey, "status %d", code)
	}
	assert.NotContains(t, logs.String(), secretKey)
}

func TestScoreRecommendations_WithoutKeyIsUnauthenticatedOverGRPC(t *testing.T) {
	harness := plugintesting.NewScorerHarness(newTestPlugin(t, nil, ""))
	harness.Start(t)
	defer harness.Stop()

	_, err := harness.Client().ScoreRecommendations(t.Context(), &pbc.ScoreRecommendationsRequest{
		Recommendations: []*pbc.Recommendation{{Id: "r1", Description: "x"}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
