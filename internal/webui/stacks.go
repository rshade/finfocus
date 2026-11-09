package webui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// stackPathPrefix is the path under which a registered stack is served:
// /s/<stack>/<path> reaches that stack's handler tree as /<path>.
const stackPathPrefix = "/s/"

// maxStackNameLen is Pulumi's limit for a simple stack name.
const maxStackNameLen = 100

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
}

func validStackName(name string) bool {
	return len(name) <= maxStackNameLen && name != "." && name != ".." && stackNamePattern.MatchString(name)
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
	if !validStackName(name) {
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
	return nil
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
// bootstrap: the root, or the root of a registered stack.
func (s *Server) isBootstrapPath(path string) bool {
	if path == "/" {
		return true
	}
	name, rest, ok := splitStackPath(path)
	return ok && rest == "/" && s.stack(name) != nil
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
		http.StripPrefix(child.prefix, child.mux).ServeHTTP(w, r)
	})
}
