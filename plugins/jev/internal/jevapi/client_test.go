package jevapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"
)

const testKey = "sk-test-secret-value"

const okBody = `{"model":"jev-1.13.0","answers":{"risk:a":{"type":"noul","noul":0.25},` +
	`"priority:a":{"type":"score","score":1.7,"confidence":0.9,"probabilities":{"0":0.1,"1":0.2,"2":0.7},` +
	`"legend":{"0":"Ignore","1":"Low","2":"Medium"}}},"usage":{"input_tokens":120,"output_tokens":12}}`

func newClient(t *testing.T, srv *httptest.Server, mutate func(*jevapi.Config)) *jevapi.Client {
	t.Helper()
	cfg := jevapi.Config{
		BaseURL:    srv.URL,
		APIKey:     testKey,
		Timeout:    2 * time.Second,
		MaxRetries: 3,
		Sleep:      func(context.Context, time.Duration) error { return nil },
	}
	if mutate != nil {
		mutate(&cfg)
	}
	c, err := jevapi.New(cfg)
	require.NoError(t, err)
	return c
}

func sampleRequest() jevapi.Request {
	return jevapi.Request{
		Model: "jev-1.13.0",
		State: []any{map[string]any{"position": 0, "description": "x"}},
		Questions: map[string]jevapi.Question{
			"risk:a":     jevapi.NoulQuestion("Is it risky?", "risky", "safe"),
			"priority:a": jevapi.ScoreQuestion("How important?", "Ignore", "Low", "Medium"),
		},
	}
}

func TestClient_SystemOneSuccess(t *testing.T) {
	var gotAuth, gotPath, gotMethod, gotType string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotMethod = r.Header.Get("Authorization"), r.URL.Path, r.Method
		gotType = r.Header.Get("Content-Type")
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("X-Typesafe-Request-Id", "req-123")
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	resp, err := newClient(t, srv, nil).SystemOne(t.Context(), sampleRequest())
	require.NoError(t, err)

	assert.Equal(t, "Bearer "+testKey, gotAuth)
	assert.Equal(t, "/v1/systemone", gotPath)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotType)
	assert.Equal(t, "jev-1.13.0", gotBody["model"])
	assert.Contains(t, gotBody, "state")
	assert.Contains(t, gotBody, "questions")

	assert.Equal(t, "jev-1.13.0", resp.Model)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, 120, resp.Usage.InputTokens)
	require.Contains(t, resp.Answers, "risk:a")
	assert.InDelta(t, 0.25, resp.Answers["risk:a"].Noul(), 1e-9)
	assert.InDelta(t, 1.7, resp.Answers["priority:a"].ScoreValue(), 1e-9)
	assert.True(t, resp.Answers["risk:a"].HasNoul())
	assert.False(t, resp.Answers["risk:a"].HasScore())
}

func TestClient_QuestionWireShape(t *testing.T) {
	b, err := json.Marshal(map[string]jevapi.Question{
		"n": jevapi.NoulQuestion("q?", "yes", "no"),
		"s": jevapi.ScoreQuestion("rate", "A", "B"),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"n":{"type":"noul","instructions":"q?","criteria":{"true":"yes","false":"no"}},`+
		`"s":{"type":"score","instructions":"rate","criteria":["A","B"]}}`, string(b))
}

func TestClient_ErrorStatuses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantKind  jevapi.Kind
		wantInMsg string
	}{
		{"unauthorized", http.StatusUnauthorized, `{"detail":"bad key"}`, jevapi.KindUnauthenticated, "bad key"},
		{"forbidden", http.StatusForbidden, `{"detail":"nope"}`, jevapi.KindPermissionDenied, "nope"},
		{"validation", http.StatusUnprocessableEntity, `{"detail":[{"loc":["body","q"],"msg":"too short"}]}`,
			jevapi.KindInvalidRequest, "too short"},
		{"server error", http.StatusInternalServerError, `oops`, jevapi.KindUnavailable, "500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("X-Typesafe-Request-Id", "req-err")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := newClient(t, srv, nil).SystemOne(t.Context(), sampleRequest())
			require.Error(t, err)
			var apiErr *jevapi.APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tt.wantKind, apiErr.Kind)
			assert.Equal(t, tt.status, apiErr.Status)
			assert.Equal(t, "req-err", apiErr.RequestID)
			assert.Contains(t, apiErr.Error(), tt.wantInMsg)
			assert.NotContains(t, apiErr.Error(), testKey)
			assert.Equal(t, int32(1), calls.Load(), "%s must not be retried", tt.name)
		})
	}
}

func TestClient_RetriesHonourRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(529)
		default:
			_, _ = w.Write([]byte(okBody))
		}
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := newClient(t, srv, func(cfg *jevapi.Config) {
		cfg.Sleep = func(_ context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		}
	})
	resp, err := c.SystemOne(t.Context(), sampleRequest())
	require.NoError(t, err)
	assert.Equal(t, "jev-1.13.0", resp.Model)
	assert.Equal(t, int32(3), calls.Load())
	assert.Equal(t, []time.Duration{3 * time.Second, time.Second}, sleeps)
}

func TestClient_RetryThatOutlastsDeadlineFailsFast(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter string
		deadline   time.Duration
		wantCalls  int32
		wantSleeps int
	}{
		{"wait past deadline returns the rate limit", "30", time.Second, 1, 0},
		{"wait inside deadline still retries", "1", time.Minute, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", tt.retryAfter)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write([]byte(okBody))
			}))
			defer srv.Close()

			sleeps := 0
			c := newClient(t, srv, func(cfg *jevapi.Config) {
				cfg.Sleep = func(context.Context, time.Duration) error {
					sleeps++
					return nil
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), tt.deadline)
			defer cancel()
			_, err := c.SystemOne(ctx, sampleRequest())
			if tt.wantCalls == 1 {
				var apiErr *jevapi.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, jevapi.KindRateLimited, apiErr.Kind)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCalls, calls.Load())
			assert.Equal(t, tt.wantSleeps, sleeps)
		})
	}
}

func TestClient_RetryAfterHTTPDate(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", time.Now().Add(5*time.Second).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	var slept time.Duration
	c := newClient(t, srv, func(cfg *jevapi.Config) {
		cfg.Sleep = func(_ context.Context, d time.Duration) error {
			slept = d
			return nil
		}
	})
	_, err := c.SystemOne(t.Context(), sampleRequest())
	require.NoError(t, err)
	assert.Greater(t, slept, 2*time.Second)
	assert.LessOrEqual(t, slept, 5*time.Second)
}

func TestClient_BackoffWithoutRetryAfterIsBoundedAndJittered(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 4 {
			w.WriteHeader(529)
			return
		}
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := newClient(t, srv, func(cfg *jevapi.Config) {
		cfg.BaseBackoff = 100 * time.Millisecond
		cfg.MaxBackoff = 300 * time.Millisecond
		cfg.Sleep = func(_ context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		}
	})
	_, err := c.SystemOne(t.Context(), sampleRequest())
	require.NoError(t, err)
	require.Len(t, sleeps, 3)
	ceilings := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond}
	for i, d := range sleeps {
		assert.GreaterOrEqual(t, d, time.Duration(0))
		assert.LessOrEqual(t, d, ceilings[i])
	}
}

func TestClient_RetriesExhausted(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantKind jevapi.Kind
	}{
		{"rate limited", http.StatusTooManyRequests, jevapi.KindRateLimited},
		{"overloaded", 529, jevapi.KindUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			c := newClient(t, srv, func(cfg *jevapi.Config) { cfg.MaxRetries = 2 })
			_, err := c.SystemOne(t.Context(), sampleRequest())
			var apiErr *jevapi.APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tt.wantKind, apiErr.Kind)
			assert.Equal(t, int32(3), calls.Load(), "one attempt plus two retries")
		})
	}
}

func TestClient_RetriesConnectionFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer cannot hijack")
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	_, err := newClient(t, srv, nil).SystemOne(t.Context(), sampleRequest())
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
}

func TestClient_ConnectionFailureExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	c := newClient(t, srv, func(cfg *jevapi.Config) { cfg.MaxRetries = 1 })
	srv.Close()

	_, err := c.SystemOne(t.Context(), sampleRequest())
	var apiErr *jevapi.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, jevapi.KindUnavailable, apiErr.Kind)
	assert.NotContains(t, err.Error(), testKey)
}

func TestClient_Timeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	c := newClient(t, srv, func(cfg *jevapi.Config) { cfg.Timeout = 50 * time.Millisecond })
	start := time.Now()
	_, err := c.SystemOne(t.Context(), sampleRequest())
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second)
}

func TestClient_ContextCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-started
		cancel()
	}()
	_, err := newClient(t, srv, nil).SystemOne(ctx, sampleRequest())
	require.ErrorIs(t, err, context.Canceled)
}

func TestClient_CancellationDuringBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	c := newClient(t, srv, func(cfg *jevapi.Config) {
		cfg.Sleep = func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		}
	})
	_, err := c.SystemOne(ctx, sampleRequest())
	require.ErrorIs(t, err, context.Canceled)
}

func TestClient_MalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not json", `<html>gateway</html>`},
		{"truncated", `{"model":"jev-1.13.0","answers":{"risk:a":`},
		{"no answers", `{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`},
		{"noul missing value", `{"model":"m","answers":{"a":{"type":"noul"}},"usage":{}}`},
		{"score missing value", `{"model":"m","answers":{"a":{"type":"score"}},"usage":{}}`},
		{"unknown answer type", `{"model":"m","answers":{"a":{"type":"choice","choice":"x"}},"usage":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := newClient(t, srv, nil).SystemOne(t.Context(), sampleRequest())
			require.ErrorIs(t, err, jevapi.ErrMalformedResponse)
		})
	}
}

func TestClient_Models(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"models":[{"name":"jev-1.13.0","description":"d","release_date":"2026-09-15"},` +
			`{"name":"jev-latest","description":"alias","release_date":"2026-09-15"}]}`))
	}))
	defer srv.Close()

	models, err := newClient(t, srv, nil).Models(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "/v1/models", gotPath)
	assert.Equal(t, "Bearer "+testKey, gotAuth)
	require.Len(t, models, 2)
	assert.Equal(t, "jev-1.13.0", models[0].Name)
}

func TestClient_ModelsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newClient(t, srv, nil).Models(t.Context())
	var apiErr *jevapi.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, jevapi.KindUnauthenticated, apiErr.Kind)
}

func TestClient_ErrorMessageIsBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"` + strings.Repeat("x", 5000) + `"}`))
	}))
	defer srv.Close()

	_, err := newClient(t, srv, nil).SystemOne(t.Context(), sampleRequest())
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 600)
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name string
		cfg  jevapi.Config
		want string
	}{
		{"empty base url", jevapi.Config{APIKey: "k"}, "base url"},
		{"bad scheme", jevapi.Config{BaseURL: "ftp://api.typesafe.ai", APIKey: "k"}, "scheme"},
		{"plain http to remote host", jevapi.Config{BaseURL: "http://api.example.com", APIKey: "k"}, "https"},
		{"empty key", jevapi.Config{BaseURL: "https://api.typesafe.ai"}, "api key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jevapi.New(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}

	_, err := jevapi.New(jevapi.Config{BaseURL: "http://127.0.0.1:9", APIKey: "k"})
	require.NoError(t, err, "loopback http is allowed")
	_, err = jevapi.New(jevapi.Config{BaseURL: "https://api.typesafe.ai", APIKey: "k"})
	require.NoError(t, err)
}
