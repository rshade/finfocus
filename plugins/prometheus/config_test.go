package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configToken = "super-secret-token-value"

func TestLoadConfig(t *testing.T) {
	t.Run("url wins over the in-cluster default", func(t *testing.T) {
		t.Setenv("FINFOCUS_PROMETHEUS_URL", "http://prom.example:9090")
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		t.Setenv("FINFOCUS_PROMETHEUS_BEARER_TOKEN", configToken)

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, "http://prom.example:9090", cfg.URL)
		assert.Equal(t, configToken, cfg.Token)
	})

	t.Run("in-cluster default", func(t *testing.T) {
		t.Setenv("FINFOCUS_PROMETHEUS_URL", "")
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		t.Setenv("FINFOCUS_PROMETHEUS_BEARER_TOKEN", configToken)

		cfg, err := LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, "http://prometheus-operated.monitoring.svc:9090", cfg.URL)
		assert.Equal(t, configToken, cfg.Token)
	})

	t.Run("unset names the url variable", func(t *testing.T) {
		t.Setenv("FINFOCUS_PROMETHEUS_URL", "")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		t.Setenv("FINFOCUS_PROMETHEUS_BEARER_TOKEN", configToken)

		_, err := LoadConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FINFOCUS_PROMETHEUS_URL")
		assert.NotContains(t, err.Error(), configToken)
	})
}

func TestRedactedURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain", raw: "http://127.0.0.1:9090", want: "http://127.0.0.1:9090"},
		{name: "path kept", raw: "https://prom.example/prometheus", want: "https://prom.example/prometheus"},
		{name: "userinfo dropped", raw: "https://user:pass@prom.example", want: "https://prom.example"},
		{name: "query dropped", raw: "https://prom.example/?api_key=secret", want: "https://prom.example/"},
		{name: "empty", raw: "", want: ""},
		{name: "unparseable", raw: "http://[::1", want: "<unparseable URL>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RedactedURL(tt.raw))
		})
	}
}
