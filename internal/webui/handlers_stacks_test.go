package webui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stacksBody struct {
	Stacks []StackEntry `json:"stacks"`
}

func authPostJSON(t *testing.T, s *Server, cookie *http.Cookie, path, origin string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, s.BaseURL()+path, strings.NewReader("{}"))
	require.NoError(t, err)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

func decodeStacks(t *testing.T, body string) []StackEntry {
	t.Helper()
	var parsed stacksBody
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	return parsed.Stacks
}

func TestStacksListIsEmptyWithNoneRegistered(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	cookie := bootstrapSession(t, s)

	resp, body := authGet(t, s, cookie, "/api/stacks")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"stacks":[]}`, body)
}

func TestStacksListReportsOrderAndState(t *testing.T) {
	t.Parallel()
	prod := readySession(t, "urn::prod")
	s := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	require.NoError(t, s.AddStack("prod", StackOptions{Session: prod}))
	require.NoError(t, s.AddStack("dev", StackOptions{Session: NewSession(context.Background(), SessionOptions{})}))
	cookie := bootstrapSession(t, s)

	_, body := authGet(t, s, cookie, "/api/stacks")
	assert.Equal(t, []StackEntry{
		{Name: "prod", State: StackStateIdle},
		{Name: "dev", State: StackStateIdle},
	}, decodeStacks(t, body))

	_, ok := s.activateStack("prod")
	require.True(t, ok)
	_, body = authGet(t, s, cookie, "/api/stacks")
	assert.Equal(t, []StackEntry{
		{Name: "prod", State: StackStateReady},
		{Name: "dev", State: StackStateIdle},
	}, decodeStacks(t, body))
}

func TestStackActivateIsIdempotentAndRunsOnce(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	s := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	require.NoError(t, s.AddStack("dev", StackOptions{
		Session:  NewSession(context.Background(), SessionOptions{}),
		Activate: func() { calls.Add(1) },
	}))
	cookie := bootstrapSession(t, s)

	for range 3 {
		resp, body := authPostJSON(t, s, cookie, "/api/stacks/dev/activate", s.BaseURL())
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		assert.JSONEq(t, `{"name":"dev","state":"loading"}`, body)
	}
	assert.Equal(t, int32(1), calls.Load())
}

func TestStackPageActivatesOnlyItsOwnStack(t *testing.T) {
	t.Parallel()
	var devCalls, prodCalls atomic.Int32
	s := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	require.NoError(t, s.AddStack("dev", StackOptions{
		Session:  NewSession(context.Background(), SessionOptions{}),
		Activate: func() { devCalls.Add(1) },
	}))
	require.NoError(t, s.AddStack("prod", StackOptions{
		Session:  NewSession(context.Background(), SessionOptions{}),
		Activate: func() { prodCalls.Add(1) },
	}))
	cookie := bootstrapSession(t, s)

	resp, _ := authGet(t, s, cookie, "/")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, _ = authGet(t, s, cookie, "/s/prod/api/overview/budget")
	require.NotEqual(t, http.StatusNotFound, resp.StatusCode)
	assert.Zero(t, devCalls.Load(), "the root page starts no stack")
	assert.Zero(t, prodCalls.Load(), "an API request under a stack does not start it")

	for range 2 {
		resp, _ = authGet(t, s, cookie, "/s/prod/")
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	assert.Equal(t, int32(1), prodCalls.Load(), "opening the stack's page starts it once")
	assert.Zero(t, devCalls.Load())

	_, body := authGet(t, s, cookie, "/api/stacks")
	assert.JSONEq(t, `{"stacks":[{"name":"dev","state":"idle"},{"name":"prod","state":"loading"}]}`, body)
}

func TestStackActivateUnknownOrInvalidNameIs404(t *testing.T) {
	t.Parallel()
	s := newStackServer(t)
	cookie := bootstrapSession(t, s)

	for _, name := range []string{"nope", "bad$name", "a%2Fb", "a b"} {
		resp, body := authPostJSON(t, s, cookie, "/api/stacks/"+name+"/activate", s.BaseURL())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, name)
		assert.JSONEq(t, `{"error":"unknown stack","code":"not_found"}`, body, name)
	}
}

func TestStacksRoutesNeedAuthenticationAndOrigin(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	s := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	require.NoError(t, s.AddStack("dev", StackOptions{Activate: func() { calls.Add(1) }}))
	cookie := bootstrapSession(t, s)

	resp, body := authGet(t, s, nil, "/api/stacks")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, body, `"code":"unauthorized"`)

	resp, body = authPostJSON(t, s, nil, "/api/stacks/dev/activate", s.BaseURL())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, body, `"code":"unauthorized"`)

	resp, _ = authPostJSON(t, s, cookie, "/api/stacks/dev/activate", "http://evil.example")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp, _ = authPostJSON(t, s, cookie, "/api/stacks/dev/activate", "")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	assert.Zero(t, calls.Load(), "a rejected request must not start the stack")
}

func TestStacksRoutesAreRootOnly(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	s := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	require.NoError(t, s.AddStack("dev", StackOptions{
		Session:  readySession(t, "urn::dev"),
		Activate: func() { calls.Add(1) },
	}))
	cookie := bootstrapSession(t, s)

	resp, body := authGet(t, s, cookie, "/s/dev/api/stacks")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NotContains(t, body, `"stacks"`)

	resp, _ = authPostJSON(t, s, cookie, "/s/dev/api/stacks/dev/activate", s.BaseURL())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Zero(t, calls.Load())
}

func TestShellListsStacksOnlyOnTheRootWithTwoOrMore(t *testing.T) {
	t.Parallel()
	const stacksMeta = `<meta name="finfocus-stacks"`

	none := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	one := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	require.NoError(t, one.AddStack("dev", StackOptions{}))
	several := newRunningServer(t, Options{Session: readySession(t, "urn::root")})
	for _, name := range []string{"prod", "dev", "test"} {
		require.NoError(t, several.AddStack(name, StackOptions{}))
	}

	tests := []struct {
		name   string
		server *Server
		path   string
		want   string
	}{
		{"no stacks", none, "/", ""},
		{"one stack is the single-stack page", one, "/", ""},
		{
			"several stacks, in registration order", several, "/",
			`<meta name="finfocus-stacks" content="prod,dev,test">`,
		},
		{"a stack's own page never lists stacks", several, "/s/dev/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cookie := bootstrapSession(t, tt.server)
			resp, body := authGet(t, tt.server, cookie, tt.path)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, `<meta name="finfocus-base"`)
			if tt.want == "" {
				assert.NotContains(t, body, stacksMeta)
				return
			}
			assert.Contains(t, body, tt.want)
		})
	}
}

// A stack list served through `tailscale serve` (--allowed-host,
// --trusted-proxy, --allowed-user) admits the identity header on the stack
// list, activation, and every stack's own tree, and refuses an unlisted login
// on each of them.
func TestStacksRoutesAdmitTrustedProxyIdentity(t *testing.T) {
	t.Parallel()
	const loopbackPeer = "127.0.0.1:50000"
	var activations atomic.Int32
	opts := trustedOptions()
	opts.Session = readySession(t, "urn::root")
	s, err := New(opts)
	require.NoError(t, err)
	require.NoError(t, s.AddStack("dev", StackOptions{
		Session:  readySession(t, "urn::dev"),
		Activate: func() { activations.Add(1) },
	}))
	require.NoError(t, s.AddStack("prod", StackOptions{Session: readySession(t, "urn::prod")}))

	request := func(method, path, login string) *http.Request {
		req := identityRequest(method, proxyHost, login)
		req.URL.Path = path
		if method == http.MethodPost {
			req = withHeader(withOrigin(req, "https://"+proxyHost), "Content-Type", "application/json")
		}
		return req
	}
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/stacks", want: http.StatusOK},
		{method: http.MethodPost, path: "/api/stacks/dev/activate", want: http.StatusAccepted},
		{method: http.MethodGet, path: "/s/dev/", want: http.StatusOK},
		{method: http.MethodGet, path: "/s/prod/api/overview/budget", want: http.StatusOK},
	}
	for _, tt := range tests {
		admitted := serve(t, s, request(tt.method, tt.path, proxyUser), loopbackPeer)
		assert.Equal(t, tt.want, admitted.Code, tt.path)
		assert.Empty(t, admitted.Result().Cookies(), "identity admission must not issue a cookie: %s", tt.path)

		refused := serve(t, s, request(tt.method, tt.path, "mallory@example.com"), loopbackPeer)
		assert.Equal(t, http.StatusUnauthorized, refused.Code, tt.path)
	}
	assert.Equal(t, int32(1), activations.Load(), "only the admitted activation runs the stack")
}
