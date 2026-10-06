// Package webui implements the localhost-only HTTP layer for `finfocus
// --web`. It is a thin transport: all cost, filter, sort, aggregation, and
// estimation behavior lives in internal/engine, internal/viewmodel, and
// internal/cli and is consumed, never reimplemented, here (FR-011a/b).
package webui

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus/internal/logging"
)

const (
	// loopbackAddr is the only address the server ever binds (FR-016a).
	loopbackAddr = "127.0.0.1"
	// sessionCookiePrefix prefixes the per-port session cookie name. The port
	// is part of the name because cookies are not isolated by port: a second
	// `finfocus --web` on another port would otherwise overwrite this cookie.
	sessionCookiePrefix = "finfocus_session_"
	// tokenBytes is the session token entropy: 128 bits, hex-encoded.
	tokenBytes = 16
	// shutdownTimeout bounds the graceful shutdown after interrupt.
	shutdownTimeout = 5 * time.Second
	// readHeaderTimeout bounds how long a client may take to send headers.
	readHeaderTimeout = 10 * time.Second
	// maxPort is the largest valid TCP port.
	maxPort = 65535
)

// Options configures a Server.
type Options struct {
	// Port pins the listen port. Zero (the default) lets the kernel assign an
	// ephemeral port; the actual port is reported via Port and URL.
	Port int
	// Logger receives request logs. When nil, a logger is taken from the
	// background context (stderr, info level).
	Logger *zerolog.Logger
}

// Server is the localhost-only web UI HTTP server. A per-session token,
// generated at construction, bootstraps an HttpOnly session cookie; every
// other request authenticates by that cookie and a matching Host header.
type Server struct {
	token  string
	logger zerolog.Logger

	reqPort    int
	actualPort atomic.Int32

	listener   net.Listener
	httpServer *http.Server
}

// New creates a Server with a fresh 128-bit crypto/rand session token.
func New(opts Options) (*Server, error) {
	if opts.Port < 0 || opts.Port > maxPort {
		return nil, fmt.Errorf("webui: port %d out of range (0-%d): %w", opts.Port, maxPort, ErrInvalidPort)
	}
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("webui: generate session token: %w", err)
	}
	s := &Server{
		token:   hex.EncodeToString(raw),
		reqPort: opts.Port,
	}
	if opts.Logger != nil {
		s.logger = *opts.Logger
	} else {
		s.logger = *logging.FromContext(context.Background())
	}
	s.httpServer = &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	return s, nil
}

// ErrInvalidPort marks an out-of-range requested port.
var ErrInvalidPort = errors.New("invalid port")

// Listen binds 127.0.0.1 on the requested port (or a kernel-assigned port
// when 0) and resolves the actual port. A busy pinned port fails with an
// error naming the port.
func (s *Server) Listen() error {
	if s.listener != nil {
		return errors.New("webui: server is already listening")
	}
	listener, err := (&net.ListenConfig{}).Listen(
		context.Background(), "tcp", net.JoinHostPort(loopbackAddr, strconv.Itoa(s.reqPort)),
	)
	if err != nil {
		return fmt.Errorf("webui: port %d on %s is unavailable: %w", s.reqPort, loopbackAddr, err)
	}
	s.listener = listener
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return fmt.Errorf("webui: listener address %s is not TCP", listener.Addr())
	}
	//nolint:gosec // G115: a bound TCP port is 0-65535, the int32 conversion cannot overflow.
	s.actualPort.Store(int32(addr.Port))
	return nil
}

// Serve accepts connections until Shutdown. It must be called after Listen.
func (s *Server) Serve() error {
	if s.listener == nil {
		return errors.New("webui: Listen must be called before Serve")
	}
	return s.httpServer.Serve(s.listener)
}

// Shutdown gracefully stops the server, waiting for in-flight requests until
// ctx expires. Keep-alives are disabled first so idle browser connections do
// not outlive the drain. If the graceful drain does not finish in time, the
// remaining connections (including freshly accepted ones a client never used)
// are force-closed: the server must always stop on interrupt (FR-015).
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.httpServer.SetKeepAlivesEnabled(false)
	err := s.httpServer.Shutdown(ctx)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if cerr := s.httpServer.Close(); cerr != nil {
		return fmt.Errorf("webui: shutdown: %w", errors.Join(err, cerr))
	}
	s.logger.Warn().
		Str("component", "webui").
		Err(err).
		Msg("graceful shutdown timed out; remaining connections force-closed")
	return nil
}

// Run listens, serves, and blocks until ctx is canceled or an interrupt
// signal arrives, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	if err := s.Listen(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		err := s.Serve()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}

// Port returns the actual bound port. It is 0 until Listen has run.
func (s *Server) Port() int {
	return int(s.actualPort.Load())
}

// Addr returns the bound listener address, or nil before Listen.
func (s *Server) Addr() net.Addr {
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

// BaseURL is the server's origin, e.g. http://127.0.0.1:8080.
func (s *Server) BaseURL() string {
	return "http://" + net.JoinHostPort(loopbackAddr, strconv.Itoa(s.Port()))
}

// URL is the bootstrap URL to print and open in the browser: the base URL
// with the session token in the query string (FR-002, FR-016).
func (s *Server) URL() string {
	return s.BaseURL() + "/?token=" + s.token
}

// cookieName returns the per-port session cookie name.
func (s *Server) cookieName() string {
	return sessionCookiePrefix + strconv.Itoa(s.Port())
}

// handler builds the route tree behind the hardening middleware. Order:
// security headers (every response) → Host check → request log → session
// auth → Origin/Content-Type checks → routes.
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticFiles())))

	return s.securityHeaders(
		s.hostCheck(
			s.requestLog(
				s.authenticate(
					s.requestHardening(mux),
				),
			),
		),
	)
}

// securityHeaders sets the contract response headers on every response,
// including rejections: no caching, no referrer, and a self-only CSP (the SPA
// loads no inline script and nothing from a CDN).
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// hostCheck rejects requests whose Host header is not exactly this server's
// 127.0.0.1:<port> or localhost:<port> (DNS-rebinding defense).
func (s *Server) hostCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port := strconv.Itoa(s.Port())
		switch r.Host {
		case net.JoinHostPort(loopbackAddr, port), net.JoinHostPort("localhost", port):
			next.ServeHTTP(w, r)
		default:
			s.writeError(w, http.StatusForbidden, "forbidden", "host header is not this server")
		}
	})
}

// statusRecorder captures the response status for request logging.
type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap exposes the underlying ResponseWriter so [http.ResponseController]
// (and middleware) can reach optional interfaces such as [http.Flusher].
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// requestLog records method, path without query string, and status. The
// session token lives in the query string and request bodies carry
// passphrases and estimate overrides, so neither is ever logged (FR-016).
func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info().
			Ctx(r.Context()).
			Str("component", "webui").
			Str("operation", "http_request").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rec.status).
			Msg("request")
	})
}

// authenticate guards every route with the per-port session cookie. The one
// exception is the token bootstrap: GET / with a ?token= query, which
// handleRoot validates. The token stays valid for the server's lifetime so a
// second tab or a reload with the printed URL works.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" && r.URL.Query().Has("token") {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(s.cookieName())
		if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.token)) != 1 {
			s.writeError(w, http.StatusUnauthorized, "unauthorized", "valid session cookie required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requestHardening rejects state-changing requests that do not come from the
// SPA itself. SameSite=Strict does not separate two localhost ports, so every
// state-changing method requires a same-origin Origin header and a JSON
// Content-Type; a GET that carries an Origin header must match too.
func (s *Server) requestHardening(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !s.isOwnOrigin(origin) {
			s.writeError(w, http.StatusForbidden, "forbidden", "origin is not this server")
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Read-only methods need no further checks.
		default:
			if origin == "" {
				s.writeError(w, http.StatusForbidden, "forbidden", "origin header required")
				return
			}
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				s.writeError(w, http.StatusUnsupportedMediaType,
					"unsupported_media_type", "Content-Type must be application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isOwnOrigin reports whether origin is this server's own origin on either
// loopback hostname.
func (s *Server) isOwnOrigin(origin string) bool {
	port := strconv.Itoa(s.Port())
	return origin == "http://"+net.JoinHostPort(loopbackAddr, port) ||
		origin == "http://"+net.JoinHostPort("localhost", port)
}

// handleRoot serves the SPA shell. With a ?token= query it validates the
// token (constant-time) and answers 303 to / with the session cookie set, so
// the token leaves the address bar and browser history. Without a query it
// serves index.html to the cookie-authenticated session.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.writeError(w, http.StatusNotFound, "not_found", "unknown path")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	if r.URL.Query().Has("token") {
		candidate := r.URL.Query().Get("token")
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(s.token)) != 1 {
			s.writeError(w, http.StatusUnauthorized, "unauthorized", "invalid session token")
			return
		}
		//nolint:gosec // G124: no Secure attribute by design — the server is plaintext
		// HTTP on loopback only, so Secure would break the session (http-api.md).
		http.SetCookie(w, &http.Cookie{
			Name:     s.cookieName(),
			Value:    s.token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	data, err := fs.ReadFile(staticFiles(), "index.html")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal", "SPA shell unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(data))
}

// apiError is the contract error shape: { error, code }.
type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError answers with the contract JSON error shape.
func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(apiError{Error: message, Code: code}); err != nil {
		s.logger.Debug().Str("component", "webui").Err(err).Msg("encode error response")
	}
}
