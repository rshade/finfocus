package webui

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	proxyHost = "dash.example.ts.net"
	proxyUser = "alice@example.com"
)

func proxyOptions() Options {
	return Options{AllowedHosts: []string{proxyHost}}
}

func trustedOptions() Options {
	return Options{AllowedHosts: []string{proxyHost}, TrustedProxy: true, AllowedUsers: []string{proxyUser}}
}

func TestAllowedHostAcceptedByHostCheck(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, proxyOptions())
	cookie := bootstrapSession(t, s)

	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "allowed host", host: proxyHost, want: http.StatusOK},
		{name: "mixed case", host: "Dash.Example.TS.net", want: http.StatusOK},
		{name: "trailing dot", host: proxyHost + ".", want: http.StatusOK},
		{name: "default https port", host: proxyHost + ":443", want: http.StatusOK},
		{name: "other port", host: proxyHost + ":8443", want: http.StatusForbidden},
		{name: "prefix near miss", host: "evil." + proxyHost, want: http.StatusForbidden},
		{name: "suffix near miss", host: proxyHost + ".evil.com", want: http.StatusForbidden},
		{name: "foreign host", host: "evil.example.com", want: http.StatusForbidden},
		{name: "loopback still accepted", host: "127.0.0.1:" + portString(s), want: http.StatusOK},
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

func TestDefaultServerRefusesProxyHost(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	cookie := bootstrapSession(t, s)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
	require.NoError(t, err)
	req.Host = proxyHost
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAllowedHostKeepsLoopbackBind(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, trustedOptions())
	addr, ok := s.Addr().(*net.TCPAddr)
	require.True(t, ok)
	assert.True(t, addr.IP.Equal(net.ParseIP(loopbackAddr)), "bind address must stay %s, got %s", loopbackAddr, addr.IP)
}

func TestAllowedOriginOnStateChangingRequest(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, proxyOptions())
	cookie := bootstrapSession(t, s)

	tests := []struct {
		name   string
		origin string
		want   int
	}{
		{name: "allowed https origin", origin: "https://" + proxyHost, want: http.StatusMethodNotAllowed},
		{name: "trailing dot origin", origin: "https://" + proxyHost + ".", want: http.StatusMethodNotAllowed},
		{name: "http origin for allowed host", origin: "http://" + proxyHost, want: http.StatusForbidden},
		{name: "foreign origin", origin: "https://evil.example", want: http.StatusForbidden},
		{name: "suffix near miss", origin: "https://" + proxyHost + ".evil.com", want: http.StatusForbidden},
		{name: "loopback origin", origin: s.BaseURL(), want: http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				s.BaseURL()+"/",
				strings.NewReader("{}"),
			)
			require.NoError(t, err)
			req.Host = proxyHost
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(cookie)
			resp, err := noRedirectClient().Do(req)
			require.NoError(t, err)
			defer func() { assert.NoError(t, resp.Body.Close()) }()
			assert.Equal(t, tt.want, resp.StatusCode)
		})
	}
}

func TestGetWithForeignOriginStillRejectedWithAllowedHost(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, proxyOptions())
	cookie := bootstrapSession(t, s)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
	require.NoError(t, err)
	req.Host = proxyHost
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSessionCookieSecureBehindHTTPSProxy(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, proxyOptions())
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "https", header: "https", want: true},
		{name: "https uppercase", header: "HTTPS", want: true},
		{name: "http", header: "http", want: false},
		{name: "absent", header: "", want: false},
		{name: "first value wins", header: "https, http", want: true},
		{name: "garbage", header: "gopher", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.URL(), nil)
			require.NoError(t, err)
			if tt.header != "" {
				req.Header.Set("X-Forwarded-Proto", tt.header)
			}
			resp, err := noRedirectClient().Do(req)
			require.NoError(t, err)
			defer func() { assert.NoError(t, resp.Body.Close()) }()
			require.Equal(t, http.StatusSeeOther, resp.StatusCode)
			cookies := resp.Cookies()
			require.Len(t, cookies, 1)
			assert.Equal(t, tt.want, cookies[0].Secure)
		})
	}
}

func TestForwardedProtoIgnoredWithoutAllowedHost(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.URL(), nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.False(t, cookies[0].Secure, "without an allowed host the cookie must stay exactly as before")
}

func TestForwardedProtoNeverGrantsAccess(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, proxyOptions())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// serve drives the server's full middleware chain without a socket, so a test
// can choose the peer address.
func serve(t *testing.T, s *Server, req *http.Request, peer string) *httptest.ResponseRecorder {
	t.Helper()
	req.RemoteAddr = peer
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)
	return rec
}

func identityRequest(method, host string, logins ...string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), method, "/", nil)
	req.Host = host
	for _, login := range logins {
		req.Header.Add(identityLoginHeader, login)
	}
	return req
}

func TestTrustedProxyIdentityAdmission(t *testing.T) {
	t.Parallel()
	const loopbackPeer = "127.0.0.1:50000"
	tests := []struct {
		name string
		opts Options
		req  *http.Request
		peer string
		want int
	}{
		{
			name: "allowed login from loopback",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser),
			peer: loopbackPeer,
			want: http.StatusOK,
		},
		{
			name: "ipv6 loopback peer",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser),
			peer: "[::1]:50000",
			want: http.StatusOK,
		},
		{
			name: "login case differs",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, "Alice@EXAMPLE.com"),
			peer: loopbackPeer,
			want: http.StatusOK,
		},
		{
			name: "non-loopback peer",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser),
			peer: "100.64.0.5:50000",
			want: http.StatusUnauthorized,
		},
		{
			name: "unparseable peer",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser),
			peer: "not-an-address",
			want: http.StatusUnauthorized,
		},
		{
			name: "trusted proxy off",
			opts: proxyOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser),
			peer: loopbackPeer,
			want: http.StatusUnauthorized,
		},
		{
			name: "login not listed",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, "mallory@example.com"),
			peer: loopbackPeer,
			want: http.StatusUnauthorized,
		},
		{
			name: "empty login",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, ""),
			peer: loopbackPeer,
			want: http.StatusUnauthorized,
		},
		{
			name: "no login header",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost),
			peer: loopbackPeer,
			want: http.StatusUnauthorized,
		},
		{
			name: "repeated login header",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser, proxyUser),
			peer: loopbackPeer,
			want: http.StatusUnauthorized,
		},
		{
			name: "second login unlisted",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, proxyHost, proxyUser, "mallory@example.com"),
			peer: loopbackPeer,
			want: http.StatusUnauthorized,
		},
		{
			name: "host not allowed",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodGet, "evil.example.com", proxyUser),
			peer: loopbackPeer,
			want: http.StatusForbidden,
		},
		{
			name: "foreign origin still refused",
			opts: trustedOptions(),
			req:  withOrigin(identityRequest(http.MethodGet, proxyHost, proxyUser), "https://evil.example"),
			peer: loopbackPeer,
			want: http.StatusForbidden,
		},
		{
			name: "foreign fetch site still refused",
			opts: trustedOptions(),
			req:  withHeader(identityRequest(http.MethodGet, proxyHost, proxyUser), "Sec-Fetch-Site", "cross-site"),
			peer: loopbackPeer,
			want: http.StatusForbidden,
		},
		{
			name: "post needs origin",
			opts: trustedOptions(),
			req:  identityRequest(http.MethodPost, proxyHost, proxyUser),
			peer: loopbackPeer,
			want: http.StatusForbidden,
		},
		{
			name: "post needs json",
			opts: trustedOptions(),
			req:  withOrigin(identityRequest(http.MethodPost, proxyHost, proxyUser), "https://"+proxyHost),
			peer: loopbackPeer,
			want: http.StatusUnsupportedMediaType,
		},
		{
			name: "post with origin and json is admitted",
			opts: trustedOptions(),
			req: withHeader(
				withOrigin(identityRequest(http.MethodPost, proxyHost, proxyUser), "https://"+proxyHost),
				"Content-Type", "application/json"),
			peer: loopbackPeer,
			want: http.StatusMethodNotAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := New(tt.opts)
			require.NoError(t, err)
			rec := serve(t, s, tt.req, tt.peer)
			assert.Equal(t, tt.want, rec.Code)
			assert.Empty(
				t,
				rec.Result().Cookies(),
				"identity admission must not issue a session cookie",
			)
		})
	}
}

func TestTrustedProxyKeepsTokenFlow(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, trustedOptions())
	cookie := bootstrapSession(t, s)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
	require.NoError(t, err)
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestIdentityHeaderDoesNotBypassHostCheckOnLoopbackName(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, trustedOptions())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.BaseURL()+"/", nil)
	require.NoError(t, err)
	req.Header.Set(identityLoginHeader, proxyUser)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(
		t,
		http.StatusUnauthorized,
		resp.StatusCode,
		"a loopback Host is not an allowed host, so identity must not admit it",
	)
}

func TestRequestLogOmitsTokenAndLogin(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	opts := trustedOptions()
	opts.Logger = &logger
	s, err := New(opts)
	require.NoError(t, err)

	token := sessionToken(t, s)
	boot := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?token="+token, nil)
	boot.Host = proxyHost
	serve(t, s, boot, "127.0.0.1:50000")
	serve(t, s, identityRequest(http.MethodGet, proxyHost, proxyUser), "127.0.0.1:50000")

	logged := buf.String()
	require.NotEmpty(t, logged, "requests must be logged")
	assert.NotContains(t, logged, token)
	assert.NotContains(t, logged, proxyUser)
}

func TestNewRejectsInvalidProxyOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		opts    Options
		wantErr error
	}{
		{name: "wildcard host", opts: Options{AllowedHosts: []string{"*.ts.net"}}, wantErr: ErrInvalidAllowedHost},
		{name: "ip host", opts: Options{AllowedHosts: []string{"10.0.0.1"}}, wantErr: ErrInvalidAllowedHost},
		{
			name:    "trusted without hosts",
			opts:    Options{TrustedProxy: true, AllowedUsers: []string{proxyUser}},
			wantErr: ErrInvalidTrustedProxy,
		},
		{
			name:    "trusted without users",
			opts:    Options{TrustedProxy: true, AllowedHosts: []string{proxyHost}},
			wantErr: ErrInvalidTrustedProxy,
		},
		{
			name:    "blank user",
			opts:    Options{TrustedProxy: true, AllowedHosts: []string{proxyHost}, AllowedUsers: []string{" "}},
			wantErr: ErrInvalidTrustedProxy,
		},
		{
			name:    "users without trusted proxy",
			opts:    Options{AllowedHosts: []string{proxyHost}, AllowedUsers: []string{proxyUser}},
			wantErr: ErrInvalidTrustedProxy,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := New(tt.opts)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, s)
		})
	}
}

func withOrigin(req *http.Request, origin string) *http.Request {
	return withHeader(req, "Origin", origin)
}

func withHeader(req *http.Request, key, value string) *http.Request {
	req.Header.Set(key, value)
	return req
}

func portString(s *Server) string {
	return strconv.Itoa(s.Port())
}
