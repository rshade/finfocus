package prometheus

import (
	"errors"
	"net/url"
	"os"
)

const (
	// inClusterPrometheusURL is the Prometheus Operator service. Any other chart
	// sets FINFOCUS_PROMETHEUS_URL.
	inClusterPrometheusURL = "http://prometheus-operated.monitoring.svc:9090"

	// missingPrometheusURL is the error text when no address can be resolved.
	// The bearer token is never part of this string.
	missingPrometheusURL = "set FINFOCUS_PROMETHEUS_URL to the Prometheus base URL"
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
	token := os.Getenv("FINFOCUS_PROMETHEUS_BEARER_TOKEN")
	if url := os.Getenv("FINFOCUS_PROMETHEUS_URL"); url != "" {
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
