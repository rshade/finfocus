package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestESCConfigRoundTripAndSet(t *testing.T) {
	t.Parallel()
	empty := []string{}
	cfg := &Config{ESC: &ESCConfig{
		Environments: &empty,
		Plugins: map[string]ESCPluginConfig{
			"recorder": {Variables: []string{"FOO"}},
		},
	}}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	var got Config
	require.NoError(t, json.Unmarshal(data, &got))
	require.NotNil(t, got.ESC)
	require.NotNil(t, got.ESC.Environments)
	assert.Empty(t, *got.ESC.Environments)
	assert.Equal(t, []string{"FOO"}, got.ESC.Plugins["recorder"].Variables)

	var unset Config
	require.NoError(t, json.Unmarshal([]byte(`{"output":{"default_format":"table"}}`), &unset))
	assert.Nil(t, unset.ESC)

	next := &Config{}
	require.NoError(t, next.Set("esc.environments", "billing/aws,shared/tags"))
	require.NoError(t, next.Set("esc.plugins.recorder.environments", "billing/recorder"))
	require.NoError(t, next.Set("esc.plugins.recorder.variables", "FOO,BAR"))
	envs, err := next.Get("esc.environments")
	require.NoError(t, err)
	assert.Equal(t, []string{"billing/aws", "shared/tags"}, envs)
	require.NoError(t, next.Set("esc.environments", ""))
	require.NotNil(t, next.ESC.Environments)
	assert.Empty(t, *next.ESC.Environments)
	assert.Contains(t, next.List(), keyESC)

	masked := next.MaskedForDisplay()
	require.NotNil(t, masked.ESC)
	assert.Equal(t, []string{"billing/recorder"}, *masked.ESC.Plugins["recorder"].Environments)
	assert.Equal(t, []string{"FOO", "BAR"}, masked.ESC.Plugins["recorder"].Variables)
}

func TestESCProjectReplacesGlobal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	overlay := filepath.Join(dir, "config.hujson")
	require.NoError(t, os.WriteFile(overlay, []byte(`{"esc":{"environments":["proj/env"]}}`), 0o600))
	globalList := []string{"global/env"}
	cfg := &Config{ESC: &ESCConfig{
		Environments: &globalList,
		Plugins:      map[string]ESCPluginConfig{"recorder": {Variables: []string{"KEEP"}}},
	}}
	require.NoError(t, ShallowMergeYAML(cfg, overlay))
	require.NotNil(t, cfg.ESC.Environments)
	assert.Equal(t, []string{"proj/env"}, *cfg.ESC.Environments)
	assert.Empty(t, cfg.ESC.Plugins)
}
