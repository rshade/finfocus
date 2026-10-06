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
