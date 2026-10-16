package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // the default log path reads the process environment
func TestBuildPageSourcesAndUnresolvedProject(t *testing.T) {
	global := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(global, []byte(`{
	  "output": {"precision": 2, "default_format": "table"},
	  "plugins": {"aws": {
	    "region": "us-east-1",
	    "secret_key": "stored-secret",
	    "auth": {"profile": "default", "token": "nested-secret"}
	  }},
	  "cost": {"budgets": {"global": {
	    "amount": 10, "currency": "USD",
	    "alerts": [{"threshold": 80, "type": "actual", "notifications": [
	      {"type": "webhook", "url": "https://hooks.example/stored-url", "headers": {"Authorization": "Bearer stored-header"}}
	    ]}]
	  }}}
	}`), 0o600))
	projectDir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.hujson"), []byte(`{
	  // project comment
	  "output": {"precision": 4}
	}`), 0o600))

	page, err := buildPage(global, projectDir, []string{
		"FINFOCUS_LOG_LEVEL=debug",
		"FINFOCUS_OUTPUT_PRECISION=nope",
		"FINFOCUS_PLUGIN_AWS_ACCESS_KEY=env-secret",
		"FINFOCUS_PLUGIN_DIR=/plugins",
		"FINFOCUS_STRICT_COMPATIBILITY=true",
	})
	require.NoError(t, err)
	assert.True(t, page.ProjectAvailable)
	assert.Empty(t, page.ProjectUnavailableReason)
	assert.NotEqual(t, AbsentVersion, page.Versions[TierGlobal])
	assert.NotEqual(t, AbsentVersion, page.Versions[TierProject])

	precision := findSetting(t, page, "output.precision")
	assert.Equal(t, SourceProject, precision.Effective.Source)
	assert.Equal(t, 4, precision.Effective.Value)
	assert.Equal(t, 2, precision.Tiers[SourceGlobal].Value)
	assert.False(t, precision.Tiers[SourceEnv].Set)

	level := findSetting(t, page, "logging.level")
	assert.Equal(t, SourceEnv, level.Effective.Source)
	assert.Equal(t, "debug", level.Effective.Value)

	strict := findSetting(t, page, "plugin_host.strict_compatibility")
	assert.Equal(t, SourceEnv, strict.Effective.Source)
	assert.Equal(t, true, strict.Effective.Value)

	secret := findSetting(t, page, "plugins.aws.secret_key")
	assert.True(t, secret.Secret)
	assert.True(t, secret.Tiers[SourceGlobal].Set)
	assert.False(t, secret.Tiers[SourceGlobal].show)
	access := findSetting(t, page, "plugins.aws.access_key")
	assert.True(t, access.Secret)
	assert.Equal(t, SourceEnv, access.Effective.Source)
	assert.False(t, access.Effective.show)

	encoded, err := json.Marshal(page)
	require.NoError(t, err)
	body := string(encoded)
	assert.NotContains(t, body, "stored-secret")
	assert.NotContains(t, body, "nested-secret")
	assert.NotContains(t, body, "env-secret")
	assert.NotContains(t, body, "stored-url")
	assert.NotContains(t, body, "stored-header")
	assert.NotContains(t, body, RedactedValue)
	assert.Contains(t, body, "plugins.aws.region")
	auth := findSetting(t, page, "plugins.aws.auth")
	assert.False(t, auth.Secret)
	shown, ok := auth.Tiers[SourceGlobal].Value.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "default", shown["profile"])
	assert.NotContains(t, shown, "token")

	var secretRow bool
	for _, row := range page.ReadOnly {
		if row.Key == "cost.budgets.global.alerts[0].notifications[0].url" {
			secretRow = true
			assert.True(t, row.Secret)
			assert.True(t, row.Set)
			assert.Equal(t, SourceGlobal, row.Source)
			assert.False(t, row.show)
		}
	}
	assert.True(t, secretRow, "notification url is listed without its value")

	missing, err := buildPage(global, "", nil)
	require.NoError(t, err)
	assert.False(t, missing.ProjectAvailable)
	assert.Equal(t, projectUnavailableReason, missing.ProjectUnavailableReason)
	assert.Equal(t, AbsentVersion, missing.Versions[TierProject])
	assert.Equal(t, SourceGlobal, findSetting(t, missing, "output.precision").Effective.Source)
}

func TestBuildPageProjectSectionShadowsGlobal(t *testing.T) {
	t.Parallel()
	global := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(global, []byte(`{"output":{"precision":9,"default_format":"json"}}`), 0o600))
	projectDir := filepath.Join(t.TempDir(), ".finfocus")
	require.NoError(t, os.MkdirAll(projectDir, 0o700))
	projectFile := []byte(`{"output":{"default_format":"ndjson"}}`)
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "config.hujson"), projectFile, 0o600))

	page, err := buildPage(global, projectDir, nil)
	require.NoError(t, err)
	precision := findSetting(t, page, "output.precision")
	assert.Equal(t, SourceDefault, precision.Effective.Source)
	assert.Equal(t, defaultPrecision, precision.Effective.Value)
	assert.Equal(t, 9, precision.Tiers[SourceGlobal].Value)
	assert.False(t, precision.Tiers[SourceProject].Set)
	format := findSetting(t, page, "output.default_format")
	assert.Equal(t, SourceProject, format.Effective.Source)
	assert.Equal(t, "ndjson", format.Effective.Value)
}

func TestMaskedForDisplayHidesPluginCredentials(t *testing.T) {
	t.Parallel()
	cfg := &Config{Plugins: map[string]PluginConfig{
		"aws": {Config: map[string]any{
			"region":     "us-east-1",
			"secret_key": "stored-secret",
			"token":      "",
			"auth":       map[string]any{"profile": "default", "token": "nested-secret"},
		}},
	}}
	masked := cfg.MaskedForDisplay()
	assert.Equal(t, "us-east-1", masked.Plugins["aws"].Config["region"])
	assert.Equal(t, RedactedValue, masked.Plugins["aws"].Config["secret_key"])
	assert.Empty(t, masked.Plugins["aws"].Config["token"])
	auth, ok := masked.Plugins["aws"].Config["auth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "default", auth["profile"])
	assert.Equal(t, RedactedValue, auth["token"])
	assert.Equal(t, "stored-secret", cfg.Plugins["aws"].Config["secret_key"])
	original, ok := cfg.Plugins["aws"].Config["auth"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nested-secret", original["token"])
}

func findSetting(t *testing.T, page Page, key string) SettingView {
	t.Helper()
	var found SettingView
	var ok bool
	for _, setting := range page.Settings {
		if setting.Key == key {
			found = setting
			ok = true
		}
	}
	require.Truef(t, ok, "setting %s not found", key)
	return found
}

func TestBuildPageResolvesZeroScoringTimeout(t *testing.T) {
	t.Parallel()
	global := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(global, []byte(`{"scoring":{"timeout_seconds":0}}`), 0o600))

	page, err := buildPage(global, "", nil)
	require.NoError(t, err)
	timeout := findSetting(t, page, "scoring.timeout_seconds")
	assert.Equal(t, DefaultScoringTimeoutSeconds, timeout.Effective.Value)
	assert.Equal(t, SourceGlobal, timeout.Effective.Source)
	assert.Equal(t, 0, timeout.Tiers[SourceGlobal].Value)
	assert.True(t, timeout.Tiers[SourceGlobal].Set)
}
