package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/forecast"
)

func forecastFixture() forecast.Document {
	budget := 500.0
	oct := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return forecast.Document{
		Currency: "USD",
		Months:   1,
		Budget:   &budget,
		Series: []forecast.Series{
			{Name: forecast.SeriesHistory, Points: []forecast.Point{{Time: oct.AddDate(0, -1, 0), Value: 120}}},
			{
				Name:   forecast.SeriesForecast,
				Points: []forecast.Point{{Time: oct, Value: 130}, {Time: nov, Value: 1234.5}},
			},
		},
		Warnings: []string{"ec2: no cost data available"},
	}
}

func newForecastServer(t *testing.T, fn func(context.Context, ForecastQuery) (forecast.Document, error)) *Server {
	t.Helper()
	session := NewSession(context.Background(), SessionOptions{Forecast: fn})
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	return newRunningServer(t, Options{Session: session})
}

func TestForecastQuery_ReturnsDocumentAndDisplay(t *testing.T) {
	t.Parallel()
	var got ForecastQuery
	s := newForecastServer(t, func(_ context.Context, q ForecastQuery) (forecast.Document, error) {
		got = q
		return forecastFixture(), nil
	})

	resp := overviewRequest(t, s, http.MethodPost, "/api/forecast/query",
		`{"months":6,"growthType":"linear","growthRate":"0.10","noHistory":true}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body ForecastResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.NotNil(t, got.Months)
	assert.Equal(t, 6, *got.Months)
	assert.Equal(t, "linear", got.GrowthType)
	assert.Equal(t, "0.10", got.GrowthRate)
	assert.True(t, got.NoHistory)

	assert.Equal(t, forecastFixture(), body.Document)
	assert.Equal(t, "$500.00 USD", body.Display.Budget)
	assert.Equal(t, "Plugin growth", body.Display.Model)
	require.Len(t, body.Display.Series, 2)
	assert.Equal(t, forecast.SeriesHistory, body.Display.Series[0].Name)
	assert.Equal(t, []ForecastPointDisplay{
		{Month: "Oct 2026", Value: "$130.00 USD"},
		{Month: "Nov 2026", Value: "$1,234.50 USD"},
	}, body.Display.Series[1].Points)
}

func TestForecastQuery_StatisticalFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	var got ForecastQuery
	s := newForecastServer(t, func(_ context.Context, q ForecastQuery) (forecast.Document, error) {
		got = q
		doc := forecastFixture()
		doc.Model, doc.ModelOrder = forecast.ModelARIMA, "ARIMA(1,1,0)"
		return doc, nil
	})
	resp := overviewRequest(t, s, http.MethodPost, "/api/forecast/query", `{"model":"arima",`+
		`"spendHistory":"2026-04-01,1\n","confidence":0.8,"growthCap":1.25,`+
		`"steps":["2027-06-01=500"],"exclude":["2025-12-01:2025-12-31"]}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "arima", got.Model)
	assert.Equal(t, "2026-04-01,1\n", got.SpendHistory)
	require.NotNil(t, got.Confidence)
	assert.InDelta(t, 0.8, *got.Confidence, 1e-12)
	require.NotNil(t, got.GrowthCap)
	assert.InDelta(t, 1.25, *got.GrowthCap, 1e-12)
	assert.Equal(t, []string{"2027-06-01=500"}, got.Steps)
	assert.Equal(t, []string{"2025-12-01:2025-12-31"}, got.Exclude)
	var body ForecastResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ARIMA(1,1,0)", body.ModelOrder)
	assert.Equal(t, "ARIMA(1,1,0)", body.Display.Model, "no confidence in the fixture")
}

func TestForecastModelDisplay(t *testing.T) {
	t.Parallel()
	confidence := 0.8
	assert.Equal(t, "SARIMA(1,1,1)(0,1,1)[7], bounds at 80% confidence", forecastModelDisplay(forecast.Document{
		Model: forecast.ModelARIMA, ModelOrder: "SARIMA(1,1,1)(0,1,1)[7]", Confidence: &confidence,
	}))
	assert.Equal(t, "Plugin growth", forecastModelDisplay(forecast.Document{}))
}

func TestForecastQuery_OmittedFieldsPassThroughEmpty(t *testing.T) {
	t.Parallel()
	var got ForecastQuery
	s := newForecastServer(t, func(_ context.Context, q ForecastQuery) (forecast.Document, error) {
		got = q
		return forecast.Document{Currency: "USD", Months: forecast.DefaultMonths}, nil
	})

	resp := overviewRequest(t, s, http.MethodPost, "/api/forecast/query", `{}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, got.Months)
	assert.Empty(t, got.GrowthType)
	assert.False(t, got.NoHistory)
	var body ForecastResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Empty(t, body.Display.Budget)
}

func TestForecastQuery_InputErrorIs400WithCLIText(t *testing.T) {
	t.Parallel()
	s := newForecastServer(t, func(context.Context, ForecastQuery) (forecast.Document, error) {
		return forecast.Document{}, forecast.ValidateMonths(0)
	})

	resp := overviewRequest(t, s, http.MethodPost, "/api/forecast/query", `{"months":0}`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var body apiError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "invalid_forecast", body.Code)
	assert.Equal(t, forecast.ErrInvalidMonths.Error(), body.Error)
}

func TestForecastQuery_UnprojectableIs422WithCLIText(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{
		forecast.ErrNoCostData,
		fmt.Errorf("%w: EUR, USD", forecast.ErrMixedCurrencies),
	} {
		s := newForecastServer(t, func(context.Context, ForecastQuery) (forecast.Document, error) {
			return forecast.Document{}, cause
		})
		resp := overviewRequest(t, s, http.MethodPost, "/api/forecast/query", `{}`)
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
		var body apiError
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "no_forecast", body.Code)
		assert.Equal(t, cause.Error(), body.Error)
	}
}

func TestForecastQuery_FailureIsGeneric(t *testing.T) {
	t.Parallel()
	s := newForecastServer(t, func(context.Context, ForecastQuery) (forecast.Document, error) {
		return forecast.Document{}, errors.New("rpc error: plugin secret detail")
	})

	resp := overviewRequest(t, s, http.MethodPost, "/api/forecast/query", `{}`)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	var body apiError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "forecast_unavailable", body.Code)
	assert.Equal(t, "forecast data unavailable", body.Error)
}

func TestForecastQuery_NotReady(t *testing.T) {
	t.Parallel()
	called := false
	session := NewSession(context.Background(), SessionOptions{
		Forecast: func(context.Context, ForecastQuery) (forecast.Document, error) {
			called = true
			return forecast.Document{}, nil
		},
	})
	s := newRunningServer(t, Options{Session: session})
	assert.Equal(t, http.StatusServiceUnavailable,
		overviewRequest(t, s, http.MethodPost, "/api/forecast/query", `{}`).StatusCode)
	assert.False(t, called)

	unwired := newRunningServer(t, Options{Session: NewSession(context.Background(), SessionOptions{})})
	assert.Equal(t, http.StatusServiceUnavailable,
		overviewRequest(t, unwired, http.MethodPost, "/api/forecast/query", `{}`).StatusCode)
}

func TestForecastQuery_RejectsBadRequests(t *testing.T) {
	t.Parallel()
	s := newForecastServer(t, func(context.Context, ForecastQuery) (forecast.Document, error) {
		return forecastFixture(), nil
	})
	for _, body := range []string{`{"unexpected":true}`, `not json`, `{"months":"6"}`} {
		assert.Equal(t, http.StatusBadRequest,
			overviewRequest(t, s, http.MethodPost, "/api/forecast/query", body).StatusCode, body)
	}

	crossOrigin, err := http.NewRequest(http.MethodPost, s.BaseURL()+"/api/forecast/query", strings.NewReader(`{}`))
	require.NoError(t, err)
	crossOrigin.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	crossOrigin.Header.Set("Origin", "https://attacker.example")
	crossOrigin.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(crossOrigin)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	form, err := http.NewRequest(http.MethodPost, s.BaseURL()+"/api/forecast/query", strings.NewReader(`{}`))
	require.NoError(t, err)
	form.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	form.Header.Set("Origin", s.BaseURL())
	form.Header.Set("Content-Type", "text/plain")
	formResp, err := http.DefaultClient.Do(form)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, formResp.Body.Close()) })
	assert.Equal(t, http.StatusUnsupportedMediaType, formResp.StatusCode)
}
