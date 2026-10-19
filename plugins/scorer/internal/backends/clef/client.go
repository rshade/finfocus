// Package clef adapts the shared System One client to Cloudflare Workers AI.
package clef

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rshade/go-decide/clefclient"

	"github.com/rshade/finfocus/plugins/scorer/internal/backends/jev"
)

// New builds a client using the released go-decide transport. BaseURL, when
// supplied, is an origin rather than a System One or Workers AI run path.
// Redirects are refused; the shared client retains timeout and retry handling.
func New(cfg jev.Config, account string) (*jev.Client, error) {
	base, err := endpoint(cfg.BaseURL, account)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	if cfg.HTTPClient != nil {
		*client = *cfg.HTTPClient
	}
	client.Transport = clefclient.NewTransport(client.Transport)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cfg.BaseURL = base
	cfg.HTTPClient = client
	return jev.New(cfg)
}

func endpoint(rawOrigin, account string) (string, error) {
	base, err := clefclient.BaseURL(account)
	if err != nil {
		return "", fmt.Errorf("configure Clef account: %w", err)
	}
	if rawOrigin == "" {
		return base, nil
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" ||
		origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return "", errors.New("clef: base URL must be an origin without credentials, path, query or fragment")
	}
	return strings.TrimRight(origin.String(), "/") + strings.TrimPrefix(base, "https://api.cloudflare.com"), nil
}
