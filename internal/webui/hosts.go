package webui

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// defaultHTTPSPort is the port a proxy serves HTTPS on when the Host header
// and the Origin carry none.
const defaultHTTPSPort = "443"

// maxLabelLength is the longest DNS label.
const maxLabelLength = 63

var (
	// ErrInvalidAllowedHost marks an --allowed-host value that is not a plain
	// DNS name with an optional port.
	ErrInvalidAllowedHost = errors.New("invalid allowed host")
	// ErrInvalidTrustedProxy marks a trusted-proxy configuration that lacks
	// its required allowed hosts or allowed users.
	ErrInvalidTrustedProxy = errors.New("invalid trusted proxy configuration")
)

// allowedHost is one normalized public name the server accepts. An empty port
// means the HTTPS default.
type allowedHost struct {
	name string
	port string
}

// normalizeAllowedHosts lowercases, trims, validates, and de-duplicates raw
// --allowed-host values. Matching is exact, so wildcards, IP literals, and
// anything that is not a plain host name with an optional port are refused.
func normalizeAllowedHosts(raw []string) ([]allowedHost, error) {
	var hosts []allowedHost
	seen := make(map[allowedHost]bool, len(raw))
	for _, value := range raw {
		host, err := parseAllowedHost(value)
		if err != nil {
			return nil, err
		}
		if !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	return hosts, nil
}

func parseAllowedHost(value string) (allowedHost, error) {
	trimmed := strings.TrimSpace(value)
	name, port, hasPort := trimmed, "", false
	switch strings.Count(trimmed, ":") {
	case 0:
	case 1:
		name, port, _ = strings.Cut(trimmed, ":")
		hasPort = true
	default:
		return allowedHost{}, fmt.Errorf("%w: %q is not a host name: %w", ErrInvalidAllowedHost, value, errIPLiteral)
	}
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if err := validateHostName(name); err != nil {
		return allowedHost{}, fmt.Errorf("%w: %q: %w", ErrInvalidAllowedHost, value, err)
	}
	if !hasPort {
		return allowedHost{name: name}, nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > maxPort || port != strconv.Itoa(number) {
		return allowedHost{}, fmt.Errorf("%w: %q has an invalid port", ErrInvalidAllowedHost, value)
	}
	if port == defaultHTTPSPort {
		port = ""
	}
	return allowedHost{name: name, port: port}, nil
}

var errIPLiteral = errors.New("IP literals are not allowed")

// validateHostName accepts dot-separated DNS labels of letters, digits, and
// inner hyphens. A name whose last label starts with a digit (127.1, 0x7f.0x1)
// reads as an IPv4 address in some resolvers, and no top-level domain starts
// with a digit, so it is refused with the IP literals.
func validateHostName(name string) error {
	if name == "" {
		return errors.New("host is empty")
	}
	if net.ParseIP(name) != nil {
		return errIPLiteral
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if err := validateLabel(label); err != nil {
			return err
		}
	}
	if last := labels[len(labels)-1]; last[0] >= '0' && last[0] <= '9' {
		return errIPLiteral
	}
	return nil
}

func validateLabel(label string) error {
	if label == "" || len(label) > maxLabelLength {
		return errors.New("host has an empty or oversized label")
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return errors.New("host label starts or ends with a hyphen")
	}
	for i := range len(label) {
		c := label[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return errors.New("host contains a character other than letters, digits, and hyphens")
		}
	}
	return nil
}

// normalizeAllowedUsers trims, lowercases, and de-duplicates --allowed-user
// logins. A blank login is refused.
func normalizeAllowedUsers(raw []string) ([]string, error) {
	var users []string
	seen := make(map[string]bool, len(raw))
	for _, value := range raw {
		login := strings.ToLower(strings.TrimSpace(value))
		if login == "" {
			return nil, fmt.Errorf("%w: an allowed user must not be blank", ErrInvalidTrustedProxy)
		}
		if !seen[login] {
			seen[login] = true
			users = append(users, login)
		}
	}
	return users, nil
}

// matchAllowedHost reports whether a request Host header names one of hosts.
// A host without a port matches a request with no port or the HTTPS default;
// a host with a port matches that port only.
func matchAllowedHost(hosts []allowedHost, requestHost string) bool {
	name, port := requestHost, ""
	if strings.Count(requestHost, ":") == 1 {
		name, port, _ = strings.Cut(requestHost, ":")
	}
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if port == defaultHTTPSPort {
		port = ""
	}
	for _, host := range hosts {
		if host.name == name && host.port == port {
			return true
		}
	}
	return false
}

// matchAllowedOrigin reports whether origin is the canonical HTTPS origin of
// one of hosts. A browser omits the default port, so only that form matches.
func matchAllowedOrigin(hosts []allowedHost, origin string) bool {
	for _, host := range hosts {
		if origin == host.origin() {
			return true
		}
	}
	return false
}

// origin is the canonical HTTPS origin of the host.
func (h allowedHost) origin() string {
	if h.port == "" {
		return "https://" + h.name
	}
	return "https://" + h.name + ":" + h.port
}

// ValidateAccess checks the proxy-related options the way New does, without
// building a server, so a caller can refuse a bad combination before it binds
// a port. Errors wrap ErrInvalidAllowedHost or ErrInvalidTrustedProxy.
func ValidateAccess(allowedHosts []string, trustedProxy bool, allowedUsers []string) error {
	_, _, err := normalizeAccess(allowedHosts, trustedProxy, allowedUsers)
	return err
}

// normalizeAccess normalizes and validates the proxy options as one unit.
func normalizeAccess(
	allowedHosts []string, trustedProxy bool, allowedUsers []string,
) ([]allowedHost, []string, error) {
	hosts, err := normalizeAllowedHosts(allowedHosts)
	if err != nil {
		return nil, nil, err
	}
	users, err := normalizeAllowedUsers(allowedUsers)
	if err != nil {
		return nil, nil, err
	}
	if err = validateTrustedProxy(trustedProxy, hosts, users); err != nil {
		return nil, nil, err
	}
	return hosts, users, nil
}

// ProxyURL is the token-free HTTPS URL of the first allowed host, or "" when
// none is configured. Trusted-proxy mode prints it because identity, not the
// token, admits that URL.
func (s *Server) ProxyURL() string {
	if len(s.allowedHosts) == 0 {
		return ""
	}
	return s.allowedHosts[0].origin() + "/"
}

// validateTrustedProxy enforces the trusted-proxy requirements: allowed users
// only make sense with it, and it needs both an allowed host and a user.
func validateTrustedProxy(trusted bool, hosts []allowedHost, users []string) error {
	switch {
	case !trusted && len(users) > 0:
		return fmt.Errorf("%w: allowed users require trusted proxy mode", ErrInvalidTrustedProxy)
	case !trusted:
		return nil
	case len(hosts) == 0:
		return fmt.Errorf("%w: trusted proxy mode requires at least one allowed host", ErrInvalidTrustedProxy)
	case len(users) == 0:
		return fmt.Errorf("%w: trusted proxy mode requires at least one allowed user", ErrInvalidTrustedProxy)
	}
	return nil
}

// identityLoginHeader is the Tailscale Serve header that names the tailnet
// user behind a request. Serve strips client-supplied copies.
const identityLoginHeader = "Tailscale-User-Login"

// identityAuthenticated reports whether r is admitted by proxy identity
// instead of the session token: trusted-proxy mode is on, the TCP peer is
// loopback, the Host is an allowed host, and the login header appears exactly
// once and names an allowed user.
func (s *Server) identityAuthenticated(r *http.Request) bool {
	if !s.trustedProxy || !loopbackPeer(r.RemoteAddr) || !matchAllowedHost(s.allowedHosts, r.Host) {
		return false
	}
	logins := r.Header.Values(identityLoginHeader)
	if len(logins) != 1 {
		return false
	}
	login := strings.ToLower(strings.TrimSpace(logins[0]))
	return login != "" && slices.Contains(s.allowedUsers, login)
}

// loopbackPeer reports whether a RemoteAddr is a loopback IP address.
func loopbackPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// forwardedHTTPS reports whether the proxy says the client used HTTPS. The
// header is client-controllable, so it may only tighten the cookie, and it is
// read only when an allowed host is configured: with none, the cookie stays
// exactly as it was before proxy support.
func (s *Server) forwardedHTTPS(r *http.Request) bool {
	if len(s.allowedHosts) == 0 {
		return false
	}
	first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(first), "https")
}
