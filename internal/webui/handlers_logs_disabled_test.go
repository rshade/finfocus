package webui

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsDisabledAnswersLikeAnUnknownPath(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})

	unknown := overviewRequest(t, s, http.MethodGet, "/api/no-such-route", "")
	require.Equal(t, http.StatusNotFound, unknown.StatusCode)
	want := readBody(t, unknown)
	assert.JSONEq(t, `{"error":"unknown path","code":"not_found"}`, want)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for _, path := range []string{"/api/logs/status", "/api/logs/stream"} {
			resp := overviewRequest(t, s, method, path, "")
			assert.Equal(t, http.StatusNotFound, resp.StatusCode, method+" "+path)
			assert.JSONEq(t, want, readBody(t, resp), method+" "+path)
		}
	}
}
