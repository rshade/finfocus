package pulumicloud

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rshade/finfocus/internal/esc"
)

// Reason describes why Cloud history is unavailable without exposing credentials.
type Reason string

const (
	// NotLoggedIn means no usable credential was discovered.
	NotLoggedIn Reason = "not_logged_in"
	// NonCloudBackend means a local or object-storage backend was selected.
	NonCloudBackend Reason = "non_cloud_backend"
	// InsecureBackend rejects cleartext authentication to non-loopback hosts.
	InsecureBackend Reason = "insecure_backend"
	// UnknownStack means a required stack identity component is missing.
	UnknownStack Reason = "unknown_stack"
	// TokenRejected means the service rejected the credential.
	TokenRejected Reason = "token_rejected"
	// NoPermission means the credential lacks service access.
	NoPermission Reason = "no_permission"
	// Unreachable means the service could not complete an availability request.
	Unreachable Reason = "unreachable"
)

// Availability describes whether the session can access Cloud history.
type Availability struct {
	Available bool
	Reason    Reason
	Session   esc.Session `json:"-"`
}

// Detect checks prerequisite reasons in order, then probes GET /api/user.
func Detect(ctx context.Context, session esc.Session, stack StackRef) Availability {
	return DetectWithClient(ctx, session, stack, nil)
}

// DetectWithClient allows callers to supply a transport with a private backend CA.
func DetectWithClient(ctx context.Context, session esc.Session, stack StackRef, httpClient *http.Client) Availability {
	unavailable := func(reason Reason) Availability { return Availability{Reason: reason} }
	if session.Token == "" {
		return unavailable(NotLoggedIn)
	}
	backend, err := url.Parse(session.Backend)
	if err != nil || backend.Host == "" || backend.User != nil || backend.RawQuery != "" || backend.Fragment != "" {
		return unavailable(NonCloudBackend)
	}
	scheme := strings.ToLower(backend.Scheme)
	if scheme != "https" && scheme != httpScheme {
		return unavailable(NonCloudBackend)
	}
	if scheme == httpScheme && !isLoopback(backend.Hostname()) {
		return unavailable(InsecureBackend)
	}
	if stack.Org == "" || stack.Project == "" || stack.Stack == "" {
		return unavailable(UnknownStack)
	}
	if probeErr := newRestClient(session, httpClient).probe(ctx); probeErr != nil {
		switch {
		case errors.Is(probeErr, ErrUnauthorized):
			return unavailable(TokenRejected)
		case errors.Is(probeErr, ErrForbidden):
			return unavailable(NoPermission)
		default:
			return unavailable(Unreachable)
		}
	}
	return Availability{Available: true, Session: session}
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
