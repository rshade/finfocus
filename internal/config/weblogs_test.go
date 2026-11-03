package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envFrom(value string) func(string) string {
	return func(name string) string {
		if name != EnvWebLogs {
			return ""
		}
		return value
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestResolveWebLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		configured *bool
		want       bool
	}{
		{name: "env 1 wins over absent config", env: "1", configured: nil, want: true},
		{name: "env true wins over absent config", env: "true", configured: nil, want: true},
		{name: "env TRUE is case-insensitive", env: "TRUE", configured: nil, want: true},
		{name: "env padded with spaces", env: "  True ", configured: nil, want: true},
		{name: "env 0 wins over absent config", env: "0", configured: nil, want: false},
		{name: "env false wins over absent config", env: "false", configured: nil, want: false},
		{name: "env 1 wins over config false", env: "1", configured: boolPtr(false), want: true},
		{name: "env 0 wins over config true", env: "0", configured: boolPtr(true), want: false},
		{name: "env true wins over config false", env: "true", configured: boolPtr(false), want: true},
		{name: "env false wins over config true", env: "false", configured: boolPtr(true), want: false},
		{name: "empty env uses config true", env: "", configured: boolPtr(true), want: true},
		{name: "empty env uses config false", env: "", configured: boolPtr(false), want: false},
		{name: "empty env and absent config default to false", env: "", configured: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveWebLogs(envFrom(tt.env), tt.configured)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveWebLogs_InvalidEnvIsError(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"maybe", "yes", "2"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			_, err := ResolveWebLogs(envFrom(value), boolPtr(true))
			require.Error(t, err)
			assert.Contains(t, err.Error(), EnvWebLogs)
			assert.Contains(t, err.Error(), "1, true, 0, false")
		})
	}
}

func TestWebLogsConfig_SetGet(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	got, err := cfg.Get("web.logs.enabled")
	require.NoError(t, err)
	assert.Equal(t, false, got)

	require.NoError(t, cfg.Set("web.logs.enabled", "true"))
	require.NotNil(t, cfg.Web)
	require.NotNil(t, cfg.Web.Logs.Enabled)
	assert.True(t, *cfg.Web.Logs.Enabled)

	got, err = cfg.Get("web.logs.enabled")
	require.NoError(t, err)
	assert.Equal(t, true, got)

	require.NoError(t, cfg.Set("web.logs.enabled", "false"))
	assert.False(t, *cfg.Web.Logs.Enabled)
	assert.Contains(t, cfg.List(), keyWeb)
}

func TestWebLogsConfig_SetErrors(t *testing.T) {
	t.Parallel()

	cfg := &Config{}

	err := cfg.Set("web.logs.enabled", "maybe")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web.logs.enabled")
	assert.Contains(t, err.Error(), "true or false")

	err = cfg.Set("web.nope", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown web setting")

	_, err = cfg.Get("web.nope")
	require.Error(t, err)
}

func TestWebLogsConfig_ProjectOverlayReplacesWeb(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	overlay := filepath.Join(dir, "config.hujson")
	require.NoError(t, os.WriteFile(overlay, []byte(`{"web":{"logs":{"enabled":true}}}`), 0o600))

	cfg := &Config{}
	require.NoError(t, ShallowMergeYAML(cfg, overlay))
	require.NotNil(t, cfg.Web)
	require.NotNil(t, cfg.Web.Logs.Enabled)
	assert.True(t, *cfg.Web.Logs.Enabled)
}

func TestWebLogsConfig_SaveRoundTripWritesKeyOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.hujson")
	require.NoError(t, os.WriteFile(source, []byte(`{"web":{"logs":{"enabled":true}}}`), 0o600))

	loaded := &Config{}
	require.NoError(t, ShallowMergeYAML(loaded, source))

	target := filepath.Join(dir, "config.hujson")
	loaded.SetConfigPath(target)
	require.NoError(t, loaded.Save())

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(data), `"web"`), "saved file: %s", data)

	reloaded := &Config{}
	require.NoError(t, ShallowMergeYAML(reloaded, target))
	require.NotNil(t, reloaded.Web)
	require.NotNil(t, reloaded.Web.Logs.Enabled)
	assert.True(t, *reloaded.Web.Logs.Enabled)
}

func TestWebLogsConfig_ValidateAcceptsWebSection(t *testing.T) {
	t.Parallel()

	result, cfg := ValidateConfigSource("config.hujson", []byte(`{"web":{"logs":{"enabled":true}}}`))
	require.True(t, result.Valid, "errors: %+v", result.Errors)
	assert.Empty(t, result.Warnings)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Web)
}
