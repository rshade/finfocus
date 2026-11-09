package webui

import (
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func TestLogsDisabledStatusIsOffAndStreamIsNotFound(t *testing.T) {
	t.Parallel()
	var buf syncBuffer
	logger := zerolog.New(&buf)
	s := newRunningServer(t, Options{Logger: &logger})

	unknown := overviewRequest(t, s, http.MethodGet, "/api/no-such-route", "")
	require.Equal(t, http.StatusNotFound, unknown.StatusCode)
	wantNotFound := readBody(t, unknown)
	assert.JSONEq(t, `{"error":"unknown path","code":"not_found"}`, wantNotFound)

	status := overviewRequest(t, s, http.MethodGet, "/api/logs/status", "")
	require.Equal(t, http.StatusOK, status.StatusCode)
	assert.JSONEq(t, `{"enabled":false,"capacity":0,"retained":0,"dropped":0,"sources":[]}`, readBody(t, status))
	assert.NotContains(t, buf.String(), "/api/logs/")

	postStatus := overviewRequest(t, s, http.MethodPost, "/api/logs/status", "{}")
	assert.Equal(t, http.StatusMethodNotAllowed, postStatus.StatusCode)
	assert.Equal(t, http.MethodGet, postStatus.Header.Get("Allow"))
	assert.JSONEq(t, `{"error":"GET only","code":"method_not_allowed"}`, readBody(t, postStatus))

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		resp := overviewRequest(t, s, method, "/api/logs/stream", "{}")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, method)
		assert.JSONEq(t, wantNotFound, readBody(t, resp), method)
	}
}

func TestLogsDisabledStatusRequiresSession(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	req, err := http.NewRequest(http.MethodGet, s.BaseURL()+"/api/logs/status", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLogsTypedNilHubIsDisabled(t *testing.T) {
	t.Parallel()
	var hub *weblog.Hub
	s := newRunningServer(t, Options{Logs: hub})

	status := overviewRequest(t, s, http.MethodGet, "/api/logs/status", "")
	require.Equal(t, http.StatusOK, status.StatusCode)
	assert.JSONEq(t, `{"enabled":false,"capacity":0,"retained":0,"dropped":0,"sources":[]}`, readBody(t, status))

	stream := overviewRequest(t, s, http.MethodGet, "/api/logs/stream", "")
	assert.Equal(t, http.StatusNotFound, stream.StatusCode)
}
