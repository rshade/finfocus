package esctest

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Configurations(t *testing.T) {
	t.Parallel()

	env := &Environment{
		Variables: map[string]Variable{
			"TOKEN":  {Value: "one", Secret: true},
			"REGION": {Value: "us-east-1", Secret: false, Rotate: true},
		},
		ETag: `"rev-1"`,
		Tags: map[string]int{"stable": 4},
	}
	srv := New("test-token", map[string]*Environment{"acme/billing/prod": env})
	t.Cleanup(srv.Close)

	status := do(t, srv, http.MethodPost, "/api/esc/environments/acme/billing/prod/open", "")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, 0, srv.Opens("acme", "billing", "prod"))

	body := authed(t, srv, http.MethodPost, "/api/esc/environments/acme/billing/prod/open")
	assert.Contains(t, body, `"id"`)
	assert.Equal(t, 1, srv.Opens("acme", "billing", "prod"))

	read := authed(t, srv, http.MethodGet, "/api/esc/environments/acme/billing/prod/open/open-1")
	assert.Contains(t, read, `"TOKEN"`)
	assert.Contains(t, read, `"secret":true`)
	assert.Contains(t, read, `"us-east-1-1"`)

	second := authed(t, srv, http.MethodPost, "/api/esc/environments/acme/billing/prod/open")
	assert.Contains(t, second, `"id"`)
	read2 := authed(t, srv, http.MethodGet, "/api/esc/environments/acme/billing/prod/open/open-2")
	assert.Contains(t, read2, `"us-east-1-2"`)
	assert.Equal(t, 2, srv.Opens("acme", "billing", "prod"))

	headReq, err := http.NewRequest(http.MethodHead, srv.URL+"/api/esc/environments/acme/billing/prod", nil)
	require.NoError(t, err)
	headReq.Header.Set("Authorization", "token test-token")
	headResp, err := http.DefaultClient.Do(headReq)
	require.NoError(t, err)
	defer headResp.Body.Close()
	assert.Equal(t, http.StatusOK, headResp.StatusCode)
	assert.Equal(t, `"rev-1"`, headResp.Header.Get("ETag"))
	assert.Equal(t, 1, srv.Heads("acme", "billing", "prod"))

	tag := authed(t, srv, http.MethodGet, "/api/esc/environments/acme/billing/prod/versions/tags/stable")
	assert.Contains(t, tag, `"revision":4`)
	env.Tags["stable"] = 9
	moved := authed(t, srv, http.MethodGet, "/api/esc/environments/acme/billing/prod/versions/tags/stable")
	assert.Contains(t, moved, `"revision":9`)
	assert.Equal(t, `"rev-1"`, headResp.Header.Get("ETag"))
}

func TestServer_StatusDelayAndMalformedTrace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		env    *Environment
		status int
		want   string
	}{
		{
			name:   "unauthorized status with message",
			env:    &Environment{Status: http.StatusUnauthorized, Message: "nope"},
			status: http.StatusUnauthorized,
			want:   `"message":"nope"`,
		},
		{
			name:   "forbidden",
			env:    &Environment{Status: http.StatusForbidden, Message: "denied"},
			status: http.StatusForbidden,
			want:   `"code":403`,
		},
		{
			name:   "not found",
			env:    &Environment{Status: http.StatusNotFound, Message: "missing"},
			status: http.StatusNotFound,
			want:   `"message":"missing"`,
		},
		{
			name:   "server error with message",
			env:    &Environment{Status: http.StatusInternalServerError, Message: "boom"},
			status: http.StatusInternalServerError,
			want:   `"message":"boom"`,
		},
		{
			name:   "server error without message",
			env:    &Environment{Status: http.StatusInternalServerError, OmitMessage: true},
			status: http.StatusInternalServerError,
			want:   `"code":500`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := New("tok", map[string]*Environment{"acme/billing/prod": tc.env})
			t.Cleanup(srv.Close)
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/esc/environments/acme/billing/prod/open", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "token tok")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.status, resp.StatusCode)
			assert.Contains(t, string(body), tc.want)
			if tc.env.OmitMessage {
				assert.NotContains(t, string(body), `"message"`)
			}
		})
	}

	t.Run("delay", func(t *testing.T) {
		t.Parallel()
		srv := New("tok", map[string]*Environment{
			"acme/billing/slow": {Delay: 150 * time.Millisecond, ETag: "e"},
		})
		t.Cleanup(srv.Close)
		start := time.Now()
		status := do(t, srv, http.MethodHead, "/api/esc/environments/acme/billing/slow", "tok")
		assert.Equal(t, http.StatusOK, status)
		assert.GreaterOrEqual(t, time.Since(start), 150*time.Millisecond)
	})

	t.Run("malformed trace", func(t *testing.T) {
		t.Parallel()
		srv := New("tok", map[string]*Environment{
			"acme/billing/bad": {
				Variables:      map[string]Variable{"TOKEN": {Value: "x", Secret: true}},
				MalformedTrace: true,
			},
		})
		t.Cleanup(srv.Close)
		_ = authed(t, srv, http.MethodPost, "/api/esc/environments/acme/billing/bad/open")
		body := authed(t, srv, http.MethodGet, "/api/esc/environments/acme/billing/bad/open/open-1")
		assert.Contains(t, body, `"trace":{}`)
	})
}

func authed(t *testing.T, srv *Server, method, path string) string {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "token "+srv.token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	return string(body)
}

func do(t *testing.T, srv *Server, method, path, token string) int {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}
