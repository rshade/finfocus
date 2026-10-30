package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

const secretType = "aws:secretsmanager/secret:Secret"

func historyViewFixture() history.ViewDocument {
	return history.ViewDocument{
		Stack: "dev",
		Snapshots: []history.CostSnapshot{{
			Timestamp:     time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			Version:       7,
			TotalMonthly:  1234.5,
			Currency:      "USD",
			ResourceCount: 2,
			ByProvider:    map[string]float64{"aws": 1234.5},
			ByType:        map[string]float64{secretType: 0.4, "aws:ec2/instance:Instance": 1234.1},
			Resources: []history.CostResource{{
				URN: "urn:pulumi:dev::app::" + secretType + "::db", Type: secretType, Provider: "aws", MonthlyCost: 0.4,
			}},
		}},
		Annotations: []history.CostAnnotation{{
			Version: 7, Message: "rotate secret", Kind: "update",
			ResourceChanges: map[string]int{"update": 1, "same": 1},
		}},
		CurrencyWarning: "Warning: Mixed currencies detected",
	}
}

func historyDiffFixture() history.DiffDocument {
	return history.DiffDocument{
		Stack:    "dev",
		From:     history.DiffPointDocument{Version: 1, Timestamp: "2026-01-01T00:00:00Z", TotalMonthly: 10},
		To:       history.DiffPointDocument{Version: 2, Timestamp: "2026-02-01T00:00:00Z", TotalMonthly: 40},
		Delta:    30,
		Currency: "USD",
		Added:    []history.DiffItemDocument{{URN: "urn:b", Type: secretType, MonthlyCost: 30, Delta: 30}},
		Removed:  []history.DiffItemDocument{},
		Changed: []history.DiffItemDocument{
			{URN: "urn:a", Type: "aws:s3:Bucket", MonthlyCost: 5, Delta: -5},
		},
		UnchangedCount: 1,
		UnchangedTotal: 5,
	}
}

func newCostHistoryServer(t *testing.T, opts SessionOptions) *Server {
	t.Helper()
	session := NewSession(context.Background(), opts)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	return newRunningServer(t, Options{Session: session})
}

func decodeAPIError(t *testing.T, resp *http.Response) apiError {
	t.Helper()
	var body apiError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

func TestHistoryView_ReturnsDocumentBudgetAndDisplay(t *testing.T) {
	t.Parallel()
	var got HistoryViewQuery
	budget := 1500.0
	s := newCostHistoryServer(t, SessionOptions{
		HistoryView: func(_ context.Context, q HistoryViewQuery) (HistoryViewResult, error) {
			got = q
			return HistoryViewResult{Document: historyViewFixture(), Budget: &budget, BudgetCurrency: "USD"}, nil
		},
	})

	resp := overviewRequest(t, s, http.MethodGet,
		"/api/history/view?from=2026-01-01&to=2026-04-01&currency=usd&strict=true", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body HistoryViewResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, HistoryViewQuery{From: "2026-01-01", To: "2026-04-01", Currency: "usd", Strict: true}, got)
	assert.Equal(t, HistoryStatusOK, body.Status)
	assert.Equal(t, historyViewFixture(), body.ViewDocument, "breakdown keys survive redaction")
	require.NotNil(t, body.Budget)
	assert.InDelta(t, 1500, *body.Budget, 0)
	assert.Equal(t, HistoryViewDisplay{
		Budget:    "$1,500.00 USD",
		Snapshots: []HistorySnapshotDisplay{{Version: 7, Date: "2026-03-15", Total: "$1,234.50 USD"}},
	}, body.Display)
}

func TestHistoryView_OmitsUnsetBudget(t *testing.T) {
	t.Parallel()
	s := newCostHistoryServer(t, SessionOptions{
		HistoryView: func(context.Context, HistoryViewQuery) (HistoryViewResult, error) {
			return HistoryViewResult{Document: history.ViewDocument{Stack: "dev"}}, nil
		},
	})
	resp := overviewRequest(t, s, http.MethodGet, "/api/history/view", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotContains(t, body, "budget")
	assert.Equal(t, map[string]any{"snapshots": []any{}}, body["display"])
}

func TestHistoryView_RejectsNonBooleanStrict(t *testing.T) {
	t.Parallel()
	called := false
	s := newCostHistoryServer(t, SessionOptions{
		HistoryView: func(context.Context, HistoryViewQuery) (HistoryViewResult, error) {
			called = true
			return HistoryViewResult{}, nil
		},
	})
	resp := overviewRequest(t, s, http.MethodGet, "/api/history/view?strict=maybe", "")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeAPIError(t, resp)
	assert.Equal(t, "invalid_history_query", body.Code)
	assert.Equal(t, `invalid strict value "maybe" (use true or false)`, body.Error)
	assert.False(t, called)
}

func TestHistoryDiff_ReturnsDocumentAndDisplay(t *testing.T) {
	t.Parallel()
	var got HistoryDiffQuery
	s := newCostHistoryServer(t, SessionOptions{
		HistoryDiff: func(_ context.Context, q HistoryDiffQuery) (history.DiffDocument, error) {
			got = q
			return historyDiffFixture(), nil
		},
	})

	resp := overviewRequest(t, s, http.MethodGet, "/api/history/diff?from=v1&to=2026-02-01&threshold=2.5", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body HistoryDiffResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, HistoryDiffQuery{From: "v1", To: "2026-02-01", Threshold: 2.5}, got)
	assert.Equal(t, HistoryStatusOK, body.Status)
	assert.Equal(t, historyDiffFixture(), body.DiffDocument)
	assert.Equal(t, HistoryDiffDisplay{
		Delta:          "+$30.00 USD",
		From:           "$10.00 USD",
		To:             "$40.00 USD",
		UnchangedTotal: "$5.00 USD",
		Added:          []HistoryItemDisplay{{MonthlyCost: "$30.00 USD", Delta: "+$30.00 USD"}},
		Removed:        []HistoryItemDisplay{},
		Changed:        []HistoryItemDisplay{{MonthlyCost: "$5.00 USD", Delta: "-$5.00 USD"}},
	}, body.Display)
}

func TestHistoryDiff_RejectsNonNumericThreshold(t *testing.T) {
	t.Parallel()
	s := newCostHistoryServer(t, SessionOptions{
		HistoryDiff: func(context.Context, HistoryDiffQuery) (history.DiffDocument, error) {
			return history.DiffDocument{}, nil
		},
	})
	resp := overviewRequest(t, s, http.MethodGet, "/api/history/diff?from=v1&threshold=lots", "")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeAPIError(t, resp)
	assert.Equal(t, "invalid_history_query", body.Code)
	assert.Equal(t, `invalid threshold "lots"`, body.Error)
}

func TestHistoryResource_ReturnsPointsAndDisplay(t *testing.T) {
	t.Parallel()
	urn := "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"
	var got string
	s := newCostHistoryServer(t, SessionOptions{
		HistoryResource: func(_ context.Context, id string) ([]HistoryPoint, error) {
			got = id
			return []HistoryPoint{
				{Timestamp: time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC), Amount: 12, Currency: "USD"},
				{Timestamp: time.Date(2026, 2, 2, 3, 0, 0, 0, time.UTC), Amount: 0, Currency: "USD"},
			}, nil
		},
	})

	resp := overviewRequest(t, s, http.MethodGet, "/api/history/resource?id="+urn, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body HistoryResourceResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, urn, got)
	assert.Equal(t, HistoryStatusOK, body.Status)
	assert.Equal(t, urn, body.ID)
	require.Len(t, body.Points, 2)
	assert.InDelta(t, 12, body.Points[0].Amount, 0)
	assert.Equal(t, []HistoryPointDisplay{
		{Date: "2026-01-02", Amount: "$12.00 USD"},
		{Date: "2026-02-02", Amount: "$0.00 USD"},
	}, body.Display.Points)
}

func TestHistoryResource_EmptySeriesIsAnArray(t *testing.T) {
	t.Parallel()
	s := newCostHistoryServer(t, SessionOptions{
		HistoryResource: func(context.Context, string) ([]HistoryPoint, error) { return nil, nil },
	})
	resp := overviewRequest(t, s, http.MethodGet, "/api/history/resource?id=urn:missing", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"ok","id":"urn:missing","points":[],"display":{"points":[]}}`, string(raw))
}

func TestHistoryResource_RequiresID(t *testing.T) {
	t.Parallel()
	s := newCostHistoryServer(t, SessionOptions{
		HistoryResource: func(context.Context, string) ([]HistoryPoint, error) { return nil, nil },
	})
	for _, path := range []string{"/api/history/resource", "/api/history/resource?id=%20"} {
		resp := overviewRequest(t, s, http.MethodGet, path, "")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, path)
		body := decodeAPIError(t, resp)
		assert.Equal(t, "invalid_query", body.Code)
		assert.Equal(t, "resource id required", body.Error)
	}
}

func costHistoryServerFailing(t *testing.T, err error) *Server {
	t.Helper()
	return newCostHistoryServer(t, SessionOptions{
		HistoryView: func(context.Context, HistoryViewQuery) (HistoryViewResult, error) {
			return HistoryViewResult{}, err
		},
		HistoryDiff: func(context.Context, HistoryDiffQuery) (history.DiffDocument, error) {
			return history.DiffDocument{}, err
		},
		HistoryResource: func(context.Context, string) ([]HistoryPoint, error) { return nil, err },
	})
}

func historyPaths() []string {
	return []string{
		"/api/history/view",
		"/api/history/diff?from=v1",
		"/api/history/resource?id=urn:x",
	}
}

func TestHistory_NoHistoryIsAStatus(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{
		fmt.Errorf("%w for stack 'dev'", history.ErrNoCostHistory),
		history.ErrNoSnapshots,
	} {
		s := costHistoryServerFailing(t, cause)
		for _, path := range historyPaths() {
			resp := overviewRequest(t, s, http.MethodGet, path, "")
			require.Equal(t, http.StatusOK, resp.StatusCode, path)
			var body map[string]any
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, HistoryStatusNoHistory, body["status"], path)
			assert.NotContains(t, body, "snapshots", path)
		}
	}
}

func TestHistory_QueryErrorIs400WithCLIText(t *testing.T) {
	t.Parallel()
	cause := &history.QueryError{Err: errors.New("snapshot version 9 not found")}
	s := costHistoryServerFailing(t, cause)
	for _, path := range historyPaths() {
		resp := overviewRequest(t, s, http.MethodGet, path, "")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, path)
		body := decodeAPIError(t, resp)
		assert.Equal(t, "invalid_history_query", body.Code)
		assert.Equal(t, "snapshot version 9 not found", body.Error)
	}
}

func TestHistory_FailureIsGenericWithoutPath(t *testing.T) {
	t.Parallel()
	cause := errors.New("opening cost history: open /home/user/.finfocus/history/app@dev.history.db: invalid database")
	s := costHistoryServerFailing(t, cause)
	for _, path := range historyPaths() {
		resp := overviewRequest(t, s, http.MethodGet, path, "")
		require.Equal(t, http.StatusBadGateway, resp.StatusCode, path)
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"error":"cost history unavailable","code":"history_unavailable"}`, string(raw))
	}
}

func TestHistory_CanceledReadIsGeneric(t *testing.T) {
	t.Parallel()
	s := costHistoryServerFailing(t, fmt.Errorf("reading: %w", context.Canceled))
	for _, path := range historyPaths() {
		resp := overviewRequest(t, s, http.MethodGet, path, "")
		require.Equal(t, http.StatusBadGateway, resp.StatusCode, path)
		assert.Equal(t, "history_unavailable", decodeAPIError(t, resp).Code)
	}
}

func TestHistory_NotReady(t *testing.T) {
	t.Parallel()
	called := false
	opts := SessionOptions{
		HistoryView: func(context.Context, HistoryViewQuery) (HistoryViewResult, error) {
			called = true
			return HistoryViewResult{}, nil
		},
		HistoryDiff: func(context.Context, HistoryDiffQuery) (history.DiffDocument, error) {
			called = true
			return history.DiffDocument{}, nil
		},
		HistoryResource: func(context.Context, string) ([]HistoryPoint, error) {
			called = true
			return nil, nil
		},
	}
	loading := newRunningServer(t, Options{Session: NewSession(context.Background(), opts)})
	unwired := newRunningServer(t, Options{Session: NewSession(context.Background(), SessionOptions{})})
	for _, s := range []*Server{loading, unwired} {
		for _, path := range historyPaths() {
			resp := overviewRequest(t, s, http.MethodGet, path, "")
			require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, path)
			body := decodeAPIError(t, resp)
			assert.Equal(t, "not_ready", body.Code)
			assert.Equal(t, "cost history is loading", body.Error)
		}
	}
	assert.False(t, called)
}

func TestHistory_RejectsWrongMethod(t *testing.T) {
	t.Parallel()
	s := costHistoryServerFailing(t, errors.New("unused"))
	resp := overviewRequest(t, s, http.MethodPost, "/api/history/view", `{}`)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}
