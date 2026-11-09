package webui

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
)

// stackPathPrefix is the path under which a registered stack is served:
// /s/<stack>/<path> reaches that stack's handler tree as /<path>.
const stackPathPrefix = "/s/"

// MaxStackNameLen is Pulumi's limit for a simple stack name.
const MaxStackNameLen = 100

// MinTabbedStacks is the fewest stacks that make a tabbed page. With one, the
// dashboard is the plain single-stack page and --stack takes the single-stack
// path.
const MinTabbedStacks = 2

var (
	// ErrInvalidStackName marks a stack name that is not a single safe URL
	// path segment.
	ErrInvalidStackName = errors.New("invalid stack name")
	// ErrDuplicateStack marks a stack name that is already registered.
	ErrDuplicateStack = errors.New("stack already registered")

	stackNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

// StackOptions configures one stack registered with AddStack.
type StackOptions struct {
	// Session is the stack's own engine and overview state. Nil creates an
	// empty session.
	Session *Session
	// Logs overrides the log source for this stack. Nil shares the server's.
	Logs LogSource
	// Activate starts the stack's work the first time a client activates it.
	// The server calls it at most once, however many activations arrive.
	// Nil means there is nothing to start.
	Activate func()
}

// StackState is the load state of a registered stack.
type StackState string

// The stack load states reported by GET /api/stacks.
const (
	// StackStateIdle means the stack has not been activated.
	StackStateIdle StackState = "idle"
	// StackStateLoading means the stack is activated and still loading.
	StackStateLoading StackState = "loading"
	// StackStateLocked means a passphrase prompt is pending.
	StackStateLocked StackState = "locked"
	// StackStateReady means the stack's overview finished.
	StackStateReady StackState = "ready"
	// StackStateFailed means the stack's pipeline ended with a failure.
	StackStateFailed StackState = "failed"
)

// StackEntry is one registered stack and its load state.
type StackEntry struct {
	Name  string     `json:"name"`
	State StackState `json:"state"`
}

// ValidStackName reports whether name is one safe URL path segment of
// [A-Za-z0-9_.-], at most 100 characters, and so can be registered with
// AddStack.
func ValidStackName(name string) bool {
	return len(name) <= MaxStackNameLen && name != "." && name != ".." && stackNamePattern.MatchString(name)
}

// AddStack serves a stack under /s/<name>/. The stack gets its own handler
// tree, session, and plugin operations, and sits behind this server's single
// outer layer: Host check, authentication, Origin hardening, and logging run
// once, before dispatch. The session cookie is one global cookie for the
// listen port (Path=/), so a browser signed in once reaches every stack.
//
// The name must be one path segment of [A-Za-z0-9_.-], at most 100
// characters, so it cannot alter the route or inject into the page. A name
// that is already registered, or a call after Shutdown, is an error.
func (s *Server) AddStack(name string, opts StackOptions) error {
	if !ValidStackName(name) {
		return fmt.Errorf("webui: stack name %q: %w", name, ErrInvalidStackName)
	}
	session := opts.Session
	if session == nil {
		session = NewSession(context.Background(), SessionOptions{})
	}
	logs := s.logs
	if opts.Logs != nil && !isNilLogSource(opts.Logs) {
		logs = opts.Logs
	}
	child := &Server{
		session:      session,
		token:        s.token,
		logger:       s.logger,
		logs:         logs,
		allowedHosts: s.allowedHosts,
		trustedProxy: s.trustedProxy,
		allowedUsers: s.allowedUsers,
		parent:       s,
		prefix:       strings.TrimSuffix(stackPathPrefix, "/") + "/" + name,
		activate:     opts.Activate,
	}
	child.mux = child.buildMux()

	s.stacksMu.Lock()
	defer s.stacksMu.Unlock()
	if s.stacksClosed {
		return http.ErrServerClosed
	}
	if _, exists := s.stacks[name]; exists {
		return fmt.Errorf("webui: stack %q: %w", name, ErrDuplicateStack)
	}
	if s.stacks == nil {
		s.stacks = map[string]*Server{}
	}
	s.stacks[name] = child
	s.stackOrder = append(s.stackOrder, name)
	return nil
}

// stackEntries lists the registered stacks in registration order with their
// load states.
func (s *Server) stackEntries() []StackEntry {
	s.stacksMu.RLock()
	defer s.stacksMu.RUnlock()
	entries := make([]StackEntry, 0, len(s.stackOrder))
	for _, name := range s.stackOrder {
		entries = append(entries, StackEntry{Name: name, State: stackState(s.stacks[name])})
	}
	return entries
}

// stacksMeta is the shell's stack list for the browser: a meta tag naming the
// registered stacks in order, or nothing. Only the root page of a server with
// two or more stacks carries it, so one stack, and any page under /s/<name>/,
// is the plain single-stack page. The names are already restricted to
// [A-Za-z0-9_.-]; they are escaped as well. The browser reads this at boot, so
// the shell never waits on a request to decide whether to show tabs.
func (s *Server) stacksMeta() string {
	if s.parent != nil {
		return ""
	}
	s.stacksMu.RLock()
	defer s.stacksMu.RUnlock()
	if len(s.stackOrder) < MinTabbedStacks {
		return ""
	}
	return `<meta name="finfocus-stacks" content="` + html.EscapeString(strings.Join(s.stackOrder, ",")) + `">`
}

// activateStack runs the stack's Activate callback on the first call only and
// reports the stack's state. It is false for a name that is not registered,
// including one that is not a valid path segment.
func (s *Server) activateStack(name string) (StackEntry, bool) {
	if !ValidStackName(name) {
		return StackEntry{}, false
	}
	child := s.stack(name)
	if child == nil {
		return StackEntry{}, false
	}
	if child.activated.CompareAndSwap(false, true) && child.activate != nil {
		child.activate()
	}
	return StackEntry{Name: name, State: stackState(child)}, true
}

// stackState derives a stack's load state. A pending prompt wins over a
// recorded failure because a rejected passphrase records one and re-prompts,
// and a stack that finished loading stays ready.
func stackState(child *Server) StackState {
	if !child.activated.Load() {
		return StackStateIdle
	}
	session := child.session
	switch {
	case session.PassphraseRequired():
		return StackStateLocked
	case session.IsReady():
		// A stack that loaded stays ready: a failed on-demand preview records a
		// failure on a pipeline that has already ended, and the stack still works.
		return StackStateReady
	case session.Failed():
		return StackStateFailed
	default:
		return StackStateLoading
	}
}

func (s *Server) stack(name string) *Server {
	s.stacksMu.RLock()
	defer s.stacksMu.RUnlock()
	return s.stacks[name]
}

// closeStacks refuses further registrations and releases every stack.
func (s *Server) closeStacks() {
	s.stacksMu.Lock()
	s.stacksClosed = true
	children := make([]*Server, 0, len(s.stacks))
	for _, child := range s.stacks {
		children = append(children, child)
	}
	s.stacksMu.Unlock()
	for _, child := range children {
		child.dropPluginOps()
		child.session.Close()
	}
}

// splitStackPath cuts /s/<name>/<rest> into the stack name and the path that
// follows it (with its leading slash, or empty when there is none).
func splitStackPath(path string) (string, string, bool) {
	tail, found := strings.CutPrefix(path, stackPathPrefix)
	if !found {
		return "", "", false
	}
	name, rest, hasRest := strings.Cut(tail, "/")
	if hasRest {
		rest = "/" + rest
	}
	return name, rest, true
}

// stackRoutePath is path with any /s/<name> prefix removed, so route-specific
// rules see the same path under a stack as at the root.
func stackRoutePath(path string) string {
	_, rest, ok := splitStackPath(path)
	if !ok {
		return path
	}
	if rest == "" {
		return "/"
	}
	return rest
}

// isBootstrapPath reports whether path is a page that can take the ?token=
// bootstrap: the root, or the root of any stack name. The registry is not
// consulted, so an invalid token gets the same answer for a registered and an
// unregistered stack.
func (s *Server) isBootstrapPath(path string) bool {
	if path == "/" {
		return true
	}
	name, rest, ok := splitStackPath(path)
	return ok && rest == "/" && ValidStackName(name)
}

// dispatch sends /s/<name>/... to that stack's handler tree with the prefix
// stripped, and everything else to root. It runs after authentication and
// hardening, so an unregistered stack is a 404 only for an authenticated
// client.
func (s *Server) dispatch(root http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, rest, ok := splitStackPath(r.URL.Path)
		if !ok {
			root.ServeHTTP(w, r)
			return
		}
		child := s.stack(name)
		if child == nil {
			if rest == "/" && r.URL.Query().Has("token") && !s.validToken(r.URL.Query().Get("token")) {
				s.rejectInvalidToken(w)
				return
			}
			s.writeError(w, http.StatusNotFound, "not_found", "unknown stack")
			return
		}
		if rest == "" {
			target := child.prefix + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			//nolint:gosec // G710: target is a registered stack prefix plus the request's own query; the host never changes.
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}
		scoped := &prefixRedirects{ResponseWriter: w, prefix: child.prefix}
		http.StripPrefix(child.prefix, child.mux).ServeHTTP(scoped, r)
	})
}

// prefixRedirects puts the stack prefix back on root-relative redirects. The
// child mux sees the stripped path, so the redirects it generates itself (path
// cleaning, a subtree pattern without its trailing slash) would otherwise
// leave the stack and land on the root session.
type prefixRedirects struct {
	http.ResponseWriter

	prefix string
}

// WriteHeader rewrites a root-relative Location before forwarding the status.
func (w *prefixRedirects) WriteHeader(status int) {
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest {
		loc := w.Header().Get("Location")
		if strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") {
			w.Header().Set("Location", w.prefix+loc)
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap exposes the underlying ResponseWriter so [http.ResponseController]
// can reach optional interfaces such as [http.Flusher].
func (w *prefixRedirects) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
