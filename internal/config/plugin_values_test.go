package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginConfigValues_KeepsSecretText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.hujson")
	body := []byte(`{
		"plugins": {
			"recorder": { "mock_response": "super-secret-value", "batch": 2 },
			"jev": { "api_key": "other" }
		}
	}`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	values, err := PluginConfigValues(path, "recorder")
	require.NoError(t, err)
	assert.Equal(t, "super-secret-value", values["mock_response"])
	assert.Equal(t, "2", values["batch"])
	assert.NotContains(t, values, "api_key")

	missing, err := PluginConfigValues(filepath.Join(dir, "absent.hujson"), "recorder")
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestPluginConfigValues_KeepsLargeIntegers(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	body := []byte(`{"plugins":{"recorder":{"batch":9007199254740993,"rate":1.5}}}`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	values, err := PluginConfigValues(path, "recorder")
	require.NoError(t, err)
	assert.Equal(t, "9007199254740993", values["batch"])
	assert.Equal(t, "1.5", values["rate"])
}

func TestPluginConfigValues_FormatsLargeYAMLFloat(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("plugins:\n  recorder:\n    batch: 1.0e20\n"), 0o600))

	values, err := PluginConfigValues(path, "recorder")
	require.NoError(t, err)
	assert.Equal(t, "100000000000000000000", values["batch"])
}
