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
)

func TestAnomaliesParityWithCLIJSON(t *testing.T) {
	t.Parallel()
	rows := anomalyFixtureRows()
	var cli bytes.Buffer
	require.NoError(t, engine.RenderAnomaliesAsJSON(&cli, engine.SelectAnomalies(rows)))
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
