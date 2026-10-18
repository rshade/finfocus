package scorer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromEnv(env(nil))
	require.NoError(t, err)
	assert.Empty(t, cfg.APIKey)
	assert.Equal(t, "https://api.typesafe.ai", cfg.BaseURL)
	assert.Equal(t, "jev-1.13.0", cfg.Model)
	assert.NotEqual(t, "jev-latest", cfg.Model, "default model must be a pinned version")
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.InDelta(t, 0.5, cfg.DuplicateThreshold, 1e-9)
	assert.Equal(t, 25, cfg.BatchSize)
}

func TestConfigFromEnv_Overrides(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromEnv(env(map[string]string{
		"SCORER_API_KEY":             "  key-value  ",
		"SCORER_BASE_URL":            "https://proxy.example.com",
		"SCORER_MODEL":               "jev-9.9.9",
		"SCORER_TIMEOUT":             "15s",
		"SCORER_DUPLICATE_THRESHOLD": "0.7",
		"SCORER_BATCH_SIZE":          "5",
	}))
	require.NoError(t, err)
	assert.Equal(t, "key-value", cfg.APIKey)
	assert.Equal(t, "https://proxy.example.com", cfg.BaseURL)
	assert.Equal(t, "jev-9.9.9", cfg.Model)
	assert.Equal(t, 15*time.Second, cfg.Timeout)
	assert.InDelta(t, 0.7, cfg.DuplicateThreshold, 1e-9)
	assert.Equal(t, 5, cfg.BatchSize)
}

func TestConfigFromEnv_RejectsBadValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		val  string
	}{
		{"timeout not a duration", "SCORER_TIMEOUT", "soon"},
		{"timeout negative", "SCORER_TIMEOUT", "-5s"},
		{"batch size zero", "SCORER_BATCH_SIZE", "0"},
		{"batch size too large", "SCORER_BATCH_SIZE", "500"},
		{"batch size not a number", "SCORER_BATCH_SIZE", "many"},
		{"threshold not a number", "SCORER_DUPLICATE_THRESHOLD", "high"},
		{"threshold zero", "SCORER_DUPLICATE_THRESHOLD", "0"},
		{"threshold above one", "SCORER_DUPLICATE_THRESHOLD", "1.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ConfigFromEnv(env(map[string]string{tt.key: tt.val}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.key)
		})
	}
}

func TestConfigFromEnv_BackendSelection(t *testing.T) {
	t.Parallel()
	cfg, err := ConfigFromEnv(env(map[string]string{
		"SCORER_BACKEND": "clef", "SCORER_API_KEY": "cf-dummy",
		"SCORER_CLEF_ACCOUNT_ID": "0123456789abcdef0123456789abcdef",
	}))
	require.NoError(t, err)
	assert.Equal(t, "clef", cfg.Model)
	assert.Equal(t, "cf-dummy", cfg.APIKey)
}

func TestConfigFromEnv_RejectsInvalidBackendAndModel(t *testing.T) {
	t.Parallel()
	for _, values := range []map[string]string{
		{"SCORER_BACKEND": "unknown"},
		{"SCORER_BACKEND": "clef", "SCORER_MODEL": "jev-1.13.0"},
		{"SCORER_DUPLICATE_THRESHOLD": "NaN"},
	} {
		_, err := ConfigFromEnv(env(values))
		require.Error(t, err)
	}
}
