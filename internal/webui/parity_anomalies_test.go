package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

func TestAnomaliesParityWithCLIJSON(t *testing.T) {
	t.Parallel()
	rows := anomalyFixtureRows()
	var cli bytes.Buffer
	require.NoError(t, engine.RenderAnomaliesAsJSON(&cli, engine.SelectAnomalies(rows, nil)))
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(rows)
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	var page map[string]any
	require.NoError(t, json.Unmarshal(body, &page))
	assert.Contains(t, page, "summary")
	delete(page, "summary")
	for _, item := range page["anomalies"].([]any) {
		row := item.(map[string]any)
		require.Contains(t, row, "display")
		delete(row, "display")
	}
	stripped, err := json.Marshal(page)
	require.NoError(t, err)

	assert.JSONEq(t, cli.String(), string(stripped), "the page adds display text and nothing else")
}

// TestAnomaliesParityWithCLIJSONForDailySeries runs the same session rows and
// the same stored series through both readers: the CLI's shared selector and
// the web handler. The page may add display text, but the method, baseline
// window, and every other field must match the CLI JSON exactly.
func TestAnomaliesParityWithCLIJSONForDailySeries(t *testing.T) {
	t.Parallel()
	rows := append(anomalyFixtureRows(), engine.OverviewRow{
		URN: "urn::spiking", Type: "aws:ec2/instance:Instance", Status: engine.StatusActive,
	})
	series := dailyAnomalyFixture()

	var cli bytes.Buffer
	require.NoError(t, engine.RenderAnomaliesAsJSON(&cli, engine.SelectAnomalies(rows, series)))
	session := NewSession(context.Background(), SessionOptions{
		DailyPoints: func(context.Context) ([]history.DailyPoint, error) {
			return series, nil
		},
	})
	session.Ready(rows)
	s := newRunningServer(t, Options{Session: session})

	response := overviewRequest(t, s, http.MethodGet, "/api/overview/anomalies", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	var page map[string]any
	require.NoError(t, json.Unmarshal(body, &page))
	delete(page, "summary")
	for _, item := range page["anomalies"].([]any) {
		row := item.(map[string]any)
		require.Contains(t, row, "display")
		delete(row, "display")
	}
	stripped, err := json.Marshal(page)
	require.NoError(t, err)

	assert.Contains(t, string(stripped), `"method":"robust-median"`)
	assert.JSONEq(t, cli.String(), string(stripped), "the page adds display text and nothing else")
}
