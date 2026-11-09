package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func driftRows(urn string) []engine.OverviewRow {
	period := engine.DateRange{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
	}
	return []engine.OverviewRow{{
		URN: urn, Type: "aws:ec2/instance:Instance", Status: engine.StatusActive,
		ActualCost: &engine.ActualCostData{MTDCost: 360, Currency: "USD", Period: period},
		CostDrift: &engine.CostDriftData{
			ExtrapolatedMonthly: 1080, Projected: 720, Delta: 360, PercentDrift: 50, IsWarning: true,
		},
	}}
}

func readySession(t *testing.T, urn string) *Session {
	t.Helper()
	session := NewSession(context.Background(), SessionOptions{})
	session.Ready(driftRows(urn))
	return session
}

func authGet(t *testing.T, s *Server, cookie *http.Cookie, path string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+path, nil)
	require.NoError(t, err)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

func anomalyResources(t *testing.T, body string) []string {
	t.Helper()
	var page AnomalyPage
	require.NoError(t, json.Unmarshal([]byte(body), &page))
	out := make([]string, 0, len(page.Anomalies))
	for _, a := range page.Anomalies {
		out = append(out, a.Resource)
	}
	return out
}

func newStackServer(t *testing.T) *Server {
	t.Helper()
	sessions := map[string]*Session{
		"root": readySession(t, "urn::root"),
		"dev":  readySession(t, "urn::dev"),
		"prod": readySession(t, "urn::prod"),
	}
	s := newRunningServer(t, Options{Session: sessions["root"]})
	require.NoError(t, s.AddStack("dev", StackOptions{Session: sessions["dev"]}))
	require.NoError(t, s.AddStack("prod", StackOptions{Session: sessions["prod"]}))
	return s
}

func TestStackRequestsReachTheirOwnSession(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	tests := []struct {
		path string
		want string
	}{
		{"/api/overview/anomalies", "urn::root"},
		{"/s/dev/api/overview/anomalies", "urn::dev"},
		{"/s/prod/api/overview/anomalies", "urn::prod"},
	}
	for _, tt := range tests {
		resp, body := authGet(t, s, cookie, tt.path)
		require.Equal(t, http.StatusOK, resp.StatusCode, tt.path)
		assert.Equal(t, []string{tt.want}, anomalyResources(t, body), tt.path)
	}
}

func TestStackStaticAssetsServeUnderPrefix(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	for _, path := range []string{"/s/dev/static/app.js", "/s/dev/static/zerolog-viewer/index.js"} {
		resp, _ := authGet(t, s, cookie, path)
		assert.Equal(t, http.StatusOK, resp.StatusCode, path)
	}
}

func TestStackUnknownIs404OnlyWhenAuthenticated(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	for _, path := range []string{"/s/nope/api/overview/anomalies", "/s/", "/s/nope/", "/s/../x"} {
		resp, body := authGet(t, s, cookie, path)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
		assert.Contains(t, body, `"code":"not_found"`, path)
	}
	for _, path := range []string{"/s/nope/api/overview/anomalies", "/s/dev/api/overview/anomalies", "/s/dev/", "/s/dev"} {
		resp, body := authGet(t, s, nil, path)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
		assert.Contains(t, body, `"code":"unauthorized"`, path)
	}
}

func TestStackWithoutTrailingSlashRedirects(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	resp, _ := authGet(t, s, cookie, "/s/dev?view=1")
	assert.Equal(t, http.StatusPermanentRedirect, resp.StatusCode)
	assert.Equal(t, "/s/dev/?view=1", resp.Header.Get("Location"))
}

func TestStackShellCarriesItsPrefix(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	tests := []struct {
		path, meta string
	}{
		{"/", `<meta name="finfocus-base" content="">`},
		{"/s/dev/", `<meta name="finfocus-base" content="/s/dev">`},
		{"/s/prod/", `<meta name="finfocus-base" content="/s/prod">`},
	}
	for _, tt := range tests {
		resp, body := authGet(t, s, cookie, tt.path)
		require.Equal(t, http.StatusOK, resp.StatusCode, tt.path)
		assert.Contains(t, body, tt.meta, tt.path)
		assert.NotContains(t, body, `="/static/`, "shell must reference assets relatively: "+tt.path)
		assert.Contains(t, body, `src="static/app.js"`, tt.path)
	}
}

func TestStackTokenBootstrapReturnsToTheStackURL(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	token := sessionToken(t, s)

	resp, _ := authGet(t, s, nil, "/s/dev/?token="+token)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/s/dev/", resp.Header.Get("Location"))
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "/", cookies[0].Path, "one cookie signs the browser into every stack")

	resp, _ = authGet(t, s, cookies[0], "/s/prod/api/overview/anomalies")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = authGet(t, s, nil, "/s/dev/?token=wrong")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp, _ = authGet(t, s, nil, "/s/nope/?token="+token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a valid token on an unknown stack is a 404")
}

func TestStackInvalidTokenRevealsNothingAboutRegisteredStacks(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)

	respKnown, bodyKnown := authGet(t, s, nil, "/s/dev/?token=wrong")
	respUnknown, bodyUnknown := authGet(t, s, nil, "/s/nope/?token=wrong")
	assert.Equal(t, http.StatusUnauthorized, respKnown.StatusCode)
	assert.Equal(t, respKnown.StatusCode, respUnknown.StatusCode)
	assert.Equal(t, bodyKnown, bodyUnknown)

	respRoot, bodyRoot := authGet(t, s, nil, "/?token=wrong")
	assert.Equal(t, http.StatusUnauthorized, respRoot.StatusCode)
	assert.Equal(t, bodyKnown, bodyRoot, "the root keeps the same invalid-token answer")
}

func TestStackMuxRedirectsStayInsideThePrefix(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	tests := map[string]string{
		"/s/dev//api/overview/anomalies":       "/s/dev/api/overview/anomalies",
		"/s/dev/api/../api/overview/anomalies": "/s/dev/api/overview/anomalies",
		"/s/dev/static":                        "/s/dev/static/",
	}
	for path, want := range tests {
		resp, _ := authGet(t, s, cookie, path)
		assert.GreaterOrEqual(t, resp.StatusCode, 300, path)
		assert.Less(t, resp.StatusCode, 400, path)
		assert.Equal(t, want, resp.Header.Get("Location"), path)
	}
}

func TestStackTokenBootstrapOnlyAtRootOfStack(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	token := sessionToken(t, s)

	resp, _ := authGet(t, s, nil, "/s/dev/api/overview/anomalies?token="+token)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAddStackValidatesNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		valid bool
	}{
		{"dev", true},
		{"prod-1", true},
		{"a.b_c", true},
		{strings.Repeat("a", 100), true},
		{"", false},
		{".", false},
		{"..", false},
		{"a/b", false},
		{"a b", false},
		{"a?b", false},
		{"a#b", false},
		{"a%2fb", false},
		{`a"b`, false},
		{"<script>", false},
		{strings.Repeat("a", 101), false},
	}
	for _, tt := range tests {
		s, err := New(Options{})
		require.NoError(t, err)
		err = s.AddStack(tt.name, StackOptions{})
		if tt.valid {
			require.NoError(t, err, tt.name)
		} else {
			require.ErrorIs(t, err, ErrInvalidStackName, tt.name)
		}
	}
}

func TestAddStackRejectsDuplicateAndLateRegistration(t *testing.T) {
	t.Parallel()
	s, err := New(Options{})
	require.NoError(t, err)
	require.NoError(t, s.AddStack("dev", StackOptions{}))
	require.ErrorIs(t, s.AddStack("dev", StackOptions{}), ErrDuplicateStack)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
	require.ErrorIs(t, s.AddStack("prod", StackOptions{}), http.ErrServerClosed)
}

func TestShutdownClosesStackSessions(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	s, err := New(Options{})
	require.NoError(t, err)
	require.NoError(t, s.AddStack("dev", StackOptions{Session: session}))
	require.NoError(t, session.Context().Err())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
	require.Error(t, session.Context().Err())
}

func TestStackLogsRoutesStayOutOfTheRequestLog(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	s := newRunningServer(t, Options{Logger: &logger})
	require.NoError(t, s.AddStack("dev", StackOptions{}))
	cookie := bootstrapSession(t, s)
	buf.Reset()

	resp, _ := authGet(t, s, cookie, "/s/dev/api/logs/status")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, buf.String(), "http_request")

	resp, _ = authGet(t, s, cookie, "/s/dev/api/overview/budget")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, buf.String(), "/s/dev/api/overview/budget")
}

func TestStackHardeningAppliesToPrefixedRequests(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		s.BaseURL()+"/s/dev/api/overview/preview", strings.NewReader("{}"))
	require.NoError(t, err)
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet,
		s.BaseURL()+"/s/dev/api/overview/anomalies", nil)
	require.NoError(t, err)
	req.AddCookie(cookie)
	req.Host = "evil.example"
	resp2, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp2.Body.Close()) }()
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
}
