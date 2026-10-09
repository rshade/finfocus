package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func overviewFixtureRows() []engine.OverviewRow {
	return []engine.OverviewRow{
		{
			URN:           "urn::cluster",
			Type:          "aws:eks/cluster:Cluster",
			ChildURNs:     []string{"urn::child"},
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 50, Currency: "USD"},
		},
		{
			URN:           "urn::child",
			Type:          "kubernetes:core/v1:Pod",
			ParentURN:     "urn::cluster",
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 10, Currency: "USD"},
		},
		{
			URN:        "urn::database",
			Type:       "aws:rds/instance:Instance",
			ActualCost: &engine.ActualCostData{MTDCost: 4, Currency: "USD"},
			Properties: map[string]any{"instanceType": "db.t3.small", "password": "never-visible"},
		},
	}
}
func overviewRequest(t *testing.T, s *Server, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, s.BaseURL()+path, strings.NewReader(body))
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	if method == http.MethodPost {
		req.Header.Set("Origin", s.BaseURL())
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })
	return resp
}
func decodeOverviewPage(t *testing.T, r *http.Response) OverviewPage {
	t.Helper()
	require.Equal(t, 200, r.StatusCode)
	var page OverviewPage
	require.NoError(t, json.NewDecoder(r.Body).Decode(&page))
	return page
}
func TestOverviewContracts(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{DayOfMonth: 7})
	session.SetData(nil, overviewFixtureRows(), engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	page := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{"sort":"name"}`))
	require.Len(t, page.Rows, 2)
	assert.Equal(t, "urn::cluster", page.Rows[0].URN)
	assert.InDelta(t, 60.0, page.Totals.TotalProjected, 1e-9)
	expanded := decodeOverviewPage(
		t,
		overviewRequest(
			t,
			s,
			"POST",
			"/api/overview/cluster/toggle",
			`{"urn":"urn::cluster","expanded":[],"sort":"name"}`,
		),
	)
	require.Len(t, expanded.Rows, 3)
	assert.Equal(t, []string{"urn::cluster"}, expanded.Expanded)
	collapsed := decodeOverviewPage(
		t,
		overviewRequest(
			t,
			s,
			"POST",
			"/api/overview/cluster/toggle",
			`{"urn":"urn::cluster","expanded":["urn::cluster"]}`,
		),
	)
	assert.Len(t, collapsed.Rows, 2)
	filtered := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{"filter":"POD"}`))
	require.Len(t, filtered.Rows, 1)
	assert.Equal(t, "urn::child", filtered.Rows[0].URN)
	for _, body := range []string{`{"sort":"bad"}`, `{"page":-1}`, `{"unexpected":1}`, `{} {}`} {
		assert.Equal(t, 400, overviewRequest(t, s, "POST", "/api/overview/query", body).StatusCode)
	}
	assert.Equal(t, 404, overviewRequest(t, s, "GET", "/api/overview/resource?urn=missing", "").StatusCode)
	detail := overviewRequest(t, s, "GET", "/api/overview/resource?urn=urn::database", "")
	require.Equal(t, 200, detail.StatusCode)
	data, err := io.ReadAll(detail.Body)
	require.NoError(t, err)
	assert.Contains(t, string(data), "db.t3.small")
	assert.NotContains(t, string(data), "never-visible")
	assert.Equal(t, 200, overviewRequest(t, s, "GET", "/api/overview/budget", "").StatusCode)
	assert.Equal(t, 409, overviewRequest(t, s, "POST", "/api/passphrase", `{"passphrase":"unexpected"}`).StatusCode)
}
func TestOverviewPaginationAndDataOwnership(t *testing.T) {
	t.Parallel()
	rows := overviewFixtureRows()
	session := NewSession(context.Background(), SessionOptions{DayOfMonth: 7})
	session.SetData(nil, rows, engine.DateRange{}, "dev")
	rows[2].Properties["instanceType"] = "changed"
	copyRows := session.Rows()
	copyRows[2].Properties["instanceType"] = "changed-again"
	assert.Equal(t, "db.t3.small", session.Rows()[2].Properties["instanceType"])
	resources := session.Resources()
	require.Len(t, resources, 3)
	assert.Equal(t, "aws", resources[2].Provider)
	resources[2].Properties["instanceType"] = "changed-third"
	assert.Equal(t, "db.t3.small", session.Resources()[2].Properties["instanceType"])
	large := make([]engine.OverviewRow, 1001)
	for i := range large {
		large[i] = engine.OverviewRow{URN: strings.Repeat("a", i+1), Type: "aws:ec2:Instance"}
	}
	session.SetData(nil, large, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	start := time.Now()
	page := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{"page":2}`))
	assert.Len(t, page.Rows, 250)
	assert.Equal(t, 5, page.TotalPages)
	assert.Less(t, time.Since(start), time.Second)
}
func TestPreviewSingleFlightAndCancellation(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	session := NewSession(
		context.Background(),
		SessionOptions{Preview: func(ctx context.Context) ([]engine.OverviewRow, error) {
			calls.Add(1)
			close(started)
			select {
			case <-release:
				return overviewFixtureRows(), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}},
	)
	s := newRunningServer(t, Options{Session: session})
	assert.Equal(t, 202, overviewRequest(t, s, "POST", "/api/overview/preview", `{}`).StatusCode)
	<-started
	assert.Equal(t, 202, overviewRequest(t, s, "POST", "/api/overview/preview", `{}`).StatusCode)
	assert.Equal(t, int32(1), calls.Load())
	close(release)
	require.Eventually(t, func() bool { return len(session.Rows()) == 3 }, time.Second, time.Millisecond)
	session.Close()
	assert.Equal(t, 503, overviewRequest(t, s, "POST", "/api/overview/preview", `{}`).StatusCode)
}
func TestPassphraseAcceptedNeverEchoed(t *testing.T) {
	t.Parallel()
	passphrases := make(chan string, 1)
	session := NewSession(
		context.Background(),
		SessionOptions{SubmitPassphrase: func(ctx context.Context, pw string) error {
			select {
			case passphrases <- pw:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
	)
	session.RequirePassphrase("dev")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(t, s, "POST", "/api/passphrase", `{"passphrase":"test-secret"}`)
	assert.Equal(t, 202, resp.StatusCode)
	assert.Equal(t, "test-secret", <-passphrases)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "test-secret")
	session.Error(1, "Unable to decrypt stack. Try again.")
	session.RequirePassphrase("dev")
	assert.Equal(t, 202, overviewRequest(t, s, "POST", "/api/passphrase", `{"passphrase":"correct"}`).StatusCode)
	assert.Equal(t, "correct", <-passphrases)
}

func TestOverviewFailurePathsAndRegisteredRoutes(t *testing.T) {
	t.Parallel()
	session := NewSession(
		context.Background(),
		SessionOptions{SubmitPassphrase: func(context.Context, string) error { return context.Canceled }},
	)
	session.SetData(engine.New(nil, nil), overviewFixtureRows(), engine.DateRange{}, "dev")
	require.NotNil(t, session.Engine())
	s, err := New(Options{Session: session})
	require.NoError(t, err)
	s.Handle("GET /api/custom", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.WriteJSON(w, http.StatusOK, map[string]string{"value": "safe"})
	}))
	require.NoError(t, s.Listen())
	go func() { _ = s.Serve() }()
	t.Cleanup(func() { assert.NoError(t, s.Shutdown(context.Background())) })
	assert.Equal(t, 200, overviewRequest(t, s, "GET", "/api/custom", "").StatusCode)
	for _, sort := range []string{"type", "delta"} {
		decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{"sort":"`+sort+`"}`))
	}
	assert.Equal(t, 400, overviewRequest(t, s, "POST", "/api/overview/cluster/toggle", `invalid`).StatusCode)
	assert.Equal(t, 404, overviewRequest(t, s, "POST", "/api/overview/cluster/toggle", `{"urn":"missing"}`).StatusCode)
	assert.Equal(t, 400, overviewRequest(t, s, "POST", "/api/passphrase", `{"unexpected":true}`).StatusCode)
	session.RequirePassphrase("dev")
	assert.Equal(t, 503, overviewRequest(t, s, "POST", "/api/passphrase", `{"passphrase":"private"}`).StatusCode)
	first, _, unsubscribe := session.subscribe()
	defer unsubscribe()
	assert.Contains(t, string(first.data), `"passphraseRequired":true`)
	assert.Equal(t, 503, overviewRequest(t, s, "POST", "/api/overview/preview", `{}`).StatusCode)
	recorder := httptest.NewRecorder()
	s.WriteJSON(recorder, 200, make(chan struct{}))
	assert.Equal(t, 500, recorder.Code)
}
func TestSessionBudgetExpansionAndClosedSnapshot(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	_, events, unsubscribe := session.subscribe()
	defer unsubscribe()
	session.Budget(nil, errors.New("sensitive plugin error"))
	event := <-events
	assert.Equal(t, "budget", event.name)
	event = <-events
	assert.NotContains(t, string(event.data), "sensitive plugin error")
	budget := engine.BuildConfigBudgetResult(
		context.Background(),
		&config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 100, Currency: "USD"}},
		40,
	)
	session.Budget(budget, nil)
	event = <-events
	assert.Contains(t, string(event.data), `"limit":100`)
	session.Expansion(overviewFixtureRows(), []string{"expanded"})
	event = <-events
	assert.Equal(t, "expansion", event.name)
	session.Row(-1, engine.OverviewRow{})
	session.Row(99, engine.OverviewRow{})
	assert.Len(t, session.Rows(), 3)
	session.Phase(1, "loading")
	session.Phase(1, "retry")
	session.Close()
	assert.Equal(t, 0, session.SubscriberCount())
	first, closed, _ := session.subscribe()
	assert.Equal(t, "snapshot", first.name)
	_, ok := <-closed
	assert.False(t, ok)
}
func TestPreviewCancellationJoinsWorkersAndReportsError(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	returned := make(chan struct{})
	session := NewSession(
		context.Background(),
		SessionOptions{Preview: func(ctx context.Context) ([]engine.OverviewRow, error) {
			close(started)
			<-ctx.Done()
			close(returned)
			return nil, ctx.Err()
		}},
	)
	require.NoError(t, session.startPreview())
	<-started
	session.Close()
	select {
	case <-returned:
	default:
		t.Fatal("preview worker outlived session shutdown")
	}
	failed := NewSession(
		context.Background(),
		SessionOptions{Preview: func(context.Context) ([]engine.OverviewRow, error) {
			return nil, errors.New("private subprocess output")
		}},
	)
	defer failed.Close()
	_, events, unsubscribe := failed.subscribe()
	defer unsubscribe()
	require.NoError(t, failed.startPreview())
	assert.Equal(t, "preview", (<-events).name)
	errorEvent := <-events
	assert.Equal(t, "error", errorEvent.name)
	assert.NotContains(t, string(errorEvent.data), "private subprocess output")
	assert.Equal(t, "preview", (<-events).name)
}
func TestPreviewFailureSurvivesReconnectAndSuccessfulRetryClearsIt(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	session := NewSession(context.Background(), SessionOptions{
		Preview: func(context.Context) ([]engine.OverviewRow, error) {
			if attempts.Add(1) == 1 {
				return nil, errors.New("private subprocess output")
			}
			return overviewFixtureRows(), nil
		},
	})
	t.Cleanup(session.Close)
	session.Phase(previewPhase, "Detecting changes...")
	require.NoError(t, session.startPreview())
	require.Eventually(t, func() bool {
		session.mu.Lock()
		defer session.mu.Unlock()
		return session.preview.Status == "error"
	}, time.Second, time.Millisecond)
	first, _, stop := session.subscribe()
	stop()
	var snapshot OverviewSnapshot
	require.NoError(t, json.Unmarshal(first.data, &snapshot))
	require.Equal(t, []ErrorEvent{{Message: "Preview failed", Phase: previewPhase}}, snapshot.Errors)
	assert.NotContains(t, string(first.data), "private subprocess output")

	require.NoError(t, session.startPreview())
	require.Eventually(t, func() bool {
		session.mu.Lock()
		defer session.mu.Unlock()
		return session.preview.Status == phaseDone
	}, time.Second, time.Millisecond)
	next, _, stop := session.subscribe()
	stop()
	require.NoError(t, json.Unmarshal(next.data, &snapshot))
	assert.Empty(t, snapshot.Errors)
	assert.NotEmpty(t, snapshot.Rows)
}

func TestOverviewDetailFiltersRecommendationsAndFormatsSavings(t *testing.T) {
	t.Parallel()
	rows := overviewFixtureRows()
	rows[0].Recommendations = []engine.Recommendation{
		{Description: "active", EstimatedSavings: 4.5},
		{Description: "dismissed", Status: engine.RecommendationStatusDismissed},
		{Description: "snoozed", Status: engine.RecommendationStatusSnoozed},
	}
	session := NewSession(context.Background(), SessionOptions{})
	session.SetData(nil, rows, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	response := overviewRequest(t, s, "GET", "/api/overview/resource?urn=urn::cluster", "")
	var detail ResourceDetail
	require.NoError(t, json.NewDecoder(response.Body).Decode(&detail))
	require.Len(t, detail.ActiveRecommendations, 1)
	assert.Equal(t, engine.FormatOverviewCurrency(4.5), detail.ActiveRecommendations[0].SavingsDisplay)
	require.Len(t, detail.Row.Recommendations, 1)
	assert.Len(t, session.Rows()[0].Recommendations, 3)
}

func TestAcceptPassphraseBroadcastsAuthoritativeSnapshot(t *testing.T) {
	t.Parallel()
	session := NewSession(
		context.Background(),
		SessionOptions{SubmitPassphrase: func(context.Context, string) error { return nil }},
	)
	session.RequirePassphrase("dev")
	_, events, unsubscribe := session.subscribe()
	defer unsubscribe()
	s := newRunningServer(t, Options{Session: session})
	assert.Equal(
		t,
		http.StatusAccepted,
		overviewRequest(t, s, "POST", "/api/passphrase", `{"passphrase":"private"}`).StatusCode,
	)
	select {
	case event := <-events:
		assert.Equal(t, "snapshot", event.name)
		assert.Contains(t, string(event.data), `"passphraseRequired":false`)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("accepted unlock did not update other tabs")
	}
}
