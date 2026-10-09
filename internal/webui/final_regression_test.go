package webui

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestShutdownBeforeServeReleasesPort(t *testing.T) {
	t.Parallel()
	s, err := New(Options{})
	require.NoError(t, err)
	require.NoError(t, s.Listen())
	address := s.listener.Addr().String()
	require.NoError(t, s.Shutdown(context.Background()))
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err, "shutdown must release the manually owned port")
	require.NoError(t, listener.Close())
	require.NoError(t, s.Shutdown(context.Background()))
	assert.ErrorIs(t, s.Serve(), http.ErrServerClosed)
}

func TestShutdownServeRace(t *testing.T) {
	t.Parallel()
	for range 30 {
		s, err := New(Options{})
		require.NoError(t, err)
		require.NoError(t, s.Listen())
		result := make(chan error, 1)
		go func() { result <- s.Serve() }()
		require.NoError(t, s.Shutdown(context.Background()))
		assert.ErrorIs(t, <-result, http.ErrServerClosed)
	}
}

func TestMixedCurrencyTotalsUnavailable(t *testing.T) {
	t.Parallel()
	rows := []engine.OverviewRow{
		{URN: "one", ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 12, Currency: "USD"}},
		{URN: "two", ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 23, Currency: "EUR"}},
		{URN: "three", ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 34, Currency: "USD"}},
	}
	session := NewSession(context.Background(), SessionOptions{})
	session.SetData(nil, rows, engine.DateRange{}, "mixed")
	s := newRunningServer(t, Options{Session: session})
	page := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{}`))
	require.Len(t, page.Rows, 3)
	assert.True(t, page.Totals.MixedCurrencies)
	assert.Empty(t, page.Totals.TotalProjectedDisplay)
	assert.Zero(t, page.Totals.TotalProjected, "partial aggregate must not escape as money")
	_, events, unsubscribe := session.subscribe()
	defer unsubscribe()
	session.Ready(rows)
	ready := <-events
	assert.Equal(t, "ready", ready.name)
	assert.Contains(t, string(ready.data), `"totalProjectedDisplay":""`)
	assert.Contains(t, string(ready.data), `"mixedCurrencies":true`)
	session.mu.Lock()
	snapshot := session.snapshotLocked()
	session.mu.Unlock()
	assert.Empty(t, snapshot.Totals.TotalProjectedDisplay)
}

func TestBudgetPresentationEvent(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	_, events, unsubscribe := session.subscribe()
	defer unsubscribe()
	result := engine.BuildConfigBudgetResult(
		context.Background(),
		&config.BudgetsConfig{
			Global: &config.ScopedBudget{
				Amount:   100.1234,
				Currency: "USD",
				Alerts:   []config.AlertConfig{{Threshold: 75, Type: config.AlertTypeActual}},
			},
		},
		75.6789,
	)
	result.Summary.OverallHealth = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING
	result.Budgets[0].Thresholds[0].Triggered = true
	session.Budget(result, nil)
	event := <-events
	assert.Contains(t, string(event.data), `"currentSpendDisplay":"$75.68"`)
	assert.Contains(t, string(event.data), `"limitDisplay":"$100.12"`)
	assert.Contains(t, string(event.data), `"utilizationDisplay":"75.6%"`)
	assert.Contains(
		t,
		string(event.data),
		`"footerDisplay":"Budget: WARNING $75.68 / $100.12 (76%)"`,
	)
	assert.Contains(t, string(event.data), `75% threshold triggered`)
}

func TestOverviewImpactDriftPresentation(t *testing.T) {
	t.Parallel()
	rows := []engine.OverviewRow{
		{
			URN:    "changed",
			Status: engine.StatusUpdating,
			PropertyDiffs: []engine.PropertyDiff{
				{Key: "instanceType", OldValue: "small", NewValue: "large"},
			},
			BaselineProjectedCost: &engine.ProjectedCostData{MonthlyCost: 30, Currency: "USD"},
			ActualCost: &engine.ActualCostData{
				MTDCost:   10.2345,
				Currency:  "USD",
				Breakdown: map[string]float64{"compute": 10.2345},
			},
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 42.3456, Currency: "USD"},
		},
	}
	session := NewSession(context.Background(), SessionOptions{DayOfMonth: 10})
	session.SetData(nil, rows, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	response := overviewRequest(t, s, "GET", "/api/overview/resource?urn=changed", "")
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"costDisplay":"$10.23"`)
	assert.Contains(t, string(data), `"name":"After Change","value":"$42.35"`)
	assert.Contains(t, string(data), `"name":"Current (est. monthly)"`)
}

func TestShutdownDrainsActiveRequest(t *testing.T) {
	t.Parallel()
	s, err := New(Options{})
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	s.Handle("GET /drain", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("finished"))
	}))
	require.NoError(t, s.Listen())
	served := make(chan error, 1)
	go func() { served <- s.Serve() }()
	response := make(chan *http.Response, 1)
	requestErr := make(chan error, 1)
	go func() {
		req, reqErr := http.NewRequest(http.MethodGet, s.BaseURL()+"/drain", nil)
		if reqErr != nil {
			requestErr <- reqErr
			return
		}
		req.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
		resp, requestError := http.DefaultClient.Do(req)
		response <- resp
		requestErr <- requestError
	}()
	<-entered
	stopped := make(chan error, 1)
	go func() { stopped <- s.Shutdown(context.Background()) }()
	require.ErrorIs(t, <-served, http.ErrServerClosed)
	select {
	case stopError := <-stopped:
		assert.Fail(t, "shutdown returned before request completed", "%v", stopError)
	default:
	}
	close(release)
	require.NoError(t, <-requestErr)
	resp := <-response
	require.NotNil(t, resp)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "finished", string(data))
	require.NoError(t, <-stopped)
}

type failingCloseListener struct {
	net.Listener

	closeErr error
}

func (l failingCloseListener) Close() error { _ = l.Listener.Close(); return l.closeErr }

func TestShutdownOwnershipErrorsAndUnstartedServer(t *testing.T) {
	t.Parallel()
	require.NoError(t, (&Server{}).Shutdown(context.Background()))
	s, err := New(Options{})
	require.NoError(t, err)
	require.ErrorContains(t, s.Serve(), "Listen must be called")
	require.NoError(t, s.Listen())
	failure := errors.New("listener close failure")
	s.listener = failingCloseListener{Listener: s.listener, closeErr: failure}
	require.ErrorIs(t, s.Shutdown(context.Background()), failure)
	require.ErrorIs(t, s.Listen(), http.ErrServerClosed)
}

func TestShutdownDeadlineForceClosesActiveRequest(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	s, err := New(Options{})
	require.NoError(t, err)
	s.Handle("GET /blocked", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(entered); <-release }))
	require.NoError(t, s.Listen())
	served := make(chan error, 1)
	go func() { served <- s.Serve() }()
	request, err := http.NewRequest(http.MethodGet, s.BaseURL()+"/blocked", nil)
	require.NoError(t, err)
	request.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	received := make(chan error, 1)
	go func() {
		response, requestError := http.DefaultClient.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		received <- requestError
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
	close(release)
	require.ErrorIs(t, <-served, http.ErrServerClosed)
	require.Error(t, <-received, "deadline closes the response connection")
}
