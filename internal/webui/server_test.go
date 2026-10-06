package webui

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/logging"
)

// newRunningServer creates a server on 127.0.0.1 with a kernel-assigned port,
// starts serving in the background, and shuts it down at test cleanup.
func newRunningServer(t *testing.T, opts Options) *Server {
	t.Helper()
	s, err := New(opts)
	require.NoError(t, err)
	require.NoError(t, s.Listen())
	go func() {
		_ = s.Serve()
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, s.Shutdown(ctx))
	})
	return s
}

// noRedirectClient returns an [http.Client] that does not follow redirects, so
// the 303 bootstrap answer can be inspected directly.
func noRedirectClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// sessionToken extracts the session token from the server's printed URL.
func sessionToken(t *testing.T, s *Server) string {
	t.Helper()
	u, err := url.Parse(s.URL())
	require.NoError(t, err)
	token := u.Query().Get("token")
	require.NotEmpty(t, token)
	return token
}

// bootstrapSession performs the token bootstrap and returns the session cookie.
func bootstrapSession(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	resp, err := noRedirectClient().Get(s.URL())
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	return cookies[0]
}

func TestTokenBootstrapSetsSessionCookie(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	token := sessionToken(t, s)

	resp, err := noRedirectClient().Get(s.URL())
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.Equal(t, "finfocus_session_"+strconv.Itoa(s.Port()), cookie.Name)
	assert.Equal(t, token, cookie.Value)
	assert.True(t, cookie.HttpOnly, "session cookie must be HttpOnly")
	assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite, "session cookie must be SameSite=Strict")
	assert.Equal(t, "/", cookie.Path)
	assert.False(t, cookie.Secure, "localhost HTTP session cookie must not be Secure")
}

func TestTokenIs128BitHex(t *testing.T) {
	t.Parallel()
	s, err := New(Options{})
	require.NoError(t, err)
	token := sessionToken(t, s)
	assert.Len(t, token, 32, "128-bit token is 32 hex characters")
	for _, r := range token {
		assert.Contains(t, "0123456789abcdef", string(r))
	}
}

func TestTokenWorksSecondTime(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})

	for range 2 {
		resp, err := noRedirectClient().Get(s.URL())
		require.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	}
}

func TestMissingOrInvalidAuth(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	base := s.BaseURL()

	tests := []struct {
		name   string
		url    string
		cookie *http.Cookie
	}{
		{name: "no token and no cookie", url: base + "/"},
		{name: "wrong token", url: base + "/?token=00000000000000000000000000000000"},
		{name: "static asset without cookie", url: base + "/static/app.js"},
		{name: "unguarded-looking api path without cookie", url: base + "/api/overview/budget"},
		{
			name:   "invalid cookie value",
			url:    base + "/",
			cookie: &http.Cookie{Name: "finfocus_session_" + strconv.Itoa(s.Port()), Value: "deadbeef"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tt.url, nil)
			require.NoError(t, err)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			resp, err := noRedirectClient().Do(req)
			require.NoError(t, err)
			defer func() { assert.NoError(t, resp.Body.Close()) }()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestValidCookieServesSPAAndStaticAssets(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	cookie := bootstrapSession(t, s)
	client := noRedirectClient()

	get := func(path string) *http.Response {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+path, nil)
		require.NoError(t, err)
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	resp := get("/")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	assert.Contains(t, string(body), "<html")

	resp = get("/static/app.js")
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp = get("/static/styles.css")
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHostHeaderValidation(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	cookie := bootstrapSession(t, s)
	port := strconv.Itoa(s.Port())

	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "127.0.0.1 with port", host: "127.0.0.1:" + port, want: http.StatusOK},
		{name: "localhost with port", host: "localhost:" + port, want: http.StatusOK},
		{name: "foreign host", host: "evil.example.com", want: http.StatusForbidden},
		{name: "loopback wrong port", host: "127.0.0.1:1", want: http.StatusForbidden},
		{name: "localhost wrong port", host: "localhost:1", want: http.StatusForbidden},
		{name: "loopback no port", host: "127.0.0.1", want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
			require.NoError(t, err)
			req.Host = tt.host
			req.AddCookie(cookie)
			resp, err := noRedirectClient().Do(req)
			require.NoError(t, err)
			defer func() { assert.NoError(t, resp.Body.Close()) }()
			assert.Equal(t, tt.want, resp.StatusCode)
		})
	}
}

func TestPostOriginAndContentTypeChecks(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	cookie := bootstrapSession(t, s)
	goodOrigin := s.BaseURL()

	post := func(origin, contentType string) *http.Response {
		t.Helper()
		req, err := http.NewRequestWithContext(
			context.Background(), http.MethodPost, s.BaseURL()+"/", strings.NewReader(`{}`),
		)
		require.NoError(t, err)
		req.AddCookie(cookie)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := noRedirectClient().Do(req)
		require.NoError(t, err)
		return resp
	}

	t.Run("missing origin", func(t *testing.T) {
		t.Parallel()
		resp := post("", "application/json")
		defer func() { assert.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("foreign origin", func(t *testing.T) {
		t.Parallel()
		resp := post("https://evil.example.com", "application/json")
		defer func() { assert.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("non-JSON content type", func(t *testing.T) {
		t.Parallel()
		resp := post(goodOrigin, "text/plain")
		defer func() { assert.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("missing content type", func(t *testing.T) {
		t.Parallel()
		resp := post(goodOrigin, "")
		defer func() { assert.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)
	})

	t.Run("same origin with JSON passes middleware", func(t *testing.T) {
		t.Parallel()
		resp := post(goodOrigin, "application/json")
		defer func() { assert.NoError(t, resp.Body.Close()) }()
		// / accepts only GET; anything but 401/403/415 proves the hardening
		// middleware let the request through.
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("localhost origin accepted", func(t *testing.T) {
		t.Parallel()
		resp := post("http://localhost:"+strconv.Itoa(s.Port()), "application/json; charset=utf-8")
		defer func() { assert.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

func TestGetWithForeignOriginRejected(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	cookie := bootstrapSession(t, s)

	tests := []struct {
		name   string
		origin string
		want   int
	}{
		{name: "foreign origin", origin: "https://evil.example.com", want: http.StatusForbidden},
		{name: "matching origin", origin: s.BaseURL(), want: http.StatusOK},
		{name: "no origin header", origin: "", want: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
			require.NoError(t, err)
			req.AddCookie(cookie)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			resp, err := noRedirectClient().Do(req)
			require.NoError(t, err)
			defer func() { assert.NoError(t, resp.Body.Close()) }()
			assert.Equal(t, tt.want, resp.StatusCode)
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})

	check := func(t *testing.T, resp *http.Response) {
		t.Helper()
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
		assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	}

	// On the token bootstrap response.
	resp, err := noRedirectClient().Get(s.URL())
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	check(t, resp)

	// On an authenticated response.
	cookie := bootstrapSession(t, s)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
	require.NoError(t, err)
	req.AddCookie(cookie)
	resp, err = noRedirectClient().Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	check(t, resp)

	// On a rejection.
	resp, err = noRedirectClient().Get(s.BaseURL() + "/")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	check(t, resp)
}

func TestRequestLogOmitsTokenAndBodies(t *testing.T) {
	t.Parallel()
	var logBuf bytes.Buffer
	logger := logging.NewLoggerWithWriter(logging.Config{Format: "json"}, &logBuf)
	s := newRunningServer(t, Options{Logger: &logger})
	token := sessionToken(t, s)

	// Token bootstrap: the query string carries the token.
	resp, err := noRedirectClient().Get(s.URL())
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	// POST with a sensitive body.
	cookie := bootstrapSession(t, s)
	const secretBody = "topsecret-body-value-7f3a"
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, s.BaseURL()+"/",
		strings.NewReader(`{"passphrase":"`+secretBody+`"}`),
	)
	require.NoError(t, err)
	req.AddCookie(cookie)
	req.Header.Set("Origin", s.BaseURL())
	req.Header.Set("Content-Type", "application/json")
	resp, err = noRedirectClient().Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := logBuf.String()
	require.NotEmpty(t, out, "requests must be logged")
	assert.Contains(t, out, `"method":"GET"`)
	assert.Contains(t, out, `"path":"/"`)
	assert.NotContains(t, out, token, "session token must never be logged")
	assert.NotContains(t, out, "token=", "query strings must never be logged")
	assert.NotContains(t, out, secretBody, "request bodies must never be logged")
}

func TestListensOnLoopbackOnly(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	addr, ok := s.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener must be a TCP address")
	require.NotNil(t, addr.IP)
	assert.True(t, addr.IP.IsLoopback(), "must bind loopback only, got %s", addr.IP)
}

func TestPortZeroAssignsActualPort(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{Port: 0})
	assert.NotZero(t, s.Port(), "kernel-assigned port must be reported")
	assert.Contains(t, s.URL(), "http://127.0.0.1:"+strconv.Itoa(s.Port())+"/?token=")
}

func TestPinnedPortUsedWhenFree(t *testing.T) {
	t.Parallel()
	// Reserve then release a port to learn a likely-free port number.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := probe.Addr().(*net.TCPAddr).Port
	require.NoError(t, probe.Close())

	s, err := New(Options{Port: port})
	require.NoError(t, err)
	require.NoError(t, s.Listen())
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, s.Shutdown(ctx))
	}()
	go func() { _ = s.Serve() }()
	assert.Equal(t, port, s.Port())
}

func TestPinnedBusyPortFailsWithClearError(t *testing.T) {
	t.Parallel()
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { assert.NoError(t, blocker.Close()) }()
	port := blocker.Addr().(*net.TCPAddr).Port

	s, err := New(Options{Port: port})
	require.NoError(t, err)
	err = s.Listen()
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(port))
	assert.Contains(t, err.Error(), "unavailable")
}

func TestInvalidPortRejected(t *testing.T) {
	t.Parallel()
	for _, port := range []int{-1, 70000} {
		_, err := New(Options{Port: port})
		require.Error(t, err)
	}
}

//nolint:paralleltest // Sends os.Interrupt to the whole test process; must not overlap other tests.
func TestGracefulShutdownOnInterrupt(t *testing.T) {
	s, err := New(Options{})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- s.Run(context.Background())
	}()

	// Wait until the server answers.
	require.Eventually(t, func() bool {
		resp, getErr := noRedirectClient().Get(s.URL())
		if getErr != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusSeeOther
	}, 10*time.Second, 25*time.Millisecond)

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))

	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after interrupt")
	}

	// The listener is closed: new connections fail.
	_, getErr := noRedirectClient().Get(s.URL())
	require.Error(t, getErr)
}

func TestRunReturnsListenError(t *testing.T) {
	t.Parallel()
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { assert.NoError(t, blocker.Close()) }()

	s, err := New(Options{Port: blocker.Addr().(*net.TCPAddr).Port})
	require.NoError(t, err)
	require.Error(t, s.Run(context.Background()))
}
