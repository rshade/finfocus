package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestRecommendationsQueryValidation(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{Session: NewSession(context.Background(), SessionOptions{})})
	for _, body := range []string{`{"sort":"bad"}`, `{"page":-1}`, `{"unexpected":true}`} {
		assert.Equal(
			t,
			http.StatusBadRequest,
			overviewRequest(t, s, "POST", "/api/recommendations/query", body).StatusCode,
		)
	}
	assert.Equal(
		t,
		http.StatusServiceUnavailable,
		overviewRequest(t, s, "POST", "/api/recommendations/query", `{}`).StatusCode,
	)
}

func TestRecommendationsQueryParity(t *testing.T) {
	t.Parallel()
	fixture := []engine.Recommendation{
		{ID: "a", ResourceID: "database", Type: "Resize", Description: "Resize", EstimatedSavings: 3, Currency: "USD"},
		{ID: "b", ResourceID: "server", Type: "Delete", EstimatedSavings: 8, Currency: "USD"},
	}
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(_ context.Context, include bool) (*engine.RecommendationsResult, error) {
			recs := append([]engine.Recommendation(nil), fixture...)
			if include {
				recs = append(
					recs,
					engine.Recommendation{
						ID:               "c",
						ResourceID:       "old",
						Type:             "Delete",
						Status:           engine.RecommendationStatusDismissed,
						EstimatedSavings: 2,
					},
				)
			}
			return &engine.RecommendationsResult{Recommendations: recs}, nil
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	for _, include := range []bool{false, true} {
		resp := overviewRequest(
			t,
			s,
			"POST",
			"/api/recommendations/query",
			fmt.Sprintf(`{"includeDismissed":%t,"sort":"savings"}`, include),
		)
		require.Equal(t, 200, resp.StatusCode)
		var page viewmodel.RecommendationPage
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		expected := append([]engine.Recommendation(nil), fixture...)
		if include {
			expected = append(
				expected,
				engine.Recommendation{
					ID:               "c",
					ResourceID:       "old",
					Type:             "Delete",
					Status:           engine.RecommendationStatusDismissed,
					EstimatedSavings: 2,
				},
			)
		}
		viewmodel.SortRecommendations(expected, viewmodel.SortBySavings)
		assert.Equal(t, expected, page.Items)
		assert.Equal(t, engine.BuildRecommendationSummary(expected), page.Summary)
	}
	resp := overviewRequest(t, s, "POST", "/api/recommendations/query", `{"filter":"DATABASE","sort":"resource"}`)
	var page viewmodel.RecommendationPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	require.Len(t, page.Items, 1)
	assert.Equal(t, "a", page.Items[0].ID)
}

func TestRecommendationDetailSignals(t *testing.T) {
	t.Parallel()
	risk, falsePositive, worth, priority, evidence := 0.1, 0.2, 0.8, 3.0, 0.4
	scores := &engine.RecommendationScores{
		Risk:                 &risk,
		FalsePositive:        &falsePositive,
		WorthActing:          &worth,
		Priority:             &priority,
		InsufficientEvidence: &evidence,
		DuplicateGroupID:     "group-1",
		NeedsReview:          true,
	}
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(_ context.Context, include bool) (*engine.RecommendationsResult, error) {
			assert.True(t, include)
			return &engine.RecommendationsResult{
				Recommendations: []engine.Recommendation{
					{ID: "signal", ResourceID: "r", Type: "Resize", Scores: scores},
				},
			}, nil
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(t, s, "GET", "/api/recommendations/item?id=signal", "")
	require.Equal(t, 200, resp.StatusCode)
	var detail struct {
		Item engine.Recommendation `json:"item"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	assert.Equal(t, scores, detail.Item.Scores)
	assert.Equal(t, 404, overviewRequest(t, s, "GET", "/api/recommendations/item?id=missing", "").StatusCode)
}

func TestReadOnlyAPIFailureContracts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		opts               SessionOptions
		ready              bool
		closed             bool
		method, path, body string
		want               int
	}{
		{name: "actual invalid json", method: "POST", path: "/api/cost/actual/query", body: "{", want: 400},
		{name: "actual detail id", method: "GET", path: "/api/cost/actual/resource", want: 400},
		{name: "rec detail id", method: "GET", path: "/api/recommendations/item", want: 400},
		{name: "actual detail not ready", method: "GET", path: "/api/cost/actual/resource?id=x", want: 503},
		{name: "rec detail not ready", method: "GET", path: "/api/recommendations/item?id=x", want: 503},
	}
	failActual := func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error) {
		return nil, errors.New("password=private-source-error")
	}
	failRecs := func(context.Context, bool) (*engine.RecommendationsResult, error) {
		return nil, errors.New("password=private-source-error")
	}
	for _, route := range []struct{ method, path, body string }{
		{"POST", "/api/cost/actual/query", "{}"}, {"GET", "/api/cost/actual/resource?id=x", ""}, {"POST", "/api/recommendations/query", "{}"}, {"GET", "/api/recommendations/item?id=x", ""},
	} {
		opts := SessionOptions{ActualCosts: failActual, Recommendations: failRecs}
		cases = append(cases, struct {
			name               string
			opts               SessionOptions
			ready              bool
			closed             bool
			method, path, body string
			want               int
		}{name: route.path + " error", opts: opts, ready: true, method: route.method, path: route.path, body: route.body, want: 502}, struct {
			name               string
			opts               SessionOptions
			ready              bool
			closed             bool
			method, path, body string
			want               int
		}{name: route.path + " loading", opts: opts, method: route.method, path: route.path, body: route.body, want: 503}, struct {
			name               string
			opts               SessionOptions
			ready              bool
			closed             bool
			method, path, body string
			want               int
		}{name: route.path + " closed", opts: opts, ready: true, closed: true, method: route.method, path: route.path, body: route.body, want: 503})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			session := NewSession(context.Background(), tc.opts)
			if tc.ready {
				session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
			}
			if tc.closed {
				session.Close()
			}
			s := newRunningServer(t, Options{Session: session})
			resp := overviewRequest(t, s, tc.method, tc.path, tc.body)
			assert.Equal(t, tc.want, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.NotContains(t, string(body), "private-source-error")
		})
	}
}

func TestReadOnlyQueriesJoinSessionShutdown(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	stopped := make(chan struct{})
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(ctx context.Context, _ bool) (*engine.RecommendationsResult, error) {
			close(started)
			<-ctx.Done()
			close(stopped)
			return nil, ctx.Err()
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodPost, s.BaseURL()+"/api/recommendations/query", strings.NewReader("{}"))
		rec := httptest.NewRecorder()
		s.handleRecommendationsQuery(rec, req)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		require.FailNow(t, "query did not start")
	}
	session.Close()
	select {
	case <-stopped:
	default:
		require.FailNow(t, "session closed before query was joined")
	}
	<-done
}

func TestRecommendationDetailDisambiguatesIDlessActions(t *testing.T) {
	t.Parallel()
	fixture := []engine.Recommendation{
		{ResourceID: "server", Type: "Resize", Description: "Resize to small", EstimatedSavings: 3},
		{ResourceID: "server", Type: "Delete", Description: "Delete unused", EstimatedSavings: 8},
	}
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(context.Context, bool) (*engine.RecommendationsResult, error) {
			return &engine.RecommendationsResult{Recommendations: fixture}, nil
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(
		t,
		s,
		"GET",
		"/api/recommendations/item?id=server&type=Delete&description=Delete%20unused",
		"",
	)
	require.Equal(t, 200, resp.StatusCode)
	var detail struct {
		Item engine.Recommendation `json:"item"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	assert.Equal(t, "Delete", detail.Item.Type)
	assert.InDelta(t, 8.0, detail.Item.EstimatedSavings, 1e-9)
}

func TestRecommendationDetailKeysAndActionSort(t *testing.T) {
	t.Parallel()
	fixture := []engine.Recommendation{
		{ResourceID: "r", Type: "Resize", Description: "resize"},
		{ResourceID: "r", Type: "Delete", Description: "delete"},
	}
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(context.Context, bool) (*engine.RecommendationsResult, error) {
			return &engine.RecommendationsResult{Recommendations: fixture}, nil
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(t, s, "POST", "/api/recommendations/query", `{"sort":"action"}`)
	require.Equal(t, 200, resp.StatusCode)
	var page viewmodel.RecommendationPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	require.Len(t, page.ItemKeys, 2)
	assert.NotEqual(t, page.ItemKeys[0], page.ItemKeys[1])
	resp = overviewRequest(t, s, "GET", "/api/recommendations/item?id=r&key="+page.ItemKeys[0], "")
	require.Equal(t, 200, resp.StatusCode)
	var detail struct {
		Item engine.Recommendation `json:"item"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	assert.Equal(t, "Delete", detail.Item.Type)
	assert.Equal(
		t,
		404,
		overviewRequest(t, s, "GET", "/api/recommendations/item?id=r&type=Delete&description=wrong", "").StatusCode,
	)
}

func TestRecommendationPartialErrorsRemainVisibleAndSafe(t *testing.T) {
	t.Parallel()
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(context.Context, bool) (*engine.RecommendationsResult, error) {
			return &engine.RecommendationsResult{
				Recommendations: []engine.Recommendation{{ID: "healthy", Type: "Resize"}},
				Errors:          []engine.RecommendationError{{PluginName: "failed", Error: "secret-password-value"}},
			}, nil
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(t, s, "POST", "/api/recommendations/query", "{}")
	require.Equal(t, 200, resp.StatusCode)
	var page struct {
		Items  []engine.Recommendation      `json:"items"`
		Errors []engine.RecommendationError `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Len(t, page.Items, 1)
	require.Len(t, page.Errors, 1)
	assert.Equal(t, "failed", page.Errors[0].PluginName)
	assert.Equal(t, "Recommendation data unavailable", page.Errors[0].Error)
}

type blockedJSONWriter struct {
	*httptest.ResponseRecorder

	entered chan struct{}
	release chan struct{}
}

func (w *blockedJSONWriter) Write(data []byte) (int, error) {
	close(w.entered)
	<-w.release
	return w.ResponseRecorder.Write(data)
}

func TestReadOnlyShutdownDoesNotWaitForSlowResponseWriter(t *testing.T) {
	t.Parallel()
	session := NewSession(
		context.Background(),
		SessionOptions{Recommendations: func(context.Context, bool) (*engine.RecommendationsResult, error) {
			return &engine.RecommendationsResult{
				Recommendations: []engine.Recommendation{{ID: "ready", Type: "Resize"}},
			}, nil
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	writer := &blockedJSONWriter{
		ResponseRecorder: httptest.NewRecorder(),
		entered:          make(chan struct{}),
		release:          make(chan struct{}),
	}
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		s.handleRecommendationsQuery(
			writer,
			httptest.NewRequest(http.MethodPost, s.BaseURL()+"/api/recommendations/query", strings.NewReader("{}")),
		)
	}()
	<-writer.entered
	closed := make(chan struct{})
	go func() { session.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		assert.Fail(t, "session shutdown waited for a slow socket write")
	}
	close(writer.release)
	<-returned
	<-closed
}
