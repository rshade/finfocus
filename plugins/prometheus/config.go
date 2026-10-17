package prometheus

import (
	"errors"
	"net/url"
	"os"
)

const (
	// EnvURL is the Prometheus base URL. Inside a cluster, an unset value
	// falls back to the Operator service. That fallback depends on this
	// process, so it is not a setting default.
	EnvURL = "FINFOCUS_PROMETHEUS_URL"
	// EnvBearerToken is the bearer credential for EnvURL. It is never logged.
	EnvBearerToken = "FINFOCUS_PROMETHEUS_BEARER_TOKEN" //nolint:gosec // The name of an environment variable, not a credential.

	// inClusterPrometheusURL is the Prometheus Operator service. Any other chart
	// sets EnvURL.
	inClusterPrometheusURL = "http://prometheus-operated.monitoring.svc:9090"

	// missingPrometheusURL is the error text when no address can be resolved.
	// The bearer token is never part of this string.
	missingPrometheusURL = "set " + EnvURL + " to the Prometheus base URL"
)

// Config is the Prometheus address the plugin queries.
// Token is sent as a bearer credential and is never written to logs or errors.
type Config struct {
	URL   string
	Token string
}

// LoadConfig resolves the Prometheus address from the environment.
// FINFOCUS_PROMETHEUS_URL wins. Inside a cluster, the Operator service is
// the default. The bearer token is returned to the caller and is not
// included in the error.
func LoadConfig() (Config, error) {
	token := os.Getenv(EnvBearerToken)
	if url := os.Getenv(EnvURL); url != "" {
		return Config{URL: url, Token: token}, nil
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return Config{URL: inClusterPrometheusURL, Token: token}, nil
	}
	return Config{}, errors.New(missingPrometheusURL)
}

// RedactedURL is raw without userinfo or query, safe to log. A URL can carry
// credentials in either place.
func RedactedURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<unparseable URL>"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
