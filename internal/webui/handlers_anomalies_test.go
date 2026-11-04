package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func anomalyFixtureRows() []engine.OverviewRow {
	period := engine.DateRange{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
	}
	return []engine.OverviewRow{
		{
			URN: "urn::steady", Type: "aws:s3/bucket:Bucket", Status: engine.StatusActive,
			ActualCost: &engine.ActualCostData{MTDCost: 10, Currency: "USD", Period: period},
		},
		{
			URN: "urn::drifting", Type: "aws:ec2/instance:Instance", Status: engine.StatusActive,
			Properties: map[string]any{"password": "never-visible"},
			ActualCost: &engine.ActualCostData{MTDCost: 360, Currency: "USD", Period: period},
			CostDrift: &engine.CostDriftData{
				ExtrapolatedMonthly: 1080, Projected: 720, Delta: 360, PercentDrift: 50, IsWarning: true,
			},
		},
		{
			URN: "urn::failed", Type: "aws:rds/instance:Instance", Status: engine.StatusActive,
			CostDrift: &engine.CostDriftData{
				ExtrapolatedMonthly: 900, Projected: 100, Delta: 800, PercentDrift: 800, IsWarning: true,
			},
			Error: &engine.OverviewRowError{URN: "urn::failed", ErrorType: engine.ErrorTypeUnknown, Message: "boom"},
		},
	}
}

func TestAnomaliesMatchSharedFunctionOverSessionRows(t *testing.T) {
	t.Parallel()
	rows := anomalyFixtureRows()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(rows)
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	var got AnomalyPage
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))

	want := engine.SelectAnomalies(rows, nil)
	assert.Equal(t, want.Considered, got.Considered)
	assert.Equal(t, "1 of 3 resources", got.Summary)
	require.Len(t, got.Anomalies, len(want.Anomalies))
	require.Len(t, got.Anomalies, 1)
	assert.Equal(t, "urn::drifting", got.Anomalies[0].Resource)
	assert.InDelta(t, want.Anomalies[0].Baseline, got.Anomalies[0].Baseline, 1e-9)
	assert.InDelta(t, want.Anomalies[0].PercentDrift, got.Anomalies[0].PercentDrift, 1e-9)
	assert.Equal(t, viewmodel.BuildAnomalyDisplay(want.Anomalies[0]), got.Anomalies[0].Display)
	assert.Equal(t, "$1,080.00", got.Anomalies[0].Display.Observed)
	assert.Equal(t, "Oct 1 – Oct 11, 2026", got.Anomalies[0].Display.Period)
}

func TestAnomaliesCarryNoRowProperties(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(anomalyFixtureRows())
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "never-visible")
	assert.NotContains(t, string(body), "password")
}

func TestAnomaliesEmptySessionReturnsEmptyList(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(nil)
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	assert.JSONEq(t, `{"considered":0,"summary":"0 of 0 resources","anomalies":[]}`, string(body))
}

func TestAnomaliesFollowSessionRowUpdates(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(anomalyFixtureRows()[:1])
	s := newRunningServer(t, Options{Session: session})

	var before AnomalyPage
	require.NoError(t, json.NewDecoder(overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "").Body).
		Decode(&before))
	assert.Empty(t, before.Anomalies)

	session.Ready(anomalyFixtureRows())

	var after AnomalyPage
	require.NoError(t, json.NewDecoder(overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "").Body).
		Decode(&after))
	assert.Len(t, after.Anomalies, 1, "the handler reads the session rows on every request")
}

// dailyAnomalyFixture builds the 80..145 baseline over fourteen days with a
// 250 spike on day fifteen for one resource written by collect-daily, plus a
// resource the series has never seen.
func dailyAnomalyFixture() []history.DailyPoint {
	points := make([]history.DailyPoint, 0, 15)
	for offset := 0; offset < 14; offset++ {
		points = append(points, history.DailyPoint{
			ResourceID: "urn::spiking",
			Day:        time.Date(2026, 10, 1+offset, 0, 0, 0, 0, time.UTC),
			Currency:   "USD",
			Amount:     80 + 5*float64(offset),
		})
	}
	points = append(points, history.DailyPoint{
		ResourceID: "urn::spiking",
		Day:        time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		Currency:   "USD",
		Amount:     250,
	})
	return points
}

func TestAnomaliesHandlerScoresDailySeries(t *testing.T) {
	t.Parallel()
	rows := anomalyFixtureRows()
	rows = append(rows, engine.OverviewRow{
		URN: "urn::spiking", Type: "aws:ec2/instance:Instance", Status: engine.StatusActive,
	})
	series := dailyAnomalyFixture()
	session := NewSession(context.Background(), SessionOptions{
		DailyPoints: func(context.Context) ([]history.DailyPoint, error) {
			return series, nil
		},
	})
	session.Ready(rows)
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	var got AnomalyPage
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))

	want := engine.SelectAnomalies(rows, series)
	assert.Equal(t, want.Considered, got.Considered)
	require.Len(t, got.Anomalies, len(want.Anomalies))
	require.Len(t, got.Anomalies, 2, "one row per resource, no second row")
	for i := range want.Anomalies {
		assert.Equal(t, want.Anomalies[i].Resource, got.Anomalies[i].Resource)
		assert.Equal(t, want.Anomalies[i].Method, got.Anomalies[i].Method)
		assert.Equal(t, want.Anomalies[i].Currency, got.Anomalies[i].Currency)
		display := viewmodel.BuildAnomalyDisplay(want.Anomalies[i])
		assert.Equal(t, display, got.Anomalies[i].Display)
	}
	assert.Equal(t, "robust-median", got.Anomalies[1].Method, "a qualifying day spike reports its own method")
	assert.Equal(t, "urn::spiking", got.Anomalies[1].Resource)
	assert.Equal(t, "drift", got.Anomalies[0].Method, "the drift row's larger deviation still sorts first")
}

func TestAnomaliesHandlerWithoutProviderKeepsDriftRows(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(anomalyFixtureRows())
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	var got AnomalyPage
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))
	require.Len(t, got.Anomalies, 1)
	assert.Equal(t, "drift", got.Anomalies[0].Method)
	assert.NotContains(t, got.Anomalies[0].Resource, "spiking")
}

func TestAnomaliesHandlerReportsSeriesFailure(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{
		DailyPoints: func(context.Context) ([]history.DailyPoint, error) {
			return nil, errors.New("daily actuals boom") // a real read failure, e.g. a corrupt store
		},
	})
	session.Ready(anomalyFixtureRows())
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	assert.Equal(t, http.StatusInternalServerError, response.StatusCode)
}

func TestAnomaliesRequireSessionAuthentication(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(anomalyFixtureRows())
	s := newRunningServer(t, Options{Session: session})

	req, err := http.NewRequest(http.MethodGet, s.BaseURL()+"/api/overview/anomalies", http.NoBody)
	require.NoError(t, err)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAnomaliesNotReadyBeforeEnrichmentFinishes(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.SetData(nil, anomalyFixtureRows(), engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	var failure struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&failure))
	assert.Equal(t, "not_ready", failure.Code, "rows are not priced yet, so an empty list would be wrong")

	session.Ready(anomalyFixtureRows())

	response = overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	assert.Equal(t, http.StatusOK, response.StatusCode)
}
