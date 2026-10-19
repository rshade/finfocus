package scorer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(values map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { value, ok := values[k]; return value, ok }
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

func TestConfigFromEnv_FirstLaunchSettings(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"FINFOCUS_PLUGIN_SCORER_BACKEND":             "clef",
		"FINFOCUS_PLUGIN_SCORER_CLEF_ACCOUNT_ID":     "0123456789abcdef0123456789abcdef",
		"FINFOCUS_PLUGIN_SCORER_API_KEY":             "file-token",
		"FINFOCUS_PLUGIN_SCORER_BASE_URL":            "https://proxy.example.com",
		"FINFOCUS_PLUGIN_SCORER_MODEL":               "clef",
		"FINFOCUS_PLUGIN_SCORER_TIMEOUT":             "15s",
		"FINFOCUS_PLUGIN_SCORER_BATCH_SIZE":          "5",
		"FINFOCUS_PLUGIN_SCORER_DUPLICATE_THRESHOLD": "0.7",
	}
	cfg, err := ConfigFromEnv(env(values))
	require.NoError(t, err)
	assert.Equal(t, "clef", cfg.Backend)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", cfg.ClefAccountID)
	assert.Equal(t, "file-token", cfg.APIKey)
	assert.Equal(t, "https://proxy.example.com", cfg.BaseURL)
	assert.Equal(t, "clef", cfg.Model)
	assert.Equal(t, 15*time.Second, cfg.Timeout)
	assert.Equal(t, 5, cfg.BatchSize)
	assert.InDelta(t, 0.7, cfg.DuplicateThreshold, 1e-9)

	// Explicit backend variables take precedence over the first-launch names.
	values[EnvAPIKey] = "environment-token"
	values[EnvClefAccountID] = "abcdef0123456789abcdef0123456789"
	cfg, err = ConfigFromEnv(env(values))
	require.NoError(t, err)
	assert.Equal(t, "environment-token", cfg.APIKey)
	assert.Equal(t, "abcdef0123456789abcdef0123456789", cfg.ClefAccountID)
}

func TestConfigFromEnv_ExplicitEmptyOverridesFileSettings(t *testing.T) {
	t.Parallel()

	cfg, err := ConfigFromEnv(env(map[string]string{
		EnvAPIKey: "", "FINFOCUS_PLUGIN_SCORER_API_KEY": "file-token",
		EnvBackend: "", "FINFOCUS_PLUGIN_SCORER_BACKEND": "clef",
		EnvBaseURL: "", "FINFOCUS_PLUGIN_SCORER_BASE_URL": "https://proxy.example.com",
		EnvModel: "", "FINFOCUS_PLUGIN_SCORER_MODEL": "clef",
	}))
	require.NoError(t, err)
	assert.Empty(t, cfg.APIKey, "an explicitly cleared token must not activate scoring")
	assert.Equal(t, "jev", cfg.Backend)
	assert.Equal(t, DefaultBaseURL, cfg.BaseURL)
	assert.Equal(t, DefaultModel, cfg.Model)
}
